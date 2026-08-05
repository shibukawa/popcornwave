package authn

import "net/http"

// EnforceDeadlines returns a client whose transport honours the context
// deadline of each request.
//
// On host Go that is already true, so the client is returned unchanged. On
// TinyGo it is not: that runtime's net/http dials and then reads with no
// deadline at all, marked TINYGO TODO handle timeouts in its own source, so a
// context.WithTimeout around a request bounds nothing and a slow or hanging
// peer holds the calling handler until the peer closes the connection.
//
// This matters here because the callers are an OpenID Connect discovery, a
// token exchange, and a JWKS fetch, all of which run on a request handler, and
// all of which talk to a host this application does not control.
//
// It is deliberately applied inside these packages rather than left to the
// application: they are the TinyGo-facing clients, they accept a RequestTimeout
// and therefore promise one, and a promise that only holds on the platform they
// were not written for is worse than no promise.
func EnforceDeadlines(client *http.Client) *http.Client {
	return enforceDeadlines(client)
}
