package oauth

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/shibukawa/popcornwave/authstate"
)

var (
	ErrInvalidConfig   = errors.New("oauth: invalid configuration")
	ErrInvalidOptions  = errors.New("oauth: invalid options")
	ErrInvalidCallback = errors.New("oauth: invalid callback")
	ErrState           = errors.New("oauth: state mismatch")
	ErrAuthorization   = errors.New("oauth: authorization failed")
	ErrToken           = errors.New("oauth: token endpoint rejected request")
	ErrMalformed       = errors.New("oauth: malformed response")
	ErrLimitExceeded   = errors.New("oauth: response limit exceeded")
	ErrHTTP            = errors.New("oauth: HTTP request failed")
	ErrAccessDenied    = errors.New("oauth: device authorization denied")
	ErrExpired         = errors.New("oauth: device authorization expired")
)

const (
	AuthBasic               = "client_secret_basic"
	AuthPost                = "client_secret_post"
	AuthNone                = "none"
	defaultStateTTL         = 5 * time.Minute
	maxStateTTL             = 30 * time.Minute
	defaultMaxResponseBytes = 64 << 10
	maxMaxResponseBytes     = 4 << 20
	defaultRequestTimeout   = 30 * time.Second
	maxRequestTimeout       = 10 * time.Minute
	maxClientValueBytes     = 4096
	maxTokenValueBytes      = 16 << 10
)

const DeviceGrantType = "urn:ietf:params:oauth:grant-type:device_code"

// Config describes a registered OAuth client. Endpoint URLs are validated at
// construction and must be HTTPS, except explicitly enabled loopback HTTP.
type Config struct {
	AuthorizationEndpoint string
	TokenEndpoint         string
	ClientID              string
	ClientSecret          string
	RedirectURI           string
	AuthMethod            string
	AllowLoopbackHTTP     bool
	// EndpointValidator may apply caller-specific host/IP trust policy to
	// both configured endpoints. It receives a copy for inspection; mutations
	// are ignored. Its errors are normalized to ErrInvalidConfig.
	EndpointValidator func(*url.URL) error
}

// Options controls state lifetime, I/O bounds, and dependencies.
type Options struct {
	Random           io.Reader
	Clock            func() time.Time
	StateTTL         time.Duration
	MaxResponseBytes int
	// RequestTimeout defaults to 30 seconds and is capped at 10 minutes.
	RequestTimeout time.Duration
	HTTPClient     *http.Client
	StateStore     authstate.Store[Transaction]
	// TransactionValidator is called after callback state correlation and
	// state/PKCE/expiry validation, before a token exchange. It must not log or
	// retain transaction secrets.
	TransactionValidator func(Transaction) error
}

// Transaction is the state kept between authorization and callback. Callers
// must treat it as immutable and must not log it.
type Transaction struct {
	State       string
	Verifier    string
	Nonce       string
	RedirectURI string
	ExpiresAt   time.Time
}

// BeginOptions controls the authorization request.
type BeginOptions struct {
	Scopes []string
	Params map[string]string
	// Nonce is an optional protocol-specific correlation value. OAuth itself
	// does not validate it; OIDC uses it and validates it after token exchange.
	Nonce string
}

// Callback contains values returned to the redirect URI.
type Callback struct {
	Code             string
	State            string
	Error            string
	ErrorDescription string
	ErrorURI         string
}

// TokenSet is a bounded token response. Raw contains only non-standard
// members and must not be used to bypass standard field validation.
type TokenSet struct {
	AccessToken  string
	TokenType    string
	RefreshToken string
	Scope        string
	ExpiresIn    *int64
	Raw          map[string]json.RawMessage
}

// DeviceConfig describes an RFC 8628 client. Public clients use AuthNone and
// carry no ClientSecret; confidential clients use AuthBasic or AuthPost.
type DeviceConfig struct {
	DeviceAuthorizationEndpoint string
	TokenEndpoint               string
	ClientID                    string
	ClientSecret                string
	AuthMethod                  string
	AllowLoopbackHTTP           bool
	EndpointValidator           func(*url.URL) error
}

// DeviceOptions controls bounded I/O and polling. Wait is primarily a test
// seam; nil uses a context-aware timer.
type DeviceOptions struct {
	HTTPClient       *http.Client
	Clock            func() time.Time
	Wait             func(context.Context, time.Duration) error
	MaxResponseBytes int
	RequestTimeout   time.Duration
}

type DeviceBeginOptions struct {
	Scopes []string
}

// DeviceAuthorization contains both user-facing instructions and the opaque
// device credential. Do not log or display DeviceCode.
type DeviceAuthorization struct {
	deviceCode              string
	UserCode                string
	VerificationURI         string
	VerificationURIComplete string
	ExpiresIn               int64
	Interval                int64
	ExpiresAt               time.Time
}

// DeviceClient is safe for concurrent use. A DeviceAuthorization value must
// still be polled by only one goroutine.
type DeviceClient struct {
	config         DeviceConfig
	clock          func() time.Time
	wait           func(context.Context, time.Duration) error
	maxResponse    int
	requestTimeout time.Duration
	httpClient     *http.Client
}

// DeviceError reports a safe OAuth error code without retaining a response
// description that might contain provider or request details.
type DeviceError struct {
	Code       string
	StatusCode int
}

func (e *DeviceError) Error() string { return ErrToken.Error() }
func (e *DeviceError) Unwrap() error { return ErrToken }

// Client is safe for concurrent use when its StateStore implementation is
// safe for concurrent use.
type Client struct {
	config               Config
	clock                func() time.Time
	stateTTL             time.Duration
	maxResponse          int
	requestTimeout       time.Duration
	random               io.Reader
	httpClient           *http.Client
	store                authstate.Store[Transaction]
	transactionValidator func(Transaction) error
}

