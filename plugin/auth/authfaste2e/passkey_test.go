package authfaste2e

import (
	"net/http"
	"strings"
	"testing"

	"github.com/shibukawa/popcornwave/contrib/passkey"
	"github.com/shibukawa/popcornwave/contrib/passkey/passkeytest"
)

// A whole passkey enrollment and login, over fasthttp.
//
// The ceremonies are what press hardest on the transport seam: a JSON body read
// after the handler was entered, a strictly-scoped cookie set on one response
// and consumed on the next, and a session rotated in the middle of a request
// whose response has not committed. Whether the ceremony itself is right is
// passkeye2e's subject; whether this transport carries it is this one's.
func TestAPasskeyEnrollsAndLogsInOverFastHTTP(t *testing.T) {
	browser := newBrowser(t)
	browser.login()
	account := strings.TrimPrefix(browser.whoami(), "oidc:")

	authenticator, err := passkeytest.NewAuthenticator(passkeytest.WithOrigin(browser.base))
	if err != nil {
		t.Fatal(err)
	}
	enroll(t, browser, authenticator)

	// Signing out leaves the credential behind, which is the whole point of
	// enrolling one.
	browser.logout()
	if identity := browser.whoami(); identity != "anonymous" {
		t.Fatalf("after logout = %q", identity)
	}

	var options passkey.RequestOptions
	browser.postJSON("/auth/passkey/login/begin", nil, &options)
	assertion, err := authenticator.Get(options)
	if err != nil {
		t.Fatal(err)
	}
	browser.postJSON("/auth/passkey/login/finish", assertion, nil)

	after := browser.whoami()
	if after != "passkey:"+account {
		t.Fatalf("after passkey login = %q, want passkey:%s", after, account)
	}
}

// A forged assertion is refused. The faults travel through the same encoder a
// valid response does, so what this proves about the transport is that the body
// it delivered is the one that was posted rather than a truncated or reused one.
func TestAForgedAssertionIsRefusedOverFastHTTP(t *testing.T) {
	browser := newBrowser(t)
	browser.login()
	authenticator, err := passkeytest.NewAuthenticator(passkeytest.WithOrigin(browser.base))
	if err != nil {
		t.Fatal(err)
	}
	enroll(t, browser, authenticator)
	browser.logout()

	authenticator.SetFault(passkeytest.FaultSignature)
	defer authenticator.SetFault(passkeytest.FaultNone)

	var options passkey.RequestOptions
	browser.postJSON("/auth/passkey/login/begin", nil, &options)
	assertion, err := authenticator.Get(options)
	if err != nil {
		t.Fatal(err)
	}
	response, payload := browser.post("/auth/passkey/login/finish", assertion)
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("a forged assertion answered %d, want 403: %s", response.StatusCode, payload)
	}
	if identity := browser.whoami(); identity != "anonymous" {
		t.Fatalf("a rejected assertion produced %q", identity)
	}
}

// The ceremony cookie is single use, so a captured response cannot be replayed.
// It is the cookie the transport writes and reads back, so this is as much a
// test of the translation as of the rule.
func TestACeremonyIsConsumedOnceOverFastHTTP(t *testing.T) {
	browser := newBrowser(t)
	browser.login()
	authenticator, err := passkeytest.NewAuthenticator(passkeytest.WithOrigin(browser.base))
	if err != nil {
		t.Fatal(err)
	}
	enroll(t, browser, authenticator)
	browser.logout()

	var options passkey.RequestOptions
	browser.postJSON("/auth/passkey/login/begin", nil, &options)
	assertion, err := authenticator.Get(options)
	if err != nil {
		t.Fatal(err)
	}
	browser.postJSON("/auth/passkey/login/finish", assertion, nil)
	browser.logout()

	// The same assertion again: the cookie is gone, so there is no ceremony left
	// to finish and nothing to replay against.
	response, _ := browser.post("/auth/passkey/login/finish", assertion)
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("a replayed assertion answered %d, want 400", response.StatusCode)
	}
}

// Enrollment is an authenticated act, and a ceremony is a same-origin POST.
func TestTheCeremonyEndpointsRefuseTheWrongShapes(t *testing.T) {
	browser := newBrowser(t)

	response, _ := browser.post("/auth/passkey/register/begin", nil)
	if response.StatusCode != http.StatusUnauthorized {
		t.Errorf("an anonymous enrollment answered %d, want 401", response.StatusCode)
	}

	request, err := http.NewRequest(http.MethodPost, browser.base+"/auth/passkey/login/begin", strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", "https://attacker.example")
	response, _ = browser.do(request)
	if response.StatusCode != http.StatusForbidden {
		t.Errorf("a cross-origin ceremony answered %d, want 403", response.StatusCode)
	}

	response, _ = browser.get("/auth/passkey/login/begin")
	if response.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("a navigation to a ceremony answered %d, want 405", response.StatusCode)
	}
}

// A body larger than the ceremony bound is refused as an oversized document
// rather than arriving truncated and failing as malformed JSON.
func TestAnOversizedCeremonyBodyIsRefused(t *testing.T) {
	browser := newBrowser(t)
	browser.login()

	request, err := http.NewRequest(http.MethodPost, browser.base+"/auth/passkey/register/finish",
		strings.NewReader(`{"id":"`+strings.Repeat("a", 128<<10)+`"}`))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", browser.base)
	response, _ := browser.do(request)
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("an oversized ceremony body answered %d, want 400", response.StatusCode)
	}
}

// The bootstrap endpoint belongs to passkey_only, so this deployment must not
// serve it: an absent endpoint answers 404 rather than 405, so the mode is not
// discoverable by probing.
func TestTheBootstrapEndpointIsAbsentHere(t *testing.T) {
	browser := newBrowser(t)
	response, _ := browser.post("/auth/passkey/bootstrap", nil)
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("bootstrap answered %d, want 404", response.StatusCode)
	}
}

func enroll(t *testing.T, b *browser, authenticator *passkeytest.Authenticator) {
	t.Helper()
	var creation passkey.CreationOptions
	b.postJSON("/auth/passkey/register/begin", nil, &creation)
	if creation.RP.ID != "localhost" {
		t.Fatalf("rp.id = %q, want the configured relying party", creation.RP.ID)
	}
	if creation.Challenge == "" {
		t.Fatal("the relying party emitted no challenge")
	}
	registration, err := authenticator.Create(creation)
	if err != nil {
		t.Fatal(err)
	}
	b.postJSON("/auth/passkey/register/finish", registration, nil)
}

// logout ends the session the way the sign-out button does.
func (b *browser) logout() {
	b.t.Helper()
	response, payload := b.postForm("/auth/logout", "")
	if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusSeeOther &&
		response.StatusCode != http.StatusNotFound {
		b.t.Fatalf("logout answered %d: %s", response.StatusCode, payload)
	}
}
