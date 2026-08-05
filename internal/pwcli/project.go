package pwcli

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/shibukawa/tinybind-go/minitoml"
)

const (
	defaultTailwindInput  = "assets/app.css"
	defaultTailwindOutput = "public/generated/app.css"
	defaultMigrationDir   = "migrations"
	defaultIdPConfig      = "devidp.toml"
	// defaultIdPPort is what the scaffold pins the development provider to. A
	// reserved port would move on every run, and the issuer it appears in is
	// part of the account identity the scaffolded resolver derives.
	defaultIdPPort = 18080
	// defaultImageQuality is libwebp's own default, so a project that sets
	// nothing gets what the encoder considers reasonable rather than a number
	// this framework invented.
	defaultImageQuality = 75
)

// Target compilers recorded by project.toolchain. Projects scaffolded before the
// key existed used TinyGo compatible routing, so that stays the default.
const (
	toolchainTinyGo = "tinygo"
	toolchainGo     = "go"
)

// Project kinds recorded by project.kind. An application builds a binary; a
// package is published as a Go module and imported by one. Every project written
// before the key existed is an application, because it was the only kind there
// was, so the absent key means that rather than an error.
const (
	kindApplication = "application"
	kindPackage     = "package"
)

type tailwindConfig struct {
	Enabled bool
	Input   string
	Output  string
	Minify  bool
}

type migrationConfig struct {
	Dir  string
	Auto bool
}

// seedConfig governs the one place pw applies a dataset on its own: after the
// developer loop rolls a schema back to reach an edited migration, which empties
// the tables below it. Seeding is clear-insert, so it never runs on an ordinary
// rebuild, where it would delete what the developer just typed in.
type seedConfig struct {
	Auto bool
}

// idpConfig selects the development identity provider `pw dev` runs beside the
// application. The port defaults to 0 because pw dev injects the resolved
// issuer, so a fixed port only matters to an externally registered client.
type idpConfig struct {
	Enabled bool
	Config  string
	Port    int
}

// otelConfig selects the telemetry viewer `pw dev` runs beside the application.
// It is on by default because an observable developer loop is the point of it,
// and the port defaults to 0 because pw dev injects the resolved endpoint. Max
// bounds the retained records per signal; zero keeps the viewer default.
type otelConfig struct {
	Enabled bool
	Port    int
	Max     int
}

// generationScope records, per generation purpose, the directories pw generate
// may read for it. Paths are project-relative and in slash form. No purpose has
// a default: a project states where each kind of generated code comes from
// instead of letting one walk of the tree feed all of them.
type generationScope struct {
	Handlers  []string
	Templates []string
	Queries   []string
	Config    []string
	// Pages lists page tree roots. An entry is a whole tree rather than a
	// directory of independent sources: one entry is one generation run and one
	// generated registry.
	Pages []string
	// Dynamo lists the directories holding dynamo-tagged types and .pw.dynamo
	// query declarations. It reads Go type declarations rather than a template
	// language, which is why it is not part of generate.queries.
	Dynamo []string
	// Firestore lists the directories holding firestore-tagged types and
	// .pw.firestore query declarations, on the same terms as Dynamo. The two
	// are separate purposes because a project may have either store, and a
	// directory listed for one is not a generation source for the other.
	Firestore []string
}

// generatePurposes are the configuration keys of generationScope, in the order
// their errors and listings are reported.
var generatePurposes = []struct {
	key    string
	target func(*generationScope) *[]string
	// optional marks a purpose whose absent key means the empty list. Only a
	// purpose added after the key set was fixed is one: every project written
	// before page trees or DynamoDB existed has no such key, and none of them
	// has the sources either.
	optional bool
}{
	{key: "generate.handlers", target: func(s *generationScope) *[]string { return &s.Handlers }},
	{key: "generate.templates", target: func(s *generationScope) *[]string { return &s.Templates }},
	{key: "generate.queries", target: func(s *generationScope) *[]string { return &s.Queries }},
	{key: "generate.config", target: func(s *generationScope) *[]string { return &s.Config }},
	{key: "generate.pages", target: func(s *generationScope) *[]string { return &s.Pages }, optional: true},
	{key: "generate.dynamo", target: func(s *generationScope) *[]string { return &s.Dynamo }, optional: true},
	{key: "generate.firestore", target: func(s *generationScope) *[]string { return &s.Firestore }, optional: true},
}

// watchConfig widens or trims the pw dev walk. Unlike generation, the walk has a
// working default, because a rebuild is triggered by any compiled input; only
// the trimming is worth leaving to the project.
type watchConfig struct {
	Includes []string
	Excludes []string
}

// packageRef is one entry of the consuming project's packages array. It names a
// module and nothing else: the declaration says the application intends to use
// that package, which is a different fact from go.mod saying the module is
// available, and it is the fact the bootstrap generator needs in order to know
// what to import.
type packageRef struct {
	Module string
}

type projectConfig struct {
	Name string
	// Kind selects which commands apply and which keys are legal. It is read
	// before anything else, so a command that does not apply to the kind is
	// reported as such rather than failing later on a missing key.
	Kind      string
	Main      string
	Toolchain string
	// Database is the engine .pw.sql sources generate for. It sits beside the
	// toolchain because both are build-time properties of the source tree: one
	// decides which compiler reads it, the other which dialect it is written
	// in. The runtime engine still comes from the rdb DSN scheme, which must
	// agree with this.
	Database  string
	Generate  generationScope
	Watch     watchConfig
	IdP       idpConfig
	Otel      otelConfig
	Migration migrationConfig
	Seed      seedConfig
	Tailwind  tailwindConfig
	// Assets is the build-time conversion set. Every field defaults to off, so
	// a project that declares nothing embeds a copy of its authored tree and
	// serves exactly what it served before any of this existed.
	Assets assetsConfig
	// Packages are the component packages an application declares. Declaring one
	// is what links it: pw generate emits the blank import from this list, so a
	// module in go.mod and not here is an ordinary dependency.
	Packages []packageRef
	// Package is the manifest a package project publishes. It is empty for an
	// application, and an application carrying the section is an error.
	Package packageManifest
}

func loadProjectConfig(root string) (projectConfig, error) {
	path := filepath.Join(root, "popcornwave.toml")
	source, err := os.ReadFile(path)
	if err != nil {
		return projectConfig{}, fmt.Errorf("read %s: %w", path, err)
	}
	document, err := minitoml.Parse(source)
	if err != nil {
		return projectConfig{}, fmt.Errorf("parse %s: %w", path, err)
	}
	known := []string{
		"project.name", "project.kind", "project.main", "project.toolchain", "project.database",
		"packages",
		"generate.handlers", "generate.templates", "generate.queries", "generate.config", "generate.pages",
		"generate.dynamo", "generate.firestore",
		"dev.watch.includes", "dev.watch.excludes",
		"seed.auto",
		"dev.idp.enabled", "dev.idp.config", "dev.idp.port",
		"dev.otel.enabled", "dev.otel.port", "dev.otel.max",
		"migration.dir", "migration.auto",
		"assets.tailwind.enabled", "assets.tailwind.input",
		"assets.tailwind.output", "assets.tailwind.minify",
		"assets.css.minify", "assets.images.enabled", "assets.images.quality",
		"assets.images.avif", "assets.scripts.enabled",
	}
	known = append(known, packageManifestKeys...)
	for _, key := range document.Keys() {
		if !slices.Contains(known, key) {
			return projectConfig{}, fmt.Errorf("popcornwave.toml: unknown key %s", key)
		}
	}
	config := projectConfig{}
	config.Name, err = scalar(document, "project.name")
	if err != nil {
		return projectConfig{}, err
	}
	config.Kind, err = optionalScalar(document, "project.kind")
	if err != nil {
		return projectConfig{}, err
	}
	if config.Kind == "" {
		config.Kind = kindApplication
	}
	if config.Kind != kindApplication && config.Kind != kindPackage {
		return projectConfig{}, fmt.Errorf("popcornwave.toml: project.kind must be %q or %q", kindApplication, kindPackage)
	}
	config.Main, err = optionalScalar(document, "project.main")
	if err != nil {
		return projectConfig{}, err
	}
	config.Toolchain, err = optionalScalar(document, "project.toolchain")
	if err != nil {
		return projectConfig{}, err
	}
	if config.Toolchain == "" {
		config.Toolchain = toolchainTinyGo
	}
	if config.Toolchain != toolchainTinyGo && config.Toolchain != toolchainGo {
		return projectConfig{}, fmt.Errorf("popcornwave.toml: project.toolchain must be %q or %q", toolchainTinyGo, toolchainGo)
	}
	config.Database, err = optionalScalar(document, "project.database")
	if err != nil {
		return projectConfig{}, err
	}
	if config.Database == "" {
		// Projects scaffolded before the key existed could only be SQLite, and
		// it generates what they already have.
		config.Database = engineSQLite
	}
	if !validEngine(config.Database) {
		return projectConfig{}, fmt.Errorf("popcornwave.toml: project.database must be %s", engineNames())
	}
	config.Generate, err = generationSources(document, root)
	if err != nil {
		return projectConfig{}, err
	}
	config.Watch, err = watchPaths(document, root)
	if err != nil {
		return projectConfig{}, err
	}
	if err := config.readKindSections(document); err != nil {
		return projectConfig{}, err
	}
	if value, ok := document.Get("dev.idp.enabled"); ok {
		config.IdP.Enabled, err = value.AsBool()
		if err != nil {
			return projectConfig{}, fmt.Errorf("popcornwave.toml: dev.idp.enabled: %w", err)
		}
	}
	config.IdP.Config, err = optionalScalar(document, "dev.idp.config")
	if err != nil {
		return projectConfig{}, err
	}
	if config.IdP.Config == "" {
		config.IdP.Config = defaultIdPConfig
	}
	if filepath.IsAbs(config.IdP.Config) {
		return projectConfig{}, fmt.Errorf("popcornwave.toml: dev.idp.config must be relative to the project")
	}
	if value, ok := document.Get("dev.idp.port"); ok {
		port, err := value.AsInt()
		if err != nil {
			return projectConfig{}, fmt.Errorf("popcornwave.toml: dev.idp.port: %w", err)
		}
		if port < 0 || port > 65535 {
			return projectConfig{}, fmt.Errorf("popcornwave.toml: dev.idp.port must be between 0 and 65535")
		}
		config.IdP.Port = int(port)
	}
	config.Otel.Enabled = true
	if value, ok := document.Get("dev.otel.enabled"); ok {
		config.Otel.Enabled, err = value.AsBool()
		if err != nil {
			return projectConfig{}, fmt.Errorf("popcornwave.toml: dev.otel.enabled: %w", err)
		}
	}
	if value, ok := document.Get("dev.otel.port"); ok {
		port, err := value.AsInt()
		if err != nil {
			return projectConfig{}, fmt.Errorf("popcornwave.toml: dev.otel.port: %w", err)
		}
		if port < 0 || port > 65535 {
			return projectConfig{}, fmt.Errorf("popcornwave.toml: dev.otel.port must be between 0 and 65535")
		}
		config.Otel.Port = int(port)
	}
	if value, ok := document.Get("dev.otel.max"); ok {
		max, err := value.AsInt()
		if err != nil {
			return projectConfig{}, fmt.Errorf("popcornwave.toml: dev.otel.max: %w", err)
		}
		if max < 0 {
			return projectConfig{}, fmt.Errorf("popcornwave.toml: dev.otel.max must not be negative")
		}
		config.Otel.Max = int(max)
	}
	config.Migration.Dir, err = optionalScalar(document, "migration.dir")
	if err != nil {
		return projectConfig{}, err
	}
	if config.Migration.Dir == "" {
		config.Migration.Dir = defaultMigrationDir
	}
	if filepath.IsAbs(config.Migration.Dir) {
		return projectConfig{}, fmt.Errorf("popcornwave.toml: migration.dir must be relative to the project")
	}
	config.Migration.Auto = true
	if value, ok := document.Get("migration.auto"); ok {
		config.Migration.Auto, err = value.AsBool()
		if err != nil {
			return projectConfig{}, fmt.Errorf("popcornwave.toml: migration.auto: %w", err)
		}
	}
	config.Seed.Auto = true
	if value, ok := document.Get("seed.auto"); ok {
		config.Seed.Auto, err = value.AsBool()
		if err != nil {
			return projectConfig{}, fmt.Errorf("popcornwave.toml: seed.auto: %w", err)
		}
	}
	for _, binding := range []struct {
		key    string
		target *bool
	}{
		{"assets.css.minify", &config.Assets.CSSMinify},
		{"assets.images.enabled", &config.Assets.Images},
		{"assets.images.avif", &config.Assets.AVIF},
		{"assets.scripts.enabled", &config.Assets.Scripts},
	} {
		value, ok := document.Get(binding.key)
		if !ok {
			continue
		}
		parsed, err := value.AsBool()
		if err != nil {
			return projectConfig{}, fmt.Errorf("popcornwave.toml: %s: %w", binding.key, err)
		}
		*binding.target = parsed
	}
	// The quality only reaches a lossy source. Its default is libwebp's own, so
	// a project that states nothing gets what the tool considers reasonable.
	config.Assets.ImageQuality = defaultImageQuality
	if value, ok := document.Get("assets.images.quality"); ok {
		quality, err := value.AsInt()
		if err != nil {
			return projectConfig{}, fmt.Errorf("popcornwave.toml: assets.images.quality: %w", err)
		}
		if quality < 1 || quality > 100 {
			return projectConfig{}, fmt.Errorf("popcornwave.toml: assets.images.quality must be between 1 and 100")
		}
		config.Assets.ImageQuality = int(quality)
	}
	if value, ok := document.Get("assets.tailwind.enabled"); ok {
		config.Tailwind.Enabled, err = value.AsBool()
		if err != nil {
			return projectConfig{}, fmt.Errorf("popcornwave.toml: assets.tailwind.enabled: %w", err)
		}
	}
	if value, ok := document.Get("assets.tailwind.minify"); ok {
		config.Tailwind.Minify, err = value.AsBool()
		if err != nil {
			return projectConfig{}, fmt.Errorf("popcornwave.toml: assets.tailwind.minify: %w", err)
		}
	}
	config.Tailwind.Input, err = optionalScalar(document, "assets.tailwind.input")
	if err != nil {
		return projectConfig{}, err
	}
	config.Tailwind.Output, err = optionalScalar(document, "assets.tailwind.output")
	if err != nil {
		return projectConfig{}, err
	}
	if config.Tailwind.Enabled {
		if config.Tailwind.Input == "" {
			config.Tailwind.Input = defaultTailwindInput
		}
		if config.Tailwind.Output == "" {
			config.Tailwind.Output = defaultTailwindOutput
		}
		if filepath.IsAbs(config.Tailwind.Input) || filepath.IsAbs(config.Tailwind.Output) {
			return projectConfig{}, fmt.Errorf("popcornwave.toml: Tailwind input and output must be relative to the project")
		}
		input := filepath.Clean(filepath.Join(root, filepath.FromSlash(config.Tailwind.Input)))
		output := filepath.Clean(filepath.Join(root, filepath.FromSlash(config.Tailwind.Output)))
		if input == output {
			return projectConfig{}, fmt.Errorf("popcornwave.toml: Tailwind input and output must be different files")
		}
	}
	return config, nil
}

// readKindSections reads the keys that belong to one project kind and rejects
// the ones that belong to the other. The two are mutually exclusive rather than
// merely unused: a package has no entry point to name, and an application has
// nothing to publish, so each key appearing under the wrong kind is a mistake
// worth naming rather than a value to ignore.
func (config *projectConfig) readKindSections(document minitoml.Document) error {
	hasManifest := false
	for _, key := range packageManifestKeys {
		if _, ok := document.Get(key); ok {
			hasManifest = true
			break
		}
	}
	_, hasPackages := document.Get("packages")
	switch config.Kind {
	case kindPackage:
		if config.Main != "" {
			return fmt.Errorf("popcornwave.toml: project.main does not apply to a package; the application that imports it owns the entry point")
		}
		if hasPackages {
			return fmt.Errorf("popcornwave.toml: packages does not apply to a package; a package depends on another through go.mod like any Go module")
		}
		if !hasManifest {
			return fmt.Errorf("popcornwave.toml: a package project requires the package section; start with package.module")
		}
		manifest, err := loadPackageManifest(document)
		if err != nil {
			return err
		}
		config.Package = manifest
		// A generated query carries one engine's placeholder syntax, chosen when
		// the package was published, and a package cannot know its consumer's
		// engine. Generating one would ship a query that compiles and then fails
		// at the first call in half the projects that install it.
		if len(config.Generate.Queries) > 0 {
			return fmt.Errorf("popcornwave.toml: generate.queries must be empty in a package; a generated query is compiled for one engine and a package cannot know its consumer's")
		}
	default:
		if config.Main == "" {
			return fmt.Errorf("popcornwave.toml: project.main is required")
		}
		if hasManifest {
			return fmt.Errorf("popcornwave.toml: the package section applies to a package; set project.kind = %q to publish this module", kindPackage)
		}
		refs, err := packageRefs(document)
		if err != nil {
			return err
		}
		config.Packages = refs
	}
	return nil
}

// sourcesExample shows an operator what a purpose entry looks like, for the
// error raised when one of the required keys is missing.
const sourcesExample = `generate.handlers = ["handlers"]`

// generationSources reads the directories each generation purpose is allowed to
// read. Every purpose key is required, and an empty list is how a project says
// that a purpose generates nothing — which a missing key cannot express.
func generationSources(document minitoml.Document, root string) (generationScope, error) {
	scope := generationScope{}
	for _, purpose := range generatePurposes {
		directories, err := purposeDirectories(document, root, purpose.key, purpose.optional)
		if err != nil {
			return generationScope{}, err
		}
		*purpose.target(&scope) = directories
	}
	if err := checkPageRoots(scope); err != nil {
		return generationScope{}, err
	}
	return scope, nil
}

// checkPageRoots keeps a page tree from being read twice. The tree run already
// compiles the page and layout templates it holds, so a directory inside a root
// that another purpose also lists would have one output written twice, by two
// runs, with different content.
func checkPageRoots(scope generationScope) error {
	overlaps := func(key string, entries []string) error {
		for _, root := range scope.Pages {
			for _, entry := range entries {
				if entry == root || strings.HasPrefix(entry, root+"/") {
					return fmt.Errorf("popcornwave.toml: %s %q is inside the page tree root %q, which generates it already", key, entry, root)
				}
				if strings.HasPrefix(root, entry+"/") {
					return fmt.Errorf("popcornwave.toml: page tree root %q is inside %s %q, which would read its templates a second time", root, key, entry)
				}
			}
		}
		return nil
	}
	if err := overlaps("generate.templates", scope.Templates); err != nil {
		return err
	}
	return overlaps("generate.handlers", scope.Handlers)
}

// purposeDirectories validates one purpose list.
func purposeDirectories(document minitoml.Document, root, key string, optional bool) ([]string, error) {
	value, ok := document.Get(key)
	if !ok {
		if optional {
			return nil, nil
		}
		return nil, fmt.Errorf("popcornwave.toml: %s is required; list the directories it reads, such as %s, or [] when it generates nothing", key, sourcesExample)
	}
	entries, err := value.AsStringSlice()
	if err != nil {
		return nil, fmt.Errorf("popcornwave.toml: %s: %w", key, err)
	}
	seen := make(map[string]bool, len(entries))
	directories := make([]string, 0, len(entries))
	for _, entry := range entries {
		if filepath.IsAbs(entry) {
			return nil, fmt.Errorf("popcornwave.toml: %s %q must be relative to the project", key, entry)
		}
		clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(entry)))
		if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
			return nil, fmt.Errorf("popcornwave.toml: %s %q must name a directory inside the project", key, entry)
		}
		if seen[clean] {
			return nil, fmt.Errorf("popcornwave.toml: %s lists %q twice", key, entry)
		}
		seen[clean] = true
		info, err := os.Stat(filepath.Join(root, filepath.FromSlash(clean)))
		if err != nil {
			return nil, fmt.Errorf("popcornwave.toml: %s %q: %w", key, entry, err)
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("popcornwave.toml: %s %q is not a directory", key, entry)
		}
		directories = append(directories, clean)
	}
	// A nested entry would have its sources planned twice within one purpose,
	// and the second plan would delete what the first one wrote.
	for _, outer := range directories {
		for _, inner := range directories {
			if outer != inner && strings.HasPrefix(inner, outer+"/") {
				return nil, fmt.Errorf("popcornwave.toml: %s %q is already covered by %q", key, inner, outer)
			}
		}
	}
	slices.Sort(directories)
	return directories, nil
}

// watchPaths reads the pw dev walk adjustments. Both lists are optional,
// because the walk of the module is already the right default.
func watchPaths(document minitoml.Document, root string) (watchConfig, error) {
	config := watchConfig{}
	var err error
	config.Includes, err = array(document, "dev.watch.includes")
	if err != nil {
		return watchConfig{}, fmt.Errorf("popcornwave.toml: dev.watch.includes: %w", err)
	}
	for _, pattern := range config.Includes {
		if filepath.IsAbs(pattern) {
			return watchConfig{}, fmt.Errorf("popcornwave.toml: dev.watch.includes paths must be relative")
		}
		if _, err := filepath.Glob(filepath.Join(root, filepath.FromSlash(pattern))); err != nil {
			return watchConfig{}, fmt.Errorf("popcornwave.toml: dev.watch.includes %q: %w", pattern, err)
		}
	}
	config.Excludes, err = array(document, "dev.watch.excludes")
	if err != nil {
		return watchConfig{}, fmt.Errorf("popcornwave.toml: dev.watch.excludes: %w", err)
	}
	for index, entry := range config.Excludes {
		if filepath.IsAbs(entry) {
			return watchConfig{}, fmt.Errorf("popcornwave.toml: dev.watch.excludes paths must be relative")
		}
		clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(entry)))
		if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
			return watchConfig{}, fmt.Errorf("popcornwave.toml: dev.watch.excludes %q must name a directory inside the project", entry)
		}
		config.Excludes[index] = clean
	}
	return config, nil
}

func scalar(document minitoml.Document, key string) (string, error) {
	value, ok := document.Get(key)
	if !ok {
		return "", fmt.Errorf("popcornwave.toml: %s is required", key)
	}
	return value.AsString()
}

func optionalScalar(document minitoml.Document, key string) (string, error) {
	value, ok := document.Get(key)
	if !ok {
		return "", nil
	}
	result, err := value.AsString()
	if err != nil {
		return "", fmt.Errorf("popcornwave.toml: %s: %w", key, err)
	}
	return result, nil
}

func array(document minitoml.Document, key string) ([]string, error) {
	value, ok := document.Get(key)
	if !ok {
		return nil, nil
	}
	return value.AsStringSlice()
}

func projectRoot(start string) (string, error) {
	current, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(current, "popcornwave.toml")); err == nil {
			return current, nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", fmt.Errorf("popcornwave.toml not found")
		}
		current = parent
	}
}
