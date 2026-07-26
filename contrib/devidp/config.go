package devidp

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/shibukawa/tinybind-go/minitoml"
)

// Defaults applied when the roster leaves a lifetime unset.
const (
	DefaultTokenTTL = time.Hour
	DefaultCodeTTL  = time.Minute

	maxTokenTTL = 12 * time.Hour
	maxCodeTTL  = 10 * time.Minute
)

var (
	// ErrConfig reports an unusable roster file or Config value.
	ErrConfig = errors.New("devidp: invalid configuration")
	// ErrUnknownUser reports a subject that is absent from the roster.
	ErrUnknownUser = errors.New("devidp: unknown user")
)

// standardScopes are always granted to a client that asks for them.
var standardScopes = []string{"openid", "profile", "email"}

// reservedClaims are produced by the provider and cannot be set by a roster.
var reservedClaims = []string{
	"iss", "sub", "aud", "exp", "iat", "nbf", "auth_time", "nonce", "azp", "at_hash",
}

// User is one selectable development identity.
type User struct {
	Key         string
	Subject     string
	DisplayName string
	ExtraScopes []string
	Claims      map[string]any
}

// Client is a relying party allowed to obtain tokens.
type Client struct {
	ID           string
	Secret       string
	RedirectURIs []string
	ValidScopes  []string
	// LoopbackRedirects accepts any loopback redirect URI instead of matching
	// RedirectURIs exactly. Only a client registered by the running tool may
	// set it; see RegisterClient.
	LoopbackRedirects bool
}

// Config is the resolved provider configuration.
type Config struct {
	// Issuer is the absolute base URL. Start fills it from the listener when empty.
	Issuer      string
	ValidScopes []string
	TokenTTL    time.Duration
	CodeTTL     time.Duration
	// SigningKey signs ID Tokens. New generates an ephemeral key when nil.
	SigningKey *rsa.PrivateKey
	Clients    []Client
	Users      []User

	// signingKeyPath is parse-time state resolved into SigningKey.
	signingKeyPath string
}

// LoadConfig reads a roster file. Paths inside it resolve from its directory.
func LoadConfig(path string) (Config, error) {
	source, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("devidp: read %s: %w", path, err)
	}
	config, err := ParseConfig(source, filepath.Dir(path))
	if err != nil {
		return Config{}, fmt.Errorf("%s: %w", path, err)
	}
	return config, nil
}

// ParseConfig parses roster bytes, resolving relative paths from base.
func ParseConfig(source []byte, base string) (Config, error) {
	document, err := minitoml.Parse(source)
	if err != nil {
		return Config{}, fmt.Errorf("%w: %w", ErrConfig, err)
	}
	config := Config{}
	users := map[string]*User{}
	clients := map[string]*Client{}
	// minitoml flattens the document, so a table header with no fields leaves
	// no trace. Register entities from the headers first; otherwise a roster
	// entry that only names a user would silently disappear.
	if err := registerTables(source, users, clients); err != nil {
		return Config{}, err
	}
	for _, key := range document.Keys() {
		value, _ := document.Get(key)
		switch {
		case strings.HasPrefix(key, "idp."):
			err = assignIDP(&config, strings.TrimPrefix(key, "idp."), value)
		case strings.HasPrefix(key, "users."):
			err = assignUser(users, strings.TrimPrefix(key, "users."), value)
		case strings.HasPrefix(key, "clients."):
			err = assignClient(clients, strings.TrimPrefix(key, "clients."), value)
		default:
			err = fmt.Errorf("%w: unknown key %s", ErrConfig, key)
		}
		if err != nil {
			return Config{}, err
		}
	}
	for _, name := range sortedKeys(users) {
		config.Users = append(config.Users, *users[name])
	}
	for _, name := range sortedKeys(clients) {
		config.Clients = append(config.Clients, *clients[name])
	}
	if config.signingKeyPath != "" {
		path := config.signingKeyPath
		if !filepath.IsAbs(path) {
			path = filepath.Join(base, path)
		}
		key, err := loadSigningKey(path)
		if err != nil {
			return Config{}, err
		}
		config.SigningKey = key
	}
	if err := config.validate(); err != nil {
		return Config{}, err
	}
	return config, nil
}

// registerTables walks the table headers of an already-parsed document.
// minitoml rejects arrays of tables and multi-line strings, so a line starting
// with '[' is always a table header.
func registerTables(source []byte, users map[string]*User, clients map[string]*Client) error {
	for _, line := range strings.Split(string(source), "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "[") {
			continue
		}
		end := strings.IndexByte(trimmed, ']')
		if end < 0 {
			continue
		}
		path := strings.TrimSpace(trimmed[1:end])
		segments := strings.Split(path, ".")
		switch {
		case path == "idp":
		case segments[0] == "users" && len(segments) == 2:
			if users[segments[1]] == nil {
				users[segments[1]] = &User{Key: segments[1], Claims: map[string]any{}}
			}
		case segments[0] == "users" && len(segments) == 3 && segments[2] == "claims":
			if users[segments[1]] == nil {
				users[segments[1]] = &User{Key: segments[1], Claims: map[string]any{}}
			}
		case segments[0] == "users" && len(segments) > 3 && segments[2] == "claims":
			return fmt.Errorf("%w: users.%s: nested claim tables are not supported", ErrConfig, path[len("users."):])
		case segments[0] == "clients" && len(segments) == 2:
			if clients[segments[1]] == nil {
				clients[segments[1]] = &Client{ID: segments[1]}
			}
		default:
			return fmt.Errorf("%w: unknown table [%s]", ErrConfig, path)
		}
	}
	return nil
}

func assignIDP(config *Config, field string, value minitoml.Value) error {
	var err error
	switch field {
	case "issuer":
		config.Issuer, err = value.AsString()
	case "valid_scopes":
		config.ValidScopes, err = value.AsStringSlice()
	case "token_ttl":
		config.TokenTTL, err = duration(value)
	case "code_ttl":
		config.CodeTTL, err = duration(value)
	case "signing_key":
		config.signingKeyPath, err = value.AsString()
	default:
		return fmt.Errorf("%w: unknown key idp.%s", ErrConfig, field)
	}
	if err != nil {
		return fmt.Errorf("%w: idp.%s: %w", ErrConfig, field, err)
	}
	return nil
}

func assignUser(users map[string]*User, path string, value minitoml.Value) error {
	name, field, ok := strings.Cut(path, ".")
	if !ok || name == "" {
		return fmt.Errorf("%w: unknown key users.%s", ErrConfig, path)
	}
	user := users[name]
	if user == nil {
		user = &User{Key: name, Claims: map[string]any{}}
		users[name] = user
	}
	if claim, ok := strings.CutPrefix(field, "claims."); ok {
		if claim == "" || strings.Contains(claim, ".") {
			return fmt.Errorf("%w: users.%s: nested claim tables are not supported", ErrConfig, path)
		}
		if contains(reservedClaims, claim) {
			return fmt.Errorf("%w: users.%s.claims.%s is a reserved claim", ErrConfig, name, claim)
		}
		user.Claims[claim] = jsonValue(value)
		return nil
	}
	var err error
	switch field {
	case "subject":
		user.Subject, err = value.AsString()
	case "display_name":
		user.DisplayName, err = value.AsString()
	case "extra_scopes":
		user.ExtraScopes, err = value.AsStringSlice()
	default:
		return fmt.Errorf("%w: unknown key users.%s", ErrConfig, path)
	}
	if err != nil {
		return fmt.Errorf("%w: users.%s: %w", ErrConfig, path, err)
	}
	return nil
}

func assignClient(clients map[string]*Client, path string, value minitoml.Value) error {
	id, field, ok := strings.Cut(path, ".")
	if !ok || id == "" {
		return fmt.Errorf("%w: unknown key clients.%s", ErrConfig, path)
	}
	client := clients[id]
	if client == nil {
		client = &Client{ID: id}
		clients[id] = client
	}
	var err error
	switch field {
	case "secret":
		client.Secret, err = value.AsString()
	case "redirect_uris":
		client.RedirectURIs, err = value.AsStringSlice()
	case "valid_scopes":
		client.ValidScopes, err = value.AsStringSlice()
	default:
		return fmt.Errorf("%w: unknown key clients.%s", ErrConfig, path)
	}
	if err != nil {
		return fmt.Errorf("%w: clients.%s: %w", ErrConfig, path, err)
	}
	return nil
}

// validate normalizes defaults and rejects a roster that cannot serve a login.
func (c *Config) validate() error {
	if c.Issuer != "" {
		if _, err := parseIssuer(c.Issuer); err != nil {
			return err
		}
	}
	if c.TokenTTL == 0 {
		c.TokenTTL = DefaultTokenTTL
	}
	if c.CodeTTL == 0 {
		c.CodeTTL = DefaultCodeTTL
	}
	if c.TokenTTL <= 0 || c.TokenTTL > maxTokenTTL {
		return fmt.Errorf("%w: idp.token_ttl must be positive and at most %s", ErrConfig, maxTokenTTL)
	}
	if c.CodeTTL <= 0 || c.CodeTTL > maxCodeTTL {
		return fmt.Errorf("%w: idp.code_ttl must be positive and at most %s", ErrConfig, maxCodeTTL)
	}
	for _, scope := range c.ValidScopes {
		if !validScopeToken(scope) {
			return fmt.Errorf("%w: idp.valid_scopes %q is not a scope token", ErrConfig, scope)
		}
	}
	if len(c.Users) == 0 {
		return fmt.Errorf("%w: at least one user is required", ErrConfig)
	}
	subjects := map[string]string{}
	for index := range c.Users {
		user := &c.Users[index]
		if user.Subject == "" {
			user.Subject = user.Key
		}
		if user.Subject == "" {
			return fmt.Errorf("%w: a user needs a key or an explicit subject", ErrConfig)
		}
		if previous, ok := subjects[user.Subject]; ok {
			return fmt.Errorf("%w: users.%s and users.%s share subject %q", ErrConfig, previous, user.Key, user.Subject)
		}
		subjects[user.Subject] = user.Key
		if user.DisplayName == "" {
			user.DisplayName = user.Key
		}
		for _, scope := range user.ExtraScopes {
			if !validScopeToken(scope) {
				return fmt.Errorf("%w: users.%s.extra_scopes %q is not a scope token", ErrConfig, user.Key, scope)
			}
		}
	}
	ids := map[string]bool{}
	for index := range c.Clients {
		client := &c.Clients[index]
		if err := validateClient(client); err != nil {
			return err
		}
		if ids[client.ID] {
			return fmt.Errorf("%w: duplicate client %q", ErrConfig, client.ID)
		}
		ids[client.ID] = true
	}
	return nil
}

func validateClient(client *Client) error {
	if client.ID == "" {
		return fmt.Errorf("%w: a client needs an id", ErrConfig)
	}
	if client.Secret == "" {
		return fmt.Errorf("%w: clients.%s.secret is required", ErrConfig, client.ID)
	}
	if !client.LoopbackRedirects && len(client.RedirectURIs) == 0 {
		return fmt.Errorf("%w: clients.%s.redirect_uris is required", ErrConfig, client.ID)
	}
	for _, redirect := range client.RedirectURIs {
		if err := validateRedirectURI(redirect); err != nil {
			return fmt.Errorf("%w: clients.%s.redirect_uris %q: %w", ErrConfig, client.ID, redirect, err)
		}
	}
	for _, scope := range client.ValidScopes {
		if !validScopeToken(scope) {
			return fmt.Errorf("%w: clients.%s.valid_scopes %q is not a scope token", ErrConfig, client.ID, scope)
		}
	}
	return nil
}

func loadSigningKey(path string) (*rsa.PrivateKey, error) {
	source, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("devidp: read signing key: %w", err)
	}
	block, _ := pem.Decode(source)
	if block == nil {
		return nil, fmt.Errorf("%w: signing key %s is not PEM", ErrConfig, path)
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("%w: signing key %s is not an RSA private key", ErrConfig, path)
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("%w: signing key %s is not an RSA private key", ErrConfig, path)
	}
	return key, nil
}

func duration(value minitoml.Value) (time.Duration, error) {
	text, err := value.AsString()
	if err != nil {
		return 0, err
	}
	return time.ParseDuration(text)
}

// jsonValue converts a TOML scalar or primitive array into a JSON-encodable value.
func jsonValue(value minitoml.Value) any {
	switch value.Kind {
	case minitoml.KindString:
		return value.Str
	case minitoml.KindBool:
		return value.Bool
	case minitoml.KindInt:
		return value.Int
	case minitoml.KindFloat:
		return value.Float
	case minitoml.KindArray:
		items := make([]any, 0, len(value.Array))
		for _, element := range value.Array {
			items = append(items, jsonValue(element))
		}
		return items
	default:
		return value.String()
	}
}

func sortedKeys[T any](values map[string]T) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func contains(values []string, candidate string) bool {
	for _, value := range values {
		if value == candidate {
			return true
		}
	}
	return false
}

// validScopeToken applies the RFC 6749 scope-token grammar.
func validScopeToken(scope string) bool {
	if scope == "" {
		return false
	}
	for index := 0; index < len(scope); index++ {
		character := scope[index]
		if character <= 0x20 || character == 0x22 || character == 0x5c || character >= 0x7f {
			return false
		}
	}
	return true
}
