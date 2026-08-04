package pwcli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// unformatted is deliberately ugly in every way the formatter fixes: the
// declaration body sits at column zero, the signature is unspaced, and the
// record is spread over three lines.
const unformattedSQL = `package queries

type Row {
  id: int
}

export statement Find(id:int):sql.one<Row>{SELECT id FROM accounts WHERE id = {id}}
`

const unformattedHTML = `package handlers

export component Home(name:string): html {
<h1>Hello, {name}</h1>
}
`

// fmtProject writes a project whose generate purposes cover handlers and
// queries, plus a samples directory no purpose lists.
func fmtProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, directory := range []string{"handlers", "queries", "samples", filepath.Join("cmd", "fixture")} {
		if err := os.MkdirAll(filepath.Join(root, directory), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeTestFile(t, filepath.Join(root, "go.mod"), "module example.test/fixture\n\ngo 1.26.0\n")
	writeTestFile(t, filepath.Join(root, "popcornwave.toml"),
		"[project]\nname = \"fixture\"\nmain = \"./cmd/fixture\"\n\n[generate]\n"+
			"handlers = [\"handlers\"]\ntemplates = [\"handlers\"]\nqueries = [\"queries\"]\nconfig = []\n")
	writeTestFile(t, filepath.Join(root, "cmd", "fixture", "main.go"), "package main\n\nfunc main() {}\n")
	writeTestFile(t, filepath.Join(root, "handlers", "home.pw.html"), unformattedHTML)
	writeTestFile(t, filepath.Join(root, "queries", "accounts.pw.sql"), unformattedSQL)
	writeTestFile(t, filepath.Join(root, "samples", "stray.pw.sql"), unformattedSQL)
	return root
}

func runFmtIn(t *testing.T, root string, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	t.Chdir(root)
	var out, errs bytes.Buffer
	err = runFmt(context.Background(), args, &out, &errs)
	return out.String(), errs.String(), err
}

func read(t *testing.T, path string) string {
	t.Helper()
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(source)
}

func TestFmtRewritesEverySourceItsPurposesList(t *testing.T) {
	root := fmtProject(t)
	stdout, stderr, err := runFmtIn(t, root)
	if err != nil {
		t.Fatalf("fmt: %v (stderr %q)", err, stderr)
	}
	if stdout != "" {
		t.Fatalf("a successful run should print nothing, got %q", stdout)
	}

	sql := read(t, filepath.Join(root, "queries", "accounts.pw.sql"))
	if !strings.Contains(sql, "export statement Find(id: int): sql.one<Row> {") {
		t.Fatalf("the SQL signature was not canonicalized:\n%s", sql)
	}
	if !strings.Contains(sql, "\n  SELECT id\n") {
		t.Fatalf("the SQL body was not indented one level:\n%s", sql)
	}
	html := read(t, filepath.Join(root, "handlers", "home.pw.html"))
	if !strings.Contains(html, "export component Home(name: string): html {") {
		t.Fatalf("the HTML signature was not canonicalized:\n%s", html)
	}
}

// A source outside every purpose is not the project's to format, on the same
// terms generation refuses to read it.
func TestFmtLeavesSourcesOutsideEveryPurpose(t *testing.T) {
	root := fmtProject(t)
	if _, stderr, err := runFmtIn(t, root); err != nil {
		t.Fatalf("fmt: %v (stderr %q)", err, stderr)
	}
	if got := read(t, filepath.Join(root, "samples", "stray.pw.sql")); got != unformattedSQL {
		t.Fatalf("a source outside every purpose was rewritten:\n%s", got)
	}
}

// Naming a path is already a decision, so it is formatted without consulting a
// purpose.
func TestFmtFormatsANamedPathOutsideEveryPurpose(t *testing.T) {
	root := fmtProject(t)
	if _, stderr, err := runFmtIn(t, root, filepath.Join("samples", "stray.pw.sql")); err != nil {
		t.Fatalf("fmt: %v (stderr %q)", err, stderr)
	}
	if got := read(t, filepath.Join(root, "samples", "stray.pw.sql")); got == unformattedSQL {
		t.Fatal("a named path was not formatted")
	}
	if got := read(t, filepath.Join(root, "queries", "accounts.pw.sql")); got != unformattedSQL {
		t.Fatal("naming a path should format that path only")
	}
}

func TestFmtIsIdempotent(t *testing.T) {
	root := fmtProject(t)
	if _, _, err := runFmtIn(t, root); err != nil {
		t.Fatal(err)
	}
	first := read(t, filepath.Join(root, "queries", "accounts.pw.sql"))
	if _, stdout, err := runFmtIn(t, root); err != nil {
		t.Fatalf("second run: %v (%s)", err, stdout)
	}
	if second := read(t, filepath.Join(root, "queries", "accounts.pw.sql")); second != first {
		t.Fatalf("second run changed the source:\n%s\n---\n%s", first, second)
	}
}

func TestFmtCheckListsDifferingPathsAndWritesNothing(t *testing.T) {
	root := fmtProject(t)
	stdout, _, err := runFmtIn(t, root, "--check")
	if err == nil {
		t.Fatal("--check on an unformatted tree must fail")
	}
	var finding *exitError
	if !errors.As(err, &finding) {
		t.Fatalf("--check should report its own findings, got %T", err)
	}
	if finding.command != "pw fmt" {
		t.Fatalf("finding attributed to %q", finding.command)
	}

	listed := strings.Fields(stdout)
	want := []string{
		filepath.Join("handlers", "home.pw.html"),
		filepath.Join("queries", "accounts.pw.sql"),
	}
	for _, path := range want {
		if !strings.Contains(stdout, path) {
			t.Fatalf("--check did not list %s; listed %v", path, listed)
		}
	}
	if len(listed) != len(want) {
		t.Fatalf("--check listed %v, want exactly %v", listed, want)
	}
	if got := read(t, filepath.Join(root, "queries", "accounts.pw.sql")); got != unformattedSQL {
		t.Fatal("--check rewrote a source")
	}
}

func TestFmtCheckSucceedsOnAFormattedTree(t *testing.T) {
	root := fmtProject(t)
	if _, _, err := runFmtIn(t, root); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, err := runFmtIn(t, root, "--check")
	if err != nil {
		t.Fatalf("--check on a formatted tree: %v (%s)", err, stderr)
	}
	if stdout != "" {
		t.Fatalf("--check on a formatted tree printed %q", stdout)
	}
}

// A parse failure is reported with its position, leaves that file alone, and
// does not stop the sources after it.
func TestFmtReportsAParseFailureAndKeepsGoing(t *testing.T) {
	root := fmtProject(t)
	broken := "export component Broken(): html {\n<p>unclosed\n"
	writeTestFile(t, filepath.Join(root, "handlers", "broken.pw.html"), broken)

	_, stderr, err := runFmtIn(t, root)
	if err == nil {
		t.Fatal("a parse failure must exit nonzero")
	}
	var finding *exitError
	if !errors.As(err, &finding) {
		t.Fatalf("expected a reported finding, got %T", err)
	}
	if !strings.Contains(stderr, "broken.pw.html") {
		t.Fatalf("the diagnostic does not name the file: %q", stderr)
	}
	if got := read(t, filepath.Join(root, "handlers", "broken.pw.html")); got != broken {
		t.Fatal("a source that failed to parse was rewritten")
	}
	if got := read(t, filepath.Join(root, "queries", "accounts.pw.sql")); got == unformattedSQL {
		t.Fatal("a parse failure stopped the run before the remaining sources")
	}
}

// The stream mode is what an editor filters a buffer through, so it must work
// with no project anywhere above the working directory.
func TestFmtStdinNeedsNoProject(t *testing.T) {
	t.Chdir(t.TempDir())

	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stdin := os.Stdin
	os.Stdin = read
	t.Cleanup(func() { os.Stdin = stdin })
	go func() {
		_, _ = write.WriteString(unformattedSQL)
		_ = write.Close()
	}()

	var out, errs bytes.Buffer
	if err := runFmt(context.Background(), []string{"--stdin=sql"}, &out, &errs); err != nil {
		t.Fatalf("stdin mode: %v (%s)", err, errs.String())
	}
	if !strings.Contains(out.String(), "export statement Find(id: int): sql.one<Row> {") {
		t.Fatalf("stdin mode did not format:\n%s", out.String())
	}
}

func TestFmtArgumentErrors(t *testing.T) {
	for _, testcase := range []struct{ name, argument string }{
		{"unknown flag", "--nope"},
		{"bad dialect", "--stdin=perl"},
		{"bad width", "--width=zero"},
		{"zero width", "--width=0"},
	} {
		t.Run(testcase.name, func(t *testing.T) {
			if _, err := parseFmtOptions([]string{testcase.argument}); err == nil {
				t.Fatalf("%s was accepted", testcase.argument)
			}
		})
	}
	if _, err := parseFmtOptions([]string{"--stdin=sql", "--check"}); err == nil {
		t.Fatal("--stdin with --check was accepted")
	}
	if _, err := parseFmtOptions([]string{"--stdin=sql", "a.pw.sql"}); err == nil {
		t.Fatal("--stdin with a path was accepted")
	}
}

// The command is only usable if it is registered where the dispatcher and the
// help text both read from; forgetting the second is how a documented list goes
// stale.
func TestFmtIsRegisteredInTheCommandList(t *testing.T) {
	var found bool
	for _, command := range commandSummaries {
		if command.name == "fmt" {
			found = true
			if command.summary == "" {
				t.Fatal("fmt has no summary line")
			}
		}
	}
	if !found {
		t.Fatal("fmt is missing from commandSummaries, so pw help omits it")
	}

	var usage bytes.Buffer
	printUsage(&usage)
	if !strings.Contains(usage.String(), fmtUsage) {
		t.Fatal("pw help does not print the fmt usage line")
	}
}
