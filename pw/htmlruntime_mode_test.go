//go:build !pwdev

package pw

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// A release build serves the core and nothing else. The set has no development
// module in it and the core has no import of one, so there is no run-time
// condition a deployed binary could be talked into taking.
func TestReleaseBuildServesOnlyTheCoreModule(t *testing.T) {
	scripts := frameworkScripts()
	if len(scripts) != 1 {
		t.Fatalf("scripts = %v, want only the core", keysOf(scripts))
	}
	core, ok := scripts[boundaryRuntimeName]
	if !ok {
		t.Fatalf("scripts = %v, want the core module", keysOf(scripts))
	}
	// A dynamic import of a literal path, which is the shape the development
	// import takes and the only shape that can name a module the release set does
	// not contain. The check used to be for any dynamic import at all, which held
	// only while the development console was the sole reason to have one; the page
	// module loader of requirement:client-signal-registry imports a URL an
	// application's own markup names, and that is a variable rather than a
	// literal. Narrowed rather than removed, because what this protects is that a
	// deployed binary cannot be talked into loading a development module.
	if strings.Contains(core, `import("`) {
		t.Error("the release core carries a dynamic import of a literal path")
	}
	if strings.Contains(core, "dev.js") || strings.Contains(core, "PW_DEV_CONSOLE_URL") {
		t.Error("the release core names a development module")
	}
	if strings.Contains(core, "devmark.webp") || strings.Contains(core, "pw-dev-launcher") {
		t.Error("the release core names the launcher or its mark")
	}
}

// The set is JavaScript alone in a release build, so the only content type the
// handler can reach for is the one every module in it takes.
func TestAReleaseSetIsJavaScriptAlone(t *testing.T) {
	for name := range frameworkScripts() {
		if got := frameworkAssetContentType(name); got != "text/javascript; charset=utf-8" {
			t.Errorf("%s: Content-Type = %q, want JavaScript", name, got)
		}
	}
}

// The mark is embedded under the pwdev constraint, so a release build has no
// bytes for it and 404s the name rather than serving something.
func TestTheMarkIsNotServedByAReleaseBuild(t *testing.T) {
	path := frameworkScriptPrefix + scriptRevision() + "/devmark.webp"
	if serveFrameworkScript(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, path, nil)) {
		t.Error("a release build served the launcher mark")
	}
}

// A name outside the set is declined here rather than answered, because the
// prefix holds more than the modules — the redraw endpoint is under it too, and
// claiming the whole namespace here would swallow it. What nobody claims is
// closed by serveReservedPath, below every one of them, so an unknown name
// still never reaches the application.
func TestUnknownFrameworkScriptIsNotFound(t *testing.T) {
	for _, path := range []string{
		frameworkScriptPrefix + scriptRevision() + "/dev.js",
		frameworkScriptPrefix + "0000000000000000/" + boundaryRuntimeName,
		frameworkScriptPrefix + scriptRevision() + "/nested/thing.js",
	} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		if serveFrameworkScript(httptest.NewRecorder(), request) {
			t.Errorf("%s: was served as a module of the set", path)
			continue
		}
		recorder := httptest.NewRecorder()
		if !serveReservedPath(recorder, request) {
			t.Errorf("%s: the reserved prefix was left open", path)
			continue
		}
		if recorder.Code != http.StatusNotFound {
			t.Errorf("%s: status = %d, want 404", path, recorder.Code)
		}
	}
}

func TestCoreModuleIsServedImmutably(t *testing.T) {
	recorder := httptest.NewRecorder()
	if !serveFrameworkScript(recorder, httptest.NewRequest(http.MethodGet, RuntimeScriptURL(), nil)) {
		t.Fatal("the runtime URL was not claimed")
	}
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	if got := recorder.Header().Get("Cache-Control"); !strings.Contains(got, "immutable") {
		t.Errorf("Cache-Control = %q, want an immutable response", got)
	}
}

func keysOf(scripts map[string]string) []string {
	names := make([]string, 0, len(scripts))
	for name := range scripts {
		names = append(names, name)
	}
	return names
}
