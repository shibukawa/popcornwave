package pwroutes

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func site(file string, line int) *Site { return &Site{File: file, Line: line, Column: 2} }

func TestTheTableSurvivesAWriteAndARead(t *testing.T) {
	root := t.TempDir()
	written := &Table{
		Entries: []Entry{
			{Pattern: "POST /rooms", Origin: OriginApplication, Site: site("handlers/rooms.go", 12), Handler: "createRoom"},
			{Pattern: "GET /{$}", Origin: OriginPage, Page: "pages/page.pw.html"},
		},
		Unresolved: []Unresolved{{Site: *site("handlers/api.go", 30), Reason: "dynamic_pattern", Message: "built at run time"}},
	}

	if err := Write(root, written); err != nil {
		t.Fatal(err)
	}
	read, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(read.Entries) != 2 || len(read.Unresolved) != 1 {
		t.Fatalf("read = %+v", read)
	}
	if read.Entries[0].Handler == "" && read.Entries[1].Handler == "" {
		t.Fatal("the handler identity did not survive")
	}
}

func TestAnAbsentTableIsNotAFailure(t *testing.T) {
	// A project that has not generated has none, and a consumer says the
	// routes were not examined rather than that they are clean.
	if _, err := Load(t.TempDir()); !errors.Is(err, ErrAbsent) {
		t.Fatalf("err = %v, want ErrAbsent", err)
	}
}

func TestATableThatDoesNotParseIsAnError(t *testing.T) {
	// Reporting a clean route section from a file nothing could read is the
	// failure the whole input mechanism exists to prevent.
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "dist"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(File)), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(root)
	if err == nil || errors.Is(err, ErrAbsent) {
		t.Fatalf("err = %v, want a read failure distinct from absence", err)
	}
}

func TestTheOrderIsStableAcrossRuns(t *testing.T) {
	// api:cli-check compares it like any other generated artifact, so identical
	// inputs have to produce identical bytes.
	root := t.TempDir()
	build := func() *Table {
		return &Table{Entries: []Entry{
			{Pattern: "POST /rooms", Origin: OriginApplication, Site: site("b.go", 2)},
			{Pattern: "GET /{$}", Origin: OriginPage, Page: "pages/page.pw.html"},
			{Pattern: "POST /rooms", Origin: OriginApplication, Site: site("a.go", 9)},
		}}
	}
	if err := Write(root, build()); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(File)))
	if err != nil {
		t.Fatal(err)
	}
	if err := Write(root, build()); err != nil {
		t.Fatal(err)
	}
	second, _ := os.ReadFile(filepath.Join(root, filepath.FromSlash(File)))

	if string(first) != string(second) {
		t.Fatal("two writes of one table produced different bytes")
	}
}

func TestDuplicatesNameEverySideOfACollision(t *testing.T) {
	// api:serve-mux panics at registration, and the reader has to choose which
	// registration to remove, so one of them is not an answer.
	table := &Table{Entries: []Entry{
		{Pattern: "POST /rooms", Origin: OriginApplication, Site: site("a.go", 1)},
		{Pattern: "POST /rooms", Origin: OriginApplication, Site: site("b.go", 2)},
		{Pattern: "GET /rooms", Origin: OriginApplication, Site: site("c.go", 3)},
	}}

	duplicates := table.Duplicates()

	if len(duplicates) != 1 || len(duplicates["POST /rooms"]) != 2 {
		t.Fatalf("duplicates = %+v", duplicates)
	}
}

func TestAFrameworkMountIsNotADuplicateOfItself(t *testing.T) {
	// Mounts are added per environment by the consumer, and one appearing
	// twice would be that consumer's bug rather than the project's.
	table := &Table{Entries: []Entry{
		{Pattern: "/healthz", Origin: OriginFramework, EnabledBy: "server.health"},
		{Pattern: "/healthz", Origin: OriginFramework, EnabledBy: "server.health"},
	}}

	if len(table.Duplicates()) != 0 {
		t.Fatalf("duplicates = %+v, want none", table.Duplicates())
	}
}

func TestAMountClashCarriesTheKeyThatEnablesIt(t *testing.T) {
	// A reader may move either side, and the key is what makes disabling the
	// mount a choice they can actually make.
	table := (&Table{Entries: []Entry{
		{Pattern: "/healthz", Origin: OriginApplication, Site: site("a.go", 1)},
	}}).WithMounts([]Mount{{Pattern: "/healthz", EnabledBy: "server.health"}})

	clashes := table.MountClashes()

	if len(clashes) != 1 {
		t.Fatalf("clashes = %+v, want one", clashes)
	}
	if clashes[0][1].EnabledBy != "server.health" {
		t.Fatalf("clash = %+v, want the configuration key", clashes[0])
	}
}

func TestAnUnsetMountPathIsNotAMount(t *testing.T) {
	// An unset path serves nothing, so it collides with nothing.
	table := (&Table{}).WithMounts([]Mount{{Pattern: "", EnabledBy: "server.health"}})

	if len(table.Entries) != 0 {
		t.Fatalf("entries = %+v, want none", table.Entries)
	}
}

func TestMountsAreAddedWithoutMutatingTheLoadedTable(t *testing.T) {
	// The loaded table is what was generated; one environment's mounts must
	// not leak into the next environment's diagnosis.
	loaded := &Table{Entries: []Entry{{Pattern: "GET /", Origin: OriginApplication}}}

	loaded.WithMounts([]Mount{{Pattern: "/healthz", EnabledBy: "server.health"}})

	if len(loaded.Entries) != 1 {
		t.Fatalf("the loaded table gained %d entries", len(loaded.Entries)-1)
	}
}
