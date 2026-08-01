package dynamo

import (
	"fmt"
	"strings"
	"time"
)

// dynamoDB bounds a table name. The service accepts three to 255 characters of
// letters, digits, underscore, hyphen and dot, so a resolved name outside that
// is rejected here rather than at the first request, where the resolved name
// would appear in no message.
const (
	minTableNameLength = 3
	maxTableNameLength = 255
)

// Config is the [middleware.dynamo] runtime binding. It is registered when this
// package is imported, so a project that does not use DynamoDB gains no key.
//
// It carries no billing mode and no capacity. Table creation happens in
// development and test, where the target is an emulator that ignores both, and
// a deployed table is defined by deployment tooling.
type Config struct {
	// Enabled opens the client and installs the middleware.
	Enabled bool `toml:"enabled" help:"Enabled opens the client and installs the middleware"`
	// Region names the AWS region. Empty falls back to the environment, and a
	// region resolvable from neither is a startup error.
	Region string `toml:"region" help:"Region names the AWS region. Empty falls back to the environment, and a region resolvable from neither is a startup error"`
	// Endpoint overrides the regional host, which is how a local emulator is
	// reached.
	Endpoint string `toml:"endpoint" help:"Endpoint overrides the regional host, which is how a local emulator is reached"`
	// AccessKeyID and its siblings are static credentials. All three are
	// optional; empty selects the driver's environment credentials.
	AccessKeyID     string `toml:"access_key_id" help:"AccessKeyID and its siblings are static credentials. All three are optional; empty selects the driver's environment credentials"`
	SecretAccessKey string `toml:"secret_access_key"`
	SessionToken    string `toml:"session_token"`
	// TablePrefix is prepended to a declared table name.
	TablePrefix string `toml:"table_prefix" help:"TablePrefix is prepended to a declared table name"`
	// TableNames maps a declared name onto a deployed one, for a name no
	// prefix produces. An entry wins over the prefix.
	//
	// It is an array of tables rather than a map because configbind binds no
	// map type, and because that is the form middleware.rdb.connections
	// already uses for a repeated element.
	TableNames []TableName `toml:"table_names" help:"TableNames maps a declared name onto a deployed one, for a name no prefix produces. An entry wins over the prefix"`
	// Timeout bounds one request.
	Timeout time.Duration `toml:"timeout" help:"Timeout bounds one request"`
	// MaxIdleConns sizes the connection pool. The rule of thumb is the
	// expected concurrency.
	MaxIdleConns int `toml:"max_idle_conns" help:"MaxIdleConns sizes the connection pool. The rule of thumb is the expected concurrency"`
	// VerifySchema reads every registered table once at startup and refuses to
	// serve on a mismatch. It is the one check deployment tooling cannot make
	// for itself, so it defaults on.
	VerifySchema bool `toml:"verify_schema" help:"VerifySchema reads every registered table once at startup and refuses to serve on a mismatch. It is the one check deployment tooling cannot make for itself, so it defaults on"`
	// AutoMigrate creates missing tables during startup. It is a development
	// convenience and is rejected elsewhere.
	AutoMigrate bool `toml:"auto_migrate" help:"AutoMigrate creates missing tables during startup. It is a development convenience and is rejected elsewhere"`
}

// TableName is one [[middleware.dynamo.table_names]] element: the name source
// declares, and the name this deployment gave it.
type TableName struct {
	Declared string `toml:"declared"`
	Deployed string `toml:"deployed"`
}

// DefaultConfig is the binding's zero-value replacement. VerifySchema defaults
// on because it is the production value of this package.
func DefaultConfig() Config {
	return Config{
		Timeout:      10 * time.Second,
		MaxIdleConns: 4,
		VerifySchema: true,
	}
}

// validate reports the configuration problems that can be seen without a
// network. development reports whether the diagnosed environment is the
// development one, which is the only place AutoMigrate is allowed.
func (config Config) validate(development bool) error {
	if !config.Enabled {
		return nil
	}
	if config.Timeout <= 0 {
		return fmt.Errorf("middleware.dynamo.timeout must be positive, got %s", config.Timeout)
	}
	if config.MaxIdleConns < 0 {
		return fmt.Errorf("middleware.dynamo.max_idle_conns cannot be negative, got %d", config.MaxIdleConns)
	}
	// One static credential without the other is a half-configured deployment
	// that would otherwise fall back to the environment and fail confusingly.
	// The values themselves never enter the message.
	hasKey := strings.TrimSpace(config.AccessKeyID) != ""
	hasSecret := strings.TrimSpace(config.SecretAccessKey) != ""
	switch {
	case hasKey && !hasSecret:
		return fmt.Errorf("middleware.dynamo.access_key_id is set without middleware.dynamo.secret_access_key")
	case hasSecret && !hasKey:
		return fmt.Errorf("middleware.dynamo.secret_access_key is set without middleware.dynamo.access_key_id")
	}
	if config.AutoMigrate && !development {
		return fmt.Errorf("middleware.dynamo.auto_migrate is a development setting; a deployed table comes from deployment tooling")
	}
	if err := validateNamePart("middleware.dynamo.table_prefix", config.TablePrefix, true); err != nil {
		return err
	}
	seen := make(map[string]bool, len(config.TableNames))
	for index, mapping := range config.TableNames {
		if strings.TrimSpace(mapping.Declared) == "" {
			return fmt.Errorf("middleware.dynamo.table_names[%d].declared cannot be empty", index)
		}
		if seen[mapping.Declared] {
			return fmt.Errorf("middleware.dynamo.table_names maps %q twice", mapping.Declared)
		}
		seen[mapping.Declared] = true
		if err := validateNamePart(
			fmt.Sprintf("middleware.dynamo.table_names[%q].deployed", mapping.Declared),
			mapping.Deployed, false); err != nil {
			return err
		}
	}
	return nil
}

// validateNamePart checks the character set of a configured name fragment. A
// prefix may be empty and a mapping target may not.
func validateNamePart(key, value string, allowEmpty bool) error {
	if value == "" {
		if allowEmpty {
			return nil
		}
		return fmt.Errorf("%s cannot be empty", key)
	}
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '_', r == '-', r == '.':
		default:
			return fmt.Errorf("%s contains %q, which DynamoDB does not accept in a table name", key, r)
		}
	}
	return nil
}

// validateResolvedName checks a name the resolver produced. It runs over the
// whole registered set at startup rather than at the first request, because a
// name that is too long otherwise surfaces as a service error that does not
// contain the resolved name.
func validateResolvedName(declared, deployed string) error {
	switch {
	case len(deployed) < minTableNameLength:
		return fmt.Errorf("table %q resolves to %q, shorter than the %d characters DynamoDB requires",
			declared, deployed, minTableNameLength)
	case len(deployed) > maxTableNameLength:
		return fmt.Errorf("table %q resolves to a name of %d characters, over the DynamoDB limit of %d",
			declared, len(deployed), maxTableNameLength)
	}
	return validateNamePart(fmt.Sprintf("table %q", declared), deployed, false)
}
