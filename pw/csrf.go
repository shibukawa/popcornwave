package pw

import (
	"context"
	"errors"
	"net/http"

	"github.com/shibukawa/popcornwave/middlewares"
	"github.com/shibukawa/popcornwave/pwruntime"
	"github.com/shibukawa/popcornwave/session"
)

func init() {
	RegisterExtension(Extension{
		Name:  "security.csrf",
		Slot:  SlotCSRF,
		Setup: setupCSRF,
	})
}

// setupCSRF installs the CSRF check when a project turned it on.
//
// It is an extension rather than a member of the framework middleware stack
// because that stack wraps the extension chain from outside, where the session
// has not been resolved yet and the token would have nothing to compare against.
func setupCSRF(ctx context.Context) (Middleware, error) {
	config := Config[SecurityConfig](ctx)
	if !config.CSRF.Enabled {
		return nil, nil
	}
	sessionConfig := Config[SessionConfig](ctx)
	sameSite, err := session.ParseSameSite(sessionConfig.Cookie.SameSite)
	if err != nil {
		return nil, err
	}
	// The origin this check compares against is reconstructed from the
	// request, and a deployment whose TLS terminates upstream reconstructs
	// http for an https browser unless the proxy set says otherwise.
	trusted, err := compileTrustedProxies(Config[ServerConfig](ctx).TrustedProxies)
	if err != nil {
		return nil, err
	}
	// The anonymous cookie follows the session cookie's own policy, so one
	// deployment decision covers both rather than two that can disagree.
	return middlewares.CSRF(config.CSRF, session.CookieOptions{
		Path:   sessionConfig.Cookie.Path,
		Domain: sessionConfig.Cookie.Domain,
		Secure: sessionConfig.Cookie.Secure,
	}, sameSite, writeCSRFProblem, trusted)
}

// writeCSRFProblem answers a refused request through the framework error path,
// so a browser gets the application's HTML error page and an API client gets a
// problem document, exactly as any other 403 does.
//
// The reason reaches the log and never the response. A refusal that named which
// check failed would tell a caller which half to work on next.
func writeCSRFProblem(w http.ResponseWriter, r *http.Request, reason error) {
	Logger(r.Context()).Log(r.Context(), LevelWarn, "CSRF check refused a request",
		String("reason", reason.Error()), String("method", r.Method), String("path", r.URL.Path))
	if responseCommitted(w) {
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	WriteProblem(w, r, Forbidden(errCSRFRefused))
}

var errCSRFRefused = errors.New("the request could not be verified")

// CSRFCookieName is the companion cookie the browser runtime reads a token from.
const CSRFCookieName = pwruntime.CSRFCookieName
