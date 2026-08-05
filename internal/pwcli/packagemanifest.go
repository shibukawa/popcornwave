package pwcli

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/shibukawa/tinybind-go/minitoml"
)

// packageManifest is the package section of a component package's
// popcornwave.toml. It declares what the module publishes, and it exists as a
// file rather than as Go because every reader needs the answer before anything
// is compiled: pw generate must know what to import in order to emit the import,
// and pw migrate applies a package's stream without an application binary at
// all. Neither can ask a process that does not exist yet.
//
// The file ships inside the module, so a consuming project reads it out of the
// module cache with no network access and no build.
type packageManifest struct {
	// Module is the Go module path, repeated from go.mod so a manifest copied
	// into the wrong module is detectable rather than silently wrong.
	Module string
	// Summary is one line, shown when pw reports or offers the package.
	Summary string
	// Import is the package path an application links, when it differs from the
	// module root.
	Import string
	// Requires is what the project must already have for the package to work.
	Requires packageRequires
	// GeneratedWith records the versions that produced the committed artifacts.
	// It constrains nothing on its own: go.mod performs the resolution, and this
	// is the evidence pw doctor compares against the supported window.
	GeneratedWith packageVersions
	// ConfigSection is the runtime configuration section the package registers.
	// It is reported and can be scaffolded on request; it is never written into
	// an environment file at install, because the package's registered defaults
	// already apply without one.
	ConfigSection string
	// Migrations is the package's own migration stream.
	Migrations packageMigrations
	// Register is the exported symbol an application calls to mount the package,
	// empty for a package that serves no route. The framework never calls it:
	// mounting is the one contribution an application has an opinion about, so
	// pw prints the call and the application writes it.
	Register string
	// Assets reports whether the package registers embedded browser files. No
	// path or URL appears here, because both are derived from content.
	Assets bool
	// Components are the .pw.html components a consumer template may call. It
	// stays empty until cross-package component resolution exists upstream; an
	// entry before then is a load error rather than a promise.
	Components []string
}

type packageRequires struct {
	// Capabilities are the project capabilities the package needs, such as
	// database.
	Capabilities []string
	// Engines are the SQL engines the package supports. Empty means it touches
	// no SQL, which is the shape of every package until generated queries can
	// carry more than one dialect.
	Engines []string
}

type packageVersions struct {
	PW       string
	TinyBind string
}

type packageMigrations struct {
	// Dir holds the stream, relative to the module root.
	Dir string
	// Stem identifies the stream. It names the version table and prefixes the
	// package's own tables, which is how a package stays detectable at any
	// version and how two packages are kept from claiming one table name.
	Stem string
	// Engines are the engines the stream is written for. It must not be
	// narrower than Requires.Engines, or the package would declare support it
	// has no schema for.
	Engines []string
}

// packageManifestKeys are the keys the package section may carry. They are
// listed here rather than inline so the application-side loader can reject them
// as one set.
var packageManifestKeys = []string{
	"package.module", "package.summary", "package.import",
	"package.requires.capabilities", "package.requires.engines",
	"package.generated_with.pw", "package.generated_with.tinybind",
	"package.config.section",
	"package.migrations.dir", "package.migrations.stem", "package.migrations.engines",
	"package.routes.register",
	"package.assets.declared",
	"package.components.exported",
}

// loadPackageManifest reads the package section of an already-parsed document.
// The document is passed in rather than a path because the package's own project
// loader has already parsed the file, and re-reading it would let the two
// disagree.
func loadPackageManifest(document minitoml.Document) (packageManifest, error) {
	manifest := packageManifest{}
	var err error
	manifest.Module, err = scalar(document, "package.module")
	if err != nil {
		return packageManifest{}, err
	}
	if manifest.Module == "" {
		return packageManifest{}, fmt.Errorf("popcornwave.toml: package.module is required")
	}
	for _, binding := range []struct {
		key    string
		target *string
	}{
		{"package.summary", &manifest.Summary},
		{"package.import", &manifest.Import},
		{"package.generated_with.pw", &manifest.GeneratedWith.PW},
		{"package.generated_with.tinybind", &manifest.GeneratedWith.TinyBind},
		{"package.config.section", &manifest.ConfigSection},
		{"package.migrations.dir", &manifest.Migrations.Dir},
		{"package.migrations.stem", &manifest.Migrations.Stem},
		{"package.routes.register", &manifest.Register},
	} {
		*binding.target, err = optionalScalar(document, binding.key)
		if err != nil {
			return packageManifest{}, err
		}
	}
	for _, binding := range []struct {
		key    string
		target *[]string
	}{
		{"package.requires.capabilities", &manifest.Requires.Capabilities},
		{"package.requires.engines", &manifest.Requires.Engines},
		{"package.migrations.engines", &manifest.Migrations.Engines},
		{"package.components.exported", &manifest.Components},
	} {
		values, err := array(document, binding.key)
		if err != nil {
			return packageManifest{}, fmt.Errorf("popcornwave.toml: %s: %w", binding.key, err)
		}
		*binding.target = values
	}
	if value, ok := document.Get("package.assets.declared"); ok {
		manifest.Assets, err = value.AsBool()
		if err != nil {
			return packageManifest{}, fmt.Errorf("popcornwave.toml: package.assets.declared: %w", err)
		}
	}
	if err := manifest.validate(); err != nil {
		return packageManifest{}, err
	}
	return manifest, nil
}

func (m packageManifest) validate() error {
	if strings.TrimSpace(m.Module) != m.Module || m.Module == "" {
		return fmt.Errorf("popcornwave.toml: package.module must be a module path")
	}
	for _, engine := range m.Requires.Engines {
		if !validEngine(engine) {
			return fmt.Errorf("popcornwave.toml: package.requires.engines %q must be %s", engine, engineNames())
		}
	}
	for _, engine := range m.Migrations.Engines {
		if !validEngine(engine) {
			return fmt.Errorf("popcornwave.toml: package.migrations.engines %q must be %s", engine, engineNames())
		}
	}
	// A package declaring support for an engine its schema was never written for
	// would install into a project it cannot serve, and the failure would land
	// at the first query rather than at the declaration.
	if m.Migrations.Dir != "" {
		for _, engine := range m.Requires.Engines {
			if !slices.Contains(m.Migrations.Engines, engine) {
				return fmt.Errorf("popcornwave.toml: package.migrations.engines does not cover %q, which package.requires.engines declares", engine)
			}
		}
	}
	if m.Migrations.Dir != "" {
		if filepath.IsAbs(m.Migrations.Dir) {
			return fmt.Errorf("popcornwave.toml: package.migrations.dir must be relative to the module")
		}
		clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(m.Migrations.Dir)))
		if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
			return fmt.Errorf("popcornwave.toml: package.migrations.dir must name a directory inside the module")
		}
		if m.Migrations.Stem == "" {
			return fmt.Errorf("popcornwave.toml: package.migrations.stem is required beside package.migrations.dir; it names the version table and the package's own tables")
		}
	}
	if m.Migrations.Stem != "" && m.Migrations.Dir == "" {
		return fmt.Errorf("popcornwave.toml: package.migrations.stem names a stream package.migrations.dir does not locate")
	}
	if m.Migrations.Stem != "" && !validMigrationStem(m.Migrations.Stem) {
		return fmt.Errorf("popcornwave.toml: package.migrations.stem %q must be lowercase letters, digits, and underscores", m.Migrations.Stem)
	}
	// Cross-package component resolution does not exist upstream: an external
	// declaration still has no spelling for a target outside its own generation
	// unit. Accepting the key would publish a promise nothing can keep.
	if len(m.Components) > 0 {
		return fmt.Errorf("popcornwave.toml: package.components.exported is not supported yet; a component in another module is not callable until an external declaration can name one")
	}
	return nil
}

// validMigrationStem keeps the stem usable as both a table name part and a file
// name part, on every engine, with no quoting anywhere.
func validMigrationStem(stem string) bool {
	if stem == "" {
		return false
	}
	for index, r := range stem {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9':
			if index == 0 {
				return false
			}
		case r == '_':
			if index == 0 {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// packageRefs reads the consuming project's declarations. The list is what links
// each module, so a duplicate is an error rather than a set: two entries for one
// module would emit one import and read as two installs.
func packageRefs(document minitoml.Document) ([]packageRef, error) {
	value, ok := document.Get("packages")
	if !ok {
		return nil, nil
	}
	tables, err := value.AsTables()
	if err != nil {
		return nil, fmt.Errorf("popcornwave.toml: packages: %w", err)
	}
	refs := make([]packageRef, 0, len(tables))
	seen := make(map[string]bool, len(tables))
	for _, table := range tables {
		for _, key := range table.Keys() {
			if key != "module" {
				return nil, fmt.Errorf("popcornwave.toml: packages: unknown key %s", key)
			}
		}
		module, err := scalar(table, "module")
		if err != nil {
			return nil, fmt.Errorf("popcornwave.toml: packages: %w", err)
		}
		if module == "" {
			return nil, fmt.Errorf("popcornwave.toml: packages: module is required")
		}
		if seen[module] {
			return nil, fmt.Errorf("popcornwave.toml: packages lists %q twice", module)
		}
		seen[module] = true
		refs = append(refs, packageRef{Module: module})
	}
	slices.SortFunc(refs, func(a, b packageRef) int { return strings.Compare(a.Module, b.Module) })
	return refs, nil
}
