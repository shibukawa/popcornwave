package authfastjwte2e

import (
	"net/http"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// A verified token authenticates the request, and what the frame recorded is
// what the handler reads. On this transport the recording is a write into the
// pooled request value, so this is the load-bearing assertion of the mode.
func TestAVerifiedBearerTokenAuthenticatesOverFastHTTP(t *testing.T) {
	response, body := call(t, "/api/thing", map[string]string{"Authorization": bearer(t, nil)})

	if response.StatusCode != http.StatusOK {
		t.Fatalf("a verified token answered %d: %s", response.StatusCode, body)
	}
	if body != "thing:bearer:caller-1" {
		t.Fatalf("body = %q, want the method and the verified subject", body)
	}
}

// An anonymous request is not a failure. The guard decides whether the path
// needed a credential, so an unprotected one serves without.
func TestAnAnonymousRequestReachesAnOpenPath(t *testing.T) {
	response, body := call(t, "/open", nil)

	if response.StatusCode != http.StatusOK {
		t.Fatalf("an anonymous request to an open path answered %d", response.StatusCode)
	}
	if body != "open:anonymous" {
		t.Fatalf("body = %q", body)
	}
}

// A protected path with no credential is refused, and the refusal names the
// scheme this deployment accepts so a client that sent nothing learns what to
// send. The realm comes from the audience, which the caller already had to know
// to have asked for a token.
func TestAProtectedPathChallengesAnAnonymousCaller(t *testing.T) {
	response, _ := call(t, "/api/thing", nil)

	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("an anonymous request to a protected path answered %d, want 401", response.StatusCode)
	}
	challenge := response.Header.Get("WWW-Authenticate")
	if !strings.HasPrefix(challenge, "Bearer realm=") {
		t.Fatalf("WWW-Authenticate = %q, want a Bearer challenge", challenge)
	}
	if !strings.Contains(challenge, audience) {
		t.Errorf("the challenge does not name the audience: %q", challenge)
	}
	if response.Header.Get("Cache-Control") != "no-store" {
		t.Errorf("Cache-Control = %q; an authorization decision must not be cached",
			response.Header.Get("Cache-Control"))
	}
}

// Every rejection is the same answer. A caller learns that the credential was
// not accepted and never which check rejected it, because the difference
// between a wrong audience, an expired token and a bad signature is an oracle
// for probing what this deployment trusts.
func TestEveryRefusalLooksAlike(t *testing.T) {
	valid := bearer(t, nil)
	broken := valid[:len(valid)-4] + "AAAA"

	refusals := map[string]string{
		"a bad signature": broken,
		"an expired token": bearer(t, func(claims map[string]any) {
			claims["exp"] = time.Now().Add(-time.Hour).Unix()
			claims["iat"] = time.Now().Add(-2 * time.Hour).Unix()
		}),
		"the wrong audience": bearer(t, func(claims map[string]any) {
			claims["aud"] = []string{"https://somewhere.else"}
		}),
		"the wrong issuer": bearer(t, func(claims map[string]any) {
			claims["iss"] = "https://not.this.issuer"
		}),
		"a token that is not one": "Bearer not-a-token",
	}

	var first string
	for name, credential := range refusals {
		t.Run(name, func(t *testing.T) {
			response, body := call(t, "/api/thing", map[string]string{"Authorization": credential})
			if response.StatusCode != http.StatusUnauthorized {
				t.Fatalf("answered %d, want 401: %s", response.StatusCode, body)
			}
			challenge := response.Header.Get("WWW-Authenticate")
			if !strings.Contains(challenge, `error="invalid_token"`) {
				t.Errorf("WWW-Authenticate = %q, want the invalid_token error", challenge)
			}
			if first == "" {
				first = body
				return
			}
			if body != first {
				t.Errorf("this refusal reads differently from the others:\n got %q\nwant %q", body, first)
			}
		})
	}
}

// Two Authorization headers are refused rather than merged: which one a proxy
// forwards is not this application's decision to guess. The reader has to see
// both to refuse them, which on this transport takes the plural accessor.
func TestASecondAuthorizationHeaderIsRefused(t *testing.T) {
	credential := bearer(t, nil)
	response, _ := call(t, "/api/thing", nil)
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("precondition: the path is not protected")
	}

	request, err := http.NewRequest(http.MethodGet, start(t).base+"/api/thing", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Add("Authorization", credential)
	request.Header.Add("Authorization", credential)
	response, err = http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("two Authorization headers answered %d, want 401", response.StatusCode)
	}
}

// A credential in an unknown scheme is not a credential, so the request is
// anonymous rather than refused, and the guard decides from there.
func TestAnUnknownSchemeIsAnonymousRatherThanRefused(t *testing.T) {
	response, body := call(t, "/open", map[string]string{"Authorization": "Basic dXNlcjpwYXNz"})

	if response.StatusCode != http.StatusOK {
		t.Fatalf("an unknown scheme on an open path answered %d", response.StatusCode)
	}
	if body != "open:anonymous" {
		t.Fatalf("body = %q, want an anonymous request", body)
	}
}

// The scheme is matched without regard to case, as RFC 7235 requires.
func TestTheSchemeIsMatchedCaseInsensitively(t *testing.T) {
	credential := bearer(t, nil)
	response, body := call(t, "/api/thing", map[string]string{
		"Authorization": "bEaReR " + strings.TrimPrefix(credential, "Bearer "),
	})

	if response.StatusCode != http.StatusOK {
		t.Fatalf("a lowercase scheme answered %d: %s", response.StatusCode, body)
	}
}

// The binary these tests run in links no net/http runtime.
//
// It is the claim the layer moves were for, and it is asserted here rather than
// only in pwconfig because this is the binary that proves the whole stack: the
// settings, the bearer runtime, the chain, and the responses above are all
// served by a build that never linked pw. A dependency arrives by any path, so
// the graph is what is checked and not the import list.
func TestTheBinaryLinksNoNetHTTPRuntime(t *testing.T) {
	output, err := exec.Command("go", "list", "-deps",
		"github.com/shibukawa/popcornweb/plugin/auth/authfastjwte2e").CombinedOutput()
	if err != nil {
		t.Fatalf("go list: %v\n%s", err, output)
	}
	for _, line := range strings.Split(string(output), "\n") {
		if strings.TrimSpace(line) == "github.com/shibukawa/popcornweb/pw" {
			t.Fatal("this package depends on the net/http runtime, so the fixture proves nothing")
		}
	}
}
