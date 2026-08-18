package pw

import (
	"testing"

	"github.com/shibukawa/popcornweb/pwconfig"
)

// The relaxations that follow the answer: an insecure session cookie is a
// development-only exception, and query bind values are logged only there.
func TestTheRelaxationsFollowTheEnvironment(t *testing.T) {
	insecure := SessionConfig{Enabled: true, Cookie: SessionCookieConfig{SameSite: "lax"}}

	if err := validateSessionConfig(insecure, pwconfig.EnvDevelopment, true); err != nil {
		t.Errorf("development refused the loopback cookie exception: %v", err)
	}
	if err := validateSessionConfig(insecure, pwconfig.EnvStaging, false); err == nil {
		t.Error("staging started with a session cookie that has no Secure")
	}

	if resolveQueryDiagnostics(queryConfig(nil), true) == nil {
		t.Error("auto should log queries in development")
	}
	if resolveQueryDiagnostics(queryConfig(nil), false) != nil {
		t.Error("auto should stay off outside development")
	}
}
