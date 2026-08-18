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

// DevLogFileVar names the private handoff from pw dev to the application
// process. The value is an absolute JSONL path selected once per developer-loop
// invocation; it is not a runtime configuration key and is never exported to
// the developer's shell.
const DevLogFileVar = "PW_DEV_LOG_FILE"

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
	value, _, err := ResolveDeclared(environ)
	return value, err
}

// ResolveDeclared is Resolve, and also reports whether the environment was named
// rather than defaulted.
//
// The distinction exists because "dev" is two different facts wearing one name.
// A deployment that sets APP_ENV=dev is asking for the development relaxations.
// A deployment that sets nothing is not asking for anything — it forgot — and
// answering "dev" to that is how a production service ends up logging every SQL
// statement with its bind values and accepting a session cookie without Secure.
//
// Which is why the relaxations key off declared rather than off the token. The
// token still defaults, because it also selects the config file and refusing to
// start over a missing variable would be a poor trade for that. `pw dev` sets
// the variable, so the ordinary development path declares it and keeps
// everything it had.
func ResolveDeclared(environ []string) (value string, declared bool, err error) {
	raw := lookup(environ, Var)
	if strings.TrimSpace(raw) == "" {
		return Default, false, nil
	}
	if !Valid(raw) {
		return "", false, fmt.Errorf("popcornweb: invalid %s %q: use lowercase letters, digits, '-' or '_'", Var, raw)
	}
	return raw, true, nil
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
