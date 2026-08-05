package firestore

import (
	"fmt"
	"strings"
	"time"
)

// Credential sources. The driver resolves GOOGLE_APPLICATION_CREDENTIALS and
// nothing else, so which source a deployment has is configured rather than
// discovered: probing the metadata server to find out would cost a round trip
// on every process that is not on Google Cloud, and would silently change the
// credential when a probe timed out.
const (
	// CredentialsServiceAccount signs a JWT with a service account key. It is
	// the default, and it needs no token endpoint round trip.
	CredentialsServiceAccount = "service_account"
	// CredentialsMetadata reads a token from the instance metadata server,
	// which is what Cloud Run, GKE and GCE have. It links no RSA code.
	CredentialsMetadata = "metadata"
	// CredentialsOAuth2 exchanges the key for an access token at the token
	// endpoint, for a deployment that requires a real one.
	CredentialsOAuth2 = "oauth2"
	// CredentialsStatic uses a token the process supplies through
	// SetTokenSource. It reads no key file.
	CredentialsStatic = "static"
)

// Config is the [middleware.firestore] runtime binding. It is registered when
// this package is imported, so a project that does not use Firestore gains no
// key.
//
// The database it names must have been created in Datastore mode. That is
// chosen when the database is created and cannot be changed afterwards, so
// startup checks it rather than letting a native-mode database answer the first
// request with a precondition failure.
//
// It carries no schema keys. Nothing creates a kind and nothing reports one, so
// a verify_schema or auto_migrate key here would configure nothing.
type Config struct {
	// Enabled opens the client and installs the middleware.
	Enabled bool `toml:"enabled" help:"Enabled opens the client and installs the middleware"`
	// ProjectID names the Google Cloud project. Empty falls back to
	// GOOGLE_CLOUD_PROJECT and then DATASTORE_PROJECT_ID, and a project
	// resolvable from none of them is a startup error.
	ProjectID string `toml:"project_id" help:"ProjectID names the Google Cloud project. Empty falls back to GOOGLE_CLOUD_PROJECT and then DATASTORE_PROJECT_ID"`
	// Database names a non-default database in that project. Empty selects the
	// project's default database.
	Database string `toml:"database" help:"Database names a non-default database in that project. Empty selects the project's default database"`
	// Namespace scopes every key this process writes and reads. It is the one
	// isolation dimension here: a kind belongs to the type, so there is no
	// prefix or name mapping to configure.
	Namespace string `toml:"namespace" help:"Namespace scopes every key this process writes and reads"`
	// Endpoint overrides the Datastore host, which is how the emulator is
	// reached. Empty falls back to DATASTORE_EMULATOR_HOST and then the
	// service. A value with no scheme is taken as http.
	Endpoint string `toml:"endpoint" help:"Endpoint overrides the Datastore host, which is how the emulator is reached"`
	// Credentials names the token source: service_account, metadata, oauth2 or
	// static.
	Credentials string `toml:"credentials" default:"service_account" help:"token source: service_account, metadata, oauth2 or static"`
	// CredentialsFile is the service account key. Empty falls back to
	// GOOGLE_APPLICATION_CREDENTIALS. Only the two signing sources read it.
	CredentialsFile string `toml:"credentials_file" help:"CredentialsFile is the service account key. Empty falls back to GOOGLE_APPLICATION_CREDENTIALS"`
	// Timeout bounds one request.
	Timeout time.Duration `toml:"timeout" default:"10s" help:"Timeout bounds one request"`
	// MaxIdleConns sizes the connection pool. The rule of thumb is the
	// expected concurrency.
	MaxIdleConns int `toml:"max_idle_conns" default:"4" help:"MaxIdleConns sizes the connection pool. The rule of thumb is the expected concurrency"`
}

// DefaultConfig is the binding's zero-value replacement, and the same values
// the default struct tags carry.
//
// Both exist because they answer different questions: configbind reads the tags
// to fill an unset key, and a caller building a Config in Go reads this.
func DefaultConfig() Config {
	return Config{
		Credentials:  CredentialsServiceAccount,
		Timeout:      10 * time.Second,
		MaxIdleConns: 4,
	}
}

// validate reports the configuration problems that can be seen without a
// network.
func (config Config) validate() error {
	if !config.Enabled {
		return nil
	}
	if config.Timeout <= 0 {
		return fmt.Errorf("middleware.firestore.timeout must be positive, got %s", config.Timeout)
	}
	if config.MaxIdleConns < 0 {
		return fmt.Errorf("middleware.firestore.max_idle_conns cannot be negative, got %d", config.MaxIdleConns)
	}
	switch config.Credentials {
	case "", CredentialsServiceAccount, CredentialsMetadata, CredentialsOAuth2, CredentialsStatic:
	default:
		return fmt.Errorf(
			"middleware.firestore.credentials is %q; it must be one of %s, %s, %s or %s",
			config.Credentials, CredentialsServiceAccount, CredentialsMetadata,
			CredentialsOAuth2, CredentialsStatic)
	}
	// A key file named for a source that never opens one is a deployment that
	// believes it configured something. The path itself is not a secret and is
	// reported; the file's contents never are.
	if strings.TrimSpace(config.CredentialsFile) != "" {
		switch config.Credentials {
		case CredentialsMetadata, CredentialsStatic:
			return fmt.Errorf(
				"middleware.firestore.credentials_file is set with credentials = %q, which reads no key file",
				config.Credentials)
		}
	}
	return nil
}

// credentials returns the configured source, filling in the default.
func (config Config) credentials() string {
	if config.Credentials == "" {
		return CredentialsServiceAccount
	}
	return config.Credentials
}
