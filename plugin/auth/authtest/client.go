package authtest

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"

	"github.com/shibukawa/popcornwave/plugin/auth"
)

// NewClient returns an HTTP client whose jar holds a real login session for the
// identity, against a server started by testutil.TestRun.
//
// This is the rung above [NewRequest]: the request travels over a real
// connection, carries a real session cookie, and passes through every framework
// middleware including the guard. The session is created by the same session
// manager a completed login uses, so its rotation, lifetime, and cookie
// attributes are the production ones.
//
// No ceremony runs. A test that means to prove that authentication itself works
// must drive the real endpoints with contrib/passkey/passkeytest or a
// development identity provider instead; this seam assumes authentication works
// and tests what happens after it.
func NewClient(serverURL string, identity Identity) (*http.Client, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Jar: jar}
	if err := Authenticate(client, serverURL, identity); err != nil {
		return nil, err
	}
	return client, nil
}

// Authenticate installs a real session for the identity into the jar of an
// existing client, which a test uses when it built the client itself.
func Authenticate(client *http.Client, serverURL string, identity Identity) error {
	if client == nil || client.Jar == nil {
		return errors.New("authtest: the client needs a cookie jar to hold the session")
	}
	target, err := url.Parse(serverURL)
	if err != nil || target.Host == "" {
		return fmt.Errorf("authtest: unusable server URL %q", serverURL)
	}
	cookies, err := SessionCookies(identity)
	if err != nil {
		return err
	}
	client.Jar.SetCookies(target, cookies)
	return nil
}

// SessionCookies creates a real session and returns the cookies a browser would
// have received, for a test that manages its own jar or drives a handler
// directly.
func SessionCookies(identity Identity) ([]*http.Cookie, error) {
	if identity.AccountID == "" {
		return nil, errors.New("authtest: an identity needs an account ID")
	}
	identity = identity.normalized()
	// A recorder stands in for the browser that a completed login would have
	// answered, so the cookie comes from the production code path rather than
	// from a copy of its rules.
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	if err := auth.EstablishSession(recorder, request, identity.sessionData(), identity.Method); err != nil {
		return nil, fmt.Errorf("authtest: establish session: %w", err)
	}
	cookies := recorder.Result().Cookies()
	if len(cookies) == 0 {
		return nil, errors.New("authtest: the session manager issued no cookie")
	}
	return cookies, nil
}
