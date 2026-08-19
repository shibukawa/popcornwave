package pwlsp

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// writeProject lays out a small project on disk and returns its root. The
// layout is the one the loader would produce for a popcornweb.toml declaring
// templates, queries, and a page tree.
func writeProject(t *testing.T) (string, *Project) {
	t.Helper()
	root := t.TempDir()
	write := func(name, body string) {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	write("templates/card.pw.html", "export component Card(label: string): html {\n  <p>{label}</p>\n}\n")
	write("pages/rooms/page.pw.html", "export component Page(): html {\n  <h1>rooms</h1>\n}\n")
	write("queries/rooms.pw.sql", "package q\ntype Room { id: int }\nexport statement RoomByID(id: int): sql.one<Room> {\n  SELECT id FROM rooms WHERE id = {id}\n}\n")
	// Outside every declared purpose, and therefore outside the model.
	write("scratch/draft.pw.html", "export component Draft(): html {\n  <p>x</p>\n}\n")
	// A directory the walk must not descend into, holding something that would
	// otherwise be indexed twice over.
	write(".git/objects/stale.pw.html", "export component Stale(): html {\n  <p>x</p>\n}\n")

	project := &Project{
		Root:       root,
		ConfigPath: filepath.Join(root, "popcornweb.toml"),
		Name:       "example",
		Sources: []SourceDir{
			{Purpose: "generate.templates", Dir: filepath.Join(root, "templates"), Kinds: DialectsFor("generate.templates")},
			{Purpose: "generate.pages", Dir: filepath.Join(root, "pages"), Kinds: DialectsFor("generate.pages")},
			{Purpose: "generate.queries", Dir: filepath.Join(root, "queries"), Kinds: DialectsFor("generate.queries")},
			{Purpose: "generate.handlers", Dir: filepath.Join(root, "handlers"), Kinds: DialectsFor("generate.handlers")},
		},
	}
	return root, project
}

func names(built index) map[string]string {
	found := map[string]string{}
	for _, declaration := range built.declarations {
		found[declaration.Name] = declaration.Container
	}
	return found
}

func TestTheIndexCoversTheDeclaredPurposesAndNothingElse(t *testing.T) {
	// decision:explicit-generation-sources: a directory is invisible to a
	// purpose until popcornweb.toml lists it, so a source outside every
	// purpose is absent rather than silently included.
	_, project := writeProject(t)

	found := names(buildIndex(project))

	for _, wanted := range []string{"Card", "Page", "Room", "RoomByID"} {
		if _, indexed := found[wanted]; !indexed {
			t.Errorf("%s is missing from the index", wanted)
		}
	}
	if _, indexed := found["Draft"]; indexed {
		t.Error("a source outside every purpose was indexed")
	}
	if _, indexed := found["Stale"]; indexed {
		t.Error("the walk descended into .git")
	}
}

func TestAPurposeReadsOnlyItsOwnDialect(t *testing.T) {
	// The queries purpose reads .pw.sql. A .pw.html dropped into it is not a
	// source that purpose generates from, so it is not a declaration the
	// editor can resolve against either.
	root, project := writeProject(t)
	stray := filepath.Join(root, "queries", "stray.pw.html")
	if err := os.WriteFile(stray, []byte("export component Stray(): html {\n  <p>x</p>\n}\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	if _, indexed := names(buildIndex(project))["Stray"]; indexed {
		t.Error("a .pw.html under the queries purpose was indexed")
	}
}

func TestAPurposeCoversItsWholeSubtree(t *testing.T) {
	// A page tree is one purpose entry and many directories below it, which is
	// what api:cli-generate walks.
	_, project := writeProject(t)

	if container := names(buildIndex(project))["Page"]; container != "pages/rooms/page.pw.html" {
		t.Fatalf("container = %q, want the nested path", container)
	}
}

func TestAContainerIsRelativeAndSlashSeparated(t *testing.T) {
	// The container is shown beside the name in a symbol list, so it must read
	// the same on every platform and must not leak the machine's directories.
	_, project := writeProject(t)

	for _, declaration := range buildIndex(project).declarations {
		if filepath.IsAbs(declaration.Container) {
			t.Fatalf("container %q is absolute", declaration.Container)
		}
	}
}

func TestASourceThatDoesNotParseIsSkippedRatherThanReported(t *testing.T) {
	// A project mid-edit would otherwise fill the client with findings about
	// files nobody has opened. The finding arrives when the developer opens it.
	root, project := writeProject(t)
	if err := os.WriteFile(filepath.Join(root, "templates", "broken.pw.html"), []byte("export component B(): html {\n  <p>unclosed\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	built := buildIndex(project)
	if _, indexed := names(built)["B"]; indexed {
		t.Error("a source that does not parse contributed a declaration")
	}
	if built.files < 4 {
		t.Fatalf("files = %d, want every readable source counted", built.files)
	}
}

func TestOwnershipFollowsThePurposeAndTheDialect(t *testing.T) {
	root, project := writeProject(t)

	if _, owned := project.owns(filepath.Join(root, "templates", "card.pw.html"), dialectHTML); !owned {
		t.Error("a template under the templates purpose is not owned")
	}
	if _, owned := project.owns(filepath.Join(root, "scratch", "draft.pw.html"), dialectHTML); owned {
		t.Error("a source outside every purpose is owned")
	}
	if _, owned := project.owns(filepath.Join(root, "queries", "rooms.pw.sql"), dialectHTML); owned {
		t.Error("a purpose owned a dialect it does not read")
	}
	purpose, _ := project.owns(filepath.Join(root, "queries", "rooms.pw.sql"), dialectSQL)
	if purpose != "generate.queries" {
		t.Fatalf("purpose = %q, want generate.queries", purpose)
	}
}

func TestAPurposeThatReadsNoTemplateContributesNothing(t *testing.T) {
	// generate.handlers and generate.config read Go. They are carried in the
	// model so a message can name them, and they add no dialect.
	if kinds := DialectsFor("generate.handlers"); len(kinds) != 0 {
		t.Fatalf("handlers reads %v, want no dialect", kinds)
	}
	if kinds := DialectsFor("generate.config"); len(kinds) != 0 {
		t.Fatalf("config reads %v, want no dialect", kinds)
	}
}

func TestBuildingAnIndexWithNoProjectIsEmptyRatherThanAnError(t *testing.T) {
	if built := buildIndex(nil); len(built.declarations) != 0 || built.files != 0 {
		t.Fatalf("index = %+v, want empty", built)
	}
}

func TestAMissingPurposeDirectoryDoesNotStopTheIndex(t *testing.T) {
	// A popcornweb.toml naming a directory that does not exist is a project
	// diagnostic, not a reason to answer nothing about the rest of the tree.
	root, project := writeProject(t)
	project.Sources = append(project.Sources, SourceDir{
		Purpose: "generate.templates",
		Dir:     filepath.Join(root, "absent"),
		Kinds:   DialectsFor("generate.templates"),
	})

	if _, indexed := names(buildIndex(project))["Card"]; !indexed {
		t.Fatal("a missing directory emptied the index")
	}
}

func TestErrNoProjectIsDistinguishableFromAFailedLoad(t *testing.T) {
	// The server chooses between the parse-only mode and a configuration
	// diagnostic on this distinction, so it has to survive wrapping.
	wrapped := errors.Join(errors.New("looking for the root"), ErrNoProject)

	if !errors.Is(wrapped, ErrNoProject) {
		t.Fatal("ErrNoProject does not survive wrapping")
	}
}
