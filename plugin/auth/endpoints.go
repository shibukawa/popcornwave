package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/shibukawa/popcornwave/contrib/oauth"
	"github.com/shibukawa/popcornwave/contrib/oidc"
	"github.com/shibukawa/popcornwave/pw"
)

const (
	// transactionCookieSuffix names the short-lived cookie holding the opaque
	// key of the pending OAuth transaction. The transaction itself, including
	// its state, nonce, and PKCE verifier, never reaches the browser.
	transactionCookieSuffix = "_txn"
	transactionCookieMaxAge = 600
	// returnPathSeparator splits the transaction key from the local path the
	// completed login returns to. The key is base64url and cannot contain it.
	returnPathSeparator = ":"
)

// endpoints owns the login, callback, and logout paths and passes every other
// request through.
func (rt *runtime) endpoints(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path, ok := canonicalPath(r)
		if !ok {
			pw.WriteProblem(w, r, pw.BadRequest())
			return
		}
		// A ceremony path is claimed before the OIDC paths so that a mode
		// without a provider never falls through to a login it cannot serve.
		if suffix, mounted := rt.passkeyPaths[path]; mounted {
			rt.handlePasskey(w, r, suffix)
			return
		}
		switch {
		case rt.config.usesOIDC() && path == rt.config.LoginPath:
			rt.handleLogin(w, r)
		case rt.config.usesOIDC() && path == rt.config.CallbackPath:
			rt.handleCallback(w, r)
		case path == rt.config.LogoutPath:
			rt.handleLogout(w, r)
		default:
			next.ServeHTTP(w, r)
		}
	})
}

func (rt *runtime) handleLogin(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet, http.MethodHead) {
		return
	}
	returnPath := localReturnPath(r.URL.Query().Get("next"))
	if pw.Authenticated(r.Context()) {
		rt.redirect(w, r, rt.landingPath(returnPath))
		return
	}
	client, err := rt.oidcClient(r.Context())
	if err != nil {
		pw.Logger(r.Context()).Log(r.Context(), pw.LevelError, "oidc discovery failed", pw.Err(err))
		pw.WriteProblem(w, r, pw.ServiceUnavailable())
		return
	}
	authorizationURL, key, err := client.BeginAuthorization(r.Context(), oidc.BeginOptions{
		Scopes: rt.config.OIDC.Scopes,
	})
	if err != nil {
		pw.Logger(r.Context()).Log(r.Context(), pw.LevelError, "oidc authorization request failed", pw.Err(err))
		pw.WriteProblem(w, r, pw.ServiceUnavailable())
		return
	}
	rt.writeTransactionCookie(w, key+returnPathSeparator+returnPath)
	rt.redirect(w, r, authorizationURL)
}

func (rt *runtime) handleCallback(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}
	key, returnPath, ok := rt.takeTransactionCookie(w, r)
	if !ok {
		pw.WriteProblem(w, r, pw.BadRequest())
		return
	}
	query := r.URL.Query()
	if providerError := query.Get("error"); providerError != "" {
		// The provider rejected the request; its description is not echoed
		// back to the browser.
		pw.Logger(r.Context()).Log(r.Context(), pw.LevelWarn, "oidc provider returned an error", pw.String("error", providerError))
		pw.WriteProblem(w, r, pw.Forbidden())
		return
	}
	client, err := rt.oidcClient(r.Context())
	if err != nil {
		pw.WriteProblem(w, r, pw.ServiceUnavailable())
		return
	}
	tokens, err := client.HandleCallback(r.Context(), key, oidc.Callback{
		State: query.Get("state"),
		Code:  query.Get("code"),
	})
	if err != nil {
		pw.Logger(r.Context()).Log(r.Context(), pw.LevelWarn, "oidc callback rejected", pw.Err(err))
		pw.WriteProblem(w, r, pw.Forbidden())
		return
	}
	identity, err := verifiedIdentity(r.Context(), client, tokens, rt.config.OIDC.IdentityClaim)
	if err != nil {
		pw.Logger(r.Context()).Log(r.Context(), pw.LevelWarn, "id token verification failed", pw.Err(err))
		pw.WriteProblem(w, r, pw.Forbidden())
		return
	}
	account, err := admit(r.Context(), rt.config.OIDC, rt.allowlist, identity)
	if err != nil {
		if !errors.Is(err, ErrAccessDenied) {
			pw.Logger(r.Context()).Log(r.Context(), pw.LevelError, "account resolution failed", pw.Err(err))
			pw.WriteProblem(w, r, pw.InternalServerError(err))
			return
		}
		// One response shape for every admission failure keeps the endpoint
		// from reporting whether an account exists.
		pw.WriteProblem(w, r, pw.Forbidden())
		return
	}
	// Rotation, rather than creation, revokes any session the browser already
	// held before this authentication.
	err = rt.manager.RotateWithMethod(w, r, SessionData{
		AccountID:   account.ID,
		Issuer:      identity.Issuer,
		Subject:     identity.Subject,
		KeyClaim:    identity.KeyClaim,
		Key:         identity.Key,
		DisplayName: account.DisplayName,
		Email:       account.Email,
	}, MethodOIDC)
	if err != nil {
		pw.Logger(r.Context()).Log(r.Context(), pw.LevelError, "session creation failed", pw.Err(err))
		pw.WriteProblem(w, r, pw.ServiceUnavailable())
		return
	}
	rt.redirect(w, r, rt.landingPath(returnPath))
}

func (rt *runtime) handleLogout(w http.ResponseWriter, r *http.Request) {
	// Logout changes server state, so it is a POST and is checked for
	// same-origin submission.
	if !allowMethod(w, r, http.MethodPost) {
		return
	}
	if !sameOrigin(r) {
		pw.WriteProblem(w, r, pw.Forbidden())
		return
	}
	if err := rt.manager.Delete(w, r); err != nil {
		pw.Logger(r.Context()).Log(r.Context(), pw.LevelError, "session deletion failed", pw.Err(err))
		pw.WriteProblem(w, r, pw.ServiceUnavailable())
		return
	}
	// Ending only the local session leaves the provider signed in, so the next
	// login returns the same user without asking and the sign-out looks like it
	// did nothing.
	if rt.config.OIDC.ProviderLogout {
		if target := rt.endSessionURL(r); target != "" {
			rt.redirect(w, r, target)
			return
		}
	}
	rt.redirect(w, r, "/")
}

// verifiedIdentity reads the ID Token claims of a callback whose nonce binding
// HandleCallback already validated.
func verifiedIdentity(ctx context.Context, client *oidc.Client, tokens oidc.TokenSet, identityClaim string) (Identity, error) {
	raw, ok := tokens.Raw["id_token"]
	if !ok {
		return Identity{}, errors.New("auth: callback response has no id_token")
	}
	var encoded string
	if err := json.Unmarshal(raw, &encoded); err != nil || encoded == "" {
		return Identity{}, errors.New("auth: malformed id_token")
	}
	idToken, err := client.VerifyIDToken(ctx, encoded)
	if err != nil {
		return Identity{}, err
	}
	return identityFrom(idToken.Claims, identityClaim), nil
}

// endSessionURL builds the RP-initiated logout request, or returns an empty
// string when the provider advertises no end session endpoint, discovery is
// unavailable, or the request cannot be built. Each of those falls back to the
// local logout rather than stranding the browser.
//
// No id_token_hint is sent: SessionData deliberately holds no token body, and
// client_id with a post_logout_redirect_uri identifies the relying party well
// enough for a provider that accepts either.
func (rt *runtime) endSessionURL(r *http.Request) string {
	client, err := rt.oidcClient(r.Context())
	if err != nil {
		pw.Logger(r.Context()).Log(r.Context(), pw.LevelWarn, "provider logout unavailable", pw.Err(err))
		return ""
	}
	target, err := client.EndSessionURL(oidc.EndSessionOptions{
		PostLogoutRedirectURI: rt.postLogoutRedirectURI(r),
	})
	if err != nil {
		pw.Logger(r.Context()).Log(r.Context(), pw.LevelWarn, "provider logout request rejected", pw.Err(err))
		return ""
	}
	return target
}

// postLogoutRedirectURI is the absolute form of the local landing path, because
// a provider needs a full URL to return the browser to.
func (rt *runtime) postLogoutRedirectURI(r *http.Request) string {
	if r.Host == "" {
		return ""
	}
	scheme := "http"
	if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	return (&url.URL{Scheme: scheme, Host: r.Host, Path: "/"}).String()
}

// oidcClient discovers the provider on first use and caches the client.
// Discovery failures are not cached, so a provider that starts later still
// works without restarting the application.
func (rt *runtime) oidcClient(ctx context.Context) (*oidc.Client, error) {
	rt.discovery.Lock()
	defer rt.discovery.Unlock()
	if rt.client != nil {
		return rt.client, nil
	}
	discoveryCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	provider, err := oidc.Discover(discoveryCtx, rt.config.OIDC.Issuer, oidc.DiscoverOptions{
		AllowLoopbackHTTP: rt.config.OIDC.AllowLoopbackHTTP,
	})
	if err != nil {
		return nil, err
	}
	client, err := oidc.NewClient(provider, oidc.Config{
		ClientID:          rt.config.OIDC.ClientID,
		ClientSecret:      rt.config.OIDC.ClientSecret,
		RedirectURI:       rt.config.OIDC.RedirectURL,
		AllowLoopbackHTTP: rt.config.OIDC.AllowLoopbackHTTP,
	}, oidc.Options{
		OAuth: oauth.Options{StateStore: rt.stateStore},
	})
	if err != nil {
		return nil, err
	}
	rt.client = client
	return client, nil
}

func (rt *runtime) writeTransactionCookie(w http.ResponseWriter, value string) {
	http.SetCookie(w, &http.Cookie{
		Name:     rt.transactionCookieName(),
		Value:    value,
		Path:     rt.config.CallbackPath,
		MaxAge:   transactionCookieMaxAge,
		Secure:   rt.cookieSecure(),
		HttpOnly: true,
		// The provider redirect is a cross-site top-level navigation, so the
		// cookie must survive it.
		SameSite: http.SameSiteLaxMode,
	})
}

// takeTransactionCookie reads and immediately expires the pending transaction
// cookie, so one callback can consume it only once.
func (rt *runtime) takeTransactionCookie(w http.ResponseWriter, r *http.Request) (key, returnPath string, ok bool) {
	cookie, err := r.Cookie(rt.transactionCookieName())
	http.SetCookie(w, &http.Cookie{
		Name:     rt.transactionCookieName(),
		Value:    "",
		Path:     rt.config.CallbackPath,
		MaxAge:   -1,
		Secure:   rt.cookieSecure(),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	if err != nil || cookie == nil || cookie.Value == "" {
		return "", "", false
	}
	key, rest, found := strings.Cut(cookie.Value, returnPathSeparator)
	if !found || key == "" {
		return "", "", false
	}
	return key, localReturnPath(rest), true
}

func (rt *runtime) transactionCookieName() string {
	return rt.manager.CookieName() + transactionCookieSuffix
}

// cookieSecure mirrors the session cookie policy, so the correlation cookie is
// never weaker than the session it protects.
func (rt *runtime) cookieSecure() bool {
	return rt.cookiePolicy.Secure
}

// landingPath prefers a validated local return path and otherwise uses the
// configured post-login path.
func (rt *runtime) landingPath(returnPath string) string {
	if returnPath != "" {
		return returnPath
	}
	return rt.config.PostLoginPath
}

func (rt *runtime) redirect(w http.ResponseWriter, r *http.Request, location string) {
	w.Header().Set("Cache-Control", "no-store")
	http.Redirect(w, r, location, http.StatusSeeOther)
}

// localReturnPath accepts only a rooted same-site path. An absolute or
// protocol-relative value would turn login into an open redirect.
func localReturnPath(value string) string {
	if value == "" || !strings.HasPrefix(value, "/") || strings.HasPrefix(value, "//") {
		return ""
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "" || parsed.Host != "" || parsed.Opaque != "" {
		return ""
	}
	if strings.Contains(value, returnPathSeparator) {
		return ""
	}
	for _, segment := range strings.Split(strings.TrimPrefix(parsed.Path, "/"), "/") {
		if segment == "." || segment == ".." {
			return ""
		}
	}
	return value
}

// sameOrigin requires an Origin header that matches the request host, falling
// back to a strict Referer check only when Origin is absent.
func sameOrigin(r *http.Request) bool {
	host := r.Host
	if origin := r.Header.Get("Origin"); origin != "" {
		parsed, err := url.Parse(origin)
		return err == nil && parsed.Host == host
	}
	referer := r.Header.Get("Referer")
	if referer == "" {
		return false
	}
	parsed, err := url.Parse(referer)
	return err == nil && parsed.Host == host
}

func allowMethod(w http.ResponseWriter, r *http.Request, allowed ...string) bool {
	for _, method := range allowed {
		if r.Method == method {
			return true
		}
	}
	w.Header().Set("Allow", strings.Join(allowed, ", "))
	pw.WriteProblem(w, r, pw.Problem{
		Status: http.StatusMethodNotAllowed, Title: "Method Not Allowed", Code: "method_not_allowed",
	})
	return false
}
