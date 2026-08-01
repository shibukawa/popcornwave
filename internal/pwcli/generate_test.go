package pwcli

import (
	"context"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/shibukawa/popcornwave/internal/pwgen"
	"github.com/shibukawa/tinybind-go/generator"
	"github.com/shibukawa/tinybind-go/templates/sqlbind"
)

// allPurposes is the scope of a directory every purpose lists, which is what
// most planDirectory tests are about.
var allPurposes = generationPurposes{handlers: true, templates: true, queries: true, config: true}

func TestPlanDirectoryGeneratesV015Artifacts(t *testing.T) {
	directory := t.TempDir()
	writeTestFile(t, filepath.Join(directory, "go.mod"), "module fixture\n\ngo 1.26.0\n")
	writeTestFile(t, filepath.Join(directory, "home.pw.html"), `package fixture

export component Home(name: string): html {
<h1>Hello, {name}</h1>
}
`)
	writeTestFile(t, filepath.Join(directory, "users.pw.sql"), `package fixture

type User {
  id: int
  name: string
}

export statement FindUser(id: int): sql.one<User> {
SELECT id, name FROM users WHERE id = {id}
}
`)

	options, err := pwgen.Options(sqlbind.DialectPostgreSQL)
	if err != nil {
		t.Fatal(err)
	}
	runner := generator.New(options)
	changes, err := planDirectory(context.Background(), runner, directory, allPurposes, nil)
	if err != nil {
		t.Fatal(err)
	}
	byName := changesByBase(changes)
	// The template runtime lives in the htmlbind module, so generation emits one
	// artifact per source and no shared per-package runtime file.
	for _, name := range []string{"home_pw_gen.go", "users_pw_gen.go"} {
		if _, ok := byName[name]; !ok {
			t.Errorf("missing generated artifact %s; got %v", name, mapKeys(byName))
		}
	}
	home := string(byName["home_pw_gen.go"].source)
	if !strings.Contains(home, "func Home(params HomeParams) htmlbind.Fragment") {
		t.Errorf("HTML artifact does not use Fragment API:\n%s", home)
	}
	sql := string(byName["users_pw_gen.go"].source)
	if !strings.Contains(sql, "func FindUser(ctx context.Context, id int) (User, error)") {
		t.Errorf("SQL artifact does not use context-only API:\n%s", sql)
	}
	if strings.Contains(sql, "FindUserContext") ||
		strings.Contains(sql, "func FindUser(ctx context.Context, db SQLQuerier") {
		t.Errorf("SQL artifact exposes the legacy executor API:\n%s", sql)
	}

	if err := applyFileChanges(changes); err != nil {
		t.Fatal(err)
	}
	changes, err = planDirectory(context.Background(), runner, directory, allPurposes, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 0 {
		t.Fatalf("generation is not deterministic: %#v", changes)
	}

	stale := filepath.Join(directory, "obsolete_pw_gen.go")
	writeTestFile(t, stale, "package fixture\n")
	changes, err = planDirectory(context.Background(), runner, directory, allPurposes, nil)
	if err != nil {
		t.Fatal(err)
	}
	byName = changesByBase(changes)
	if !byName["obsolete_pw_gen.go"].remove {
		t.Fatalf("stale generated file was not scheduled for removal: %#v", changes)
	}
	if err := applyFileChanges(changes); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatalf("stale generated file still exists: %v", err)
	}
}

// A live source is a template-level declaration the framework never sees
// directly: what reaches pw is a plan flagged live and a boundary that keeps
// delivering. This holds the generation half of that path, so a template a
// project writes today produces what api:live-delivery-protocol serves.
func TestPlanDirectoryGeneratesLiveBoundaries(t *testing.T) {
	directory := t.TempDir()
	writeTestFile(t, filepath.Join(directory, "go.mod"), "module fixture\n\ngo 1.26.0\n")
	writeTestFile(t, filepath.Join(directory, "gauge.pw.html"), `package fixture

type Point {
  label: string
  value: int
}

external live WatchMetrics(id: string): Point

export component Gauge(id: string): html {
{await point = WatchMetrics(id)}
  <p>{point.label}: {point.value}</p>
{fallback}
  <p>waiting</p>
{/await}
}
`)

	options, err := pwgen.Options(sqlbind.DialectPostgreSQL)
	if err != nil {
		t.Fatal(err)
	}
	changes, err := planDirectory(context.Background(), generator.New(options), directory, allPurposes, nil)
	if err != nil {
		t.Fatal(err)
	}
	gauge := string(changesByBase(changes)["gauge_pw_gen.go"].source)
	// The flag is what decision:automatic-async-render-selection probes to decide
	// whether the document ends by inviting a live connection.
	if !strings.Contains(strings.Join(strings.Fields(gauge), " "), "HasLiveBlock: true") {
		t.Errorf("generated plan does not carry the live flag:\n%s", gauge)
	}
	if !strings.Contains(gauge, "htmlbind.Live(") {
		t.Errorf("generated plan does not open a live boundary:\n%s", gauge)
	}
	// The leading context is mandatory for a live source, because a source with
	// no context has nothing to make it return when the subscription ends.
	if !strings.Contains(gauge, "WatchMetrics(ctx,") {
		t.Errorf("generated call does not pass the subscription context:\n%s", gauge)
	}
}

func TestMergeArtifactsProducesOneValidGoFile(t *testing.T) {
	source, err := mergeArtifacts([]generator.Artifact{
		{
			PackageName: "fixture",
			Content: []byte(`package fixture

import "fmt"

func first() { fmt.Print("first") }
`),
		},
		{
			PackageName: "fixture",
			Content: []byte(`package fixture

import "strings"

func second() { _ = strings.Builder{} }
`),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	if strings.Count(text, "package fixture") != 1 ||
		!strings.Contains(text, `"fmt"`) ||
		!strings.Contains(text, `"strings"`) ||
		!strings.Contains(text, "func first()") ||
		!strings.Contains(text, "func second()") {
		t.Fatalf("unexpected merged source:\n%s", text)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "merged.go", source, parser.AllErrors); err != nil {
		t.Fatalf("merged source is invalid: %v\n%s", err, text)
	}
}

func TestPlanDirectoryRegistersDocumentShell(t *testing.T) {
	directory := t.TempDir()
	writeTestFile(t, filepath.Join(directory, "go.mod"), "module fixture\n\ngo 1.26.0\n")
	writeTestFile(t, filepath.Join(directory, "document.pw.html"), `package fixture

export component Document(children: html?): html {
<!doctype html><html><body><slot /></body></html>
}
`)
	options, err := pwgen.Options(sqlbind.DialectPostgreSQL)
	if err != nil {
		t.Fatal(err)
	}
	changes, err := planDirectory(context.Background(), generator.New(options), directory, allPurposes, nil)
	if err != nil {
		t.Fatal(err)
	}
	document := string(changesByBase(changes)["document_pw_gen.go"].source)
	for _, fragment := range []string{
		`"github.com/shibukawa/popcornwave/pw"`,
		"func init()",
		"pw.RegisterHTMLDocument(BindDocument(DocumentParams{}))",
	} {
		if !strings.Contains(document, fragment) {
			t.Fatalf("document artifact is missing %q:\n%s", fragment, document)
		}
	}
}

func TestPlanBootstrapLinkGeneratesRuntimeRegistrationImports(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "cmd", "fixture"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "templates"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(root, "go.mod"), "module example.test/fixture\n\ngo 1.26.0\n")
	writeTestFile(t, filepath.Join(root, "public.go"), "package publicassets\n")
	writeTestFile(t, filepath.Join(root, "cmd", "fixture", "main.go"), "package main\n\nfunc main() {}\n")
	writeTestFile(t, filepath.Join(root, "templates", "document.pw.html"), `package templates

export component Document(children: html?): html {
<html><body><slot /></body></html>
}
`)
	config := projectConfig{Main: "./cmd/fixture", Generate: generationScope{Templates: []string{"templates"}}}
	changes, err := planBootstrapLink(root, config, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 1 {
		t.Fatalf("changes = %#v", changes)
	}
	source := string(changes[0].source)
	for _, expected := range []string{
		`_ "example.test/fixture"`,
		`_ "example.test/fixture/templates"`,
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("link source is missing %q:\n%s", expected, source)
		}
	}
}

func writeTestFile(t *testing.T, path, source string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
}

func changesByBase(changes []fileChange) map[string]fileChange {
	out := make(map[string]fileChange, len(changes))
	for _, change := range changes {
		out[filepath.Base(change.path)] = change
	}
	return out
}

func mapKeys(values map[string]fileChange) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	return keys
}

// Generation reads the configured directories and nothing else, so a template
// kept outside them produces no artifact and is reported instead.
func TestRunGenerateStopsAtConfiguredSources(t *testing.T) {
	root := t.TempDir()
	for _, directory := range []string{"handlers", filepath.Join("cmd", "fixture"), "samples"} {
		if err := os.MkdirAll(filepath.Join(root, directory), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeTestFile(t, filepath.Join(root, "go.mod"), "module example.test/fixture\n\ngo 1.26.0\n")
	writeTestFile(t, filepath.Join(root, "popcornwave.toml"),
		"[project]\nname = \"fixture\"\nmain = \"./cmd/fixture\"\n\n[generate]\n"+
			"handlers = [\"handlers\"]\ntemplates = [\"handlers\"]\nqueries = []\nconfig = []\n")
	writeTestFile(t, filepath.Join(root, "cmd", "fixture", "main.go"), "package main\n\nfunc main() {}\n")
	page := `package %s

export component Home(name: string): html {
<h1>Hello, {name}</h1>
}
`
	writeTestFile(t, filepath.Join(root, "handlers", "home.pw.html"), fmt.Sprintf(page, "handlers"))
	writeTestFile(t, filepath.Join(root, "samples", "home.pw.html"), fmt.Sprintf(page, "samples"))

	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(previous)

	var output strings.Builder
	if err := runGenerate(context.Background(), nil, &output); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "handlers", "home_pw_gen.go")); err != nil {
		t.Fatalf("listed source was not generated from: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "samples", "home_pw_gen.go")); !os.IsNotExist(err) {
		t.Fatalf("unlisted source was generated from: %v", err)
	}
	if !strings.Contains(output.String(), "samples/home.pw.html is outside generate.templates") {
		t.Fatalf("unlisted source was not reported:\n%s", output.String())
	}
}

// A project migrating from whole-project discovery keeps generated files in
// directories the list no longer covers, and they are named rather than left to
// register stale content at runtime.
func TestRunGenerateReportsStaleArtifactsOutsideSources(t *testing.T) {
	root := t.TempDir()
	for _, directory := range []string{"handlers", filepath.Join("cmd", "fixture")} {
		if err := os.MkdirAll(filepath.Join(root, directory), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeTestFile(t, filepath.Join(root, "go.mod"), "module example.test/fixture\n\ngo 1.26.0\n")
	writeTestFile(t, filepath.Join(root, "popcornwave.toml"),
		"[project]\nname = \"fixture\"\nmain = \"./cmd/fixture\"\n\n[generate]\n"+
			"handlers = [\"handlers\"]\ntemplates = [\"handlers\"]\nqueries = []\nconfig = []\n")
	writeTestFile(t, filepath.Join(root, "cmd", "fixture", "main.go"), "package main\n\nfunc main() {}\n")
	writeTestFile(t, filepath.Join(root, "tinybind_openapi_pw_gen.go"), "package fixture\n")
	writeTestFile(t, filepath.Join(root, "cmd", "fixture", "popcornwave_bootstrap_pw_gen.go"), "package main\n")

	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(previous)

	var output strings.Builder
	if err := runGenerate(context.Background(), nil, &output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "tinybind_openapi_pw_gen.go was generated outside every generate purpose") {
		t.Fatalf("stale artifact was not reported:\n%s", output.String())
	}
	if strings.Contains(output.String(), "popcornwave_bootstrap_pw_gen.go was generated outside") {
		t.Fatalf("the bootstrap linker is written on purpose and must not be reported:\n%s", output.String())
	}
}

// A directory listed for one purpose contributes only that purpose's artifacts,
// which is what keeps a query package from being analyzed for routes and a
// handler package from generating renderers for a template it only stores.
func TestRunGeneratePurposesSelectArtifactsPerDirectory(t *testing.T) {
	root := t.TempDir()
	for _, directory := range []string{"handlers", "queries", filepath.Join("cmd", "fixture")} {
		if err := os.MkdirAll(filepath.Join(root, directory), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeTestFile(t, filepath.Join(root, "go.mod"), "module example.test/fixture\n\ngo 1.26.0\n")
	// handlers holds a page template, but only the query purpose lists queries,
	// so the template beside the handler is generated and the one in queries is
	// not.
	writeTestFile(t, filepath.Join(root, "popcornwave.toml"),
		"[project]\nname = \"fixture\"\nmain = \"./cmd/fixture\"\n\n[generate]\n"+
			"handlers = [\"handlers\"]\ntemplates = [\"handlers\"]\nqueries = [\"queries\"]\nconfig = []\n")
	writeTestFile(t, filepath.Join(root, "cmd", "fixture", "main.go"), "package main\n\nfunc main() {}\n")
	writeTestFile(t, filepath.Join(root, "handlers", "home.pw.html"), `package handlers

export component Home(name: string): html {
<h1>Hello, {name}</h1>
}
`)
	writeTestFile(t, filepath.Join(root, "queries", "users.pw.sql"), `package queries

type User {
  id: int
  name: string
}

export statement FindUser(id: int): sql.one<User> {
SELECT id, name FROM users WHERE id = {id}
}
`)
	writeTestFile(t, filepath.Join(root, "queries", "note.pw.html"), `package queries

export component Note(): html {
<p>stored beside the queries, generated by nobody</p>
}
`)

	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(previous)

	var output strings.Builder
	if err := runGenerate(context.Background(), nil, &output); err != nil {
		t.Fatal(err)
	}
	for _, generated := range []string{
		filepath.Join("handlers", "home_pw_gen.go"),
		filepath.Join("queries", "users_pw_gen.go"),
	} {
		if _, err := os.Stat(filepath.Join(root, generated)); err != nil {
			t.Fatalf("%s was not generated: %v", generated, err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "queries", "note_pw_gen.go")); !os.IsNotExist(err) {
		t.Fatalf("a template outside generate.templates was generated: %v", err)
	}
	if !strings.Contains(output.String(), "queries/note.pw.html is outside generate.templates") {
		t.Fatalf("the unlisted template was not reported:\n%s", output.String())
	}
}

// An absolute path buries the interesting part of a generation log, so the
// prefix the operator is already standing in comes off.
func TestChangePathsShortenAgainstTheWorkingDirectory(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "handlers"), 0o755); err != nil {
		t.Fatal(err)
	}
	changes := []fileChange{
		{path: filepath.Join(root, "handlers", "home_pw_gen.go")},
		{path: filepath.Join(root, "templates", "document_pw_gen.go")},
		{path: filepath.Join(t.TempDir(), "elsewhere_pw_gen.go")},
	}

	t.Chdir(root)
	paths := changePaths(root, changes)
	for _, want := range []string{"handlers/home_pw_gen.go", "templates/document_pw_gen.go"} {
		if !slices.Contains(paths, want) {
			t.Fatalf("paths = %#v, want %q relative to the project root", paths, want)
		}
	}
	// A file the operator could not find from here keeps its absolute form.
	if !slices.ContainsFunc(paths, filepath.IsAbs) {
		t.Fatalf("paths = %#v, want the outside file left absolute", paths)
	}

	// From a subdirectory, its own files shorten to bare names and the rest
	// still shortens against the root.
	t.Chdir(filepath.Join(root, "handlers"))
	paths = changePaths(root, changes)
	if !slices.Contains(paths, "home_pw_gen.go") {
		t.Fatalf("paths = %#v, want a bare name for the working directory", paths)
	}
	if !slices.Contains(paths, "templates/document_pw_gen.go") {
		t.Fatalf("paths = %#v, want the root-relative form for a sibling", paths)
	}
}

// dynamoFixture is a package with a tagged type, a call that directs the codec,
// and one access-pattern declaration.
func writeDynamoFixture(t *testing.T, directory string) {
	t.Helper()
	writeTestFile(t, filepath.Join(directory, "go.mod"), "module fixture\n\ngo 1.26.0\n")
	writeTestFile(t, filepath.Join(directory, "reading.go"), `package fixture

import (
	"context"

	"github.com/shibukawa/tinybind-go/dynamobind"
)

type Reading struct {
	Sensor  string  `+"`dynamo:\"sensor,partitionkey\"`"+`
	At      int64   `+"`dynamo:\"at,sortkey\"`"+`
	Celsius float64 `+"`dynamo:\"celsius\"`"+`
}

func Store(ctx context.Context, reading Reading) error {
	return dynamobind.Store(ctx, "reading", reading)
}
`)
	// A declaration file carries no package line, unlike .pw.sql: the package
	// is the directory it sits in.
	writeTestFile(t, filepath.Join(directory, "readings.pw.dynamo"), `export statement ReadingsSince(sensor: string, from: int64): dynamo.many<Reading> {
  table reading
  key sensor = {sensor} and at > {from}
}
`)
}

func TestPlanDirectoryGeneratesDynamoArtifacts(t *testing.T) {
	directory := t.TempDir()
	writeDynamoFixture(t, directory)

	options, err := pwgen.Options(sqlbind.DialectPostgreSQL)
	if err != nil {
		t.Fatal(err)
	}
	purposes := generationPurposes{handlers: true, dynamo: true}
	changes, err := planDirectory(context.Background(), generator.New(options), directory, purposes, nil)
	if err != nil {
		t.Fatal(err)
	}
	var generated string
	for _, change := range changes {
		if strings.Contains(string(change.source), "ReadingsSince") {
			generated = string(change.source)
		}
	}
	if generated == "" {
		t.Fatalf("no artifact carried the declared query; planned %d files", len(changes))
	}
	// The whole point of the declaration: the call site names neither the
	// client nor the table, and no attribute string survives into it.
	if !strings.Contains(generated, "func ReadingsSince(ctx context.Context, sensor string, from int64") {
		t.Fatalf("generated signature is not context-first:\n%s", generated)
	}
	// A reserved word cannot reach an expression, because every attribute is
	// aliased whether or not it needs to be.
	if !strings.Contains(generated, `"#k0": "sensor"`) {
		t.Fatalf("attributes must be aliased unconditionally:\n%s", generated)
	}
}

func TestPlanDirectoryLeavesDynamoSourcesUnreadWithoutThePurpose(t *testing.T) {
	directory := t.TempDir()
	writeDynamoFixture(t, directory)
	// The declaration names a table and a type that exist, so anything the run
	// produces for it would be valid. It must still produce nothing: an
	// unlisted source is not parsed, which is what keeps a deliberate sample
	// beside the code from being generated from.
	options, err := pwgen.Options(sqlbind.DialectPostgreSQL)
	if err != nil {
		t.Fatal(err)
	}
	changes, err := planDirectory(context.Background(), generator.New(options), directory,
		generationPurposes{handlers: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, change := range changes {
		if strings.Contains(string(change.source), "ReadingsSince") {
			t.Fatalf("a directory outside generate.dynamo generated a query:\n%s", change.source)
		}
	}
}

func TestPlanDirectoryRegistersTheGeneratedTable(t *testing.T) {
	directory := t.TempDir()
	writeDynamoFixture(t, directory)

	options, err := pwgen.Options(sqlbind.DialectPostgreSQL)
	if err != nil {
		t.Fatal(err)
	}
	changes, err := planDirectory(context.Background(), generator.New(options), directory,
		generationPurposes{handlers: true, dynamo: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var registration string
	for _, change := range changes {
		if strings.Contains(string(change.source), "RegisterTable") {
			registration = string(change.source)
		}
	}
	if registration == "" {
		t.Fatal("a generated table definition must register itself, or pw migrate creates nothing")
	}
	// The declared name is the snake_case of the type, which is what a
	// .pw.dynamo table clause and an item call both name.
	if !strings.Contains(registration, `dynamo.RegisterTable("reading", ReadingTable)`) {
		t.Fatalf("registration does not name the declared table:\n%s", registration)
	}
}

func TestDeclaredTableNameIsSnakeCase(t *testing.T) {
	for typeName, want := range map[string]string{
		"Reading":       "reading",
		"UserSession":   "user_session",
		"OAuthState":    "o_auth_state",
		"ID":            "id",
		"HTTPRequestID": "http_request_id",
	} {
		if got := declaredTableName(typeName); got != want {
			t.Errorf("declaredTableName(%q) = %q, want %q", typeName, got, want)
		}
	}
}
