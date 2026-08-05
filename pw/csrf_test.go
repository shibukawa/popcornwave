package pw

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/shibukawa/popcornwave/pwruntime"
	"github.com/shibukawa/tinybind-go/htmlbind"
)

// formFragment is the shape generation emits for a component holding an unsafe
// form: the hidden field is the first child, and no author wrote it.
func formFragment() HTMLFragment {
	builder := htmlbind.Builder[struct{}]{}
	return htmlbind.Bind(&htmlbind.Plan[struct{}]{
		Ops: []htmlbind.Op[struct{}]{
			builder.Static(`<form method="post" action="/orders">`),
			builder.CSRFField("_csrf"),
			builder.Static(`<button>buy</button></form>`),
		},
	}, struct{}{})
}

var hiddenValue = regexp.MustCompile(`name="_csrf" value="([^"]*)"`)

func sessionRequest(secret string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/orders/new", nil)
	if secret == "" {
		return r
	}
	return r.WithContext(pwruntime.WithCSRFSecret(r.Context(), secret))
}

// The render path supplies the token, so a page carrying a form gets one
// without the handler, the template, or the author naming it.
func TestWriteHTMLChainPutsAVerifyingTokenInAnUnsafeForm(t *testing.T) {
	secret, err := pwruntime.NewCSRFSecret(nil)
	if err != nil {
		t.Fatalf("NewCSRFSecret: %v", err)
	}
	recorder := httptest.NewRecorder()
	WriteHTMLChain(recorder, sessionRequest(secret), nil, formFragment())
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\n%s", recorder.Code, recorder.Body.String())
	}
	match := hiddenValue.FindStringSubmatch(recorder.Body.String())
	if match == nil {
		t.Fatalf("no hidden field in:\n%s", recorder.Body.String())
	}
	if !pwruntime.VerifyCSRFToken(secret, match[1]) {
		t.Errorf("the emitted token does not verify against the session secret")
	}
}

// Two renders of one page emit different bytes, which is what denies a
// compression oracle anything to accumulate.
func TestRenderedTokensDifferBetweenResponses(t *testing.T) {
	secret, err := pwruntime.NewCSRFSecret(nil)
	if err != nil {
		t.Fatalf("NewCSRFSecret: %v", err)
	}
	seen := map[string]bool{}
	for range 2 {
		recorder := httptest.NewRecorder()
		WriteHTMLChain(recorder, sessionRequest(secret), nil, formFragment())
		match := hiddenValue.FindStringSubmatch(recorder.Body.String())
		if match == nil {
			t.Fatalf("no hidden field in:\n%s", recorder.Body.String())
		}
		seen[match[1]] = true
	}
	if len(seen) != 2 {
		t.Error("two responses carried the same token bytes")
	}
}

// A request with no session fails the render rather than emitting an empty
// field. htmlbind.WithoutCSRFToken would have rendered one, which is right for a
// mail body and wrong for a response: it would put an unprotected form on screen
// and say nothing.
func TestUnsafeFormWithoutASessionFailsInsteadOfRenderingEmpty(t *testing.T) {
	recorder := httptest.NewRecorder()
	WriteHTMLChain(recorder, sessionRequest(""), nil, formFragment())
	if recorder.Code == http.StatusOK {
		t.Fatalf("status = 200, want a failure\n%s", recorder.Body.String())
	}
	if match := hiddenValue.FindStringSubmatch(recorder.Body.String()); match != nil {
		t.Errorf("an unprotected field reached the response: %q", match[1])
	}
}

// A page with no unsafe form renders the same with or without a session, so
// adopting this costs nothing to a project that has none.
func TestPageWithNoFormIsUnaffected(t *testing.T) {
	withSession := httptest.NewRecorder()
	secret, err := pwruntime.NewCSRFSecret(nil)
	if err != nil {
		t.Fatalf("NewCSRFSecret: %v", err)
	}
	WriteHTMLChain(withSession, sessionRequest(secret), nil, staticFragment(`<h1>home</h1>`))
	without := httptest.NewRecorder()
	WriteHTMLChain(without, sessionRequest(""), nil, staticFragment(`<h1>home</h1>`))
	if withSession.Body.String() != without.Body.String() {
		t.Errorf("a page with no form differed:\n%q\n%q", withSession.Body.String(), without.Body.String())
	}
}

// The runtime reads the token when it issues a request, not when the page
// loaded. A value fixed at render could not survive a rotation, and the live
// connection is exactly the long-lived case where one happens.
func TestBoundaryRuntimeSendsTheTokenItReadsFromTheCookie(t *testing.T) {
	for _, fragment := range []string{
		`const csrfCookieName = "` + CSRFCookieName + `";`,
		`const csrfHeaderName = "X-CSRF-Token";`,
		// Read per request, so a rotation reaches an open screen.
		"function csrfToken() {",
		// Every request this runtime issues goes through one helper, so a call
		// site cannot omit the header by being written without it.
		"function withCSRF(headers) {",
		`headers: withCSRF({ [modeHeader]: liveMode }),`,
	} {
		if !strings.Contains(boundaryRuntimeScript, fragment) {
			t.Errorf("the runtime is missing %q", fragment)
		}
	}
}

// The name is a contract between the Go side and the script, so the two must
// not be able to drift: a module script cannot read it off its own tag.
func TestCSRFCookieNameIsOneValueOnBothSides(t *testing.T) {
	if CSRFCookieName == "" {
		t.Fatal("the cookie name is empty")
	}
	if !strings.Contains(boundaryRuntimeScript, `"`+CSRFCookieName+`"`) {
		t.Errorf("the script does not name %q", CSRFCookieName)
	}
}
