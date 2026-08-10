package pwgen_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/popcornwave/internal/pwgen"
	"github.com/shibukawa/tinybind-go/generator"
	"github.com/shibukawa/tinybind-go/templates/sqlbind"
	"golang.org/x/tools/go/packages"
)

// The acceptance criterion of the call-registration requirement: code written
// against this framework the ordinary way can be rewritten for the second
// transport.
//
// It runs the module's own analysis over authored handler code written the way
// an application writes it, with the registry pw ships. A refusal here is not the fixture's mistake — it
// is a pw call whose transport arguments this framework never declared, and an
// application author has no way to fix that. Which is why it fails here rather
// than reaching a user.
//
// The target is internal/transportfixture rather than a generated package: a
// generated file is emitted per backend rather than rewritten, so analyzing one
// asks the wrong question. The examples would be the better target and are not
// usable as one — each is its own module whose query and template packages exist
// only after pw generate, and nothing in CI builds them.
func TestAuthoredHandlersCanBeRewrittenForTheSecondTransport(t *testing.T) {
	options, err := pwgen.Options(sqlbind.DialectSQLite)
	if err != nil {
		t.Fatal(err)
	}
	transform := generator.DefaultTransformOptions()
	transform.Calls = options.Calls.Set

	dir, err := filepath.Abs(filepath.Join("..", "transportfixture"))
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := packages.Load(&packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax |
			packages.NeedTypes | packages.NeedTypesInfo | packages.NeedDeps | packages.NeedImports,
		Dir: dir,
	}, "./...")
	if err != nil {
		t.Fatalf("the handler fixture could not be loaded: %v", err)
	}

	analyzed := 0
	for _, pkg := range loaded {
		if len(pkg.Errors) > 0 || pkg.TypesInfo == nil {
			for _, e := range pkg.Errors {
				t.Errorf("%s does not type-check, so the analysis says nothing: %v", pkg.PkgPath, e)
			}
			continue
		}
		plan, err := generator.AnalyzeTransform(pkg, transform)
		if err != nil {
			t.Errorf("%s: %v", pkg.PkgPath, err)
			continue
		}
		if len(plan.Admitted) == 0 && len(plan.Refusals) == 0 {
			continue
		}
		analyzed++
		for _, refusal := range plan.Refusals {
			t.Errorf("%s cannot be rewritten:\n%s", pkg.PkgPath, refusal.Error())
		}
	}
	if analyzed == 0 {
		t.Fatal("no package held a transport-taking function, so this proves nothing")
	}
	t.Logf("analyzed %d packages holding handlers", analyzed)
}

// A pw entry taking the transport with no registered pattern is the omission
// this framework's users cannot fix, so it is caught where it is made.
func TestEveryTransportTakingPwEntryHasAPattern(t *testing.T) {
	options, err := pwgen.Options(sqlbind.DialectSQLite)
	if err != nil {
		t.Fatal(err)
	}
	registered := map[string]bool{}
	for _, pattern := range options.Calls.Set {
		if function := pattern.Target.Function; function != nil &&
			function.PackagePath == "github.com/shibukawa/popcornwave/pw" {
			registered[function.Name] = true
		}
	}
	// Read from the source of truth rather than restated here, so an entry
	// added to pw without a pattern fails this rather than being remembered.
	for _, name := range pwEntriesTakingTheTransport(t) {
		if !registered[name] {
			t.Errorf("pw.%s takes the transport and has no registered call pattern; "+
				"every handler calling it would be refused with a remedy only this framework can supply", name)
		}
	}
}

// pwEntriesTakingTheTransport reads pw's exported functions and returns those
// whose signature names a writer or a request.
func pwEntriesTakingTheTransport(t *testing.T) []string {
	t.Helper()
	dir, err := filepath.Abs(filepath.Join("..", "..", "pw"))
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := packages.Load(&packages.Config{
		Mode: packages.NeedName | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo,
		Dir:  dir,
	}, ".")
	if err != nil || len(loaded) == 0 {
		t.Skipf("pw could not be loaded, so this proves nothing: %v", err)
	}
	pkg := loaded[0]
	if pkg.Types == nil {
		t.Skip("pw carries no type information here")
	}
	var names []string
	scope := pkg.Types.Scope()
	for _, name := range scope.Names() {
		object := scope.Lookup(name)
		if !object.Exported() {
			continue
		}
		signature := strings.ReplaceAll(object.Type().String(), " ", "")
		if !strings.Contains(signature, "net/http.ResponseWriter") &&
			!strings.Contains(signature, "*net/http.Request") {
			continue
		}
		if !strings.HasPrefix(object.Type().String(), "func(") {
			continue
		}
		names = append(names, name)
	}
	return names
}
