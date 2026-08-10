package pwcli

import (
	"bytes"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/popcornwave/internal/pwgen"
	"github.com/shibukawa/tinybind-go/generator"
	"github.com/shibukawa/tinybind-go/templates/sqlbind"
)

// declaredSecondBuild is the value generateProject builds for a project that
// declared the fasthttp backend, with the warnings routed somewhere a test can
// read them.
func declaredSecondBuild(t *testing.T, warnings io.Writer) secondBuild {
	t.Helper()
	options, err := pwgen.Options(sqlbind.DialectSQLite)
	if err != nil {
		t.Fatal(err)
	}
	transform := pwgen.FastTransform(options.Calls.Set)
	return secondBuild{transform: &transform, warnings: warnings}
}

// The point of the whole derivation: handlers an application wrote against
// net/http come out taking the second transport's request, calling the same
// names through the sibling package, and constrained to the build that has it.
func TestTheDerivedHandlersTakeTheSecondTransportsRequest(t *testing.T) {
	derived, ok, err := planTransport(filepath.Join("..", "transportfixture"),
		declaredSecondBuild(t, io.Discard))
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("a package full of handlers derived nothing")
	}
	source := string(derived.Content)
	for _, want := range []string{
		"//go:build fasthttp",
		`pw "github.com/shibukawa/popcornwave/pwfast"`,
		"func APIHandler(ctx *fasthttp.RequestCtx)",
		// The shared helper, which is why admission closes over the call graph:
		// deriving the handler and leaving the function it hands the request to
		// would produce a package that does not compile.
		"func renderError(ctx *fasthttp.RequestCtx, err error)",
	} {
		if !strings.Contains(source, want) {
			t.Errorf("the derived source does not carry %q:\n%s", want, source)
		}
	}
	if strings.Contains(source, "http.ResponseWriter") {
		t.Errorf("a net/http writer survived the derivation:\n%s", source)
	}
	if derived.Kind != generator.ArtifactTransport {
		t.Errorf("the derived source arrived as %q", derived.Kind)
	}
}

// Generated code is output rather than input, and this is the case that decides
// it: the last run's binder captures the request in a closure, which the
// eligibility rule correctly refuses. Without the filter a project laid out this
// way could not generate at all, and the developer would be sent to fix a file
// they did not write.
func TestGeneratedSourceIsNotDerivedASecondTime(t *testing.T) {
	directory := filepath.Join("..", "pagesfixture", "pages", "users", "id_")
	second := declaredSecondBuild(t, io.Discard)

	// What the analysis says before the filter, so the filter is shown doing
	// something rather than asserted to.
	pkg, err := loadForTransform(directory)
	if err != nil || pkg == nil {
		t.Fatalf("the fixture package did not load: %v", err)
	}
	plan, err := generator.AnalyzeTransform(pkg, *second.transform)
	if err != nil {
		t.Fatal(err)
	}
	refusedGenerated := false
	for _, refusal := range plan.Refusals {
		if generated(refusal.Position.Filename) {
			refusedGenerated = true
		}
	}
	if !refusedGenerated {
		t.Fatal("no generated file was refused, so this fixture no longer proves the filter is needed")
	}

	derived, ok, err := planTransport(directory, second)
	if err != nil {
		t.Fatalf("the filtered derivation failed: %v", err)
	}
	if !ok {
		t.Fatal("the authored server action derived nothing")
	}
	source := string(derived.Content)
	if !strings.Contains(source, "func Rename(ctx *fasthttp.RequestCtx)") {
		t.Errorf("the authored server action was not derived:\n%s", source)
	}
	// The route decoder and the binder are emitted for the second transport by
	// the emitters that own them, so a copy derived from the net/http ones would
	// be a second declaration of each.
	for _, unwanted := range []string{"func DecodeRoute", "func bindrenameRequest", "func writerenameResponse"} {
		if strings.Contains(source, unwanted) {
			t.Errorf("generated code was derived a second time: %s\n%s", unwanted, source)
		}
	}
}

// A file holding a transport handler beside declarations both builds need
// cannot be excluded by a tag without taking those with it. The derived source
// is correct either way, so it is a warning on the way past rather than a
// refusal — but an unsaid one would leave an author to discover it as a compile
// error in a build they have not run yet.
func TestAFileMixingAHandlerWithSharedDeclarationsIsReported(t *testing.T) {
	var warnings bytes.Buffer
	if _, _, err := planTransport(filepath.Join("..", "pagesfixture", "pages", "users", "id_"),
		declaredSecondBuild(t, &warnings)); err != nil {
		t.Fatal(err)
	}
	reported := warnings.String()
	if !strings.Contains(reported, "action.go") {
		t.Errorf("the mixed authored file was not named:\n%s", reported)
	}
	if !strings.Contains(reported, "file of their own") {
		t.Errorf("the report does not say what to do about it:\n%s", reported)
	}
}

// The option stays free to not take: a project that declared no second build
// runs none of this, and pays neither the type check nor the output.
func TestNothingIsDerivedWithoutTheDeclaration(t *testing.T) {
	_, ok, err := planTransport(filepath.Join("..", "transportfixture"), secondBuild{})
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("a project that declared no second build had its handlers derived")
	}
}

// A refusal naming a pw entry is this framework's missing call pattern rather
// than the application's mistake, and the message has to say so: an application
// author has no edit that supplies one.
func TestARefusalNamingAFrameworkEntryIsReportedAsAFrameworkDefect(t *testing.T) {
	message := refusalError("handlers", generator.TransformRefusals{{
		Function: "Show",
		Kind:     generator.RefusalUnknownCall,
		Detail:   "passes r to pw.SomethingUnregistered, whose transport arguments are undeclared",
	}}).Error()
	if !strings.Contains(message, "defect in Popcorn Wave") {
		t.Errorf("a refusal on a pw call reads as an application mistake:\n%s", message)
	}

	application := refusalError("handlers", generator.TransformRefusals{{
		Function: "Show",
		Kind:     generator.RefusalUnknownCall,
		Detail:   "passes r to tracing.Start, whose transport arguments are undeclared",
	}}).Error()
	if strings.Contains(application, "defect in Popcorn Wave") {
		t.Errorf("a refusal on a third-party call was blamed on the framework:\n%s", application)
	}
	if !strings.Contains(application, "remedy:") {
		t.Errorf("the upstream remedy did not reach the reader:\n%s", application)
	}
}

// The one thing a declared second build still cannot generate is said out loud,
// because its absence is a 500 on the first request rather than a build error:
// the fasthttp binder registry is populated from generated init functions, and
// an unregistered type is discovered when a request arrives.
func TestTheMissingFastBindersAreReportedRatherThanLeftToRuntime(t *testing.T) {
	binding := generator.Artifact{Kind: generator.ArtifactBinding}
	derived := generator.Artifact{Kind: generator.ArtifactTransport}
	if !owesBinders([]generator.Artifact{binding, derived}) {
		t.Error("a package with derived handlers and generated binders was reported as owing nothing")
	}
	// Handlers that bind nothing need no binder, so naming them would be noise.
	if owesBinders([]generator.Artifact{derived}) {
		t.Error("a package deriving handlers that bind nothing was reported as owing binders")
	}
	if owesBinders([]generator.Artifact{binding}) {
		t.Error("a package with no second build at all was reported as owing binders")
	}

	var report strings.Builder
	owed := &bindersOwed{}
	owed.record("handlers")
	owed.record("api")
	if !owed.report(&report) {
		t.Fatal("outstanding work was recorded and nothing was reported")
	}
	said := report.String()
	for _, want := range []string{"api, handlers", "500", "tinybind-alternate-backend-support"} {
		if !strings.Contains(said, want) {
			t.Errorf("the report does not carry %q:\n%s", want, said)
		}
	}

	// A run with nothing outstanding says nothing, so a project that binds no
	// typed value never sees it.
	var quiet strings.Builder
	if (&bindersOwed{}).report(&quiet) || quiet.Len() > 0 {
		t.Errorf("a run owing nothing reported:\n%s", quiet.String())
	}
	// Nil is the value every directory of a project without the declaration
	// carries, so recording into it has to be harmless.
	var absent *bindersOwed
	absent.record("handlers")
	if absent.report(io.Discard) {
		t.Error("a project without the declaration reported outstanding work")
	}
}

// A directory with no Go package in it is the ordinary case rather than a
// failure: a page tree root holds templates and nothing else until this run
// writes its registry, and the tree fixture is exactly that shape.
func TestADirectoryWithNoGoPackageDerivesNothing(t *testing.T) {
	directory := filepath.Join("..", "pagesfixture", "fastpages")
	pkg, err := loadForTransform(directory)
	if err != nil {
		t.Fatal(err)
	}
	if pkg != nil {
		t.Fatalf("a directory of templates loaded as a package named %q", pkg.Name)
	}
	if _, ok, err := planTransport(directory, declaredSecondBuild(t, io.Discard)); err != nil || ok {
		t.Errorf("planTransport over a directory with no Go in it returned (%v, %v)", ok, err)
	}
}

// authoredOnly reads the file a declaration came from, so a plan whose
// candidates carry no position would silently keep everything. This holds the
// suffix rule itself rather than the behavior above it.
func TestTheFilterReadsTheGeneratedSuffix(t *testing.T) {
	for path, want := range map[string]bool{
		"route_pw_gen.go":                     true,
		"routes_fast_pw_gen.go":               true,
		filepath.Join("pages", "a_pw_gen.go"): true,
		"handlers.go":                         false,
		"pw_gen.go":                           false,
		"gen.go":                              false,
	} {
		if got := generated(path); got != want {
			t.Errorf("generated(%q) = %v, want %v", path, got, want)
		}
	}
}

// loadForTransform must give the analysis a package it can answer about, so a
// mode missing type information would make every refusal disappear rather than
// be reported.
func TestTheLoadCarriesWhatTheAnalysisReads(t *testing.T) {
	pkg, err := loadForTransform(filepath.Join("..", "transportfixture"))
	if err != nil || pkg == nil {
		t.Fatalf("the handler fixture did not load: %v", err)
	}
	if pkg.TypesInfo == nil || pkg.Fset == nil || len(pkg.Syntax) == 0 {
		t.Fatal("the load carries no type information, so the analysis would admit nothing and refuse nothing")
	}
}
