package pwcli

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/shibukawa/popcornwave/internal/pwenv"
	"github.com/shibukawa/popcornwave/internal/pwmigrate"
	"github.com/shibukawa/popcornwave/plugin/auth"
	"github.com/shibukawa/popcornwave/sessionstore"
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

	// Registering the account seams is the whole application-side wiring: it
	// also imports plugin/auth, whose extensions serve the endpoints.
	main := files["cmd/demo/main.go"]
	if !strings.Contains(main, "handlers.RegisterAccounts()") {
		t.Fatalf("main.go does not install the account seams:\n%s", main)
	}
	resolver, ok := files["handlers/accounts.go"]
	if !ok || !strings.Contains(resolver, "auth.SetAccountResolver(resolveAccount)") {
		t.Fatalf("accounts.go = %q", resolver)
	}
	// The framework tables come from the packages that own them.
	if files["migrations/00002_"+sessionstore.MigrationName+".sql"] != mustSessionMigration() {
		t.Fatal("the scaffolded session migration is not the one sessionstore/sqlite publishes")
	}
	if files["migrations/00003_"+auth.MigrationName+".sql"] != mustAuthMigration() {
		t.Fatal("the scaffolded auth migration is not the one plugin/auth publishes")
	}

	config := files[pwenv.FileName(pwenv.Development)]
	for _, expected := range []string{
		"[session]", `backend = "rdb"`, "cookie.secure = false", `rdb.source = "middleware"`,
		`post_login_path = "/"`, "protection.unauthenticated", `logout_scope = "reconfirm"`,
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
	if !strings.Contains(template, `href={loginPath}>Sign in</a>`) {
		t.Fatalf("no login link:\n%s", template)
	}
}

// The scaffolded keys must be registered, otherwise the generated project fails
// to start on an unknown key.
//
// The scan runs from [session] to the end of the file, so it covers every
// section the scaffold appends after that point, not only the auth ones.
func TestScaffoldedAuthKeysAreRegistered(t *testing.T) {
	known := map[string]bool{
		// [session]
		"enabled": true, "backend": true, "ttl": true, "idle_timeout": true,
		"cookie.name": true, "cookie.secure": true, "rdb.source": true,
		// [auth]
		"mode": true, "post_login_path": true,
		"protection.include": true, "protection.unauthenticated": true,
		"recent_auth_max_age": true, "registration.policy": true, "recovery.policy": true,
		"bootstrap.issue_ttl": true, "bootstrap.enrollment_ttl": true, "bootstrap.max_attempts": true,
		// [auth.passkey]
		"rp_id": true, "rp_name": true, "origins": true,
		"user_verification": true, "discoverable": true,
		// [auth.oidc]
		"issuer": true, "client_id": true, "client_secret": true,
		"redirect_url": true, "scopes": true, "identity_claim": true,
		"admission": true, "auto_provision": true,
		"logout_scope": true, "allow_loopback_http": true,
		// [session]
		"keyring.secret": true,
		// [auth], where every session lifetime is declared
		"session.ttl": true, "session.idle_timeout": true,
		// [security]
		"csrf.enabled": true, "csrf.include": true, "csrf.exclude": true,
	}
	for _, mode := range []string{authOIDC, authOIDCPasskey, authPasskey} {
		t.Run(mode, func(t *testing.T) {
			config := scaffoldFiles(initOptions{Name: "demo", Auth: mode})[pwenv.FileName(pwenv.Development)]
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
		})
	}
}

// passkey_only has no provider, so it scaffolds neither an OIDC section nor an
// identity provider roster, and it must choose both lifecycle policies.
func TestScaffoldPasskeyOnly(t *testing.T) {
	files := scaffoldFiles(initOptions{Name: "demo", Auth: authPasskey, AuthEmulator: true})
	config := files[pwenv.FileName(pwenv.Development)]
	for _, want := range []string{
		`mode = "passkey_only"`, `registration.policy = "administrator"`,
		`recovery.policy = "administrator"`, `rp_id = "localhost"`,
	} {
		if !strings.Contains(config, want) {
			t.Fatalf("config is missing %q:\n%s", want, config)
		}
	}
	if strings.Contains(config, "[auth.oidc]") {
		t.Fatal("passkey_only reads no OIDC setting, so the scaffold must write none")
	}
	if _, ok := files[defaultIdPConfig]; ok {
		t.Fatal("passkey-only must not scaffold an identity provider roster")
	}
	accounts := files["handlers/accounts.go"]
	for _, want := range []string{"SetAccountLookup", "SetAccountActivator", "IssueBootstrapCredential"} {
		if !strings.Contains(accounts, want) {
			t.Fatalf("handlers/accounts.go is missing %q:\n%s", want, accounts)
		}
	}
	if !strings.Contains(files["public/passkey.js"], "redeemBootstrap") {
		t.Fatal("passkey_only needs the browser side of the bootstrap redemption")
	}
}

// oidc_passkey serves both, so it carries the provider registration and the
// relying-party registration together.
func TestScaffoldOIDCPasskey(t *testing.T) {
	files := scaffoldFiles(initOptions{Name: "demo", Auth: authOIDCPasskey, AuthEmulator: true})
	config := files[pwenv.FileName(pwenv.Development)]
	for _, want := range []string{
		`mode = "oidc_passkey"`, "[auth.oidc]", "[auth.passkey]",
		`recovery.policy = "oidc"`,
		// The origin has to sit inside the RP ID, and an address cannot be one.
		`redirect_url = "http://localhost:8080/auth/callback"`,
	} {
		if !strings.Contains(config, want) {
			t.Fatalf("config is missing %q:\n%s", want, config)
		}
	}
	if strings.Contains(config, "registration.policy") {
		t.Fatal("oidc_passkey lets registration default to the provider login")
	}
	accounts := files["handlers/accounts.go"]
	if !strings.Contains(accounts, "SetAccountResolver") || !strings.Contains(accounts, "SetAccountLookup") {
		t.Fatalf("oidc_passkey needs both account seams:\n%s", accounts)
	}
	if strings.Contains(accounts, "SetAccountActivator") {
		t.Fatal("only passkey_only activates a provisional account")
	}
	if _, ok := files[defaultIdPConfig]; !ok {
		t.Fatal("oidc_passkey with the emulator needs an identity provider roster")
	}
}

// Only a mode that mounts a ceremony endpoint needs the browser side.
func TestBrowserScaffoldFollowsTheMode(t *testing.T) {
	for mode, want := range map[string]bool{
		authNone: false, authOIDC: false, authOIDCPasskey: true, authPasskey: true,
	} {
		_, ok := scaffoldFiles(initOptions{Name: "demo", Auth: mode})["public/passkey.js"]
		if ok != want {
			t.Fatalf("auth %s scaffolded passkey.js = %v, want %v", mode, ok, want)
		}
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
		pressKey(tea.KeyEnter), // Router
		pressKey(tea.KeyEnter), // Tailwind
		typeText("2"),          // Authentication: OIDC
		typeText("1"),          // Store: SQLite
		pressKey(tea.KeyEnter), // DynamoDB
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

// oidc_passkey still needs a provider, so the wizard keeps asking for one.
func TestInitWizardAsksForTheProviderForOIDCPasskey(t *testing.T) {
	t.Chdir(t.TempDir())
	model := feedWizard(t, newTestWizard(defaultInitOptions()),
		typeText("demo"), pressKey(tea.KeyEnter),
		pressKey(tea.KeyEnter), // TinyGo
		pressKey(tea.KeyEnter), // Router
		pressKey(tea.KeyEnter), // Tailwind
		typeText("3"),          // Authentication: OIDC and passkey
		typeText("1"),          // Store: SQLite
		pressKey(tea.KeyEnter), // DynamoDB
		pressKey(tea.KeyEnter), // Session storage
	)
	if model.reviewing() {
		t.Fatal("the provider step was skipped for a mode that uses one")
	}
	if label := model.steps[model.index].label(); label != "OIDC provider" {
		t.Fatalf("step = %q, want the provider question", label)
	}
}

func TestInitWizardSkipsTheProviderStepWithoutOIDC(t *testing.T) {
	t.Chdir(t.TempDir())
	seeded := initOptions{TinyGo: true, Devbox: true, Database: true, Engine: engineSQLite, Redis: true, Auth: authOIDC, AuthEmulator: true}
	model := feedWizard(t, newTestWizard(seeded),
		typeText("demo"), pressKey(tea.KeyEnter),
		pressKey(tea.KeyEnter), // TinyGo
		pressKey(tea.KeyEnter), // Router
		pressKey(tea.KeyEnter), // Tailwind
		typeText("4"),          // Authentication: Passkey only
		typeText("1"),          // Store: SQLite
		pressKey(tea.KeyEnter), // DynamoDB
		pressKey(tea.KeyEnter), // Session storage
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
		pressKey(tea.KeyEnter), // Router
		pressKey(tea.KeyEnter), // Tailwind
		pressKey(tea.KeyEnter), // Authentication: None
		pressKey(tea.KeyEnter), // Database
		pressKey(tea.KeyEnter), // Database engine
		pressKey(tea.KeyEnter), // DynamoDB
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
// The scaffolded keyring is generated per project rather than shipped in this
// source. A constant here would be a published credential in every project that
// ever ran pw init, which is the thing the placeholder check exists to catch.
func TestTheScaffoldedKeyringIsGeneratedPerProject(t *testing.T) {
	// The scaffold writes [session] alongside [auth] today, so this asks for an
	// authentication mode to get one.
	first := scaffoldFiles(initOptions{Name: "demo", Auth: authOIDC})["config.dev.toml"]
	second := scaffoldFiles(initOptions{Name: "demo", Auth: authOIDC})["config.dev.toml"]
	secretOf := func(config string) string {
		for _, line := range strings.Split(config, "\n") {
			if key, value, found := strings.Cut(strings.TrimSpace(line), "="); found &&
				strings.TrimSpace(key) == "keyring.secret" {
				return strings.Trim(strings.TrimSpace(value), `"`)
			}
		}
		return ""
	}
	one, other := secretOf(first), secretOf(second)
	if one == "" {
		t.Fatalf("no keyring was scaffolded:\n%s", first)
	}
	if one == other {
		t.Error("two projects were scaffolded with the same keyring")
	}
	decoded, err := base64.StdEncoding.DecodeString(one)
	if err != nil {
		t.Fatalf("the scaffolded keyring is not base64: %v", err)
	}
	if len(decoded) < 32 {
		t.Errorf("the scaffolded keyring is %d bytes, want at least 32", len(decoded))
	}
}

func TestScaffoldFollowsTheSessionBackendChoice(t *testing.T) {
	rdbFiles := scaffoldFiles(initOptions{Name: "demo", Database: true, Auth: authOIDC, Session: sessionRDB})
	if !strings.Contains(rdbFiles["cmd/demo/main.go"], `_ "github.com/shibukawa/popcornwave/sessionstore/sqlite"`) {
		t.Errorf("rdb backend is not imported:\n%s", rdbFiles["cmd/demo/main.go"])
	}
	// The login ceremony records live in the database whichever backend holds
	// the sessions.
	if !strings.Contains(rdbFiles["cmd/demo/main.go"], `_ "github.com/shibukawa/popcornwave/authstate/sqlite"`) {
		t.Errorf("ceremony store is not imported:\n%s", rdbFiles["cmd/demo/main.go"])
	}
	if !strings.Contains(rdbFiles["config.dev.toml"], `backend = "rdb"`) ||
		!strings.Contains(rdbFiles["config.dev.toml"], `rdb.source = "middleware"`) {
		t.Errorf("rdb session config:\n%s", rdbFiles["config.dev.toml"])
	}
	if _, ok := rdbFiles["migrations/00002_"+sessionstore.MigrationName+".sql"]; !ok {
		t.Error("rdb backend did not scaffold its session migration")
	}
	if _, ok := rdbFiles["migrations/00003_"+auth.MigrationName+".sql"]; !ok {
		t.Error("auth migration is not numbered after the session one")
	}

	cookieFiles := scaffoldFiles(initOptions{Name: "demo", Database: true, Auth: authOIDC, Session: sessionCookie})
	if strings.Contains(cookieFiles["cmd/demo/main.go"], "sessionstore/") {
		t.Errorf("the built-in cookie backend was imported:\n%s", cookieFiles["cmd/demo/main.go"])
	}
	// Every backend seals a record into the browser while a session is still
	// anonymous, so the keyring is scaffolded whatever the backend is, and the
	// cookie backend adds no key of its own.
	if !strings.Contains(cookieFiles["config.dev.toml"], "keyring.secret = ") {
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
	if !strings.Contains(redisFiles["cmd/demo/main.go"], `_ "github.com/shibukawa/popcornwave/sessionstore/redis"`) {
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

// mustSessionMigration and mustAuthMigration are the SQLite migrations the
// scaffold writes, which is the dialect these fixtures use.
func mustSessionMigration() string {
	migration, err := sessionstore.MigrationSQL("sqlite", "popcornwave_session")
	if err != nil {
		panic(err)
	}
	return migration
}

func mustAuthMigration() string {
	migration, err := auth.MigrationSQL("sqlite")
	if err != nil {
		panic(err)
	}
	return migration
}

// A SQL store is one package per engine, so the engine answer decides which
// storage packages the entry point imports and which dialect the migrations
// are written in.
func TestScaffoldImportsTheStoresOfTheSelectedEngine(t *testing.T) {
	for engine, want := range map[string][]string{
		engineSQLite:   {"sessionstore/sqlite", "authstate/sqlite"},
		enginePostgres: {"sessionstore/postgres", "authstate/postgres"},
		engineMySQL:    {"sessionstore/mysql", "authstate/mysql"},
	} {
		t.Run(engine, func(t *testing.T) {
			files := scaffoldFiles(initOptions{
				Name: "demo", Database: true, Engine: engine, Auth: authOIDC, Session: sessionRDB,
			})
			main := files["cmd/demo/main.go"]
			for _, path := range want {
				if !strings.Contains(main, `_ "github.com/shibukawa/popcornwave/`+path+`"`) {
					t.Errorf("%s is not imported:\n%s", path, main)
				}
			}
			// The migration has to be readable by the engine that will run it.
			migration := files["migrations/00002_"+sessionstore.MigrationName+".sql"]
			expected, err := sessionstore.MigrationSQL(engine, "popcornwave_session")
			if err != nil {
				t.Fatal(err)
			}
			if migration != expected {
				t.Errorf("session migration is not in the %s dialect:\n%s", engine, migration)
			}
		})
	}
}

// A project with a session gets the CSRF shape written out and switched off.
// Turning it on later should be uncommenting rather than looking the keys up.
func TestScaffoldWritesTheCSRFSectionOffByDefault(t *testing.T) {
	config := scaffoldFiles(initOptions{Name: "demo", Auth: authOIDC})[pwenv.FileName(pwenv.Development)]
	if !strings.Contains(config, "[security]") {
		t.Fatalf("no security section:\n%s", config)
	}
	if !strings.Contains(config, "csrf.enabled = false") {
		t.Errorf("CSRF is not scaffolded off:\n%s", config)
	}
	// The anonymous path is a comment, so a project that needs it finds the
	// keys rather than the documentation.
	if !strings.Contains(config, "# csrf.anonymous.enabled = true") {
		t.Errorf("the anonymous path is not shown:\n%s", config)
	}
}

// A page tree's action endpoints are POSTs reachable with the session cookie
// and nothing else in front of them, so the prefix must be in the include list.
func TestScaffoldCoversThePageActionPrefix(t *testing.T) {
	pages := scaffoldFiles(initOptions{Name: "demo", Auth: authOIDC, Router: routerDiscovered})[pwenv.FileName(pwenv.Development)]
	if !strings.Contains(pages, `csrf.include = ["/_action/**", "/**"]`) {
		t.Errorf("a page tree did not cover the action prefix:\n%s", pages)
	}
}

// Without a session there is nothing to bind a token to, so the section would
// only describe a check that could not pass.
func TestScaffoldWritesNoSecuritySectionWithoutASession(t *testing.T) {
	config := scaffoldFiles(initOptions{Name: "demo"})[pwenv.FileName(pwenv.Development)]
	if strings.Contains(config, "[security]") {
		t.Errorf("a session-less project got a security section:\n%s", config)
	}
}

// Choosing DynamoDB for the login asks for no SQL engine behind it. All four
// tables plugin/auth owns move to DynamoDB with auth.backend, so there is
// nothing left for a relational database to carry.
func TestInitWizardAsksForNoEngineBehindADynamoLogin(t *testing.T) {
	t.Chdir(t.TempDir())
	model := feedWizard(t, newTestWizard(defaultInitOptions()),
		typeText("demo"), pressKey(tea.KeyEnter),
		pressKey(tea.KeyEnter), // TinyGo
		pressKey(tea.KeyEnter), // Router
		pressKey(tea.KeyEnter), // Tailwind
		typeText("2"),          // Authentication: OIDC
		typeText("4"),          // Store: DynamoDB
	)
	if label := model.steps[model.index].label(); label == "Database engine" {
		t.Fatal("a DynamoDB login still asked for a SQL engine")
	}
	model = feedWizard(t, model,
		pressKey(tea.KeyEnter), // Session storage
		pressKey(tea.KeyEnter), // OIDC provider
	)
	options := wizardResult(model, defaultInitOptions())
	if !options.Dynamo || options.Database {
		t.Fatalf("options = %#v", options)
	}
	// Neither the engine nor the DynamoDB question is asked again: the store
	// answer settled both.
	for _, index := range model.activeSteps() {
		switch label := model.steps[index].label(); label {
		case "DynamoDB", "Database engine", "Database":
			t.Fatalf("%q was asked after DynamoDB was the store answer", label)
		}
	}
}

// The project that answer produces is the one the backend was built for: no rdb
// section, and the four auth tables named as DynamoDB's.
func TestDynamoLoginScaffoldsTheDynamoAuthBackend(t *testing.T) {
	options := initOptions{
		Name: "demo", Router: routerRegistered, Auth: authOIDC, AuthStore: dynamoStore,
		Dynamo: true, Database: false, Session: sessionDynamo,
	}
	if backend := authBackend(options); backend != "dynamo" {
		t.Fatalf("auth backend = %q", backend)
	}
	config := authRuntimeConfig(options)
	if !strings.Contains(config, `backend = "dynamo"`) {
		t.Fatalf("[auth] section does not name the backend:\n%s", config)
	}
	// Both halves are imported: the ceremony store and the account-side stores.
	imports := sessionBackendImport(options)
	for _, wanted := range []string{"authstate/dynamo", "authstore/dynamo"} {
		if !strings.Contains(imports, wanted) {
			t.Fatalf("imports do not carry %s:\n%s", wanted, imports)
		}
	}
	if strings.Contains(imports, "authstate/sqlite") {
		t.Fatalf("a relational ceremony store was imported anyway:\n%s", imports)
	}
}

// A relational login is untouched by any of this.
func TestRelationalLoginKeepsTheRelationalBackend(t *testing.T) {
	options := initOptions{
		Name: "demo", Router: routerRegistered, Auth: authOIDC, AuthStore: engineSQLite,
		Database: true, Engine: engineSQLite, Session: sessionRDB,
	}
	if backend := authBackend(options); backend != "rdb" {
		t.Fatalf("auth backend = %q", backend)
	}
	if !strings.Contains(authRuntimeConfig(options), `backend = "rdb"`) {
		t.Fatal("[auth] section does not name the relational backend")
	}
	imports := sessionBackendImport(options)
	if !strings.Contains(imports, "authstate/sqlite") || strings.Contains(imports, "authstore/dynamo") {
		t.Fatalf("imports = %s", imports)
	}
}

// A login with neither store has nowhere to keep its records, and the refusal
// names both ways out rather than only the database.
func TestALoginNeedsAStore(t *testing.T) {
	_, err := parseInitArgs([]string{"demo", "--auth=oidc", "--no-database"})
	if err == nil || !strings.Contains(err.Error(), "--dynamo") {
		t.Fatalf("a login with no store = %v", err)
	}
	// Naming the other store is accepted, and the session follows it rather
	// than defaulting to a pool the project does not open.
	options, err := parseInitArgs([]string{"demo", "--auth=oidc", "--no-database", "--dynamo"})
	if err != nil {
		t.Fatalf("a DynamoDB-only login = %v", err)
	}
	if backend := sessionBackend(options); backend != sessionDynamo {
		t.Fatalf("session backend = %q", backend)
	}
	if backend := authBackend(options); backend != "dynamo" {
		t.Fatalf("auth backend = %q", backend)
	}
}

// A SQL login still reaches the DynamoDB question: the store answer covered one
// kind of store, and the other one is a separate decision.
func TestInitWizardStillAsksAboutDynamoAfterASQLLogin(t *testing.T) {
	t.Chdir(t.TempDir())
	model := feedWizard(t, newTestWizard(defaultInitOptions()),
		typeText("demo"), pressKey(tea.KeyEnter),
		pressKey(tea.KeyEnter), // TinyGo
		pressKey(tea.KeyEnter), // Router
		pressKey(tea.KeyEnter), // Tailwind
		typeText("2"),          // Authentication: OIDC
		typeText("1"),          // Store: SQLite
	)
	if label := model.steps[model.index].label(); label != "DynamoDB" {
		t.Fatalf("step = %q, want the other kind of store", label)
	}
	model = feedWizard(t, model,
		typeText("1"),          // DynamoDB: Yes
		pressKey(tea.KeyEnter), // Session storage
		pressKey(tea.KeyEnter), // OIDC provider
	)
	options := wizardResult(model, defaultInitOptions())
	if !options.Dynamo || options.Engine != engineSQLite {
		t.Fatalf("options = %#v", options)
	}
}

// Partial updates are scaffolded off, with the key they need shown rather than
// left for the reader to find. Enabling them without one is a startup failure.
func TestScaffoldWritesTheUpdateSectionOffByDefault(t *testing.T) {
	config := scaffoldFiles(initOptions{Name: "demo", Auth: authOIDC})[pwenv.FileName(pwenv.Development)]
	if !strings.Contains(config, "[html.update]") {
		t.Fatalf("no update section:\n%s", config)
	}
	if !strings.Contains(config, "enabled = false") {
		t.Errorf("updates are not scaffolded off:\n%s", config)
	}
	if !strings.Contains(config, "# validator_key =") {
		t.Errorf("the validator key is not shown:\n%s", config)
	}
}

// Session storage is not a login. A project that declares only a language
// preference still needs the middleware, and the framework installs it from
// session.enabled rather than from an authentication plugin.
func TestSessionIsScaffoldedWithoutALogin(t *testing.T) {
	files := scaffoldFiles(initOptions{Name: "demo", Database: true})
	config := files["config.dev.toml"]
	if !strings.Contains(config, "[session]") {
		t.Fatalf("a project without a login got no session section:\n%s", config)
	}
	if strings.Contains(config, "[auth]") {
		t.Fatal("a project without a login got an auth section")
	}
	// Cookie is the coherent default there: no table, no migration, no import
	// to hold state that fits in a sealed cookie.
	if !strings.Contains(config, `backend = "cookie"`) {
		t.Fatalf("session backend without a login:\n%s", config)
	}
	if strings.Contains(files["cmd/demo/main.go"], "sessionstore/") {
		t.Error("a cookie session imported a storage plugin")
	}
	if strings.Contains(files["cmd/demo/main.go"], "authstate/") {
		t.Error("a project with no login imported the login ceremony store")
	}
	for name := range files {
		if strings.Contains(name, sessionstore.MigrationName) {
			t.Errorf("a cookie session scaffolded a table migration: %s", name)
		}
	}
	// The keyring is still written, because a sealed slot needs one whatever
	// the backend is.
	if !strings.Contains(config, "keyring.secret = ") {
		t.Error("no keyring was scaffolded for a session that seals")
	}
}

// Asking for a server backend without a login is honored, and it brings the
// import with it rather than leaving a configuration nothing serves.
func TestAServerBackendWithoutALoginBringsItsImport(t *testing.T) {
	files := scaffoldFiles(initOptions{Name: "demo", Database: true, Session: sessionRDB})
	if !strings.Contains(files["config.dev.toml"], `backend = "rdb"`) {
		t.Fatal("the selected backend was not scaffolded")
	}
	if !strings.Contains(files["cmd/demo/main.go"], "sessionstore/") {
		t.Errorf("the rdb backend was configured without its import:\n%s", files["cmd/demo/main.go"])
	}
}
