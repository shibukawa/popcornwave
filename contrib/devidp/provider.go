package devidp

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/shibukawa/popcornwave/contrib/internal/authn"
)

// Bounds on in-memory state. Every value is small because the provider serves
// one developer or one test at a time.
const (
	maxPendingAuthorizations = 256
	maxIssuedCodes           = 256
	maxAccessTokens          = 512
	secretBytes              = 32

	// environmentVariable is the framework runtime environment selector. It is
	// read directly so this package depends on nothing outside contrib.
	environmentVariable = "APP_ENV"
)

// ErrClosed reports use of a provider after Close.
var ErrClosed = errors.New("devidp: provider is closed")

// Options tunes provider behavior. The zero value is production-shaped for a
// development tool: real clock, crypto/rand, no logging, manual login.
type Options struct {
	Now    func() time.Time
	Random io.Reader
	Logf   func(format string, args ...any)
	// LoginUser pre-selects a subject so authorization skips the login screen.
	LoginUser string
}

// Provider serves the development OpenID Provider endpoints.
type Provider struct {
	issuer   string
	tokenTTL time.Duration
	codeTTL  time.Duration

	key *rsa.PrivateKey
	kid string

	now    func() time.Time
	random io.Reader
	logf   func(format string, args ...any)

	handler http.Handler

	mu        sync.Mutex
	closed    bool
	users     []User
	bySubject map[string]*User
	// extraScopes are the configured idp.valid_scopes, kept so a roster reload
	// can recompute the advertised scope set.
	extraScopes []string
	scopes      []string
	clients     map[string]*Client
	loginUser   string
	pending     map[string]*pendingAuthorization
	codes       map[string]*issuedCode
	tokens      map[string]*accessToken
}

type pendingAuthorization struct {
	clientID    string
	redirectURI string
	state       string
	nonce       string
	challenge   string
	scopes      []string
	csrf        string
	expiresAt   time.Time
}

type issuedCode struct {
	clientID    string
	redirectURI string
	challenge   string
	nonce       string
	subject     string
	scopes      []string
	issuedAt    time.Time
	expiresAt   time.Time
}

type accessToken struct {
	clientID  string
	subject   string
	scopes    []string
	expiresAt time.Time
}

// New builds a provider from a validated configuration.
func New(config Config, options Options) (*Provider, error) {
	if err := config.validate(); err != nil {
		return nil, err
	}
	if config.Issuer == "" {
		return nil, fmt.Errorf("%w: an issuer is required; use Start to derive one from a listener", ErrConfig)
	}
	if _, err := parseIssuer(config.Issuer); err != nil {
		return nil, err
	}
	provider := &Provider{
		issuer:      strings.TrimSuffix(config.Issuer, "/"),
		tokenTTL:    config.TokenTTL,
		codeTTL:     config.CodeTTL,
		key:         config.SigningKey,
		now:         options.Now,
		random:      options.Random,
		logf:        options.Logf,
		users:       append([]User(nil), config.Users...),
		bySubject:   map[string]*User{},
		extraScopes: append([]string(nil), config.ValidScopes...),
		scopes:      supportedScopes(config),
		clients:     map[string]*Client{},
		pending:     map[string]*pendingAuthorization{},
		codes:       map[string]*issuedCode{},
		tokens:      map[string]*accessToken{},
	}
	if provider.now == nil {
		provider.now = time.Now
	}
	if provider.random == nil {
		provider.random = rand.Reader
	}
	if provider.logf == nil {
		provider.logf = func(string, ...any) {}
	}
	if provider.key == nil {
		key, err := rsa.GenerateKey(provider.random, 2048)
		if err != nil {
			return nil, fmt.Errorf("devidp: generate signing key: %w", err)
		}
		provider.key = key
	}
	provider.kid = keyID(&provider.key.PublicKey)
	for index := range provider.users {
		user := &provider.users[index]
		provider.bySubject[user.Subject] = user
	}
	for index := range config.Clients {
		client := config.Clients[index]
		provider.clients[client.ID] = &client
	}
	if options.LoginUser != "" {
		if err := provider.SetLoginUser(options.LoginUser); err != nil {
			return nil, err
		}
	}
	provider.handler = provider.routes()
	return provider, nil
}

// Issuer returns the base URL every discovery URL is built from.
func (p *Provider) Issuer() string { return p.issuer }

// Handler serves the provider endpoints under the issuer base path.
func (p *Provider) Handler() http.Handler { return p.handler }

// Users returns the roster in login-screen order.
func (p *Provider) Users() []User {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]User(nil), p.users...)
}

// ClientSpec describes a client the running tool registers for itself.
type ClientSpec struct {
	// ID is generated when empty.
	ID string
	// RedirectURIs are matched exactly unless LoopbackRedirects is set.
	RedirectURIs []string
	// LoopbackRedirects accepts any loopback callback, which lets a tool
	// register before it knows the application port or callback path.
	LoopbackRedirects bool
	ValidScopes       []string
}

// Credentials are the generated secrets for a registered client.
type Credentials struct {
	ID     string
	Secret string
}

// RegisterClient adds an ephemeral client and returns its generated credentials.
// The secret exists only in memory and is not recoverable after Close.
func (p *Provider) RegisterClient(spec ClientSpec) (Credentials, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return Credentials{}, ErrClosed
	}
	id := spec.ID
	if id == "" {
		generated, err := authn.GenerateSecret(p.random, 8)
		if err != nil {
			return Credentials{}, fmt.Errorf("devidp: generate client id: %w", err)
		}
		id = "pw-dev-" + generated
	}
	if _, exists := p.clients[id]; exists {
		return Credentials{}, fmt.Errorf("%w: duplicate client %q", ErrConfig, id)
	}
	secret, err := authn.GenerateSecret(p.random, secretBytes)
	if err != nil {
		return Credentials{}, fmt.Errorf("devidp: generate client secret: %w", err)
	}
	client := &Client{
		ID:                id,
		Secret:            secret,
		RedirectURIs:      append([]string(nil), spec.RedirectURIs...),
		ValidScopes:       append([]string(nil), spec.ValidScopes...),
		LoopbackRedirects: spec.LoopbackRedirects,
	}
	if err := validateClient(client); err != nil {
		return Credentials{}, err
	}
	p.clients[id] = client
	return Credentials{ID: id, Secret: secret}, nil
}

// Reload replaces the roster and scope set in place. Registered clients, the
// issuer, and the signing key survive, so an edited roster reaches the running
// application without restarting it or reissuing its injected credentials.
//
// A pre-selected login user that left the roster is cleared rather than kept
// as a dangling subject.
func (p *Provider) Reload(config Config) error {
	if err := config.validate(); err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return ErrClosed
	}
	p.users = append([]User(nil), config.Users...)
	p.bySubject = map[string]*User{}
	for index := range p.users {
		p.bySubject[p.users[index].Subject] = &p.users[index]
	}
	p.extraScopes = append([]string(nil), config.ValidScopes...)
	p.scopes = supportedScopes(Config{ValidScopes: p.extraScopes, Users: p.users})
	if p.loginUser != "" {
		if _, ok := p.bySubject[p.loginUser]; !ok {
			p.logf("devidp: pre-selected user %q left the roster; the login screen is back", p.loginUser)
			p.loginUser = ""
		}
	}
	return nil
}

// SetLoginUser pre-selects a subject so the login screen is skipped. An empty
// subject restores manual selection.
func (p *Provider) SetLoginUser(subject string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return ErrClosed
	}
	if subject == "" {
		p.loginUser = ""
		return nil
	}
	if _, ok := p.bySubject[subject]; !ok {
		return fmt.Errorf("%w: %q", ErrUnknownUser, subject)
	}
	p.loginUser = subject
	return nil
}

// LoginUser reports the pre-selected subject, if any.
func (p *Provider) LoginUser() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.loginUser
}

// Close destroys signing key material and every pending authorization.
func (p *Provider) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return nil
	}
	p.closed = true
	p.pending = map[string]*pendingAuthorization{}
	p.codes = map[string]*issuedCode{}
	p.tokens = map[string]*accessToken{}
	p.clients = map[string]*Client{}
	p.key = nil
	return nil
}

// Server is a provider bound to a listener.
type Server struct {
	*Provider
	listener net.Listener
	server   *http.Server
	done     chan struct{}
}

// Start listens on addr, derives the issuer from the resolved address when the
// configuration leaves it empty, and serves the provider.
//
// addr must be a loopback address unless the configuration carries an explicit
// issuer, because the provider performs no authentication.
func Start(ctx context.Context, addr string, config Config, options Options) (*Server, error) {
	if addr == "" {
		addr = "127.0.0.1:0"
	}
	if err := requireDevelopmentEnvironment(); err != nil {
		return nil, err
	}
	if err := requireLoopbackAddr(addr); err != nil {
		return nil, err
	}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("devidp: listen %s: %w", addr, err)
	}
	if config.Issuer == "" {
		config.Issuer = "http://" + listener.Addr().String()
	}
	provider, err := New(config, options)
	if err != nil {
		listener.Close()
		return nil, err
	}
	server := &Server{
		Provider: provider,
		listener: listener,
		server:   &http.Server{Handler: provider.Handler(), ReadHeaderTimeout: 5 * time.Second},
		done:     make(chan struct{}),
	}
	go func() {
		defer close(server.done)
		_ = server.server.Serve(listener)
	}()
	if ctx != nil {
		context.AfterFunc(ctx, func() { _ = server.Close() })
	}
	provider.logf("devidp: development identity provider on %s; no password is checked", provider.Issuer())
	return server, nil
}

// Close stops the listener and destroys provider state.
func (s *Server) Close() error {
	shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := s.server.Shutdown(shutdown)
	<-s.done
	return errors.Join(err, s.Provider.Close())
}

// supportedScopes merges the standard scopes with every configured extra scope.
func supportedScopes(config Config) []string {
	scopes := append([]string(nil), standardScopes...)
	add := func(candidates []string) {
		for _, scope := range candidates {
			if !contains(scopes, scope) {
				scopes = append(scopes, scope)
			}
		}
	}
	add(config.ValidScopes)
	for _, user := range config.Users {
		add(user.ExtraScopes)
	}
	return scopes
}

// Endpoint returns an absolute provider URL, for example "/authorize".
func (p *Provider) Endpoint(path string) string { return p.endpoint(path) }

// grantableScopes intersects the request with the provider, client, and user.
func (p *Provider) grantableScopes(requested []string, client *Client, user *User) []string {
	granted := make([]string, 0, len(requested))
	for _, scope := range requested {
		if !contains(p.scopes, scope) {
			continue
		}
		if len(client.ValidScopes) != 0 && !contains(client.ValidScopes, scope) {
			continue
		}
		if !contains(standardScopes, scope) && !contains(user.ExtraScopes, scope) {
			continue
		}
		if !contains(granted, scope) {
			granted = append(granted, scope)
		}
	}
	return granted
}

func parseIssuer(issuer string) (*url.URL, error) {
	parsed, err := url.Parse(issuer)
	if err != nil || !parsed.IsAbs() || parsed.Host == "" {
		return nil, fmt.Errorf("%w: issuer %q must be an absolute URL", ErrConfig, issuer)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("%w: issuer %q must use http or https", ErrConfig, issuer)
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, fmt.Errorf("%w: issuer %q must not carry a query or fragment", ErrConfig, issuer)
	}
	return parsed, nil
}

func validateRedirectURI(redirect string) error {
	parsed, err := url.Parse(redirect)
	if err != nil || !parsed.IsAbs() || parsed.Host == "" {
		return errors.New("must be an absolute URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return errors.New("must use http or https")
	}
	if parsed.Fragment != "" {
		return errors.New("must not carry a fragment")
	}
	return nil
}

// isLoopbackURL reports whether a redirect target stays on this machine.
// RFC 6761 reserves localhost and every name under it for the loopback
// interface, so a development host such as app.localhost counts too.
func isLoopbackURL(parsed *url.URL) bool {
	host := parsed.Hostname()
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	address := net.ParseIP(host)
	return address != nil && address.IsLoopback()
}

// requireDevelopmentEnvironment refuses to serve in a deployed environment.
// The provider authenticates nobody, so a production process that reaches this
// code has a defect, not a configuration problem.
func requireDevelopmentEnvironment() error {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(environmentVariable))) {
	case "prod", "production":
		return fmt.Errorf("%w: %s selects a production environment; the development identity provider authenticates nobody",
			ErrConfig, environmentVariable)
	}
	return nil
}

func requireLoopbackAddr(addr string) error {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("%w: listen address %q: %w", ErrConfig, addr, err)
	}
	if host == "" {
		return fmt.Errorf("%w: listen address %q must name a loopback host", ErrConfig, addr)
	}
	if host == "localhost" {
		return nil
	}
	address := net.ParseIP(host)
	if address == nil || !address.IsLoopback() {
		return fmt.Errorf("%w: listen address %q is not loopback; the provider authenticates nobody", ErrConfig, addr)
	}
	return nil
}

// keyID derives a stable kid from the public key so a restart is visible to a
// relying party that cached the previous key set.
func keyID(public *rsa.PublicKey) string {
	digest := sha256.Sum256(public.N.Bytes())
	return base64.RawURLEncoding.EncodeToString(digest[:12])
}
