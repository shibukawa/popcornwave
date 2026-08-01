package passkeyonlye2e

import (
	"net/http"
	"testing"

	"github.com/shibukawa/popcornwave/contrib/passkey"
	"github.com/shibukawa/popcornwave/contrib/passkey/passkeytest"
)

// TestPasskeyOnlyStory walks the whole passkey_only path: an administrator
// issues a login ID and a temporary secret, the holder redeems it for one
// restricted enrollment, and the resulting passkey is thereafter the only way
// in.
func TestPasskeyOnlyStory(t *testing.T) {
	browser := newBrowser(t)
	authenticator := browser.authenticator()
	secret := issue(t, "login-story", "account-story")

	if identity := browser.whoami(); identity != "anonymous" {
		t.Fatalf("before redemption = %q", identity)
	}
	browser.postExpecting("/auth/passkey/bootstrap", redemption{LoginID: "login-story", Secret: secret}, http.StatusOK)

	// A redeemed secret authorizes one enrollment and nothing else. It is not a
	// login, so the request is still anonymous.
	if identity := browser.whoami(); identity != "anonymous" {
		t.Fatalf("a redeemed bootstrap credential produced %q, want no session", identity)
	}

	enroll(t, browser, authenticator)

	// Finishing the first credential is what creates the session.
	after := browser.whoami()
	if after != "passkey:account-story" {
		t.Fatalf("after enrollment = %q, want passkey:account-story", after)
	}

	browser.logout()
	if identity := browser.whoami(); identity != "anonymous" {
		t.Fatalf("after logout = %q", identity)
	}
	passkeyLogin(t, browser, authenticator)
	if identity := browser.whoami(); identity != "passkey:account-story" {
		t.Fatalf("after passkey login = %q", identity)
	}
}

// A bootstrap credential is single use, so the same secret cannot open a second
// enrollment even for the account that already used it.
func TestBootstrapCredentialIsSpentByTheFirstEnrollment(t *testing.T) {
	browser := newBrowser(t)
	secret := issue(t, "login-once", "account-once")
	browser.postExpecting("/auth/passkey/bootstrap", redemption{LoginID: "login-once", Secret: secret}, http.StatusOK)
	enroll(t, browser, browser.authenticator())

	second := newBrowser(t)
	second.postExpecting("/auth/passkey/bootstrap", redemption{LoginID: "login-once", Secret: secret}, http.StatusForbidden)
	if identity := second.whoami(); identity != "anonymous" {
		t.Fatalf("a spent credential produced %q", identity)
	}
}

// Every redemption failure answers identically, so a guess cannot tell an
// unknown login ID from a wrong secret.
func TestRedemptionFailuresAreIndistinguishable(t *testing.T) {
	browser := newBrowser(t)
	secret := issue(t, "login-enum", "account-enum")

	for _, testCase := range []struct {
		name string
		body redemption
	}{
		{"unknown login id", redemption{LoginID: "never-issued", Secret: secret}},
		{"wrong secret", redemption{LoginID: "login-enum", Secret: "not-the-secret"}},
		{"empty secret", redemption{LoginID: "login-enum", Secret: ""}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			newBrowser(t).postExpecting("/auth/passkey/bootstrap", testCase.body, http.StatusForbidden)
		})
	}
	_ = browser
}

// The attempt budget is spent by wrong guesses, and once it is gone the correct
// secret no longer works either.
func TestAttemptBudgetIsExhaustedByGuesses(t *testing.T) {
	secret := issue(t, "login-budget", "account-budget")
	// bootstrap.max_attempts is 3 in this deployment.
	for range 3 {
		newBrowser(t).postExpecting("/auth/passkey/bootstrap",
			redemption{LoginID: "login-budget", Secret: "wrong"}, http.StatusForbidden)
	}
	newBrowser(t).postExpecting("/auth/passkey/bootstrap",
		redemption{LoginID: "login-budget", Secret: secret}, http.StatusForbidden)
}

// Without a redeemed credential there is no way into enrollment at all, which
// is the whole security property of passkey_only.
func TestEnrollmentIsUnreachableWithoutRedemption(t *testing.T) {
	browser := newBrowser(t)
	browser.postExpecting("/auth/passkey/register/begin", nil, http.StatusUnauthorized)
	browser.postExpecting("/auth/passkey/register/finish", nil, http.StatusUnauthorized)
}

// passkey_only mounts no OIDC endpoint, so the login path of the other modes is
// simply not there.
func TestOIDCEndpointsAreAbsent(t *testing.T) {
	browser := newBrowser(t)
	for _, path := range []string{"/auth/login", "/auth/callback"} {
		response, err := browser.client.Get(browser.base + path)
		if err != nil {
			t.Fatal(err)
		}
		_ = response.Body.Close()
		if response.StatusCode != http.StatusNotFound {
			t.Fatalf("GET %s status = %d, want 404", path, response.StatusCode)
		}
	}
}

// An enrollment ticket authorizes one registration, so a second attempt with
// the same browser has nothing left to spend.
func TestEnrollmentTicketIsSingleUse(t *testing.T) {
	browser := newBrowser(t)
	secret := issue(t, "login-ticket", "account-ticket")
	browser.postExpecting("/auth/passkey/bootstrap", redemption{LoginID: "login-ticket", Secret: secret}, http.StatusOK)
	enroll(t, browser, browser.authenticator())
	browser.logout()

	// The ticket was consumed by the enrollment that used it.
	browser.postExpecting("/auth/passkey/register/begin", nil, http.StatusUnauthorized)
}

// A forged assertion is refused here exactly as it is under oidc_passkey, and a
// deployment with no other login method must not fall open.
func TestForgedAssertionIsRefused(t *testing.T) {
	browser := newBrowser(t)
	authenticator := browser.authenticator()
	secret := issue(t, "login-forge", "account-forge")
	browser.postExpecting("/auth/passkey/bootstrap", redemption{LoginID: "login-forge", Secret: secret}, http.StatusOK)
	enroll(t, browser, authenticator)
	browser.logout()

	for _, fault := range []passkeytest.Fault{
		passkeytest.FaultSignature,
		passkeytest.FaultOrigin,
		passkeytest.FaultChallenge,
		passkeytest.FaultUserVerification,
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

func enroll(t *testing.T, b *browser, authenticator *passkeytest.Authenticator) {
	t.Helper()
	var creation passkey.CreationOptions
	b.postJSON("/auth/passkey/register/begin", nil, &creation)
	if creation.RP.ID != "localhost" || creation.Challenge == "" {
		t.Fatalf("creation options = %+v, want the configured relying party and a challenge", creation)
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
