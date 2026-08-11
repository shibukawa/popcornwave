package pwcli

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/popcornwave/internal/pwgen"
	"github.com/shibukawa/tinybind-go/generator"
	"github.com/shibukawa/tinybind-go/templates/sqlbind"
)

// deriveInto runs one directory through the generation a project declaring the
// fasthttp backend gets, and returns the artifacts by kind.
func deriveInto(t *testing.T, directory string) (map[generator.ArtifactKind][]generator.Artifact, error) {
	t.Helper()
	options, err := pwgen.Options(sqlbind.DialectSQLite)
	if err != nil {
		t.Fatal(err)
	}
	transform := pwgen.FastTransform(options.Calls.Set)
	options.Transform = &transform

	artifacts, err := generator.New(options).GenerateArtifacts(context.Background(),
		generator.GenerateRequest{
			Dir: directory, SQLContextOnlyAPI: true,
			// These are about the derivation, and a page tree's templates are
			// compiled by the tree run rather than the flat one — exactly as
			// planDirectory disables them for a directory with no templates
			// purpose.
			HTMLTemplatePattern: disabledTemplatePattern,
		})
	if err != nil {
		return nil, err
	}
	byKind := map[generator.ArtifactKind][]generator.Artifact{}
	for _, artifact := range artifacts {
		byKind[artifact.Kind] = append(byKind[artifact.Kind], artifact)
	}
	return byKind, nil
}

func onlyArtifact(t *testing.T, byKind map[generator.ArtifactKind][]generator.Artifact, kind generator.ArtifactKind) generator.Artifact {
	t.Helper()
	found := byKind[kind]
	if len(found) != 1 {
		t.Fatalf("expected one %s artifact, got %d", kind, len(found))
	}
	return found[0]
}

// The point of the whole derivation: handlers an application wrote against
// net/http come out taking the second transport's request, calling the same
// names through the sibling package, and constrained to the build that has it.
func TestTheDerivedHandlersTakeTheSecondTransportsRequest(t *testing.T) {
	byKind, err := deriveInto(t, filepath.Join("..", "transportfixture"))
	if err != nil {
		t.Fatal(err)
	}
	source := string(onlyArtifact(t, byKind, generator.ArtifactTransport).Content)
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
}

// The derived handlers bind and write through a registry of their own, so the
// binders follow them or the second build compiles and answers 500 on the first
// request: pwfast.Parse dispatches through generated init functions, and an
// unregistered type is found when a request arrives rather than at link time.
func TestTheSecondBuildGetsItsOwnBindersAndWriters(t *testing.T) {
	byKind, err := deriveInto(t, filepath.Join("..", "transportfixture"))
	if err != nil {
		t.Fatal(err)
	}
	source := string(onlyArtifact(t, byKind, generator.ArtifactTransportBinding).Content)
	for _, want := range []string{
		"//go:build fasthttp",
		`httpbind "github.com/shibukawa/tinybind-go/fasthttpbind"`,
		"func writeGreeting(r *fasthttp.RequestCtx, v Greeting) error",
		"httpbind.RegisterWrite[Greeting](writeGreeting)",
	} {
		if !strings.Contains(source, want) {
			t.Errorf("the derived binders do not carry %q:\n%s", want, source)
		}
	}
	// The net/http half is still generated beside it, unconstrained here and
	// constrained by planDirectory from its imports.
	if len(byKind[generator.ArtifactBinding]) == 0 {
		t.Error("the first transport's binders were dropped when the second build was asked for")
	}
}

// Generated code is output rather than input. The case that decides it is the
// last run's binder, which captures the request in a closure to read the body
// lazily and would be refused as if a developer had written it — on the second
// run of a package, never the first.
//
// system:tinybind v0.5.5 skips generated files during the derivation, reading
// the header prefix this project registers. This holds that: a package whose
// generated binder is committed derives its authored handler and nothing else.
func TestGeneratedSourceIsNotDerivedASecondTime(t *testing.T) {
	byKind, err := deriveInto(t, filepath.Join("..", "pagesfixture", "pages", "users", "id_"))
	if err != nil {
		t.Fatalf("a package holding its previous output failed to derive: %v", err)
	}
	source := string(onlyArtifact(t, byKind, generator.ArtifactTransport).Content)
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

// The generator offers a route registration for the second transport too, on
// the router its transform target names. A page tree here installs on
// pwfastpage.Router and brings its own registry, so taking both would mean two
// registries and a dependency on a router no application built on this
// framework imports.
func TestTheGeneratorsOwnRouteRegistrationIsDeclined(t *testing.T) {
	if (generationPurposes{handlers: true, pages: true, templates: true}).keeps(generator.ArtifactTransportRoutes) {
		t.Error("the second transport's route registration was kept")
	}
	for _, kind := range []generator.ArtifactKind{
		generator.ArtifactTransport, generator.ArtifactTransportBinding,
	} {
		if !(generationPurposes{handlers: true}).keeps(kind) {
			t.Errorf("%s was dropped from a handlers directory", kind)
		}
		if (generationPurposes{templates: true}).keeps(kind) {
			t.Errorf("%s was kept for a directory that holds no handlers", kind)
		}
	}
}

// The option stays free to not take: a project that declared no second build
// gets neither half, and its output is what it was before the feature existed.
func TestNothingIsDerivedWithoutTheDeclaration(t *testing.T) {
	options, err := pwgen.Options(sqlbind.DialectSQLite)
	if err != nil {
		t.Fatal(err)
	}
	artifacts, err := generator.New(options).GenerateArtifacts(context.Background(),
		generator.GenerateRequest{Dir: filepath.Join("..", "transportfixture"), SQLContextOnlyAPI: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, artifact := range artifacts {
		switch artifact.Kind {
		case generator.ArtifactTransport, generator.ArtifactTransportBinding, generator.ArtifactTransportRoutes:
			t.Errorf("a project that declared no second build produced %s", artifact.Kind)
		}
	}
}

// A refusal naming a pw entry is this framework's missing call pattern rather
// than the application's mistake, and the message has to say so: an application
// author has no edit that supplies one.
func TestARefusalNamingAFrameworkEntryIsReportedAsAFrameworkDefect(t *testing.T) {
	ordinary := transportRefusal("handlers", errors.New("generate templates: no dialect"))
	if !strings.Contains(ordinary.Error(), "no dialect") {
		t.Errorf("an ordinary generation failure lost its reason:\n%s", ordinary)
	}
	if strings.Contains(ordinary.Error(), "defect in Popcorn Wave") {
		t.Errorf("an ordinary generation failure was blamed on the framework:\n%s", ordinary)
	}

	message := transportRefusal("handlers", fmt.Errorf("generate transport: %w",
		generator.TransformRefusals{{
			Function: "Show",
			Kind:     generator.RefusalUnknownCall,
			Detail:   "passes r to pw.SomethingUnregistered, whose transport arguments are undeclared",
		}})).Error()
	if !strings.Contains(message, "defect in Popcorn Wave") {
		t.Errorf("a refusal on a pw call reads as an application mistake:\n%s", message)
	}

	application := transportRefusal("handlers", fmt.Errorf("generate transport: %w",
		generator.TransformRefusals{{
			Function: "Show",
			Kind:     generator.RefusalUnknownCall,
			Detail:   "passes r to tracing.Start, whose transport arguments are undeclared",
		}})).Error()
	if strings.Contains(application, "defect in Popcorn Wave") {
		t.Errorf("a refusal on a third-party call was blamed on the framework:\n%s", application)
	}
	if !strings.Contains(application, "remedy:") {
		t.Errorf("the upstream remedy did not reach the reader:\n%s", application)
	}
}
