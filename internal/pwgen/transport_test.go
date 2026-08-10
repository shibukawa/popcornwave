package pwgen_test

import (
	"io/fs"
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

// The examples are the real audience, and they are analyzed when they can be.
//
// Each is its own module whose query and template packages exist only after pw
// generate, and generated files are not committed, so a fresh checkout cannot
// load them. That makes this opportunistic rather than required: an example
// that loads is analyzed and any refusal fails, and one that does not is
// reported as skipped rather than passed over in silence.
//
// todo/stdhttp is excluded on purpose. It is the plain net/http comparison this
// repository keeps beside the framework one, it calls none of the pw surface,
// and a refusal there would be correct.
func TestTheExamplesCanBeRewrittenWhereTheyCanBeLoaded(t *testing.T) {
	options, err := pwgen.Options(sqlbind.DialectSQLite)
	if err != nil {
		t.Fatal(err)
	}
	transform := generator.DefaultTransformOptions()
	transform.Calls = options.Calls.Set

	root, err := filepath.Abs(filepath.Join("..", "..", "examples"))
	if err != nil {
		t.Fatal(err)
	}
	var modules []string
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case entry.IsDir(), entry.Name() != "go.mod":
			return nil
		}
		if module := filepath.Dir(path); filepath.Base(module) != "stdhttp" {
			modules = append(modules, module)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	analyzed, skipped := 0, 0
	for _, module := range modules {
		loaded, err := packages.Load(&packages.Config{
			Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax |
				packages.NeedTypes | packages.NeedTypesInfo | packages.NeedDeps | packages.NeedImports,
			Dir: module,
		}, "./...")
		if err != nil {
			skipped++
			continue
		}
		for _, pkg := range loaded {
			if len(pkg.Errors) > 0 || pkg.TypesInfo == nil {
				skipped++
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
				// A generated file is emitted per backend rather than rewritten,
				// so a refusal in one is not a finding about this application:
				// the fasthttp build generates its own.
				if strings.HasSuffix(refusal.Position.Filename, "_pw_gen.go") {
					continue
				}
				t.Errorf("%s cannot be rewritten:\n%s", pkg.PkgPath, refusal.Error())
			}
		}
	}
	t.Logf("analyzed %d example packages holding handlers, skipped %d that could not be loaded "+
		"(run pw generate in each example to include them)", analyzed, skipped)
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

// TestEveryRegisteredCallHasSomewhereToLand checks the other half of the
// contract: a registered pattern says a call may be rewritten, and the rewrite
// only moves the qualifier, so pwfast must declare the same name.
//
// The pattern test above proves a pw entry is not refused. It cannot prove the
// rewrite compiles, and for seven entries it did not: Redirect, RedirectSeeOther,
// QueryValue, FormValue, IsBot and OpenAPIJSON had no counterpart at all, and
// WriteStatus had one under a better name, which is the same defect wearing a
// disguise. Registration turned a refusal that named the occurrence into a build
// error in generated output, which is a worse failure than the one the refusal
// contract exists to give.
func TestEveryRegisteredCallHasSomewhereToLand(t *testing.T) {
	options, err := pwgen.Options(sqlbind.DialectSQLite)
	if err != nil {
		t.Fatal(err)
	}
	landing := exportedNames(t, filepath.Join("..", "..", "pwfast"))
	if len(landing) == 0 {
		t.Skip("pwfast could not be loaded, so this proves nothing")
	}
	for _, pattern := range options.Calls.Set {
		function := pattern.Target.Function
		if function == nil || function.PackagePath != "github.com/shibukawa/popcornwave/pw" {
			continue
		}
		// Only a call with declared transport slots is one the transform
		// rewrites. The config and sub-command registrations are also patterns
		// and carry no slots, because they take no transport and the second
		// build calls them exactly as the first one does.
		if !pattern.Transport.Declared() {
			continue
		}
		if !landing[function.Name] {
			t.Errorf("pw.%s is registered as rewritable and pwfast declares no %s; "+
				"a handler calling it would be rewritten into code that does not compile",
				function.Name, function.Name)
		}
	}
}

// exportedNames loads one package and returns the set of names it exports.
func exportedNames(t *testing.T, relative string) map[string]bool {
	t.Helper()
	dir, err := filepath.Abs(relative)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := packages.Load(&packages.Config{
		Mode: packages.NeedName | packages.NeedTypes,
		Dir:  dir,
	}, ".")
	if err != nil || len(loaded) == 0 || loaded[0].Types == nil {
		return nil
	}
	names := map[string]bool{}
	scope := loaded[0].Types.Scope()
	for _, name := range scope.Names() {
		if object := scope.Lookup(name); object.Exported() {
			names[name] = true
		}
	}
	return names
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
