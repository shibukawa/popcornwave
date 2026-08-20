package pwlsp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// renameProject writes a declaration named from two places: its own file, and
// handwritten Go that calls what it generated.
func renameProject(t *testing.T) (string, *Project) {
	t.Helper()
	root, project := graphProject(t)
	path := filepath.Join(root, "handlers", "rooms.go")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "package handlers\n\nfunc show() {\n\trow, _ := queries.RoomByID(ctx, db, 1)\n\t_ = row\n}\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return root, project
}

func planFor(t *testing.T, project *Project, from, to string) RenamePlan {
	t.Helper()
	plan, err := PlanRenameIn(project, from, to)
	if err != nil {
		t.Fatalf("PlanRenameIn: %v", err)
	}
	return plan
}

func TestARenameReachesTheTemplateAndTheGoThatCallsIt(t *testing.T) {
	// The crossing is the whole reason requirement:pw-language-server deferred
	// rename: a name decides a Go symbol, and a set that stopped at the
	// template boundary would leave a project that does not compile.
	_, project := renameProject(t)

	plan := planFor(t, project, "RoomByID", "RoomByIdentifier")

	if plan.GoCallSites != 1 {
		t.Fatalf("go call sites = %d, want the handwritten call", plan.GoCallSites)
	}
	var sql, handler bool
	for uri := range plan.Changes {
		if strings.HasSuffix(uri, "rooms.pw.sql") {
			sql = true
		}
		if strings.HasSuffix(uri, "handlers/rooms.go") {
			handler = true
		}
	}
	if !sql || !handler {
		t.Fatalf("changes = %v, want both sides", plan.Changes)
	}
}

func TestAGeneratedFileIsNeverEditedByARename(t *testing.T) {
	// policy:generated-artifacts owns it, and the next generation writes the
	// new name itself.
	root, project := renameProject(t)
	if err := os.WriteFile(filepath.Join(root, "queries", "rooms_pw_gen.go"),
		[]byte("package queries\n\nfunc RoomByID() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	for uri := range planFor(t, project, "RoomByID", "RoomByIdentifier").Changes {
		if strings.HasSuffix(uri, "_pw_gen.go") {
			t.Fatalf("the plan edits %s", uri)
		}
	}
}

func TestACollisionIsRefusedBeforeAnythingIsWritten(t *testing.T) {
	// Both names are knowable now, and finding out after the write means a
	// half-renamed project.
	_, project := renameProject(t)

	plan := planFor(t, project, "RoomByID", "Row")

	if len(plan.Refusals) == 0 {
		t.Fatal("renaming onto an existing declaration was allowed")
	}
	if !strings.Contains(plan.Summary(), "already declared") {
		t.Fatalf("summary = %q", plan.Summary())
	}
}

func TestANameTheParserWouldRefuseIsRefusedHere(t *testing.T) {
	// The rename would produce a file that no longer parses, which is worse
	// than the rename not happening.
	_, project := renameProject(t)

	plan := planFor(t, project, "RoomByID", "roomByID")

	if len(plan.Refusals) == 0 || !strings.Contains(plan.Summary(), "PascalCase") {
		t.Fatalf("refusals = %v", plan.Refusals)
	}
}

func TestRenamingToTheSameNameIsRefused(t *testing.T) {
	_, project := renameProject(t)

	if plan := planFor(t, project, "RoomByID", "RoomByID"); len(plan.Refusals) == 0 {
		t.Fatal("a rename to the same name was planned")
	}
}

func TestAnUnknownDeclarationIsAnErrorRatherThanAnEmptyPlan(t *testing.T) {
	// An empty plan reads as "nothing to do", and the developer misspelled a
	// name instead.
	_, project := renameProject(t)

	if _, err := PlanRenameIn(project, "Nowhere", "Somewhere"); err == nil {
		t.Fatal("a name nothing declares was planned")
	}
}

func TestEditsAreOrderedSoAnEarlierOneDoesNotMoveALaterOne(t *testing.T) {
	// They are applied in sequence against one string, so a set ordered the
	// other way would corrupt every edit after the first.
	_, project := renameProject(t)

	for _, edits := range planFor(t, project, "RoomByID", "RoomByIdentifier").Changes {
		for index := 1; index < len(edits); index++ {
			before, after := edits[index-1].Range.Start, edits[index].Range.Start
			if after.Line > before.Line || (after.Line == before.Line && after.Character > before.Character) {
				t.Fatalf("edits ascend: %+v then %+v", before, after)
			}
		}
	}
}

func TestApplyingAPlanProducesTheNewName(t *testing.T) {
	root, project := renameProject(t)
	plan := planFor(t, project, "RoomByID", "RoomByIdentifier")

	for uri, edits := range plan.Changes {
		path := PathOf(uri)
		source, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		text := string(source)
		for _, edit := range edits {
			start, end := OffsetsOf(text, edit.Range)
			text = text[:start] + edit.NewText + text[end:]
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	for _, name := range []string{"queries/rooms.pw.sql", "handlers/rooms.go"} {
		source, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(source), "RoomByIdentifier") {
			t.Fatalf("%s did not get the new name:\n%s", name, source)
		}
	}
}
