package middlewares

import (
	"net/http"
	"time"

	"github.com/shibukawa/popcornwave/pwruntime"
	"github.com/shibukawa/popcornwave/session"
)

// AnonymousCSRFConfig turns on a CSRF secret for visitors with no session.
//
// It is off by default, because policy:csrf-protection requires a validated
// session on a protected unsafe request and most deployments have no unsafe
// form on a public page. A project that serves one — a contact post, a search
// post — turns this on rather than excluding the path and relying on the origin
// check alone.
type AnonymousCSRFConfig struct {
	Enabled bool `default:"false"`
	// Secret signs the cookie carrying the secret. It is required when Enabled,
	// because an unsigned one is the naive double-submit shape: anyone who can
	// set a cookie could then satisfy the check.
	Secret string `secret:"mask" env:"SECURITY_CSRF_ANONYMOUS_SECRET" help:"base64 secret signing the anonymous CSRF cookie" dependon:".enabled"`
	// PreviousSecrets keep cookies written before a rotation readable.
	PreviousSecrets []string `secret:"mask" help:"retired secrets kept readable during a rotation" dependon:".enabled"`
	// TTL bounds how long one anonymous secret lives.
	TTL time.Duration `default:"12h" dependon:".enabled"`
}

// anonymousCSRF issues and reads the secret for a visitor with no session.
//
// The secret is the cookie rather than a stored record. Minting a session for
// every anonymous visitor would let unauthenticated traffic decide how many
// rows the store holds: a crawler, not the user base, would set the count, and
// those rows expire rather than being deleted, because there is no logout. The
// signature is what makes storing nothing safe, since a value the server did
// not issue cannot be presented.
type anonymousCSRF struct {
	jar *session.Jar[string]
	// runtimeCookie mirrors the token onto the cookie the browser runtime
	// reads, so an anonymous page drives an update exactly as a session one
	// does. The jar cookie holds the secret; this one holds a masked token.
	runtimeCookie session.CookieOptions
	sameSite      http.SameSite
	ttl           time.Duration
}

func newAnonymousCSRF(config AnonymousCSRFConfig, cookie session.CookieOptions, sameSite http.SameSite) (*anonymousCSRF, error) {
	keys, err := session.ParseKeyring(append([]string{config.Secret}, config.PreviousSecrets...)...)
	if err != nil {
		return nil, err
	}
	ttl := config.TTL
	if ttl <= 0 {
		ttl = 12 * time.Hour
	}
	jar, err := session.NewJar[string](nil, session.JarOptions{
		Mode: session.CookieSigned,
		Keys: keys,
		Cookie: session.CookieOptions{
			Name: anonymousCookieName,
			Path: cookie.Path, Domain: cookie.Domain,
			Secure: cookie.Secure,
			// The secret itself is never read by script; only the masked token
			// beside it is.
			HTTPOnly: true,
			SameSite: sameSite,
		},
		MaxAge: ttl,
	})
	if err != nil {
		return nil, err
	}
	return &anonymousCSRF{jar: jar, runtimeCookie: cookie, sameSite: sameSite, ttl: ttl}, nil
}

// anonymousCookieName holds the signed secret. It is separate from the token
// cookie so each carries one thing: this one the server verifies against, that
// one the runtime sends.
const anonymousCookieName = "pw_csrf_anon"

// ensure returns the request carrying a CSRF secret, issuing one when the
// visitor has neither a session nor a usable cookie.
//
// It runs on every request rather than only unsafe ones, because a GET is what
// renders the form that needs the token.
func (a *anonymousCSRF) ensure(w http.ResponseWriter, r *http.Request) *http.Request {
	if _, ok := pwruntime.CSRFSecret(r.Context()); ok {
		return r
	}
	secret, err := a.jar.Load(r)
	minted := false
	if err != nil || secret == "" {
		// An absent, expired, or unsigned cookie is replaced rather than
		// refused: there is nothing to protect yet, and a visitor whose cookie
		// aged out would otherwise be stuck.
		secret, err = pwruntime.NewCSRFSecret(nil)
		if err != nil {
			return r
		}
		if err := a.jar.Save(w, secret); err != nil {
			return r
		}
		minted = true
	}
	// The runtime reads its token from the ordinary cookie, so a newly issued
	// secret needs one written beside it. A lost token cookie is rewritten too,
	// which is what keeps the pair self-healing.
	if minted || !hasCookie(r, pwruntime.CSRFCookieName) {
		a.writeRuntimeCookie(w, secret)
	}
	return r.WithContext(pwruntime.WithCSRFSecret(r.Context(), secret))
}

func (a *anonymousCSRF) writeRuntimeCookie(w http.ResponseWriter, secret string) {
	token, err := pwruntime.CSRFToken(secret, nil)
	if err != nil || token == "" {
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:   pwruntime.CSRFCookieName,
		Value:  token,
		Path:   a.runtimeCookie.Path,
		Domain: a.runtimeCookie.Domain,
		MaxAge: int(a.ttl.Seconds()),
		Secure: a.runtimeCookie.Secure,
		// Never HttpOnly: the runtime reads this one.
		HttpOnly: false,
		SameSite: a.sameSite,
	})
}

func hasCookie(r *http.Request, name string) bool {
	cookie, err := r.Cookie(name)
	return err == nil && cookie != nil && cookie.Value != ""
}
