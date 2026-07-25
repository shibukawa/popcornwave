package pwcli

import (
	"errors"
	"go/parser"
	"go/token"
	"io"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/shibukawa/popcornwave/internal/pwgen"
	"github.com/shibukawa/tinybind-go/generator"
)

func TestParseInitArgs(t *testing.T) {
	for _, testcase := range []struct {
		name string
		args []string
		want initOptions
	}{
		{name: "name only keeps the TinyGo default", args: []string{"demo"}, want: initOptions{Name: "demo", TinyGo: true}},
		{name: "shortcut flags", args: []string{"demo", "--tailwind", "--no-tinygo"}, want: initOptions{Name: "demo", Tailwind: true}},
		{name: "explicit tinygo", args: []string{"--tinygo", "demo"}, want: initOptions{Name: "demo", TinyGo: true}},
		{name: "no name requests the wizard", args: nil, want: initOptions{TinyGo: true}},
		{name: "interactive with a seeded name", args: []string{"-i", "demo"}, want: initOptions{Name: "demo", TinyGo: true, Interactive: true}},
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
	for _, args := range [][]string{{"--unknown"}, {"one", "two"}} {
		if _, err := parseInitArgs(args); err == nil {
			t.Fatalf("parseInitArgs(%v) accepted invalid input", args)
		}
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
	files := scaffoldFiles(initOptions{Name: "fixture"})
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
	files := scaffoldFiles(initOptions{Name: "fixture", TinyGo: true})
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
}

func TestScaffoldConfigLoadsBackForBothToolchains(t *testing.T) {
	for _, tinygo := range []bool{true, false} {
		root := t.TempDir()
		options := initOptions{Name: "fixture", TinyGo: tinygo, Tailwind: true}
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
	options, err := pwgen.Options()
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
		pressKey(tea.KeyEnter), // Tailwind: keep No
		pressKey(tea.KeyEnter), // review
	)
	if !model.confirmed {
		t.Fatalf("wizard did not confirm: index = %d", model.index)
	}
	options := wizardResult(model, defaultInitOptions())
	if options != (initOptions{Name: "demo"}) {
		t.Fatalf("options = %#v", options)
	}
}

func TestInitWizardDigitShortcutSelectsTailwind(t *testing.T) {
	t.Chdir(t.TempDir())
	model := newTestWizard(defaultInitOptions())
	model = feedWizard(t, model,
		typeText("demo"), pressKey(tea.KeyEnter),
		typeText("1"),          // TinyGo: Yes
		typeText("1"),          // Tailwind: Yes
		pressKey(tea.KeyEnter), // review
	)
	if !model.confirmed {
		t.Fatalf("wizard did not confirm: index = %d", model.index)
	}
	options := wizardResult(model, defaultInitOptions())
	if options != (initOptions{Name: "demo", TinyGo: true, Tailwind: true}) {
		t.Fatalf("options = %#v", options)
	}
}

func TestInitWizardSeedsAnswersFromShortcutFlags(t *testing.T) {
	steps := initWizardSteps(initOptions{Name: "seeded", TinyGo: true, Tailwind: true})
	want := []string{"seeded", "Yes", "Yes"}
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
	)
	view := model.View()
	if !strings.Contains(view, "Review") {
		t.Fatalf("review screen missing:\n%s", view)
	}
	for _, step := range model.steps {
		if !strings.Contains(view, step.label()) {
			t.Errorf("review screen omits %q:\n%s", step.label(), view)
		}
	}
}

// TestRunInitWizardOverKeystrokes exercises the real Bubble Tea program, so the
// wired-up input handling and result extraction stay covered.
func TestRunInitWizardOverKeystrokes(t *testing.T) {
	t.Chdir(t.TempDir())
	// demo, enter, down, enter, enter, enter
	keystrokes := "demo\r\x1b[B\r\r\r"
	options, err := runInitWizard(defaultInitOptions(),
		tea.WithInput(strings.NewReader(keystrokes)),
		tea.WithOutput(io.Discard),
		tea.WithoutRenderer(),
		tea.WithoutSignalHandler(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if options != (initOptions{Name: "demo"}) {
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
	if !errors.Is(err, errInitCanceled) {
		t.Fatalf("err = %v, want %v", err, errInitCanceled)
	}
}

func newTestWizard(defaults initOptions) wizardModel {
	model := wizardModel{steps: initWizardSteps(defaults), theme: newWizardTheme()}
	model.Init() // Focus the first step the way the Bubble Tea runtime does.
	return model
}

func wizardResult(model wizardModel, defaults initOptions) initOptions {
	options := defaults
	for _, step := range model.steps {
		step.apply(&options)
	}
	return options
}

func feedWizard(t *testing.T, model wizardModel, messages ...tea.Msg) wizardModel {
	t.Helper()
	for _, message := range messages {
		next, _ := model.Update(message)
		updated, ok := next.(wizardModel)
		if !ok {
			t.Fatalf("Update returned %T", next)
		}
		model = updated
	}
	return model
}

func typeText(text string) tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(text)} }

func pressKey(key tea.KeyType) tea.Msg { return tea.KeyMsg{Type: key} }
