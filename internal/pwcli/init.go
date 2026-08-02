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
	"sort"
	"strconv"
	"strings"

	"github.com/charmbracelet/x/term"
	"github.com/shibukawa/popcornwave/internal/pwenv"
	"github.com/shibukawa/popcornwave/internal/pwgen"
	"github.com/shibukawa/popcornwave/plugin/auth"
	"github.com/shibukawa/popcornwave/sessionstore"

	// The CLI writes a migration in any dialect, so it links every engine the
	// framework knows rather than the one this machine happens to run.
	_ "github.com/shibukawa/popcornwave/authstate/mysql"
	_ "github.com/shibukawa/popcornwave/authstate/postgres"
	_ "github.com/shibukawa/popcornwave/authstate/sqlite"
	_ "github.com/shibukawa/popcornwave/sessionstore/mysql"
	_ "github.com/shibukawa/popcornwave/sessionstore/postgres"
	_ "github.com/shibukawa/popcornwave/sessionstore/sqlite"
)

// Where the starter page sends a reader who wants more than it holds. Literal,
// because a landing page that needs a lookup to render is not a starting point.
const (
	documentationURL = "https://shibukawa.github.io/popcornwave/"
	repositoryURL    = "https://github.com/shibukawa/popcornwave"
)

const initUsage = "usage: pw init [<project-name>] [--yes] [--router=registered|discovered|both] [--tailwind] [--no-tinygo] [--no-devbox] [--no-database] [--db=sqlite|postgres|mysql] [--dynamo] [--no-redis] [--auth=none|oidc|oidc-passkey|passkey] [--session=rdb|cookie|redis] [--devidp]"

// Authentication modes the wizard and the --auth flag select between. They map
// onto the plugin/auth modes, with none meaning no [auth] configuration.
const (
	authNone        = "none"
	authOIDC        = "oidc"
	authOIDCPasskey = "oidc-passkey"
	authPasskey     = "passkey"
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
func usesOIDC(mode string) bool { return mode == authOIDC || mode == authOIDCPasskey }

// usesPasskey reports whether the mode mounts the ceremony endpoints, which
// decides whether the project needs a relying-party registration and the
// browser side that calls navigator.credentials.
func usesPasskey(mode string) bool { return mode == authOIDCPasskey || mode == authPasskey }

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
func sessionBackendPlugin(backend, engine string) string {
	switch backend {
	case sessionRDB:
		// A SQL store is one package per engine, because no engine reads
		// another's DDL.
		return "github.com/shibukawa/popcornwave/sessionstore/" + engineDialect(engine)
	case sessionRedis:
		return "github.com/shibukawa/popcornwave/sessionstore/redis"
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
	// defaultDynamoDir holds the dynamo-tagged types and .pw.dynamo queries of
	// requirement:dynamodb-store. It is its own purpose because a directory
	// contributes only the artifact kinds whose purpose lists it.
	defaultDynamoDir = "records"
)

// dynamoDevboxPackage is the local DynamoDB server pw dev starts, the address of
// which is what the scaffolded endpoint points at.
const dynamoDevboxPackage = "dynamodb-local@latest"

// dynamoStore names DynamoDB where a store is being chosen rather than an
// engine. It is not an engine name and never reaches project.database, because
// requirement:dynamodb-store is a second kind of store rather than a fourth
// dialect.
const dynamoStore = "dynamo"

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
	// AuthStore is the store the login was chosen through: an engine name, or
	// dynamoStore. It is a wizard answer rather than a scaffold input — what it
	// writes is already in Database, Engine, and Dynamo — and it exists so the
	// follow-up question knows which store the operator has not been asked
	// about yet.
	AuthStore string
	// Dynamo adds the DynamoDB store. It is a second kind of store rather than
	// a fourth SQL engine, so it stands beside the Database answer instead of
	// replacing it, and either, both, or neither is a valid project.
	Dynamo bool
	// Yes skips the wizard and takes the flags with the defaults for everything
	// they do not answer. It is the only way to run non-interactively in a
	// terminal, because the project name alone no longer means the caller has
	// answered anything but the name.
	Yes bool
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
		case "-y", "--yes":
			options.Yes = true
		case "-i", "--interactive":
			// Accepted and ignored: the wizard it used to request is now what
			// every terminal run does.
		case "--devbox":
			options.Devbox = true
		case "--no-devbox":
			options.Devbox = false
		case "--database":
			options.Database = true
		case "--no-database":
			options.Database = false
		case "--dynamo":
			options.Dynamo = true
		case "--no-dynamo":
			options.Dynamo = false
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
				case authNone, authOIDC, authOIDCPasskey, authPasskey:
					options.Auth = mode
				default:
					return initOptions{}, fmt.Errorf("init: --auth must be %s, %s, %s, or %s",
						authNone, authOIDC, authOIDCPasskey, authPasskey)
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
	// The wizard runs whether or not a name was given. A caller who knows the
	// name has still not answered the store, authentication, router, or
	// toolchain questions, and a question never asked is an option never
	// discovered. --yes is the way to say the flags are the whole answer, and a
	// session with no terminal is that case by construction.
	if !options.Yes && interactiveTerminal() {
		options, err = runInitWizard(options)
		if errors.Is(err, errWizardCanceled) {
			fmt.Fprintln(stdout, "init canceled")
			return nil
		}
		if err != nil {
			return err
		}
	} else if options.Name == "" {
		// Nothing can supply the name here: there is no wizard to ask in, and no
		// default worth guessing for a directory this command is about to create.
		return fmt.Errorf("init: a project name is required without the wizard; %s", initUsage)
	}
	name := options.Name
	destination, err := initDestination(name)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(destination, 0o755); err != nil {
		return err
	}
	progress := newProgressRegion(stdout)
	progress.Phase("writing " + name)
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
	progress.Phase("resolving modules")
	tidy := exec.Command("go", "mod", "tidy")
	tidy.Dir = destination
	tidy.Stdout = stdout
	tidy.Stderr = stdout
	tidy.Env = os.Environ()
	if err := tidy.Run(); err != nil {
		progress.Done()
		return fmt.Errorf("initialize Go module: %w", err)
	}
	progress.Phase("generating")
	previous, err := os.Getwd()
	if err != nil {
		return err
	}
	if err := os.Chdir(destination); err != nil {
		return err
	}
	generated, generateErr := generateProject(context.Background(), false, stdout, false)
	restoreErr := os.Chdir(previous)
	progress.Done()
	if generateErr != nil {
		return fmt.Errorf("generate starter: %w", generateErr)
	}
	if restoreErr != nil {
		return restoreErr
	}
	fmt.Fprintf(stdout, "\nCreated %s\n", name)
	reportCreatedSources(stdout, files)
	// The generated files are build inputs, named after sources the operator has
	// not read yet and excluded by the .gitignore this same scaffold wrote.
	// Listing them answers no question; saying how to remake them does.
	fmt.Fprintf(stdout, "\n%d generated files, rebuilt any time by pw generate\n", generated)
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

// reportCreatedSources names the handwritten files the scaffold wrote, grouped
// by the directory they landed in. These are the files the operator now owns,
// edits, and commits, which is what a report of a new project should be about.
func reportCreatedSources(stdout io.Writer, files map[string]string) {
	grouped := map[string][]string{}
	for path := range files {
		directory, file := "", path
		if cut := strings.LastIndex(path, "/"); cut >= 0 {
			directory, file = path[:cut], path[cut+1:]
		}
		grouped[directory] = append(grouped[directory], file)
	}
	directories := make([]string, 0, len(grouped))
	for directory := range grouped {
		directories = append(directories, directory)
	}
	// The project root sorts first as the empty string, which is where the
	// files an operator opens first happen to live.
	sort.Strings(directories)
	fmt.Fprintln(stdout)
	for _, directory := range directories {
		names := grouped[directory]
		sort.Strings(names)
		label := directory + "/"
		if directory == "" {
			label = "."
		}
		fmt.Fprintf(stdout, "  %-14s %s\n", label, strings.Join(names, "  "))
	}
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
		case capabilityDynamo:
			if !options.Dynamo {
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
	if options.Dynamo {
		// The local server the scaffolded endpoint points at, added the way the
		// Valkey and the SQL servers are.
		devboxPackages = append(devboxPackages, dynamoDevboxPackage)
	}
	if options.Redis {
		devboxPackages = append(devboxPackages, "valkey@latest")
	}
	if options.TinyGo {
		devboxPackages = append(devboxPackages, "tinygo@latest")
	}
	configTailwind := ""
	// Declining Tailwind costs the utilities, not the page: the starter page is
	// styled either way, by the toolchain or by a stylesheet the application
	// owns from the moment it is written.
	// public/app.css is written either way. With Tailwind it carries the error
	// pages only, because those are framework-shaped and their class names are
	// not utilities; without it, it carries the starter page as well.
	homeStylesheet := `<link rel="stylesheet" href="/public/app.css">`
	homeClasses := ""
	if options.Tailwind {
		configTailwind = tailwindProjectConfig()
		devboxPackages = append(devboxPackages, tailwindDevboxPackage)
		homeStylesheet += `<link rel="stylesheet" href="/public/generated/app.css">`
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
# The dynamo-tagged types and .pw.dynamo queries of the DynamoDB store, which is
# its own purpose because it shares no source kind with the SQL path.
dynamo = [` + quotedList(scaffoldGenerationScope(options).Dynamo) + `]

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
# The slog text encoding, which is what a terminal reads. An unset key already
# resolves to this in dev and to "json" everywhere else; it is written out here
# so the format this file produces is visible rather than inferred.
stdout_format = "plaintext"
` + databaseRuntimeConfig(options) + dynamoRuntimeConfig(options) + authRuntimeConfig(options),
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
		"templates/errors.go":   errorRegistrationScaffold(),
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
		".vscode/extensions.json": editorExtensionsScaffold(options),
		".editorconfig":           editorConfigScaffold(),
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
	files["public/app.css"] = applicationStylesheet(options)
	if options.Dynamo {
		files[defaultDynamoDir+"/note.go"] = dynamoRecordScaffold()
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
	// The emulator is an OpenID Provider, so a mode without one never gets a
	// roster even if the option survived from an earlier answer.
	if options.AuthEmulator && usesOIDC(options.Auth) {
		files[defaultIdPConfig] = devIdPRoster()
	}
	if usesPasskey(options.Auth) {
		files["public/passkey.js"] = passkeyBrowserScaffold(options)
	}
	if servesLogin(options) {
		files["public/presence.js"] = presenceBrowserScaffold()
	}
	if servesLogin(options) {
		files["handlers/accounts.go"] = accountsScaffold(options)
		// The framework tables come from the packages that own them. A fresh
		// project has only the application schema, so these take the versions
		// after it; pw add would take whatever is free at that point instead.
		// Only the rdb backend owns a table: a cookie or Redis session leaves
		// the numbering to the auth migration alone.
		version := 2
		dialect := engineDialect(options.Engine)
		if sessionBackend(options) == sessionRDB {
			// A migration is written in the dialect of the engine this project
			// selected, because no engine reads another's DDL.
			migration, err := sessionstore.MigrationSQL(dialect, "popcornwave_session")
			if err != nil {
				panic(err)
			}
			files["migrations/00002_"+sessionstore.MigrationName+".sql"] = migration
			version = 3
		}
		authMigration, err := auth.MigrationSQL(dialect)
		if err != nil {
			panic(err)
		}
		files[fmt.Sprintf("migrations/0000%d_%s.sql", version, auth.MigrationName)] = authMigration
	}
	return files
}

// servesLogin reports whether the framework mounts authentication endpoints for
// this project.
// servesLogin reports whether the framework mounts authentication endpoints for
// this project. An unset answer reads as none, the way effectiveRouter reads an
// unset router: a caller that never asked gets the shape it had before the
// question existed.
func servesLogin(options initOptions) bool {
	return options.Auth != "" && options.Auth != authNone
}

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

// dynamoRuntimeConfig is the middleware.dynamo section. It is independent of
// middleware.rdb: a project may have either, both, or neither, because DynamoDB
// is a second kind of store rather than another SQL engine.
func dynamoRuntimeConfig(options initOptions) string {
	if !options.Dynamo {
		return ""
	}
	return dynamoRuntimeSection()
}

// dynamoRuntimeSection is what api:cli-init scaffolds and api:cli-add appends,
// so both reach the same file state. The credentials are development values:
// the local server does not verify a signature, and the region is a placeholder
// it accepts.
func dynamoRuntimeSection() string {
	return `
# The DynamoDB store, independent of middleware.rdb. These values point at the
# amazon/dynamodb-local server pw dev starts; deployment supplies its own.
[middleware.dynamo]
enabled = true
region = "us-east-1"
endpoint = "http://127.0.0.1:8000"
access_key_id = "local"
secret_access_key = "local"
# Development creates tables from the registered definitions. A deployed table
# comes from deployment tooling, so this key is a configuration error outside dev.
auto_migrate = true
`
}

// dynamoRecordScaffold is the starter typed record. Its table is created from
// the generated definition rather than from a migration file, because the
// DynamoDB schema is the set of registered tables and has no version sequence.
func dynamoRecordScaffold() string {
	return `package records

import (
	"context"
	"time"

	"github.com/shibukawa/tinybind-go/dynamobind"
)

// A dynamo-tagged struct declares a table. pw generate emits its item codec,
// its key builder, and the table definition that creates the table in
// development, all beside this file and none of it by hand.
//
// There is no migration file for this: the DynamoDB schema is the set of
// registered table definitions, so a table is added by declaring a type rather
// than by adding a version.
type Note struct {
	ID        string    ` + "`dynamo:\"id,partitionkey\"`" + `
	CreatedAt time.Time ` + "`dynamo:\"created_at,sortkey\"`" + `
	Body      string    ` + "`dynamo:\"body\"`" + `
}

// The generator emits a codec only for the directions something actually uses,
// so these two calls are what make EncodeItem, DecodeItem, and ItemKey appear
// beside this file. Delete them and the generated code shrinks to match.
//
// The client comes from the request context, installed by the dynamo
// middleware, so nothing here takes a handle.
func StoreNote(ctx context.Context, note Note) error {
	return dynamobind.Store(ctx, "notes", note)
}

func LoadNote(ctx context.Context, id string, createdAt time.Time) (Note, error) {
	return dynamobind.Load[Note](ctx, "notes", Note{ID: id, CreatedAt: createdAt}.ItemKey())
}
`
}

// sessionBackendImport contributes the storage the configuration selects.
// Session storage is opt-in by blank import, so an application links the one
// backend it configured and nothing else. The cookie backend is built into pw
// and needs no line here.
func sessionBackendImport(options initOptions) string {
	if !servesLogin(options) {
		return ""
	}
	imports := ""
	if plugin := sessionBackendPlugin(sessionBackend(options), options.Engine); plugin != "" {
		imports += "\n\t// session.backend = \"" + sessionBackend(options) +
			"\" is served by this import; storage is opt-in.\n\t_ " + strconv.Quote(plugin)
	}
	// The login ceremony records live in the database whichever backend holds
	// the sessions, so their engine is imported for every login.
	imports += "\n\t// The single-use login records this engine stores.\n\t_ " +
		strconv.Quote("github.com/shibukawa/popcornwave/authstate/"+engineDialect(options.Engine))
	return imports
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
	return "\n\t// Installed before Run: the framework calls these while it serves a login.\n\thandlers.RegisterAccounts()"
}

// passkeyBrowserScaffold is the browser half of a ceremony. The framework
// serves the endpoints but cannot run navigator.credentials for the page, so a
// project needs this much script; it has no dependencies and is meant to be
// read and replaced.
func passkeyBrowserScaffold(options initOptions) string {
	bootstrap := ""
	if options.Auth == authPasskey {
		bootstrap = `
// redeemBootstrap trades an administrator-issued login ID and one-time secret
// for one restricted enrollment. It creates no session: finishing the
// registration is what signs the account in.
export async function redeemBootstrap(loginId, secret) {
  await post("/auth/passkey/bootstrap", { login_id: loginId, secret });
  return register();
}

wire("passkey-bootstrap", (form) => {
  const data = new FormData(form);
  return redeemBootstrap(data.get("login_id"), data.get("secret"));
});
`
	}
	return `// Passkey ceremonies, driven from the page.
//
// The framework serves /auth/passkey/*; this file only converts between the
// Base64url the endpoints speak and the ArrayBuffers the WebAuthn API wants,
// which is the whole reason a script is needed at all.

const decode = (value) => {
  const padded = value.replace(/-/g, "+").replace(/_/g, "/");
  const binary = atob(padded + "=".repeat((4 - (padded.length % 4)) % 4));
  return Uint8Array.from(binary, (c) => c.charCodeAt(0));
};

const encode = (buffer) =>
  btoa(String.fromCharCode(...new Uint8Array(buffer)))
    .replace(/\+/g, "-")
    .replace(/\//g, "_")
    .replace(/=+$/, "");

async function post(path, body) {
  const response = await fetch(path, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    // The endpoints are same-origin only, and a session cookie has to travel.
    credentials: "same-origin",
    body: JSON.stringify(body ?? {}),
  });
  if (!response.ok) {
    throw new Error("passkey: " + path + " failed with " + response.status);
  }
  return response.json();
}

// register adds a passkey to the account this browser is already allowed to
// enroll for.
export async function register() {
  const options = await post("/auth/passkey/register/begin");
  const credential = await navigator.credentials.create({
    publicKey: {
      ...options,
      challenge: decode(options.challenge),
      user: { ...options.user, id: decode(options.user.id) },
      excludeCredentials: (options.excludeCredentials ?? []).map((c) => ({
        ...c,
        id: decode(c.id),
      })),
    },
  });
  return post("/auth/passkey/register/finish", {
    id: credential.id,
    rawId: encode(credential.rawId),
    type: credential.type,
    response: {
      clientDataJSON: encode(credential.response.clientDataJSON),
      attestationObject: encode(credential.response.attestationObject),
      transports: credential.response.getTransports?.() ?? [],
    },
  });
}

// login signs in with a passkey. No user name is asked for: the credential
// itself names the account.
export async function login() {
  const options = await post("/auth/passkey/login/begin");
  const assertion = await navigator.credentials.get({
    publicKey: {
      ...options,
      challenge: decode(options.challenge),
      allowCredentials: (options.allowCredentials ?? []).map((c) => ({
        ...c,
        id: decode(c.id),
      })),
    },
  });
  return post("/auth/passkey/login/finish", {
    id: assertion.id,
    rawId: encode(assertion.rawId),
    type: assertion.type,
    response: {
      clientDataJSON: encode(assertion.response.clientDataJSON),
      authenticatorData: encode(assertion.response.authenticatorData),
      signature: encode(assertion.response.signature),
      userHandle: assertion.response.userHandle
        ? encode(assertion.response.userHandle)
        : undefined,
    },
  });
}

// wire binds a control by id, so the page needs no inline script and the
// template stays free of JavaScript.
function wire(id, run) {
  const element = document.getElementById(id);
  if (!element) return;
  const event = element.tagName === "FORM" ? "submit" : "click";
  element.addEventListener(event, async (e) => {
    e.preventDefault();
    try {
      await run(element);
      location.reload();
    } catch (error) {
      const status = document.getElementById("passkey-status");
      if (status) status.textContent = String(error.message ?? error);
    }
  });
}

wire("passkey-login", login);
wire("passkey-register", register);
` + bootstrap
}

// accountsScaffold wires the account seams the selected mode needs. Each one is
// a small function the framework calls; the application owns account storage.
func accountsScaffold(options initOptions) string {
	imports := "\t\"context\"\n\n\t\"github.com/shibukawa/popcornwave/plugin/auth\"\n"
	body := "// RegisterAccounts installs the account seams. Call it from main before\n// pw.Run.\nfunc RegisterAccounts() {\n"
	if usesOIDC(options.Auth) {
		body += "\tauth.SetAccountResolver(resolveAccount)\n"
	}
	if usesPasskey(options.Auth) {
		body += "\tauth.SetAccountLookup(lookupAccount)\n"
	}
	if options.Auth == authPasskey {
		body += "\tauth.SetAccountActivator(activateAccount)\n"
	}
	body += "}\n"
	if usesOIDC(options.Auth) {
		body += `
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
	if usesPasskey(options.Auth) {
		body += `
// lookupAccount answers with the account behind a stable identifier.
//
// A passkey assertion resolves a credential to an account ID, which is the
// opposite direction from resolveAccount, so the framework asks here instead.
// Replace it with a read from your own table; returning the identifier alone
// is enough to authenticate but shows the user no name.
func lookupAccount(ctx context.Context, accountID string) (auth.Account, error) {
	return auth.Account{ID: accountID, DisplayName: accountID}, nil
}
`
	}
	if options.Auth == authPasskey {
		body += `
// activateAccount marks a provisional account usable. The framework runs it
// inside the transaction that persists the first passkey, so an account never
// becomes active without a credential and never gains a credential without
// becoming active. Replace the body with your own UPDATE.
func activateAccount(ctx context.Context, accountID string) error {
	return nil
}

// IssueFirstPasskey provisions the login ID and one-time secret that open one
// passkey enrollment, and returns the secret for delivery out of band.
//
// The secret is returned exactly once and is never stored; only its digest is.
// Put this behind administrator authorization before exposing it: anyone who
// can call it can enroll a credential for any account.
func IssueFirstPasskey(ctx context.Context, loginID, accountID string) (string, error) {
	return auth.IssueBootstrapCredential(ctx, loginID, accountID, auth.PurposeInitialPasskey)
}
`
	}
	return "package handlers\n\nimport (\n" + imports + ")\n\n" + body
}

// homeHandlerScaffold renders the starter page. With authentication it reads
// the signed-in user; the login itself belongs to the framework.
func homeHandlerScaffold(options initOptions) string {
	project := strconv.Quote(options.Name)
	if !servesLogin(options) {
		return `package handlers

import (
	"net/http"

	"github.com/shibukawa/popcornwave/pw"
)

// homeInput is what this route reads from the request.
type homeInput struct {
	// Name is who the page greets. Anything the request does not carry falls
	// back to the declared default.
	Name string ` + "`query:\"name\" default:\"World\"`" + `
}

func init() { mux.HandleFunc("GET /{$}", home) }

// The comment below is not decoration. pw generate reads a handler's godoc into
// the OpenAPI document this project serves: the first sentence becomes the
// operation summary and the rest its description, and the field comments in
// homeInput describe the parameters. Write them and /docs explains the route;
// leave them out and it lists a path and nothing else.
//
// This paragraph is separated by a blank line, so it is not part of that godoc
// and does not reach the document.

// home renders the starter landing page.
//
// The greeting is whoever the request names, and the project the page was
// scaffolded for otherwise.
func home(w http.ResponseWriter, r *http.Request) {
	input, err := pw.Parse[homeInput](r)
	if err != nil {
		pw.WriteProblem(w, r, pw.BadRequest(err))
		return
	}
	pw.WriteHTML(w, r, Home(HomeParams{Name: input.Name, Project: ` + project + `}))
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

func init() { mux.HandleFunc("GET /{$}", home) }

// The comment below is not decoration. pw generate reads a handler's godoc into
// the OpenAPI document this project serves: the first sentence becomes the
// operation summary and the rest its description. Write them and /docs explains
// the route; leave them out and it lists a path and nothing else.
//
// This paragraph is separated by a blank line, so it is not part of that godoc
// and does not reach the document.

// home renders the starter landing page, signed in or not.
//
// A signed-in visitor is greeted by display name and offered a sign-out
// control; everyone else is offered the ways in that this project configured.
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
		Name:        name,
		Project:     ` + project + `,
		SignedIn:    signedIn,
		Email:       user.Email,
		LoginPath:   url.URL{Path: "/auth/login"},
		LogoutPath:  url.URL{Path: "/auth/logout"},
		Passkey:     ` + passkeyLiteral(usesPasskey(options.Auth)) + `,
		ProviderLogin: ` + passkeyLiteral(usesOIDC(options.Auth)) + `,
		Bootstrap:   ` + passkeyLiteral(options.Auth == authPasskey) + `,
	}))
}
`
}

// passkeyLiteral renders a Go bool literal for the scaffold.
func passkeyLiteral(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

// landingStyle names the class of each element on the starter page. Tailwind
// selected means utilities; declined means the class names the scaffolded
// stylesheet defines. The two tables exist so there is one template rather than
// one per answer, because two page scaffolds would drift apart.
type landingStyle struct {
	Page    string
	Eyebrow string
	Title   string
	Lead    string
	Section string
	Heading string
	List    string
	Link    string
	Note    string
}

func landingStyleFor(options initOptions) landingStyle {
	if options.Tailwind {
		return landingStyle{
			Page:    ` class="space-y-10"`,
			Eyebrow: ` class="text-sm font-semibold uppercase tracking-widest text-indigo-600"`,
			Title:   ` class="mt-1 text-4xl font-bold tracking-tight"`,
			Lead:    ` class="mt-3 text-slate-600"`,
			Section: ` class="rounded-xl border border-slate-200 p-6"`,
			Heading: ` class="text-lg font-semibold"`,
			List:    ` class="mt-3 space-y-2 text-slate-700"`,
			Link:    ` class="font-medium text-indigo-600 underline underline-offset-2"`,
			Note:    ` class="text-sm text-slate-500"`,
		}
	}
	return landingStyle{
		Page:    ` class="page"`,
		Eyebrow: ` class="eyebrow"`,
		Title:   ` class="title"`,
		Lead:    ` class="lead"`,
		Section: ` class="card"`,
		Heading: ` class="card-heading"`,
		List:    ` class="card-list"`,
		Link:    ` class="link"`,
		Note:    ` class="note"`,
	}
}

// landingStylesheet is the application-owned CSS a project that declined
// Tailwind gets. Declining the toolchain should cost the utilities, not the
// page, so the same structure is styled by hand instead of going unstyled.
// applicationStylesheet is the CSS the project owns. The error pages are always
// in it: their class names come from the framework's own templates rather than
// from a toolchain, so a project that took Tailwind still needs them defined.
func applicationStylesheet(options initOptions) string {
	if options.Tailwind {
		return errorPageStylesheet()
	}
	return landingStylesheet() + errorPageStylesheet()
}

// errorPageStylesheet styles the scaffolded error templates. It is small on
// purpose: an error page is rare, and the one thing it owes a reader is that the
// status and the title are legible.
func errorPageStylesheet() string {
	return `
.error { margin: 0 auto; max-width: 34rem; padding: 4rem 1.5rem; }
.error-status { color: #9aa3b8; font-size: 3rem; font-weight: 700; line-height: 1; margin: 0; }
.error-title { font-size: 1.5rem; margin: .5rem 0 0; }
.error-detail { margin: 1rem 0 0; }
.error-fields { margin: 1rem 0 0; padding-left: 1.25rem; }
.error-fields:empty { display: none; }
.error-code, .error-request { color: #5b6478; font-family: ui-monospace, monospace; font-size: .8rem; margin: 1.5rem 0 0; }
`
}

func landingStylesheet() string {
	return `:root { color-scheme: light dark; --edge: #d8dce4; --muted: #5b6478; --accent: #4f46e5; }
body { font: 16px/1.6 system-ui, sans-serif; margin: 0 auto; max-width: 46rem; padding: 3rem 1.5rem; }
.page > * + * { margin-top: 2.5rem; }
.eyebrow { color: var(--accent); font-size: .8rem; font-weight: 600; letter-spacing: .12em; text-transform: uppercase; }
.title { font-size: 2.25rem; letter-spacing: -.02em; margin: .25rem 0 0; }
.lead { color: var(--muted); margin: .75rem 0 0; }
.card { border: 1px solid var(--edge); border-radius: .75rem; padding: 1.5rem; }
.card-heading { font-size: 1.05rem; margin: 0; }
.card-list { color: var(--muted); margin: .75rem 0 0; padding-left: 1.25rem; }
.card-list li + li { margin-top: .5rem; }
.link { color: var(--accent); font-weight: 500; }
.note { color: var(--muted); font-size: .875rem; }
button, .button { border: 1px solid var(--edge); border-radius: .5rem; background: none; cursor: pointer; font: inherit; padding: .4rem .9rem; }
@media (prefers-color-scheme: dark) { :root { --edge: #333a4a; --muted: #9aa3b8; --accent: #a5b4fc; } }
`
}

// landingSections builds the body of the starter page. Every line of it comes
// from an answer this project was actually scaffolded with, so a declined
// capability is never advertised as present, and the links are literal because
// a landing page that needs a lookup to render is not a starting point.
func landingSections(options initOptions, style landingStyle, self string) string {
	var body strings.Builder
	fmt.Fprintf(&body, "  <section%s>\n    <h2%s>What this project has</h2>\n    <ul%s>\n",
		style.Section, style.Heading, style.List)
	for _, item := range landingIncluded(options) {
		fmt.Fprintf(&body, "      <li>%s</li>\n", item)
	}
	body.WriteString("    </ul>\n  </section>\n")

	fmt.Fprintf(&body, "  <section%s>\n    <h2%s>What to do next</h2>\n    <ul%s>\n",
		style.Section, style.Heading, style.List)
	for _, item := range landingNextSteps(options, self) {
		fmt.Fprintf(&body, "      <li>%s</li>\n", item)
	}
	body.WriteString("    </ul>\n  </section>\n")

	fmt.Fprintf(&body, `  <section%s>
    <h2%s>Documentation</h2>
    <ul%s>
      <li><a%s href="%s">Guides, tutorial, and reference</a></li>
      <li><a%s href="%s">Source and issues</a></li>
    </ul>
  </section>
`, style.Section, style.Heading, style.List, style.Link, documentationURL, style.Link, repositoryURL)
	return body.String()
}

// landingIncluded describes the project as it was scaffolded, one line per
// answer that wrote something.
func landingIncluded(options initOptions) []string {
	var items []string
	if routerHasRegistered(options.Router) {
		items = append(items, "Routes written in Go on <code>"+muxTypeName(options)+"</code>, registered from <code>"+defaultRegisteredDir+"/</code>")
	}
	if routerHasDiscovered(options.Router) {
		items = append(items, "A page tree in <code>"+defaultDiscoveredDir+"/</code>, where a directory with a page template is a route")
	}
	if options.Database {
		items = append(items, "A "+engineFor(options.Engine).Label+" database, with <code>migrations/</code> and typed SQL in <code>queries/</code>")
	}
	if options.Dynamo {
		items = append(items, "A DynamoDB store, with typed records in <code>"+defaultDynamoDir+"/</code>")
	}
	if servesLogin(options) {
		items = append(items, "Authentication, with the sign-in controls below served by the framework")
	}
	if options.Tailwind {
		items = append(items, "Tailwind CSS, compiled from <code>assets/app.css</code>")
	}
	if options.Devbox {
		items = append(items, "A Devbox environment pinning the toolchain and the development services")
	}
	return items
}

// landingNextSteps names the commands that do something to this project now.
func landingNextSteps(options initOptions, self string) []string {
	steps := []string{"Edit this page: <code>" + self + "</code>"}
	if routerHasRegistered(options.Router) {
		steps = append(steps, "Add a route: <code>pw new handler</code>")
	}
	if routerHasDiscovered(options.Router) {
		steps = append(steps, "Add a page: <code>pw new page</code>")
	}
	if options.Database {
		steps = append(steps, "Write a query in <code>queries/</code> and a migration in <code>migrations/</code>; both start out commented")
	}
	if declined := declinedCapabilities(options); len(declined) > 0 {
		steps = append(steps, "Install what this project skipped ("+strings.Join(declined, ", ")+"): <code>pw add &lt;capability&gt;</code>")
	}
	return steps
}

// muxTypeName names the router type this project's handlers register on.
func muxTypeName(options initOptions) string {
	if options.TinyGo {
		return "pw.ServeMux"
	}
	return "http.ServeMux"
}

// homeTemplateScaffold renders the starter page. The logout control is a form
// because the endpoint accepts POST only.
func homeTemplateScaffold(options initOptions) string {
	style := landingStyleFor(options)
	if !servesLogin(options) {
		return `package handlers

export component Home(name: string, project: string): html {
<div` + style.Page + `>
  <header>
    <p` + style.Eyebrow + `>Popcorn Wave</p>
    <h1` + style.Title + `>{project}</h1>
    <p` + style.Lead + `>Hello, {name}. This page is yours to delete; nothing in the framework reads it.</p>
  </header>
` + landingSections(options, style, "handlers/home.pw.html") + `</div>
}
`
	}
	return `package handlers

export component Home(name: string, project: string, signedIn: bool, email: string, loginPath: url, logoutPath: url, passkey: bool, providerLogin: bool, bootstrap: bool): html {
<div` + style.Page + `>
  <header>
    <p` + style.Eyebrow + `>Popcorn Wave</p>
    <h1` + style.Title + `>{project}</h1>
    <p` + style.Lead + `>Hello, {name}. This page is yours to delete; nothing in the framework reads it.</p>
  </header>
  <section` + style.Section + `>
    <h2` + style.Heading + `>Account</h2>
    {if signedIn}
      <p>Signed in as {email}</p>
      {if passkey}
        <p><button type="button" id="passkey-register">Add a passkey</button></p>
      {/if}
      <form method="post" action={logoutPath}>
        <button type="submit">Sign out</button>
      </form>
      <!-- Signing out keeps the session at the identity provider and makes the next
           sign-in here ask again, which is what auth.oidc.logout_scope defaults to.
           To offer a sign-out-everywhere control as well, set
           auth.oidc.allow_global_logout_request and add a second form posting to
           the same path with <input type="hidden" name="scope" value="global">. -->
    {else}
      {if providerLogin}
        <p><a` + style.Link + ` href={loginPath}>Sign in</a></p>
      {/if}
      {if passkey}
        <p><button type="button" id="passkey-login">Sign in with a passkey</button></p>
      {/if}
      {if bootstrap}
        <form id="passkey-bootstrap">
          <p` + style.Note + `>First sign-in: use the login ID and one-time secret an administrator issued.</p>
          <input name="login_id" placeholder="login ID">
          <input name="secret" type="password" placeholder="one-time secret">
          <button type="submit">Enroll a passkey</button>
        </form>
      {/if}
    {/if}
    {if passkey}
      <p id="passkey-status"></p>
      <script type="module" src="/public/passkey.js"></script>
    {/if}
  </section>
` + landingSections(options, style, "handlers/home.pw.html") + `</div>
}
`
}

// devIdPProjectConfig enables the development identity provider for pw dev.
func devIdPProjectConfig(options initOptions) string {
	if !options.AuthEmulator || !usesOIDC(options.Auth) {
		return ""
	}
	return `
[dev.idp]
enabled = true
config = "` + defaultIdPConfig + `"
# A fixed port keeps the issuer URL stable across restarts, and the issuer is
# half of what identifies an account: the scaffolded resolver builds an account
# ID out of the issuer and the subject. On an automatically reserved port every
# pw dev would hand the same person a new account, and everything stored against
# the old one would be gone. Change it if something else here already listens.
port = ` + strconv.Itoa(defaultIdPPort) + `
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
		return ""
	}
	// The browser token is opaque in every backend; this selects where the
	// record behind it lives. A backend other than cookie reaches the binary
	// through the blank import in main.
	section := `
[session]
enabled = true
backend = "` + sessionBackend(options) + `"
ttl = "12h"
idle_timeout = "1h"
cookie.name = "pw_session"
# Loopback development only. Keep secure = true everywhere else.
cookie.secure = false
` + sessionBackendConfig(options) + `

# The framework serves every authentication path itself, so the application
# registers no authentication route. Logout is POST only.
[auth]
enabled = true
mode = "` + authConfigMode(options.Auth) + `"
post_login_path = "/"
# Opt in per path; everything else stays public.
protection.include = []
protection.unauthenticated = "redirect"
`
	if usesPasskey(options.Auth) {
		section += authPasskeyConfig(options)
	}
	if usesOIDC(options.Auth) {
		section += authOIDCConfig(options)
	}
	return section
}

// authConfigMode maps the scaffold choice onto the plugin/auth mode name.
func authConfigMode(mode string) string {
	switch mode {
	case authOIDCPasskey:
		return "oidc_passkey"
	case authPasskey:
		return "passkey_only"
	default:
		return "oidc_only"
	}
}

// authPasskeyConfig writes the relying-party registration and the account
// lifecycle policies the selected mode requires.
func authPasskeyConfig(options initOptions) string {
	lifecycle := `
# Two login methods, so a lost passkey is recoverable through the provider.
recovery.policy = "oidc"
`
	if options.Auth == authPasskey {
		// Nothing can stand in for a provider, so both policies are explicit
		// and the issued credential is bounded.
		lifecycle = `
# No provider can stand in for either policy, so both are chosen here. An
# administrator issues the login ID and one-time secret that open a first
# enrollment; see handlers/accounts.go.
registration.policy = "administrator"
recovery.policy = "administrator"
# How long an issued secret stays redeemable, measured from issuance: it spans
# delivery, so it is the longer of the two.
bootstrap.issue_ttl = "24h"
# How long the enrollment stays open after a successful redemption: it spans one
# ceremony at the keyboard, so it is short. A redemption grants a ticket for one
# registration, not a session, so the request stays unauthenticated until the
# registration finishes.
bootstrap.enrollment_ttl = "10m"
bootstrap.max_attempts = 5
`
	}
	return lifecycle + `# How recently a request must have authenticated before it may add or remove a
# login method.
recent_auth_max_age = "5m"

# A relying party is scoped to a domain. "localhost" is a secure origin for
# WebAuthn without TLS, which is why development needs no certificate; an IP
# literal such as 127.0.0.1 can never be an RP ID.
[auth.passkey]
rp_id = "localhost"
rp_name = "` + options.Name + `"
origins = ["http://localhost:8080"]
user_verification = "required"
discoverable = "preferred"
`
}

// authOIDCConfig writes the provider registration. The values stay empty for
// the emulator because pw dev injects them, and the application refuses to
// start if neither the file nor the environment supplies them.
func authOIDCConfig(options initOptions) string {
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
[auth.oidc]` + provider + `
redirect_url = "` + authDevelopmentOrigin(options) + `/auth/callback"
scopes = ["profile", "email"]
identity_claim = "sub"
admission = "authenticated"
auto_provision = true
# What a logout does to the provider session, which is shared with every other
# application signed in through it.
#   reconfirm: keep it, and make the next login here ask again
#   global:    end it, signing the user out of those applications too
logout_scope = "reconfirm"
allow_loopback_http = ` + loopback + `
`
}

// authDevelopmentOrigin is the origin the browser uses in development. A
// passkey mode must be reached by name rather than by address, because the RP
// ID is a domain and the origin has to sit inside it.
func authDevelopmentOrigin(options initOptions) string {
	if usesPasskey(options.Auth) {
		return "http://localhost:8080"
	}
	return "http://127.0.0.1:8080"
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

func main() {
	// Names the API document served at server.openapi_path and shown by the
	// reference UI at /docs. Without it both fall back to "Application API".
	if err := pw.SetOpenAPIInfo(pw.OpenAPIInfo{Title: "` + name + `", Version: "0.1.0"}); err != nil {
		log.Fatal(err)
	}
` + authBootstrap(options) + `
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
	style := landingStyleFor(options)
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
<div` + style.Page + `>
  <header>
    <p` + style.Eyebrow + `>Popcorn Wave</p>
    <h1` + style.Title + `>` + options.Name + `</h1>
    <p` + style.Lead + `>A directory holding a page template is a route, so <a` + style.Link + ` href="/greet/world">/greet/world</a> is served by the directory beside this one.</p>
  </header>
` + landingSections(options, style, root+"/"+pwgen.PageFile) + `</div>
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
	if options.Dynamo {
		scope.Dynamo = []string{defaultDynamoDir}
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

// presenceBrowserScaffold writes the browser half of auth.assurance.presence.
//
// Include it from a page only when that setting is on. It reports one bit per
// tick, so nothing about what the user did leaves the browser and behavioral
// analysis is impossible rather than merely discouraged.
func presenceBrowserScaffold() string {
	return `// Reports whether anybody is at the keyboard, so a session ends when a person
// leaves rather than when requests stop arriving. Those are different things: a
// page holding a live connection keeps requesting with nobody there, and a
// person reading one page for an hour requests nothing at all.
//
// What is sent is one boolean per tick and, when the clock jumped, how far.
// No key, no coordinate, and no timing pattern leaves this file.

const INTERVAL_MS = 60_000;

// Set by any input and cleared by each report, so the whole of the state is
// "did anything happen since last time".
let active = false;
let lastTick = Date.now();

const mark = () => { active = true; };
for (const type of ["pointerdown", "pointermove", "keydown", "wheel", "scroll", "touchstart"]) {
  // Passive, and it returns immediately once the flag is set, so even
  // pointermove costs nothing measurable.
  addEventListener(type, mark, { passive: true });
}
// A tab becoming visible is interaction; becoming hidden is not, and is left to
// the tick to notice.
addEventListener("visibilitychange", () => { if (!document.hidden) mark(); });

async function tick() {
  const now = Date.now();
  // Nothing reports a machine waking. A gap far larger than the interval is
  // how it is inferred, and it counts as absence rather than as presence.
  const gap = Math.max(0, Math.round((now - lastTick - INTERVAL_MS) / 1000));
  lastTick = now;
  const report = { active, gap };
  active = false;
  try {
    const response = await fetch("/auth/logout/presence", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify(report),
      credentials: "same-origin",
    });
    // The server ends the session when it decides nobody is here. Reloading
    // lands on whatever an anonymous visitor sees.
    if (response.status === 401 || response.status === 403) location.reload();
  } catch {
    // A failed report is not a presence claim. The server treats silence as
    // absence, which is the safe direction.
  }
}

setInterval(tick, INTERVAL_MS);
`
}

// editorConfigScaffold states the indent, encoding, and line endings the
// scaffolded sources are written with. The Go rule restates what gofmt already
// does, so an editor with no Go support does not fight it, and the two-space
// rule is the width a .pw.html block is indented by.
func editorConfigScaffold() string {
	return `root = true

[*]
charset = utf-8
end_of_line = lf
insert_final_newline = true
trim_trailing_whitespace = true

[*.go]
indent_style = tab

[*.{pw.html,html,pw.sql,sql,toml,json,css,js,mjs,yaml,yml}]
indent_style = space
indent_size = 2
`
}

// editorExtensionsScaffold recommends the extensions the scaffolded sources
// need to be edited the way they were written. Recommendations only: nothing is
// listed as unwanted, and nothing here is required to build the project.
func editorExtensionsScaffold(options initOptions) string {
	recommendations := []string{"golang.go", "EditorConfig.EditorConfig"}
	// A declined capability is never advertised as present, so a project
	// without Tailwind is not told to install its extension.
	if options.Tailwind {
		recommendations = append(recommendations, "bradlc.vscode-tailwindcss")
	}
	var body strings.Builder
	body.WriteString("{\n    \"recommendations\": [\n")
	for index, name := range recommendations {
		separator := ","
		if index == len(recommendations)-1 {
			separator = ""
		}
		fmt.Fprintf(&body, "        %q%s\n", name, separator)
	}
	body.WriteString("    ]\n}\n")
	return body.String()
}

// errorRegistrationScaffold connects the error templates to the framework.
// Without it they are generated and never reached, which is what they were
// before this file existed.
func errorRegistrationScaffold() string {
	return `package templates

import "github.com/shibukawa/popcornwave/pw"

// The framework renders one of these when a request fails and the client would
// rather have a page than a problem document. It also renders one in place of a
// page whose async boundary failed with no recover clause.
//
// The problem arrives already bounded: outside development it carries the
// status and the title only, so nothing here has to decide what is safe to
// show. Add a status to the switch and the framework starts using it.
func init() {
	pw.RegisterHTMLErrorPage(func(problem pw.Problem) pw.HTMLFragment {
		fields := make([]string, 0, len(problem.Fields))
		for _, field := range problem.Fields {
			fields = append(fields, field.Field+": "+field.Message)
		}
		switch problem.Status {
		case 400:
			return Error400(Error400Params{Status: problem.Status, Title: problem.Title,
				Detail: problem.Message, Code: problem.Code, Fields: fields})
		case 401:
			return Error401(Error401Params{Status: problem.Status, Title: problem.Title,
				Detail: problem.Message, Code: problem.Code, Fields: fields})
		case 403:
			return Error403(Error403Params{Status: problem.Status, Title: problem.Title,
				Detail: problem.Message, Code: problem.Code, Fields: fields})
		case 404:
			return Error404(Error404Params{Status: problem.Status, Title: problem.Title,
				Detail: problem.Message, Code: problem.Code, Fields: fields})
		case 409:
			return Error409(Error409Params{Status: problem.Status, Title: problem.Title,
				Detail: problem.Message, Code: problem.Code, Fields: fields})
		case 413:
			return Error413(Error413Params{Status: problem.Status, Title: problem.Title,
				Detail: problem.Message, Code: problem.Code, Fields: fields})
		default:
			return Error500(Error500Params{Status: problem.Status, Title: problem.Title,
				Detail: problem.Message, Code: problem.Code, Fields: fields})
		}
	})
}
`
}

func errorTemplate(pkg, component, title string) string {
	// The parameters are the api:error-renderer model. How much of it arrives
	// filled in is the framework's decision, not this template's: outside
	// development it hands over the status and the title and nothing else, so
	// the same file serves a developer and the public without a branch here.
	return "package " + pkg + `

export component ` + component + `(status: int, title: string, detail: string, code: string, requestId: string, fields: string[]): html {
  <main class="error">
    <p class="error-status">{status}</p>
    <h1 class="error-title">` + title + `</h1>
    {if detail != ''}<p class="error-detail">{detail}</p>{/if}
    <ul class="error-fields">
    {for field in fields}
      <li>{field}</li>
    {/for}
    </ul>
    {if code != ''}<p class="error-code">{code}</p>{/if}
    {if requestId != ''}<p class="error-request">Request {requestId}</p>{/if}
  </main>
}
`
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
