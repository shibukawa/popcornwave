package middlewares

import (
	"net/http"
	"time"

	"github.com/shibukawa/popcornweb/pwruntime"
	"github.com/shibukawa/popcornweb/session"
)

// CSRFSecret is the per-session secret the check validates against, held in a
// registered session slot like any other piece of per-browser state.
//
// One slot serves both populations. A visitor with no login gets the secret in
// the sealed cookie a session.Private slot rides while it is anonymous, so no
// server record is written for a crawler that merely loads a page with a form.
// The login rotation moves the same slot onto the configured backend and mints
// a fresh secret with it, which is what stops a token minted before a sign-in
// from being presented after one.
//
// The alternative was a signed cookie for anonymous visitors beside a record
// field for authenticated ones. That split existed to avoid minting a stored
// session per anonymous visitor; a private slot costs no server row while
// anonymous, so the reason for two mechanisms went away.
type CSRFSecret struct {
	Secret string `json:"secret"`
}

// CSRFSecretSlot is the registration key of that slot. The framework declares
// it only where the check is turned on, so a project with CSRF off carries no
// slot and needs no keyring on its account.
const CSRFSecretSlot = "pw_csrf_secret"

// csrfSecrets issues and reads the secret, and keeps the companion cookie the
// browser runtime reads in step with it.
type csrfSecret struct {
	cookie   session.CookieOptions
	sameSite http.SameSite
	ttl      time.Duration
}

// ensure returns the request carrying a CSRF secret, minting one when the
// browser has none.
//
// It runs for protected unsafe requests and for safe requests that negotiate
// HTML, because those are the requests that validate or render a form token.
func (c *csrfSecret) ensure(w http.ResponseWriter, r *http.Request) *http.Request {
	handle, ok := session.Value[CSRFSecret](r.Context())
	if !ok {
		// No session middleware, or the slot was not declared. The check that
		// follows refuses rather than passing, which is the safe direction.
		return r
	}
	held, present := handle.Get()
	minted := false
	if !present || held.Secret == "" {
		secret, err := pwruntime.NewCSRFSecret(nil)
		if err != nil {
			return r
		}
		if err := handle.Set(CSRFSecret{Secret: secret}); err != nil {
			// An oversized or unwritable slot leaves the request without a
			// secret, and the check refuses it.
			return r
		}
		held, minted = CSRFSecret{Secret: secret}, true
	}
	// The runtime reads its token from an ordinary cookie, so a newly minted
	// secret needs one written beside it. A lost token cookie is rewritten too,
	// which is what keeps the pair self-healing after a rotation.
	if minted || !hasCookie(r, c.cookie.Name) {
		c.writeRuntimeCookie(w, held.Secret)
	}
	return r.WithContext(pwruntime.WithCSRFSecret(r.Context(), held.Secret))
}

// writeRuntimeCookie hands the browser runtime a masked token.
//
// The value is masked like every other emission: the cookie is not in a
// compressed body, so it is not what a compression oracle reads, but sending
// the bare secret would put the thing verification compares against into a
// place script can read.
func (c *csrfSecret) writeRuntimeCookie(w http.ResponseWriter, secret string) {
	if c.cookie.Name == "" || secret == "" {
		return
	}
	token, err := pwruntime.CSRFToken(secret, nil)
	if err != nil || token == "" {
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:   c.cookie.Name,
		Value:  token,
		Path:   c.cookie.Path,
		Domain: c.cookie.Domain,
		MaxAge: int(c.ttl.Seconds()),
		Secure: c.cookie.Secure,
		// Never HttpOnly: the runtime reads this one.
		HttpOnly: false,
		SameSite: c.sameSite,
	})
}

func hasCookie(r *http.Request, name string) bool {
	cookie, err := r.Cookie(name)
	return err == nil && cookie != nil && cookie.Value != ""
}
