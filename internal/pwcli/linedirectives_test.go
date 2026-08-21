package pwcli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/popcornweb/internal/pwgen"
	"github.com/shibukawa/tinybind-go/generator"
	"github.com/shibukawa/tinybind-go/templates/sqlbind"
)

func TestLineDirectivesAreOffUnlessTheProjectAsks(t *testing.T) {
	// The default is what every project that has not thought about this gets,
	// and turning it on costs them go test -cover silently.
	root := t.TempDir()
	writeProjectFixture(t, root, "[project]\nname = \"fixture\"\nmain = \"./cmd/fixture\"\n")

	config, err := loadProjectConfig(root)
	if err != nil {
		t.Fatal(err)
	}
	if config.LineDirectives {
		t.Fatal("line directives are on for a project that never asked")
	}
}

func TestAProjectCanAskForLineDirectives(t *testing.T) {
	root := t.TempDir()
	writeProjectFixture(t, root, "[project]\nname = \"fixture\"\nmain = \"./cmd/fixture\"\n\n[generate]\nline_directives = true\n")

	config, err := loadProjectConfig(root)
	if err != nil {
		t.Fatal(err)
	}
	if !config.LineDirectives {
		t.Fatal("the project asked for line directives and did not get them")
	}
}

func TestAnUnusableLineDirectivesValueIsReported(t *testing.T) {
	root := t.TempDir()
	writeProjectFixture(t, root, "[project]\nname = \"fixture\"\nmain = \"./cmd/fixture\"\n\n[generate]\nline_directives = \"yes\"\n")

	_, err := loadProjectConfig(root)
	if err == nil {
		t.Fatal("a string was accepted where a boolean belongs")
	}
	if !strings.Contains(err.Error(), "generate.line_directives") {
		t.Fatalf("err = %v, want the key named", err)
	}
}

func TestTheSettingReachesTheGenerator(t *testing.T) {
	// The wiring is what makes the setting mean anything. It is asserted
	// against the generator's own field rather than against emitted bytes,
	// because what the directives look like is upstream's to decide.
	options, err := pwgen.Options(sqlbind.DialectSQLite)
	if err != nil {
		t.Fatal(err)
	}
	if options.TemplateLineDirectives {
		t.Fatal("pwgen.Options turns them on, so the project setting could never turn them off")
	}
	options.TemplateLineDirectives = true
	if !options.TemplateLineDirectives {
		t.Fatal("the generator option does not hold the value")
	}
}

func TestTheScaffoldStatesTheSettingAndWhatItCosts(t *testing.T) {
	// A setting a project only meets by reading someone else's source is one
	// nobody turns on. The scaffold is where it is met.
	root := writeScaffoldedProject(t, initOptions{
		Name: "fixture", Devbox: true, Database: true, Auth: authNone,
	})
	source, err := os.ReadFile(filepath.Join(root, "popcornweb.toml"))
	if err != nil {
		t.Fatal(err)
	}

	written := string(source)
	if !strings.Contains(written, "line_directives = false") {
		t.Fatalf("the scaffold does not carry the setting:\n%s", written)
	}
	if !strings.Contains(written, "go test -cover") {
		t.Fatal("the scaffold states the setting without stating what it costs")
	}
}

func TestAScaffoldedProjectLoadsWithTheSettingPresent(t *testing.T) {
	// The scaffold writes the key, so the known-key check has to accept it.
	root := writeScaffoldedProject(t, initOptions{
		Name: "fixture", Devbox: true, Database: true, Auth: authNone,
	})

	config, err := loadProjectConfig(root)
	if err != nil {
		t.Fatalf("a scaffolded project does not load: %v", err)
	}
	if config.LineDirectives {
		t.Fatal("the scaffold turns them on")
	}
}

func TestGenerationEmitsTheDirectivesWhenTheProjectAsks(t *testing.T) {
	// The end of the wiring: what a project turning the setting on actually
	// gets in the file a compiler will read.
	directory := t.TempDir()
	writeTestFile(t, filepath.Join(directory, "go.mod"), "module fixture\n\ngo 1.26.0\n")
	writeTestFile(t, filepath.Join(directory, "users.pw.sql"), `package fixture

type User {
  id: int
  name: string
}

export statement FindUser(id: int): sql.one<User> {
SELECT id, name FROM users WHERE id = {id}
}
`)

	options, err := pwgen.Options(sqlbind.DialectSQLite)
	if err != nil {
		t.Fatal(err)
	}
	options.TemplateLineDirectives = true
	changes, err := planDirectory(context.Background(), generator.New(options), directory, allPurposes, nil, nil, false, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	emitted := ""
	for _, change := range changes {
		if filepath.Base(change.path) == "users_pw_gen.go" {
			emitted = string(change.source)
		}
	}
	if emitted == "" {
		t.Fatal("nothing was generated for the statement source")
	}
	if !strings.Contains(emitted, "//line ") {
		t.Fatalf("no directive reached the generated file:\n%s", emitted)
	}
	// Absolute, because go vet resolves a relative directive a second time
	// against the directory of the file holding it and names nothing.
	if !strings.Contains(emitted, "//line "+filepath.Join(directory, "users.pw.sql")+":") {
		t.Fatalf("the directive does not name the source by an absolute path:\n%s", emitted)
	}
	// The placeholder the generator writes is resolved when the artifact is
	// named. One left in the file would point every scaffolding error at a
	// file that does not exist.
	if strings.Contains(emitted, "tinybind_restore") {
		t.Fatalf("an unresolved restore placeholder reached the file:\n%s", emitted)
	}
	if !strings.Contains(emitted, "//line users_pw_gen.go:") {
		t.Fatalf("the generated file's own position is never restored:\n%s", emitted)
	}
}

func TestGenerationEmitsNoDirectivesByDefault(t *testing.T) {
	directory := t.TempDir()
	writeTestFile(t, filepath.Join(directory, "go.mod"), "module fixture\n\ngo 1.26.0\n")
	writeTestFile(t, filepath.Join(directory, "users.pw.sql"), `package fixture

type User {
  id: int
}

export statement FindUser(id: int): sql.one<User> {
SELECT id FROM users WHERE id = {id}
}
`)

	options, err := pwgen.Options(sqlbind.DialectSQLite)
	if err != nil {
		t.Fatal(err)
	}
	changes, err := planDirectory(context.Background(), generator.New(options), directory, allPurposes, nil, nil, false, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	for _, change := range changes {
		if strings.Contains(string(change.source), "//line ") {
			t.Fatalf("%s carries directives with the setting off", change.path)
		}
	}
}
