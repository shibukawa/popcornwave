package passkeyonlye2e

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

type deployment struct {
	origin string
}

var (
	once    sync.Once
	shared  *deployment
	buildEr error
)

// start builds the one passkey_only deployment this binary may have.
func start(t *testing.T) *deployment {
	t.Helper()
	once.Do(func() { shared, buildEr = build() })
	if buildEr != nil {
		t.Fatalf("deployment: %v", buildEr)
	}
	return shared
}

func build() (*deployment, error) {
	directory, err := os.MkdirTemp("", "passkeyonly")
	if err != nil {
		return nil, err
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
	origin := strings.Replace(app.URL, "127.0.0.1", "localhost", 1)

	database := filepath.ToSlash(filepath.Join(directory, "passkeyonly.db"))
	if err := applyFrameworkMigrations(directory, database); err != nil {
		return nil, err
	}
	configPath, err := writeConfig(directory, origin, database)
	if err != nil {
		return nil, err
	}
	pw.SetConfigLoadOptions(configbind.LoadOptions{
		Vendor:             "popcornwave-passkeyonly-e2e",
		Tool:               "passkeyonly-e2e",
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
	return mux
}

// browser drives the ceremony endpoints the way a browser does.
type browser struct {
	t      *testing.T
	client *http.Client
	base   string
}

func newBrowser(t *testing.T) *browser {
	t.Helper()
	deployment := start(t)
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	return &browser{t: t, client: &http.Client{Jar: jar}, base: deployment.origin}
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

func (b *browser) authenticator() *passkeytest.Authenticator {
	b.t.Helper()
	authenticator, err := passkeytest.NewAuthenticator(passkeytest.WithOrigin(b.base))
	if err != nil {
		b.t.Fatal(err)
	}
	return authenticator
}

// issue provisions one account and hands back the login ID and the raw secret,
// which is what an administrator delivers out of band.
func issue(t *testing.T, loginID, accountID string) string {
	t.Helper()
	start(t)
	secret, err := auth.IssueBootstrapCredential(context.Background(), loginID, accountID, auth.PurposeInitialPasskey)
	if err != nil {
		t.Fatalf("IssueBootstrapCredential: %v", err)
	}
	return secret
}

type redemption struct {
	LoginID string `json:"login_id"`
	Secret  string `json:"secret"`
}

func applyFrameworkMigrations(directory, database string) error {
	migrations := filepath.Join(directory, "migrations")
	if err := os.MkdirAll(migrations, 0o750); err != nil {
		return err
	}
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

func writeConfig(directory, appURL, database string) (string, error) {
	content := fmt.Sprintf(`
[server]
public.enabled = false

[middleware.rdb]
enabled = true
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
mode = "passkey_only"
post_login_path = "/"
recent_auth_max_age = "5m"
registration.policy = "administrator"
recovery.policy = "administrator"
bootstrap.issue_ttl = "24h"
bootstrap.enrollment_ttl = "10m"
bootstrap.max_attempts = 3

[auth.passkey]
rp_id = "localhost"
rp_name = "Passkey Only"
origins = ["%s"]
user_verification = "required"
discoverable = "preferred"
`, database, appURL)
	path := filepath.Join(directory, "config.toml")
	return path, os.WriteFile(path, []byte(content), 0o600)
}
