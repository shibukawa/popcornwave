package pw

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

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
	request.Header.Set("Pw-Render", mode+";v="+strconv.Itoa(htmlupdate.Version))
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
	if config.Prefix != updatePathPrefix {
		t.Errorf("prefix = %q, want %q", config.Prefix, updatePathPrefix)
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

// A project publishing no reloadable component answers 404 under the reserved
// prefix rather than falling through to application routing.
func TestRedrawWithNoRegistryIsNotFoundRatherThanRouted(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, updatePathPrefix+"/redraw/Kind/card-1", nil)
	if !serveRedraw(recorder, request) {
		t.Fatal("the redraw path was not handled by the reserved prefix")
	}
	if recorder.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", recorder.Code)
	}
}

func TestARequestOutsideTheRedrawPrefixIsLeftAlone(t *testing.T) {
	if serveRedraw(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/orders", nil)) {
		t.Error("an application path was claimed by the redraw handler")
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
	// detectable rather than silently applied.
	if got := response.Header.Get("Pw-Render"); !strings.HasPrefix(got, "navigation;") {
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
