package authfast

import (
	"fmt"

	"github.com/shibukawa/popcornweb/plugin/auth"
	"github.com/shibukawa/popcornweb/pwfast"
	"github.com/shibukawa/popcornweb/pwruntime"
	"github.com/shibukawa/popcornweb/session"
	"github.com/shibukawa/tinygodriver/fasthttp"
)

// Exchange carries the authentication endpoints over this transport.
//
// It is the whole of what this package contributes to a login: every rule about
// what an endpoint does — when a transaction cookie is consumed, which failures
// answer 403 rather than 400, what a step-up has to match — is in plugin/auth
// and is reached through auth.Exchange. Two implementations of a login would be
// two chances to leave a hole in one of them.
func Exchange(r *fasthttp.RequestCtx) auth.Exchange { return newExchange(r) }

type exchange struct {
	// Carrier supplies the three session operations, so the cookie translation
	// exists once for the session and the login both.
	session.Carrier
	r *fasthttp.RequestCtx
}

func newExchange(r *fasthttp.RequestCtx) *exchange {
	return &exchange{Carrier: pwfast.NewCarrier(r), r: r}
}

func (x *exchange) Method() string { return string(x.r.Method()) }
func (x *exchange) Path() string   { return string(x.r.Path()) }

// RawPath is the path before this transport normalized and decoded it, which is
// where an encoded separator is still visible.
//
// Not RequestURI, which is what a first cut used: that carries the query string
// too, so a %2F anywhere in a query value would have refused a request whose
// path was ordinary.
func (x *exchange) RawPath() string { return string(x.r.URI().PathOriginal()) }

func (x *exchange) Target() string { return string(x.r.RequestURI()) }

func (x *exchange) Query(name string) string { return string(x.r.QueryArgs().Peek(name)) }

// FormValue reads the submitted body only, never the query.
//
// This transport's own FormValue falls back through the query string and the
// multipart form; net/http's PostFormValue, which this mirrors, does neither. A
// logout scope that could be set from the query would be settable by a link.
func (x *exchange) FormValue(name string) string { return string(x.r.PostArgs().Peek(name)) }

func (x *exchange) Header(name string) string {
	return string(x.r.Request.Header.Peek(name))
}

func (x *exchange) HeaderValues(name string) []string {
	raw := x.r.Request.Header.PeekAll(name)
	values := make([]string, 0, len(raw))
	for _, value := range raw {
		values = append(values, string(value))
	}
	return values
}

// Body returns the request body, refusing one longer than limit.
//
// The check is on what arrived rather than on a bounded reader, because this
// transport has already read the body by the time a handler runs: the server's
// own MaxRequestBodySize is what stops an oversized one from being read at all,
// and this is the per-endpoint expression of the same limit.
func (x *exchange) Body(limit int64) ([]byte, error) {
	body := x.r.Request.Body()
	if limit > 0 && int64(len(body)) > limit {
		return nil, fmt.Errorf("request body is %d bytes, over the %d byte limit", len(body), limit)
	}
	return body, nil
}

func (x *exchange) IsTLS() bool { return x.r.IsTLS() }

// RemoteAddress is the parsed peer address.
//
// RemoteIP rather than RemoteAddr: this transport's RemoteAddr is whatever
// net.Addr the listener produced, which for a non-TCP listener is a name rather
// than an address, and an unparseable peer is never trusted by anything that
// reads this.
func (x *exchange) RemoteAddress() string { return x.r.RemoteIP().String() }

func (x *exchange) Host() string { return string(x.r.Host()) }

func (x *exchange) SetHeader(name, value string) { x.r.Response.Header.Set(name, value) }

func (x *exchange) Write(status int, body []byte) {
	x.r.SetStatusCode(status)
	if len(body) > 0 {
		_, _ = x.r.Write(body)
	}
}

func (x *exchange) Problem(err error) { pwfast.WriteProblem(x.r, err) }

func (x *exchange) Redirect(location string, status int) { pwfast.Redirect(x.r, location, status) }

// RecordAuthentication writes the result into the request value, which is what
// this transport has instead of a derived context. Every reader downstream is
// unchanged, because that value answers Value from the store this writes to.
func (x *exchange) RecordAuthentication(authentication pwruntime.Authentication) {
	pwruntime.StoreAuthentication(x.r, authentication)
}

func (x *exchange) AttachSession(resolved session.Resolved) { resolved.StoreOn(x.r) }
