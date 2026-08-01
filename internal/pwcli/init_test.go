package pwcli

import (
	"errors"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/shibukawa/popcornwave/internal/pwgen"
	"github.com/shibukawa/tinybind-go/generator"
	"github.com/shibukawa/tinybind-go/templates/sqlbind"
)

func TestParseInitArgs(t *testing.T) {
	for _, testcase := range []struct {
		name string
		args []string
		want initOptions
	}{
		{name: "name only keeps the TinyGo default", args: []string{"demo"}, want: initOptions{Name: "demo", TinyGo: true, Database: true, Engine: engineSQLite, Redis: true, Devbox: true, Auth: authNone, Session: sessionRDB}},
		{name: "shortcut flags", args: []string{"demo", "--tailwind", "--no-tinygo"}, want: initOptions{Name: "demo", Tailwind: true, Database: true, Engine: engineSQLite, Redis: true, Devbox: true, Auth: authNone, Session: sessionRDB}},
		{name: "explicit tinygo", args: []string{"--tinygo", "demo"}, want: initOptions{Name: "demo", TinyGo: true, Database: true, Engine: engineSQLite, Redis: true, Devbox: true, Auth: authNone, Session: sessionRDB}},
		{name: "no name requests the wizard", args: nil, want: initOptions{TinyGo: true, Database: true, Engine: engineSQLite, Redis: true, Devbox: true, Auth: authNone, Session: sessionRDB}},
		{name: "interactive with a seeded name", args: []string{"-i", "demo"}, want: initOptions{Name: "demo", TinyGo: true, Database: true, Engine: engineSQLite, Redis: true, Devbox: true, Interactive: true, Auth: authNone, Session: sessionRDB}},
		{name: "oidc with the local emulator", args: []string{"demo", "--auth=oidc", "--devidp"}, want: initOptions{Name: "demo", TinyGo: true, Database: true, Engine: engineSQLite, Redis: true, Devbox: true, Auth: authOIDC, AuthEmulator: true, Session: sessionRDB}},
		{name: "passkey drops a stray emulator flag", args: []string{"demo", "--auth=passkey", "--devidp"}, want: initOptions{Name: "demo", TinyGo: true, Database: true, Engine: engineSQLite, Redis: true, Devbox: true, Auth: authPasskey, Session: sessionRDB}},
		{name: "engine shortcut", args: []string{"demo", "--db=postgres"}, want: initOptions{Name: "demo", TinyGo: true, Database: true, Engine: enginePostgres, Redis: true, Devbox: true, Auth: authNone, Session: sessionRDB}},
		{name: "mysql engine shortcut", args: []string{"demo", "--db=mysql"}, want: initOptions{Name: "demo", TinyGo: true, Database: true, Engine: engineMySQL, Redis: true, Devbox: true, Auth: authNone, Session: sessionRDB}},
		{name: "declined database keeps the default engine unapplied", args: []string{"demo", "--no-database"}, want: initOptions{Name: "demo", TinyGo: true, Engine: engineSQLite, Redis: true, Devbox: true, Auth: authNone, Session: sessionRDB}},
	} {
		t.Run(testcase.name, func(t *testing.T) {
			options, err := parseInitArgs(testcase.args)
			if err != nil {
				t.Fatal(err)
			}
			if options != testcase.want {
				t.Fatalf("options = %#v, want %#v", options, testcase.want)
			}
		})
	}
}

func TestParseInitArgsRejectsBadInput(t *testing.T) {
	for _, args := range [][]string{
		{"--unknown"},
		{"one", "two"},
		{"demo", "--db=oracle"},
		// The engine answer applies only inside a project that has a database,
		// so selecting one while declining the database is a contradiction
		// rather than a value to silently drop.
		{"demo", "--db=postgres", "--no-database"},
	} {
		if _, err := parseInitArgs(args); err == nil {
			t.Fatalf("parseInitArgs(%v) accepted invalid input", args)
		}
	}
}

// TestScaffoldPerEngine asserts that one answer moves the DSN, the schema
// dialect, the development server, and the driver import together.
func TestScaffoldPerEngine(t *testing.T) {
	for _, testcase := range []struct {
		engine         string
		dsn            string
		schemaFragment string
		devboxPackage  string
		driverImport   string
		maxOpenConns   string
	}{
		{
			engine:         engineSQLite,
			dsn:            `dsn = "sqlite://demo.db"`,
			schemaFragment: "name TEXT NOT NULL",
			maxOpenConns:   "max_open_conns = 1",
		},
		{
			engine:         enginePostgres,
			dsn:            `dsn = "postgres://demo:demo@127.0.0.1:5432/demo?sslmode=disable"`,
			schemaFragment: "name TEXT NOT NULL",
			devboxPackage:  "postgresql@latest",
			driverImport:   "github.com/shibukawa/popcornwave/database/postgres",
			maxOpenConns:   "max_open_conns = 10",
		},
		{
			engine:         engineMySQL,
			dsn:            `dsn = "mysql://demo:demo@tcp(127.0.0.1:3306)/demo"`,
			schemaFragment: "name VARCHAR(255) NOT NULL",
			devboxPackage:  "mysql80@latest",
			driverImport:   "github.com/shibukawa/popcornwave/database/mysql",
			maxOpenConns:   "max_open_conns = 10",
		},
	} {
		t.Run(testcase.engine, func(t *testing.T) {
			options := defaultInitOptions()
			options.Name = "demo"
			options.Engine = testcase.engine
			files := scaffoldFiles(options)

			config := files["config.dev.toml"]
			if !strings.Contains(config, testcase.dsn) {
				t.Fatalf("config does not carry %s:\n%s", testcase.dsn, config)
			}
			if !strings.Contains(config, testcase.maxOpenConns) {
				t.Fatalf("config does not carry %s:\n%s", testcase.maxOpenConns, config)
			}
			if schema := files["migrations/00001_init.sql"]; !strings.Contains(schema, testcase.schemaFragment) {
				t.Fatalf("schema does not carry %q:\n%s", testcase.schemaFragment, schema)
			}
			main := files["cmd/demo/main.go"]
			if testcase.driverImport == "" {
				if strings.Contains(main, "popcornwave/database/") {
					t.Fatalf("main links an engine pw already carries:\n%s", main)
				}
			} else if !strings.Contains(main, testcase.driverImport) {
				t.Fatalf("main does not link %s:\n%s", testcase.driverImport, main)
			}
			devbox := files["devbox.json"]
			if testcase.devboxPackage != "" && !strings.Contains(devbox, testcase.devboxPackage) {
				t.Fatalf("devbox.json does not declare %s:\n%s", testcase.devboxPackage, devbox)
			}
			if _, wrote := files["queries/users.pw.sql"]; !wrote {
				t.Fatal("no query example was written")
			}
			// A generate purpose may only name a directory that exists.
			if queries := scaffoldGenerationScope(options).Queries; len(queries) == 0 {
				t.Fatal("generate.queries names no directory for the query example")
			}
			// Generation reads the dialect from here, so a project states its
			// engine once and both the DSN and the placeholders follow.
			project := files["popcornwave.toml"]
			if !strings.Contains(project, `database = "`+testcase.engine+`"`) {
				t.Fatalf("popcornwave.toml does not record the engine:\n%s", project)
			}
		})
	}
}

// TestScaffoldDeclinedDatabaseIgnoresEngine asserts a skipped question never
// applies its answer, even when a default is sitting in the options.
func TestScaffoldDeclinedDatabaseIgnoresEngine(t *testing.T) {
	options := defaultInitOptions()
	options.Name = "demo"
	options.Database = false
	options.Engine = enginePostgres
	files := scaffoldFiles(options)

	if strings.Contains(files["config.dev.toml"], "[middleware.rdb]") {
		t.Fatal("a declined database still wrote an rdb section")
	}
	if strings.Contains(files["cmd/demo/main.go"], "popcornwave/database/") {
		t.Fatal("a declined database still linked an engine")
	}
	if strings.Contains(files["devbox.json"], "postgresql@") {
		t.Fatal("a declined database still declared a development server")
	}
	if _, wrote := files["migrations/00001_init.sql"]; wrote {
		t.Fatal("a declined database still wrote a migration")
	}
}

func TestMainInitWizardNeedsTerminal(t *testing.T) {
	var output strings.Builder
	if code := Main([]string{"init", "-i", "demo"}, &output, &output); code != 1 {
		t.Fatalf("code = %d, output = %q", code, output.String())
	}
	if !strings.Contains(output.String(), "needs a terminal") {
		t.Fatalf("unexpected error: %q", output.String())
	}
}

func TestScaffoldFilesWithoutTinyGoUsesStandardServeMux(t *testing.T) {
	files := scaffoldFiles(initOptions{Name: "fixture", Devbox: true})
	index := files["handlers/index.go"]
	for _, want := range []string{`import "net/http"`, "http.NewServeMux()", "func Handlers() *http.ServeMux"} {
		if !strings.Contains(index, want) {
			t.Errorf("handlers/index.go does not contain %q:\n%s", want, index)
		}
	}
	if strings.Contains(index, "popcornwave/pw") {
		t.Errorf("host-only scaffold still routes through pw:\n%s", index)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "handlers/index.go", index, parser.AllErrors); err != nil {
		t.Fatalf("scaffold is invalid Go: %v\n%s", err, index)
	}
	if !strings.Contains(files["popcornwave.toml"], `toolchain = "go"`) {
		t.Errorf("popcornwave.toml does not record the host toolchain:\n%s", files["popcornwave.toml"])
	}
	if strings.Contains(files["devbox.json"], "tinygo") {
		t.Errorf("devbox.json installs TinyGo for a host-only project:\n%s", files["devbox.json"])
	}
	// The handler API is toolchain independent; only routing changes.
	if !strings.Contains(files["handlers/home_handler.go"], "pw.WriteHTML(w, r,") {
		t.Errorf("handler scaffold lost its pw rendering:\n%s", files["handlers/home_handler.go"])
	}
	if !strings.Contains(files["cmd/fixture/main.go"], "pw.Run(context.Background(), handlers.Handlers())") {
		t.Errorf("entry point scaffold changed:\n%s", files["cmd/fixture/main.go"])
	}
}

func TestScaffoldFilesWithTinyGoUsesPwServeMux(t *testing.T) {
	files := scaffoldFiles(initOptions{Name: "fixture", TinyGo: true, Devbox: true})
	index := files["handlers/index.go"]
	for _, want := range []string{"pw.NewServeMux()", "func Handlers() *pw.ServeMux"} {
		if !strings.Contains(index, want) {
			t.Errorf("handlers/index.go does not contain %q:\n%s", want, index)
		}
	}
	if !strings.Contains(files["popcornwave.toml"], `toolchain = "tinygo"`) {
		t.Errorf("popcornwave.toml does not record the TinyGo toolchain:\n%s", files["popcornwave.toml"])
	}
	if !strings.Contains(files["devbox.json"], `"tinygo@latest"`) {
		t.Errorf("devbox.json is missing the TinyGo toolchain:\n%s", files["devbox.json"])
	}
	helper := files["tinygohelper.go"]
	for _, want := range []string{
		"//go:build tinygo\n",
		"package publicassets",
		`import _ "github.com/shibukawa/tinygodriver/netdev"`,
	} {
		if !strings.Contains(helper, want) {
			t.Errorf("tinygohelper.go does not contain %q:\n%s", want, helper)
		}
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "tinygohelper.go", helper, parser.AllErrors); err != nil {
		t.Fatalf("tinygohelper.go is invalid Go: %v\n%s", err, helper)
	}
}

// The netdev registration only matters under TinyGo, and the host-only scaffold
// should not carry a build-tagged file that never compiles.
func TestScaffoldFilesWithoutTinyGoOmitsNetdevHelper(t *testing.T) {
	files := scaffoldFiles(initOptions{Name: "fixture"})
	if _, ok := files["tinygohelper.go"]; ok {
		t.Errorf("host-only scaffold includes tinygohelper.go:\n%s", files["tinygohelper.go"])
	}
}

func TestScaffoldConfigLoadsBackForBothToolchains(t *testing.T) {
	for _, tinygo := range []bool{true, false} {
		root := t.TempDir()
		options := initOptions{Name: "fixture", TinyGo: tinygo, Tailwind: true, Devbox: true}
		scope := scaffoldGenerationScope(options)
		for _, sources := range [][]string{scope.Handlers, scope.Templates, scope.Queries, scope.Config} {
			for _, source := range sources {
				if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(source)), 0o755); err != nil {
					t.Fatal(err)
				}
			}
		}
		writeTestFile(t, filepath.Join(root, "popcornwave.toml"), scaffoldFiles(options)["popcornwave.toml"])
		config, err := loadProjectConfig(root)
		if err != nil {
			t.Fatalf("tinygo=%v: %v", tinygo, err)
		}
		if config.Toolchain != projectToolchain(options) {
			t.Errorf("tinygo=%v: toolchain = %q", tinygo, config.Toolchain)
		}
	}
}

// Route discovery has to cover both mux types, otherwise a host-only project
// would silently generate no route metadata.
func TestGeneratorDiscoversBothServeMuxTypes(t *testing.T) {
	options, err := pwgen.Options(sqlbind.DialectPostgreSQL)
	if err != nil {
		t.Fatal(err)
	}
	if options.ServeMuxes.Disabled {
		t.Fatal("ServeMux discovery is disabled")
	}
	want := []generator.TypePattern{
		{PackagePath: "net/http", Name: "ServeMux"},
		{PackagePath: "github.com/shibukawa/tinygodriver/httpmux", Name: "ServeMux"},
	}
	for _, pattern := range want {
		found := false
		for _, discovered := range options.ServeMuxes.Set {
			if discovered == pattern {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%s.%s is not discovered: %#v", pattern.PackagePath, pattern.Name, options.ServeMuxes.Set)
		}
	}
}

func TestInitWizardCollectsAnswers(t *testing.T) {
	t.Chdir(t.TempDir())
	model := newTestWizard(defaultInitOptions())
	model = feedWizard(t, model,
		typeText("demo"), pressKey(tea.KeyEnter), // project name
		pressKey(tea.KeyDown), pressKey(tea.KeyEnter), // TinyGo: No
		pressKey(tea.KeyEnter), // Router: keep Registered
		pressKey(tea.KeyEnter), // Tailwind: keep No
		pressKey(tea.KeyEnter), // Database: keep Yes
		pressKey(tea.KeyEnter), // Database engine: keep SQLite
		pressKey(tea.KeyEnter), // Authentication: keep None
		pressKey(tea.KeyEnter), // Devbox: keep Yes
		pressKey(tea.KeyEnter), // Redis or Valkey: keep Yes
		pressKey(tea.KeyEnter), // review
	)
	if !model.confirmed {
		t.Fatalf("wizard did not confirm: index = %d", model.index)
	}
	options := wizardResult(model, defaultInitOptions())
	if options != (initOptions{Name: "demo", Router: routerRegistered, Devbox: true, Database: true, Engine: engineSQLite, Redis: true, Auth: authNone, Session: sessionRDB}) {
		t.Fatalf("options = %#v", options)
	}
}

func TestInitWizardDigitShortcutSelectsTailwind(t *testing.T) {
	t.Chdir(t.TempDir())
	model := newTestWizard(defaultInitOptions())
	model = feedWizard(t, model,
		typeText("demo"), pressKey(tea.KeyEnter),
		typeText("1"),          // TinyGo: Yes
		typeText("2"),          // Router: discovered pages
		typeText("1"),          // Tailwind: Yes
		typeText("1"),          // Database: Yes
		typeText("1"),          // Database engine: SQLite
		typeText("1"),          // Authentication: None
		typeText("1"),          // Devbox: Yes
		typeText("1"),          // Redis or Valkey: Yes
		pressKey(tea.KeyEnter), // review
	)
	if !model.confirmed {
		t.Fatalf("wizard did not confirm: index = %d", model.index)
	}
	options := wizardResult(model, defaultInitOptions())
	if options != (initOptions{Name: "demo", Router: routerDiscovered, TinyGo: true, Tailwind: true, Devbox: true, Database: true, Engine: engineSQLite, Redis: true, Auth: authNone, Session: sessionRDB}) {
		t.Fatalf("options = %#v", options)
	}
}

func TestInitWizardSeedsAnswersFromShortcutFlags(t *testing.T) {
	steps := initWizardSteps(initOptions{
		Name: "seeded", Router: routerBoth, TinyGo: true, Tailwind: true, Devbox: true,
		Database: true, Engine: engineSQLite, Redis: true, Auth: authOIDC,
		Session: sessionRedis, AuthEmulator: true,
	})
	want := []string{
		"seeded", "Yes", "Both", "Yes", "Yes", "SQLite", "OIDC",
		"Redis or Valkey", "Local emulator", "Yes", "Yes",
	}
	if len(steps) != len(want) {
		t.Fatalf("wizard has %d steps, want %d", len(steps), len(want))
	}
	for index, step := range steps {
		if step.value() != want[index] {
			t.Errorf("step %d (%s) value = %q, want %q", index, step.label(), step.value(), want[index])
		}
	}
}

func TestInitWizardRejectsUnusableName(t *testing.T) {
	t.Chdir(t.TempDir())
	model := feedWizard(t, newTestWizard(defaultInitOptions()), pressKey(tea.KeyEnter))
	if model.index != 0 {
		t.Fatalf("wizard advanced past an empty name: index = %d", model.index)
	}
	if !strings.Contains(model.View(), "a project name is required") {
		t.Fatalf("missing validation message:\n%s", model.View())
	}

	model = feedWizard(t, newTestWizard(defaultInitOptions()), typeText("has space"), pressKey(tea.KeyEnter))
	if model.index != 0 {
		t.Fatalf("wizard accepted an invalid name: index = %d", model.index)
	}
}

func TestInitWizardGoesBackAndCancels(t *testing.T) {
	t.Chdir(t.TempDir())
	model := feedWizard(t, newTestWizard(defaultInitOptions()),
		typeText("demo"), pressKey(tea.KeyEnter),
		pressKey(tea.KeyEsc),
	)
	if model.index != 0 {
		t.Fatalf("esc did not return to the previous step: index = %d", model.index)
	}
	if model = feedWizard(t, model, pressKey(tea.KeyEsc)); model.canceled {
		t.Fatal("esc on the first step discarded the wizard")
	}
	canceled := feedWizard(t, newTestWizard(defaultInitOptions()), pressKey(tea.KeyCtrlC))
	if !canceled.canceled {
		t.Fatal("ctrl+c did not cancel")
	}
	if view := canceled.View(); view != "" {
		t.Fatalf("canceled wizard left output: %q", view)
	}
}

func TestInitWizardReviewListsEveryStep(t *testing.T) {
	t.Chdir(t.TempDir())
	model := feedWizard(t, newTestWizard(defaultInitOptions()),
		typeText("demo"), pressKey(tea.KeyEnter),
		pressKey(tea.KeyEnter),
		pressKey(tea.KeyEnter),
		pressKey(tea.KeyEnter),
		pressKey(tea.KeyEnter),
		pressKey(tea.KeyEnter),
		pressKey(tea.KeyEnter),
		pressKey(tea.KeyEnter),
		pressKey(tea.KeyEnter),
	)
	view := model.View()
	if !strings.Contains(view, "Review") {
		t.Fatalf("review screen missing:\n%s", view)
	}
	for _, index := range model.activeSteps() {
		if !strings.Contains(view, model.steps[index].label()) {
			t.Errorf("review screen omits %q:\n%s", model.steps[index].label(), view)
		}
	}
	if strings.Contains(view, "OIDC provider") {
		t.Errorf("review screen lists a skipped step:\n%s", view)
	}
}

// TestRunInitWizardOverKeystrokes exercises the real Bubble Tea program, so the
// wired-up input handling and result extraction stay covered.
func TestRunInitWizardOverKeystrokes(t *testing.T) {
	t.Chdir(t.TempDir())
	// demo, enter, down, enter (TinyGo: No), enter (router), enter (Tailwind),
	// enter (database), enter (engine), enter (auth), enter (devbox),
	// enter (redis), enter (review)
	keystrokes := "demo\r\x1b[B\r\r\r\r\r\r\r\r\r"
	options, err := runInitWizard(defaultInitOptions(),
		tea.WithInput(strings.NewReader(keystrokes)),
		tea.WithOutput(io.Discard),
		tea.WithoutRenderer(),
		tea.WithoutSignalHandler(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if options != (initOptions{Name: "demo", Router: routerRegistered, Devbox: true, Database: true, Engine: engineSQLite, Redis: true, Auth: authNone, Session: sessionRDB}) {
		t.Fatalf("options = %#v", options)
	}
}

func TestRunInitWizardReportsCancellation(t *testing.T) {
	t.Chdir(t.TempDir())
	_, err := runInitWizard(defaultInitOptions(),
		tea.WithInput(strings.NewReader("demo\r\x03")),
		tea.WithOutput(io.Discard),
		tea.WithoutRenderer(),
		tea.WithoutSignalHandler(),
	)
	if !errors.Is(err, errWizardCanceled) {
		t.Fatalf("err = %v, want %v", err, errWizardCanceled)
	}
}

func newTestWizard(defaults initOptions) wizardModel[initOptions] {
	model := newInitWizard(defaults)
	model.Init() // Focus the first step the way the Bubble Tea runtime does.
	return model
}

func wizardResult(model wizardModel[initOptions], defaults initOptions) initOptions {
	model.defaults = defaults
	return model.answers()
}

func feedWizard(t *testing.T, model wizardModel[initOptions], messages ...tea.Msg) wizardModel[initOptions] {
	t.Helper()
	for _, message := range messages {
		next, _ := model.Update(message)
		updated, ok := next.(wizardModel[initOptions])
		if !ok {
			t.Fatalf("Update returned %T", next)
		}
		model = updated
	}
	return model
}

func typeText(text string) tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(text)} }

func pressKey(key tea.KeyType) tea.Msg { return tea.KeyMsg{Type: key} }

// TestScaffoldDocumentLoadsTheBoundaryRuntime keeps a new project able to apply
// streamed await boundaries. Without the reference a page whose template
// declares an async parameter still renders, but every fallback stays put and
// nothing reports why, so this is worth asserting rather than reviewing.
func TestScaffoldDocumentLoadsTheBoundaryRuntime(t *testing.T) {
	files := scaffoldFiles(initOptions{Name: "fixture"})

	document := files["templates/document.pw.html"]
	if !strings.Contains(document, "external RuntimeScriptURL(): url") {
		t.Errorf("document does not declare the runtime helper:\n%s", document)
	}
	if !strings.Contains(document, `<script type="module" src={RuntimeScriptURL()}></script>`) {
		t.Errorf("document does not reference the runtime module:\n%s", document)
	}

	helper := files["templates/templates.go"]
	if !strings.Contains(helper, "func RuntimeScriptURL() *url.URL") {
		t.Errorf("templates.go does not implement the declared external:\n%s", helper)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "templates/templates.go", helper, parser.AllErrors); err != nil {
		t.Fatalf("scaffold is invalid Go: %v\n%s", err, helper)
	}
}

// The session backend is opt-in by blank import, so a project that scaffolds a
// login has to carry the line that registers the storage it configured.
func TestScaffoldWithLoginImportsItsSessionBackend(t *testing.T) {
	files := scaffoldFiles(initOptions{Name: "fixture", Database: true, Auth: authOIDC})
	main := files["cmd/fixture/main.go"]
	if !strings.Contains(main, `_ "github.com/shibukawa/popcornwave/sessionstore/sqlite"`) {
		t.Errorf("entry point does not register the rdb session backend:\n%s", main)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "cmd/fixture/main.go", main, parser.AllErrors); err != nil {
		t.Fatalf("entry point is invalid Go: %v\n%s", err, main)
	}
	// A project without a login configures no session storage, so it carries
	// no storage import either.
	plain := scaffoldFiles(initOptions{Name: "fixture"})["cmd/fixture/main.go"]
	if strings.Contains(plain, "sessionstore/") {
		t.Errorf("a project without a login imports a session backend:\n%s", plain)
	}
}
