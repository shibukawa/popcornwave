package pwfast

import (
	"strings"
	"testing"

	"github.com/shibukawa/popcornwave/pwruntime"
	"github.com/shibukawa/tinygodriver/fasthttp"
)

func guarded(policy GuardPolicy) fasthttp.RequestHandler {
	return Compose(func(r *fasthttp.RequestCtx) { _, _ = r.WriteString("protected page") },
		Frame{Slot: SlotGuard, Middleware: Guard(policy)})
}

func TestAnUnauthenticatedRequestIsRedirectedWhenConfigured(t *testing.T) {
	handler := guarded(GuardPolicy{
		Protected: func(path string) bool { return strings.HasPrefix(path, "/admin") },
		LoginURL:  func(path string) string { return "/login?next=" + path },
		Redirect:  true,
	})

	status, header, body := serve(t, handler, "/admin/users")
	if status != fasthttp.StatusSeeOther {
		t.Errorf("status = %d, want 303", status)
	}
	if body == "protected page" {
		t.Error("the protected page was served to an unauthenticated caller")
	}
	if !strings.Contains(header, "Location: /login?next=/admin/users") {
		t.Errorf("no login redirect:\n%s", header)
	}
	// Never cached: a shared cache holding this would show a login redirect to
	// a signed-in reader, or worse the other way round.
	if !strings.Contains(header, "Cache-Control: no-store") {
		t.Errorf("the guard answer was cacheable:\n%s", header)
	}
}

func TestAnUnauthenticatedAPIRequestIs401WithAChallenge(t *testing.T) {
	handler := guarded(GuardPolicy{
		Protected:   func(string) bool { return true },
		BearerRealm: "example",
	})
	status, header, _ := serve(t, handler, "/api/items")
	if status != fasthttp.StatusUnauthorized {
		t.Errorf("status = %d, want 401", status)
	}
	// Case-insensitively: this transport canonicalises the name as
	// Www-Authenticate and the other as WWW-Authenticate, and a client does not
	// care which. The shared test seam reads headers this way for exactly this
	// reason; a raw-header assertion has to do it by hand.
	if !strings.Contains(strings.ToLower(header), `www-authenticate: bearer realm="example"`) {
		t.Errorf("no challenge was sent:\n%s", header)
	}
}

func TestAnUnprotectedPathIsServed(t *testing.T) {
	handler := guarded(GuardPolicy{Protected: func(path string) bool { return path == "/admin" }})
	if _, _, body := serve(t, handler, "/about"); body != "protected page" {
		t.Errorf("an unprotected path was refused: %q", body)
	}
}

// A policy protecting nothing installs no frame, so a deployment that has not
// configured protection pays nothing and cannot half-have it.
func TestAnEmptyPolicyInstallsNoFrame(t *testing.T) {
	if _, _, body := serve(t, guarded(GuardPolicy{}), "/admin"); body != "protected page" {
		t.Errorf("an unconfigured guard refused: %q", body)
	}
}

// The login target is a URL a browser is sent to, and it carries a path taken
// from the request, so it goes through the same refusal every redirect does.
func TestAGuardWillNotRedirectToAScriptURL(t *testing.T) {
	handler := guarded(GuardPolicy{
		Protected: func(string) bool { return true },
		LoginURL:  func(string) string { return "javascript:alert(1)" },
		Redirect:  true,
	})
	status, header, _ := serve(t, handler, "/admin")
	if strings.Contains(strings.ToLower(header), "location: javascript:") {
		t.Errorf("the guard sent a script URL:\n%s", header)
	}
	if status == fasthttp.StatusSeeOther {
		t.Errorf("status = %d, want a refusal", status)
	}
}

// The same canonicalisation the CSRF check uses, for the same reason: a path
// that cannot be matched unambiguously could select a different target than the
// one the policy decided about.
func TestAnAmbiguousPathIsRefusedByTheGuard(t *testing.T) {
	handler := guarded(GuardPolicy{Protected: func(string) bool { return true }})
	if status, _, body := serve(t, handler, "/admin//deep"); status == fasthttp.StatusOK {
		t.Errorf("an ambiguous path was served: %d %q", status, body)
	}
}

// The guard reaches the chain from options, positioned by its slot — below the
// session, so it observes whatever the session and authentication established.
func TestTheChainInstallsTheGuardWhenAPolicyIsGiven(t *testing.T) {
	publishChainSettings(t, pwruntime.ChainSettings{})
	handler, err := Middlewares(func(r *fasthttp.RequestCtx) {
		_, _ = r.WriteString("protected page")
	}, RuntimeOptions{Guard: GuardPolicy{
		Protected: func(path string) bool { return strings.HasPrefix(path, "/admin") },
	}})
	if err != nil {
		t.Fatal(err)
	}
	if status, _, body := serve(t, handler, "/admin"); status != fasthttp.StatusUnauthorized {
		t.Errorf("a protected path answered %d %q", status, body)
	}
	if _, _, body := serve(t, handler, "/about"); body != "protected page" {
		t.Errorf("an unprotected path was refused: %q", body)
	}
}
