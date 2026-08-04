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
	// defaultConsolePort sits beside the identity provider's, for the opposite
	// reason: not because anything derives an identity from it, but because a
	// console is bookmarked, and a reserved port would hand out a new address
	// every run.
	defaultConsolePort = 18081
)

// Target compilers recorded by project.toolchain. Projects scaffolded before the
// key existed used TinyGo compatible routing, so that stays the default.
const (
	toolchainTinyGo = "tinygo"
	toolchainGo     = "go"
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

// consoleConfig selects the development console `pw dev` serves beside the
// application: one loopback listener holding the index and every pane.
//
// The port is fixed rather than reserved, which is the opposite of every other
// development listener here. A reserved port moves on every run, and a surface
// the developer bookmarks and returns to all day cannot move. The telemetry
// receiver keeps its reserved port because the address it publishes is one a
// process is handed rather than one a person types.
type consoleConfig struct {
	Enabled bool
	Port    int
	// Assets enables the static asset pane. Each pane has a key of its own so
	// that turning one off is not turning the console off.
	Assets bool
	// Overlay puts the loop's failures over the pages the application serves.
	// Turning it off is what makes a development page byte-identical to a
	// production one, because nothing is served to load.
	Overlay bool
	// Reload reloads a page whose application has been replaced. It only
	// applies where the overlay is already attached.
	Reload bool
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
}

// watchConfig widens or trims the pw dev walk. Unlike generation, the walk has a
// working default, because a rebuild is triggered by any compiled input; only
// the trimming is worth leaving to the project.
type watchConfig struct {
	Includes []string
	Excludes []string
}

type projectConfig struct {
	Name      string
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
	Console   consoleConfig
	Migration migrationConfig
	Seed      seedConfig
	Tailwind  tailwindConfig
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
		"project.name", "project.main", "project.toolchain", "project.database",
		"generate.handlers", "generate.templates", "generate.queries", "generate.config", "generate.pages",
		"generate.dynamo",
		"dev.watch.includes", "dev.watch.excludes",
		"seed.auto",
		"dev.idp.enabled", "dev.idp.config", "dev.idp.port",
		"dev.otel.enabled", "dev.otel.port", "dev.otel.max",
		"dev.console.enabled", "dev.console.port", "dev.console.assets.enabled",
		"dev.console.overlay.enabled", "dev.console.overlay.reload",
		"migration.dir", "migration.auto",
		"assets.tailwind.enabled", "assets.tailwind.input",
		"assets.tailwind.output", "assets.tailwind.minify",
	}
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
	config.Main, err = scalar(document, "project.main")
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
	if config.Main == "" {
		return projectConfig{}, fmt.Errorf("popcornwave.toml: project.main is required")
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
	config.Console.Enabled = true
	config.Console.Assets = true
	config.Console.Overlay = true
	config.Console.Reload = true
	config.Console.Port = defaultConsolePort
	if value, ok := document.Get("dev.console.enabled"); ok {
		config.Console.Enabled, err = value.AsBool()
		if err != nil {
			return projectConfig{}, fmt.Errorf("popcornwave.toml: dev.console.enabled: %w", err)
		}
	}
	if value, ok := document.Get("dev.console.assets.enabled"); ok {
		config.Console.Assets, err = value.AsBool()
		if err != nil {
			return projectConfig{}, fmt.Errorf("popcornwave.toml: dev.console.assets.enabled: %w", err)
		}
	}
	if value, ok := document.Get("dev.console.overlay.enabled"); ok {
		config.Console.Overlay, err = value.AsBool()
		if err != nil {
			return projectConfig{}, fmt.Errorf("popcornwave.toml: dev.console.overlay.enabled: %w", err)
		}
	}
	if value, ok := document.Get("dev.console.overlay.reload"); ok {
		config.Console.Reload, err = value.AsBool()
		if err != nil {
			return projectConfig{}, fmt.Errorf("popcornwave.toml: dev.console.overlay.reload: %w", err)
		}
	}
	if value, ok := document.Get("dev.console.port"); ok {
		port, err := value.AsInt()
		if err != nil {
			return projectConfig{}, fmt.Errorf("popcornwave.toml: dev.console.port: %w", err)
		}
		if port < 0 || port > 65535 {
			return projectConfig{}, fmt.Errorf("popcornwave.toml: dev.console.port must be between 0 and 65535")
		}
		config.Console.Port = int(port)
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
