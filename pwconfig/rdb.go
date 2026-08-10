package pwconfig

import (
	"errors"
	"fmt"
	"strings"

	"github.com/shibukawa/popcornwave/database"
	"github.com/shibukawa/popcornwave/pwruntime"
)

// The connection groups are resolved here rather than beside the pools they
// open, because which group receives a migration is a question the settings
// file answers and not one the pool does. Opening is the runtime's; deciding
// what was asked for is this package's, and a tool that needs the schema
// without a running server needs the second half without the first.

// ResolveConnections expands the configuration into the effective connection
// list, in configuration order.
//
// An element naming no group joins the default group, so the single-database
// project writes one element and names nothing.
func ResolveConnections(config RDBConfig) ([]RDBConnectionConfig, error) {
	if len(config.Connections) == 0 {
		// A file written against the removed single-DSN form lands here: the
		// key it sets is claimed by no binding, so the section reads as an
		// enabled database with no pool behind it.
		return nil, errors.New("middleware.rdb.enabled needs at least one [[middleware.rdb.connections]] element; the middleware.rdb.dsn form was removed, so move that DSN into an element")
	}
	connections := make([]RDBConnectionConfig, 0, len(config.Connections))
	for _, connection := range config.Connections {
		if connection.Group == "" {
			connection.Group = pwruntime.DefaultConnectionGroup
		}
		connections = append(connections, connection)
	}
	return connections, nil
}

// connectionGroups lists the configured group names in configuration order.
func connectionGroups(connections []RDBConnectionConfig) []string {
	var groups []string
	seen := make(map[string]bool, len(connections))
	for _, connection := range connections {
		if !seen[connection.Group] {
			seen[connection.Group] = true
			groups = append(groups, connection.Group)
		}
	}
	return groups
}

// writableGroups lists the groups holding at least one writable connection.
func writableGroups(connections []RDBConnectionConfig) []string {
	var groups []string
	seen := make(map[string]bool, len(connections))
	for _, connection := range connections {
		if connection.ReadOnly || seen[connection.Group] {
			continue
		}
		seen[connection.Group] = true
		groups = append(groups, connection.Group)
	}
	return groups
}

// ResolveDefaultGroup names the group serving statements that pin no group.
func ResolveDefaultGroup(config RDBConfig, connections []RDBConnectionConfig) (string, error) {
	groups := connectionGroups(connections)
	if configured := strings.TrimSpace(config.DefaultGroup); configured != "" {
		if !containsString(groups, configured) {
			return "", fmt.Errorf("middleware.rdb.default_group %q names no configured connection group", configured)
		}
		return configured, nil
	}
	if len(groups) != 1 {
		return "", errors.New("middleware.rdb.default_group is required when more than one connection group is configured")
	}
	return groups[0], nil
}

// ResolveWriteGroup names the group serving framework-owned writes.
//
// An ambiguous topology is a startup error rather than a silent pick, because
// picking the wrong one writes to a replica.
func ResolveWriteGroup(config RDBConfig, connections []RDBConnectionConfig) (string, error) {
	return ResolveWritableGroup(config, connections, strings.TrimSpace(config.WriteGroup), "middleware.rdb.write_group")
}

// ResolveMigrationGroup names the group receiving migrations and seed data.
func ResolveMigrationGroup(config RDBConfig, connections []RDBConnectionConfig) (string, error) {
	if configured := strings.TrimSpace(config.MigrationGroup); configured != "" {
		return ResolveWritableGroup(config, connections, configured, "middleware.rdb.migration_group")
	}
	return ResolveWriteGroup(config, connections)
}

func ResolveWritableGroup(config RDBConfig, connections []RDBConnectionConfig, configured, key string) (string, error) {
	writable := writableGroups(connections)
	if configured != "" {
		if !containsString(connectionGroups(connections), configured) {
			return "", fmt.Errorf("%s %q names no configured connection group", key, configured)
		}
		if !containsString(writable, configured) {
			return "", fmt.Errorf("%s %q holds no writable connection", key, configured)
		}
		return configured, nil
	}
	switch len(writable) {
	case 0:
		return "", errors.New("middleware.rdb has no writable connection; framework-owned writes have nowhere to go")
	case 1:
		return writable[0], nil
	default:
		return "", fmt.Errorf("%s is required when more than one group holds a writable connection", key)
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

// Target resolves a configured DSN onto the engine that opens it, naming the
// scheme when nothing claims it.
func Target(configured string) (database.Target, error) {
	target, err := database.Resolve(configured)
	if err != nil {
		return database.Target{}, fmt.Errorf("popcornwave: %w", err)
	}
	return target, nil
}

// MigrationDSN reports the DSN of the group receiving migrations and seed data.
//
// It is exported because migration and seeding tooling has to reach the schema
// without reimplementing group resolution.
func (config RDBConfig) MigrationDSN() (string, error) {
	if !config.Enabled {
		return "", errors.New("popcornwave: middleware.rdb.enabled is false")
	}
	connections, err := ResolveConnections(config)
	if err != nil {
		return "", fmt.Errorf("popcornwave: %w", err)
	}
	group, err := ResolveMigrationGroup(config, connections)
	if err != nil {
		return "", fmt.Errorf("popcornwave: %w", err)
	}
	for _, connection := range connections {
		if connection.Group != group {
			continue
		}
		if _, err := Target(connection.DSN); err != nil {
			return "", fmt.Errorf("popcornwave: %w", err)
		}
		return connection.DSN, nil
	}
	return "", fmt.Errorf("popcornwave: migration group %q has no connection", group)
}
