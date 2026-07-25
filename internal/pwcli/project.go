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

type projectConfig struct {
	Name       string
	Main       string
	ExtraWatch []string
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
		"project.name", "project.main",
		"dev.extra_watch",
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
