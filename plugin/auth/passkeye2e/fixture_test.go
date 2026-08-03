package passkeye2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/shibukawa/popcornwave/contrib/devidp"
	"github.com/shibukawa/popcornwave/contrib/passkey/passkeytest"
	"github.com/shibukawa/popcornwave/internal/pwmigrate"
	"github.com/shibukawa/popcornwave/plugin/auth"
	"github.com/shibukawa/popcornwave/pw"
	"github.com/shibukawa/popcornwave/sessionstore"
	_ "github.com/shibukawa/popcornwave/sessionstore/sqlite"
	"github.com/shibukawa/tinybind-go/configbind"

	// Storage is opt-in by blank import: the sessions, the single-use
	// ceremony records, and the driver the DSN names.
	_ "github.com/shibukawa/popcornwave/authstate/sqlite"
	_ "github.com/shibukawa/tinygodriver/database/sql/sqlite"
)

// deployment is the one oidc_passkey application this test binary may build.
type deployment struct {
	origin string
}

var (
	once    sync.Once
	shared  *deployment
	buildEr error
)

// start builds the deployment on first use. Every test shares it, because
// framework configuration is parsed once per process.
func start(t *testing.T) *deployment {
	t.Helper()
	once.Do(func() { shared, buildEr = build() })
	if buildEr != nil {
		t.Fatalf("deployment: %v", buildEr)
	}
	return shared
}

func build() (*deployment, error) {
	directory, err := os.MkdirTemp("", "passkeye2e")
	if err != nil {
		return nil, err
	}
	provider, err := devidp.Start(context.Background(), "127.0.0.1:0", devidp.Config{
		Users: []devidp.User{{
			Key: "alice", Subject: "alice-subject", DisplayName: "Alice Example",
			Claims: map[string]any{"email": "alice@example.com"},
		}},
	}, devidp.Options{LoginUser: "alice-subject"})
	if err != nil {
		return nil, fmt.Errorf("identity provider: %w", err)
	}

	var handler http.Handler
	var handlerMu sync.RWMutex
	app := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerMu.RLock()
		current := handler
		handlerMu.RUnlock()
		if current == nil {
			http.Error(w, "not ready", http.StatusServiceUnavailable)
			return
		}
		current.ServeHTTP(w, r)
	}))
	// An RP ID is a domain, so the deployment reaches its own listener by name.
	// The 127.0.0.1 that httptest reports could never be one.
	origin := strings.Replace(app.URL, "127.0.0.1", "localhost", 1)

	database := filepath.ToSlash(filepath.Join(directory, "passkey.db"))
	if err := applyFrameworkMigrations(directory, database); err != nil {
		return nil, err
	}
	credentials, err := provider.RegisterClient(devidp.ClientSpec{LoopbackRedirects: true})
	if err != nil {
		return nil, err
	}
	configPath, err := writeConfig(directory, provider.Issuer(), origin, database, credentials)
	if err != nil {
		return nil, err
	}
	pw.SetConfigLoadOptions(configbind.LoadOptions{
		Vendor:             "popcornwave-passkey-e2e",
		Tool:               "passkey-e2e",
		ExplicitConfigPath: configPath,
		Args:               []string{},
		Environ:            []string{},
	})
	built, err := pw.Middlewares(application())
	if err != nil {
		return nil, fmt.Errorf("framework initialization: %w", err)
	}
	handlerMu.Lock()
	handler = built
	handlerMu.Unlock()
	return &deployment{origin: origin}, nil
}

// application reports which method authenticated the request, so a test can
// tell an OIDC session from a passkey session.
func application() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /whoami", func(w http.ResponseWriter, r *http.Request) {
		authentication := pw.RequestAuthentication(r.Context())
		if !authentication.Authenticated {
			_, _ = fmt.Fprint(w, "anonymous")
			return
		}
		user, _ := auth.User(r.Context())
		_, _ = fmt.Fprintf(w, "%s:%s", authentication.Method, user.AccountID)
	})
	// A path behind policy:authenticated-path-protection, so a test can prove
	// that a seam reaches past the real guard rather than around it.
	mux.HandleFunc("GET /private", func(w http.ResponseWriter, r *http.Request) {
		user, _ := auth.User(r.Context())
		_, _ = fmt.Fprintf(w, "private:%s", user.AccountID)
	})
	return mux
}

// browser drives the ceremony endpoints the way a browser does: same origin,
// JSON bodies, and a cookie jar carrying the session and the single-use
// ceremony cookie.
type browser struct {
	t      *testing.T
	client *http.Client
	base   string
}

// newBrowser returns an anonymous browser against the shared deployment. Each
// test gets its own jar, so sessions do not leak between tests.
func newBrowser(t *testing.T) *browser {
	t.Helper()
	deployment := start(t)
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	return &browser{t: t, client: &http.Client{Jar: jar}, base: deployment.origin}
}

// newLoggedInBrowser returns a browser holding a session created through OIDC,
// which is how an oidc_passkey account exists before it enrolls anything.
func newLoggedInBrowser(t *testing.T) (*browser, *passkeytest.Authenticator) {
	t.Helper()
	b := newBrowser(t)
	response, err := b.client.Get(b.base + "/auth/login")
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if identity := b.whoami(); !strings.HasPrefix(identity, "oidc:") {
		t.Fatalf("fixture login = %q, want an oidc session", identity)
	}
	authenticator, err := passkeytest.NewAuthenticator(passkeytest.WithOrigin(b.base))
	if err != nil {
		t.Fatal(err)
	}
	return b, authenticator
}

func (b *browser) post(path string, body any) (*http.Response, []byte) {
	b.t.Helper()
	var encoded []byte
	if body != nil {
		var err error
		if encoded, err = json.Marshal(body); err != nil {
			b.t.Fatal(err)
		}
	}
	request, err := http.NewRequest(http.MethodPost, b.base+path, bytes.NewReader(encoded))
	if err != nil {
		b.t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	// A browser sets this on every state-changing fetch, and the endpoint
	// refuses the request without a matching one.
	request.Header.Set("Origin", b.base)
	return b.do(request)
}

func (b *browser) do(request *http.Request) (*http.Response, []byte) {
	b.t.Helper()
	response, err := b.client.Do(request)
	if err != nil {
		b.t.Fatal(err)
	}
	defer response.Body.Close()
	payload := new(bytes.Buffer)
	if _, err := payload.ReadFrom(response.Body); err != nil {
		b.t.Fatal(err)
	}
	return response, payload.Bytes()
}

func (b *browser) postExpecting(path string, body any, want int) []byte {
	b.t.Helper()
	response, payload := b.post(path, body)
	if response.StatusCode != want {
		b.t.Fatalf("POST %s status = %d, want %d; body %q", path, response.StatusCode, want, payload)
	}
	return payload
}

func (b *browser) postJSON(path string, body any, target any) {
	b.t.Helper()
	payload := b.postExpecting(path, body, http.StatusOK)
	if err := json.Unmarshal(payload, target); err != nil {
		b.t.Fatalf("POST %s returned %q: %v", path, payload, err)
	}
}

func (b *browser) whoami() string {
	b.t.Helper()
	response, err := b.client.Get(b.base + "/whoami")
	if err != nil {
		b.t.Fatal(err)
	}
	defer response.Body.Close()
	payload := new(bytes.Buffer)
	if _, err := payload.ReadFrom(response.Body); err != nil {
		b.t.Fatal(err)
	}
	return payload.String()
}

func (b *browser) logout() {
	b.t.Helper()
	request, err := http.NewRequest(http.MethodPost, b.base+"/auth/logout", nil)
	if err != nil {
		b.t.Fatal(err)
	}
	request.Header.Set("Origin", b.base)
	b.do(request)
}

func applyFrameworkMigrations(directory, database string) error {
	migrations := filepath.Join(directory, "migrations")
	if err := os.MkdirAll(migrations, 0o750); err != nil {
		return err
	}
	// The versions here are this fixture's, not the packages': a project picks
	// whatever is free when the file is written.
	sessionMigration, err := sessionstore.MigrationSQL("sqlite", "popcornwave_session")
	if err != nil {
		return err
	}
	authMigration, err := auth.MigrationSQL("sqlite")
	if err != nil {
		return err
	}
	for version, migration := range map[int]struct{ name, content string }{
		1: {sessionstore.MigrationName, sessionMigration},
		2: {auth.MigrationName, authMigration},
	} {
		name := fmt.Sprintf("%05d_%s.sql", version, migration.name)
		if err := os.WriteFile(filepath.Join(migrations, name), []byte(migration.content), 0o600); err != nil {
			return err
		}
	}
	sources, err := pwmigrate.Sources(migrations)
	if err != nil {
		return err
	}
	target, err := pwmigrate.Open("sqlite://" + database)
	if err != nil {
		return err
	}
	defer target.Close()
	_, err = pwmigrate.Apply(context.Background(), target, sources, pwmigrate.ActionUp, 0)
	return err
}

func writeConfig(directory, issuer, appURL, database string, credentials devidp.Credentials) (string, error) {
	content := fmt.Sprintf(`
[server]
public.enabled = false

[middleware.rdb]
enabled = true

[[middleware.rdb.connections]]
group = "default"
dsn = "sqlite://%s"
connect_timeout = "5s"
max_open_conns = 1
max_idle_conns = 1

[session]
enabled = true
backend = "rdb"
ttl = "1h"
idle_timeout = "30m"
cookie.name = "pw_session"
cookie.secure = false

[auth]
enabled = true
mode = "oidc_passkey"
post_login_path = "/"
recent_auth_max_age = "5m"
recovery.policy = "oidc"
protection.include = ["/private"]
protection.unauthenticated = "unauthorized"

[auth.passkey]
rp_id = "localhost"
rp_name = "Passkey End To End"
origins = ["%s"]
user_verification = "required"
discoverable = "preferred"

[auth.oidc]
issuer = "%s"
client_id = "%s"
client_secret = "%s"
redirect_url = "%s/auth/callback"
admission = "authenticated"
allow_loopback_http = true
`, database, appURL, issuer, credentials.ID, credentials.Secret, appURL)
	path := filepath.Join(directory, "config.toml")
	return path, os.WriteFile(path, []byte(content), 0o600)
}
