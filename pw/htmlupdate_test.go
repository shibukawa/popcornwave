package pw

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/shibukawa/popcornwave/pwruntime"
	"github.com/shibukawa/tinybind-go/htmlbind"
	"github.com/shibukawa/tinybind-go/htmlupdate"
)

func updateConfig() HTMLConfig {
	config := defaultHTMLConfig
	config.Update = HTMLUpdateConfig{Enabled: true, ValidatorKey: "test-validator-key", MaxManifestBytes: 8 << 10}
	return config
}

// updateRequest carries the render header a client sends to ask for a delta.
func updateRequest(t *testing.T, mode string) *http.Request {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/search?q=go", nil)
	// The version is whatever the client writes and the response echoes it back;
	// since v0.3.5 the module compares the build rather than a version number, so
	// a mode with no version is a valid request.
	request.Header.Set("Pw-Render", mode)
	// An unstamped binary has no vcs.revision, so the module falls back to a
	// per-process identity. Reading the effective value is what a rendered page
	// would have carried.
	request.Header.Set("Pw-Build", updateOptions(updateConfig()).RuntimeConfig().Build)
	return request
}

// A request that asks for nothing gets the document it always got. That is what
// keeps a crawler, curl, and a browser without the runtime unaffected.
func TestNoRenderHeaderStillServesTheDocument(t *testing.T) {
	config := updateConfig()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/search?q=go", nil)
	if serveUpdate(recorder, request, nil, staticFragment(`<h1>hi</h1>`), config, nil) {
		t.Fatal("a request with no render header was answered as an update")
	}
}

// A page rendered by another build holds client state this binary cannot vouch
// for, and none of that is visible in a validator.
func TestAnotherBuildIsAnsweredWithTheDocument(t *testing.T) {
	config := updateConfig()
	request := updateRequest(t, "navigation")
	request.Header.Set("Pw-Build", "some-other-build")
	if serveUpdate(httptest.NewRecorder(), request, nil, staticFragment(`<h1>hi</h1>`), config, nil) {
		t.Fatal("a request from another build was answered as an update")
	}
}

// The live mode shares the header with the update modes, so the module resolves
// it to a complete document. Testing it before delegating is the one thing the
// shared header costs, and this is the check that it is paid.
func TestTheLiveTokenIsNotTakenAsAnUpdate(t *testing.T) {
	config := updateConfig()
	if serveUpdate(httptest.NewRecorder(), updateRequest(t, "live"), nil, staticFragment(`<h1>hi</h1>`), config, nil) {
		t.Fatal("the live token was answered as a navigation delta")
	}
}

func TestUpdatesOffLeaveEveryRequestOnTheDocumentPath(t *testing.T) {
	config := defaultHTMLConfig
	if config.Update.Enabled {
		t.Fatal("updates default to on")
	}
	if len(documentRenderOptions(config, "")) != 0 {
		t.Error("a project with updates off pays for the runtime tag")
	}
}

// The runtime reference and its configuration are contributed at the render
// call, so no application file carries them and no shell edit can drop them.
func TestTheRuntimeIsContributedRatherThanScaffolded(t *testing.T) {
	nodes := updateHeadNodes(updateConfig(), "token-value")
	if len(nodes) != 2 {
		t.Fatalf("head nodes = %d, want the meta and the script", len(nodes))
	}
	// A malformed node would fail the render before the first byte, so the
	// nodes have to be writable to be worth contributing.
	tags, err := htmlbind.RenderHeadNodes(nodes)
	if err != nil {
		t.Fatalf("the contributed nodes are not writable: %v", err)
	}
	joined := strings.Join(tags, "")
	if !strings.Contains(joined, RuntimeScriptURL()) {
		t.Errorf("no runtime reference:\n%s", joined)
	}
	if !strings.Contains(joined, `type="module"`) {
		t.Errorf("the reference is not a module script:\n%s", joined)
	}
	// The configuration is inert escaped metadata, never inline script: a
	// module cannot read its own tag, and a policy must not need a nonce.
	if !strings.Contains(joined, updateConfigMetaName) {
		t.Errorf("no runtime configuration:\n%s", joined)
	}
	if strings.Contains(joined, "<script>") {
		t.Errorf("inline script reached the head:\n%s", joined)
	}
}

// The server and the browser build the same configuration object, so the two
// cannot disagree about a name.
func TestTheRuntimeConfigurationNamesWhatTheServerUses(t *testing.T) {
	nodes := updateHeadNodes(updateConfig(), "token-value")
	tags, err := htmlbind.RenderHeadNodes(nodes)
	if err != nil {
		t.Fatal(err)
	}
	encoded := tags[0]
	start := strings.Index(encoded, `content="`) + len(`content="`)
	end := strings.LastIndex(encoded, `"`)
	raw := strings.NewReplacer("&#34;", `"`, "&amp;", "&", "&lt;", "<", "&gt;", ">", "&#39;", "'").Replace(encoded[start:end])
	var config htmlupdate.RuntimeConfig
	if err := json.Unmarshal([]byte(raw), &config); err != nil {
		t.Fatalf("the configuration is not readable JSON: %v\n%s", err, raw)
	}
	if config.Header != UpdateHeaderPrefix {
		t.Errorf("header = %q, want %q", config.Header, UpdateHeaderPrefix)
	}
	if config.Attr != UpdateAttributePrefix {
		t.Errorf("attr = %q, want %q", config.Attr, UpdateAttributePrefix)
	}
	if config.Global != UpdateGlobalName {
		t.Errorf("global = %q, want %q", config.Global, UpdateGlobalName)
	}
	// The token travels here so the runtime can set the header on a mutating
	// request without the page having to carry it in script.
	if config.CSRF != "token-value" {
		t.Errorf("csrf = %q", config.CSRF)
	}
}

// An unkeyed digest of low-entropy content lets a guess be confirmed by
// comparing digests, so this is refused before the port is bound.
func TestUpdatesRequireAValidatorKey(t *testing.T) {
	config := updateConfig()
	config.Update.ValidatorKey = ""
	if err := validateUpdateConfig(config); err == nil {
		t.Fatal("updates were accepted with no validator key")
	}
	if err := validateUpdateConfig(updateConfig()); err != nil {
		t.Fatalf("a keyed configuration was refused: %v", err)
	}
	off := defaultHTMLConfig
	if err := validateUpdateConfig(off); err != nil {
		t.Fatalf("updates off were refused: %v", err)
	}
}

// redrawRequest is what the browser sends: the page's own URL, the redraw mode,
// and the component named in headers rather than in the path.
func redrawRequest(t *testing.T, kind, instance, query string) *http.Request {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/orders?"+query, nil)
	request.Header.Set("Pw-Render", "redraw")
	request.Header.Set("Pw-Kind", kind)
	request.Header.Set("Pw-Instance", instance)
	request.Header.Set("Pw-Build", updateOptions(updateConfig()).RuntimeConfig().Build)
	return request.WithContext(pwruntime.WithResources(request.Context(), pwruntime.Resources{
		Configs: map[reflect.Type]any{reflect.TypeFor[HTMLConfig](): updateConfig()},
	}))
}

func cardComponent(kind string) htmlupdate.Reloadable {
	return htmlupdate.Reloadable{
		KindID: kind,
		Render: func(_ *http.Request, instanceID string, values url.Values) (htmlbind.Fragment, error) {
			return staticFragment(`<article id="` + instanceID + `">page ` + values.Get("page") + `</article>`), nil
		},
	}
}

// withEmptyReloadableRegistry isolates one test from the process-wide set a
// generated init would have filled. The registry is global on purpose — it is
// what a package's init publishes into — so a test that adds to it has to put
// back what it found.
func withEmptyReloadableRegistry(t *testing.T) {
	t.Helper()
	reloadableState.Lock()
	saved := struct {
		registry *htmlupdate.Registry
		count    int
		failure  error
	}{reloadableState.registry, reloadableState.count, reloadableState.failure}
	reloadableState.registry, reloadableState.count, reloadableState.failure = &htmlupdate.Registry{}, 0, nil
	reloadableState.Unlock()
	t.Cleanup(func() {
		reloadableState.Lock()
		defer reloadableState.Unlock()
		reloadableState.registry, reloadableState.count, reloadableState.failure = saved.registry, saved.count, saved.failure
	})
}

// The escape hatch behind Redraw: a handler that publishes a set of its own
// rather than the one its page's markup reaches.
func TestRedrawComponentsAnswersAComponentTheHandlerNamed(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := redrawRequest(t, "fixture.card.Card", "card-1", "page=2")
	if !RedrawComponents(recorder, request, cardComponent("fixture.card.Card")) {
		t.Fatal("a redraw request was not answered")
	}
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", recorder.Code, recorder.Body.String())
	}
	if body := recorder.Body.String(); !strings.Contains(body, "page 2") {
		t.Errorf("the redraw did not render the component: %s", body)
	}
}

// Naming the components is what bounds the surface. A page cannot be asked to
// render one it never shows, even though the process publishes it elsewhere.
func TestRedrawComponentsRefusesAComponentThisHandlerDidNotName(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := redrawRequest(t, "fixture.other.Panel", "panel-1", "")
	if !RedrawComponents(recorder, request, cardComponent("fixture.card.Card")) {
		t.Fatal("an unnamed component was not answered at all")
	}
	if recorder.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", recorder.Code)
	}
}

// An ordinary page request passes straight through, which is what lets the call
// sit at the top of a handler that mostly renders documents.
func TestRedrawComponentsLeavesAnOrdinaryRequestAlone(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/orders", nil)
	request = request.WithContext(pwruntime.WithResources(request.Context(), pwruntime.Resources{
		Configs: map[reflect.Type]any{reflect.TypeFor[HTMLConfig](): updateConfig()},
	}))
	if RedrawComponents(recorder, request, cardComponent("fixture.card.Card")) {
		t.Error("a document request was claimed as a redraw")
	}
	if recorder.Body.Len() != 0 {
		t.Errorf("a document request had bytes written to it: %s", recorder.Body.String())
	}
}

// A page rendered by another build is not refused: at a page URL the right
// answer to a stale redraw is the page, which the caller is about to render.
func TestRedrawComponentsFromAnotherBuildFallsThroughToThePage(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := redrawRequest(t, "fixture.card.Card", "card-1", "")
	request.Header.Set("Pw-Build", "some-other-build")
	if RedrawComponents(recorder, request, cardComponent("fixture.card.Card")) {
		t.Error("a redraw from another build was answered rather than left to the page")
	}
}

// The page tree half: a generated route handler has no seam of its own, so the
// render entry answers from the process-wide published set.
func TestTheRenderEntryAnswersARegisteredRedraw(t *testing.T) {
	withEmptyReloadableRegistry(t)
	if err := RegisterReloadable(cardComponent("fixture.card.Card")); err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	request := redrawRequest(t, "fixture.card.Card", "card-1", "page=5")
	if !serveRegisteredRedraw(recorder, request, updateConfig()) {
		t.Fatal("a registered redraw was not answered by the render entry")
	}
	if body := recorder.Body.String(); !strings.Contains(body, "page 5") {
		t.Errorf("the redraw did not render the component: %s", body)
	}
}

// A project publishing none leaves every request on the document path.
func TestTheRenderEntryIgnoresRedrawWithNoRegistry(t *testing.T) {
	withEmptyReloadableRegistry(t)
	recorder := httptest.NewRecorder()
	if serveRegisteredRedraw(recorder, redrawRequest(t, "any.Kind", "card-1", ""), updateConfig()) {
		t.Error("a project publishing nothing answered a redraw")
	}
}

// The kind covers a component's name, parameters, and markup but not its
// package, so two identical templates in different packages produce the same
// one. A generated init cannot answer that, which is why startup does.
func TestADuplicateRegistrationIsAStartupDiagnostic(t *testing.T) {
	withEmptyReloadableRegistry(t)
	component := cardComponent("fixture.card.Card")
	if err := RegisterReloadable(component); err != nil {
		t.Fatal(err)
	}
	if err := validateReloadableRegistration(); err != nil {
		t.Fatalf("a clean registration was reported as a failure: %v", err)
	}
	// The generated init discards this, which is exactly the case under test.
	_ = RegisterReloadable(component)
	err := validateReloadableRegistration()
	if err == nil {
		t.Fatal("a duplicate kind was accepted")
	}
	if !strings.Contains(err.Error(), "fixture.card.Card") {
		t.Errorf("the diagnostic does not name the component: %v", err)
	}
}

// The end this whole path exists for: one URL answers a delta to a client that
// asked, and the ordinary document to everyone else.
func TestANavigationRequestIsAnsweredWithADelta(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := updateRequest(t, "navigation")
	if !serveUpdate(recorder, request, nil, staticFragment(`<h1>results</h1>`), updateConfig(), nil) {
		t.Fatal("a navigation request was not answered as an update")
	}
	response := recorder.Result()
	// A delta is a record stream, not a document. A cache that handed one to a
	// document request would put JSON records on screen.
	if got := response.Header.Get("Content-Type"); strings.Contains(got, "text/html") {
		t.Errorf("Content-Type = %q, want the record stream", got)
	}
	// The served mode is echoed, so a proxy that substituted a body is
	// detectable rather than silently applied. The version echoed is the one the
	// request carried, and since v0.3.5 the client sends a bare mode token.
	if got := response.Header.Get("Pw-Render"); got != "navigation" {
		t.Errorf("Pw-Render = %q", got)
	}
	// Both representations of one URL vary on what selected them.
	vary := strings.Join(response.Header.Values("Vary"), ",")
	for _, name := range []string{"Pw-Render", "Pw-Build"} {
		if !strings.Contains(vary, name) {
			t.Errorf("Vary %q is missing %s", vary, name)
		}
	}
	if recorder.Body.Len() == 0 {
		t.Error("the delta carried no records")
	}
}

// The document path keeps every byte it had. Adopting updates must not change
// what a request that asks for none receives.
func TestTheDocumentPathIsUnchangedByEnablingUpdates(t *testing.T) {
	off := httptest.NewRecorder()
	WriteHTMLChain(off, httptest.NewRequest(http.MethodGet, "/", nil), nil, staticFragment(`<h1>home</h1>`))
	if !strings.Contains(off.Body.String(), "<h1>home</h1>") {
		t.Fatalf("the document did not render:\n%s", off.Body.String())
	}
}
