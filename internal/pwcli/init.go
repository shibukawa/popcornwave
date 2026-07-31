package pwcli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"

	"github.com/charmbracelet/x/term"
	"github.com/shibukawa/popcornwave/internal/pwenv"
	"github.com/shibukawa/popcornwave/internal/pwgen"
	"github.com/shibukawa/popcornwave/plugin/auth"
	"github.com/shibukawa/popcornwave/plugin/session/rdb"
)

const initUsage = "usage: pw init [<project-name>] [--interactive] [--router=registered|discovered|both] [--tailwind] [--no-tinygo] [--no-devbox] [--no-database] [--db=sqlite|postgres|mysql] [--no-redis] [--auth=none|oidc|passkey] [--session=rdb|cookie|redis] [--devidp]"

// Authentication modes the wizard and the --auth flag select between. They map
// onto the plugin/auth modes, with none meaning no [auth] configuration.
const (
	authNone    = "none"
	authOIDC    = "oidc"
	authPasskey = "passkey"
)

// Session storage backends the wizard and the --session flag select between.
// They are the api:session-backend-plugin names, and each one but cookie is
// contributed to the binary by a blank import the scaffold writes.
const (
	sessionRDB    = "rdb"
	sessionCookie = "cookie"
	sessionRedis  = "redis"
)

// usesOIDC reports whether a mode needs an OpenID Provider.
func usesOIDC(mode string) bool { return mode == authOIDC }

// sessionBackend is the selected backend. An options value built without the
// answer, which every pre-session caller is, scaffolds the default.
func sessionBackend(options initOptions) string {
	if options.Session == "" {
		return sessionRDB
	}
	return options.Session
}

// sessionBackendPlugin names the import that registers a backend. The cookie
// backend is built into pw, so it needs none.
func sessionBackendPlugin(backend string) string {
	switch backend {
	case sessionRDB:
		return "github.com/shibukawa/popcornwave/plugin/session/rdb"
	case sessionRedis:
		return "github.com/shibukawa/popcornwave/plugin/session/redis"
	default:
		return ""
	}
}

// Router answers of pw init. The two routers coexist on one mux, so this
// decides which of them the scaffold writes rather than how the framework
// behaves.
//
// These three names are the project's vocabulary for the pair, and everything
// that talks about a router uses them: the flag, the wizard, and the capability
// catalog of pw add. What each router reads is a directory, and which directory
// is a data:project-config value rather than part of the name.
const (
	// routerRegistered writes registrations in Go: any method, any response,
	// and the generated OpenAPI document.
	routerRegistered = "registered"
	// routerDiscovered derives routes from a directory tree: a directory
	// holding a page template is a route, and generation writes the
	// registration.
	routerDiscovered = "discovered"
	routerBoth       = "both"
)

// The directories pw init scaffolds each router into. They are defaults rather
// than fixed names: generation, pw new, and pw dev all read the data:project-config
// purpose lists, so a project renames a tree by editing popcornwave.toml and
// moving the directory.
const (
	defaultRegisteredDir = "handlers"
	defaultDiscoveredDir = "pages"
	defaultTemplatesDir  = "templates"
)

// effectiveRouter reads an unset answer as the registered router: that is the
// shape every project scaffolded before page trees existed has, and the one a
// caller that never asked should get.
func effectiveRouter(router string) string {
	if router == "" {
		return routerRegistered
	}
	return router
}

func routerHasRegistered(router string) bool {
	router = effectiveRouter(router)
	return router == routerRegistered || router == routerBoth
}

func routerHasDiscovered(router string) bool {
	return router == routerDiscovered || router == routerBoth
}

func validRouter(router string) bool {
	return router == routerRegistered || router == routerDiscovered || router == routerBoth
}

// initOptions holds every project bootstrap choice. Shortcut flags and the
// wizard produce the same value, and scaffoldFiles is its only consumer.
type initOptions struct {
	Name string
	// Router selects the routers this project starts with. Either can be
	// installed later, so this is a starting point rather than a mode.
	Router   string
	TinyGo   bool
	Tailwind bool
	// Database scaffolds the rdb configuration, the migration directory, and
	// the SQL example. Declining it removes all three together, because none
	// of them is useful without the others.
	Database bool
	// Engine names the database. It decides the DSN, the dialect of the
	// starter schema, the development server, and the driver the binary links,
	// and applies only to a project that took the database.
	Engine string
	// Redis adds the Valkey development server to the Devbox environment.
	Redis bool
	// Devbox scaffolds the reproducible development environment. Declining it
	// leaves the toolchain and the services to the operator, which is what a
	// project already standardized on Nix, Docker, or asdf wants.
	Devbox bool
	Auth   string
	// Session selects where login sessions are stored. It only applies to a
	// project that scaffolds a login.
	Session string
	// AuthEmulator scaffolds the development identity provider instead of
	// pointing the project at an external one. It only applies to an OIDC mode.
	AuthEmulator bool
	Interactive  bool
}

// defaultInitOptions keeps TinyGo compatible routing as the scaffold default so
// the shortcut form matches decision:stdlib-servemux.
func defaultInitOptions() initOptions {
	return initOptions{
		// Router is left unset rather than defaulted: effectiveRouter reads that
		// as the registered tree, which keeps the rule in one place and a
		// scripted run on the shape it has always produced.
		TinyGo:   true,
		Devbox:   true,
		Database: true,
		Engine:   engineSQLite,
		Redis:    true,
		Auth:     authNone,
		Session:  sessionRDB,
	}
}

func parseInitArgs(args []string) (initOptions, error) {
	options := defaultInitOptions()
	var positional []string
	// Tracked so --db can contradict --no-database. Without it the engine
	// would silently apply to a project that has no database to apply it to.
	engineSelected := false
	for _, arg := range args {
		switch arg {
		case "--tailwind":
			options.Tailwind = true
		case "--no-tailwind":
			options.Tailwind = false
		case "--tinygo":
			options.TinyGo = true
		case "--no-tinygo":
			options.TinyGo = false
		case "-i", "--interactive":
			options.Interactive = true
		case "--devbox":
			options.Devbox = true
		case "--no-devbox":
			options.Devbox = false
		case "--database":
			options.Database = true
		case "--no-database":
			options.Database = false
		case "--redis":
			options.Redis = true
		case "--no-redis":
			options.Redis = false
		case "--devidp":
			options.AuthEmulator = true
		case "--no-devidp":
			options.AuthEmulator = false
		default:
			if backend, ok := strings.CutPrefix(arg, "--session="); ok {
				switch backend {
				case sessionRDB, sessionCookie, sessionRedis:
					options.Session = backend
				default:
					return initOptions{}, fmt.Errorf("init: --session must be %s, %s, or %s",
						sessionRDB, sessionCookie, sessionRedis)
				}
				continue
			}
			if engine, ok := strings.CutPrefix(arg, "--db="); ok {
				if !validEngine(engine) {
					return initOptions{}, fmt.Errorf("init: --db must be %s", engineNames())
				}
				options.Engine = engine
				engineSelected = true
				continue
			}
			if router, ok := strings.CutPrefix(arg, "--router="); ok {
				if !validRouter(router) {
					return initOptions{}, fmt.Errorf("init: --router must be %s, %s, or %s",
						routerRegistered, routerDiscovered, routerBoth)
				}
				options.Router = router
				continue
			}
			if mode, ok := strings.CutPrefix(arg, "--auth="); ok {
				switch mode {
				case authNone, authOIDC, authPasskey:
					options.Auth = mode
				default:
					return initOptions{}, fmt.Errorf("init: --auth must be %s, %s, or %s",
						authNone, authOIDC, authPasskey)
				}
				continue
			}
			if strings.HasPrefix(arg, "-") {
				return initOptions{}, fmt.Errorf("init: unknown option %q", arg)
			}
			positional = append(positional, arg)
		}
	}
	if len(positional) > 1 {
		return initOptions{}, errors.New(initUsage)
	}
	if len(positional) == 1 {
		options.Name = strings.TrimSpace(positional[0])
	}
	if !usesOIDC(options.Auth) {
		options.AuthEmulator = false
	}
	if !options.Devbox && options.Redis {
		// The Valkey answer only ever writes a Devbox package, so without the
		// environment there is nothing for it to do.
		options.Redis = false
	}
	if !options.Database && servesLogin(options) {
		// The login ceremony and admission tables are server state in every
		// session backend, so authentication needs the database whichever one
		// stores the sessions themselves.
		return initOptions{}, fmt.Errorf("init: --auth=%s keeps its login ceremony and allowlist tables in the database; drop --no-database", options.Auth)
	}
	if !options.Database && engineSelected {
		return initOptions{}, errors.New("init: --db selects the database engine; drop --no-database")
	}
	options = normalizeSession(options)
	return options, nil
}

// normalizeSession settles what the session backend implies. A Redis-backed
// session wants the development server that serves it, and a project without a
// login stores no sessions at all.
func normalizeSession(options initOptions) initOptions {
	if !servesLogin(options) {
		return options
	}
	if sessionBackend(options) == sessionRedis && options.Devbox {
		options.Redis = true
	}
	return options
}

// interactiveTerminal reports whether the wizard can drive the current session.
func interactiveTerminal() bool {
	return term.IsTerminal(os.Stdin.Fd()) && term.IsTerminal(os.Stdout.Fd())
}

func runInit(args []string, stdout io.Writer) error {
	options, err := parseInitArgs(args)
	if err != nil {
		return err
	}
	if options.Name == "" || options.Interactive {
		if !interactiveTerminal() {
			return fmt.Errorf("init: the wizard needs a terminal; %s", initUsage)
		}
		options, err = runInitWizard(options)
		if errors.Is(err, errWizardCanceled) {
			fmt.Fprintln(stdout, "init canceled")
			return nil
		}
		if err != nil {
			return err
		}
	}
	name := options.Name
	destination, err := initDestination(name)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(destination, 0o755); err != nil {
		return err
	}
	files := scaffoldFiles(options)
	for path, content := range files {
		target := filepath.Join(destination, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := writeScaffoldFile(target, []byte(content)); err != nil {
			return err
		}
	}
	tidy := exec.Command("go", "mod", "tidy")
	tidy.Dir = destination
	tidy.Stdout = stdout
	tidy.Stderr = stdout
	tidy.Env = os.Environ()
	if err := tidy.Run(); err != nil {
		return fmt.Errorf("initialize Go module: %w", err)
	}
	previous, err := os.Getwd()
	if err != nil {
		return err
	}
	if err := os.Chdir(destination); err != nil {
		return err
	}
	generateErr := runGenerate(context.Background(), nil, stdout)
	restoreErr := os.Chdir(previous)
	if generateErr != nil {
		return fmt.Errorf("generate starter: %w", generateErr)
	}
	if restoreErr != nil {
		return restoreErr
	}
	fmt.Fprintf(stdout, "\nCreated %s\n", name)
	// The wizard says this beside each answer, but a scripted run never sees
	// it, and a declined capability is the one thing about the new project an
	// operator may not know is reversible.
	if declined := declinedCapabilities(options); len(declined) > 0 {
		fmt.Fprintf(stdout, "\nNot included: %s\n  pw add <capability> enables one later\n",
			strings.Join(declined, ", "))
	}
	// The cookie backend seals its records under a secret the project cannot
	// invent for itself, so an operator has to supply one before the first run.
	if servesLogin(options) && sessionBackend(options) == sessionCookie {
		fmt.Fprint(stdout, "\nThe cookie session backend needs its sealing secret:\n"+
			"  export SESSION_COOKIE_SECRET=$(openssl rand -base64 32)\n")
	}
	if notice := databaseEngineNotice(options); notice != "" {
		fmt.Fprint(stdout, notice)
	}
	// Without the Devbox environment nothing pins the CSS toolchain, and the
	// first pw dev would fail on a binary the scaffold never installed.
	if options.Tailwind && !options.Devbox {
		fmt.Fprintf(stdout, "\nTailwind CSS needs its own toolchain here:\n  install %s\n",
			tailwindToolchainRequirement)
	}
	fmt.Fprintf(stdout, "\n  cd %s\n%s  pw dev\n", name, devboxNextStep(options))
	return nil
}

// devboxNextStep names the shell to enter first, for a project that has one.
func devboxNextStep(options initOptions) string {
	if !options.Devbox {
		return ""
	}
	return "  devbox shell\n"
}

// declinedCapabilities names what this project did not take, in catalog order.
func declinedCapabilities(options initOptions) []string {
	var declined []string
	for _, capability := range capabilityOrder {
		switch capability {
		case capabilityDevbox:
			if !options.Devbox {
				declined = append(declined, capability)
			}
		case capabilityDatabase:
			if !options.Database {
				declined = append(declined, capability)
			}
		case capabilityRedis:
			if !options.Redis {
				declined = append(declined, capability)
			}
		case capabilityAuth:
			if !servesLogin(options) {
				declined = append(declined, capability)
			}
		case capabilityTailwind:
			if !options.Tailwind {
				declined = append(declined, capability)
			}
		}
	}
	return declined
}

func writeScaffoldFile(target string, content []byte) error {
	temp, err := os.CreateTemp(filepath.Dir(target), "."+filepath.Base(target)+".tmp-*")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if err := temp.Chmod(0o644); err != nil {
		_ = temp.Close()
		return err
	}
	if _, err := temp.Write(content); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(tempPath, target)
}

func validProjectName(name string) bool {
	if name == "" || name == "." || name == ".." {
		return false
	}
	for _, r := range name {
		if !(r == '-' || r == '_' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9') {
			return false
		}
	}
	return true
}

// initDestination resolves the project directory and refuses collisions.
func initDestination(name string) (string, error) {
	if !validProjectName(name) {
		return "", fmt.Errorf("invalid project name %q", name)
	}
	destination, err := filepath.Abs(name)
	if err != nil {
		return "", err
	}
	entries, readErr := os.ReadDir(destination)
	switch {
	case readErr == nil && len(entries) > 0:
		return "", fmt.Errorf("destination %s is not empty", destination)
	case readErr != nil && !os.IsNotExist(readErr):
		return "", readErr
	}
	return destination, nil
}

// validateProjectName reports the wizard-facing reason a name is unusable.
func validateProjectName(name string) error {
	if name == "" {
		return errors.New("a project name is required")
	}
	_, err := initDestination(name)
	return err
}

func scaffoldFiles(options initOptions) map[string]string {
	name := options.Name
	moduleExtra := frameworkModuleDirective()
	devboxPackages := []string{"go@latest"}
	if options.Database {
		if server := engineFor(options.Engine).DevboxPackage; server != "" {
			devboxPackages = append(devboxPackages, server)
		}
	}
	if options.Redis {
		devboxPackages = append(devboxPackages, "valkey@latest")
	}
	if options.TinyGo {
		devboxPackages = append(devboxPackages, "tinygo@latest")
	}
	configTailwind := ""
	homeStylesheet := ""
	homeClasses := ""
	if options.Tailwind {
		configTailwind = tailwindProjectConfig()
		devboxPackages = append(devboxPackages, tailwindDevboxPackage)
		homeStylesheet = `<link rel="stylesheet" href="/public/generated/app.css">`
		homeClasses = ` class="mx-auto max-w-3xl p-8 text-slate-900"`
	}
	files := map[string]string{
		"go.mod": "module " + name + "\n\ngo 1.26.0\n\n" + moduleExtra,
		"popcornwave.toml": `[project]
name = "` + name + `"
main = "./cmd/` + name + `"
toolchain = "` + projectToolchain(options) + `"
` + projectDatabaseConfig(options) + `
# Each purpose reads only the directories it lists, and nothing else. A source
# directory is invisible to that purpose until it appears here.
[generate]
handlers = [` + quotedList(scaffoldGenerationScope(options).Handlers) + `]
templates = [` + quotedList(scaffoldGenerationScope(options).Templates) + `]
queries = [` + quotedList(scaffoldGenerationScope(options).Queries) + `]
config = [` + quotedList(scaffoldGenerationScope(options).Config) + `]
# A page tree root is one generation run: every directory below it holding a
# page template is a route, and the registration is generated.
pages = [` + quotedList(scaffoldGenerationScope(options).Pages) + `]

# pw dev walks the module for rebuild inputs. Add what the walk misses, and
# exclude a subtree that only makes the walk slower.
[dev.watch]
includes = []
excludes = []
` + devIdPProjectConfig(options) + configTailwind,
		pwenv.FileName(pwenv.Development): `# Development runtime configuration.
# APP_ENV selects this file; add config.stg.toml and config.prod.toml as needed.
[server]
port = 8080
# Operational endpoints answer only at the paths named here; an unset key
# serves nothing, so this file lists every address the server responds on.
health = "/healthz"
readiness = "/readyz"
openapi = "/openapi.json"
# Scalar API reference for the document above, served at server.api_doc_path
# (/docs). Leave this key out of staging and production configs to keep the UI
# private.
api_doc = "scalar"

[observability]
minimum_level = "debug"
service_name = "` + name + `"
` + databaseRuntimeConfig(options) + authRuntimeConfig(options),
		"cmd/" + name + "/main.go": mainScaffold(options),
		"templates/document.pw.html": `package templates

external RuntimeScriptURL(): url

export component Document(children: html?): html {
<!doctype html>
<html lang="en"><head><meta charset="utf-8"><title>Popcorn Wave</title>` + homeStylesheet +
			`<script type="module" src={RuntimeScriptURL()}></script></head>
<body` + homeClasses + `><slot /></body></html>
}
`,
		"templates/templates.go": `package templates

import (
	"net/url"

	"github.com/shibukawa/popcornwave/pw"
)

// RuntimeScriptURL backs the external declaration in document.pw.html.
//
// The runtime it names applies the sections of a page that arrive after the
// rest of it, which is what a template declaring an ` + "`async`" + ` parameter needs.
// A page without one loads it and finds nothing to do.
//
// The template calls this rather than writing a literal path, because the URL
// carries a revision derived from the script's own bytes: an upgrade that
// changes the runtime changes the URL, and a literal would go on pointing at
// bytes the server no longer serves.
func RuntimeScriptURL() *url.URL { return &url.URL{Path: pw.RuntimeScriptURL()} }
`,
		"templates/400.pw.html": errorTemplate("templates", "Error400", "Bad Request"),
		"templates/401.pw.html": errorTemplate("templates", "Error401", "Unauthorized"),
		"templates/403.pw.html": errorTemplate("templates", "Error403", "Forbidden"),
		"templates/404.pw.html": errorTemplate("templates", "Error404", "Not Found"),
		"templates/409.pw.html": errorTemplate("templates", "Error409", "Conflict"),
		"templates/413.pw.html": errorTemplate("templates", "Error413", "Payload Too Large"),
		"templates/500.pw.html": errorTemplate("templates", "Error500", "Internal Server Error"),
		"public.go": `package publicassets

import (
	"embed"
	"io/fs"

	"github.com/shibukawa/popcornwave/middlewares"
)

//go:embed all:public
var embeddedPublic embed.FS

func init() {
	middlewares.RegisterPublicFS(PublicFS())
}

func PublicFS() fs.FS {
	result, err := fs.Sub(embeddedPublic, "public")
	if err != nil {
		panic(err)
	}
	return result
}
`,
		"public/.keep": "",
		".vscode/settings.json": `{
    "files.exclude": {
        "**/*_pw_gen.go": true
    }
}
`,
		// The binary pattern is anchored: a bare name would also ignore cmd/<name>/.
		// devbox.d holds the service configuration devbox writes on first run,
		// so pw dev leaves no change behind in a fresh checkout.
		".gitignore": ".devbox/\ndevbox.d/\n/" + name + "\n*_pw_gen.go\npublic/**/*.zstd\n*.db\n",
	}
	if routerHasRegistered(options.Router) {
		for path, source := range registeredRouterScaffold(options, defaultRegisteredDir) {
			files[path] = source
		}
	}
	if routerHasDiscovered(options.Router) {
		for path, source := range pageTreeScaffold(options, defaultDiscoveredDir) {
			files[path] = source
		}
	}
	if options.Devbox {
		files["devbox.json"] = devboxScaffold(devboxPackages)
		files["devbox.lock"] = "{}\n"
	}
	if options.Database {
		files["queries/users.pw.sql"] = starterQuery()
		files["migrations/00001_init.sql"] = engineFor(options.Engine).Schema
	}
	if options.TinyGo {
		files["tinygohelper.go"] = `//go:build tinygo

package publicassets

// TinyGo's net package routes every socket through a Netdever that the program
// has to register itself; without one the server dies at startup with
// "Netdev not set". The blank import registers the host OS driver during init.
// Standard Go builds skip this file and use the real net package.
import _ "github.com/shibukawa/tinygodriver/netdev"
`
	}
	if options.Tailwind {
		files[defaultTailwindInput] = tailwindEntryScaffold(scaffoldGenerationScope(options))
		files[defaultTailwindOutput] = "/* Generated by Tailwind CSS. */\n"
	}
	if options.AuthEmulator {
		files[defaultIdPConfig] = devIdPRoster()
	}
	if servesLogin(options) {
		files["handlers/accounts.go"] = accountResolverScaffold()
		// The framework tables come from the packages that own them. A fresh
		// project has only the application schema, so these take the versions
		// after it; pw add would take whatever is free at that point instead.
		// Only the rdb backend owns a table: a cookie or Redis session leaves
		// the numbering to the auth migration alone.
		version := 2
		if sessionBackend(options) == sessionRDB {
			files["migrations/00002_"+rdb.MigrationName+".sql"] = rdb.MigrationSQL("popcornwave_session")
			version = 3
		}
		files[fmt.Sprintf("migrations/0000%d_%s.sql", version, auth.MigrationName)] = auth.MigrationSQL()
	}
	return files
}

// servesLogin reports whether the framework mounts the authentication
// endpoints for this project. Only the OIDC modes have an implementation.
func servesLogin(options initOptions) bool { return usesOIDC(options.Auth) }

// databaseRuntimeConfig writes the rdb section the scaffolded migrations and
// queries need. A project that declined the database has neither, so the
// section would configure a pool nothing opens.
func databaseRuntimeConfig(options initOptions) string {
	if !options.Database {
		return ""
	}
	engine := engineFor(options.Engine)
	return databaseRuntimeSection(engine.DSN(options.Name), engine)
}

// databaseRuntimeSection is the rdb configuration api:cli-init scaffolds and
// api:cli-add appends, so both reach the same file state. The pool bounds come
// from the engine: one connection is right for a file SQLite writes serially
// and wrong for a server that expects a pool.
func databaseRuntimeSection(dsn string, engine databaseEngine) string {
	return `
# The scaffolded migrations and queries need a database; pw dev and pw migrate
# read this DSN.
[middleware.rdb]
enabled = true
dsn = "` + dsn + `"
connect_timeout = "5s"
max_open_conns = ` + strconv.Itoa(engine.MaxOpenConns) + `
max_idle_conns = ` + strconv.Itoa(engine.MaxIdleConns) + `
`
}

// sessionBackendImport contributes the storage the configuration selects.
// Session storage is opt-in by blank import, so an application links the one
// backend it configured and nothing else. The cookie backend is built into pw
// and needs no line here.
func sessionBackendImport(options initOptions) string {
	plugin := ""
	if servesLogin(options) {
		plugin = sessionBackendPlugin(sessionBackend(options))
	}
	if plugin == "" {
		return ""
	}
	return "\n\t// session.backend = \"" + sessionBackend(options) +
		"\" is served by this import; storage is opt-in.\n\t_ " + strconv.Quote(plugin)
}

// projectDatabaseConfig records the engine .pw.sql sources are generated for.
// A project without a database writes no key, because there is nothing to
// generate and the default would be an answer to a question it never asked.
func projectDatabaseConfig(options initOptions) string {
	if !options.Database {
		return ""
	}
	return "database = \"" + options.Engine + "\"\n"
}

// databaseDriverImport links the selected engine into the application binary.
// pw links SQLite itself, so only a server engine adds an import; without it
// the pool refuses to open and names the import to add.
func databaseDriverImport(options initOptions) string {
	if !options.Database {
		return ""
	}
	path := engineFor(options.Engine).DriverImport
	if path == "" {
		return ""
	}
	return "\n\t// Registers the engine middleware.rdb.dsn names.\n\t_ " + strconv.Quote(path)
}

// authBootstrap installs the account resolver. That call is the whole
// application-side wiring of a login: it also imports plugin/auth, whose
// extensions serve the endpoints and resolve the session.
func authBootstrap(options initOptions) string {
	if !servesLogin(options) {
		return ""
	}
	return "\n\t// Installed before Run: the framework calls it during the OIDC callback.\n\thandlers.RegisterAccountResolver()"
}

// accountResolverScaffold links a verified identity to an application account.
// The starter derives the account from the identity itself, so a new project
// logs in before it has an account table; the comment names the seam.
func accountResolverScaffold() string {
	return `package handlers

import (
	"context"

	"github.com/shibukawa/popcornwave/plugin/auth"
)

// RegisterAccountResolver installs the account resolver. Call it from main
// before pw.Run: the framework verifies the OIDC identity and then asks this
// function which local account it belongs to.
func RegisterAccountResolver() { auth.SetAccountResolver(resolveAccount) }

// resolveAccount answers with the account behind a verified identity.
//
// This starter derives one instead of storing it, which is enough to log in
// and read the user. Replace it with a lookup against your own table as soon
// as the application owns accounts: the link is the issuer plus the verified
// claim auth.oidc.identity_claim selected, never the email address.
func resolveAccount(ctx context.Context, identity auth.Identity, provision bool) (auth.Account, error) {
	displayName, _ := identity.Claims.String("name")
	if displayName == "" {
		displayName = identity.Key
	}
	email, _ := identity.Claims.String("email")
	return auth.Account{
		ID:          identity.Issuer + "|" + identity.Key,
		DisplayName: displayName,
		Email:       email,
	}, nil
}
`
}

// homeHandlerScaffold renders the starter page. With authentication it reads
// the signed-in user; the login itself belongs to the framework.
func homeHandlerScaffold(options initOptions) string {
	if !servesLogin(options) {
		return `package handlers

import (
	"net/http"

	"github.com/shibukawa/popcornwave/pw"
)

type homeInput struct {
	Name string ` + "`query:\"name\" default:\"World\"`" + `
}

func init() { mux.HandleFunc("GET /", home) }

func home(w http.ResponseWriter, r *http.Request) {
	input, err := pw.Parse[homeInput](r)
	if err != nil {
		pw.WriteProblem(w, r, pw.BadRequest(err))
		return
	}
	pw.WriteHTML(w, r, Home(HomeParams{Name: input.Name}))
}
`
	}
	return `package handlers

import (
	"net/http"
	"net/url"

	"github.com/shibukawa/popcornwave/plugin/auth"
	"github.com/shibukawa/popcornwave/pw"
)

func init() { mux.HandleFunc("GET /", home) }

func home(w http.ResponseWriter, r *http.Request) {
	// The framework resolved the session before this handler ran.
	user, signedIn := auth.User(r.Context())
	name := "World"
	if signedIn {
		name = user.DisplayName
		if name == "" {
			name = user.Subject
		}
	}
	pw.WriteHTML(w, r, Home(HomeParams{
		Name:       name,
		SignedIn:   signedIn,
		Email:      user.Email,
		LoginPath:  url.URL{Path: "/auth/login"},
		LogoutPath: url.URL{Path: "/auth/logout"},
	}))
}
`
}

// homeTemplateScaffold renders the starter page. The logout control is a form
// because the endpoint accepts POST only.
func homeTemplateScaffold(options initOptions) string {
	if !servesLogin(options) {
		return `package handlers

export component Home(name: string): html {
<h1 class="text-3xl font-bold">Hello, {name}</h1>
}
`
	}
	return `package handlers

export component Home(name: string, signedIn: bool, email: string, loginPath: url, logoutPath: url): html {
<h1 class="text-3xl font-bold">Hello, {name}</h1>
{if signedIn}
  <p>Signed in as {email}</p>
  <form method="post" action={logoutPath}>
    <button type="submit">Sign out</button>
  </form>
{else}
  <p><a href={loginPath}>Sign in</a></p>
{/if}
}
`
}

// devIdPProjectConfig enables the development identity provider for pw dev.
func devIdPProjectConfig(options initOptions) string {
	if !options.AuthEmulator {
		return ""
	}
	return `
[dev.idp]
enabled = true
config = "` + defaultIdPConfig + `"
`
}

// devIdPRoster is the starter user list. Every value here is a development
// fixture: the provider checks no credential, so nothing in it is a secret.
func devIdPRoster() string {
	return `# Development identity provider users, selected on the login screen.
# pw dev serves these; no password is checked, so this file never ships.

[users.admin]
display_name = "Administrator"
extra_scopes = ["admin"]
[users.admin.claims]
email = "admin@example.com"
role = "admin"

[users.member]
display_name = "Member"
[users.member.claims]
email = "member@example.com"
role = "member"
`
}

// authRuntimeConfig writes the [auth] section for the selected mode. The OIDC
// provider values stay empty for the emulator because pw dev injects them, and
// the application refuses to start if neither the file nor the environment
// supplies them.
func authRuntimeConfig(options initOptions) string {
	if !servesLogin(options) {
		if options.Auth == authPasskey {
			// Recorded, not enabled: no implementation exists yet, and an
			// enabled mode without one fails at startup.
			return `
# Passkey-only login has no implementation yet. Enable it once one is
# registered; plugin/auth rejects the mode today.
# [auth]
# enabled = true
# mode = "passkey_only"
`
		}
		return ""
	}
	provider := `
# Supply these from the environment in every deployed environment:
# AUTH_OIDC_ISSUER, AUTH_OIDC_CLIENT_ID, AUTH_OIDC_CLIENT_SECRET.
issuer = ""
client_id = ""
client_secret = ""`
	loopback := "false"
	if options.AuthEmulator {
		provider = `
# pw dev runs the development identity provider and injects AUTH_OIDC_ISSUER,
# AUTH_OIDC_CLIENT_ID, and AUTH_OIDC_CLIENT_SECRET, so these stay empty here.
# Running the application without pw dev requires setting them yourself.`
		// The development issuer is loopback http, which an https-only client
		// would refuse.
		loopback = "true"
	}
	return `
# The browser token is opaque in every backend; this selects where the record
# behind it lives. A backend other than cookie reaches the binary through the
# blank import in cmd/` + options.Name + `/main.go.
[session]
enabled = true
backend = "` + sessionBackend(options) + `"
ttl = "12h"
idle_timeout = "1h"
cookie.name = "pw_session"
# Loopback development only. Keep secure = true everywhere else.
cookie.secure = false
` + sessionBackendConfig(options) + `

# The framework serves login_path, callback_path, and logout_path itself, so
# the application registers no authentication route. Logout is POST only.
[auth]
enabled = true
mode = "oidc_only"
post_login_path = "/"
# Opt in per path; everything else stays public.
protection.include = []
protection.unauthenticated = "redirect"

[auth.oidc]` + provider + `
redirect_url = "http://127.0.0.1:8080/auth/callback"
scopes = ["profile", "email"]
identity_claim = "sub"
admission = "authenticated"
auto_provision = true
# Sign out of the provider as well. Without it the provider stays signed in and
# the next login returns the same user without asking.
provider_logout = true
allow_loopback_http = ` + loopback + `
`
}

// sessionBackendConfig writes the keys of the selected backend and only those:
// keys of a backend nothing opens would describe storage that is not there.
func sessionBackendConfig(options initOptions) string {
	switch sessionBackend(options) {
	case sessionCookie:
		return `# The record is sealed into a second cookie, so this deployment stores no
# sessions at all. Generate the secret with: openssl rand -base64 32
# Keep it in the environment. During a rotation the old value moves into
# cookie_store.previous_secrets, which keeps issued records readable.
cookie_store.secret = "${SESSION_COOKIE_SECRET}"
`
	case sessionRedis:
		return `# Redis or Valkey holds each record under its own TTL, so no sweep runs.
redis.dsn = "redis://127.0.0.1:6379/0"
redis.key_prefix = "pw:session:"
`
	default:
		return `# The session table lives in the database configured above, created by the
# scaffolded migration.
rdb.source = "middleware"
`
	}
}

// projectToolchain names the compiler the project is scaffolded for.
func projectToolchain(options initOptions) string {
	if options.TinyGo {
		return toolchainTinyGo
	}
	return toolchainGo
}

// muxScaffold emits the route registry. TinyGo projects go through pw.ServeMux
// so one import works on both toolchains; host-only projects keep the standard
// library type, which api:cli-generate discovers just the same.
// mainScaffold writes the entry point for the routers the project took. Both
// register on one mux, and the order they do it in does not matter: a generated
// page route does not shadow a hand-registered subtree, and the standard
// library panics on a duplicate rather than silently letting one win.
func mainScaffold(options initOptions) string {
	name := options.Name
	registered, discovered := defaultRegisteredDir, defaultDiscoveredDir
	registeredPkg, discoveredPkg := goPackageIdentifier(registered), goPackageIdentifier(discovered)
	imports := "\t\"context\"\n\t\"log\"\n\n"
	body := ""
	handler := ""
	switch router := effectiveRouter(options.Router); {
	case router == routerRegistered:
		imports += "\t\"" + name + "/" + registered + "\"\n\t\"github.com/shibukawa/popcornwave/pw\""
		handler = registeredPkg + ".Handlers()"
	case router == routerDiscovered:
		imports += "\t\"" + name + "/" + discovered + "\"\n\t\"github.com/shibukawa/popcornwave/pw\""
		body = "\tmux := pw.NewServeMux()\n\t" + discoveredPkg + ".Register(mux)\n"
		handler = "mux"
	default:
		imports += "\t\"" + name + "/" + registered + "\"\n\t\"" + name + "/" + discovered +
			"\"\n\t\"github.com/shibukawa/popcornwave/pw\""
		body = "\t// The page routes join the handler mux. Registration order does not\n" +
			"\t// matter; a duplicate pattern would panic here rather than shadow.\n" +
			"\tmux := " + registeredPkg + ".Handlers()\n\t" + discoveredPkg + ".Register(mux)\n"
		handler = "mux"
	}
	return `package main

import (
` + imports + databaseDriverImport(options) + sessionBackendImport(options) + `
)

func main() {` + authBootstrap(options) + `
` + body + `	if err := pw.Run(context.Background(), ` + handler + `); err != nil {
		log.Fatal(err)
	}
}
`
}

// registeredRouterScaffold writes a handler package into directory: the mux the
// package owns, one route, and the page it renders.
//
// The directory is a parameter because it is a data:project-config value: the
// scaffold picks the default, and everything downstream reads generate.handlers
// rather than the name.
func registeredRouterScaffold(options initOptions, directory string) map[string]string {
	return map[string]string{
		directory + "/index.go":        muxScaffold(options),
		directory + "/home_handler.go": homeHandlerScaffold(options),
		directory + "/home.pw.html":    homeTemplateScaffold(options),
	}
}

// pageTreeScaffold writes a page tree into root: a layout, the root page, and
// one dynamic route whose Go entry point runs between the request and the
// render.
//
// The root is a parameter for the same reason the handler directory is: it is
// what generate.pages names, and a project may name it something else.
func pageTreeScaffold(options initOptions, root string) map[string]string {
	// The tree root is a Go package like every directory below it, so its name
	// follows the directory rather than the other way round.
	pkg := goPackageIdentifier(root)
	return map[string]string{
		// A layout must declare children as html: that shape is what makes the
		// template compiler emit the wrapper the generated chain calls.
		root + "/" + pwgen.LayoutFile: `package ` + pkg + `

export component Layout(children: html): html {
<div class="page"><slot required /></div>
}
`,
		// A directory holding this file is a route. This one is the tree root,
		// which serves GET /.
		root + "/" + pwgen.PageFile: `package ` + pkg + `

export component Page(): html {
<h1 class="text-3xl font-bold">Hello, Popcorn Wave</h1>
<p><a href="/greet/world">a page with a dynamic segment</a></p>
}
`,
		// One trailing underscore marks a dynamic segment, so this directory
		// serves GET /greet/{name}. Brackets are impossible: the directory is
		// also a Go package, and the toolchain rejects an illegal import path
		// element before it evaluates anything else.
		root + "/greet/name_/" + pwgen.PageFile: `package name_

export component Page(greeting: string): html {
<h1 class="text-3xl font-bold">{greeting}</h1>
}
`,
		// page.go is optional. Adding it puts Go between the request and the
		// render: these parameters are the route's dynamic segments in order,
		// and these results are the page component's parameters.
		root + "/greet/name_/page.go": `package name_

func Load(name string) (string, error) {
	return "Hello, " + name, nil
}
`,
	}
}

func muxScaffold(options initOptions) string {
	if options.TinyGo {
		return `package handlers

import "github.com/shibukawa/popcornwave/pw"

var mux = pw.NewServeMux()

func Handlers() *pw.ServeMux { return mux }
`
	}
	return `package handlers

import "net/http"

var mux = http.NewServeMux()

func Handlers() *http.ServeMux { return mux }
`
}

// scaffoldGenerationScope maps the starter directories onto the purposes that
// read them. The scaffold writes every purpose explicitly because none has a
// default, so what each purpose reads is readable from the first run. handlers
// appears twice because the starter page template sits beside its handler, and
// the main package carries the configuration the application registers.
func scaffoldGenerationScope(options initOptions) generationScope {
	scope := generationScope{
		// templates always holds the document shell and the error pages, which
		// both routers render through.
		Templates: []string{defaultTemplatesDir},
		Config:    []string{"cmd/" + options.Name},
	}
	if routerHasRegistered(options.Router) {
		scope.Handlers = []string{defaultRegisteredDir}
		// A page template sits beside the handler that renders it.
		scope.Templates = []string{defaultRegisteredDir, defaultTemplatesDir}
	}
	if routerHasDiscovered(options.Router) {
		// A tree root is one generation run, not a directory of independent
		// sources, so it is never listed under another purpose.
		scope.Pages = []string{defaultDiscoveredDir}
	}
	if options.Database {
		// A purpose may only name directories that exist, so a project without
		// the database says so with an empty list rather than a stale entry.
		scope.Queries = []string{"queries"}
	}
	return scope
}

// quotedList renders a string slice as the body of a TOML or JSON array.
func quotedList(values []string) string {
	quoted := make([]string, len(values))
	for index, value := range values {
		quoted[index] = strconv.Quote(value)
	}
	return strings.Join(quoted, ", ")
}

func errorTemplate(pkg, component, title string) string {
	return "package " + pkg + "\n\nexport component " + component + "(): html {\n" +
		"<h1>" + title + "</h1>\n}\n"
}

func frameworkModuleDirective() string {
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Path == "github.com/shibukawa/popcornwave" &&
		info.Main.Version != "" && info.Main.Version != "(devel)" && !strings.Contains(info.Main.Version, "+dirty") {
		return "require github.com/shibukawa/popcornwave " + info.Main.Version + "\n"
	}
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return "require github.com/shibukawa/popcornwave latest\n"
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
	return "require github.com/shibukawa/popcornwave v0.0.0\n\nreplace github.com/shibukawa/popcornwave => " + filepath.ToSlash(root) + "\n"
}
