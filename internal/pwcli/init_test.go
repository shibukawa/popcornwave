package pwcli

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"strconv"
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
		{name: "name only keeps the TinyGo default", args: []string{"demo"}, want: initOptions{Name: "demo", TinyGo: true, Database: true, Engine: engineSQLite, Redis: true, Devbox: true, Auth: authNone, Session: sessionRDB, Skills: skillsClaude}},
		{name: "shortcut flags", args: []string{"demo", "--tailwind", "--no-tinygo"}, want: initOptions{Name: "demo", Tailwind: true, Database: true, Engine: engineSQLite, Redis: true, Devbox: true, Auth: authNone, Session: sessionRDB, Skills: skillsClaude}},
		{name: "explicit tinygo", args: []string{"--tinygo", "demo"}, want: initOptions{Name: "demo", TinyGo: true, Database: true, Engine: engineSQLite, Redis: true, Devbox: true, Auth: authNone, Session: sessionRDB, Skills: skillsClaude}},
		{name: "no name requests the wizard", args: nil, want: initOptions{TinyGo: true, Database: true, Engine: engineSQLite, Redis: true, Devbox: true, Auth: authNone, Session: sessionRDB, Skills: skillsClaude}},
		{name: "the retired interactive flag is accepted and changes nothing", args: []string{"-i", "demo"}, want: initOptions{Name: "demo", TinyGo: true, Database: true, Engine: engineSQLite, Redis: true, Devbox: true, Auth: authNone, Session: sessionRDB, Skills: skillsClaude}},
		{name: "yes takes the flags as the whole answer", args: []string{"--yes", "demo"}, want: initOptions{Name: "demo", TinyGo: true, Database: true, Engine: engineSQLite, Redis: true, Devbox: true, Yes: true, Auth: authNone, Session: sessionRDB, Skills: skillsClaude}},
		{name: "oidc with the local emulator", args: []string{"demo", "--auth=oidc", "--devidp"}, want: initOptions{Name: "demo", TinyGo: true, Database: true, Engine: engineSQLite, Redis: true, Devbox: true, Auth: authOIDC, AuthEmulator: true, Session: sessionRDB, Skills: skillsClaude}},
		{name: "passkey drops a stray emulator flag", args: []string{"demo", "--auth=passkey", "--devidp"}, want: initOptions{Name: "demo", TinyGo: true, Database: true, Engine: engineSQLite, Redis: true, Devbox: true, Auth: authPasskey, Session: sessionRDB, Skills: skillsClaude}},
		{name: "engine shortcut", args: []string{"demo", "--db=postgres"}, want: initOptions{Name: "demo", TinyGo: true, Database: true, Engine: enginePostgres, Redis: true, Devbox: true, Auth: authNone, Session: sessionRDB, Skills: skillsClaude}},
		{name: "mysql engine shortcut", args: []string{"demo", "--db=mysql"}, want: initOptions{Name: "demo", TinyGo: true, Database: true, Engine: engineMySQL, Redis: true, Devbox: true, Auth: authNone, Session: sessionRDB, Skills: skillsClaude}},
		{name: "declined database keeps the default engine unapplied", args: []string{"demo", "--no-database"}, want: initOptions{Name: "demo", TinyGo: true, Engine: engineSQLite, Redis: true, Devbox: true, Auth: authNone, Session: sessionRDB, Skills: skillsClaude}},
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
			driverImport:   "github.com/shibukawa/popcornwave/database/sqlite",
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
			if !strings.Contains(main, testcase.driverImport) {
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

// Without a terminal there is no wizard to ask the name in, and a directory
// this command is about to create is not something to guess a name for.
func TestMainInitWithoutTerminalNeedsAName(t *testing.T) {
	t.Chdir(t.TempDir())
	var output strings.Builder
	if code := Main([]string{"init"}, &output, &output); code != 1 {
		t.Fatalf("code = %d, output = %q", code, output.String())
	}
	if !strings.Contains(output.String(), "project name is required") {
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
	model := startWizard(t, presetManual, "demo", defaultInitOptions())
	// Every other answer is left at the default the list shows, which is what
	// a Manual run that only wanted to change one thing does.
	model = answerHubRow(t, model, "TinyGo support", 2) // No
	model = confirmHub(t, model)
	if !model.confirmed {
		t.Fatalf("wizard did not confirm: on %q", currentStep(model))
	}
	options := wizardResult(model, defaultInitOptions())
	options.Preset = ""
	if options != (initOptions{Name: "demo", Router: routerRegistered, Devbox: true, Database: true, Engine: engineSQLite, Redis: true, Auth: authNone, Session: sessionRDB, Skills: skillsClaude}) {
		t.Fatalf("options = %#v", options)
	}
}

func TestInitWizardDigitShortcutSelectsTailwind(t *testing.T) {
	t.Chdir(t.TempDir())
	model := startWizard(t, presetManual, "demo", defaultInitOptions())
	model = answerHubRow(t, model, "Router", 2)       // discovered pages
	model = answerHubRow(t, model, "Tailwind CSS", 1) // Yes
	model = confirmHub(t, model)
	if !model.confirmed {
		t.Fatalf("wizard did not confirm: on %q", currentStep(model))
	}
	options := wizardResult(model, defaultInitOptions())
	options.Preset = ""
	if options != (initOptions{Name: "demo", Router: routerDiscovered, TinyGo: true, Tailwind: true, Devbox: true, Database: true, Engine: engineSQLite, Redis: true, Auth: authNone, Session: sessionRDB, Skills: skillsClaude}) {
		t.Fatalf("options = %#v", options)
	}
}

func TestInitWizardSeedsAnswersFromShortcutFlags(t *testing.T) {
	steps := initWizardSteps(initOptions{
		Name: "seeded", Router: routerBoth, TinyGo: true, Tailwind: true, Devbox: true,
		Database: true, Engine: engineSQLite, Redis: true, Dynamo: true, Auth: authOIDC,
		Session: sessionRedis, SessionExplicit: true, AuthEmulator: true,
	})
	// Every step is listed, asked or not: the seeds have to reach the ones a
	// different set of answers would have reached instead. The preset row
	// leads, and the two name rows are the one question asked two ways.
	want := []string{
		"Web site with login", "seeded", "seeded",
		"Yes", "Both", "Yes", "OIDC", "DynamoDB",
		"Yes", "SQLite", "Yes", "No",
		"Redis or Valkey", "Local emulator", "Yes", "Yes",
		".claude",
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

func TestInitWizardDefaultsSessionIntentToVolatile(t *testing.T) {
	options := defaultInitOptions()
	options.Auth = authOIDC
	for _, step := range initWizardSteps(options) {
		if step.label() == "Session storage" {
			if got := step.value(); got != "Development, reset on restart" {
				t.Fatalf("default session choice = %q", got)
			}
			return
		}
	}
	t.Fatal("session storage step was not shown")
}

func TestInitWizardRejectsUnusableName(t *testing.T) {
	t.Chdir(t.TempDir())
	empty := feedWizard(t, newTestWizard(defaultInitOptions()),
		pickPreset(t, presetManual), pressKey(tea.KeyEnter))
	if currentStep(empty) != "Project name" {
		t.Fatalf("wizard advanced past an empty name: now on %q", currentStep(empty))
	}
	if !strings.Contains(empty.View(), "a project name is required") {
		t.Fatalf("missing validation message:\n%s", empty.View())
	}

	spaced := feedWizard(t, newTestWizard(defaultInitOptions()),
		pickPreset(t, presetManual), typeText("has space"), pressKey(tea.KeyEnter))
	if currentStep(spaced) != "Project name" {
		t.Fatalf("wizard accepted an invalid name: now on %q", currentStep(spaced))
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
	model := startWizard(t, presetManual, "demo", defaultInitOptions())
	view := model.View()
	if !strings.Contains(view, "Review") {
		t.Fatalf("answer list missing:\n%s", view)
	}
	for _, index := range model.activeSteps() {
		if !strings.Contains(view, model.steps[index].label()) {
			t.Errorf("answer list omits %q:\n%s", model.steps[index].label(), view)
		}
	}
	if strings.Contains(view, "OIDC provider") {
		t.Errorf("answer list carries a skipped step:\n%s", view)
	}
}

// TestInitWizardOpensTheAnswerListAfterTheName keeps the two questions a run
// walks from growing back into the ten it replaced. A preset answers
// everything after the name, and Manual leaves those answers on a list rather
// than asking them one after another.
func TestInitWizardOpensTheAnswerListAfterTheName(t *testing.T) {
	t.Chdir(t.TempDir())
	for _, preset := range []string{presetManual, presetWebsiteLogin} {
		model := startWizard(t, preset, "demo", defaultInitOptions())
		if !model.inHub || model.editing != hubNoStep {
			t.Fatalf("%s: wizard is still walking questions; now on %q", preset, currentStep(model))
		}
	}
}

// TestInitWizardEditsAnAnswerAndReturnsToTheList is the whole of the hub: a
// question opens from a row, and accepting it lands back on the list rather
// than on the question after it.
func TestInitWizardEditsAnAnswerAndReturnsToTheList(t *testing.T) {
	t.Chdir(t.TempDir())
	model := startWizard(t, presetManual, "demo", defaultInitOptions())
	opened := openHubRow(t, model, "Tailwind CSS")
	if currentStep(opened) != "Tailwind CSS" {
		t.Fatalf("row did not open its question: now on %q", currentStep(opened))
	}
	// The digit picks Yes, which is not where the cursor already sat, so the
	// options can only carry it if the edit reached them.
	answered := feedWizard(t, opened, typeText("1"))
	if answered.editing != hubNoStep {
		t.Fatalf("accepting an answer did not return to the list: now on %q", currentStep(answered))
	}
	if !wizardResult(answered, defaultInitOptions()).Tailwind {
		t.Fatal("the answer the question accepted did not reach the options")
	}
	// Leaving a question without answering it returns to the list too, since
	// on a list there is no question before it to go back to.
	escaped := feedWizard(t, openHubRow(t, answered, "Devbox environment"), pressKey(tea.KeyEsc))
	if escaped.editing != hubNoStep {
		t.Fatalf("esc did not return to the list: now on %q", currentStep(escaped))
	}
}

// TestInitWizardMarksAnswersNobodyOpened keeps the list honest about which of
// its rows are decisions. Without the mark a hub with no first-to-last order
// reads as though every value on it was considered.
func TestInitWizardMarksAnswersNobodyOpened(t *testing.T) {
	t.Chdir(t.TempDir())
	model := startWizard(t, presetManual, "demo", defaultInitOptions())
	if !strings.Contains(model.View(), "(default)") {
		t.Fatalf("no row is marked as a default:\n%s", model.View())
	}
	answered := feedWizard(t, openHubRow(t, model, "Tailwind CSS"), pressKey(tea.KeyEnter))
	view := answered.View()
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, "Tailwind CSS") && strings.Contains(line, "(default)") {
			t.Fatalf("an answered row is still marked a default:\n%s", view)
		}
	}
}

// TestRunInitWizardOverKeystrokes exercises the real Bubble Tea program, so the
// wired-up input handling and result extraction stay covered.
func TestRunInitWizardOverKeystrokes(t *testing.T) {
	t.Chdir(t.TempDir())
	// The whole run: accept the first preset, type the name, then the answer
	// list. On the list the cursor starts on the first row the walk did not ask
	// about, which is TinyGo support, so three Ups walk back over the two rows
	// above it and wrap onto the create row beneath the answers.
	//
	// The preset is accepted with Enter rather than with its digit because the
	// runtime coalesces adjacent runes: a digit typed immediately before the
	// project name arrives as one "1demo" key, which the choice step does not
	// recognise and the wizard then waits forever for the answer it missed.
	//
	// The program waits for input it never gets when the count is short either,
	// so a missing keystroke here is a hung test rather than a failing one.
	up := "\x1b[A"
	keystrokes := "\r" + "demo\r" + up + up + up + "\r"
	options, err := runInitWizard(defaultInitOptions(),
		tea.WithInput(strings.NewReader(keystrokes)),
		tea.WithOutput(io.Discard),
		tea.WithoutRenderer(),
		tea.WithoutSignalHandler(),
	)
	if err != nil {
		t.Fatal(err)
	}
	// The first preset, answered through the real program rather than through
	// the model directly.
	// The seed carries the same default answers parseInitArgs starts from,
	// which is where the agent skill answer comes from in a real run.
	want := applyPreset(initPresetCatalog[0], initOptions{Name: "demo", Skills: skillsClaude})
	want.Preset = initPresetCatalog[0].name
	if options != normalizeSession(want) {
		t.Fatalf("options = %#v, want %#v", options, normalizeSession(want))
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

// pickPreset answers the preset step by name, using the digit shortcut the
// choice step already accepts.
func pickPreset(t *testing.T, name string) tea.Msg {
	t.Helper()
	for index, preset := range initPresetCatalog {
		if preset.name == name {
			return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{rune('1' + index)}}
		}
	}
	t.Fatalf("no preset named %q", name)
	return nil
}

// startWizard walks the two questions pw init asks before the hub opens: the
// preset, and the project name.
func startWizard(t *testing.T, preset, name string, defaults initOptions) wizardModel[initOptions] {
	t.Helper()
	return feedWizard(t, newTestWizard(defaults),
		pickPreset(t, preset),
		typeText(name), pressKey(tea.KeyEnter),
	)
}

// openHubRow puts the cursor on the row with this label and opens it, which is
// how every question after the name is reached.
func openHubRow(t *testing.T, model wizardModel[initOptions], label string) wizardModel[initOptions] {
	t.Helper()
	if !model.inHub {
		t.Fatalf("wizard is not on the answer list; current step is %q", currentStep(model))
	}
	for _, index := range model.activeSteps() {
		if model.steps[index].label() == label {
			model.cursor = index
			return feedWizard(t, model, pressKey(tea.KeyEnter))
		}
	}
	t.Fatalf("no row labelled %q on the answer list; rows are %v", label, hubLabels(model))
	return model
}

// confirmHub moves the cursor to the create row and accepts it.
func confirmHub(t *testing.T, model wizardModel[initOptions]) wizardModel[initOptions] {
	t.Helper()
	if !model.inHub || model.editing != hubNoStep {
		t.Fatalf("wizard is not on the answer list; current step is %q", currentStep(model))
	}
	model.cursor = hubConfirmRow
	return feedWizard(t, model, pressKey(tea.KeyEnter))
}

// answerHubRow opens a row, picks its numbered choice, and lands back on the
// list, which is one edit on the hub.
func answerHubRow(t *testing.T, model wizardModel[initOptions], label string, choice int) wizardModel[initOptions] {
	t.Helper()
	return feedWizard(t, openHubRow(t, model, label), typeText(strconv.Itoa(choice)))
}

// hubRow reports whether the answer list carries a row with this label, which
// is how a skipped question is asserted absent.
func hubRow(model wizardModel[initOptions], label string) bool {
	for _, index := range model.activeSteps() {
		if model.steps[index].label() == label {
			return true
		}
	}
	return false
}

func hubLabels(model wizardModel[initOptions]) []string {
	labels := make([]string, 0, len(model.steps))
	for _, index := range model.activeSteps() {
		labels = append(labels, model.steps[index].label())
	}
	return labels
}

// currentStep names the question on screen, whether the wizard walked to it or
// it was opened from the answer list. It is empty on the list itself.
func currentStep(model wizardModel[initOptions]) string {
	if model.inHub {
		if model.editing == hubNoStep {
			return ""
		}
		return model.steps[model.editing].label()
	}
	if model.index >= len(model.steps) {
		return ""
	}
	return model.steps[model.index].label()
}

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
	// The canonical spelling: scaffoldFiles formats every template source the
	// way pw fmt would, which quotes the interpolated attribute.
	if !strings.Contains(document, `<script type="module" src="{RuntimeScriptURL()}"></script>`) {
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

// The starter record's item calls and the table pw generate registers have to
// name one table. They are derived separately — the calls are written here by
// hand, the registration from declaredTableName — so a plausible-looking plural
// in the scaffold would produce a table nothing reads and reads of a table
// nothing creates.
func TestDynamoScaffoldNamesTheRegisteredTable(t *testing.T) {
	source := dynamoRecordScaffold()
	file, err := parser.ParseFile(token.NewFileSet(), "records/note.go", source, parser.AllErrors)
	if err != nil {
		t.Fatalf("dynamo scaffold is invalid Go: %v\n%s", err, source)
	}

	// The type declaring a partition key is the one that owns a table, which is
	// the same marker the generated registration is derived from.
	var want string
	ast.Inspect(file, func(node ast.Node) bool {
		spec, ok := node.(*ast.TypeSpec)
		if !ok {
			return true
		}
		structure, ok := spec.Type.(*ast.StructType)
		if !ok {
			return true
		}
		for _, field := range structure.Fields.List {
			if field.Tag != nil && strings.Contains(field.Tag.Value, "partitionkey") {
				want = declaredTableName(spec.Name.Name)
				return false
			}
		}
		return true
	})
	if want == "" {
		t.Fatal("the dynamo scaffold declares no partition key, so it registers no table")
	}

	named := 0
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || len(call.Args) < 2 {
			return true
		}
		if !strings.Contains(source[call.Fun.Pos()-1:call.Fun.End()-1], "dynamobind.") {
			return true
		}
		// The table is the first string literal: Args[1] in the context form
		// and Args[2] in the handle-taking On form the scaffold now uses.
		var literal *ast.BasicLit
		for _, arg := range call.Args {
			if candidate, ok := arg.(*ast.BasicLit); ok && candidate.Kind == token.STRING {
				literal = candidate
				break
			}
		}
		if literal == nil {
			return true
		}
		named++
		if got, _ := strconv.Unquote(literal.Value); got != want {
			t.Errorf("scaffolded call names table %q, but pw generate registers %q", got, want)
		}
		return true
	})
	if named == 0 {
		t.Fatal("the dynamo scaffold makes no item call, so nothing directs the codec")
	}
}
