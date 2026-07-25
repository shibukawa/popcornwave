// Package pwenv resolves the runtime environment token that selects
// project-local configuration files.
package pwenv

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Var names the environment variable that selects the runtime environment.
const Var = "APP_ENV"

// Well-known runtime environments. Any other lowercase token is also accepted.
const (
	Development = "dev"
	Staging     = "stg"
	Production  = "prod"
)

// Default is used when Var is unset or empty.
const Default = Development

// NeutralFileName is the environment-neutral name used by the user and system
// configuration directories.
const NeutralFileName = "config.toml"

// Resolve reads Var from environ, or from the process environment when environ
// is nil, and validates it as a config filename component.
func Resolve(environ []string) (string, error) {
	value := lookup(environ, Var)
	if strings.TrimSpace(value) == "" {
		return Default, nil
	}
	if !Valid(value) {
		return "", fmt.Errorf("popcornwave: invalid %s %q: use lowercase letters, digits, '-' or '_'", Var, value)
	}
	return value, nil
}

// Valid reports whether value is usable as a config filename component.
func Valid(value string) bool {
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_':
		default:
			return false
		}
	}
	return value != ""
}

// FileName returns the project-local configuration file name for env.
func FileName(env string) string {
	return "config." + env + ".toml"
}

// IsFileName reports whether name is an environment-specific configuration
// file. The environment-neutral config.toml is never read from a project tree.
func IsFileName(name string) bool {
	return strings.HasPrefix(name, "config.") && strings.HasSuffix(name, ".toml") && name != NeutralFileName
}

// ReadPaths returns the project-local candidates for env in search order: the
// working directory first, then its config/ directory.
func ReadPaths(env string) []string {
	name := FileName(env)
	return []string{name, filepath.Join("config", name)}
}

func lookup(environ []string, key string) string {
	if environ == nil {
		return os.Getenv(key)
	}
	prefix := key + "="
	value := ""
	for _, entry := range environ {
		if strings.HasPrefix(entry, prefix) {
			value = strings.TrimPrefix(entry, prefix)
		}
	}
	return value
}
