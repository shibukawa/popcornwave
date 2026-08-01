package oidc

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"net/http"
	urlpkg "net/url"
	"strconv"
	"strings"
	"time"

	"github.com/shibukawa/popcornwave/contrib/internal/authn"
	"github.com/shibukawa/popcornwave/contrib/jwt"
	"github.com/shibukawa/popcornwave/contrib/oauth"
)

const (
	defaultMaxTokenBytes   = 16 << 10
	defaultMaxSegmentBytes = 8 << 10
	maxMaxTokenBytes       = 4 << 20
	maxMaxSegmentBytes     = 2 << 20
)

// NewClient creates an OIDC client over a validated provider.
func NewClient(provider *Provider, config Config, options Options) (*Client, error) {
	if provider == nil || config.ClientID == "" || config.ClientSecret == "" || config.RedirectURI == "" {
		return nil, ErrInvalidConfig
	}
	if options.Leeway < 0 || options.Leeway > 10*time.Minute ||
		options.MaxTokenBytes < 0 || options.MaxTokenBytes > maxMaxTokenBytes ||
		options.MaxSegmentBytes < 0 || options.MaxSegmentBytes > maxMaxSegmentBytes {
		return nil, ErrInvalidOptions
	}
	random := options.Random
	if random == nil {
		random = rand.Reader
	}
	clock := options.Clock
	if clock == nil {
		clock = time.Now
	}
	algorithms := append([]string(nil), options.AllowedAlgorithms...)
	if len(algorithms) == 0 {
		algorithms = []string{"RS256"}
	}
	for _, algorithm := range algorithms {
		if algorithm != "RS256" {
			return nil, ErrInvalidOptions
		}
	}
	maxTokenBytes := options.MaxTokenBytes
	if maxTokenBytes == 0 {
		maxTokenBytes = defaultMaxTokenBytes
	}
	maxSegmentBytes := options.MaxSegmentBytes
	if maxSegmentBytes == 0 {
		maxSegmentBytes = defaultMaxSegmentBytes
	}
	oauthConfig := oauth.Config{
		AuthorizationEndpoint: provider.authorizationEndpoint,
		TokenEndpoint:         provider.tokenEndpoint,
		ClientID:              config.ClientID, ClientSecret: config.ClientSecret,
		RedirectURI: config.RedirectURI, AuthMethod: config.AuthMethod,
		AllowLoopbackHTTP: config.AllowLoopbackHTTP,
	}
	oauthOptions := options.OAuth
	callerValidator := oauthOptions.TransactionValidator
	oauthOptions.TransactionValidator = func(transaction oauth.Transaction) error {
		if callerValidator != nil {
			if err := callerValidator(transaction); err != nil {
				return err
			}
		}
		nonce, err := authn.DecodeBase64URL(transaction.Nonce, 256, 64)
		if err != nil || len(nonce) < 16 {
			return ErrNonce
		}
		return nil
	}
	if oauthOptions.Random == nil {
		oauthOptions.Random = random
	}
	if oauthOptions.Clock == nil {
		oauthOptions.Clock = clock
	}
	if oauthOptions.HTTPClient == nil {
		oauthOptions.HTTPClient = provider.options.httpClient
	}
	client, err := oauth.NewClient(oauthConfig, oauthOptions)
	if err != nil {
		return nil, err
	}
	return &Client{provider: provider, clientID: config.ClientID, allowLoopbackHTTP: config.AllowLoopbackHTTP, oauth: client, random: random, clock: clock, allowedAlgorithms: algorithms, leeway: options.Leeway, maxTokenBytes: maxTokenBytes, maxSegmentBytes: maxSegmentBytes}, nil
}

// BeginAuthorization generates and stores the OIDC nonce in the OAuth
// transaction. The returned key is opaque and must be supplied to callback.
func (c *Client) BeginAuthorization(ctx context.Context, options BeginOptions) (string, string, error) {
	if c == nil || ctx == nil {
		return "", "", ErrInvalidOptions
	}
	nonce, err := authn.GenerateSecret(c.random, 32)
	if err != nil {
		return "", "", err
	}
	scopes := append([]string(nil), options.Scopes...)
	hasOpenID := false
	for _, scope := range scopes {
		if scope == "openid" {
			hasOpenID = true
			break
		}
	}
	if !hasOpenID {
		scopes = append([]string{"openid"}, scopes...)
	}
	params, err := authenticationParams(options)
	if err != nil {
		return "", "", err
	}
	urlValue, key, err := c.oauth.BeginAuthorization(ctx, oauth.BeginOptions{Scopes: scopes, Params: params, Nonce: nonce})
	return urlValue, key, err
}

// promptValues are the values OpenID Connect Core defines. An unlisted value
// is refused rather than forwarded: a provider that does not recognize it may
// ignore it silently, and a caller would read the resulting silence as an
// honored request.
var promptValues = map[string]bool{"none": true, "login": true, "consent": true, "select_account": true}

// authenticationParams merges the typed authentication parameters into the
// caller's Params. Setting either through Params directly is refused, because
// max_age carries a verification obligation this package cannot see through an
// untyped map, and a request that asks for freshness without checking the
// answer is worse than one that never asked.
func authenticationParams(options BeginOptions) (map[string]string, error) {
	for key := range options.Params {
		if key == "max_age" || key == "prompt" {
			return nil, ErrInvalidOptions
		}
	}
	if options.MaxAge == nil && len(options.Prompt) == 0 {
		return options.Params, nil
	}
	params := make(map[string]string, len(options.Params)+2)
	for key, value := range options.Params {
		params[key] = value
	}
	if options.MaxAge != nil {
		if *options.MaxAge < 0 {
			return nil, ErrInvalidOptions
		}
		params["max_age"] = strconv.FormatInt(int64(*options.MaxAge/time.Second), 10)
	}
	if len(options.Prompt) > 0 {
		if len(options.Prompt) > len(promptValues) {
			return nil, ErrInvalidOptions
		}
		seen := make(map[string]bool, len(options.Prompt))
		for _, value := range options.Prompt {
			if !promptValues[value] || seen[value] {
				return nil, ErrInvalidOptions
			}
			seen[value] = true
		}
		if seen["none"] && len(options.Prompt) > 1 {
			return nil, ErrInvalidOptions
		}
		params["prompt"] = strings.Join(options.Prompt, " ")
	}
	return params, nil
}

// HandleCallback exchanges the code and verifies the returned ID Token,
// including its nonce against the atomically consumed transaction.
func (c *Client) HandleCallback(ctx context.Context, key string, callback Callback, options CallbackOptions) (TokenSet, IDToken, error) {
	set, transaction, err := c.oauth.HandleCallbackWithTransaction(ctx, key, callback)
	if err != nil {
		return TokenSet{}, IDToken{}, err
	}
	if !strings.EqualFold(set.TokenType, "Bearer") {
		return TokenSet{}, IDToken{}, ErrIDToken
	}
	nonce, nonceErr := authn.DecodeBase64URL(transaction.Nonce, 256, 64)
	if nonceErr != nil || len(nonce) < 16 {
		return TokenSet{}, IDToken{}, ErrNonce
	}
	raw, ok := set.Raw["id_token"]
	if !ok {
		return TokenSet{}, IDToken{}, ErrIDToken
	}
	var idToken string
	if json.Unmarshal(raw, &idToken) != nil || idToken == "" {
		return TokenSet{}, IDToken{}, ErrIDToken
	}
	verified, err := c.verifyIDToken(ctx, idToken, transaction.Nonce)
	if err != nil {
		return TokenSet{}, IDToken{}, err
	}
	// OpenID Connect requires auth_time whenever max_age was requested, so its
	// absence here means the provider did not answer the question that was
	// asked. Treating that as a completed re-authentication is the whole of
	// the trap this check exists to close.
	if options.RequireAuthTime && verified.AuthTime == nil {
		return TokenSet{}, IDToken{}, ErrAuthTime
	}
	return set, verified, nil
}

// VerifyIDToken verifies JWT signature and registered OIDC claims. It also
// requires a nonce claim, but cannot bind that claim to a browser transaction.
// Callers outside HandleCallback should use VerifyIDTokenWithNonce instead.
func (c *Client) VerifyIDToken(ctx context.Context, raw string) (IDToken, error) {
	return c.verifyIDToken(ctx, raw, "")
}

func (c *Client) VerifyIDTokenWithNonce(ctx context.Context, raw, expectedNonce string) (IDToken, error) {
	if expectedNonce == "" {
		return IDToken{}, ErrNonce
	}
	return c.verifyIDToken(ctx, raw, expectedNonce)
}

func (c *Client) verifyIDToken(ctx context.Context, raw, expectedNonce string) (IDToken, error) {
	if c == nil || ctx == nil || raw == "" {
		return IDToken{}, ErrIDToken
	}
	token, err := jwt.Parse(raw, jwt.ParseOptions{MaxTokenBytes: c.maxTokenBytes, MaxSegmentBytes: c.maxSegmentBytes})
	if err != nil {
		return IDToken{}, ErrIDToken
	}
	claims, err := jwt.Verify(token, jwt.KeyResolverFunc(func(header jwt.Header) (jwt.VerificationKey, error) { return c.provider.resolveKey(ctx, header) }), jwt.VerifyOptions{AllowedAlgorithms: c.allowedAlgorithms, Issuer: c.provider.issuer, Audience: c.clientID, Clock: c.clock, Leeway: c.leeway})
	if err != nil {
		return IDToken{}, ErrIDToken
	}
	if claims.Subject == "" || claims.IssuedAt == nil {
		return IDToken{}, ErrIDToken
	}
	nonce, ok := claims.String("nonce")
	if !ok || nonce == "" {
		return IDToken{}, ErrNonce
	}
	if expectedNonce != "" && !authn.EqualSecret(expectedNonce, nonce) {
		return IDToken{}, ErrNonce
	}
	if err := validateAuthorizedParty(claims, c.clientID); err != nil {
		return IDToken{}, err
	}
	authTime, err := c.authTime(claims)
	if err != nil {
		return IDToken{}, err
	}
	acr, acrErr := stringClaim(claims, "acr")
	if acrErr != nil {
		return IDToken{}, ErrIDToken
	}
	return IDToken{Claims: claims, Nonce: nonce, AuthTime: authTime, ACR: acr}, nil
}

// authTime reads the auth_time claim. A present claim of the wrong shape is an
// error rather than an absence, because a caller measuring freshness from a
// value silently dropped would measure from nothing at all.
func (c *Client) authTime(claims jwt.Claims) (*time.Time, error) {
	raw, present := claims.Value("auth_time")
	if !present {
		return nil, nil
	}
	// encoding/json will decode a quoted "1700000000" into a json.Number, so
	// the JSON type is checked on the raw bytes first. A provider that quotes
	// the value is reporting a string where the specification requires a
	// number, and guessing its intent would accept a shape nothing verified.
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || (trimmed[0] != '-' && (trimmed[0] < '0' || trimmed[0] > '9')) {
		return nil, ErrAuthTime
	}
	var number json.Number
	if json.Unmarshal(trimmed, &number) != nil {
		return nil, ErrAuthTime
	}
	seconds, err := number.Int64()
	if err != nil || seconds < 0 {
		return nil, ErrAuthTime
	}
	value := time.Unix(seconds, 0).UTC()
	// An authentication in the future is a broken or hostile provider clock.
	// Accepting it would make any freshness requirement trivially satisfiable.
	if value.After(c.clock().Add(c.leeway)) {
		return nil, ErrAuthTime
	}
	return &value, nil
}

// stringClaim distinguishes an absent claim from one present with a non-string
// value, which the Claims.String accessor reports identically.
func stringClaim(claims jwt.Claims, name string) (string, error) {
	if _, present := claims.Value(name); !present {
		return "", nil
	}
	value, ok := claims.String(name)
	if !ok {
		return "", ErrIDToken
	}
	return value, nil
}

func validateAuthorizedParty(claims jwt.Claims, clientID string) error {
	_, present := claims.Value("azp")
	azp, validString := claims.String("azp")
	if present && (!validString || azp == "" || azp != clientID) {
		return ErrIDToken
	}
	if len(claims.Audience) > 1 && (!present || azp != clientID) {
		return ErrIDToken
	}
	return nil
}

// EndSessionURL builds an RP-initiated logout request for the discovered end
// session endpoint. It returns an empty string when the provider advertises
// none, which lets a caller fall back to ending its own session only.
//
// The specification recommends id_token_hint and some providers require it;
// client_id is always sent so a provider that accepts either still identifies
// the relying party.
func (c *Client) EndSessionURL(options EndSessionOptions) (string, error) {
	if c == nil {
		return "", ErrInvalidOptions
	}
	endpoint := c.provider.EndSessionEndpoint()
	if endpoint == "" {
		return "", nil
	}
	parsed, err := urlpkg.Parse(endpoint)
	if err != nil {
		return "", ErrDiscovery
	}
	query := parsed.Query()
	query.Set("client_id", c.clientID)
	if options.IDToken != "" {
		if len(options.IDToken) > c.maxTokenBytes || !validCompactToken(options.IDToken) {
			return "", ErrIDToken
		}
		query.Set("id_token_hint", options.IDToken)
	}
	if options.PostLogoutRedirectURI != "" {
		redirect, err := authn.ParseEndpoint(options.PostLogoutRedirectURI, c.allowLoopbackHTTP)
		if err != nil || redirect.Fragment != "" {
			return "", ErrInvalidOptions
		}
		query.Set("post_logout_redirect_uri", redirect.String())
	}
	if options.State != "" {
		if len(options.State) > maxEndSessionStateBytes || !validStateValue(options.State) {
			return "", ErrInvalidOptions
		}
		query.Set("state", options.State)
	}
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

// validCompactToken rejects anything that cannot be a compact JWS, so a caller
// cannot smuggle whitespace or control bytes into a query parameter.
func validCompactToken(value string) bool {
	for index := 0; index < len(value); index++ {
		character := value[index]
		switch {
		case character >= 'A' && character <= 'Z':
		case character >= 'a' && character <= 'z':
		case character >= '0' && character <= '9':
		case character == '-' || character == '_' || character == '.':
		default:
			return false
		}
	}
	return strings.Count(value, ".") == 2
}

func validStateValue(value string) bool {
	for index := 0; index < len(value); index++ {
		if value[index] <= 0x20 || value[index] >= 0x7f {
			return false
		}
	}
	return true
}

// UserInfo fetches the optional UserInfo endpoint with a bearer token.
func (c *Client) UserInfo(ctx context.Context, accessToken string) (map[string]json.RawMessage, error) {
	return c.userInfo(ctx, accessToken, "")
}

// UserInfoWithSubject fetches UserInfo and requires its sub claim to match the
// subject from a previously verified ID Token.
func (c *Client) UserInfoWithSubject(ctx context.Context, accessToken, expectedSubject string) (map[string]json.RawMessage, error) {
	if expectedSubject == "" {
		return nil, ErrUserInfo
	}
	return c.userInfo(ctx, accessToken, expectedSubject)
}

func (c *Client) userInfo(ctx context.Context, accessToken, expectedSubject string) (map[string]json.RawMessage, error) {
	if c == nil || ctx == nil || !validBearerToken(accessToken) || len(accessToken) > 16384 || c.provider.userInfoEndpoint == "" {
		return nil, ErrUserInfo
	}
	requestCtx := ctx
	var cancel context.CancelFunc
	if c.provider.options.requestTimeout > 0 {
		requestCtx, cancel = context.WithTimeout(ctx, c.provider.options.requestTimeout)
		defer cancel()
	}
	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, c.provider.userInfoEndpoint, nil)
	if err != nil {
		return nil, ErrHTTP
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	response, err := c.provider.options.httpClient.Do(req)
	if err != nil {
		return nil, ErrHTTP
	}
	defer response.Body.Close()
	body, err := authn.ReadBounded(response.Body, int64(c.provider.options.maxResponseBytes))
	if err != nil {
		if errors.Is(err, authn.ErrLimitExceeded) {
			return nil, ErrLimitExceeded
		}
		return nil, ErrHTTP
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, ErrUserInfo
	}
	if err := authn.ValidateJSON(body, authn.JSONOptions{MaxBytes: c.provider.options.maxResponseBytes, MaxDepth: 8, MaxMembers: 128}); err != nil {
		return nil, ErrUserInfo
	}
	var result map[string]json.RawMessage
	if json.Unmarshal(body, &result) != nil || result == nil {
		return nil, ErrUserInfo
	}
	var subject string
	if rawSubject, ok := result["sub"]; !ok || json.Unmarshal(rawSubject, &subject) != nil || subject == "" {
		return nil, ErrUserInfo
	}
	if expectedSubject != "" && !authn.EqualSecret(expectedSubject, subject) {
		return nil, ErrUserInfo
	}
	return result, nil
}

// validBearerToken rejects control and whitespace bytes before an access token
// is copied into an Authorization header. OAuth bearer tokens are opaque, but
// HTTP header values cannot safely carry these bytes.
func validBearerToken(value string) bool {
	if value == "" {
		return false
	}
	for index := 0; index < len(value); index++ {
		if value[index] < 0x21 || value[index] > 0x7e {
			return false
		}
	}
	return true
}
