package session

import (
	"context"
	"net/http"
)

// Carrier is everything a session needs from the transport it is served over.
//
// It is three methods because that is all a session actually touches: it reads
// the cookies that arrived, it sets the cookies that leave, and it needs the
// request's context to reach a store. Everything else this package does — the
// token, the sealing, the slot map, the rotation rules, the deadlines — is
// arithmetic over those.
//
// Having found that out, the interface is what lets a second transport carry a
// session without a second implementation of any of it. The alternative was a
// parallel copy of the lifecycle, and a session is the last thing that should
// exist twice: two implementations of when a token rotates, or of which cookie
// is cleared on a stale record, are two chances to leave a session valid that
// should have ended.
//
// The cookie type is net/http's. It is a plain data struct with no behaviour
// and no transport in it — a name, a value, and the attributes a browser is
// told — so it describes a cookie rather than implementing one, and a transport
// that spells cookies differently translates at its own edge.
type Carrier interface {
	// Cookies returns the cookies the request carried.
	Cookies() []*http.Cookie
	// SetCookie adds one Set-Cookie to the response.
	SetCookie(cookie *http.Cookie)
	// Context is the request's context, which is what a store is reached
	// through.
	Context() context.Context
}

// HTTPCarrier carries a session over net/http.
func HTTPCarrier(w http.ResponseWriter, r *http.Request) Carrier {
	return &httpCarrier{writer: w, request: r}
}

type httpCarrier struct {
	writer  http.ResponseWriter
	request *http.Request
	// cookies is the parsed Cookie header, kept because the token and every
	// jar-backed slot read it and parsing once per request is the point.
	cookies []*http.Cookie
	parsed  bool
}

func (c *httpCarrier) Cookies() []*http.Cookie {
	if !c.parsed {
		c.parsed = true
		if c.request != nil {
			c.cookies = c.request.Cookies()
		}
	}
	return c.cookies
}

func (c *httpCarrier) SetCookie(cookie *http.Cookie) {
	if c.writer == nil || cookie == nil {
		return
	}
	http.SetCookie(c.writer, cookie)
}

func (c *httpCarrier) Context() context.Context {
	if c.request == nil {
		return context.Background()
	}
	return c.request.Context()
}

// writable reports whether a response is reachable, which a store checks before
// it seals a record into one.
func writable(carrier Carrier) bool {
	if carrier == nil {
		return false
	}
	if c, ok := carrier.(*httpCarrier); ok {
		return c.writer != nil
	}
	return true
}

// readable reports whether a request is reachable.
func readable(carrier Carrier) bool {
	if carrier == nil {
		return false
	}
	if c, ok := carrier.(*httpCarrier); ok {
		return c.request != nil
	}
	return true
}

// lookupCookie finds one cookie by name.
//
// Names are compared exactly, as RFC 6265 requires: a cookie name is
// case-sensitive, and matching loosely would let a client set Session beside
// session and choose which one a store reads.
func lookupCookie(cookies []*http.Cookie, name string) *http.Cookie {
	for _, cookie := range cookies {
		if cookie != nil && cookie.Name == name {
			return cookie
		}
	}
	return nil
}
