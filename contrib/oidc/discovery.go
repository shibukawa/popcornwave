package oidc

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/shibukawa/popcornwave/contrib/internal/authn"
	"github.com/shibukawa/popcornwave/contrib/jwt"
)

// Discover validates the issuer metadata and fetches its initial signing key
// set. Redirects are disabled for all discovery and JWKS requests.
func Discover(ctx context.Context, issuer string, options DiscoverOptions) (*Provider, error) {
	if ctx == nil || issuer == "" || options.RequestTimeout < 0 || options.RequestTimeout > 10*time.Minute ||
		options.MaxResponseBytes < 0 || options.MaxResponseBytes > maxMaxResponseBytes ||
		options.JWKSMaxBytes < 0 || options.JWKSMaxBytes > maxJWKSBytes ||
		options.JWKSMaxKeys < 0 || options.JWKSMaxKeys > maxJWKSKeys ||
		options.JWKSCacheTTL < 0 || options.JWKSCacheTTL > maxJWKSCacheTTL ||
		options.JWKSStaleTTL < 0 || options.JWKSStaleTTL > maxJWKSStaleTTL ||
		(options.JWKSCacheTTL > 0 && options.JWKSStaleTTL > 0 && options.JWKSStaleTTL < options.JWKSCacheTTL) {
		return nil, ErrInvalidOptions
	}
	issuerURL, err := authn.ParseEndpoint(issuer, options.AllowLoopbackHTTP)
	if err != nil || issuerURL.RawQuery != "" || issuerURL.Fragment != "" {
		return nil, ErrDiscovery
	}
	if options.EndpointValidator != nil {
		candidate := *issuerURL
		if err := options.EndpointValidator(&candidate); err != nil {
			return nil, ErrDiscovery
		}
	}
	httpClient := options.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{}
	} else {
		copy := *httpClient
		httpClient = &copy
	}
	httpClient.CheckRedirect = authn.RejectRedirect
	clock := options.Clock
	if clock == nil {
		clock = time.Now
	}
	maxResponse := options.MaxResponseBytes
	if maxResponse == 0 {
		maxResponse = defaultMaxResponseBytes
	}
	cacheTTL := options.JWKSCacheTTL
	if cacheTTL == 0 {
		cacheTTL = defaultJWKSCacheTTL
	}
	staleTTL := options.JWKSStaleTTL
	if staleTTL == 0 {
		staleTTL = defaultJWKSStaleTTL
	}
	requestTimeout := options.RequestTimeout
	if requestTimeout == 0 {
		requestTimeout = defaultRequestTimeout
	}
	provider := &Provider{issuer: issuer, options: providerOptions{
		httpClient: httpClient, clock: clock, requestTimeout: requestTimeout,
		maxResponseBytes: maxResponse, jwksMaxBytes: options.JWKSMaxBytes,
		jwksMaxKeys: options.JWKSMaxKeys, cacheTTL: cacheTTL, staleTTL: staleTTL,
		allowLoopbackHTTP: options.AllowLoopbackHTTP, endpointValidator: options.EndpointValidator,
	}}
	discoveryURL := *issuerURL
	discoveryURL.Path = strings.TrimSuffix(discoveryURL.Path, "/") + "/.well-known/openid-configuration"
	discoveryURL.RawPath = ""
	body, err := provider.get(ctx, discoveryURL.String())
	if err != nil {
		return nil, err
	}
	var document discoveryDocument
	if err := authn.ValidateJSON(body, authn.JSONOptions{MaxBytes: maxResponse, MaxDepth: 8, MaxMembers: maxDiscoveryMembers}); err != nil {
		return nil, classifyJSON(err)
	}
	if err := json.Unmarshal(body, &document); err != nil || document.Issuer == "" || document.AuthorizationEndpoint == "" || document.TokenEndpoint == "" || document.JWKSURI == "" {
		return nil, ErrDiscovery
	}
	if document.Issuer != issuer {
		return nil, ErrDiscovery
	}
	provider.authorizationEndpoint, err = provider.endpoint(document.AuthorizationEndpoint)
	if err != nil {
		return nil, ErrDiscovery
	}
	provider.tokenEndpoint, err = provider.endpoint(document.TokenEndpoint)
	if err != nil {
		return nil, ErrDiscovery
	}
	provider.jwksURI, err = provider.endpoint(document.JWKSURI)
	if err != nil {
		return nil, ErrDiscovery
	}
	if document.UserInfoEndpoint != "" {
		provider.userInfoEndpoint, err = provider.endpoint(document.UserInfoEndpoint)
		if err != nil {
			return nil, ErrDiscovery
		}
	}
	if err := provider.refresh(ctx); err != nil {
		return nil, err
	}
	return provider, nil
}

func (p *Provider) endpoint(raw string) (string, error) {
	parsed, err := authn.ParseEndpoint(raw, p.options.allowLoopbackHTTP)
	if err != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", ErrDiscovery
	}
	if p.options.endpointValidator != nil {
		candidate := *parsed
		if err := p.options.endpointValidator(&candidate); err != nil {
			return "", err
		}
	}
	return parsed.String(), nil
}

func (p *Provider) get(ctx context.Context, endpoint string) ([]byte, error) {
	body, _, err := p.getResponse(ctx, endpoint)
	return body, err
}

func (p *Provider) getResponse(ctx context.Context, endpoint string) ([]byte, http.Header, error) {
	requestCtx := ctx
	var cancel context.CancelFunc
	if p.options.requestTimeout > 0 {
		requestCtx, cancel = context.WithTimeout(ctx, p.options.requestTimeout)
		defer cancel()
	}
	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, nil, ErrHTTP
	}
	req.Header.Set("Accept", "application/json")
	response, err := p.options.httpClient.Do(req)
	if err != nil {
		return nil, nil, ErrHTTP
	}
	defer response.Body.Close()
	body, err := authn.ReadBounded(response.Body, int64(p.options.maxResponseBytes))
	if err != nil {
		if errors.Is(err, authn.ErrLimitExceeded) {
			return nil, nil, ErrLimitExceeded
		}
		return nil, nil, ErrHTTP
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, response.Header, ErrDiscovery
	}
	return body, response.Header, nil
}

func (p *Provider) refresh(ctx context.Context) error {
	if p == nil || ctx == nil {
		return ErrInvalidOptions
	}
	// Serialize cache refreshes so concurrent unknown-kid or expiry paths
	// cannot amplify a single verification into an unbounded request burst.
	p.refreshMu.Lock()
	defer p.refreshMu.Unlock()
	body, headers, err := p.getResponse(ctx, p.jwksURI)
	if err != nil {
		return err
	}
	keys, err := jwt.ParseJWKS(body, jwt.JWKSOptions{MaxBytes: p.options.jwksMaxBytes, MaxKeys: p.options.jwksMaxKeys})
	if err != nil {
		if errors.Is(err, jwt.ErrLimitExceeded) {
			return ErrLimitExceeded
		}
		return ErrDiscovery
	}
	fetchedAt := p.options.clock()
	freshness := p.options.cacheTTL
	maxAge, hasMaxAge, noStore := parseCacheControl(headers.Get("Cache-Control"))
	if hasMaxAge {
		if maxAge <= int64(freshness/time.Second) {
			freshness = time.Duration(maxAge) * time.Second
		}
	}
	staleUntil := fetchedAt.Add(p.options.staleTTL)
	if noStore {
		staleUntil = fetchedAt
	}
	p.mu.Lock()
	p.keys = keys
	p.fetchedAt = fetchedAt
	p.cacheExpiresAt = fetchedAt.Add(freshness)
	p.staleExpiresAt = staleUntil
	p.mu.Unlock()
	return nil
}

func (p *Provider) resolveKey(ctx context.Context, header jwt.Header) (jwt.VerificationKey, error) {
	now := p.options.clock()
	p.mu.RLock()
	keys, cacheExpiresAt, staleExpiresAt := p.keys, p.cacheExpiresAt, p.staleExpiresAt
	p.mu.RUnlock()
	if keys == nil || (!staleExpiresAt.IsZero() && !now.Before(staleExpiresAt)) {
		if err := p.refresh(ctx); err != nil {
			return jwt.VerificationKey{}, jwt.ErrKeyNotFound
		}
		p.mu.RLock()
		keys = p.keys
		p.mu.RUnlock()
	} else if !cacheExpiresAt.IsZero() && !now.Before(cacheExpiresAt) {
		_ = p.refresh(ctx)
		p.mu.RLock()
		keys = p.keys
		p.mu.RUnlock()
	}
	key, err := keys.ResolveKey(header)
	if err == nil {
		return key, nil
	}
	if errors.Is(err, jwt.ErrKeyNotFound) {
		// A new key is allowed exactly one bounded refresh attempt.
		if refreshErr := p.refresh(ctx); refreshErr == nil {
			p.mu.RLock()
			keys = p.keys
			p.mu.RUnlock()
			return keys.ResolveKey(header)
		}
	}
	return jwt.VerificationKey{}, err
}

func parseCacheMaxAge(value string) (int64, bool) {
	maxAge, hasMaxAge, noStore := parseCacheControl(value)
	return maxAge, hasMaxAge || noStore
}

func parseCacheControl(value string) (int64, bool, bool) {
	noStore := false
	noCache := false
	var maxAge int64
	hasMaxAge := false
	for _, directive := range strings.Split(value, ",") {
		directive = strings.TrimSpace(directive)
		name, raw, found := strings.Cut(directive, "=")
		if !found && strings.EqualFold(directive, "no-store") {
			noStore = true
			continue
		}
		if !found && strings.EqualFold(directive, "no-cache") {
			noCache = true
			continue
		}
		if !found || !strings.EqualFold(strings.TrimSpace(name), "max-age") {
			continue
		}
		seconds, err := strconv.ParseInt(strings.Trim(strings.TrimSpace(raw), "\""), 10, 64)
		if err != nil || seconds < 0 {
			continue
		}
		maxAge, hasMaxAge = seconds, true
	}
	if noCache {
		return 0, true, noStore
	}
	return maxAge, hasMaxAge, noStore
}

func classifyJSON(err error) error {
	if errors.Is(err, authn.ErrLimitExceeded) {
		return ErrLimitExceeded
	}
	return ErrDiscovery
}
