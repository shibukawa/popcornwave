package devconsole

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

func startConsole(t *testing.T, panes ...Pane) *Console {
	t.Helper()
	console, err := New("127.0.0.1:0", Project{Name: "app", Environment: "dev"}, panes, nil)
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

// The stream sends what is true now before it waits, so a page that connects
// after the transition it cares about is still told about it.
func TestStreamSendsTheCurrentStateBeforeWaiting(t *testing.T) {
	console := startConsole(t)
	console.Failed("generating", "undefined: Titel")

	state := readStreamEvent(t, console, nil)
	if state.Status != StatusFailed || state.Diagnostic == nil {
		t.Fatalf("first event = %+v, want the failure already recorded", state)
	}
}

func TestStreamPushesLaterTransitions(t *testing.T) {
	console := startConsole(t)
	console.Publish("starting services", StatusStarting, nil)

	state := readStreamEvent(t, console, func() {
		console.Failed("building CSS", "tailwindcss: exit status 1")
	})
	if state.Status != StatusFailed || state.Phase != "building CSS" {
		t.Fatalf("pushed event = %+v, want the CSS failure", state)
	}
}

// A page served by the application is a different origin to the console, so a
// stream it cannot read is a stream that does not work.
func TestStreamAllowsALoopbackOrigin(t *testing.T) {
	console := startConsole(t)
	request, err := http.NewRequest(http.MethodGet, console.URL()+"/api/loop-state/stream", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Origin", "http://127.0.0.1:8080")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if got := response.Header.Get("Access-Control-Allow-Origin"); got != "http://127.0.0.1:8080" {
		t.Errorf("Access-Control-Allow-Origin = %q, want the loopback origin echoed", got)
	}
	if got := response.Header.Get("Content-Type"); !strings.HasPrefix(got, "text/event-stream") {
		t.Errorf("Content-Type = %q, want an event stream", got)
	}
}

func TestIndexLinksTheAPIDocumentationTheApplicationServes(t *testing.T) {
	console, err := New("127.0.0.1:0", Project{
		Name: "app", Environment: "dev",
		ApplicationURL: "http://localhost:8080",
		APIDocURL:      "http://localhost:8080/reference",
		APIDocKey:      "server.api_doc",
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(console.Close)
	_, body := get(t, console.URL()+"/")
	if !strings.Contains(body, "http://localhost:8080/reference") {
		t.Errorf("the index never linked the documentation:\n%s", body)
	}
}

func TestIndexNamesTheKeyWhenTheAPIDocumentationIsOff(t *testing.T) {
	console, err := New("127.0.0.1:0", Project{
		Name: "app", Environment: "dev", APIDocKey: "server.api_doc",
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(console.Close)
	_, body := get(t, console.URL()+"/")
	if !strings.Contains(body, "server.api_doc") {
		t.Errorf("the index never named the key that enables it:\n%s", body)
	}
}

// readStreamEvent opens the stream, runs during if given, and returns the last
// state the stream delivered.
func readStreamEvent(t *testing.T, console *Console, during func()) State {
	t.Helper()
	response, err := http.Get(console.URL() + "/api/loop-state/stream")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { response.Body.Close() })
	reader := bufio.NewReader(response.Body)
	read := func() State {
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				t.Fatalf("read stream: %v", err)
			}
			payload, ok := strings.CutPrefix(strings.TrimSpace(line), "data: ")
			if !ok {
				continue
			}
			var state State
			if err := json.Unmarshal([]byte(payload), &state); err != nil {
				t.Fatalf("decode %q: %v", payload, err)
			}
			return state
		}
	}
	first := read()
	if during == nil {
		return first
	}
	during()
	return read()
}

// A project with no seed datasets is offered no action, rather than a button
// that fails when pressed.
func TestReseedIsOfferedOnlyWhenAvailable(t *testing.T) {
	console := startConsole(t)
	if console.CanReseed() {
		t.Error("reseed was offered before any action was installed")
	}
	if _, body := get(t, console.URL()+"/"); strings.Contains(body, "reseed") {
		t.Errorf("the index offered reseed with no action:\n%s", body)
	}
	console.SetReseed(func(context.Context) error { return nil })
	if _, body := get(t, console.URL()+"/"); !strings.Contains(body, "reseed") {
		t.Errorf("the index did not offer reseed:\n%s", body)
	}
}

func TestReseedRunsTheActionAndReportsIt(t *testing.T) {
	console := startConsole(t)
	ran := false
	console.SetReseed(func(context.Context) error { ran = true; return nil })

	response, err := http.Post(console.URL()+"/api/reseed", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if !ran {
		t.Error("the reseed action was not run")
	}
	if location := response.Request.URL.Query().Get("seeded"); location == "" {
		// The client follows the redirect, so the landing URL carries the result.
		t.Errorf("landed at %s, want the seeded marker", response.Request.URL)
	}
}

// A failing reseed reports why on the index rather than leaving the developer
// to check the terminal.
func TestReseedReportsAFailure(t *testing.T) {
	console := startConsole(t)
	console.SetReseed(func(context.Context) error { return errors.New("dataset users.yaml: no such table") })

	response, err := http.Post(console.URL()+"/api/reseed", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(response.Body)
	if !strings.Contains(string(body), "no such table") {
		t.Errorf("the failure was not reported:\n%s", body)
	}
}
