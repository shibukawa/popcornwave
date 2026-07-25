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
	ErrUserInfo       = errors.New("oidc: invalid UserInfo response")
	ErrLimitExceeded  = errors.New("oidc: response limit exceeded")
	ErrHTTP           = errors.New("oidc: HTTP request failed")
)

const (
	defaultMaxResponseBytes = 64 << 10
	maxMaxResponseBytes     = 4 << 20
	maxJWKSBytes            = 16 << 20
	maxJWKSKeys             = 4096
	defaultRequestTimeout   = 30 * time.Second
	defaultJWKSCacheTTL     = 5 * time.Minute
	defaultJWKSStaleTTL     = 15 * time.Minute
	maxJWKSCacheTTL         = time.Hour
	maxJWKSStaleTTL         = 24 * time.Hour
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

type Client struct {
	provider          *Provider
	clientID          string
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
}

type Callback = oauth.Callback
type TokenSet = oauth.TokenSet

type IDToken struct {
	Claims jwt.Claims
	Nonce  string
}

type discoveryDocument struct {
	Issuer                string `json:"issuer"`
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	JWKSURI               string `json:"jwks_uri"`
	UserInfoEndpoint      string `json:"userinfo_endpoint"`
}

type userInfoResult map[string]json.RawMessage
