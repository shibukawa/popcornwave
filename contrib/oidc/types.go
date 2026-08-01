package oidc

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/shibukawa/popcornwave/contrib/jwt"
	"github.com/shibukawa/popcornwave/contrib/oauth"
)

var (
	ErrInvalidConfig  = errors.New("oidc: invalid configuration")
	ErrInvalidOptions = errors.New("oidc: invalid options")
	ErrDiscovery      = errors.New("oidc: invalid discovery metadata")
	ErrNonce          = errors.New("oidc: nonce mismatch")
	ErrIDToken        = errors.New("oidc: invalid ID token")
	// ErrAuthTime reports an ID Token that carries no usable auth_time when
	// the caller required one. It is distinct from ErrIDToken because the
	// token is otherwise valid and the caller's remedy is different: the
	// provider answered a freshness request it did not honor.
	ErrAuthTime = errors.New("oidc: missing or invalid auth_time")
	ErrUserInfo       = errors.New("oidc: invalid UserInfo response")
	ErrLimitExceeded  = errors.New("oidc: response limit exceeded")
	ErrHTTP           = errors.New("oidc: HTTP request failed")
)

const (
	defaultMaxResponseBytes = 64 << 10
	maxMaxResponseBytes     = 4 << 20
	// maxDiscoveryMembers bounds every object member and array element of a
	// discovery document. Published providers advertise long capability
	// arrays, so this counts far more than the document's top-level members
	// while the response byte limit still bounds the whole payload.
	maxDiscoveryMembers   = 512
	maxJWKSBytes          = 16 << 20
	maxJWKSKeys           = 4096
	defaultRequestTimeout = 30 * time.Second
	defaultJWKSCacheTTL   = 5 * time.Minute
	defaultJWKSStaleTTL   = 15 * time.Minute
	maxJWKSCacheTTL       = time.Hour
	maxJWKSStaleTTL       = 24 * time.Hour
)

type DiscoverOptions struct {
	HTTPClient *http.Client
	Clock      func() time.Time
	// RequestTimeout defaults to 30 seconds and is capped at 10 minutes.
	RequestTimeout    time.Duration
	MaxResponseBytes  int
	JWKSMaxBytes      int
	JWKSMaxKeys       int
	JWKSCacheTTL      time.Duration
	JWKSStaleTTL      time.Duration
	AllowLoopbackHTTP bool
	// EndpointValidator receives a copy of each issuer/discovered endpoint URL
	// for caller-specific host/IP trust checks. Mutations are ignored.
	EndpointValidator func(*url.URL) error
}

// Provider contains validated discovery metadata and a bounded JWKS cache.
type Provider struct {
	issuer                string
	authorizationEndpoint string
	tokenEndpoint         string
	jwksURI               string
	userInfoEndpoint      string
	endSessionEndpoint    string
	options               providerOptions
	mu                    sync.RWMutex
	refreshMu             sync.Mutex
	keys                  *jwt.JWKS
	fetchedAt             time.Time
	cacheExpiresAt        time.Time
	staleExpiresAt        time.Time
}

type providerOptions struct {
	httpClient        *http.Client
	clock             func() time.Time
	requestTimeout    time.Duration
	maxResponseBytes  int
	jwksMaxBytes      int
	jwksMaxKeys       int
	cacheTTL          time.Duration
	staleTTL          time.Duration
	allowLoopbackHTTP bool
	endpointValidator func(*url.URL) error
}

type Config struct {
	ClientID          string
	ClientSecret      string
	RedirectURI       string
	AuthMethod        string
	AllowLoopbackHTTP bool
}

type Options struct {
	OAuth             oauth.Options
	Random            io.Reader
	Clock             func() time.Time
	AllowedAlgorithms []string
	Leeway            time.Duration
	MaxTokenBytes     int
	MaxSegmentBytes   int
}

// maxEndSessionStateBytes bounds the opaque value returned to the post-logout
// redirect URI.
const maxEndSessionStateBytes = 512

type Client struct {
	provider          *Provider
	clientID          string
	allowLoopbackHTTP bool
	oauth             *oauth.Client
	random            io.Reader
	clock             func() time.Time
	allowedAlgorithms []string
	leeway            time.Duration
	maxTokenBytes     int
	maxSegmentBytes   int
}

type BeginOptions struct {
	Scopes []string
	Params map[string]string
	// MaxAge asks the provider to re-authenticate when the end user's last
	// active authentication is older than this. Nil sends nothing; a zero
	// duration is meaningful and sends max_age=0, which asks for
	// authentication now. Sub-second values truncate toward zero, which only
	// ever tightens the request.
	//
	// A provider that honors max_age MUST return auth_time, so a caller that
	// sets this sets CallbackOptions.RequireAuthTime as well. Sending the
	// parameter without checking the answer proves nothing.
	MaxAge *time.Duration
	// Prompt carries the OpenID Connect prompt values. Only none, login,
	// consent, and select_account are accepted, and none may not be combined
	// with another value.
	//
	// Unlike MaxAge, prompt is unverifiable: no claim reports whether the
	// provider honored it, so it improves an interaction and never proves one.
	Prompt []string
}

// CallbackOptions controls verification of the returned ID Token.
type CallbackOptions struct {
	// RequireAuthTime rejects a token carrying no usable auth_time. Set it
	// whenever BeginOptions.MaxAge was set, because OpenID Connect requires
	// the claim in exactly that case, and its absence means the provider did
	// not answer the question that was asked.
	//
	// This package verifies that auth_time is present and sane. Comparing it
	// against a freshness requirement is the caller's, because the meaning of
	// "recent enough" belongs to the caller's policy rather than to the
	// protocol.
	RequireAuthTime bool
}

type Callback = oauth.Callback
type TokenSet = oauth.TokenSet

type IDToken struct {
	Claims jwt.Claims
	Nonce  string
	// AuthTime is the verified auth_time claim, the moment the provider last
	// actively authenticated the end user. It is nil when the claim is absent.
	//
	// It is not the moment this token was issued: a provider may satisfy an
	// authorization request from a single sign-on session established much
	// earlier, so freshness is measured from here and never from arrival.
	AuthTime *time.Time
	// ACR is the verified acr claim, empty when absent. This package does not
	// rank its values, because their meaning is defined by the provider.
	ACR string
}

type discoveryDocument struct {
	Issuer                string `json:"issuer"`
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	JWKSURI               string `json:"jwks_uri"`
	UserInfoEndpoint      string `json:"userinfo_endpoint"`
	EndSessionEndpoint    string `json:"end_session_endpoint"`
}

// EndSessionOptions builds an RP-initiated logout request.
type EndSessionOptions struct {
	// IDToken is the raw ID Token of the session being ended. It is
	// RECOMMENDED by the specification and required by some providers.
	IDToken string
	// PostLogoutRedirectURI must be registered with the provider.
	PostLogoutRedirectURI string
	// State is returned unchanged to the post-logout URI.
	State string
}

type userInfoResult map[string]json.RawMessage
