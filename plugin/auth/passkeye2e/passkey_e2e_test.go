package passkeye2e

import (
	"net/http"
	"strings"
	"testing"

	"github.com/shibukawa/popcornwave/contrib/passkey"
	"github.com/shibukawa/popcornwave/contrib/passkey/passkeytest"
)

// TestOIDCPasskeyStory walks the whole oidc_passkey path: an OIDC login
// bootstraps the account, the account enrolls a passkey, and the passkey alone
// then logs it back in.
func TestOIDCPasskeyStory(t *testing.T) {
	browser, authenticator := newLoggedInBrowser(t)
	account := strings.TrimPrefix(browser.whoami(), "oidc:")

	enroll(t, browser, authenticator)

	// Signing out leaves the credential behind, which is the whole point of
	// enrolling one.
	browser.logout()
	if identity := browser.whoami(); identity != "anonymous" {
		t.Fatalf("after logout = %q", identity)
	}

	passkeyLogin(t, browser, authenticator)
	after := browser.whoami()
	if !strings.HasPrefix(after, "passkey:") {
		t.Fatalf("after passkey login = %q, want a passkey session", after)
	}
	if strings.TrimPrefix(after, "passkey:") != account {
		t.Fatalf("passkey login landed on %q, want account %q", after, account)
	}
}

// TestForgedAssertionsAreRefused proves the endpoint verifies the ceremony
// rather than trusting what the browser posted. Every fault travels through the
// same encoder as a valid response, so this exercises the real path.
func TestForgedAssertionsAreRefused(t *testing.T) {
	browser, authenticator := newLoggedInBrowser(t)
	enroll(t, browser, authenticator)
	browser.logout()

	for _, fault := range []passkeytest.Fault{
		passkeytest.FaultSignature,
		passkeytest.FaultOrigin,
		passkeytest.FaultChallenge,
		passkeytest.FaultRPID,
		passkeytest.FaultUserPresence,
		passkeytest.FaultUserVerification,
		passkeytest.FaultUserHandle,
	} {
		t.Run(string(fault), func(t *testing.T) {
			authenticator.SetFault(fault)
			defer authenticator.SetFault(passkeytest.FaultNone)

			var options passkey.RequestOptions
			browser.postJSON("/auth/passkey/login/begin", nil, &options)
			assertion, err := authenticator.Get(options)
			if err != nil {
				t.Fatal(err)
			}
			browser.postExpecting("/auth/passkey/login/finish", assertion, http.StatusForbidden)
			if identity := browser.whoami(); identity != "anonymous" {
				t.Fatalf("a rejected assertion produced %q", identity)
			}
		})
	}
}

// A counter that does not advance is what a cloned authenticator looks like, so
// the login is refused rather than merely noted.
func TestReplayedCounterIsRefused(t *testing.T) {
	browser, authenticator := newLoggedInBrowser(t)
	enroll(t, browser, authenticator)
	browser.logout()
	passkeyLogin(t, browser, authenticator)
	browser.logout()

	authenticator.SetFault(passkeytest.FaultSignCount)
	defer authenticator.SetFault(passkeytest.FaultNone)
	var options passkey.RequestOptions
	browser.postJSON("/auth/passkey/login/begin", nil, &options)
	assertion, err := authenticator.Get(options)
	if err != nil {
		t.Fatal(err)
	}
	browser.postExpecting("/auth/passkey/login/finish", assertion, http.StatusForbidden)
	if identity := browser.whoami(); identity != "anonymous" {
		t.Fatalf("a replayed counter produced %q", identity)
	}
}

// Ceremony state is single use, so a captured response cannot be replayed.
func TestCeremonyStateIsConsumedOnce(t *testing.T) {
	browser, authenticator := newLoggedInBrowser(t)
	enroll(t, browser, authenticator)
	browser.logout()

	var options passkey.RequestOptions
	browser.postJSON("/auth/passkey/login/begin", nil, &options)
	assertion, err := authenticator.Get(options)
	if err != nil {
		t.Fatal(err)
	}
	browser.postExpecting("/auth/passkey/login/finish", assertion, http.StatusOK)
	browser.logout()

	// The same assertion again: the cookie is gone, so there is no ceremony
	// left to finish and nothing to replay against.
	browser.postExpecting("/auth/passkey/login/finish", assertion, http.StatusBadRequest)
}

// A well-formed assertion from a credential this deployment never stored is
// refused exactly like a bad signature, so a probe learns nothing.
func TestUnknownCredentialIsRefusedLikeABadSignature(t *testing.T) {
	browser, authenticator := newLoggedInBrowser(t)
	enroll(t, browser, authenticator)

	// A second account enrolls its own device. Its credential is valid, but it
	// is not the one this browser will claim.
	stranger, strangerAuthenticator := newLoggedInBrowser(t)
	enroll(t, stranger, strangerAuthenticator)

	browser.logout()
	var options passkey.RequestOptions
	browser.postJSON("/auth/passkey/login/begin", nil, &options)

	// An authenticator that holds no credential for this relying party cannot
	// answer at all, which is the client-side half of the same refusal.
	empty, err := passkeytest.NewAuthenticator(passkeytest.WithOrigin(browser.base))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := empty.Get(options); err == nil {
		t.Fatal("an authenticator with no credential produced an assertion")
	}
	_ = authenticator
}

// Enrollment is an authenticated act.
func TestEnrollmentRequiresASession(t *testing.T) {
	browser := newBrowser(t)
	browser.postExpecting("/auth/passkey/register/begin", nil, http.StatusUnauthorized)
	browser.postExpecting("/auth/passkey/register/finish", nil, http.StatusUnauthorized)
}

// A ceremony is a same-origin POST. Neither a cross-origin post nor a
// navigation is one.
func TestCeremonyEndpointsRefuseNonBrowserShapes(t *testing.T) {
	browser := newBrowser(t)

	request, err := http.NewRequest(http.MethodPost, browser.base+"/auth/passkey/login/begin", strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", "https://attacker.example")
	response, _ := browser.do(request)
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("cross-origin ceremony status = %d, want 403", response.StatusCode)
	}

	get, err := browser.client.Get(browser.base + "/auth/passkey/login/begin")
	if err != nil {
		t.Fatal(err)
	}
	defer get.Body.Close()
	if get.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("GET status = %d, want 405", get.StatusCode)
	}
}

// The bootstrap endpoint belongs to passkey_only, so this deployment must not
// serve it at all: an absent endpoint answers 404 rather than 405, so the mode
// is not discoverable by probing.
func TestBootstrapEndpointIsAbsentInOIDCPasskey(t *testing.T) {
	browser := newBrowser(t)
	response, _ := browser.post("/auth/passkey/bootstrap", nil)
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("bootstrap status = %d, want 404", response.StatusCode)
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
	b.postExpecting("/auth/passkey/register/finish", registration, http.StatusOK)
}

func passkeyLogin(t *testing.T, b *browser, authenticator *passkeytest.Authenticator) {
	t.Helper()
	var options passkey.RequestOptions
	b.postJSON("/auth/passkey/login/begin", nil, &options)
	assertion, err := authenticator.Get(options)
	if err != nil {
		t.Fatal(err)
	}
	b.postExpecting("/auth/passkey/login/finish", assertion, http.StatusOK)
}
