package handlers

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	// The generated document shell registers itself during init, the way the
	// generated bootstrap links it into the binary. Only the page route needs
	// it; the fragment routes are what prove they do not.
	_ "htmx_fragment/templates"
)

// reset restores the seeded board, because every handler here writes to one
// package-level store.
func reset(t *testing.T) {
	t.Helper()
	tasks.mu.Lock()
	defer tasks.mu.Unlock()
	tasks.nextID = 4
	tasks.tasks = []Task{
		{Id: "1", Title: "Draft the release notes", Owner: "ada", Priority: "high"},
		{Id: "2", Title: "Review the fragment guide", Owner: "grace", Priority: "normal"},
		{Id: "3", Title: "Archive last quarter's board", Owner: "ada", Priority: "low"},
	}
}

func do(t *testing.T, request *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	// Routing through the mux is what binds {id}, so these tests exercise the
	// same path a browser reaches.
	Handlers().ServeHTTP(recorder, request)
	return recorder
}

func form(t *testing.T, method, target string, values url.Values) *http.Request {
	t.Helper()
	request := httptest.NewRequest(method, target, strings.NewReader(values.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return request
}

// TestPageCarriesTheDocument is the contrast the rest of the file is measured
// against: one route answers with a whole document, and it is the only one.
func TestPageCarriesTheDocument(t *testing.T) {
	reset(t)
	recorder := do(t, httptest.NewRequest(http.MethodGet, "/", nil))

	body := recorder.Body.String()
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}
	if !strings.HasPrefix(body, "<!doctype html>") {
		t.Fatalf("the page is not a document: %q", firstBytes(body))
	}
	if !strings.Contains(body, "htmx.org@2.0.10") {
		t.Error("the document does not load htmx")
	}
	if !strings.Contains(body, `id="task-list"`) {
		t.Error("the page did not compose the list component the partials re-render")
	}
}

// TestListFragmentIsTheRegionAlone asserts the property the example exists to
// show: the response is the swap target's markup, with no document around it.
func TestListFragmentIsTheRegionAlone(t *testing.T) {
	reset(t)
	recorder := do(t, httptest.NewRequest(http.MethodGet, "/tasks?q=ada", nil))

	body := recorder.Body.String()
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}
	for _, unwanted := range []string{"<!doctype", "<html", "<head", "<body", "htmx.org"} {
		if strings.Contains(strings.ToLower(body), unwanted) {
			t.Errorf("a fragment carried %q: %q", unwanted, firstBytes(body))
		}
	}
	if !strings.Contains(body, "Draft the release notes") || strings.Contains(body, "Review the fragment guide") {
		t.Errorf("the filter did not apply: %q", body)
	}
	// Buffered like the non-streaming page branch, and classified as nothing:
	// one branch means one representation, so no client heuristic runs here.
	if recorder.Header().Get("Content-Length") == "" {
		t.Error("a fragment is buffered, so it declares a length")
	}
	if got := strings.Join(recorder.Header().Values("Vary"), ","); strings.Contains(got, "User-Agent") {
		t.Errorf("Vary = %q, want no client classification on this path", got)
	}
}

// TestRejectedFormComesBackAsHTML covers the status decision in createTask: a
// typo is answered with the re-rendered form, because htmx swaps only 2xx and a
// problem document would leave the page silent.
func TestRejectedFormComesBackAsHTML(t *testing.T) {
	reset(t)
	recorder := do(t, form(t, http.MethodPost, "/tasks", url.Values{
		"title":    {""},
		"owner":    {"ada"},
		"priority": {"high"},
	}))

	body := recorder.Body.String()
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want the form back", recorder.Code)
	}
	if got := recorder.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Errorf("content type = %q", got)
	}
	if !strings.Contains(body, `<p class="error">`) {
		t.Errorf("no field error reached the form: %q", body)
	}
	// What the operator typed survives the round trip.
	if !strings.Contains(body, `name="owner" value="ada"`) {
		t.Errorf("the entered owner was dropped: %q", body)
	}
	if total, _ := tasks.counts(); total != 3 {
		t.Errorf("a rejected submission wrote a task: total = %d", total)
	}
}

func TestAcceptedFormSwapsThePanelBack(t *testing.T) {
	reset(t)
	recorder := do(t, form(t, http.MethodPost, "/tasks", url.Values{
		"title":    {"Ship the htmx example"},
		"owner":    {"ada"},
		"priority": {"high"},
		"q":        {"ada"},
	}))

	body := recorder.Body.String()
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}
	if !strings.HasPrefix(strings.TrimSpace(body), `<section id="task-panel"`) {
		t.Fatalf("the response is not the swap target alone: %q", firstBytes(body))
	}
	if !strings.Contains(body, "Ship the htmx example") {
		t.Error("the new task is missing from the returned list")
	}
	// The filter travelled with the submission, so the returned list is still
	// the filtered one.
	if strings.Contains(body, "Review the fragment guide") {
		t.Error("the response ignored the filter the form carried")
	}
	if !strings.Contains(body, `<p class="note">`) {
		t.Error("the confirmation note is missing")
	}
}

func TestRemoveAnswersTheListAndRejectsAStaleClick(t *testing.T) {
	reset(t)
	recorder := do(t, httptest.NewRequest(http.MethodDelete, "/tasks/2?q=", nil))

	body := recorder.Body.String()
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}
	if strings.Contains(body, "Review the fragment guide") {
		t.Errorf("the removed task is still listed: %q", body)
	}

	stale := do(t, httptest.NewRequest(http.MethodDelete, "/tasks/2", nil))
	if stale.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 for a row that is already gone", stale.Code)
	}
	if got := stale.Header().Get("Content-Type"); got != "application/problem+json" {
		t.Errorf("content type = %q", got)
	}
}

// TestSummaryArrivesSettled is the async property on this path: the boundary
// blocks here rather than streaming, so the markup is finished and carries
// neither the fallback nor anything a client runtime would have to apply.
func TestSummaryArrivesSettled(t *testing.T) {
	reset(t)
	recorder := do(t, httptest.NewRequest(http.MethodGet, "/tasks/summary", nil))

	body := recorder.Body.String()
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}
	if !strings.Contains(body, "3 tasks · 1 of them high priority") {
		t.Fatalf("the boundary did not settle in place: %q", body)
	}
	if strings.Contains(body, "Counting…") {
		t.Error("a settled fragment shipped its fallback")
	}
	for _, framing := range []string{"tb-boundary", "tb-apply", "tb-runtime"} {
		if strings.Contains(body, framing) {
			t.Errorf("a fragment carried %q framing: %q", framing, body)
		}
	}
	if recorder.Header().Get("Content-Length") == "" {
		t.Error("an await-capable fragment is still buffered, so it declares a length")
	}
}

// TestHeadContributionsAreRejected keeps the guardrail visible: a fragment has
// no head, so a scoped style block fails loudly instead of arriving unstyled.
func TestHeadContributionsAreRejected(t *testing.T) {
	reset(t)
	recorder := do(t, httptest.NewRequest(http.MethodGet, "/tasks/broken", nil))

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want the rejection", recorder.Code)
	}
	if got := recorder.Header().Get("Content-Type"); got != "application/problem+json" {
		t.Errorf("content type = %q", got)
	}
	if strings.Contains(recorder.Body.String(), "demo-badge") {
		t.Error("the fragment was rendered anyway")
	}
}

func firstBytes(body string) string {
	if len(body) > 200 {
		return body[:200] + "…"
	}
	return body
}
