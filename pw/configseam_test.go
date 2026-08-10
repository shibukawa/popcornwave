package pw

import (
	"testing"

	"github.com/shibukawa/popcornwave/pwconfig"
)

// The framework's configuration and its resolved environment are process state
// owned by pwconfig, so a test that needs one of them to differ says so through
// that package's seam and restores it when it finishes.
//
// They were direct writes into a package map while the registry lived here.
// Moving the registry out is what turned that into an API question, and the
// answer is two helpers rather than an exported map.

// swapEnvForTest resolves the environment to value for the rest of the test.
func swapEnvForTest(t *testing.T, value string, declared bool) {
	t.Helper()
	t.Cleanup(pwconfig.SwapEnv(value, declared))
}

// swapConfigForTest installs one configuration binding for the rest of the test
// and returns the pointer it installed, so a test can adjust a field partway
// through without reinstalling the whole value.
func swapConfigForTest[T any](t *testing.T, value T) *T {
	t.Helper()
	installed, restore := pwconfig.Swap(value)
	t.Cleanup(restore)
	return installed
}
