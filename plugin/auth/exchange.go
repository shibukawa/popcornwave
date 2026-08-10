package auth

import (
	"net/http"
	"strings"

	"github.com/shibukawa/popcornwave/internal/requestorigin"
	"github.com/shibukawa/popcornwave/pwruntime"
	"github.com/shibukawa/popcornwave/session"
)

// An Exchange is everything the authentication endpoints need from the
// transport carrying one request.
//
// It exists for the reason session.Carrier does, one layer up. A session turned
// out to touch three operations, so an interface of three let a second transport
// carry one without a second copy of the rotation rules. The endpoints touch
// more than three — they read a query, take a form field, decode a body, expire
// a cookie, and answer with a redirect or a problem — but the same argument
// applies with more force, because what would be duplicated is a login: two
// implementations of when a transaction cookie is consumed, or of which failures
// answer 403 rather than 400, are two chances to leave a hole in one of them.
//
// So every rule below this line is written once against this interface, and each
// transport supplies the thirty-odd lines that read its own request value.
//
// The cookie and header currency is net/http's, exactly as Carrier's is, and for
// the same reason: http.Cookie is a data struct describing a cookie rather than
// implementing one, and a transport that spells cookies differently translates
// at its own edge.
type Exchange interface {
	// Cookies, SetCookie, and Context are the session's three, and a login is a
	// session, so the endpoints reach the manager through this value directly.
	session.Carrier

	// Method is the request method.
	Method() string
	// Path is the request path with percent-encoding decoded.
	Path() string
	// RawPath is the path as it arrived, before decoding. It is read only to
	// refuse an encoded separator, which is invisible in the decoded form.
	RawPath() string
	// Target is the path and query as sent, which is what a return path records
	// so a completed login lands back on the whole request.
	Target() string
	// Query returns one query parameter, or the empty string.
	Query(name string) string
	// FormValue returns one submitted form field, or the empty string.
	FormValue(name string) string
	// Header returns one request header, or the empty string.
	Header(name string) string
	// HeaderValues returns every value of one request header.
	//
	// It exists for Authorization alone, and only because a second one is
	// refused rather than merged: which of two a proxy forwards is not this
	// application's decision to guess, and a reader that saw only the first
	// could not tell there had been two.
	HeaderValues(name string) []string
	// Body reads the request body, refusing one longer than limit. The refusal
	// is an error rather than a truncation, so an oversized document is
	// answered as one rather than as malformed JSON.
	Body(limit int64) ([]byte, error)

	// IsTLS reports whether this hop arrived over TLS, which is the fact
	// internal/requestorigin outranks every forwarded header with.
	IsTLS() bool
	// RemoteAddress is the peer, which decides whether a forwarded header is
	// evidence of anything.
	RemoteAddress() string
	// Host is the authority the request named, which the origin is built from.
	Host() string

	// SetHeader sets one response header.
	SetHeader(name, value string)
	// Write commits a status and a body.
	Write(status int, body []byte)
	// Problem answers with the framework's problem document, rendered through
	// whatever error page this deployment registered.
	Problem(err error)
	// Redirect sends the browser elsewhere, through the framework's own helper
	// rather than the transport's: an update request is a fetch, so a 303 would
	// be followed and its target applied as a region set for the wrong page.
	Redirect(location string, status int)

	// RecordAuthentication publishes the verified authentication result, so
	// everything after this frame reads it from the request.
	RecordAuthentication(pwruntime.Authentication)
	// AttachSession publishes a session the endpoints resolved themselves,
	// which is what a caller with no session middleware above it needs.
	AttachSession(session.Resolved)
}

// scheme resolves the scheme the client actually used, from the three facts
// internal/requestorigin reads.
func (rt *runtime) scheme(x Exchange) string {
	return rt.proxies.SchemeOf(x.IsTLS(), x.RemoteAddress(), x.Header("X-Forwarded-Proto"))
}

// sameOrigin requires a whole origin, scheme included, matching this
// deployment's own or one it declared.
//
// The comparison is internal/requestorigin's, the same one the CSRF middleware
// makes. This package used to compare host names alone, which admitted an http
// caller to an https deployment and supported no declared origin at all.
func (rt *runtime) sameOrigin(x Exchange) bool {
	return requestorigin.MatchesOrigin(rt.proxies.OriginOf(x.Host(), rt.scheme(x)),
		x.Header("Origin"), x.Header("Referer"), rt.trustedOrigins)
}

// attachSession resolves the session of an exchange that has none, for the
// callers that legitimately have no session middleware above them.
//
// The middleware is the normal path and has already done this, in which case
// the manager returns what is attached and republishing it changes nothing.
func (rt *runtime) attachSession(x Exchange) error {
	resolved, err := rt.manager.AttachTo(x)
	if err != nil {
		return err
	}
	x.AttachSession(resolved)
	return nil
}

// allowMethod refuses a method the endpoint does not serve, naming the ones it
// does, and reports whether the request may continue.
//
// The Allow header is set before the problem is written, and on the second
// transport that ordering is the whole of what makes it survive: an error path
// that resets the response would take the header with it, which is why the
// problem writer is reached through the exchange rather than through the
// transport's own error helper.
func allowMethod(x Exchange, allowed ...string) bool {
	for _, method := range allowed {
		if x.Method() == method {
			return true
		}
	}
	x.SetHeader("Allow", strings.Join(allowed, ", "))
	x.Problem(pwruntime.Problem{
		Status: http.StatusMethodNotAllowed, Title: "Method Not Allowed", Code: "method_not_allowed",
	})
	return false
}

// requestCookie reads one request cookie by name, reporting whether it was
// present and non-empty.
//
// Names are compared exactly, as RFC 6265 requires: a cookie name is
// case-sensitive, and matching loosely would let a client set Session beside
// session and choose which one an endpoint reads.
func requestCookie(x Exchange, name string) (string, bool) {
	for _, candidate := range x.Cookies() {
		if candidate != nil && candidate.Name == name {
			return candidate.Value, candidate.Value != ""
		}
	}
	return "", false
}

// logger is the request logger. Every endpoint reaches it the same way on both
// transports, because the second transport's request value is its context.
func logger(x Exchange) pwruntime.Logger { return pwruntime.ReadLogger(x.Context()) }

// authenticated reports whether the exchange carries a verified identity.
func authenticated(x Exchange) bool {
	return pwruntime.RequestAuthentication(x.Context()).Authenticated
}
