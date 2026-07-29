package pwcli

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/shibukawa/tinybind-go/minitoml"
)

const (
	defaultTailwindInput  = "assets/app.css"
	defaultTailwindOutput = "public/generated/app.css"
	defaultMigrationDir   = "migrations"
	defaultIdPConfig      = "devidp.toml"
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

type projectConfig struct {
	Name       string
	Main       string
	Toolchain  string
	ExtraWatch []string
	IdP        idpConfig
	Otel       otelConfig
	Migration  migrationConfig
	Tailwind   tailwindConfig
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
		"project.name", "project.main", "project.toolchain",
		"dev.extra_watch",
		"dev.idp.enabled", "dev.idp.config", "dev.idp.port",
		"dev.otel.enabled", "dev.otel.port", "dev.otel.max",
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
	config.ExtraWatch, err = array(document, "dev.extra_watch")
	if err != nil {
		return projectConfig{}, fmt.Errorf("popcornwave.toml: dev.extra_watch: %w", err)
	}
	for _, pattern := range config.ExtraWatch {
		if filepath.IsAbs(pattern) {
			return projectConfig{}, fmt.Errorf("popcornwave.toml: dev.extra_watch paths must be relative")
		}
		if _, err := filepath.Glob(filepath.Join(root, filepath.FromSlash(pattern))); err != nil {
			return projectConfig{}, fmt.Errorf("popcornwave.toml: dev.extra_watch %q: %w", pattern, err)
		}
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
