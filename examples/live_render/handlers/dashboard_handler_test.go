package handlers

import (
	"bufio"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	// The generated document shell registers itself during init, the way the
	// generated bootstrap links it into the binary.
	_ "live_render/templates"
)

const chromeUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 " +
	"(KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36"

func browserRequest(target string) *http.Request {
	request := httptest.NewRequest(http.MethodGet, target, nil)
	request.Header.Set("User-Agent", chromeUserAgent)
	return request
}

// The document is what a reader gets before any live delivery exists, and it
// has to be a complete page on its own — plus an invitation to connect.
func TestDashboardDocumentInvitesALiveConnection(t *testing.T) {
	recorder := httptest.NewRecorder()
	dashboard(recorder, browserRequest("/"))

	body := recorder.Body.String()
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}
	if !strings.Contains(body, `<tb-stream-end state="live"`) {
		t.Fatalf("the document does not say live work remains:\n%s", body)
	}
	// The static panel belongs to the first pass, so it is on the wire before
	// anything a boundary produced.
	static := strings.Index(body, "This panel is here to be ignored")
	fallback := strings.Index(body, "sampling")
	if static < 0 || fallback < 0 || static > fallback {
		t.Errorf("the static panel did not precede the fallbacks")
	}
	// One clause holds a settle-once binding and a live one, and the boundary
	// renders only once both have a value: the fetched title and the message
	// list arrive together or not at all. An empty room is a value like any
	// other, so the list may legitimately be empty here.
	// The opening tag is matched without its ">" because the element carries
	// classes: an assertion that reads a whole tag breaks on styling, which says
	// nothing about the binding it is here to check.
	if strings.Contains(body, "#general") != strings.Contains(body, "<ul") {
		t.Errorf("the mixed clause rendered one binding without the other:\n%s", body)
	}
}

// A client that runs no script receives the settled document: one real render
// of every live region rather than a placeholder that nothing will replace.
func TestDashboardAnswersANonBrowserClientWithContent(t *testing.T) {
	recorder := httptest.NewRecorder()
	dashboard(recorder, httptest.NewRequest(http.MethodGet, "/", nil))

	body := recorder.Body.String()
	if strings.Contains(body, "tb-boundary") || strings.Contains(body, "tb-stream-end") {
		t.Fatalf("a non-browser client received boundary framing:\n%s", body)
	}
	if !strings.Contains(body, "requests/s") {
		t.Errorf("the gauge did not render its first delivery in place:\n%s", body)
	}
}

// The live connection is the same request to the same URL, and this is the
// property the example exists to show: deliveries arrive while the sources are
// still producing, and no document markup travels with them.
func TestDashboardStreamsDeliveriesOnTheSameURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(dashboard))
	defer server.Close()

	request, err := http.NewRequest(http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("User-Agent", chromeUserAgent)
	request.Header.Set("Pw-Response-Mode", "live")
	client := &http.Client{Timeout: 10 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()

	if got := response.Header.Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", got)
	}
	// A live stream opens on the same record grammar every other update mode
	// uses: a head record first, then one record per delivery, then a terminator.
	// It used to open with a control record of its own, which was one transport
	// wearing two shapes.
	reader := bufio.NewReader(response.Body)
	open, err := reader.ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(open, `"r":"head"`) {
		t.Fatalf("first record = %q, want the head record", open)
	}
	// The first delivery is the room, which arrives as soon as both of its
	// bindings have a value; the gauge follows a second later.
	delivery, err := reader.ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(delivery, `"r":"await"`) || !strings.Contains(delivery, `"id":"tb-`) ||
		!strings.Contains(delivery, `"html":`) {
		t.Fatalf("second record = %q, want a delivery", delivery)
	}
	if strings.Contains(delivery, "This panel is here to be ignored") {
		t.Error("the live response transferred the static panel")
	}
}
