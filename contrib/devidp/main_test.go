package devidp_test

import (
	"os"
	"testing"
)

// The provider refuses to build in a named non-development environment. An unset
// APP_ENV passes on its own, but a developer whose shell carries APP_ENV=stg
// would otherwise watch this suite fail for a reason that is not about the code.
// Tests that care about the lock itself override this with t.Setenv.
func TestMain(m *testing.M) {
	if _, set := os.LookupEnv("APP_ENV"); !set {
		os.Setenv("APP_ENV", "dev")
	}
	os.Exit(m.Run())
}
