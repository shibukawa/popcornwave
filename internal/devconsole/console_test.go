package devconsole

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func startConsole(t *testing.T, panes ...Pane) *Console {
	t.Helper()
	console, err := New("127.0.0.1:0", Project{Name: "app", Environment: "dev"}, panes)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	t.Cleanup(console.Close)
	return console
}

func get(t *testing.T, url string) (int, string) {
	t.Helper()
	response, err := http.Get(url)
	if err != nil {
		t.Fatalf("get %s: %v", url, err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read %s: %v", url, err)
	}
	return response.StatusCode, string(body)
}

func TestConsoleServesTheIndexOnALoopbackPort(t *testing.T) {
	console := startConsole(t)
	if !strings.HasPrefix(console.URL(), "http://127.0.0.1:") {
		t.Fatalf("URL = %q, want a loopback address", console.URL())
	}
	status, body := get(t, console.URL()+"/")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	if !strings.Contains(body, "app") {
		t.Errorf("the index never named the project:\n%s", body)
	}
}

func TestIndexNamesADisabledPaneAndTheKeyThatEnablesIt(t *testing.T) {
	console := startConsole(t,
		Pane{Slug: "assets", Title: "assets", DisabledBy: "dev.console.assets.enabled"},
		Pane{Slug: "telemetry", Title: "telemetry", Handler: http.NotFoundHandler()},
	)
	_, body := get(t, console.URL()+"/")
	if !strings.Contains(body, "dev.console.assets.enabled") {
		t.Errorf("a disabled pane was hidden rather than explained:\n%s", body)
	}
	if !strings.Contains(body, `href="/telemetry/"`) {
		t.Errorf("the index never linked the enabled pane:\n%s", body)
	}
}

func TestUndeterminedApplicationURLIsSaidRatherThanGuessed(t *testing.T) {
	console := startConsole(t)
	_, body := get(t, console.URL()+"/")
	if !strings.Contains(body, "undetermined") {
		t.Errorf("an unknown application address was not reported as unknown:\n%s", body)
	}
}

func TestPaneIsMountedUnderItsSlugWithThePrefixStripped(t *testing.T) {
	console := startConsole(t, Pane{Slug: "assets", Title: "assets",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, "path="+r.URL.Path)
		})})
	if _, body := get(t, console.URL()+"/assets/"); body != "path=/" {
		t.Errorf("body = %q, want the prefix stripped", body)
	}
	if _, body := get(t, console.URL()+"/assets/deep/file.css"); body != "path=/deep/file.css" {
		t.Errorf("body = %q, want the prefix stripped", body)
	}
}

// A pane's root paths exist for the telemetry bundle, which resolves its API
// against the document origin and so cannot follow its page under a prefix.
func TestPaneRootPathsAreServedAtTheConsoleRoot(t *testing.T) {
	console := startConsole(t, Pane{Slug: "telemetry", Title: "telemetry",
		RootPaths: []string{"/api/snapshot"},
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, "path="+r.URL.Path)
		})})
	if _, body := get(t, console.URL()+"/api/snapshot"); body != "path=/api/snapshot" {
		t.Errorf("body = %q, want the unstripped root path", body)
	}
}

func TestLoopStateStartsEmptyAndRecordsTransitions(t *testing.T) {
	console := startConsole(t)
	if state := console.State(); state.Phase != "" || state.Status != "" {
		t.Fatalf("state = %+v, want an empty one before any phase", state)
	}
	console.Publish("generating", StatusStarting, nil)
	console.Failed("generating", "handlers/home.go:12:3: undefined: Titel")

	state := console.State()
	if state.Status != StatusFailed {
		t.Errorf("status = %q, want %q", state.Status, StatusFailed)
	}
	if state.Diagnostic == nil || !strings.Contains(state.Diagnostic.Text, "undefined: Titel") {
		t.Fatalf("diagnostic = %+v, want the text unchanged", state.Diagnostic)
	}
	_, body := get(t, console.URL()+"/")
	if !strings.Contains(body, "undefined: Titel") {
		t.Errorf("the index never showed the diagnostic:\n%s", body)
	}
}

func TestHealthyClearsAPreviousDiagnosticAndAdvancesTheBuild(t *testing.T) {
	console := startConsole(t)
	console.Failed("building", "exit status 1")
	console.Publish("running", StatusHealthy, nil)

	first := console.State()
	if first.Diagnostic != nil {
		t.Errorf("diagnostic = %+v, want a healthy transition to clear it", first.Diagnostic)
	}
	if first.Build == "" {
		t.Fatal("a healthy transition left the build identity empty")
	}
	// A restart is what makes an open page stale, so the identity moves on the
	// transition back to healthy and not on every publish.
	console.Publish("running", StatusHealthy, nil)
	if console.State().Build != first.Build {
		t.Error("the build identity moved without a restart")
	}
	console.Failed("building", "exit status 1")
	console.Publish("running", StatusHealthy, nil)
	if console.State().Build == first.Build {
		t.Error("the build identity did not move across a restart")
	}
}

func TestLoopStateIsReadableAsJSON(t *testing.T) {
	console := startConsole(t)
	console.Failed("applying migrations", "no Down for 003_add_index.sql")

	_, body := get(t, console.URL()+"/api/loop-state")
	var state State
	if err := json.Unmarshal([]byte(body), &state); err != nil {
		t.Fatalf("decode %q: %v", body, err)
	}
	if state.Phase != "applying migrations" || state.Status != StatusFailed {
		t.Errorf("state = %+v, want the failed migration phase", state)
	}
	if state.Diagnostic == nil || state.Diagnostic.Text != "no Down for 003_add_index.sql" {
		t.Errorf("diagnostic = %+v, want the text unchanged", state.Diagnostic)
	}
}

// A console that could not listen is an ordinary outcome, and every call site
// in the loop would otherwise need a branch for it.
func TestNilConsoleToleratesEveryCall(t *testing.T) {
	var console *Console
	console.Publish("generating", StatusStarting, nil)
	console.Failed("generating", "boom")
	console.Close()
	if console.URL() != "" || console.State().Phase != "" {
		t.Error("a nil console answered as though it were running")
	}
}
