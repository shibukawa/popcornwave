package devidp

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/shibukawa/popcornwave/contrib/internal/authn"
)

// routes registers every endpoint under the issuer base path so the handler can
// be mounted directly on the host named by the issuer.
func (p *Provider) routes() http.Handler {
	base := ""
	if parsed, err := url.Parse(p.issuer); err == nil {
		base = strings.TrimSuffix(parsed.Path, "/")
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET "+base+"/.well-known/openid-configuration", p.handleDiscovery)
	mux.HandleFunc("GET "+base+"/jwks.json", p.handleJWKS)
	mux.HandleFunc("GET "+base+"/authorize", p.handleAuthorize)
	mux.HandleFunc("POST "+base+"/authorize", p.handleAuthorize)
	mux.HandleFunc("GET "+base+"/login", p.handleLoginPage)
	mux.HandleFunc("POST "+base+"/login", p.handleLoginSubmit)
	mux.HandleFunc("POST "+base+"/token", p.handleToken)
	mux.HandleFunc("GET "+base+"/end_session", p.handleEndSession)
	mux.HandleFunc("POST "+base+"/end_session", p.handleEndSession)
	mux.HandleFunc("GET "+base+"/userinfo", p.handleUserInfo)
	mux.HandleFunc("POST "+base+"/userinfo", p.handleUserInfo)
	return mux
}

func (p *Provider) endpoint(path string) string { return p.issuer + path }

func (p *Provider) handleDiscovery(w http.ResponseWriter, r *http.Request) {
	p.mu.Lock()
	scopes := append([]string(nil), p.scopes...)
	p.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{
		"issuer":                                p.issuer,
		"authorization_endpoint":                p.endpoint("/authorize"),
		"token_endpoint":                        p.endpoint("/token"),
		"userinfo_endpoint":                     p.endpoint("/userinfo"),
		"end_session_endpoint":                  p.endpoint("/end_session"),
		"jwks_uri":                              p.endpoint("/jwks.json"),
		"response_types_supported":              []string{"code"},
		"response_modes_supported":              []string{"query"},
		"grant_types_supported":                 []string{"authorization_code"},
		"subject_types_supported":               []string{"public"},
		"id_token_signing_alg_values_supported": []string{"RS256"},
		"scopes_supported":                      scopes,
		"token_endpoint_auth_methods_supported": []string{"client_secret_basic", "client_secret_post"},
		"code_challenge_methods_supported":      []string{"S256"},
		"claims_supported":                      p.claimNames(),
	})
}

func (p *Provider) claimNames() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	names := []string{"iss", "sub", "aud", "exp", "iat", "auth_time", "nonce"}
	for index := range p.users {
		for name := range p.users[index].Claims {
			if !contains(names, name) {
				names = append(names, name)
			}
		}
	}
	return names
}

func (p *Provider) handleJWKS(w http.ResponseWriter, r *http.Request) {
	document, err := p.jwksDocument()
	if err != nil {
		http.Error(w, "provider is closed", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(document)
}

// handleAuthorize validates the request, then either renders the selection
// screen or issues a code directly when a login user is pre-selected.
func (p *Provider) handleAuthorize(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		p.renderError(w, http.StatusBadRequest, "The authorization request could not be parsed.")
		return
	}
	client := p.client(r.Form.Get("client_id"))
	if client == nil {
		p.renderError(w, http.StatusBadRequest, "Unknown client_id. Register the client before starting a login.")
		return
	}
	redirect, err := p.matchRedirect(client, r.Form.Get("redirect_uri"))
	if err != nil {
		p.renderError(w, http.StatusBadRequest, "The redirect_uri is not registered for this client.")
		return
	}
	state := r.Form.Get("state")
	if responseType := r.Form.Get("response_type"); responseType != "code" {
		redirectError(w, r, redirect, "unsupported_response_type", "only the authorization code flow is supported", state)
		return
	}
	if method := r.Form.Get("code_challenge_method"); method != "S256" {
		redirectError(w, r, redirect, "invalid_request", "code_challenge_method must be S256", state)
		return
	}
	challenge := r.Form.Get("code_challenge")
	if !validChallenge(challenge) {
		redirectError(w, r, redirect, "invalid_request", "code_challenge is missing or malformed", state)
		return
	}
	scopes := strings.Fields(r.Form.Get("scope"))
	if !contains(scopes, "openid") {
		redirectError(w, r, redirect, "invalid_scope", "the openid scope is required", state)
		return
	}
	maxAge, ok := parseMaxAge(r.Form.Get("max_age"))
	if !ok {
		redirectError(w, r, redirect, "invalid_request", "max_age must be a non-negative number of seconds", state)
		return
	}
	prompt := strings.Fields(r.Form.Get("prompt"))
	if !validPrompt(prompt) {
		redirectError(w, r, redirect, "invalid_request", "prompt carries an unknown value, or none beside another", state)
		return
	}
	pending := &pendingAuthorization{
		clientID:    client.ID,
		redirectURI: redirect,
		state:       state,
		nonce:       r.Form.Get("nonce"),
		challenge:   challenge,
		scopes:      scopes,
		expiresAt:   p.now().Add(p.codeTTL),
		maxAge:      maxAge,
		prompt:      prompt,
	}
	key, err := p.storePending(pending)
	if err != nil {
		redirectError(w, r, redirect, "server_error", "the provider could not start an authorization", state)
		return
	}

	existing := p.sessionOf(r)
	// A provider answers from its own session when it may. That is what makes a
	// relying party's max_age worth sending, and what makes auth_time worth
	// checking: the token arrives now, carrying a proof from earlier.
	if existing != nil && !needsReauthentication(prompt, maxAge, existing, p.now()) {
		p.completeAuthorization(w, r, key, existing.subject, existing.authTime)
		return
	}
	if contains(prompt, "none") {
		// prompt=none forbids interaction, so a provider that would have to ask
		// says so instead of asking.
		p.takePending(key)
		redirectError(w, r, redirect, "login_required", "the end user must authenticate, which prompt=none forbids", state)
		return
	}
	if subject := p.LoginUser(); subject != "" && !contains(prompt, "select_account") {
		p.completeLogin(w, r, key, subject)
		return
	}
	p.renderLogin(w, r, key, pending, "")
}

// parseMaxAge reads the max_age parameter. The bool reports a usable value; a
// negative result means the request carried none.
func parseMaxAge(raw string) (int64, bool) {
	if raw == "" {
		return -1, true
	}
	seconds, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || seconds < 0 {
		return 0, false
	}
	return seconds, true
}

// validPrompt accepts the values OpenID Connect Core defines, and refuses none
// beside any other because the two ask for opposite things.
func validPrompt(values []string) bool {
	for _, value := range values {
		switch value {
		case "none", "login", "consent", "select_account":
		default:
			return false
		}
	}
	return !(contains(values, "none") && len(values) > 1)
}

// needsReauthentication decides whether this provider session may answer the
// request as it stands.
func needsReauthentication(prompt []string, maxAge int64, session *providerSession, now time.Time) bool {
	if contains(prompt, "login") || contains(prompt, "select_account") {
		return true
	}
	if maxAge >= 0 && now.Sub(session.authTime) > time.Duration(maxAge)*time.Second {
		return true
	}
	return false
}

// sessionOf reads the provider login of the requesting browser.
func (p *Provider) sessionOf(r *http.Request) *providerSession {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return nil
	}
	return p.session(cookie.Value)
}

// handleLoginPage re-renders the selection screen for a pending authorization.
func (p *Provider) handleLoginPage(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("auth")
	pending := p.peekPending(key)
	if pending == nil {
		p.renderError(w, http.StatusBadRequest, "This login link has expired. Start the login again from the application.")
		return
	}
	p.renderLogin(w, r, key, pending, "")
}

func (p *Provider) handleLoginSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		p.renderError(w, http.StatusBadRequest, "The selection could not be parsed.")
		return
	}
	key := r.Form.Get("auth")
	pending := p.peekPending(key)
	if pending == nil {
		p.renderError(w, http.StatusBadRequest, "This login link has expired. Start the login again from the application.")
		return
	}
	if !authn.EqualSecret(pending.csrf, r.Form.Get("csrf")) {
		p.renderError(w, http.StatusBadRequest, "The selection could not be verified. Start the login again.")
		return
	}
	if r.Form.Get("cancel") != "" {
		p.takePending(key)
		redirectError(w, r, pending.redirectURI, "access_denied", "the developer cancelled the login", pending.state)
		return
	}
	p.completeLogin(w, r, key, r.Form.Get("subject"))
}

// completeLogin records a fresh authentication of this browser and completes the
// pending authorization. Everything that actually asks the developer who they
// are goes through here, so this is the one place a provider session begins.
func (p *Provider) completeLogin(w http.ResponseWriter, r *http.Request, key, subject string) {
	now := p.now()
	if sessionKey, err := p.startSession(subject, now); err == nil {
		http.SetCookie(w, &http.Cookie{
			Name:     sessionCookieName,
			Value:    sessionKey,
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		})
	}
	p.completeAuthorization(w, r, key, subject, now)
}

// completeAuthorization consumes the pending authorization and redirects with a
// code. authTime is when the end user was actually authenticated, which an
// answer from an existing session carries forward unchanged.
func (p *Provider) completeAuthorization(w http.ResponseWriter, r *http.Request, key, subject string, authTime time.Time) {
	pending := p.peekPending(key)
	if pending == nil {
		p.renderError(w, http.StatusBadRequest, "This login link has expired. Start the login again from the application.")
		return
	}
	user := p.user(subject)
	if user == nil {
		p.renderLogin(w, r, key, pending, "Select a user from the roster.")
		return
	}
	client := p.client(pending.clientID)
	if client == nil {
		p.takePending(key)
		redirectError(w, r, pending.redirectURI, "invalid_client", "the client is no longer registered", pending.state)
		return
	}
	now := p.now()
	code := &issuedCode{
		clientID:    pending.clientID,
		redirectURI: pending.redirectURI,
		challenge:   pending.challenge,
		nonce:       pending.nonce,
		subject:     user.Subject,
		scopes:      p.grantableScopes(pending.scopes, client, user),
		issuedAt:    now,
		expiresAt:   now.Add(p.codeTTL),
		authTime:    authTime,
	}
	value, err := p.issueCode(key, code)
	if err != nil {
		redirectError(w, r, pending.redirectURI, "server_error", "the provider could not issue a code", pending.state)
		return
	}
	target, err := url.Parse(pending.redirectURI)
	if err != nil {
		p.renderError(w, http.StatusBadRequest, "The redirect_uri is unusable.")
		return
	}
	query := target.Query()
	query.Set("code", value)
	if pending.state != "" {
		query.Set("state", pending.state)
	}
	target.RawQuery = query.Encode()
	p.logf("devidp: signed in as %s for client %s", user.Subject, pending.clientID)
	http.Redirect(w, r, target.String(), http.StatusFound)
}

func (p *Provider) handleToken(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeTokenError(w, http.StatusBadRequest, "invalid_request", "the token request could not be parsed")
		return
	}
	clientID, secret, ok := clientCredentials(r)
	if !ok {
		w.Header().Set("WWW-Authenticate", `Basic realm="devidp"`)
		writeTokenError(w, http.StatusUnauthorized, "invalid_client", "client authentication is required")
		return
	}
	client := p.client(clientID)
	if client == nil || !authn.EqualSecret(client.Secret, secret) {
		w.Header().Set("WWW-Authenticate", `Basic realm="devidp"`)
		writeTokenError(w, http.StatusUnauthorized, "invalid_client", "client authentication failed")
		return
	}
	if r.Form.Get("grant_type") != "authorization_code" {
		writeTokenError(w, http.StatusBadRequest, "unsupported_grant_type", "only authorization_code is supported")
		return
	}
	code := p.takeCode(r.Form.Get("code"))
	if code == nil || p.now().After(code.expiresAt) || code.clientID != client.ID {
		writeTokenError(w, http.StatusBadRequest, "invalid_grant", "the authorization code is unknown, expired, or already used")
		return
	}
	if r.Form.Get("redirect_uri") != code.redirectURI {
		writeTokenError(w, http.StatusBadRequest, "invalid_grant", "the redirect_uri does not match the authorization request")
		return
	}
	verifier := r.Form.Get("code_verifier")
	if err := authn.ValidatePKCEVerifier(verifier); err != nil {
		writeTokenError(w, http.StatusBadRequest, "invalid_grant", "the code_verifier is missing or malformed")
		return
	}
	challenge, err := authn.PKCEChallengeS256(verifier)
	if err != nil || !authn.EqualSecret(challenge, code.challenge) {
		writeTokenError(w, http.StatusBadRequest, "invalid_grant", "the code_verifier does not match the code_challenge")
		return
	}
	user := p.user(code.subject)
	if user == nil {
		writeTokenError(w, http.StatusBadRequest, "invalid_grant", "the authenticated user left the roster")
		return
	}
	now := p.now()
	idToken, err := p.idToken(code, user, now)
	if err != nil {
		writeTokenError(w, http.StatusInternalServerError, "server_error", "the provider could not sign an ID Token")
		return
	}
	access, err := p.issueAccessToken(code, now)
	if err != nil {
		writeTokenError(w, http.StatusInternalServerError, "server_error", "the provider could not issue an access token")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	writeJSON(w, http.StatusOK, map[string]any{
		"access_token": access,
		"token_type":   "Bearer",
		"expires_in":   int(p.tokenTTL / time.Second),
		"id_token":     idToken,
		"scope":        strings.Join(code.scopes, " "),
	})
}

// handleEndSession implements RP-initiated logout. A relying party that only
// drops its own cookie leaves the provider session intact, and the next login
// silently signs the same user straight back in; ending both is the point of
// this endpoint.
func (p *Provider) handleEndSession(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		p.renderError(w, http.StatusBadRequest, "The logout request could not be parsed.")
		return
	}
	clientID := r.Form.Get("client_id")
	subject := ""
	if hint := r.Form.Get("id_token_hint"); hint != "" {
		claims, err := p.verifyJWT(hint)
		if err != nil {
			p.renderError(w, http.StatusBadRequest, "The id_token_hint was not issued by this provider.")
			return
		}
		subject = claimString(claims, "sub")
		if audience := claimString(claims, "aud"); audience != "" {
			if clientID != "" && clientID != audience {
				p.renderError(w, http.StatusBadRequest, "The id_token_hint belongs to a different client.")
				return
			}
			clientID = audience
		}
	}
	client := p.client(clientID)
	if client == nil {
		p.renderError(w, http.StatusBadRequest, "Unknown client_id.")
		return
	}
	// Every token issued to this subject stops working, so a stale access
	// token cannot outlive the logout.
	p.revokeTokens(clientID, subject)
	// The provider session goes too. Leaving it would make the next
	// authorization answer silently from it, which is the behaviour a global
	// sign-out exists to prevent.
	p.endSessions(subject)
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	redirect := r.Form.Get("post_logout_redirect_uri")
	if redirect == "" {
		p.renderError(w, http.StatusOK, "You are signed out of the development identity provider.")
		return
	}
	target, err := p.matchPostLogoutRedirect(client, redirect)
	if err != nil {
		p.renderError(w, http.StatusBadRequest, "The post_logout_redirect_uri must be a local URL.")
		return
	}
	parsed, err := url.Parse(target)
	if err != nil {
		p.renderError(w, http.StatusBadRequest, "The post_logout_redirect_uri is unusable.")
		return
	}
	if state := r.Form.Get("state"); state != "" {
		query := parsed.Query()
		query.Set("state", state)
		parsed.RawQuery = query.Encode()
	}
	p.logf("devidp: ended the session of %s for client %s", subjectOrAny(subject), clientID)
	http.Redirect(w, r, parsed.String(), http.StatusFound)
}

func subjectOrAny(subject string) string {
	if subject == "" {
		return "an unnamed user"
	}
	return subject
}

func (p *Provider) handleUserInfo(w http.ResponseWriter, r *http.Request) {
	header := r.Header.Get("Authorization")
	value, ok := strings.CutPrefix(header, "Bearer ")
	if !ok || strings.TrimSpace(value) == "" {
		w.Header().Set("WWW-Authenticate", `Bearer realm="devidp"`)
		http.Error(w, "", http.StatusUnauthorized)
		return
	}
	token := p.lookupAccessToken(strings.TrimSpace(value))
	if token == nil {
		w.Header().Set("WWW-Authenticate", `Bearer error="invalid_token"`)
		http.Error(w, "", http.StatusUnauthorized)
		return
	}
	user := p.user(token.subject)
	if user == nil {
		w.Header().Set("WWW-Authenticate", `Bearer error="invalid_token"`)
		http.Error(w, "", http.StatusUnauthorized)
		return
	}
	claims := identityClaims(user, token.scopes)
	claims["sub"] = user.Subject
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, claims)
}

// matchPostLogoutRedirect accepts any local URL, registered or not.
//
// A real provider requires the post-logout URL to be registered, and a relying
// party must register it there. This provider deliberately does not: it exists
// so a login works before any registration exists, and demanding one for the
// logout would put the friction back in the one place it was removed from. The
// target still has to be local, so this cannot become an open redirect for
// anything reachable from outside the machine.
func (p *Provider) matchPostLogoutRedirect(client *Client, raw string) (string, error) {
	if err := validateRedirectURI(raw); err != nil {
		return "", err
	}
	for _, candidate := range client.RedirectURIs {
		if candidate == raw {
			return raw, nil
		}
	}
	parsed, err := url.Parse(raw)
	if err != nil || !isLoopbackURL(parsed) {
		return "", errNotRegistered
	}
	p.logf("devidp: accepting unregistered post-logout redirect %s for client %s", raw, client.ID)
	return raw, nil
}

// matchRedirect applies exact matching, or the RFC 8252 loopback relaxation for
// a client the running tool registered before it knew the application port.
func (p *Provider) matchRedirect(client *Client, raw string) (string, error) {
	if err := validateRedirectURI(raw); err != nil {
		return "", err
	}
	for _, candidate := range client.RedirectURIs {
		if candidate == raw {
			return raw, nil
		}
	}
	if client.LoopbackRedirects {
		parsed, err := url.Parse(raw)
		if err != nil || !isLoopbackURL(parsed) {
			return "", errNotRegistered
		}
		p.logf("devidp: accepting loopback callback %s for ephemeral client %s", raw, client.ID)
		return raw, nil
	}
	return "", errNotRegistered
}

// clientCredentials reads client_secret_basic, then client_secret_post.
func clientCredentials(r *http.Request) (string, string, bool) {
	if id, secret, ok := r.BasicAuth(); ok {
		decodedID, errID := url.QueryUnescape(id)
		decodedSecret, errSecret := url.QueryUnescape(secret)
		if errID != nil || errSecret != nil {
			return "", "", false
		}
		if decodedID == "" {
			return "", "", false
		}
		return decodedID, decodedSecret, true
	}
	id := r.Form.Get("client_id")
	secret := r.Form.Get("client_secret")
	if id == "" || secret == "" {
		return "", "", false
	}
	return id, secret, true
}

// validChallenge applies the RFC 7636 code_challenge grammar for S256.
func validChallenge(challenge string) bool {
	if len(challenge) < 43 || len(challenge) > 128 {
		return false
	}
	for index := 0; index < len(challenge); index++ {
		character := challenge[index]
		switch {
		case character >= 'A' && character <= 'Z':
		case character >= 'a' && character <= 'z':
		case character >= '0' && character <= '9':
		case character == '-' || character == '_':
		default:
			return false
		}
	}
	return true
}

func redirectError(w http.ResponseWriter, r *http.Request, redirect, code, description, state string) {
	target, err := url.Parse(redirect)
	if err != nil {
		http.Error(w, description, http.StatusBadRequest)
		return
	}
	query := target.Query()
	query.Set("error", code)
	query.Set("error_description", description)
	if state != "" {
		query.Set("state", state)
	}
	target.RawQuery = query.Encode()
	http.Redirect(w, r, target.String(), http.StatusFound)
}

func writeTokenError(w http.ResponseWriter, status int, code, description string) {
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, status, map[string]any{"error": code, "error_description": description})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	encoded, err := json.Marshal(body)
	if err != nil {
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(encoded)
}
