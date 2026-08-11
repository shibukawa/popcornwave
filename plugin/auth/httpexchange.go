package auth

import (
	"context"
	"io"
	"net/http"

	"github.com/shibukawa/popcornwave/pwextension"
	"github.com/shibukawa/popcornwave/pwruntime"
	"github.com/shibukawa/popcornwave/session"
)

// HTTPExchange carries the authentication endpoints over net/http.
//
// It is exported because the endpoints are reachable from outside this package
// — Hint takes a writer and a request, and an application that mounts its own
// login page calls it — and because a test that drives one endpoint directly
// needs the same value the middleware builds.
func HTTPExchange(w http.ResponseWriter, r *http.Request) Exchange {
	return newHTTPExchange(w, r)
}

func newHTTPExchange(w http.ResponseWriter, r *http.Request) *httpExchange {
	return &httpExchange{writer: w, request: r, carrier: session.HTTPCarrier(w, r)}
}

// httpExchange is the net/http half of the seam.
//
// It keeps the request rather than a copy of what it read, because two of the
// operations replace it: recording an authentication and attaching a session
// both derive a context, and everything asked afterwards has to see the derived
// one. request() is what the rest of the chain must be given.
type httpExchange struct {
	writer  http.ResponseWriter
	request *http.Request
	carrier session.Carrier
}

func (x *httpExchange) Cookies() []*http.Cookie       { return x.carrier.Cookies() }
func (x *httpExchange) SetCookie(cookie *http.Cookie) { x.carrier.SetCookie(cookie) }
func (x *httpExchange) Context() context.Context      { return x.request.Context() }

func (x *httpExchange) Method() string  { return x.request.Method }
func (x *httpExchange) Path() string    { return x.request.URL.Path }
func (x *httpExchange) RawPath() string { return x.request.URL.RawPath }
func (x *httpExchange) Target() string  { return x.request.URL.RequestURI() }

func (x *httpExchange) Query(name string) string     { return x.request.URL.Query().Get(name) }
func (x *httpExchange) FormValue(name string) string { return x.request.PostFormValue(name) }
func (x *httpExchange) Header(name string) string    { return x.request.Header.Get(name) }

func (x *httpExchange) HeaderValues(name string) []string { return x.request.Header.Values(name) }

// Body reads through MaxBytesReader rather than a LimitReader, so an oversized
// document fails as one instead of arriving truncated and failing as malformed.
func (x *httpExchange) Body(limit int64) ([]byte, error) {
	if x.request.Body == nil {
		return nil, nil
	}
	return io.ReadAll(http.MaxBytesReader(x.writer, x.request.Body, limit))
}

func (x *httpExchange) IsTLS() bool           { return x.request.TLS != nil }
func (x *httpExchange) RemoteAddress() string { return x.request.RemoteAddr }
func (x *httpExchange) Host() string          { return x.request.Host }

func (x *httpExchange) SetHeader(name, value string) { x.writer.Header().Set(name, value) }

func (x *httpExchange) Write(status int, body []byte) {
	x.writer.WriteHeader(status)
	_, _ = x.writer.Write(body)
}

// Problem and Redirect go through popcornwave/pwextension rather than through
// the runtime directly. That is what keeps this package linkable without it:
// the runtime publishes both answers, and a build that has no runtime in it
// gets the document without the error page rather than nothing at all.
func (x *httpExchange) Problem(err error) { pwextension.Problem(x.writer, x.request, err) }

func (x *httpExchange) Redirect(location string, status int) {
	pwextension.Redirect(x.writer, x.request, location, status)
}

func (x *httpExchange) RecordAuthentication(authentication pwruntime.Authentication) {
	x.request = x.request.WithContext(pwruntime.WithAuthentication(x.request.Context(), authentication))
}

func (x *httpExchange) AttachSession(resolved session.Resolved) {
	x.request = x.request.WithContext(resolved.Attach(x.request.Context()))
}

// continueWith calls next with whatever this exchange recorded.
//
// It is the one operation the interface does not carry, because continuing a
// chain is not something the endpoints decide: they say "this request may pass"
// and each transport spells that its own way.
func (x *httpExchange) continueWith(next http.Handler) { next.ServeHTTP(x.writer, x.request) }

// httpFrame wraps a neutral step in this transport's middleware shape.
//
// Every frame this package installs goes through it, so the wrapping exists
// once: build the exchange, run the step, and let the step decide whether the
// chain continues.
func httpFrame(step Step) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			exchange := newHTTPExchange(w, r)
			step(exchange, func() { exchange.continueWith(next) })
		})
	}
}
