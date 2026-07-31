package passkeye2e

import (
	"net/http"
	"testing"

	"github.com/shibukawa/popcornwave/plugin/auth"
	"github.com/shibukawa/popcornwave/plugin/auth/authtest"
)

// The guard denies an anonymous request to a protected path, which is the
// baseline every seam below has to beat.
func TestGuardDeniesAnonymousAccess(t *testing.T) {
	browser := newBrowser(t)
	response, err := browser.client.Get(browser.base + "/private")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("anonymous /private status = %d, want 401", response.StatusCode)
	}
}

// The server rung reaches past the real guard with a real session cookie, so an
// application test gets behind authentication without running a ceremony.
func TestAuthenticatedClientReachesAGuardedPath(t *testing.T) {
	deployment := start(t)
	client, err := authtest.NewClient(deployment.origin, authtest.Identity{
		AccountID: "seam-account", DisplayName: "Seam",
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	response, err := client.Get(deployment.origin + "/private")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("/private status = %d, want 200", response.StatusCode)
	}
	body := readBody(t, response)
	if body != "private:seam-account" {
		t.Fatalf("/private body = %q, want private:seam-account", body)
	}
}

// The session the seam creates is an ordinary one: it reports the method it was
// given and it ends through the real logout endpoint.
func TestSeamSessionIsAnOrdinarySession(t *testing.T) {
	deployment := start(t)
	client, err := authtest.NewClient(deployment.origin, authtest.Identity{
		AccountID: "seam-passkey", Method: auth.MethodPasskey,
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	b := &browser{t: t, client: client, base: deployment.origin}
	if identity := b.whoami(); identity != "passkey:seam-passkey" {
		t.Fatalf("whoami = %q, want passkey:seam-passkey", identity)
	}

	b.logout()
	if identity := b.whoami(); identity != "anonymous" {
		t.Fatalf("after logout = %q, want anonymous", identity)
	}
	// The guard denies the same client once its session is gone, which proves
	// the cookie was doing the work rather than the seam.
	response, err := client.Get(deployment.origin + "/private")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("/private after logout = %d, want 401", response.StatusCode)
	}
}

// Two seam clients are two independent accounts, so a test can model more than
// one user at a time.
func TestSeamClientsAreIndependent(t *testing.T) {
	deployment := start(t)
	first, err := authtest.NewClient(deployment.origin, authtest.Identity{AccountID: "seam-one"})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	second, err := authtest.NewClient(deployment.origin, authtest.Identity{AccountID: "seam-two"})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	for _, testCase := range []struct {
		client *http.Client
		want   string
	}{
		{first, "oidc:seam-one"},
		{second, "oidc:seam-two"},
	} {
		b := &browser{t: t, client: testCase.client, base: deployment.origin}
		if identity := b.whoami(); identity != testCase.want {
			t.Fatalf("whoami = %q, want %q", identity, testCase.want)
		}
	}
}

func TestSeamRefusesAnIncompleteIdentity(t *testing.T) {
	deployment := start(t)
	if _, err := authtest.NewClient(deployment.origin, authtest.Identity{}); err == nil {
		t.Fatal("NewClient accepted an identity with no account")
	}
	if err := authtest.Authenticate(&http.Client{}, deployment.origin,
		authtest.Identity{AccountID: "seam-nojar"}); err == nil {
		t.Fatal("Authenticate accepted a client with no cookie jar")
	}
}

func readBody(t *testing.T, response *http.Response) string {
	t.Helper()
	buffer := make([]byte, 0, 64)
	chunk := make([]byte, 256)
	for {
		n, err := response.Body.Read(chunk)
		buffer = append(buffer, chunk[:n]...)
		if err != nil {
			break
		}
	}
	return string(buffer)
}
