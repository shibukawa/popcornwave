// Package auth implements the authentication endpoints the Popcorn Wave
// runtime mounts from the `[auth]` configuration.
//
// Importing it is the whole integration:
//
//	import _ "github.com/shibukawa/popcornwave/auth"
//
// The runtime then serves auth.login_path, auth.callback_path, and
// auth.logout_path itself. An application registers no route, writes no OIDC
// code, and reads the result with pw.CurrentUser.
package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/shibukawa/popcornwave/contrib/authstate/memory"
	"github.com/shibukawa/popcornwave/contrib/oauth"
	"github.com/shibukawa/popcornwave/contrib/oidc"
	"github.com/shibukawa/popcornwave/pw"
)

func init() { pw.RegisterAuthProvider(New) }

// transactionTTL bounds how long a browser may take between the login redirect
// and the callback.
const transactionTTL = 10 * time.Minute

// New builds the endpoints for a resolved configuration. The runtime calls it;
// an application only calls it to mount the endpoints somewhere else.
func New(config pw.AuthConfig, session pw.SessionConfig) (pw.AuthHandlers, error) {
	if !config.UsesOIDC() {
		return pw.AuthHandlers{}, errors.New("popcornwave/auth: only the oidc and oidc_passkey modes are implemented")
	}
	codec, err := newSessionCodec(session)
	if err != nil {
		return pw.AuthHandlers{}, err
	}
	store, err := memory.NewStore[oauth.Transaction](memory.Options{})
	if err != nil {
		return pw.AuthHandlers{}, err
	}
	service := &service{config: config, codec: codec, store: store}
	return pw.AuthHandlers{
		Login:    http.HandlerFunc(service.login),
		Callback: http.HandlerFunc(service.callback),
		Logout:   http.HandlerFunc(service.logout),
		Resolve:  service.resolve,
	}, nil
}

type service struct {
	config pw.AuthConfig
	codec  *sessionCodec
	store  *memory.Store[oauth.Transaction]

	discovery sync.Once
	provider  *oidc.Provider
	discErr   error

	mu      sync.Mutex
	clients map[string]*oidc.Client
}

// resolve puts the identity of an existing session on every request context.
// An unusable or expired cookie is dropped silently: an anonymous request is a
// normal state, not an error.
func (s *service) resolve(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		value := cookieValue(r, sessionCookieName)
		if value == "" {
			next.ServeHTTP(w, r)
			return
		}
		current, err := s.codec.decode(value)
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r.WithContext(pw.WithIdentity(r.Context(), current.identity)))
	})
}

func (s *service) login(w http.ResponseWriter, r *http.Request) {
	client, err := s.client(r)
	if err != nil {
		s.fail(w, r, "start login", err)
		return
	}
	target, key, err := client.BeginAuthorization(r.Context(), oidc.BeginOptions{Scopes: s.scopes()})
	if err != nil {
		s.fail(w, r, "start login", err)
		return
	}
	setCookie(w, r, transactionCookieName, key, time.Now().Add(transactionTTL))
	http.Redirect(w, r, target, http.StatusFound)
}

func (s *service) callback(w http.ResponseWriter, r *http.Request) {
	key := cookieValue(r, transactionCookieName)
	clearCookie(w, r, transactionCookieName)
	if key == "" {
		s.fail(w, r, "complete login", errors.New("the login was not started on this browser"))
		return
	}
	client, err := s.client(r)
	if err != nil {
		s.fail(w, r, "complete login", err)
		return
	}
	query := r.URL.Query()
	tokens, err := client.HandleCallback(r.Context(), key, oidc.Callback{
		Code:             query.Get("code"),
		State:            query.Get("state"),
		Error:            query.Get("error"),
		ErrorDescription: query.Get("error_description"),
	})
	if err != nil {
		s.fail(w, r, "complete login", err)
		return
	}
	established, err := s.sessionOf(r.Context(), client, tokens)
	if err != nil {
		s.fail(w, r, "complete login", err)
		return
	}
	value, expires, err := s.codec.encode(established)
	if err != nil {
		s.fail(w, r, "complete login", err)
		return
	}
	setCookie(w, r, sessionCookieName, value, expires)
	http.Redirect(w, r, s.config.PostLoginRedirect, http.StatusFound)
}

// logout ends the local session and, unless the operator turned it off, the
// provider session as well. Clearing only the local cookie leaves the provider
// signed in, and the next login silently returns the same user: the sign-out
// looks like it did nothing.
func (s *service) logout(w http.ResponseWriter, r *http.Request) {
	// The session cookie is SameSite=Lax, so a cross-site POST never carries
	// it. The origin check covers same-site subdomains as well.
	if !sameOrigin(r) {
		pw.WriteProblem(w, r, pw.Forbidden(errors.New("cross-origin logout")))
		return
	}
	current, decodeErr := s.codec.decode(cookieValue(r, sessionCookieName))
	clearCookie(w, r, sessionCookieName)
	clearCookie(w, r, transactionCookieName)

	if s.config.OIDC.ProviderLogout {
		idToken := ""
		if decodeErr == nil {
			idToken = current.idToken
		}
		if target := s.endSessionURL(r, idToken); target != "" {
			http.Redirect(w, r, target, http.StatusSeeOther)
			return
		}
	}
	http.Redirect(w, r, s.config.PostLogoutRedirect, http.StatusSeeOther)
}

// endSessionURL builds the RP-initiated logout request, or returns an empty
// string when the provider advertises no end session endpoint, discovery is
// unavailable, or the request cannot be built. Every one of those cases falls
// back to the local logout rather than stranding the user.
func (s *service) endSessionURL(r *http.Request, idToken string) string {
	client, err := s.client(r)
	if err != nil {
		pw.Logger(r.Context()).WarnContext(r.Context(), "provider logout unavailable", "error", err)
		return ""
	}
	target, err := client.EndSessionURL(oidc.EndSessionOptions{
		IDToken:               idToken,
		PostLogoutRedirectURI: s.postLogoutRedirectURI(r),
	})
	if err != nil {
		pw.Logger(r.Context()).WarnContext(r.Context(), "provider logout request rejected", "error", err)
		return ""
	}
	return target
}

// postLogoutRedirectURI is where the provider sends the browser back to. It is
// absolute because the provider needs a full URL, and it is derived from this
// origin so it always matches the local path the operator configured.
func (s *service) postLogoutRedirectURI(r *http.Request) string {
	if r.Host == "" {
		return ""
	}
	scheme := "http"
	if isSecureRequest(r) {
		scheme = "https"
	}
	return (&url.URL{Scheme: scheme, Host: r.Host, Path: s.config.PostLogoutRedirect}).String()
}

// sessionOf reads the verified ID Token. HandleCallback already validated its
// signature, issuer, audience, and nonce, so this second parse only extracts
// claims. The raw token is kept for the logout hint.
func (s *service) sessionOf(ctx context.Context, client *oidc.Client, tokens oidc.TokenSet) (session, error) {
	raw, ok := tokens.Raw["id_token"]
	if !ok {
		return session{}, errors.New("the provider returned no ID Token")
	}
	var compact string
	if err := json.Unmarshal(raw, &compact); err != nil {
		return session{}, err
	}
	idToken, err := client.VerifyIDToken(ctx, compact)
	if err != nil {
		return session{}, err
	}
	identity := pw.Identity{
		Subject: idToken.Claims.Subject,
		Issuer:  idToken.Claims.Issuer,
		Claims:  map[string]string{},
	}
	identity.Name, _ = idToken.Claims.String("name")
	identity.Email, _ = idToken.Claims.String("email")
	for _, name := range []string{"preferred_username", "given_name", "family_name", "picture", "role", "groups"} {
		if value, ok := idToken.Claims.String(name); ok && value != "" {
			identity.Claims[name] = value
		}
	}
	return session{identity: identity, idToken: compact}, nil
}

// client returns the OIDC client for the redirect URI this request implies.
// When auth.oidc.redirect_url is empty the URI follows the request origin,
// which is what makes a scaffolded project work on whatever port it starts on.
func (s *service) client(r *http.Request) (*oidc.Client, error) {
	provider, err := s.discover(r.Context())
	if err != nil {
		return nil, err
	}
	redirect, err := s.redirectURI(r)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if client, ok := s.clients[redirect]; ok {
		return client, nil
	}
	client, err := oidc.NewClient(provider, oidc.Config{
		ClientID:          s.config.OIDC.ClientID,
		ClientSecret:      s.config.OIDC.ClientSecret,
		RedirectURI:       redirect,
		AllowLoopbackHTTP: true,
	}, oidc.Options{OAuth: oauth.Options{StateStore: s.store, StateTTL: transactionTTL}})
	if err != nil {
		return nil, err
	}
	if s.clients == nil {
		s.clients = map[string]*oidc.Client{}
	}
	s.clients[redirect] = client
	return client, nil
}

func (s *service) discover(ctx context.Context) (*oidc.Provider, error) {
	s.discovery.Do(func() {
		s.provider, s.discErr = oidc.Discover(ctx, s.config.OIDC.Issuer, oidc.DiscoverOptions{
			AllowLoopbackHTTP: true,
		})
	})
	return s.provider, s.discErr
}

func (s *service) redirectURI(r *http.Request) (string, error) {
	if configured := strings.TrimSpace(s.config.OIDC.RedirectURL); configured != "" {
		return configured, nil
	}
	if r.Host == "" {
		return "", errors.New("auth.oidc.redirect_url is required when the request carries no host")
	}
	scheme := "http"
	if isSecureRequest(r) {
		scheme = "https"
	}
	return (&url.URL{Scheme: scheme, Host: r.Host, Path: s.config.CallbackPath}).String(), nil
}

func (s *service) scopes() []string {
	if len(s.config.OIDC.Scopes) > 0 {
		return s.config.OIDC.Scopes
	}
	return []string{"openid", "profile", "email"}
}

// fail logs the cause and returns a generic response, so a provider error
// never reaches the browser as protocol detail.
func (s *service) fail(w http.ResponseWriter, r *http.Request, action string, err error) {
	pw.Logger(r.Context()).ErrorContext(r.Context(), "authentication failed", "action", action, "error", err)
	pw.WriteProblem(w, r, pw.Unauthorized(errors.New("could not "+action)))
}

// sameOrigin compares the declared origin with the request host. A request
// without Origin or Referer is accepted because non-browser clients omit both
// and the cookie policy already blocks the cross-site browser case.
func sameOrigin(r *http.Request) bool {
	declared := r.Header.Get("Origin")
	if declared == "" || declared == "null" {
		declared = r.Header.Get("Referer")
	}
	if declared == "" {
		return true
	}
	parsed, err := url.Parse(declared)
	if err != nil || parsed.Host == "" {
		return false
	}
	return strings.EqualFold(parsed.Host, r.Host)
}
