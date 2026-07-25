package dbseed

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func seedDir(t *testing.T, names ...string) string {
	t.Helper()
	directory := t.TempDir()
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(directory, name), []byte("member:\n- { id: 1 }\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return directory
}

func TestResolveAllInLexicalOrder(t *testing.T) {
	directory := seedDir(t, "020_second.yaml", "010_first.yaml", "notes.txt")

	paths, err := Resolve(directory, nil)
	if err != nil {
		t.Fatal(err)
	}
	got := make([]string, len(paths))
	for index, path := range paths {
		got[index] = filepath.Base(path)
	}
	if strings.Join(got, ",") != "010_first.yaml,020_second.yaml" {
		t.Fatalf("paths = %v, want the two datasets in lexical order", got)
	}
}

func TestResolveNamedDatasets(t *testing.T) {
	directory := seedDir(t, "users.yaml", "orders.yaml")

	// Argument order wins over lexical order, and .yaml may be omitted.
	paths, err := Resolve(directory, []string{"users", "orders.yaml"})
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 2 ||
		filepath.Base(paths[0]) != "users.yaml" ||
		filepath.Base(paths[1]) != "orders.yaml" {
		t.Fatalf("paths = %v, want users.yaml then orders.yaml", paths)
	}
}

func TestResolveRejectsBadInput(t *testing.T) {
	directory := seedDir(t, "users.yaml")

	if _, err := Resolve(directory, []string{"missing"}); err == nil {
		t.Fatal("missing dataset was accepted")
	}
	if _, err := Resolve("", nil); err == nil {
		t.Fatal("empty directory was accepted")
	}
	if _, err := Resolve(t.TempDir(), nil); err == nil {
		t.Fatal("empty seed directory was accepted")
	}
}

func TestResolveDialect(t *testing.T) {
	dialect, err := ResolveDialect("sqlite://:memory:")
	if err != nil {
		t.Fatal(err)
	}
	if dialect != "sqlite" {
		t.Fatalf("dialect = %q, want sqlite", dialect)
	}
	if _, err := ResolveDialect("oracle://localhost/orcl"); err == nil {
		t.Fatal("unsupported driver was accepted")
	}
	if _, err := ResolveDialect("no-scheme"); err == nil {
		t.Fatal("DSN without a scheme was accepted")
	}
}
