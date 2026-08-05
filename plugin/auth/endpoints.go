package auth

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/shibukawa/popcornwave/contrib/oauth"
	"github.com/shibukawa/popcornwave/contrib/oidc"
	"github.com/shibukawa/popcornwave/internal/pathpattern"
	"github.com/shibukawa/popcornwave/internal/requestorigin"
	"github.com/shibukawa/popcornwave/pw"
	"github.com/shibukawa/popcornwave/pwruntime"
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
	// reconfirmCookieSuffix names the cookie carrying the one thing a
	// reconfirm logout leaves behind: that the next authorization request must
	// not be answered silently.
	//
	// It needs no integrity protection. Its only effect is to add prompt,
	// which can add an interaction and never remove one, so a forged value
	// costs a redundant confirmation and grants nothing.
	reconfirmCookieSuffix = "_reconfirm"
	// reconfirmCookieMaxAge outlives a browser sitting on the landing page
	// before signing in again, without becoming a lasting setting.
	reconfirmCookieMaxAge = 3600
	// logoutScopeField lets a logout form escalate to a global sign-out where
	// the deployment permits it.
	logoutScopeField = "scope"
)

// Logout scopes decide what a logout does to the provider session. There is no
// local-only scope: see OIDCConfig.LogoutScope.
const (
	LogoutScopeReconfirm = "reconfirm"
	LogoutScopeGlobal    = "global"
)

// authenticate finalizes the request authentication and then serves the login
// endpoints.
//
// Deriving the authentication is this package's job rather than the session
// package's: a session is storage, and only what is in this package's own slot
// says that the browser holding it is a signed-in account. SlotAuthentication
// is where that is settled, which is what the slot is named for.
func (rt *runtime) authenticate(next http.Handler) http.Handler {
	endpoints := rt.endpoints(next)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if data, ok := Session(r.Context()); ok {
			// The account is re-read here, not only where the session was
			// created: suspending an account is how a deployment answers a
			// compromise, and the session it needs to reach is one that already
			// exists. rt.accounts bounds how often that read happens.
			switch rt.accounts.admit(r.Context(), data.AccountID) {
			case accountEnded:
				// The account may no longer act, so the session goes with it and
				// the request continues as an anonymous one. Destroying it here
				// means the browser stops carrying a session nothing will honour
				// rather than presenting it until it expires.
				if err := rt.manager.Destroy(w, r); err != nil {
					pw.Logger(r.Context()).Log(r.Context(), pw.LevelError,
						"session of an ended account could not be destroyed", pw.Err(err))
				}
			case accountUnknown:
				// The credential was not judged. Refusing with 503 rather than
				// 401 says the request may succeed on a retry and keeps a
				// suspension from being conditional on the account store being
				// reachable, which is the same trade policy:token-revocation
				// settles the same way.
				pw.Logger(r.Context()).Log(r.Context(), pw.LevelError,
					"account behind a session could not be read", pw.String("account", data.AccountID))
				pw.WriteProblem(w, r, pw.ServiceUnavailable())
				return
			default:
				r = r.WithContext(pwruntime.WithAuthentication(r.Context(), pwruntime.Authentication{
					Authenticated:   true,
					Subject:         data.AccountID,
					Method:          data.Method,
					Principal:       data,
					AuthenticatedAt: data.AuthenticatedAt,
				}))
			}
		}
		endpoints.ServeHTTP(w, r)
	})
}

// endpoints owns the login, callback, and logout paths and passes every other
// request through.
func (rt *runtime) endpoints(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path, ok := pathpattern.CanonicalPath(r)
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
		case rt.hint != nil && path == rt.forgetPath():
			rt.handleForget(w, r)
		case rt.config.Assurance.Presence.Enabled && path == rt.presencePath():
			rt.handlePresence(w, r)
		default:
			next.ServeHTTP(w, r)
		}
	})
}

func (rt *runtime) handleLogin(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet, http.MethodHead) {
		return
	}
	query := r.URL.Query()
	returnPath := localReturnPath(query.Get("next"))
	// A step-up arrives already authenticated and asks to prove again, so the
	// usual shortcut of sending an authenticated browser straight to the
	// landing path would defeat it.
	stepUp, stepUpWindow := rt.pendingStepUp(r, query)
	if pw.Authenticated(r.Context()) && !stepUp {
		rt.redirect(w, r, rt.landingPath(returnPath))
		return
	}
	client, err := rt.oidcClient(r.Context())
	if err != nil {
		pw.Logger(r.Context()).Log(r.Context(), pw.LevelError, "oidc discovery failed", pw.Err(err))
		pw.WriteProblem(w, r, pw.ServiceUnavailable())
		return
	}
	begin := oidc.BeginOptions{Scopes: rt.config.OIDC.Scopes}
	if stepUp {
		// max_age is the load-bearing half: the provider must return auth_time
		// when it is sent, so the answer can be checked. prompt=login only
		// improves the odds of an interactive confirmation and reports nothing.
		begin.MaxAge = &stepUpWindow
		begin.Prompt = []string{"login"}
	} else if rt.takeReconfirm(w, r) {
		// select_account names who the provider knows, which is the account
		// picker a returning user expects, and login stops it from being
		// answered from a single sign-on session the logout left alive.
		//
		// A shared-device deployment withholds select_account: it exists to
		// surface exactly what that mode is trying to hide.
		if rt.config.SharedDevice {
			begin.Prompt = []string{"login"}
		} else {
			begin.Prompt = []string{"select_account", "login"}
		}
	}
	authorizationURL, key, err := client.BeginAuthorization(r.Context(), begin)
	if err != nil {
		pw.Logger(r.Context()).Log(r.Context(), pw.LevelError, "oidc authorization request failed", pw.Err(err))
		pw.WriteProblem(w, r, pw.ServiceUnavailable())
		return
	}
	rt.writeTransactionCookie(w, encodeTransaction(key, stepUp, stepUpWindow, returnPath))
	rt.redirect(w, r, authorizationURL)
}

// stepUpQueryField carries the requested window into the login endpoint. It is
// not trusted: a larger value only weakens the request, and the guard that sent
// the browser here re-evaluates its own requirement when it returns, so a
// tampered window fails that check instead of passing it.
const stepUpQueryField = "max_age"

// pendingStepUp reports whether this login is a re-proof of the session the
// browser already holds, and the window it must satisfy.
func (rt *runtime) pendingStepUp(r *http.Request, query url.Values) (bool, time.Duration) {
	raw := query.Get(stepUpQueryField)
	if raw == "" || !pw.Authenticated(r.Context()) {
		return false, 0
	}
	seconds, err := strconv.Atoi(raw)
	if err != nil || seconds < 0 || seconds > int(maxStepUpWindow/time.Second) {
		return false, 0
	}
	return true, time.Duration(seconds) * time.Second
}

// maxStepUpWindow bounds the window a redirect may name, so a hostile link
// cannot ask for one so large that the re-proof is satisfied by anything.
const maxStepUpWindow = 24 * time.Hour

// stepUpPath builds the local login URL that re-proves the current session and
// returns to the operation that was refused.
func (rt *runtime) stepUpPath(r *http.Request, window time.Duration) string {
	values := url.Values{}
	values.Set(stepUpQueryField, strconv.Itoa(int(window/time.Second)))
	if next := localReturnPath(r.URL.RequestURI()); next != "" {
		values.Set("next", next)
	}
	return rt.config.LoginPath + "?" + values.Encode()
}

func (rt *runtime) handleCallback(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}
	key, stepUp, stepUpWindow, returnPath, ok := rt.takeTransaction(w, r)
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
	_, idToken, err := client.HandleCallback(r.Context(), key, oidc.Callback{
		State: query.Get("state"),
		Code:  query.Get("code"),
	}, oidc.CallbackOptions{RequireAuthTime: stepUp})
	if err != nil {
		pw.Logger(r.Context()).Log(r.Context(), pw.LevelWarn, "oidc callback rejected", pw.Err(err))
		pw.WriteProblem(w, r, pw.Forbidden())
		return
	}
	identity := identityFrom(idToken.Claims, rt.config.OIDC.IdentityClaim)
	if stepUp {
		rt.completeStepUp(w, r, identity, idToken, stepUpWindow, returnPath)
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
	data := SessionData{
		AccountID:   account.ID,
		Issuer:      identity.Issuer,
		Subject:     identity.Subject,
		KeyClaim:    identity.KeyClaim,
		Key:         identity.Key,
		DisplayName: account.DisplayName,
		Email:       account.Email,
	}
	// A provider that reported auth_time without being asked still reported the
	// truth about when it authenticated this person, and that is what freshness
	// is measured from.
	if idToken.AuthTime != nil {
		data.ProviderAuthTime = idToken.AuthTime.Unix()
	}
	err = rt.establish(w, r, data, MethodOIDC)
	if err != nil {
		pw.Logger(r.Context()).Log(r.Context(), pw.LevelError, "session creation failed", pw.Err(err))
		pw.WriteProblem(w, r, pw.ServiceUnavailable())
		return
	}
	rt.rememberSignIn(w, data)
	rt.redirect(w, r, rt.landingPath(returnPath))
}

// completeStepUp refreshes the assurance of the session the browser already
// holds. It never provisions, never links, and never changes which account is
// signed in: a step-up that accepted any successful login would be an account
// swap with the previous account's guarded operation already staged.
func (rt *runtime) completeStepUp(w http.ResponseWriter, r *http.Request, identity Identity, idToken oidc.IDToken, window time.Duration, returnPath string) {
	view, ok := Session(r.Context())
	if !ok {
		// The session ended while the browser was at the provider. Nothing is
		// left to refresh, and silently creating one would turn a re-proof into
		// a login the guard never asked for.
		pw.WriteProblem(w, r, pw.Forbidden())
		return
	}
	if identity.Issuer != view.Issuer || identity.KeyClaim != view.KeyClaim || identity.Key != view.Key || identity.Key == "" {
		pw.Logger(r.Context()).Log(r.Context(), pw.LevelWarn, "step-up completed by a different identity")
		pw.WriteProblem(w, r, pw.Forbidden())
		return
	}
	// Admission still applies, so an identity that lost it fails the re-proof
	// rather than refreshing a session it may no longer have.
	if _, err := admit(r.Context(), rt.config.OIDC, rt.allowlist, identity); err != nil {
		if !errors.Is(err, ErrAccessDenied) {
			pw.Logger(r.Context()).Log(r.Context(), pw.LevelError, "step-up admission failed", pw.Err(err))
			pw.WriteProblem(w, r, pw.InternalServerError(err))
			return
		}
		pw.WriteProblem(w, r, pw.Forbidden())
		return
	}
	data := view
	if idToken.AuthTime != nil {
		data.ProviderAuthTime = idToken.AuthTime.Unix()
	}
	if window == 0 {
		data.StepUpAt = time.Now().Unix()
	}
	// Rotation, because an assurance change is an authentication-strength
	// change: the previous token is revoked and the CSRF secret turns with it.
	if err := rt.establish(w, r, data, MethodOIDC); err != nil {
		pw.Logger(r.Context()).Log(r.Context(), pw.LevelError, "step-up session rotation failed", pw.Err(err))
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
	if !rt.sameOrigin(r) {
		pw.WriteProblem(w, r, pw.Forbidden())
		return
	}
	// An explicit sign-out asks to be forgotten, so the hint goes with the
	// session. Only expiry leaves one behind.
	rt.forgetSignIn(w)
	// The local session goes first and unconditionally, whatever the selected
	// scope does afterward.
	if err := rt.endSession(w, r); err != nil {
		pw.Logger(r.Context()).Log(r.Context(), pw.LevelError, "session deletion failed", pw.Err(err))
		pw.WriteProblem(w, r, pw.ServiceUnavailable())
		return
	}
	if rt.logoutScope(r) == LogoutScopeGlobal {
		if target := rt.endSessionURL(r); target != "" {
			rt.redirect(w, r, target)
			return
		}
		// A provider advertising no end session endpoint degrades to reconfirm
		// rather than to a silent logout: the next login must still ask.
	}
	// Reconfirm asks the provider for nothing. It records that the next
	// authorization request carries prompt, which is what stops the provider
	// from answering silently from a single sign-on session this logout could
	// not reach.
	rt.markReconfirm(w)
	rt.redirect(w, r, "/")
}

// forgetPath is the not-me control of a login screen. It hangs off the logout
// path because it is the same act at a lower level: one ends a session, the
// other ends the memory of one.
func (rt *runtime) forgetPath() string { return rt.config.LogoutPath + "/forget" }

// handleForget clears the sign-in hint. It requires no session and no
// authentication, because the person pressing it is by definition not signed
// in, and it grants nothing: clearing a hint can only make the next sign-in
// longer.
//
// It is still POST and same-origin, for the reason the logout endpoint is: a
// control reachable by a link or a prefetch is one anything that fetches URLs
// can trigger on the user's behalf.
func (rt *runtime) handleForget(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodPost) {
		return
	}
	if !rt.sameOrigin(r) {
		pw.WriteProblem(w, r, pw.Forbidden())
		return
	}
	rt.forgetSignIn(w)
	rt.redirect(w, r, "/")
}

// logoutScope resolves the configured scope, allowing a request to escalate to
// global and never to downgrade. Escalation costs the user extra sign-outs;
// a forced downgrade would leave the provider session alive after the user
// asked to leave it.
func (rt *runtime) logoutScope(r *http.Request) string {
	if rt.config.OIDC.LogoutScope == LogoutScopeGlobal {
		return LogoutScopeGlobal
	}
	if rt.config.OIDC.AllowGlobalLogoutRequest && r.PostFormValue(logoutScopeField) == LogoutScopeGlobal {
		return LogoutScopeGlobal
	}
	return LogoutScopeReconfirm
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

// encodeTransaction packs the correlation key, whether this is a re-proof of an
// existing session, its window, and the local return path into one value.
//
// The key is base64url and the window is digits, so neither can contain the
// separator and only the trailing return path can.
func encodeTransaction(key string, stepUp bool, window time.Duration, returnPath string) string {
	marker := "0"
	if stepUp {
		marker = strconv.Itoa(int(window/time.Second) + 1)
	}
	return key + returnPathSeparator + marker + returnPathSeparator + returnPath
}

// takeTransactionCookie reads and immediately expires the pending transaction
// cookie, so one callback can consume it only once.
//
// stepUp reports a re-proof of the session the browser already holds, and
// window is what it must satisfy. Both come from the server-set cookie rather
// than from the callback query, so the requirement a login started with is the
// one its callback enforces.
func (rt *runtime) takeTransaction(w http.ResponseWriter, r *http.Request) (key string, stepUp bool, window time.Duration, returnPath string, ok bool) {
	rawKey, rest, found := rt.takeTransactionCookie(w, r)
	if !found {
		return "", false, 0, "", false
	}
	marker, path, split := strings.Cut(rest, returnPathSeparator)
	if !split {
		return "", false, 0, "", false
	}
	value, err := strconv.Atoi(marker)
	if err != nil || value < 0 {
		return "", false, 0, "", false
	}
	if value > 0 {
		// The marker is the window plus one, so zero stays available to mean
		// "not a step-up" while a zero-second window is still expressible.
		return rawKey, true, time.Duration(value-1) * time.Second, localReturnPath(path), true
	}
	return rawKey, false, 0, localReturnPath(path), true
}

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
	return key, rest, true
}

func (rt *runtime) transactionCookieName() string {
	return rt.manager.CookieName() + transactionCookieSuffix
}

func (rt *runtime) reconfirmCookieName() string {
	return rt.manager.CookieName() + reconfirmCookieSuffix
}

// markReconfirm records that the next authorization request must carry prompt.
// This is the whole of a reconfirm logout: nothing is sent to the provider, and
// the provider session of every other relying party is untouched.
func (rt *runtime) markReconfirm(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     rt.reconfirmCookieName(),
		Value:    "1",
		Path:     rt.config.LoginPath,
		MaxAge:   reconfirmCookieMaxAge,
		Secure:   rt.cookieSecure(),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

// takeReconfirm reports whether the pending reconfirmation is set and clears
// it, so the intent survives only until the next authorization request rather
// than becoming a permanent setting.
func (rt *runtime) takeReconfirm(w http.ResponseWriter, r *http.Request) bool {
	cookie, err := r.Cookie(rt.reconfirmCookieName())
	pending := err == nil && cookie != nil && cookie.Value != ""
	if !pending {
		return false
	}
	http.SetCookie(w, &http.Cookie{
		Name:     rt.reconfirmCookieName(),
		Value:    "",
		Path:     rt.config.LoginPath,
		MaxAge:   -1,
		Secure:   rt.cookieSecure(),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	return true
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

// sameOrigin requires a whole origin, scheme included, matching this
// deployment's own or one it declared.
//
// The comparison is internal/requestorigin, the same one the CSRF middleware
// makes. This package used to compare host names alone, which admitted an http
// caller to an https deployment and supported no declared origin at all.
func (rt *runtime) sameOrigin(r *http.Request) bool {
	return requestorigin.Matches(r, rt.trustedOrigins)
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
