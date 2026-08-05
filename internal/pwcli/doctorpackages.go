package pwcli

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/shibukawa/popcornwave/internal/pwcheck"
)

// checkPackages reports where a project's component package declarations and
// what the build actually carries disagree.
//
// Declaring a package is the whole install, so every failure here is a
// disagreement between the declaration and something else — the module graph,
// the database, or the version that generated the package — rather than a step
// somebody forgot to run.
func (r *checkRun) checkPackages(ctx context.Context) {
	declared := r.State.config.Packages
	resolved := make([]resolvedPackage, 0, len(declared))
	for _, ref := range declared {
		dir, err := moduleDir(ctx, r.Root, ref.Module)
		if err != nil {
			if errors.Is(err, errPackageNotInModuleGraph) {
				r.report(pwcheck.PackageNotResolved,
					fmt.Sprintf("packages declares %q, and the module graph does not carry it", ref.Module),
					"popcornwave.toml packages")
			}
			continue
		}
		manifest, err := readPackageManifest(dir)
		if err != nil {
			r.report(pwcheck.PackageNotResolved,
				fmt.Sprintf("packages declares %q: %v", ref.Module, err),
				"popcornwave.toml packages")
			continue
		}
		resolved = append(resolved, resolvedPackage{Module: ref.Module, Dir: dir, Manifest: manifest})
	}
	r.checkPackageImports(ctx, resolved)
	r.checkPackageVersions(ctx, resolved)
	r.checkUndeclaredPackages(ctx, declared)
}

// checkPackageImports reports a declared package whose import path holds no Go.
//
// Generation blank-imports that path, so the failure otherwise surfaces at
// `go build` as the Go tool's "no required module provides package" — which
// names neither the declaration that caused it nor the manifest key that
// decides the path. A package whose Go lives below its module root has to say
// so in `package.import`, and forgetting that is the ordinary way to get here.
func (r *checkRun) checkPackageImports(ctx context.Context, resolved []resolvedPackage) {
	for _, pkg := range resolved {
		path := pkg.ImportPath()
		if packageHasGo(ctx, r.Root, path) {
			continue
		}
		message := fmt.Sprintf("%q has no Go package at %s, which is what the generated bootstrap imports", pkg.Module, path)
		if pkg.Manifest.Import == "" {
			message += "; its manifest sets no package.import, so the module root is used"
		}
		r.report(pwcheck.PackageImportMissing, message, "popcornwave.toml packages")
	}
}

// packageHasGo asks the Go tool whether an import path resolves to a package.
// The question is the same one the compiler will ask about the generated blank
// import, so a directory listing of .go files would only approximate it — build
// constraints, a directory of tests alone, and a replace directive all change
// the answer.
func packageHasGo(ctx context.Context, root, importPath string) bool {
	command := exec.CommandContext(ctx, "go", "list", "-e", "-f", "{{if .Error}}error{{else}}{{.Name}}{{end}}", importPath)
	command.Dir = root
	output, err := command.Output()
	if err != nil {
		// The Go tool could not answer at all. That is a different condition
		// from an empty package, and PW0140 already covers an unresolvable
		// module, so this stays quiet rather than guessing.
		return true
	}
	answer := strings.TrimSpace(string(output))
	return answer != "" && answer != "error"
}

// checkUndeclaredPackages reports a module in the graph that publishes a package
// section this project never declared.
//
// Nothing links such a module on its own, so this is not a failure. It is a
// surprise worth naming: a dependency shipping assets and a schema that the
// project has not asked for is exactly what the declaration exists to make
// visible.
func (r *checkRun) checkUndeclaredPackages(ctx context.Context, declared []packageRef) {
	byModule := make(map[string]bool, len(declared))
	for _, ref := range declared {
		byModule[ref.Module] = true
	}
	command := exec.CommandContext(ctx, "go", "list", "-m", "-f", "{{.Path}}\t{{.Dir}}", "all")
	command.Dir = r.Root
	output, err := command.Output()
	if err != nil {
		// Without the graph this reports nothing rather than guessing. A project
		// whose modules are not downloaded has louder problems than this.
		return
	}
	for _, line := range strings.Split(string(output), "\n") {
		module, dir, found := strings.Cut(strings.TrimSpace(line), "\t")
		if !found || dir == "" || module == "" || byModule[module] {
			continue
		}
		manifest, err := readPackageManifest(dir)
		if err != nil || manifest.Module != module {
			continue
		}
		r.report(pwcheck.PackageNotDeclared,
			fmt.Sprintf("%q publishes a component package and this project does not declare it", module),
			"go.mod")
	}
}

// projectFrameworkVersion is the Popcorn Wave version this project resolves,
// which is what a package's committed artifacts will actually link against. The
// version of the pw binary running the check is a different thing and would be
// the wrong comparison: a CLI can diagnose a project it was not built alongside.
func projectFrameworkVersion(ctx context.Context, root string) string {
	command := exec.CommandContext(ctx, "go", "list", "-m", "-f", "{{.Version}}", frameworkModulePath)
	command.Dir = root
	output, err := command.Output()
	if err != nil {
		return ""
	}
	version := strings.TrimSpace(string(output))
	if version == "" || version == "(devel)" {
		return ""
	}
	return version
}

const frameworkModulePath = "github.com/shibukawa/popcornwave"

// checkPackageVersions compares what generated each package's committed
// artifacts against what this project builds with.
//
// A package generated by an older version is accepted: its artifacts call a
// runtime that still exists. A newer one may call an entry this version does not
// have, which is a compile error worth naming before the compiler reaches it.
func (r *checkRun) checkPackageVersions(ctx context.Context, resolved []resolvedPackage) {
	current := projectFrameworkVersion(ctx, r.Root)
	if current == "" {
		// A project building against a local checkout or a replace directive has
		// no version to compare, and inventing one would report a mismatch that
		// is not there.
		return
	}
	for _, pkg := range resolved {
		generated := pkg.Manifest.GeneratedWith.PW
		if generated == "" {
			continue
		}
		if compareVersions(generated, current) > 0 {
			r.report(pwcheck.PackageGeneratorNewer,
				fmt.Sprintf("%q was generated with Popcorn Wave %s, and this project builds with %s",
					pkg.Module, generated, current),
				"popcornwave.toml packages")
		}
	}
}
