package pwcli

import (
	"bytes"
	"context"
	"os/exec"
	"slices"
	"strings"
)

// importGraph is the set of packages the application main package reaches,
// including blank imports. Registration in this framework is a property of the
// linked binary rather than of any file, so this set is what makes a missing
// plugin or driver import visible without building or running anything.
type importGraph struct {
	packages map[string]bool
	// Err records why the graph is unavailable, so the checks that need it are
	// reported as not run rather than as passing.
	Err error
}

func (g importGraph) available() bool { return g.Err == nil && len(g.packages) > 0 }

// links reports whether the application reaches the given import path.
func (g importGraph) links(path string) bool { return g.packages[path] }

// linksPrefix reports whether the application reaches any package below path.
func (g importGraph) linksPrefix(prefix string) bool {
	for path := range g.packages {
		if path == prefix || strings.HasPrefix(path, prefix+"/") {
			return true
		}
	}
	return false
}

// resolveImportGraph lists the transitive imports of the main package. go list
// resolves the same package set the compiler would, so a renamed or blank
// import is an edge like any other.
func resolveImportGraph(ctx context.Context, root, mainPackage string) importGraph {
	command := exec.CommandContext(ctx, "go", "list", "-deps", "-f", "{{.ImportPath}}", mainPackage)
	command.Dir = root
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	if err := command.Run(); err != nil {
		// A project that does not build is exactly when doctor is run, so the
		// packages go list did resolve are kept and the failure is reported.
		graph := importGraph{packages: map[string]bool{}, Err: goListError(err, stderr.String())}
		for _, line := range strings.Split(stdout.String(), "\n") {
			if path := strings.TrimSpace(line); path != "" {
				graph.packages[path] = true
			}
		}
		if len(graph.packages) > 0 {
			return graph
		}
		return importGraph{Err: graph.Err}
	}
	packages := map[string]bool{}
	for _, line := range strings.Split(stdout.String(), "\n") {
		if path := strings.TrimSpace(line); path != "" {
			packages[path] = true
		}
	}
	return importGraph{packages: packages}
}

func goListError(err error, stderr string) error {
	message := strings.TrimSpace(stderr)
	if message == "" {
		return err
	}
	lines := strings.Split(message, "\n")
	if len(lines) > 3 {
		lines = lines[:3]
	}
	return &listError{detail: strings.Join(lines, "; ")}
}

type listError struct{ detail string }

func (e *listError) Error() string { return e.detail }

// Framework and plugin packages the checks reason about. A third-party plugin
// is not in this list and is not reported as missing: the checks only speak
// about wiring they can name a remedy for.
const (
	frameworkPackage    = "github.com/shibukawa/popcornwave/pw"
	authPluginPackage   = "github.com/shibukawa/popcornwave/plugin/auth"
	rdbSessionPackage   = "github.com/shibukawa/popcornwave/sessionstore/sqlite"
	devIdPPackagePrefix = "github.com/shibukawa/popcornwave/contrib/devidp"
	sqliteDriverPackage = "github.com/shibukawa/tinygodriver/database/sql/sqlite"
	mysqlDriverPackage  = "github.com/shibukawa/tinygodriver/database/sql/mysql"
)

// sessionBackendPackages maps a session.backend value to the plugin that
// registers it.
var sessionBackendPackages = map[string]string{
	"rdb": rdbSessionPackage,
}

// driverPackages maps a DSN scheme to the driver package that answers it.
var driverPackages = map[string]string{
	"sqlite": sqliteDriverPackage,
	"mysql":  mysqlDriverPackage,
}

// configPrefixOwners maps a configuration prefix to the package that must be
// linked for the prefix to mean anything. A prefix absent here belongs to the
// framework, which every application links through pw.
var configPrefixOwners = map[string]string{
	"auth": authPluginPackage,
}

// frameworkPrefixes are the bindings pw registers on its own.
var frameworkPrefixes = []string{"server", "security", "session", "observability", "middleware", "html"}

func sortStrings(values []string) { slices.Sort(values) }
