package pw

import "testing"

// Every registered key that names a secret must be redacted, and every key that
// only describes one must not: an operator who sees [REDACTED] on a duration
// learns to read past it.
func TestSecretKeyRedaction(t *testing.T) {
	for key, want := range map[string]bool{
		// Secrets.
		"auth.oidc.client_secret": true,
		"middleware.rdb.dsn":      true,
		"session.rdb.dsn":         true,
		"idp.private_key":         true,
		"some.password":           true,
		"provider.refresh_token":  true,
		"issued.credential":       true,
		// Settings that merely describe a secret.
		"auth.bootstrap.issue_ttl":      false,
		"auth.bootstrap.enrollment_ttl": false,
		"auth.bootstrap.max_attempts":   false,
		"idp.token_ttl":                 false,
		"auth.oidc.client_id":           false,
		"auth.passkey.rp_id":            false,
		"server.port":                   false,
		// A path to a secret is a path, not the secret.
		"idp.private_key_path": false,
	} {
		if got := isSecretKey(key); got != want {
			t.Errorf("isSecretKey(%q) = %v, want %v", key, got, want)
		}
	}
}

// Case is not part of the decision.
func TestSecretKeyRedactionIgnoresCase(t *testing.T) {
	if !isSecretKey("AUTH.OIDC.CLIENT_SECRET") {
		t.Error("an uppercase secret key was not redacted")
	}
}
