package pw

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/shibukawa/popcornweb/middlewares"
	"github.com/shibukawa/popcornweb/pwruntime"
	"github.com/shibukawa/popcornweb/session"
	"github.com/shibukawa/tinybind-go/templates/htmlbind"
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
	config := ConfigContext[SecurityConfig](ctx)
	if !config.CSRF.Enabled {
		return nil, nil
	}
	if err := checkCSRFFieldName(config.CSRF.FormField); err != nil {
		return nil, err
	}
	sessionConfig := ConfigContext[SessionConfig](ctx)
	sameSite, err := session.ParseSameSite(sessionConfig.Cookie.SameSite)
	if err != nil {
		return nil, err
	}
	// The origin this check compares against is reconstructed from the
	// request, and a deployment whose TLS terminates upstream reconstructs
	// http for an https browser unless the proxy set says otherwise.
	trusted, err := compileTrustedProxies(ConfigContext[ServerConfig](ctx).TrustedProxies)
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

// generatedCSRFField is the hidden field every generated form carries.
//
// It is the template compiler's own default, and generation never overrides it:
// the option lives on the compiler and the route tree forwards neither it nor
// the mode, so a page tree emits this name whatever a project configured.
const generatedCSRFField = htmlbind.DefaultCSRFFieldName

// checkCSRFFieldName refuses a configured field name generated forms will not
// carry.
//
// Without this the failure is a 403 on every form submission, with the reason in
// the log only and nothing pointing at the setting that caused it — the check
// compares against a field the markup never wrote, so it is the request that
// looks wrong rather than the configuration. Refusing at startup names the
// setting instead.
//
// A project hand-writing every form and wanting another name has to write this
// one, which is the cost of the middleware reading one field for the whole
// application.
func checkCSRFFieldName(configured string) error {
	if configured == "" || configured == generatedCSRFField {
		return nil
	}
	return fmt.Errorf(
		"security.csrf.form_field is %q, and every generated form carries %q: "+
			"template generation does not take this setting, so the check would refuse every submission",
		configured, generatedCSRFField)
}

// writeCSRFProblem answers a refused request through the framework error path,
// so a browser gets the application's HTML error page and an API client gets a
// problem document, exactly as any other 403 does.
//
// The reason reaches the log and never the response. A refusal that named which
// check failed would tell a caller which half to work on next.
func writeCSRFProblem(w http.ResponseWriter, r *http.Request, reason error) {
	LoggerContext(r.Context()).Log(r.Context(), LevelWarn, "CSRF check refused a request",
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
