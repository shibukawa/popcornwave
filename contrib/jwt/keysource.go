package jwt

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/shibukawa/popcornwave/contrib/internal/authn"
)

// ErrDiscovery reports a metadata document that could not be fetched, parsed,
// or trusted. It carries no detail from the response, because a caller acting
// on the difference between "no document" and "wrong issuer" would be acting on
// something an attacker controls.
var ErrDiscovery = errors.New("jwt: key discovery failed")

// Metadata paths. Both are well-known locations under the issuer.
const (
	openIDConfigurationPath = "/.well-known/openid-configuration"
	authorizationServerPath = "/.well-known/oauth-authorization-server"
)

// KeySourceMode selects where the signing key set is found.
type KeySourceMode int

const (
	// DiscoverOpenID reads the OpenID Connect provider metadata.
	DiscoverOpenID KeySourceMode = iota
	// DiscoverAuthorizationServer reads the RFC 8414 metadata.
	DiscoverAuthorizationServer
	// KeySetDirect skips metadata entirely and uses the configured URI.
	KeySetDirect
)

const (
	defaultKeySetCacheTTL      = 15 * time.Minute
	defaultKeySourceTimeout    = 10 * time.Second
	defaultMetadataMaxBytes    = 64 << 10
	maxKeySourceCacheTTL       = 24 * time.Hour
	maxMetadataMembers         = 128
	defaultRefreshCooldown     = time.Minute
	maxKeySourceRequestTimeout = time.Minute
)

// KeySourceOptions configures a RemoteKeySet.
type KeySourceOptions struct {
	// Mode selects metadata discovery or a direct key set.
	Mode KeySourceMode
	// KeySetURI is read under KeySetDirect and ignored otherwise.
	KeySetURI string
	// AllowLoopbackHTTP permits an http issuer whose host is loopback, and
	// relaxes the same-origin rule below for it. Development only.
	AllowLoopbackHTTP bool
	// RefreshCooldown bounds how often an unknown kid may trigger a fetch, so
	// a stream of forged kid values cannot be amplified into traffic against
	// the issuer.
	RefreshCooldown time.Duration
	// CacheTTL is how long a fetched key set is used before it is refetched.
	CacheTTL time.Duration
	// RequestTimeout bounds one metadata or key set request.
	RequestTimeout time.Duration
	// JWKS bounds the parsed key document.
	JWKS JWKSOptions
	// HTTPClient is copied, and its redirect policy replaced: a redirect on a
	// key fetch is a request to take keys from somewhere else.
	HTTPClient *http.Client
	// Clock is injectable for tests.
	Clock func() time.Time
}

// RemoteKeySet resolves verification keys from an issuer's published key set.
//
// It is a KeyResolver, so it plugs into Verify unchanged. Resolution is
// synchronous and never blocks on the network once the cache is warm.
//
// The key set must share the issuer's origin. A metadata document that could
// point the key source at another host would make the issuer check decorative:
// whoever controls that host would control which signatures verify.
type RemoteKeySet struct {
	issuer  string
	options KeySourceOptions

	mu          sync.Mutex
	keySetURI   string
	keys        *JWKS
	fetchedAt   time.Time
	lastAttempt time.Time
}

// NewRemoteKeySet validates the options and returns a key set that has not yet
// fetched anything.
//
// Discovery is deferred to the first token that needs a key, so an application
// starts while its authorization server is unreachable and reports the failure
// on the request that needed it rather than at boot.
func NewRemoteKeySet(issuer string, options KeySourceOptions) (*RemoteKeySet, error) {
	if issuer == "" {
		return nil, ErrInvalidOptions
	}
	if options.CacheTTL < 0 || options.CacheTTL > maxKeySourceCacheTTL ||
		options.RefreshCooldown < 0 || options.RefreshCooldown > maxKeySourceCacheTTL ||
		options.RequestTimeout < 0 || options.RequestTimeout > maxKeySourceRequestTimeout {
		return nil, ErrInvalidOptions
	}
	issuerURL, err := authn.ParseEndpoint(issuer, options.AllowLoopbackHTTP)
	if err != nil || issuerURL.RawQuery != "" || issuerURL.Fragment != "" {
		return nil, ErrInvalidOptions
	}
	if options.CacheTTL == 0 {
		options.CacheTTL = defaultKeySetCacheTTL
	}
	if options.RefreshCooldown == 0 {
		options.RefreshCooldown = defaultRefreshCooldown
	}
	if options.RequestTimeout == 0 {
		options.RequestTimeout = defaultKeySourceTimeout
	}
	if options.Clock == nil {
		options.Clock = time.Now
	}
	set := &RemoteKeySet{issuer: issuer, options: options}
	if options.Mode == KeySetDirect {
		uri, err := set.trustedEndpoint(options.KeySetURI)
		if err != nil {
			return nil, ErrInvalidOptions
		}
		set.keySetURI = uri
	}
	return set, nil
}

// Resolve returns the verification key for a header, fetching or refreshing the
// key set when it must.
//
// An unknown kid triggers at most one refresh, and no more often than the
// cooldown: rotation is the honest reason a kid is unknown, and a forged kid is
// the dishonest one, so the retry exists but is rate limited.
func (set *RemoteKeySet) Resolve(ctx context.Context, header Header) (VerificationKey, error) {
	if set == nil {
		return VerificationKey{}, ErrKeyNotFound
	}
	keys, err := set.current(ctx)
	if err != nil {
		return VerificationKey{}, err
	}
	key, err := keys.ResolveKey(header)
	if !errors.Is(err, ErrKeyNotFound) {
		return key, err
	}
	refreshed, ok := set.refreshForUnknownKey(ctx)
	if !ok {
		return VerificationKey{}, ErrKeyNotFound
	}
	return refreshed.ResolveKey(header)
}

// ResolverFor adapts this key set to the KeyResolver interface Verify takes.
// The context is captured because ResolveKey has nowhere to carry one, and a
// key fetch without a deadline is a request handler without one.
func (set *RemoteKeySet) ResolverFor(ctx context.Context) KeyResolver {
	return KeyResolverFunc(func(header Header) (VerificationKey, error) {
		return set.Resolve(ctx, header)
	})
}

// current returns a key set that is within its cache TTL, fetching one when
// there is none or the cached one has aged out.
func (set *RemoteKeySet) current(ctx context.Context) (*JWKS, error) {
	set.mu.Lock()
	defer set.mu.Unlock()
	now := set.options.Clock()
	if set.keys != nil && now.Sub(set.fetchedAt) < set.options.CacheTTL {
		return set.keys, nil
	}
	if err := set.fetchLocked(ctx, now); err != nil {
		if set.keys != nil {
			// A refresh failure on a set that still parses is not a reason to
			// stop verifying: the keys did not become untrustworthy because the
			// issuer became unreachable.
			return set.keys, nil
		}
		return nil, err
	}
	return set.keys, nil
}

// refreshForUnknownKey performs the single rate-limited refetch an unknown kid
// is allowed to cause. It reports false when the cooldown has not elapsed.
func (set *RemoteKeySet) refreshForUnknownKey(ctx context.Context) (*JWKS, bool) {
	set.mu.Lock()
	defer set.mu.Unlock()
	now := set.options.Clock()
	if now.Sub(set.lastAttempt) < set.options.RefreshCooldown {
		return nil, false
	}
	if err := set.fetchLocked(ctx, now); err != nil {
		return nil, false
	}
	return set.keys, true
}

// fetchLocked resolves the key set URI when it is not known yet and replaces
// the cached keys. The caller holds the mutex, so concurrent requests for an
// unknown kid produce one fetch rather than one each.
func (set *RemoteKeySet) fetchLocked(ctx context.Context, now time.Time) error {
	set.lastAttempt = now
	if set.keySetURI == "" {
		uri, err := set.discover(ctx)
		if err != nil {
			return err
		}
		set.keySetURI = uri
	}
	body, err := set.get(ctx, set.keySetURI)
	if err != nil {
		return err
	}
	keys, err := ParseJWKS(body, set.options.JWKS)
	if err != nil {
		return err
	}
	set.keys = keys
	set.fetchedAt = now
	return nil
}

// discover reads the metadata document and returns its trusted jwks_uri.
//
// The document's own issuer must equal the configured one. Without that check a
// deployment pointed at the wrong host would accept whatever that host called
// itself.
func (set *RemoteKeySet) discover(ctx context.Context) (string, error) {
	path := openIDConfigurationPath
	if set.options.Mode == DiscoverAuthorizationServer {
		path = authorizationServerPath
	}
	metadataURL, err := url.Parse(set.issuer)
	if err != nil {
		return "", ErrDiscovery
	}
	metadataURL.Path = strings.TrimSuffix(metadataURL.Path, "/") + path
	metadataURL.RawPath = ""
	body, err := set.get(ctx, metadataURL.String())
	if err != nil {
		return "", err
	}
	if err := authn.ValidateJSON(body, authn.JSONOptions{
		MaxBytes: defaultMetadataMaxBytes, MaxDepth: 8, MaxMembers: maxMetadataMembers,
	}); err != nil {
		return "", ErrDiscovery
	}
	var document struct {
		Issuer  string `json:"issuer"`
		JWKSURI string `json:"jwks_uri"`
	}
	if err := json.Unmarshal(body, &document); err != nil {
		return "", ErrDiscovery
	}
	if document.Issuer != set.issuer || document.JWKSURI == "" {
		return "", ErrDiscovery
	}
	return set.trustedEndpoint(document.JWKSURI)
}

// trustedEndpoint accepts a key set URI only on the issuer's own origin.
//
// The loopback allowance covers development, where the issuer and the key set
// are the same local process and neither is https.
func (set *RemoteKeySet) trustedEndpoint(raw string) (string, error) {
	parsed, err := authn.ParseEndpoint(raw, set.options.AllowLoopbackHTTP)
	if err != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", ErrDiscovery
	}
	issuerURL, err := url.Parse(set.issuer)
	if err != nil {
		return "", ErrDiscovery
	}
	if !strings.EqualFold(parsed.Host, issuerURL.Host) {
		return "", ErrDiscovery
	}
	if parsed.Scheme != issuerURL.Scheme {
		return "", ErrDiscovery
	}
	return parsed.String(), nil
}

// get performs one bounded GET with redirects rejected.
func (set *RemoteKeySet) get(ctx context.Context, endpoint string) ([]byte, error) {
	client := set.options.HTTPClient
	if client == nil {
		client = &http.Client{}
	} else {
		copied := *client
		client = &copied
	}
	client.CheckRedirect = authn.RejectRedirect

	requestCtx, cancel := context.WithTimeout(ctx, set.options.RequestTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(requestCtx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, ErrDiscovery
	}
	request.Header.Set("Accept", "application/json")
	response, err := client.Do(request)
	if err != nil {
		return nil, ErrDiscovery
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return nil, ErrDiscovery
	}
	maxBytes := set.options.JWKS.MaxBytes
	if maxBytes <= 0 || maxBytes > defaultMetadataMaxBytes {
		maxBytes = defaultMetadataMaxBytes
	}
	body, err := authn.ReadBounded(response.Body, int64(maxBytes))
	if err != nil {
		return nil, ErrDiscovery
	}
	return body, nil
}
