package middlewares

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/shibukawa/popcornwave/internal/pathpattern"
	"github.com/shibukawa/popcornwave/pwruntime"
	"github.com/shibukawa/popcornwave/session"
	"github.com/shibukawa/tinybind-go/htmlupdate"
)

// CSRFConfig controls the synchronizer-token check on unsafe browser requests.
//
// Enabled is false by default. A project turns it on together with the include
// patterns that say what it covers, because a middleware installed over nothing
// reads as protection that is not there.
type CSRFConfig struct {
	Enabled        bool     `default:"false"`
	Include        []string `default:"[\"/**\"]" dependon:".enabled"`
	Exclude        []string `env:"-" dependon:".enabled"`
	FormField      string   `default:"_csrf" dependon:".enabled"`
	Header         string   `default:"X-CSRF-Token" dependon:".enabled"`
	TrustedOrigins []string `env:"-" dependon:".enabled"`
	// Anonymous issues a secret to a visitor with no session, so a public page
	// may carry an unsafe form.
	Anonymous AnonymousCSRFConfig `dependon:".enabled"`
}

// DefaultCSRF returns the shipped defaults: off, and covering everything once
// turned on.
func DefaultCSRF() CSRFConfig {
	return CSRFConfig{
		Enabled:   false,
		Include:   []string{"/**"},
		FormField: htmlupdate.DefaultCSRFFieldName,
		Header:    htmlupdate.DefaultCSRFHeaderName,
	}
}

// CSRFRejection writes the response for a request the check refused.
type CSRFRejection func(w http.ResponseWriter, r *http.Request, reason error)

// CSRF validates a session-bound token and the request origin on unsafe
// requests matching the configured paths.
//
// A safe method is never checked: policy:csrf-protection covers what changes
// state, and a GET that changes state is a defect the token would only hide.
func CSRF(config CSRFConfig, cookie session.CookieOptions, sameSite http.SameSite, reject CSRFRejection) (Middleware, error) {
	if reject == nil {
		reject = writeCSRFStatus
	}
	include, err := pathpattern.Compile(config.Include)
	if err != nil {
		return nil, err
	}
	exclude, err := pathpattern.Compile(config.Exclude)
	if err != nil {
		return nil, err
	}
	// The module owns both halves of the token's name: generation writes the
	// field and this reads it, so taking the reader from there is what keeps
	// the two from disagreeing.
	options := htmlupdate.Options{CSRFFieldName: config.FormField, CSRFHeaderName: config.Header}
	trusted := make(map[string]bool, len(config.TrustedOrigins))
	for _, origin := range config.TrustedOrigins {
		trusted[origin] = true
	}
	var anonymous *anonymousCSRF
	if config.Enabled && config.Anonymous.Enabled {
		anonymous, err = newAnonymousCSRF(config.Anonymous, cookie, sameSite)
		if err != nil {
			return nil, err
		}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !config.Enabled {
				next.ServeHTTP(w, r)
				return
			}
			// Issuance runs before the method check, because a GET is what
			// renders the form the token goes into.
			if anonymous != nil {
				r = anonymous.ensure(w, r)
			}
			if safeMethod(r.Method) {
				next.ServeHTTP(w, r)
				return
			}
			path, ok := pathpattern.CanonicalPath(r)
			if !ok {
				// A path that cannot be matched unambiguously could select a
				// different routed target than the one this decided about.
				reject(w, r, errCSRFPath)
				return
			}
			if !protectedPath(include, exclude, path) {
				next.ServeHTTP(w, r)
				return
			}
			if err := checkOrigin(r, trusted); err != nil {
				reject(w, r, err)
				return
			}
			secret, ok := pwruntime.CSRFSecret(r.Context())
			if !ok {
				// No session means no secret to have signed anything, so there
				// is nothing this request could present that would be valid.
				reject(w, r, errCSRFNoSession)
				return
			}
			// The presented token carries its own pad, so the value to compare
			// against is rebuilt from it rather than being the stored secret.
			presented := options.CSRFToken(r)
			if err := options.VerifyCSRF(r, pwruntime.ExpectedCSRFToken(secret, presented)); err != nil {
				reject(w, r, err)
				return
			}
			next.ServeHTTP(w, r)
		})
	}, nil
}

func safeMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	default:
		return false
	}
}

func protectedPath(include, exclude []pathpattern.Pattern, path string) bool {
	if pathpattern.MatchAny(exclude, path) {
		return false
	}
	return pathpattern.MatchAny(include, path)
}

// checkOrigin refuses a request whose origin is neither this host nor one the
// deployment named.
//
// Origin is preferred because a browser sets it on exactly the requests this
// protects. Referer is the fallback for the case where a proxy stripped it, and
// it is read strictly: a missing one is a refusal rather than a pass, since
// treating absence as trust would make the whole check optional.
func checkOrigin(r *http.Request, trusted map[string]bool) error {
	host := requestOrigin(r)
	if origin := r.Header.Get("Origin"); origin != "" && origin != "null" {
		if origin == host || trusted[origin] {
			return nil
		}
		return errCSRFOrigin
	}
	referer := r.Header.Get("Referer")
	if referer == "" {
		return errCSRFOrigin
	}
	parsed, err := url.Parse(referer)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return errCSRFOrigin
	}
	origin := parsed.Scheme + "://" + parsed.Host
	if origin == host || trusted[origin] {
		return nil
	}
	return errCSRFOrigin
}

// requestOrigin reconstructs this request's own origin.
//
// A deployment behind a proxy that rewrites the scheme has to name its origin
// in TrustedOrigins, because nothing here trusts a forwarded header: doing so
// would let a caller assert the value the check compares against.
func requestOrigin(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	host := r.Host
	if host == "" {
		return ""
	}
	return scheme + "://" + strings.TrimSuffix(host, ":")
}

func writeCSRFStatus(w http.ResponseWriter, r *http.Request, reason error) {
	pwruntime.ReadLogger(r.Context()).Log(r.Context(), pwruntime.LevelWarn, "CSRF check refused a request",
		pwruntime.String("reason", reason.Error()), pwruntime.String("method", r.Method))
	if Committed(w) {
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
}

// The refusal reasons. They are distinct so a log line says which check failed
// without the response ever saying so: policy:csrf-protection keeps the
// diagnosis server-side, since a precise refusal tells an attacker which half
// to work on.
var (
	errCSRFPath      = csrfError("request path cannot be matched unambiguously")
	errCSRFNoSession = csrfError("no session, so no token could be valid")
	errCSRFOrigin    = csrfError("origin is neither this host nor a trusted one")
)

type csrfError string

func (e csrfError) Error() string { return "csrf: " + string(e) }
