package auth_test

import (
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"testing"

	"github.com/shibukawa/popcornwave/auth"
	"github.com/shibukawa/popcornwave/pw"
	"github.com/shibukawa/popcornwave/testutil"
)

const roster = `
[users.admin]
display_name = "Administrator"
[users.admin.claims]
email = "admin@example.com"
name = "Ada Admin"
role = "admin"

[users.guest]
display_name = "Guest"
[users.guest.claims]
email = "guest@example.com"
name = "Gary Guest"
`

// whoami is the entire application: no route registration, no OIDC code.
var whoami = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	identity, ok := pw.CurrentUser(r.Context())
	if !ok {
		_, _ = io.WriteString(w, "anonymous")
		return
	}
	role, _ := identity.Claim("role")
	_, _ = io.WriteString(w, "signed in as "+identity.Subject+" ("+identity.Name+", "+identity.Email+", role="+role+")")
})

func runServer(t *testing.T, loginUser string) *testutil.Server {
	t.Helper()
	return runServerWithProviderLogout(t, loginUser, true)
}

func runServerWithProviderLogout(t *testing.T, loginUser string, providerLogout bool) *testutil.Server {
	t.Helper()
	return testutil.TestRun(t, whoami, func(config *testutil.Config) {
		testutil.Update[pw.ServerConfig](config, func(server *pw.ServerConfig) {
			server.Public.Enabled = false
		})
		testutil.Update[pw.SessionConfig](config, func(session *pw.SessionConfig) {
			session.Enabled = true
			session.Secret = "development-session-secret"
		})
	}, testutil.WithIdentityProvider(
		testutil.WithIdPRoster(roster),
		testutil.WithLoginUser(loginUser),
		testutil.WithIdPBinding(func(config *testutil.Config, idp testutil.IdPInfo) {
			testutil.Update[pw.AuthConfig](config, func(auth *pw.AuthConfig) {
				auth.Enabled = true
				auth.Mode = pw.AuthModeOIDC
				auth.LoginPath = pw.DefaultLoginPath
				auth.CallbackPath = pw.DefaultCallbackPath
				auth.LogoutPath = pw.DefaultLogoutPath
				auth.PostLoginRedirect = "/"
				auth.PostLogoutRedirect = "/"
				auth.OIDC.Issuer = idp.Issuer
				auth.OIDC.ClientID = idp.ClientID
				auth.OIDC.ClientSecret = idp.ClientSecret
				auth.OIDC.ProviderLogout = providerLogout
			})
		}),
	))
}

// browser follows redirects and keeps cookies, the way a real one does.
func browser(t *testing.T) *http.Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookie jar: %v", err)
	}
	return &http.Client{Jar: jar}
}

func get(t *testing.T, client *http.Client, target string) (int, string) {
	t.Helper()
	response, err := client.Get(target) //nolint:noctx // loopback test server
	if err != nil {
		t.Fatalf("GET %s: %v", target, err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return response.StatusCode, string(body)
}

func TestLoginAndLogoutNeedNoApplicationCode(t *testing.T) {
	server := runServer(t, "admin")
	client := browser(t)

	if _, body := get(t, client, server.URL+"/"); body != "anonymous" {
		t.Fatalf("before login = %q", body)
	}

	// One GET to the framework-owned login path completes the whole flow.
	status, body := get(t, client, server.URL+pw.DefaultLoginPath)
	if status != http.StatusOK {
		t.Fatalf("login status = %d, body = %q", status, body)
	}
	if body != "signed in as admin (Ada Admin, admin@example.com, role=admin)" {
		t.Fatalf("after login = %q", body)
	}

	// The session survives a fresh request.
	if _, body := get(t, client, server.URL+"/"); !strings.HasPrefix(body, "signed in as admin") {
		t.Fatalf("session did not persist: %q", body)
	}

	response, err := client.PostForm(server.URL+pw.DefaultLogoutPath, url.Values{}) //nolint:noctx // loopback test server
	if err != nil {
		t.Fatalf("logout: %v", err)
	}
	defer response.Body.Close()
	logoutBody, _ := io.ReadAll(response.Body)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("logout status = %d, body = %q", response.StatusCode, logoutBody)
	}
	if string(logoutBody) != "anonymous" {
		t.Fatalf("after logout = %q", logoutBody)
	}
}

func TestLogoutRejectsGet(t *testing.T) {
	server := runServer(t, "admin")
	client := browser(t)
	get(t, client, server.URL+pw.DefaultLoginPath)

	status, _ := get(t, client, server.URL+pw.DefaultLogoutPath)
	if status != http.StatusMethodNotAllowed {
		t.Fatalf("GET logout status = %d, want 405", status)
	}
	// Still signed in: a link or a prefetch cannot end the session.
	if _, body := get(t, client, server.URL+"/"); !strings.HasPrefix(body, "signed in as admin") {
		t.Fatalf("session ended on a GET: %q", body)
	}
}

func TestLogoutRejectsACrossOriginPost(t *testing.T) {
	server := runServer(t, "admin")
	client := browser(t)
	get(t, client, server.URL+pw.DefaultLoginPath)

	request, err := http.NewRequestWithContext(t.Context(), http.MethodPost, server.URL+pw.DefaultLogoutPath, nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	request.Header.Set("Origin", "https://evil.example")
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("logout: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", response.StatusCode)
	}
	if _, body := get(t, client, server.URL+"/"); !strings.HasPrefix(body, "signed in as admin") {
		t.Fatalf("a cross-origin post ended the session: %q", body)
	}
}

func TestCallbackWithoutATransactionFails(t *testing.T) {
	server := runServer(t, "admin")
	client := browser(t)

	status, _ := get(t, client, server.URL+pw.DefaultCallbackPath+"?code=forged&state=forged")
	if status != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", status)
	}
	if _, body := get(t, client, server.URL+"/"); body != "anonymous" {
		t.Fatalf("a forged callback established a session: %q", body)
	}
}

func TestAForgedSessionCookieIsIgnored(t *testing.T) {
	server := runServer(t, "admin")
	client := browser(t)

	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL+"/", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	// A payload whose signature was not produced by the session secret.
	request.AddCookie(&http.Cookie{Name: "pw_session", Value: "eyJzdWIiOiJhZG1pbiIsImV4cCI6NDEwMjQ0NDgwMH0.forged"})
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(response.Body)
	if string(body) != "anonymous" {
		t.Fatalf("forged cookie was accepted: %q", body)
	}
}

func TestDifferentUsersProduceDifferentIdentities(t *testing.T) {
	server := runServer(t, "guest")
	client := browser(t)

	if _, body := get(t, client, server.URL+pw.DefaultLoginPath); !strings.Contains(body, "guest@example.com") {
		t.Fatalf("guest login = %q", body)
	}
}

func TestEnablingAuthWithoutAProviderIsAStartupError(t *testing.T) {
	// The provider is registered by importing this package, so the negative
	// case is covered by pw itself; here we prove the registered one refuses a
	// mode it does not implement.
	if _, err := auth.New(pw.AuthConfig{Enabled: true, Mode: pw.AuthModePasskey}, pw.SessionConfig{}); err == nil {
		t.Fatal("expected passkey_only to be rejected")
	}
	if _, err := auth.New(pw.AuthConfig{Enabled: true, Mode: pw.AuthModeOIDC}, pw.SessionConfig{}); err == nil {
		t.Fatal("expected a missing session secret to be rejected")
	}
}

// recordRedirects follows redirects while remembering every hop, so a test can
// assert that logout actually visited the provider.
func recordRedirects(t *testing.T, client *http.Client) *[]string {
	t.Helper()
	visited := &[]string{}
	client.CheckRedirect = func(request *http.Request, via []*http.Request) error {
		*visited = append(*visited, request.URL.String())
		if len(via) > 10 {
			return http.ErrUseLastResponse
		}
		return nil
	}
	return visited
}

func TestLogoutEndsTheProviderSessionToo(t *testing.T) {
	server := runServer(t, "admin")
	client := browser(t)
	get(t, client, server.URL+pw.DefaultLoginPath)

	visited := recordRedirects(t, client)
	response, err := client.PostForm(server.URL+pw.DefaultLogoutPath, url.Values{}) //nolint:noctx // loopback test server
	if err != nil {
		t.Fatalf("logout: %v", err)
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(response.Body)
	if string(body) != "anonymous" {
		t.Fatalf("after logout = %q", body)
	}

	issuer := server.IdPInfo().Issuer
	var endSession *url.URL
	for _, hop := range *visited {
		parsed, err := url.Parse(hop)
		if err != nil {
			t.Fatalf("parse %q: %v", hop, err)
		}
		if strings.HasPrefix(hop, issuer) && parsed.Path == "/end_session" {
			endSession = parsed
		}
	}
	if endSession == nil {
		t.Fatalf("logout never reached the provider: %v", *visited)
	}
	query := endSession.Query()
	if query.Get("client_id") != server.IdPInfo().ClientID {
		t.Fatalf("client_id = %q", query.Get("client_id"))
	}
	// The hint is what lets the provider end the right session instead of
	// guessing from a cookie it may not have.
	if hint := query.Get("id_token_hint"); strings.Count(hint, ".") != 2 {
		t.Fatalf("id_token_hint = %q", hint)
	}
	if redirect := query.Get("post_logout_redirect_uri"); !strings.HasPrefix(redirect, server.URL) {
		t.Fatalf("post_logout_redirect_uri = %q", redirect)
	}
}

func TestProviderLogoutCanBeTurnedOff(t *testing.T) {
	server := runServerWithProviderLogout(t, "admin", false)
	client := browser(t)
	get(t, client, server.URL+pw.DefaultLoginPath)

	visited := recordRedirects(t, client)
	response, err := client.PostForm(server.URL+pw.DefaultLogoutPath, url.Values{}) //nolint:noctx // loopback test server
	if err != nil {
		t.Fatalf("logout: %v", err)
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(response.Body)
	if string(body) != "anonymous" {
		t.Fatalf("after logout = %q", body)
	}
	for _, hop := range *visited {
		if strings.HasPrefix(hop, server.IdPInfo().Issuer) {
			t.Fatalf("logout reached the provider despite provider_logout=false: %v", *visited)
		}
	}
}

func TestLogoutWithoutASessionStillSignsOutLocally(t *testing.T) {
	server := runServer(t, "admin")
	client := browser(t)

	// No login first: there is no ID Token to hint with, and the endpoint must
	// still answer rather than fail.
	response, err := client.PostForm(server.URL+pw.DefaultLogoutPath, url.Values{}) //nolint:noctx // loopback test server
	if err != nil {
		t.Fatalf("logout: %v", err)
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(response.Body)
	if response.StatusCode != http.StatusOK || string(body) != "anonymous" {
		t.Fatalf("status = %d, body = %q", response.StatusCode, body)
	}
}
