package pwcli

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/shibukawa/popcornwave/internal/pwenv"
	"github.com/shibukawa/popcornwave/internal/pwmigrate"
	"github.com/shibukawa/popcornwave/plugin/auth"
	"github.com/shibukawa/popcornwave/plugin/session/rdb"
)

func TestParseInitArgsRejectsAnUnknownAuthMode(t *testing.T) {
	if _, err := parseInitArgs([]string{"demo", "--auth=basic"}); err == nil ||
		!strings.Contains(err.Error(), "--auth must be") {
		t.Fatalf("err = %v", err)
	}
}

func TestScaffoldWiresTheLocalEmulator(t *testing.T) {
	files := scaffoldFiles(initOptions{Name: "demo", TinyGo: true, Auth: authOIDC, AuthEmulator: true})

	project := files["popcornwave.toml"]
	if !strings.Contains(project, "[dev.idp]") || !strings.Contains(project, "enabled = true") {
		t.Fatalf("popcornwave.toml = %q", project)
	}
	roster, ok := files[defaultIdPConfig]
	if !ok {
		t.Fatalf("%s was not scaffolded", defaultIdPConfig)
	}
	if !strings.Contains(roster, "[users.admin]") {
		t.Fatalf("roster = %q", roster)
	}
	config := files[pwenv.FileName(pwenv.Development)]
	if !strings.Contains(config, `mode = "oidc_only"`) {
		t.Fatalf("config = %q", config)
	}
	// pw dev supplies the provider values, so the committed file must not
	// carry an issuer or a client credential at all.
	for _, key := range []string{"issuer =", "client_id =", "client_secret ="} {
		if strings.Contains(config, key) {
			t.Fatalf("config declares %s even though pw dev injects it:\n%s", key, config)
		}
	}
	for _, expected := range []string{"AUTH_OIDC_ISSUER", "allow_loopback_http = true"} {
		if !strings.Contains(config, expected) {
			t.Fatalf("config is missing %s:\n%s", expected, config)
		}
	}
}

func TestScaffoldWiresAnExternalProvider(t *testing.T) {
	files := scaffoldFiles(initOptions{Name: "demo", TinyGo: true, Auth: authOIDC})

	if strings.Contains(files["popcornwave.toml"], "[dev.idp]") {
		t.Fatal("an external provider must not enable the development identity provider")
	}
	if _, ok := files[defaultIdPConfig]; ok {
		t.Fatalf("%s must not be scaffolded for an external provider", defaultIdPConfig)
	}
	config := files[pwenv.FileName(pwenv.Development)]
	for _, expected := range []string{
		`issuer = ""`, `client_id = ""`, `client_secret = ""`, "allow_loopback_http = false",
	} {
		if !strings.Contains(config, expected) {
			t.Fatalf("config is missing %s:\n%s", expected, config)
		}
	}
}

func TestScaffoldWiresTheFrameworkOwnedEndpoints(t *testing.T) {
	files := scaffoldFiles(initOptions{Name: "demo", TinyGo: true, Auth: authOIDC, AuthEmulator: true})

	// Registering the resolver is the whole application-side wiring: it also
	// imports plugin/auth, whose extensions serve the endpoints.
	main := files["cmd/demo/main.go"]
	if !strings.Contains(main, "handlers.RegisterAccountResolver()") {
		t.Fatalf("main.go does not install the account resolver:\n%s", main)
	}
	resolver, ok := files["handlers/accounts.go"]
	if !ok || !strings.Contains(resolver, "auth.SetAccountResolver(resolveAccount)") {
		t.Fatalf("accounts.go = %q", resolver)
	}
	// The framework tables come from the packages that own them.
	if files["migrations/00002_"+rdb.MigrationName+".sql"] != rdb.MigrationSQL("popcornwave_session") {
		t.Fatal("the scaffolded session migration is not the one plugin/session/rdb publishes")
	}
	if files["migrations/00003_"+auth.MigrationName+".sql"] != auth.MigrationSQL() {
		t.Fatal("the scaffolded auth migration is not the one plugin/auth publishes")
	}

	config := files[pwenv.FileName(pwenv.Development)]
	for _, expected := range []string{
		"[session]", `backend = "rdb"`, "cookie.secure = false", `rdb.source = "middleware"`,
		`post_login_path = "/"`, "protection.unauthenticated", "provider_logout = true",
	} {
		if !strings.Contains(config, expected) {
			t.Fatalf("config is missing %s:\n%s", expected, config)
		}
	}
	// The starter page reads the account and signs out with a POST form.
	handler := files["handlers/home_handler.go"]
	if !strings.Contains(handler, "auth.User(r.Context())") {
		t.Fatalf("home handler does not read the session:\n%s", handler)
	}
	template := files["handlers/home.pw.html"]
	if !strings.Contains(template, `<form method="post" action={logoutPath}>`) {
		t.Fatalf("logout must be a POST form:\n%s", template)
	}
	if !strings.Contains(template, `<a href={loginPath}>`) {
		t.Fatalf("no login link:\n%s", template)
	}
}

// The scaffolded keys must be the ones the plugins register, otherwise the
// generated project fails to start on an unknown key.
func TestScaffoldedAuthKeysAreRegistered(t *testing.T) {
	known := map[string]bool{
		// [session]
		"enabled": true, "backend": true, "ttl": true, "idle_timeout": true,
		"cookie.name": true, "cookie.secure": true, "rdb.source": true,
		// [auth]
		"mode": true, "post_login_path": true,
		"protection.include": true, "protection.unauthenticated": true,
		// [auth.oidc]
		"issuer": true, "client_id": true, "client_secret": true,
		"redirect_url": true, "scopes": true, "identity_claim": true,
		"admission": true, "auto_provision": true,
		"provider_logout": true, "allow_loopback_http": true,
	}
	config := scaffoldFiles(initOptions{Name: "demo", Auth: authOIDC})[pwenv.FileName(pwenv.Development)]
	section := config[strings.Index(config, "[session]"):]
	for _, line := range strings.Split(section, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "[") {
			continue
		}
		key, _, found := strings.Cut(line, "=")
		if !found {
			t.Fatalf("unparsable line %q", line)
		}
		if !known[strings.TrimSpace(key)] {
			t.Fatalf("scaffolded key %q is not registered by the plugins", strings.TrimSpace(key))
		}
	}
}

func TestScaffoldRecordsPasskeyOnlyWithoutEnablingIt(t *testing.T) {
	files := scaffoldFiles(initOptions{Name: "demo", Auth: authPasskey})
	config := files[pwenv.FileName(pwenv.Development)]
	// plugin/auth rejects the mode, so an enabled section would fail at
	// startup. The choice is recorded as a comment instead.
	if !strings.Contains(config, `# mode = "passkey_only"`) {
		t.Fatalf("config = %q", config)
	}
	if _, ok := files["handlers/accounts.go"]; ok {
		t.Fatal("a mode with no implementation must not scaffold a resolver")
	}
	if _, ok := files[defaultIdPConfig]; ok {
		t.Fatal("passkey-only must not scaffold an identity provider roster")
	}
}

func TestScaffoldWritesNoAuthSectionForNone(t *testing.T) {
	files := scaffoldFiles(initOptions{Name: "demo", Auth: authNone})
	if strings.Contains(files[pwenv.FileName(pwenv.Development)], "[auth]") {
		t.Fatal("auth none must not write an [auth] section")
	}
	if _, ok := files["handlers/accounts.go"]; ok {
		t.Fatal("auth none must not scaffold a resolver")
	}
}

func TestInitWizardAsksForTheProviderOnlyForOIDC(t *testing.T) {
	t.Chdir(t.TempDir())
	model := feedWizard(t, newTestWizard(defaultInitOptions()),
		typeText("demo"), pressKey(tea.KeyEnter),
		pressKey(tea.KeyEnter), // TinyGo
		pressKey(tea.KeyEnter), // Tailwind
		pressKey(tea.KeyEnter), // Database
		typeText("2"),          // Authentication: OIDC
	)
	if model.reviewing() {
		t.Fatal("choosing OIDC must ask where the session lives")
	}
	// A login asks for its storage first: it is the answer the provider
	// question has no bearing on.
	if label := model.steps[model.index].label(); label != "Session storage" {
		t.Fatalf("step = %q", label)
	}
	model = feedWizard(t, model, typeText("1")) // Session storage: Database
	if label := model.steps[model.index].label(); label != "OIDC provider" {
		t.Fatalf("step = %q", label)
	}
	model = feedWizard(t, model, typeText("1")) // Local emulator
	options := wizardResult(model, defaultInitOptions())
	if options.Auth != authOIDC || !options.AuthEmulator {
		t.Fatalf("options = %#v", options)
	}
}

func TestInitWizardSkipsTheProviderStepWithoutOIDC(t *testing.T) {
	t.Chdir(t.TempDir())
	seeded := initOptions{TinyGo: true, Devbox: true, Database: true, Redis: true, Auth: authOIDC, AuthEmulator: true}
	model := feedWizard(t, newTestWizard(seeded),
		typeText("demo"), pressKey(tea.KeyEnter),
		pressKey(tea.KeyEnter), // TinyGo
		pressKey(tea.KeyEnter), // Tailwind
		pressKey(tea.KeyEnter), // Database
		typeText("3"),          // Authentication: Passkey only
	)
	// The provider question is skipped; the environment questions still follow.
	if label := model.steps[model.index].label(); label != "Devbox environment" {
		t.Fatalf("step = %q, want the provider question skipped", label)
	}
	// The seeded emulator answer must not survive a mode that has no provider.
	options := wizardResult(model, seeded)
	if options.Auth != authPasskey || options.AuthEmulator {
		t.Fatalf("options = %#v", options)
	}
}

func TestInitWizardGoesBackPastASkippedStep(t *testing.T) {
	t.Chdir(t.TempDir())
	model := feedWizard(t, newTestWizard(defaultInitOptions()),
		typeText("demo"), pressKey(tea.KeyEnter),
		pressKey(tea.KeyEnter), // TinyGo
		pressKey(tea.KeyEnter), // Tailwind
		pressKey(tea.KeyEnter), // Database
		pressKey(tea.KeyEnter), // Authentication: None
		pressKey(tea.KeyEnter), // Devbox
		pressKey(tea.KeyEnter), // Redis or Valkey
		pressKey(tea.KeyEsc),   // back from the review screen
	)
	if label := model.steps[model.index].label(); label != "Redis or Valkey" {
		t.Fatalf("esc landed on %q, want the last asked question", label)
	}
}

// TestScaffoldedMigrationsApply guards the version ranges of the scaffold. The
// framework packages own everything below 00010, so an application migration
// that reused one of those versions made a fresh project fail its very first
// pw migrate up with a duplicate version.
func TestScaffoldedMigrationsApply(t *testing.T) {
	for _, mode := range []string{authNone, authOIDC} {
		files := scaffoldFiles(initOptions{Name: "demo", TinyGo: true, Database: true, Auth: mode, AuthEmulator: usesOIDC(mode)})
		directory := t.TempDir()
		for path, content := range files {
			name, ok := strings.CutPrefix(path, "migrations/")
			if !ok {
				continue
			}
			if err := os.WriteFile(filepath.Join(directory, name), []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		sources, err := pwmigrate.Sources(directory)
		if err != nil {
			t.Fatalf("auth=%s: %v", mode, err)
		}
		target, err := pwmigrate.Open("sqlite://" + filepath.Join(t.TempDir(), "scaffold.db"))
		if err != nil {
			t.Fatalf("auth=%s: %v", mode, err)
		}
		result, err := pwmigrate.Apply(t.Context(), target, sources, pwmigrate.ActionUp, 0)
		_ = target.Close()
		if err != nil {
			t.Fatalf("auth=%s: %v", mode, err)
		}
		if result.Current == 0 {
			t.Fatalf("auth=%s applied no migration", mode)
		}
	}
}

// The session backend decides three scaffold outputs at once: the import that
// contributes it, the configuration keys that describe it, and whether a
// migration is needed at all.
func TestScaffoldFollowsTheSessionBackendChoice(t *testing.T) {
	rdbFiles := scaffoldFiles(initOptions{Name: "demo", Database: true, Auth: authOIDC, Session: sessionRDB})
	if !strings.Contains(rdbFiles["cmd/demo/main.go"], `_ "github.com/shibukawa/popcornwave/plugin/session/rdb"`) {
		t.Errorf("rdb backend is not imported:\n%s", rdbFiles["cmd/demo/main.go"])
	}
	if !strings.Contains(rdbFiles["config.dev.toml"], `backend = "rdb"`) ||
		!strings.Contains(rdbFiles["config.dev.toml"], `rdb.source = "middleware"`) {
		t.Errorf("rdb session config:\n%s", rdbFiles["config.dev.toml"])
	}
	if _, ok := rdbFiles["migrations/00002_"+rdb.MigrationName+".sql"]; !ok {
		t.Error("rdb backend did not scaffold its session migration")
	}
	if _, ok := rdbFiles["migrations/00003_"+auth.MigrationName+".sql"]; !ok {
		t.Error("auth migration is not numbered after the session one")
	}

	cookieFiles := scaffoldFiles(initOptions{Name: "demo", Database: true, Auth: authOIDC, Session: sessionCookie})
	if strings.Contains(cookieFiles["cmd/demo/main.go"], "plugin/session/") {
		t.Errorf("the built-in cookie backend was imported:\n%s", cookieFiles["cmd/demo/main.go"])
	}
	if !strings.Contains(cookieFiles["config.dev.toml"], `cookie_store.secret = "${SESSION_COOKIE_SECRET}"`) {
		t.Errorf("cookie session config:\n%s", cookieFiles["config.dev.toml"])
	}
	if strings.Contains(cookieFiles["config.dev.toml"], "rdb.source") {
		t.Error("cookie backend wrote keys of a backend it does not use")
	}
	// No session table exists, so the auth migration takes the free version.
	if _, ok := cookieFiles["migrations/00002_"+auth.MigrationName+".sql"]; !ok {
		t.Errorf("auth migration was not renumbered: %v", migrationNames(cookieFiles))
	}

	redisFiles := scaffoldFiles(initOptions{Name: "demo", Database: true, Auth: authOIDC, Session: sessionRedis})
	if !strings.Contains(redisFiles["cmd/demo/main.go"], `_ "github.com/shibukawa/popcornwave/plugin/session/redis"`) {
		t.Errorf("redis backend is not imported:\n%s", redisFiles["cmd/demo/main.go"])
	}
	if !strings.Contains(redisFiles["config.dev.toml"], "redis.dsn") {
		t.Errorf("redis session config:\n%s", redisFiles["config.dev.toml"])
	}
	if _, ok := redisFiles["migrations/00002_"+auth.MigrationName+".sql"]; !ok {
		t.Errorf("auth migration was not renumbered: %v", migrationNames(redisFiles))
	}
}

// A Redis-backed session brings the server that serves it into the development
// environment, rather than leaving a project that cannot start.
func TestRedisSessionTakesTheDevelopmentServer(t *testing.T) {
	options, err := parseInitArgs([]string{"demo", "--auth=oidc", "--session=redis", "--no-redis"})
	if err != nil {
		t.Fatal(err)
	}
	if !options.Redis {
		t.Fatalf("options = %#v", options)
	}
	if _, err := parseInitArgs([]string{"demo", "--session=memcached"}); err == nil {
		t.Fatal("an unknown backend was accepted")
	}
}

func migrationNames(files map[string]string) []string {
	var names []string
	for path := range files {
		if strings.HasPrefix(path, "migrations/") {
			names = append(names, path)
		}
	}
	sort.Strings(names)
	return names
}
