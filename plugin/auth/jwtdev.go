//go:build !pwdev

package auth

import (
	"errors"
	"net/http"
)

// devRelaxationBuilt reports whether this binary contains the development
// relaxation. It is the build lock of policy:dev-token-relaxation, and it is the
// only one of the four that is structural: in this build the relaxed verifier is
// not code that fails to run, it is code that is not here.
const devRelaxationBuilt = false

// checkDevRelaxation refuses a configuration that asks for a relaxation this
// binary cannot perform.
//
// It fails rather than ignoring the field. configbind would otherwise bind a
// setting nothing reads, and a security setting that is silently dropped reads
// as configured security: an operator who wrote it down would believe the
// development shortcut was available, and one who left it in by accident would
// never learn that a production binary was carrying it.
func checkDevRelaxation(config JWTConfig) error {
	if config.Dev.TrustUnverifiedTokens {
		return errors.New("auth.jwt.dev.trust_unverified_tokens requires the pwdev build mode, which this binary was not built with; remove the setting or build with -tags pwdev under pw dev")
	}
	return nil
}

// devAdmits is the relaxed path. In this build there is none, so every caller
// falls through to ordinary verification.
func (v *bearerVerifier) devAdmits(*http.Request) (Identity, bool) {
	return Identity{}, false
}

// markDevResponse is a no-op here, because no response in this build was
// admitted without verification.
func markDevResponse(http.ResponseWriter) {}
