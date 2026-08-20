package pwcli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/shibukawa/popcornweb/internal/pwroutes"
	"github.com/shibukawa/tinybind-go/parser"
)

func TestASiteIsRecordedRelativeToTheProjectRoot(t *testing.T) {
	// The table is read on other machines and by other tools, so it must carry
	// none of this one's directories.
	root := t.TempDir()
	collected := newRouteCollector(root)

	collected.add(filepath.Join(root, "handlers"), &parser.Result{
		Routes: []parser.Route{{
			Method: "POST", Path: "/rooms",
			Site:    parser.Position{File: filepath.Join(root, "handlers", "rooms.go"), Line: 12, Column: 2},
			Handler: parser.Handler{Form: "named", Name: "createRoom"},
		}},
	})

	entries := collected.table().Entries
	if len(entries) != 1 {
		t.Fatalf("entries = %+v", entries)
	}
	if entries[0].Site.File != "handlers/rooms.go" {
		t.Fatalf("site = %q, want a root-relative slash path", entries[0].Site.File)
	}
	if entries[0].Pattern != "POST /rooms" || entries[0].Handler != "createRoom" {
		t.Fatalf("entry = %+v", entries[0])
	}
}

func TestOnePackageAnalyzedTwiceContributesOneEntry(t *testing.T) {
	// A directory listed under two purposes is analyzed under each, and the
	// same registration must not be reported as a duplicate of itself — which
	// is precisely what PW0201 would then report.
	root := t.TempDir()
	collected := newRouteCollector(root)
	result := &parser.Result{Routes: []parser.Route{{
		Method: "GET", Path: "/",
		Site: parser.Position{File: filepath.Join(root, "handlers", "home.go"), Line: 8, Column: 2},
	}}}

	collected.add(filepath.Join(root, "handlers"), result)
	collected.add(filepath.Join(root, "handlers"), result)

	if entries := collected.table().Entries; len(entries) != 1 {
		t.Fatalf("entries = %+v, want one", entries)
	}
}

func TestAnUnresolvedRegistrationIsKeptRatherThanDropped(t *testing.T) {
	// data:route-table keeps it so a consumer states a limit instead of
	// reporting a clean table it cannot back up.
	root := t.TempDir()
	collected := newRouteCollector(root)

	collected.add(root, &parser.Result{Diagnostics: []parser.Diagnostic{{
		File: filepath.Join(root, "handlers", "api.go"), Line: 30, Column: 2,
		Reason: parser.ReasonDynamicPattern, Message: "not a compile-time constant",
	}}})

	unresolved := collected.table().Unresolved
	if len(unresolved) != 1 {
		t.Fatalf("unresolved = %+v, want one", unresolved)
	}
	if unresolved[0].Site.File != "handlers/api.go" || unresolved[0].Reason != parser.ReasonDynamicPattern {
		t.Fatalf("unresolved = %+v", unresolved[0])
	}
}

func TestADiagnosticWithNoPositionIsNotRecorded(t *testing.T) {
	// An entry a reader cannot open says nothing they can act on.
	root := t.TempDir()
	collected := newRouteCollector(root)

	collected.add(root, &parser.Result{Diagnostics: []parser.Diagnostic{{Reason: "other", Message: "somewhere"}}})

	if unresolved := collected.table().Unresolved; len(unresolved) != 0 {
		t.Fatalf("unresolved = %+v, want none", unresolved)
	}
}

func TestPageRoutesAreReadThroughTheSameDiscoveryTheRegistryUses(t *testing.T) {
	// Walking the directories here would get the root pattern wrong: a tree
	// root registers as /{$}, because a bare / is a prefix pattern in the
	// standard library and would answer every unmatched path.
	root := t.TempDir()
	for _, directory := range []string{"pages", filepath.Join("pages", "rooms")} {
		if err := os.MkdirAll(filepath.Join(root, directory), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	page := "export component Page(): html {\n  <p>x</p>\n}\n"
	for _, at := range []string{filepath.Join("pages", "page.pw.html"), filepath.Join("pages", "rooms", "page.pw.html")} {
		if err := os.WriteFile(filepath.Join(root, at), []byte(page), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	collected := newRouteCollector(root)

	if err := collectPageRoutes(root, projectConfig{Generate: generationScope{Pages: []string{"pages"}}}, collected); err != nil {
		t.Fatal(err)
	}

	patterns := map[string]string{}
	for _, entry := range collected.table().Entries {
		patterns[entry.Pattern] = entry.Page
	}
	if patterns["GET /{$}"] != "pages/page.pw.html" {
		t.Fatalf("patterns = %+v, want the root as GET /{$}", patterns)
	}
	if patterns["GET /rooms"] != "pages/rooms/page.pw.html" {
		t.Fatalf("patterns = %+v, want the nested route", patterns)
	}
}

func TestADeclaredTreeThatIsNotThereDoesNotFailTheRun(t *testing.T) {
	// It is a configuration finding of its own, and failing here would take
	// the generation that produced everything else with it.
	root := t.TempDir()
	collected := newRouteCollector(root)

	if err := collectPageRoutes(root, projectConfig{Generate: generationScope{Pages: []string{"absent"}}}, collected); err != nil {
		t.Fatalf("a missing tree failed the run: %v", err)
	}
}

func TestTheWrittenTableCarriesBothHalvesOfTheURLSpace(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "pages"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "pages", "page.pw.html"),
		[]byte("export component Page(): html {\n  <p>x</p>\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	collected := newRouteCollector(root)
	collected.add(root, &parser.Result{Routes: []parser.Route{{
		Method: "POST", Path: "/rooms",
		Site: parser.Position{File: filepath.Join(root, "handlers", "rooms.go"), Line: 3},
	}}})

	if err := writeRouteTable(root, projectConfig{Generate: generationScope{Pages: []string{"pages"}}}, collected); err != nil {
		t.Fatal(err)
	}

	table, err := pwroutes.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	origins := map[pwroutes.Origin]int{}
	for _, entry := range table.Entries {
		origins[entry.Origin]++
	}
	if origins[pwroutes.OriginApplication] != 1 || origins[pwroutes.OriginPage] != 1 {
		t.Fatalf("origins = %+v, want one of each half", origins)
	}
	// The framework mounts are not here: their paths are runtime configuration
	// and differ by environment, which a generation cannot know.
	if origins[pwroutes.OriginFramework] != 0 {
		t.Fatalf("origins = %+v, want no framework mount in a written table", origins)
	}
}
