package pwcli

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/shibukawa/popcornweb/internal/pwlsp"
)

// dirsFor returns the purpose entries the model carries for one key.
func dirsFor(project *pwlsp.Project, purpose string) []string {
	var found []string
	for _, source := range project.Sources {
		if source.Purpose == purpose {
			found = append(found, source.Dir)
		}
	}
	return found
}

func TestTheModelCarriesEveryDeclaredPurpose(t *testing.T) {
	// The server answers the scope api:cli-generate reads, so the model is
	// built from the same generatePurposes table generation walks rather than
	// from a second list that could fall behind it.
	root := t.TempDir()
	for _, directory := range []string{"handlers", "templates", "queries", "cmd/fixture", "pages"} {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(directory)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeTestFile(t, filepath.Join(root, "popcornweb.toml"), `[project]
name = "fixture"
main = "./cmd/fixture"

[generate]
handlers = ["handlers"]
templates = ["handlers", "templates"]
queries = ["queries"]
config = ["cmd/fixture"]
pages = ["pages"]
`)

	project, err := loadLSPProject(root)
	if err != nil {
		t.Fatalf("loadLSPProject: %v", err)
	}
	if project.Name != "fixture" || project.Root != root {
		t.Fatalf("project = %+v", project)
	}
	if got := dirsFor(project, "generate.templates"); len(got) != 2 {
		t.Fatalf("templates = %v, want both declared directories", got)
	}
	if got := dirsFor(project, "generate.queries"); len(got) != 1 || got[0] != filepath.Join(root, "queries") {
		t.Fatalf("queries = %v, want an absolute path under the root", got)
	}
	if got := dirsFor(project, "generate.pages"); len(got) != 1 {
		t.Fatalf("pages = %v, want the page tree root", got)
	}
}

func TestEachPurposeCarriesTheDialectsItReads(t *testing.T) {
	root := t.TempDir()
	for _, directory := range []string{"handlers", "queries"} {
		if err := os.MkdirAll(filepath.Join(root, directory), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeTestFile(t, filepath.Join(root, "popcornweb.toml"), `[project]
name = "fixture"
main = "./cmd/fixture"

[generate]
handlers = ["handlers"]
templates = ["handlers"]
queries = ["queries"]
config = []
`)

	project, err := loadLSPProject(root)
	if err != nil {
		t.Fatalf("loadLSPProject: %v", err)
	}
	for _, source := range project.Sources {
		switch source.Purpose {
		case "generate.handlers", "generate.config":
			if len(source.Kinds) != 0 {
				t.Errorf("%s reads %v, want no template dialect", source.Purpose, source.Kinds)
			}
		case "generate.templates", "generate.queries":
			if len(source.Kinds) == 0 {
				t.Errorf("%s reads no dialect", source.Purpose)
			}
		}
	}
}

func TestTheModelIsFoundFromADirectoryBelowTheRoot(t *testing.T) {
	// An editor opens a file, not a project. The root is whatever
	// popcornweb.toml stands above it.
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "handlers", "deep"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeProjectFixture(t, root, "[project]\nname = \"fixture\"\nmain = \"./cmd/fixture\"\n")

	project, err := loadLSPProject(filepath.Join(root, "handlers", "deep"))
	if err != nil {
		t.Fatalf("loadLSPProject: %v", err)
	}
	if project.Root != root {
		t.Fatalf("root = %q, want %q", project.Root, root)
	}
}

func TestNoConfigurationAnywhereIsErrNoProject(t *testing.T) {
	// The server distinguishes this from a failed load: one selects the
	// parse-only mode, the other is a diagnostic on a file that exists.
	if _, err := loadLSPProject(t.TempDir()); !errors.Is(err, pwlsp.ErrNoProject) {
		t.Fatalf("err = %v, want ErrNoProject", err)
	}
}

func TestAConfigurationThatWillNotLoadIsReportedAsItself(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "popcornweb.toml"), "[project]\nname = \"fixture\"\n")

	_, err := loadLSPProject(root)
	if err == nil {
		t.Fatal("an unusable configuration loaded")
	}
	if errors.Is(err, pwlsp.ErrNoProject) {
		t.Fatal("a configuration that exists was reported as absent")
	}
}
