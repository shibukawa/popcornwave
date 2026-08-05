package passkeye2e

import (
	"os"
	"testing"
)

// The fixture starts the development identity provider, which refuses to build
// in a named non-development environment. An unset APP_ENV passes on its own;
// this only keeps a developer's own APP_ENV=stg out of the suite.
func TestMain(m *testing.M) {
	if _, set := os.LookupEnv("APP_ENV"); !set {
		os.Setenv("APP_ENV", "dev")
	}
	os.Exit(m.Run())
}
