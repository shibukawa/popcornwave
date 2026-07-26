package handlers

import (
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"testing"
	"time"

	// The blank import is the whole authentication integration, exactly as in
	// cmd/helloworld/main.go.
	_ "github.com/shibukawa/popcornwave/auth"
	"github.com/shibukawa/popcornwave/pw"
	"github.com/shibukawa/popcornwave/testutil"
	_ "helloworld"
	_ "helloworld/templates"
)

// TestLoginSignsInAndOut drives the framework-owned endpoints against the
// development identity provider. The application under test registers no
// authentication route.
func TestLoginSignsInAndOut(t *testing.T) {
	server := testutil.TestRun(t, Handlers(), func(config *testutil.Config) {
		testutil.Update[pw.MiddlewareConfig](config, func(middleware *pw.MiddlewareConfig) {
			middleware.RDB = pw.RDBConfig{
				Enabled:        true,
				DSN:            "sqlite://:memory:",
				ConnectTimeout: time.Second,
				MaxOpenConns:   1,
				MaxIdleConns:   1,
			}
		})
		testutil.Update[pw.SessionConfig](config, func(session *pw.SessionConfig) {
			session.Enabled = true
			session.Secret = "test-session-secret"
		})
	}, testutil.WithMigrations("../migrations"), testutil.WithIdentityProvider(
		testutil.WithIdPConfig("../devidp.toml"),
		testutil.WithLoginUser("admin"),
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
			})
		}),
	))

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Jar: jar}

	if body := fetch(t, client, server.URL+"/"); !strings.Contains(body, "Sign in") {
		t.Fatalf("anonymous page has no sign-in control: %s", body)
	}

	// WithLoginUser pre-selects the roster user, so this single request walks
	// the authorization redirect, the callback, and the session cookie.
	body := fetch(t, client, server.URL+pw.DefaultLoginPath)
	for _, fragment := range []string{"Ada Administrator", "Sign out", `method="post"`} {
		if !strings.Contains(body, fragment) {
			t.Fatalf("signed-in page is missing %q: %s", fragment, body)
		}
	}

	response, err := client.PostForm(server.URL+pw.DefaultLogoutPath, url.Values{}) //nolint:noctx // loopback test server
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	after, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(after), "Sign in") {
		t.Fatalf("page after logout is still signed in: %s", after)
	}
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
