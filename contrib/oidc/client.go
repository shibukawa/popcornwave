package oidc

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/shibukawa/petitweb-go/contrib/internal/authn"
	"github.com/shibukawa/petitweb-go/contrib/jwt"
	"github.com/shibukawa/petitweb-go/contrib/oauth"
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
	return &Client{provider: provider, clientID: config.ClientID, oauth: client, random: random, clock: clock, allowedAlgorithms: algorithms, leeway: options.Leeway, maxTokenBytes: maxTokenBytes, maxSegmentBytes: maxSegmentBytes}, nil
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
	urlValue, key, err := c.oauth.BeginAuthorization(ctx, oauth.BeginOptions{Scopes: scopes, Params: options.Params, Nonce: nonce})
	return urlValue, key, err
}

// HandleCallback exchanges the code and verifies the returned ID Token,
// including its nonce against the atomically consumed transaction.
func (c *Client) HandleCallback(ctx context.Context, key string, callback Callback) (TokenSet, error) {
	set, transaction, err := c.oauth.HandleCallbackWithTransaction(ctx, key, callback)
	if err != nil {
		return TokenSet{}, err
	}
	if !strings.EqualFold(set.TokenType, "Bearer") {
		return TokenSet{}, ErrIDToken
	}
	nonce, nonceErr := authn.DecodeBase64URL(transaction.Nonce, 256, 64)
	if nonceErr != nil || len(nonce) < 16 {
		return TokenSet{}, ErrNonce
	}
	raw, ok := set.Raw["id_token"]
	if !ok {
		return TokenSet{}, ErrIDToken
	}
	var idToken string
	if json.Unmarshal(raw, &idToken) != nil || idToken == "" {
		return TokenSet{}, ErrIDToken
	}
	if _, err := c.verifyIDToken(ctx, idToken, transaction.Nonce); err != nil {
		return TokenSet{}, err
	}
	return set, nil
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
	return IDToken{Claims: claims, Nonce: nonce}, nil
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
