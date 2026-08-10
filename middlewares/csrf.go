package middlewares

import (
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/shibukawa/popcornwave/internal/pathpattern"
	"github.com/shibukawa/popcornwave/internal/requestorigin"
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
	Enabled   bool     `default:"false"`
	Include   []string `default:"[\"/**\"]" dependon:".enabled"`
	Exclude   []string `env:"-" dependon:".enabled"`
	FormField string   `default:"_csrf" dependon:".enabled"`
	Header    string   `default:"X-CSRF-Token" dependon:".enabled"`
	// CookieName is the companion cookie carrying the masked token the browser
	// runtime reads. It is never HttpOnly, because the runtime has to read it.
	//
	// It belongs here rather than to the session cookie policy: the secret is a
	// registered session slot like any other, and writing this cookie is the
	// check's own job rather than something the session does on its behalf.
	CookieName     string   `default:"pw_csrf" dependon:".enabled" help:"companion cookie carrying the token the browser runtime reads"`
	TrustedOrigins []string `env:"-" dependon:".enabled"`
	// TTL bounds the companion cookie the runtime reads. The secret itself is
	// bounded by the session slot that holds it.
	TTL time.Duration `default:"12h" dependon:".enabled" help:"lifetime of the companion token cookie"`
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
//
// trustedProxies are the peer networks whose X-Forwarded-Proto is read when
// reconstructing this deployment's own origin. Without them a deployment whose
// TLS terminates upstream reconstructs an http origin for an https browser and
// refuses every unsafe request. The declared TrustedOrigins remain the stronger
// half of the comparison either way.
func CSRF(config CSRFConfig, cookie session.CookieOptions, sameSite http.SameSite, reject CSRFRejection, trustedProxies []*net.IPNet) (Middleware, error) {
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
	trusted := requestorigin.Set(config.TrustedOrigins...)
	proxies := requestorigin.FromNetworks(trustedProxies)
	ttl := config.TTL
	if ttl <= 0 {
		ttl = 12 * time.Hour
	}
	runtimeCookie := cookie
	runtimeCookie.Name = config.CookieName
	if runtimeCookie.Name == "" {
		runtimeCookie.Name = pwruntime.CSRFCookieName
	}
	secrets := &csrfSecret{cookie: runtimeCookie, sameSite: sameSite, ttl: ttl}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !config.Enabled {
				next.ServeHTTP(w, r)
				return
			}
			if safeMethod(r.Method) {
				// Only an HTML response needs a token to render an unsafe form.
				// API reads and asset requests stay session-free.
				if csrfHTMLRequest(r) {
					r = secrets.ensure(w, r)
				}
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
			if err := checkOrigin(proxies, r, trusted); err != nil {
				reject(w, r, err)
				return
			}
			r = secrets.ensure(w, r)
			secret, ok := pwruntime.CSRFSecret(r.Context())
			if !ok {
				// Nothing issued a secret, so there is nothing this request
				// could present that would be valid.
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

// csrfHTMLRequest reports whether a safe request is expected to render HTML.
// Browsers send either an HTML Accept value or a document navigation target.
// A generic */* request does not justify allocating session state merely in
// case the handler might render a form.
func csrfHTMLRequest(r *http.Request) bool {
	if strings.EqualFold(strings.TrimSpace(r.Header.Get("Sec-Fetch-Dest")), "document") {
		return true
	}
	for remainder := r.Header.Get("Accept"); remainder != ""; {
		var mediaRange string
		mediaRange, remainder, _ = strings.Cut(remainder, ",")
		mediaType, _, _ := strings.Cut(mediaRange, ";")
		if strings.EqualFold(strings.TrimSpace(mediaType), "text/html") {
			return true
		}
	}
	return false
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
	return pathpattern.Protected(include, exclude, path)
}

// checkOrigin refuses a request whose origin is neither this host nor one the
// deployment named.
//
// The comparison itself is requestorigin.Matches, shared with the
// authentication endpoints so the two cannot drift; this keeps only the error
// this middleware answers with.
func checkOrigin(proxies requestorigin.Proxies, r *http.Request, trusted map[string]bool) error {
	if proxies.Matches(r, trusted) {
		return nil
	}
	return errCSRFOrigin
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
