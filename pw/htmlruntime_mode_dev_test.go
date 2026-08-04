//go:build pwdev

package pw

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The set is resolved once per process, so a test that needs a console address
// has to be the one that sets it. These run in their own process because the
// build tag selects them.
func TestDevelopmentModuleIsServedBesideTheCore(t *testing.T) {
	if developmentConsoleURL() == "" {
		t.Skip("no console address in the environment; see TestMain")
	}
	scripts := frameworkScripts()
	if _, ok := scripts[developmentModuleName]; !ok {
		t.Fatalf("scripts = %v, want the development module", scripts)
	}
	core := scripts[boundaryRuntimeName]
	if !strings.Contains(core, `import("./`+developmentModuleName+`")`) {
		t.Error("the core does not import the development module")
	}
	// The import is relative, so it resolves inside the revision directory the
	// core was served from and nothing has to rewrite it.
	if strings.Contains(core, `import("/`) {
		t.Error("the development import is absolute; it must stay relative to the revision directory")
	}
}

func TestDevelopmentModuleIsFetchable(t *testing.T) {
	if developmentConsoleURL() == "" {
		t.Skip("no console address in the environment")
	}
	recorder := httptest.NewRecorder()
	path := frameworkScriptURL(developmentModuleName)
	if !serveFrameworkScript(recorder, httptest.NewRequest(http.MethodGet, path, nil)) {
		t.Fatalf("%s was not claimed", path)
	}
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, developmentConsoleURL()) {
		t.Error("the console address was not baked into the served module")
	}
	if !strings.Contains(body, "/api/loop-state") {
		t.Error("the module does not read the loop state")
	}
}

// The address is quoted as JSON rather than concatenated, so a value carrying a
// quote cannot close the string and continue as code.
func TestConsoleAddressIsQuotedIntoTheModule(t *testing.T) {
	module := developmentModule(`http://x"+alert(1)+"`)
	if strings.Contains(module, `"+alert(1)+"`) {
		t.Errorf("the address was spliced in unquoted:\n%s", module)
	}
	if !strings.Contains(module, `\"`) {
		t.Errorf("the address was not escaped:\n%s", module)
	}
}

// With no console running there is nothing for the module to talk to, so the
// set and the revision are the release ones.
func TestNoConsoleMeansNoDevelopmentModule(t *testing.T) {
	t.Setenv(DevConsoleURLVar, "")
	if developmentImport() != "" {
		t.Error("an import was emitted with no console address")
	}
	if scripts := developmentScripts(); len(scripts) != 0 {
		t.Errorf("scripts = %v, want none", scripts)
	}
}
