//go:build pwdev

package auth

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/shibukawa/popcornwave/contrib/jwt"
	"github.com/shibukawa/popcornwave/pw"
)

// devRelaxationBuilt reports whether this binary contains the development
// relaxation. See policy:dev-token-relaxation for the four locks; this file is
// present only when the first one is open.
const devRelaxationBuilt = true

// DevUnverifiedHeader marks a response whose request was admitted without
// verification.
//
// A warning in a log is read after the fact by whoever thought to look. This is
// visible to the client that made the call and to every proxy between, which is
// where someone notices that a deployment they believed was verifying is not.
const DevUnverifiedHeader = "X-Pw-Auth-Unverified"

// checkDevRelaxation validates the environment lock at startup.
//
// The build lock is this file existing. The configuration lock is the field
// itself. This is the third: a relaxation is refused outright anywhere but a
// declared development environment, matching policy:devidp-safety, because the
// pwdev binary is exactly the one that could reach a deployment by accident.
//
// It is an allowlist. It used to name the three environments it refused —
// "stg", "prod", "production" — which meant that "staging", "prd", "live",
// "uat", "canary", and every other spelling a deployment might use walked past
// a lock built to stop exactly them. A list of the environments that must not
// open this can never be complete; a list of the one that may is complete by
// construction.
//
// An unset APP_ENV passes, because it resolves to development like everywhere
// else in the framework. What no longer passes is a deployment that named an
// environment and named something other than development.
func checkDevRelaxation(config JWTConfig) error {
	if !config.Dev.TrustUnverifiedTokens {
		return nil
	}
	if !pw.Development() {
		return fmt.Errorf("auth.jwt.dev.trust_unverified_tokens needs %s=%q; it is %q here, and the setting turns token verification off",
			pw.EnvVar, pw.EnvDevelopment, pw.Env())
	}
	return nil
}

// devAdmits takes the relaxed path when every remaining lock is open.
//
// It never calls the verifier, and the verifier never gains a branch for
// alg none. That is the point of the separation: an accepted "none" would be a
// hole in the production code path that a configuration mistake could reach,
// and a path behind a build tag is not.
//
// What is relaxed is one switch rather than a menu: signature, issuer,
// audience, token type, times, and the algorithm allowlist all go together,
// because a per-check menu is a thing to get half right. What is not relaxed is
// the identity claim, admission, revocation, and the parser bounds — a
// developer exercises the real organization rule and the real revocation path,
// and a decoder is still a decoder.
func (v *bearerVerifier) devAdmits(r *http.Request) (Identity, bool) {
	if !v.config.Dev.TrustUnverifiedTokens {
		return Identity{}, false
	}
	if !loopbackRequest(r) {
		// No opt-out, matching the listen rule of policy:devidp-safety. A device
		// on the network that needs a signed token has requirement:contrib-devidp.
		return Identity{}, false
	}
	compact, err := v.bearerCredential(r)
	if err != nil {
		return Identity{}, false
	}
	// Still a compact JWT with a decodable claim set, and still bounded.
	// Relaxation removes checks; it does not add tolerance for malformed input.
	token, err := jwt.Parse(fillEmptySignature(compact),
		jwt.ParseOptions{MaxTokenBytes: v.config.MaxTokenBytes})
	if err != nil {
		return Identity{}, false
	}
	identity := identityFrom(token.Claims, v.config.IdentityClaim)
	if identity.Key == "" {
		return Identity{}, false
	}
	identity.TokenID = token.Claims.ID
	// A hand-written token usually carries no iat, and the subject form of
	// policy:token-revocation compares against one. Treating a missing iat as
	// now keeps that comparison meaningful rather than skipping it.
	identity.IssuedAt = v.now().UTC()
	if token.Claims.IssuedAt != nil {
		identity.IssuedAt = time.Unix(*token.Claims.IssuedAt, 0).UTC()
	}
	if token.Claims.ExpiresAt != nil {
		identity.ExpiresAt = time.Unix(*token.Claims.ExpiresAt, 0).UTC()
	}
	pw.Logger(r.Context()).Log(r.Context(), pw.LevelWarn,
		"bearer token admitted without verification",
		pw.String("subject", identity.Key),
		pw.String("setting", "auth.jwt.dev.trust_unverified_tokens"))
	return identity, true
}

// markDevResponse tells the client that the identity behind this response was
// never verified.
func markDevResponse(w http.ResponseWriter) {
	w.Header().Set(DevUnverifiedHeader, "true")
}

// fillEmptySignature gives a signature-less token a placeholder signature so
// the ordinary parser will read it.
//
// An alg none token is conventionally written with an empty third segment, and
// jwt.Parse refuses that — correctly, because a token with no signature is not
// one the verifier should ever look at. Rather than weaken the parser or write a
// second one, the relaxed path substitutes a byte the parser accepts and then
// ignores what it decoded to. Nothing about this reaches a build without the tag.
func fillEmptySignature(compact string) string {
	if strings.HasSuffix(compact, ".") && strings.Count(compact, ".") == 2 {
		return compact + "AA"
	}
	return compact
}

// loopbackRequest reports whether the request arrived directly from a loopback
// address.
//
// It reads RemoteAddr only. A forwarded header is a claim made by whoever sent
// the request, and this is the one check where believing that claim would hand
// the relaxation to the network.
//
// RemoteAddr alone is not enough, though, and that is the second half of this.
// Put nginx, Caddy, or `docker run -p` on the same host and every request in the
// world arrives from 127.0.0.1: the address stops describing the client and
// starts describing the proxy, turning a local-only relaxation into an
// internet-facing one without anything in the configuration changing.
//
// Nothing at the network layer distinguishes those two cases, so this uses the
// one artifact a proxy leaves behind: a forwarding header. Their presence is
// read as "something relayed this", never as an address to trust. The direction
// is safe — an attacker who adds one is refused, and an attacker who strips one
// still has to get past a proxy that adds it back.
func loopbackRequest(r *http.Request) bool {
	if relayed(r) {
		return false
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	address := net.ParseIP(host)
	return address != nil && address.IsLoopback()
}

// forwardingHeaders are the headers a reverse proxy adds. Any of them means
// RemoteAddr describes a relay rather than the caller.
var forwardingHeaders = []string{
	"Forwarded",
	"X-Forwarded-For",
	"X-Forwarded-Host",
	"X-Forwarded-Proto",
	"X-Real-Ip",
}

func relayed(r *http.Request) bool {
	for _, name := range forwardingHeaders {
		if r.Header.Get(name) != "" {
			return true
		}
	}
	return false
}
