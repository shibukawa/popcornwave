package handlers

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/shibukawa/popcornwave/plugin/auth"
	"github.com/shibukawa/popcornwave/pw"
	"github.com/shibukawa/popcornwave/testutil"
	_ "oidclogin"
	_ "oidclogin/templates"

	// Storage is opt-in by blank import. main.go carries these for the binary;
	// a test builds its own binary and has to link them itself.
	_ "github.com/shibukawa/popcornwave/authstate/sqlite"
	_ "github.com/shibukawa/popcornwave/database/sqlite"
	_ "github.com/shibukawa/popcornwave/sessionstore/sqlite"
)

// TestLoginProvisionsAnAccountAndSignsOut drives the framework endpoints
// against the same development identity provider pw dev starts. The
// application under test registers only its account resolver.
func TestLoginProvisionsAnAccountAndSignsOut(t *testing.T) {
	RegisterAccountResolver()
	// The redirect URI is fixed at client construction, so the test server
	// needs a port it can name in advance.
	port := reservePort(t)
	server := testutil.TestRun(t, Handlers(), func(config *testutil.Config) {
		testutil.Update[pw.ServerConfig](config, func(server *pw.ServerConfig) {
			server.Port = port
			server.Public.Enabled = false
		})
		testutil.Update[pw.MiddlewareConfig](config, func(middleware *pw.MiddlewareConfig) {
			middleware.RDB = pw.RDBConfig{
				Enabled: true,
				Connections: []pw.RDBConnectionConfig{{
					DSN:            "sqlite://:memory:",
					ConnectTimeout: time.Second,
					MaxOpenConns:   1,
					MaxIdleConns:   1,
				}},
			}
		})
		testutil.Update[pw.SessionConfig](config, func(session *pw.SessionConfig) {
			session.Enabled = true
			session.Backend = "rdb"
			session.TTL = time.Hour
			session.Cookie.Secure = false
		})
		testutil.Update[auth.Config](config, func(settings *auth.Config) {
			settings.Enabled = true
			settings.Mode = "oidc_only"
			settings.PostLoginPath = "/"
			settings.Protection.Include = []string{"/mypage"}
			settings.Protection.Unauthenticated = "redirect"
			settings.OIDC.RedirectURL = fmt.Sprintf("http://127.0.0.1:%d/auth/callback", port)
			settings.OIDC.Scopes = []string{"profile", "email"}
			settings.OIDC.IdentityClaim = "employee_number"
			settings.OIDC.Admission = "authenticated"
			settings.OIDC.AutoProvision = true
			settings.OIDC.ProviderLogout = true
			settings.OIDC.AllowLoopbackHTTP = true
		})
	}, testutil.WithMigrations("../migrations"), testutil.WithIdentityProvider(
		testutil.WithIdPConfig("../devidp.toml"),
		testutil.WithLoginUser("hanako"),
		testutil.WithIdPBinding(func(config *testutil.Config, idp testutil.IdPInfo) {
			testutil.Update[auth.Config](config, func(settings *auth.Config) {
				settings.OIDC.Issuer = idp.Issuer
				settings.OIDC.ClientID = idp.ClientID
				settings.OIDC.ClientSecret = idp.ClientSecret
			})
		}),
	))

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Jar: jar}

	// The guard sends an anonymous request through the login first, and the
	// pre-selected roster user completes it without a browser.
	body := fetch(t, client, server.URL+"/mypage")
	for _, fragment := range []string{"Hanako Yamada", "employee_number", "EMP-0001"} {
		if !strings.Contains(body, fragment) {
			t.Fatalf("protected page is missing %q: %s", fragment, body)
		}
	}

	// A browser form post carries Origin, and the endpoint requires it.
	request, err := http.NewRequestWithContext(t.Context(), http.MethodPost, server.URL+"/auth/logout",
		strings.NewReader(url.Values{}.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Origin", server.URL)
	var visited []string
	client.CheckRedirect = func(request *http.Request, via []*http.Request) error {
		visited = append(visited, request.URL.String())
		return nil
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if _, err := io.ReadAll(response.Body); err != nil {
		t.Fatal(err)
	}
	if response.Request.URL.Path != "/" {
		t.Fatalf("logout landed on %q", response.Request.URL)
	}
	// Logging out ends the provider session as well, so the browser passes
	// through the provider on its way back. Without that hop the provider
	// stays signed in and the next login returns the same user silently.
	issuer := server.IdPInfo().Issuer
	endSession := false
	for _, hop := range visited {
		if strings.HasPrefix(hop, issuer+"/end_session") {
			endSession = true
		}
	}
	if !endSession {
		t.Fatalf("logout never reached the provider: %v", visited)
	}

	// The session is gone: the protected page is only reachable by logging in
	// again, which the pre-selected user does silently.
	if body := fetch(t, client, server.URL+"/"); strings.Contains(body, "Hanako Yamada") {
		t.Fatalf("the session survived the logout: %s", body)
	}
}

// reservePort returns a loopback port that is free right now.
func reservePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	return port
}

func fetch(t *testing.T, client *http.Client, target string) string {
	t.Helper()
	response, err := client.Get(target) //nolint:noctx // loopback test server
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("GET %s: %d %s", target, response.StatusCode, body)
	}
	return string(body)
}
