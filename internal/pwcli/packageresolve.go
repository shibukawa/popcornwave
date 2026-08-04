package pwcli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/shibukawa/popcornwave/internal/pwmigrate"
	"github.com/shibukawa/tinybind-go/minitoml"
)

// resolvedPackage is a declared module located on disk with its manifest read.
// Resolution happens through the Go tool rather than by guessing a module cache
// path, so a replace directive, a workspace, and a vendored build all resolve to
// whatever the build itself would use.
type resolvedPackage struct {
	Module   string
	Dir      string
	Manifest packageManifest
}

// ImportPath is what the generated bootstrap blank-imports. A package whose Go
// lives below the module root says so in the manifest; otherwise the module path
// is the import path.
func (p resolvedPackage) ImportPath() string {
	if p.Manifest.Import != "" {
		return p.Manifest.Import
	}
	return p.Module
}

// errPackageNotInModuleGraph reports a declaration the build cannot resolve. It
// is distinguished from other failures because the fix is a go get rather than
// anything about the package itself.
var errPackageNotInModuleGraph = errors.New("not in the module graph")

// resolvePackages locates every declared package and reads its manifest. It
// returns them in declaration order, which loadProjectConfig already sorted by
// module path, so the generated import block does not move when a declaration is
// reordered.
func resolvePackages(ctx context.Context, root string, refs []packageRef) ([]resolvedPackage, error) {
	if len(refs) == 0 {
		return nil, nil
	}
	resolved := make([]resolvedPackage, 0, len(refs))
	for _, ref := range refs {
		dir, err := moduleDir(ctx, root, ref.Module)
		if err != nil {
			return nil, fmt.Errorf("packages %q: %w", ref.Module, err)
		}
		manifest, err := readPackageManifest(dir)
		if err != nil {
			return nil, fmt.Errorf("packages %q: %w", ref.Module, err)
		}
		// A manifest carrying a different module path is one copied out of
		// another repository, and every identity derived from it — the asset
		// URLs, the migration stem, the doctor report — would name the wrong
		// package.
		if manifest.Module != ref.Module {
			return nil, fmt.Errorf("packages %q: its manifest declares package.module = %q", ref.Module, manifest.Module)
		}
		resolved = append(resolved, resolvedPackage{Module: ref.Module, Dir: dir, Manifest: manifest})
	}
	return resolved, nil
}

// moduleDir asks the Go tool where a module's source is. An unresolvable module
// is reported as a declaration the build graph does not carry, because that is
// the actionable half: the fix is adding it to go.mod, not editing the package.
func moduleDir(ctx context.Context, root, module string) (string, error) {
	command := exec.CommandContext(ctx, "go", "list", "-m", "-f", "{{.Dir}}", module)
	command.Dir = root
	output, err := command.Output()
	if err != nil {
		return "", errPackageNotInModuleGraph
	}
	dir := strings.TrimSpace(string(output))
	if dir == "" {
		// A module in the graph with no directory has not been downloaded. The
		// build would fetch it; a manifest read cannot, so it is reported rather
		// than silently skipped.
		return "", fmt.Errorf("%w: run go mod download", errPackageNotInModuleGraph)
	}
	return dir, nil
}

// readPackageManifest reads the package section of a module's own
// popcornwave.toml. It parses the file directly rather than going through
// loadProjectConfig, because that loader validates generation directories
// against the project it is loading, and a dependency's directories are not this
// project's to check.
func readPackageManifest(dir string) (packageManifest, error) {
	path := filepath.Join(dir, "popcornwave.toml")
	source, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return packageManifest{}, fmt.Errorf("no popcornwave.toml, so it publishes no package section; remove the declaration or use it as an ordinary Go dependency")
		}
		return packageManifest{}, err
	}
	document, err := minitoml.Parse(source)
	if err != nil {
		return packageManifest{}, fmt.Errorf("parse %s: %w", path, err)
	}
	kind, err := optionalScalar(document, "project.kind")
	if err != nil {
		return packageManifest{}, err
	}
	if kind != kindPackage {
		return packageManifest{}, fmt.Errorf("its popcornwave.toml is not a package; a declaration claims a capability the module does not publish")
	}
	return loadPackageManifest(document)
}

// checkPackageCompatibility reports what the project cannot satisfy. It runs
// before anything is written, so a package that cannot work here is named rather
// than half installed.
func checkPackageCompatibility(config projectConfig, resolved []resolvedPackage, capabilities map[string]bool) error {
	for _, pkg := range resolved {
		// The engine is checked here and again before the stream applies. A
		// package installed into a project whose engine it never wrote schema
		// for would fail at the first migration rather than at the declaration.
		if len(pkg.Manifest.Requires.Engines) > 0 && !slices.Contains(pkg.Manifest.Requires.Engines, config.Database) {
			return fmt.Errorf(
				"packages %q supports %s, and this project uses %q",
				pkg.Module, strings.Join(pkg.Manifest.Requires.Engines, ", "), config.Database,
			)
		}
		for _, capability := range pkg.Manifest.Requires.Capabilities {
			if capabilities != nil && !capabilities[capability] {
				return fmt.Errorf("packages %q needs the %s capability, which this project does not have; run pw add %s first", pkg.Module, capability, capability)
			}
		}
	}
	// Two packages writing into one version table would each read the other's
	// applied versions as its own, so the collision is refused at the project
	// level as well as at registration.
	stems := make(map[string]string, len(resolved))
	for _, pkg := range resolved {
		stem := pkg.Manifest.Migrations.Stem
		if stem == "" {
			continue
		}
		if owner, taken := stems[stem]; taken {
			return fmt.Errorf("packages %q and %q share the migration stem %q", owner, pkg.Module, stem)
		}
		stems[stem] = pkg.Module
	}
	return nil
}

// packageStreams builds the migration streams of every declared package, in the
// order they must apply.
//
// The sources come from the module directory rather than from the package's
// embedded copy, because pw applies migrations without an application binary and
// therefore cannot reach another binary's embedded data. The two are the same
// files; a package whose embed pattern misses one is a package whose two readers
// disagree, which its own release check exists to catch.
func packageStreams(ctx context.Context, root string, resolved []resolvedPackage) ([]pwmigrate.Stream, error) {
	withStream := make([]resolvedPackage, 0, len(resolved))
	for _, pkg := range resolved {
		if pkg.Manifest.Migrations.Dir != "" {
			withStream = append(withStream, pkg)
		}
	}
	if len(withStream) == 0 {
		return nil, nil
	}
	ordered, err := orderByModuleGraph(ctx, root, withStream)
	if err != nil {
		return nil, err
	}
	streams := make([]pwmigrate.Stream, 0, len(ordered))
	for _, pkg := range ordered {
		dir := filepath.Join(pkg.Dir, filepath.FromSlash(pkg.Manifest.Migrations.Dir))
		sources, err := pwmigrate.Sources(dir)
		if err != nil {
			return nil, fmt.Errorf("packages %q: %w", pkg.Module, err)
		}
		streams = append(streams, pwmigrate.Stream{
			Module:  pkg.Module,
			Stem:    pkg.Manifest.Migrations.Stem,
			Sources: sources,
		})
	}
	return streams, nil
}

// orderByModuleGraph sorts declared packages so a package applies after every
// declared package it depends on.
//
// The Go module graph is the only dependency direction that can exist between
// two packages, and it is the one that matters: a package cannot reference a
// table it has never seen, so if one package's schema builds on another's, it
// imports it. Nothing else is derived from the graph.
func orderByModuleGraph(ctx context.Context, root string, packages []resolvedPackage) ([]resolvedPackage, error) {
	byModule := make(map[string]resolvedPackage, len(packages))
	for _, pkg := range packages {
		byModule[pkg.Module] = pkg
	}
	edges, err := declaredModuleEdges(ctx, root, byModule)
	if err != nil {
		return nil, err
	}
	// Kahn's algorithm over the declared subgraph, with the ready set kept in
	// module path order so the sequence is identical on every run.
	remaining := make(map[string]int, len(packages))
	for module := range byModule {
		remaining[module] = 0
	}
	for _, targets := range edges {
		for target := range targets {
			if _, declared := remaining[target]; declared {
				remaining[target]++
			}
		}
	}
	ordered := make([]resolvedPackage, 0, len(packages))
	for len(remaining) > 0 {
		ready := make([]string, 0, len(remaining))
		for module, count := range remaining {
			if count == 0 {
				ready = append(ready, module)
			}
		}
		if len(ready) == 0 {
			// The Go module graph cannot hold a cycle, so this is unreachable
			// through go.mod. Falling back to module path order keeps a future
			// edge source from turning an ordering question into a hang.
			for module := range remaining {
				ready = append(ready, module)
			}
			slices.Sort(ready)
			for _, module := range ready {
				ordered = append(ordered, byModule[module])
			}
			break
		}
		slices.Sort(ready)
		for _, module := range ready {
			ordered = append(ordered, byModule[module])
			delete(remaining, module)
			for target := range edges[module] {
				if _, declared := remaining[target]; declared {
					remaining[target]--
				}
			}
		}
	}
	// Edges point from a dependency to its dependent, so this order already puts
	// a package after everything it depends on.
	return ordered, nil
}

// declaredModuleEdges reads go mod graph and keeps only the edges between
// declared packages. The full graph is far larger than the declared set, and
// nothing outside it can carry a schema dependency.
func declaredModuleEdges(ctx context.Context, root string, declared map[string]resolvedPackage) (map[string]map[string]bool, error) {
	command := exec.CommandContext(ctx, "go", "mod", "graph")
	command.Dir = root
	output, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("read the module graph: %w", err)
	}
	edges := make(map[string]map[string]bool)
	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		from, to := modulePathOf(fields[0]), modulePathOf(fields[1])
		if from == to {
			continue
		}
		if _, ok := declared[from]; !ok {
			continue
		}
		if _, ok := declared[to]; !ok {
			continue
		}
		// from depends on to, so to must apply first: the edge is stored in
		// dependency-to-dependent direction.
		if edges[to] == nil {
			edges[to] = make(map[string]bool)
		}
		edges[to][from] = true
	}
	return edges, nil
}

// modulePathOf strips the version suffix go mod graph appends. The main module
// appears without one.
func modulePathOf(node string) string {
	if at := strings.LastIndex(node, "@"); at >= 0 {
		return node[:at]
	}
	return node
}
