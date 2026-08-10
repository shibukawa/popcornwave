package pwcli

import (
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"

	"github.com/shibukawa/popcornwave/internal/pwgen"
	"github.com/shibukawa/tinybind-go/generator"
	"golang.org/x/tools/go/packages"
)

// secondBuild is what a project declaring the fasthttp backend adds to every
// directory a run generates for.
//
// It is one value rather than a bool and a transform because the two are the
// same fact: a project that declared the second build has a transform, and one
// that did not has neither. declared() is what every branch reads.
type secondBuild struct {
	transform *generator.TransformOptions
	// warnings receives what the derivation could do nothing about, which today
	// is an authored file the build tag cannot cleanly exclude. It is a warning
	// rather than an error because the file is the application's to lay out and
	// the derived source is correct either way; what breaks is the other half.
	warnings io.Writer
	// owed collects the one thing a declared second build still cannot generate,
	// so the run says it once at the end rather than per directory. It is a
	// pointer because the value is copied down into every directory and the
	// answer belongs to the run.
	owed *bindersOwed
}

func (s secondBuild) declared() bool { return s.transform != nil }

// bindersOwed names the packages whose second build has derived handlers and no
// binders to serve them.
//
// The emitter exists and works: system:tinybind produces the fasthttp binders
// from the same plan as the net/http ones, and reaching it needs no new API.
// What blocks it is an ordering. Its entry point derives the handlers before
// emitting the binders, and the derivation reads every file of the loaded
// package including the ones the last run generated — so the second generation
// of a package whose request type reads the body refuses on its own previous
// output, and the binder phase is never reached. The first run of that package
// emits them and every run after it does not, which is why this is reported
// rather than taken: a generated file that appears once and is swept the next
// time is worse than one that was never written.
//
// The fix is upstream and is the same rule this package applies locally:
// generated code is output rather than input, so the derivation should not read
// it. See requirement:tinybind-alternate-backend-support.
//
// What it costs meanwhile is not a build error but a runtime one. pwfast.Parse
// dispatches through a registry of generated binders, and an unregistered type
// is reported when the first request arrives rather than when the binary is
// linked. Saying so at generation time is the only place a developer can hear
// it before then.
type bindersOwed struct{ directories []string }

func (o *bindersOwed) record(directory string) {
	if o == nil {
		return
	}
	o.directories = append(o.directories, directory)
}

// report writes the outstanding work once, and reports whether it wrote
// anything.
func (o *bindersOwed) report(out io.Writer) bool {
	if o == nil || len(o.directories) == 0 {
		return false
	}
	sort.Strings(o.directories)
	fmt.Fprintf(out, "pw: the fasthttp build has derived handlers and no generated binders in %s.\n",
		strings.Join(o.directories, ", "))
	fmt.Fprintln(out, "pw: those handlers bind or write a typed value, and the fasthttp binder registry")
	fmt.Fprintln(out, "pw: is populated by a generator phase this framework cannot reach today, so the")
	fmt.Fprintln(out, "pw: first request to reach one answers 500 rather than failing at build time.")
	fmt.Fprintln(out, "pw: See requirement:tinybind-alternate-backend-support.")
	return true
}

// owesBinders reports whether this directory's derived handlers need binders
// that nothing generated.
//
// The condition is a binding artifact beside a derived one: a binder was
// emitted for the net/http build, so the same types are reachable from the
// derived handlers and the fasthttp registry has nothing for them. A package
// deriving handlers that bind nothing owes nothing and is not named.
func owesBinders(artifacts []generator.Artifact) bool {
	derived, binds := false, false
	for _, artifact := range artifacts {
		switch artifact.Kind {
		case generator.ArtifactTransport:
			derived = true
		case generator.ArtifactBinding:
			binds = true
		}
	}
	return derived && binds
}

// transportArtifactBase names the derived handlers. One file per package, not
// one per source: admission closes over the call graph, so a handler and the
// helper it hands the request to may be authored apart.
const transportArtifactBase = "transport"

// planTransport derives one package's handlers for the second transport.
//
// It runs the analysis and the rewrite here rather than letting the generator
// run them as part of GenerateArtifacts, for one reason: the generator analyzes
// every file of the loaded package, and in a project laid out this way that
// includes the files the last run generated. Deriving those a second time is
// wrong twice over — a generated page decoder would be emitted both by the
// fasthttp page emitter and by the rewrite of the net/http one, and a generated
// binder is refused outright, because it captures the request in a closure and
// the eligibility rule correctly calls that an escape.
//
// So a refusal in generated code would fail a run over source the developer did
// not write and cannot fix. What decides it is the same rule discovery already
// follows: generated code is output rather than input.
func planTransport(directory string, second secondBuild) (generator.Artifact, bool, error) {
	if !second.declared() {
		return generator.Artifact{}, false, nil
	}
	pkg, err := loadForTransform(directory)
	if err != nil || pkg == nil {
		return generator.Artifact{}, false, err
	}
	plan, err := generator.AnalyzeTransform(pkg, *second.transform)
	if err != nil {
		return generator.Artifact{}, false, fmt.Errorf("%s: %w", directory, err)
	}
	plan = authoredOnly(pkg, plan)
	if len(plan.Refusals) > 0 {
		return generator.Artifact{}, false, refusalError(directory, plan.Refusals)
	}
	if len(plan.Admitted) == 0 {
		return generator.Artifact{}, false, nil
	}
	out, err := generator.RewriteTransform(pkg, plan, *second.transform)
	if err != nil {
		return generator.Artifact{}, false, fmt.Errorf("%s: %w", directory, err)
	}
	for _, warning := range out.LayoutWarnings {
		fmt.Fprintln(second.warnings, warning)
	}
	return generator.Artifact{
		Kind:        generator.ArtifactTransport,
		Destination: generator.DestinationGoPackage,
		OutputBase:  transportArtifactBase,
		Extension:   generator.ExtensionGo,
		PackageName: pkg.Name,
		// The source arrives carrying //go:build fasthttp, which the merge
		// preserves and constrainNetHTTP defers to.
		Content: out.Source,
	}, true, nil
}

// loadForTransform type-checks one directory, or reports that there was no Go
// package there to check.
//
// A directory holding only templates is the ordinary case rather than a
// failure: a page tree root has no Go source of its own until this run writes
// its registry.
func loadForTransform(directory string) (*packages.Package, error) {
	absolute, err := filepath.Abs(directory)
	if err != nil {
		return nil, err
	}
	loaded, err := packages.Load(&packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax |
			packages.NeedTypes | packages.NeedTypesInfo | packages.NeedDeps | packages.NeedImports,
		Dir: absolute,
	}, ".")
	if err != nil {
		return nil, fmt.Errorf("%s: %w", directory, err)
	}
	if len(loaded) == 0 {
		return nil, nil
	}
	pkg := loaded[0]
	if pkg.TypesInfo == nil || len(pkg.Syntax) == 0 {
		return nil, nil
	}
	// A package that does not type-check says nothing about what is
	// transformable, and the reason it does not is reported by the compiler with
	// better context than anything reconstructed here.
	if len(pkg.Errors) > 0 {
		return nil, nil
	}
	return pkg, nil
}

// authoredOnly drops everything the last run generated, so the derivation reads
// what a developer wrote and nothing else.
//
// The file name is what decides it. Every file this project generates ends in
// the same suffix, whichever writer produced it, and the sweep in planDirectory
// deletes any that no longer has a producer — so the suffix is the identity
// rather than a convention that might drift.
func authoredOnly(pkg *packages.Package, plan *generator.TransformPlan) *generator.TransformPlan {
	if plan == nil {
		return &generator.TransformPlan{}
	}
	out := &generator.TransformPlan{}
	for _, candidate := range plan.Admitted {
		if generated(pkg.Fset.Position(candidate.Decl.Pos()).Filename) {
			continue
		}
		out.Admitted = append(out.Admitted, candidate)
	}
	for _, refusal := range plan.Refusals {
		if generated(refusal.Position.Filename) {
			continue
		}
		out.Refusals = append(out.Refusals, refusal)
	}
	return out
}

func generated(path string) bool {
	return strings.HasSuffix(filepath.Base(path), pwgen.PageComponentSuffix)
}

// refusalError renders what the second build cannot compile.
//
// Adoption is all-or-nothing, so one refusal stops the run: there is no adapter
// for a handler to fall back to, and generating the rest would leave a package
// that silently serves fewer routes.
//
// The upstream text is relayed rather than reworded, per rule:transport-handle-checks,
// with one addition it cannot make: a refusal naming a pw call is this
// framework's missing call pattern rather than the application's mistake, and
// saying so is the difference between a developer editing their handler for an
// hour and reporting a framework bug in a minute.
func refusalError(directory string, refusals generator.TransformRefusals) error {
	message := "the fasthttp build cannot be derived from " + directory + ":\n" + refusals.Error()
	if names := frameworkRefusals(refusals); len(names) > 0 {
		message += "\n" + strings.Join(names, "\n") +
			"\nThose name framework entries with no registered call pattern, which is a defect in " +
			"Popcorn Wave rather than in this application: no edit to a handler can supply one. " +
			"Please report it.\n"
	}
	return fmt.Errorf("%s", message)
}

// frameworkRefusals picks out the refusals caused by a pw call.
//
// It reads the rendered detail rather than the call graph, because the detail is
// what upstream publishes about a refusal and re-deriving the callee here would
// be a second opinion to keep in agreement. The detail names the callee by its
// package name and function, so "pw." is the whole test.
func frameworkRefusals(refusals generator.TransformRefusals) []string {
	var out []string
	for _, refusal := range refusals {
		if refusal.Kind != generator.RefusalUnknownCall || !strings.Contains(refusal.Detail, " to pw.") {
			continue
		}
		out = append(out, "  "+refusal.Position.String()+"  "+refusal.Detail)
	}
	return out
}
