package pw

import (
	"io/fs"
	"sort"
	"strings"
	"sync"
)

// Package is one linked component package. An imported package registers itself
// from an init function, so only a package the binary actually links contributes
// anything, exactly as an Extension does.
//
// The value carries identity and bytes and nothing else. Behaviour a package
// contributes goes through the mechanisms that already exist: middleware and
// startup work through RegisterExtension, configuration through the generated
// configuration binding, and routes through an exported Register function the
// application calls on its own mux. Registration installs nothing that answers
// a request.
type Package struct {
	// Module is the Go module path. It is the identity everything else is keyed
	// by, and it must match the package.module of the module's manifest so a
	// declaration and a linked binary describe the same thing.
	Module string
	// Version is the module version the package was built at, supplied by the
	// package as a constant. It is evidence for a compatibility report rather
	// than a constraint: go.mod already performed the resolution.
	Version string
	// Assets holds embedded browser files, or nil. They are read once when the
	// asset mount is built, never per request and never from a filesystem, which
	// is what lets a TinyGo target with no filesystem serve them at all.
	Assets fs.FS
	// Migrations holds the package's own migration stream, or nil. The stream is
	// applied before the application's, so a package's tables exist before
	// anything the application writes can reference them.
	Migrations fs.FS
	// MigrationStem identifies the stream. It names the stream's version table
	// and prefixes the package's own tables, which is how two packages are kept
	// from claiming one name silently. It is required when Migrations is set.
	MigrationStem string
}

var packageState = struct {
	sync.Mutex
	registered []Package
}{}

// RegisterPackage adds one component package to the framework. Imported packages
// call it from an init function.
//
// Registration is identity rather than activation: nothing a package registers
// reaches a request until the application mounts its assets, applies its stream,
// or calls its own Register.
func RegisterPackage(pkg Package) {
	if strings.TrimSpace(pkg.Module) == "" {
		panic("popcornwave: empty package module")
	}
	if pkg.Migrations != nil && strings.TrimSpace(pkg.MigrationStem) == "" {
		panic("popcornwave: package " + pkg.Module + " has migrations with no stem")
	}
	packageState.Lock()
	defer packageState.Unlock()
	for _, existing := range packageState.registered {
		if existing.Module == pkg.Module {
			panic("popcornwave: duplicate package " + pkg.Module)
		}
		// Two packages sharing a stem would write into one version table and
		// prefix their tables identically, so each would see the other's applied
		// versions as its own.
		if pkg.MigrationStem != "" && existing.MigrationStem == pkg.MigrationStem {
			panic("popcornwave: packages " + existing.Module + " and " + pkg.Module + " share the migration stem " + pkg.MigrationStem)
		}
	}
	packageState.registered = append(packageState.registered, pkg)
}

// Packages returns the registered packages in module path order, which is how a
// running application answers what it actually linked. The order is stable so a
// report does not move between runs.
func Packages() []Package {
	packageState.Lock()
	defer packageState.Unlock()
	ordered := append([]Package(nil), packageState.registered...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Module < ordered[j].Module })
	return ordered
}
