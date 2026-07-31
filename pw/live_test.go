package pw

import (
	"bufio"
	"context"
	"errors"
	"iter"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/shibukawa/tinybind-go/htmlbind"
)

// livePage builds the shape generation emits for a component whose await clause
// binds a live source: the same boundary, with a binding that keeps delivering.
func livePage(source func(context.Context) iter.Seq2[string, error]) htmlbind.Fragment {
	return htmlbind.Bind(livePlan(source, "<main>", "</main>"), struct{}{})
}

func livePlan(source func(context.Context) iter.Seq2[string, error], open, close string) *htmlbind.Plan[struct{}] {
	outer := htmlbind.Builder[struct{}]{}
	return &htmlbind.Plan[struct{}]{
		HasAwaitBlock: true,
		HasLiveBlock:  true,
		Ops: []htmlbind.Op[struct{}]{
			outer.Static(open),
			liveOp(source),
			outer.Static(close),
		},
	}
}

func liveOp(source func(context.Context) iter.Seq2[string, error]) htmlbind.Op[struct{}] {
	outer := htmlbind.Builder[struct{}]{}
	primary := htmlbind.Builder[string]{}
	return htmlbind.Live(
		func(ctx context.Context, _ struct{}) []htmlbind.LiveBinding[string] {
			return []htmlbind.LiveBinding[string]{
				func(deliver func(func(*string), error) bool) error {
					for value, err := range source(ctx) {
						delivered := value
						if !deliver(func(scope *string) { *scope = delivered }, err) {
							return nil
						}
					}
					return nil
				},
			}
		},
		func(struct{}) string { return "" },
		func(_ struct{}, err AsyncError) AsyncError { return err },
		[]htmlbind.Op[string]{
			primary.Static("<p>"),
			primary.Text(func(value string) string { return value }),
			primary.Static("</p>"),
		},
		[]htmlbind.Op[struct{}]{outer.Static("<p>waiting</p>")},
		nil,
	)
}

// liveValues yields each item and then ends, which is a source that finishes.
func liveValues(items ...string) func(context.Context) iter.Seq2[string, error] {
	return func(ctx context.Context) iter.Seq2[string, error] {
		return func(yield func(string, error) bool) {
			for _, item := range items {
				select {
				case <-ctx.Done():
					return
				default:
				}
				if !yield(item, nil) {
					return
				}
			}
		}
	}
}

// liveThenQuiet delivers once and then says nothing until its context ends,
// which is the shape of a real watch between events.
func liveThenQuiet(first string) func(context.Context) iter.Seq2[string, error] {
	return func(ctx context.Context) iter.Seq2[string, error] {
		return func(yield func(string, error) bool) {
			if !yield(first, nil) {
				return
			}
			<-ctx.Done()
		}
	}
}

func liveRequest(target string) *http.Request {
	request := browserRequest(target)
	request.Header.Set(ResponseModeHeader, LiveResponseMode)
	return request
}

func recordLines(t *testing.T, body string) []string {
	t.Helper()
	lines := []string{}
	for _, line := range strings.Split(body, "\n") {
		if strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

// A document holding a live boundary ends by saying so, because a client that
// cannot tell a finished screen from a continuing one either never updates or
// pays a whole page execution to find out there was nothing to deliver.
func TestStreamedDocumentMarksLiveWorkRemaining(t *testing.T) {
	recorder := httptest.NewRecorder()
	WriteHTML(recorder, browserRequest("/"), livePage(liveThenQuiet("first")))

	body := recorder.Body.String()
	if !strings.Contains(body, `<tb-stream-end state="live"`) {
		t.Fatalf("document does not mark live work remaining:\n%s", body)
	}
	if !strings.HasSuffix(strings.TrimSpace(body), "</tb-stream-end>") {
		t.Errorf("the marker is not the last thing written:\n%s", body)
	}
	// The first delivery commits as an ordinary completion, so the first paint
	// shows content rather than a loading state.
	if !strings.Contains(body, "<p>first</p>") {
		t.Errorf("the first delivery did not commit:\n%s", body)
	}
	if recorder.Header().Get("Vary") == "" || !strings.Contains(recorder.Header().Get("Vary"), ResponseModeHeader) {
		t.Errorf("Vary = %q, want the response mode header", recorder.Header().Values("Vary"))
	}
}

// A page whose boundaries all settle says the opposite, so the screen costs no
// speculative request.
func TestStreamedDocumentMarksItselfFinal(t *testing.T) {
	recorder := httptest.NewRecorder()
	WriteHTML(recorder, browserRequest("/"), asyncPage(asyncPageParams{Body: Resolved("ready")}))

	body := recorder.Body.String()
	if !strings.Contains(body, `<tb-stream-end state="final"`) {
		t.Fatalf("document does not mark itself final:\n%s", body)
	}
	for _, header := range recorder.Header().Values("Vary") {
		if strings.Contains(header, ResponseModeHeader) {
			t.Errorf("a page with nothing live varies on the mode header: %q", header)
		}
	}
}

// A response that gave up on its own document says that too: the fallbacks it
// committed are not going to be replaced by it.
func TestStreamedDocumentMarksItselfFailed(t *testing.T) {
	recorder := httptest.NewRecorder()
	WriteHTML(recorder, browserRequest("/"), noRecoverPage(asyncPageParams{Body: Failed[string](errors.New("upstream is down"))}))

	body := recorder.Body.String()
	if !strings.Contains(body, `<tb-stream-end state="failed"`) {
		t.Fatalf("document does not mark itself failed:\n%s", body)
	}
}

// The buffered branch settles its live boundaries in place and writes no
// placeholder, so it carries no marker and stays byte-identical to what a
// crawler received before live boundaries existed.
func TestBufferedDocumentCarriesNoMarker(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := browserRequest("/")
	request = request.WithContext(withTestHTMLConfig(request.Context(), HTMLConfig{}))
	WriteHTML(recorder, request, livePage(liveValues("first")))

	body := recorder.Body.String()
	if strings.Contains(body, "tb-stream-end") {
		t.Fatalf("a buffered document carries a marker:\n%s", body)
	}
	if !strings.Contains(body, "<p>first</p>") {
		t.Errorf("the buffered branch did not render the first delivery:\n%s", body)
	}
}

func TestLiveModeStreamsEveryDelivery(t *testing.T) {
	recorder := httptest.NewRecorder()
	WriteHTML(recorder, liveRequest("/"), livePage(liveValues("one", "two", "three")))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}
	if got := recorder.Header().Get("Content-Type"); got != liveMediaType {
		t.Errorf("Content-Type = %q, want %q", got, liveMediaType)
	}
	if got := recorder.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", got)
	}
	lines := recordLines(t, recorder.Body.String())
	if len(lines) < 3 {
		t.Fatalf("records = %v", lines)
	}
	if !strings.HasPrefix(lines[0], `{"control":"open"`) {
		t.Errorf("first record = %q, want the open record", lines[0])
	}
	// No document markup reaches the stream: the body is executed and discarded,
	// so a reconnect repaints nothing that was not live.
	body := recorder.Body.String()
	if strings.Contains(body, "<main>") || strings.Contains(body, "tb-boundary") {
		t.Errorf("live response transferred document markup:\n%s", body)
	}
	deliveries := 0
	for _, line := range lines {
		if strings.Contains(line, `"id":"tb-1"`) {
			deliveries++
		}
	}
	if deliveries != 3 {
		t.Errorf("deliveries = %d, want 3 (one per value)", deliveries)
	}
	last := lines[len(lines)-1]
	if !strings.Contains(last, `"control":"closed"`) || !strings.Contains(last, `"reason":"done"`) {
		t.Errorf("last record = %q, want a done close", last)
	}
}

// A live request for a page with nothing live is answered rather than held
// open, and it is told not to come back.
func TestLiveModeClosesWhenNothingIsLive(t *testing.T) {
	recorder := httptest.NewRecorder()
	WriteHTML(recorder, liveRequest("/"), asyncPage(asyncPageParams{Body: Resolved("ready")}))

	lines := recordLines(t, recorder.Body.String())
	if len(lines) != 1 || !strings.Contains(lines[0], `"reason":"done"`) {
		t.Fatalf("records = %v, want one done close", lines)
	}
}

// Closing at the configured lifetime is what buys back authorization re-checks
// and deploy rollover, so it must not read as a failure: the client is expected
// straight back.
func TestLiveResponseClosesAtItsLifetimeForRetry(t *testing.T) {
	config := defaultHTMLConfig
	config.LiveMaxDuration = 40 * time.Millisecond
	config.LiveDurationJitter = 0
	config.LiveIdleTimeout = 0
	request := liveRequest("/")
	request = request.WithContext(withTestHTMLConfig(request.Context(), config))

	recorder := httptest.NewRecorder()
	WriteHTML(recorder, request, livePage(liveThenQuiet("first")))

	lines := recordLines(t, recorder.Body.String())
	last := lines[len(lines)-1]
	if !strings.Contains(last, `"reason":"retry"`) {
		t.Fatalf("last record = %q, want a retry close", last)
	}
	if !strings.Contains(last, `"retry_after_ms":`) {
		t.Errorf("a retry close carries no delay hint: %q", last)
	}
}

// A response nothing is delivering on is holding a goroutine, a source, and a
// connection for a screen that is learning nothing.
func TestLiveResponseClosesWhenIdle(t *testing.T) {
	config := defaultHTMLConfig
	config.LiveMaxDuration = 0
	config.LiveIdleTimeout = 40 * time.Millisecond
	request := liveRequest("/")
	request = request.WithContext(withTestHTMLConfig(request.Context(), config))

	recorder := httptest.NewRecorder()
	start := time.Now()
	WriteHTML(recorder, request, livePage(liveThenQuiet("first")))

	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("idle response stayed open for %s", elapsed)
	}
	lines := recordLines(t, recorder.Body.String())
	last := lines[len(lines)-1]
	if !strings.Contains(last, `"reason":"retry"`) {
		t.Fatalf("last record = %q, want a retry close", last)
	}
}

// Reaching the boundary bound is reported and closes the response, because a
// screen quietly missing one panel's updates is worse than one that stops.
func TestLiveResponseStopsAtItsBoundaryBound(t *testing.T) {
	config := defaultHTMLConfig
	config.LiveMaxBoundaries = 1
	request := liveRequest("/")
	request = request.WithContext(withTestHTMLConfig(request.Context(), config))

	outer := htmlbind.Builder[struct{}]{}
	plan := &htmlbind.Plan[struct{}]{
		HasAwaitBlock: true,
		HasLiveBlock:  true,
		Ops: []htmlbind.Op[struct{}]{
			outer.Static("<main>"),
			liveOp(liveThenQuiet("left")),
			liveOp(liveThenQuiet("right")),
			outer.Static("</main>"),
		},
	}

	recorder := httptest.NewRecorder()
	WriteHTML(recorder, request, htmlbind.Bind(plan, struct{}{}))

	lines := recordLines(t, recorder.Body.String())
	ids := map[string]bool{}
	for _, line := range lines {
		if strings.Contains(line, `"id":"tb-1"`) {
			ids["tb-1"] = true
		}
		if strings.Contains(line, `"id":"tb-2"`) {
			ids["tb-2"] = true
		}
	}
	if len(ids) != 1 {
		t.Fatalf("served %d boundaries under a bound of 1: %v", len(ids), lines)
	}
	if last := lines[len(lines)-1]; !strings.Contains(last, `"control":"closed"`) {
		t.Errorf("last record = %q, want a close", last)
	}
}

// A bound on one response buys nothing against a client that opens ten.
func TestLiveResponsesAreBoundedPerClient(t *testing.T) {
	config := defaultHTMLConfig
	config.LiveMaxResponses = 1
	config.LiveMaxDuration = 0
	config.LiveIdleTimeout = 0

	held, holding := context.WithCancel(context.Background())
	defer holding()
	var open sync.WaitGroup
	open.Add(1)
	var first sync.WaitGroup
	first.Add(1)
	go func() {
		defer first.Done()
		request := liveRequest("/")
		ctx := withTestHTMLConfig(held, config)
		request = request.WithContext(ctx)
		recorder := httptest.NewRecorder()
		WriteHTML(recorder, request, livePage(func(ctx context.Context) iter.Seq2[string, error] {
			return func(yield func(string, error) bool) {
				if !yield("first", nil) {
					return
				}
				open.Done()
				<-ctx.Done()
			}
		}))
	}()
	open.Wait()

	request := liveRequest("/")
	request = request.WithContext(withTestHTMLConfig(request.Context(), config))
	recorder := httptest.NewRecorder()
	WriteHTML(recorder, request, livePage(liveThenQuiet("second")))

	lines := recordLines(t, recorder.Body.String())
	if len(lines) != 1 || !strings.Contains(lines[0], `"reason":"retry"`) {
		t.Fatalf("records = %v, want one refusal asking for a retry", lines)
	}
	holding()
	first.Wait()
}

// Live delivery depends on the placeholders only the streaming branch writes,
// so disabling streaming disables it rather than leaving a client connected to
// a stream whose ids address nothing.
func TestStreamingDisabledRefusesLiveMode(t *testing.T) {
	config := defaultHTMLConfig
	config.Streaming = false
	request := liveRequest("/")
	request = request.WithContext(withTestHTMLConfig(request.Context(), config))

	recorder := httptest.NewRecorder()
	WriteHTML(recorder, request, livePage(liveThenQuiet("first")))

	lines := recordLines(t, recorder.Body.String())
	if len(lines) != 1 || !strings.Contains(lines[0], `"reason":"done"`) {
		t.Fatalf("records = %v, want one done close", lines)
	}
}

// The runtime and the framing are one design, so the script has to carry the
// half that reads these records.
func TestRuntimeScriptCarriesTheLiveHalf(t *testing.T) {
	for _, fragment := range []string{
		`customElements.define("tb-stream-end"`,
		"Pw-Response-Mode",
		"export function stopLive",
		// Completion is decided from readyState and the marker together: one
		// says when the question can be answered, the other what the answer is.
		// Keying on DOMContentLoaded instead would ask before the subresources
		// a complete document waits for, and keying on the marker alone cannot
		// tell a truncated document from one still arriving.
		`document.addEventListener("readystatechange"`,
		`document.readyState !== "complete"`,
	} {
		if !strings.Contains(boundaryRuntimeScript, fragment) {
			t.Errorf("the runtime script does not carry %q", fragment)
		}
	}
}

// A delivery has to reach the client while the source is still producing.
// A recorder cannot show that: only a real connection can say whether the
// records were flushed as they were written or held until the response ended.
func TestLiveDeliveriesArriveWhileTheSourceIsStillProducing(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		WriteHTML(w, r, livePage(func(ctx context.Context) iter.Seq2[string, error] {
			return func(yield func(string, error) bool) {
				if !yield("first", nil) {
					return
				}
				select {
				case <-release:
				case <-ctx.Done():
					return
				}
				yield("second", nil)
			}
		}))
	}))
	defer server.Close()

	request, err := http.NewRequest(http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("User-Agent", chromeUserAgent)
	request.Header.Set(ResponseModeHeader, LiveResponseMode)
	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()

	reader := bufio.NewReader(response.Body)
	// The open record and the first delivery both arrive while the source is
	// blocked, so reading them proves nothing waited for the stream to end.
	for _, want := range []string{`"control":"open"`, `"id":"tb-1"`} {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("reading %s: %v", want, err)
		}
		if !strings.Contains(line, want) {
			t.Fatalf("record = %q, want one containing %s", line, want)
		}
	}
	if !strings.Contains(response.Header.Get("Content-Type"), liveMediaType) {
		t.Errorf("Content-Type = %q", response.Header.Get("Content-Type"))
	}
}

// A subscription must not outlive the client that opened it: the source is a
// goroutine, and the only thing that can stop an endless one is the context it
// was given.
func TestLiveSourceStopsWhenTheClientDisconnects(t *testing.T) {
	observed := make(chan struct{})
	served := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(served)
		WriteHTML(w, r, livePage(func(ctx context.Context) iter.Seq2[string, error] {
			return func(yield func(string, error) bool) {
				if !yield("first", nil) {
					return
				}
				<-ctx.Done()
				close(observed)
			}
		}))
	}))
	defer server.Close()

	request, err := http.NewRequest(http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("User-Agent", chromeUserAgent)
	request.Header.Set(ResponseModeHeader, LiveResponseMode)
	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	reader := bufio.NewReader(response.Body)
	if _, err := reader.ReadString('\n'); err != nil {
		t.Fatal(err)
	}
	response.Body.Close()

	select {
	case <-observed:
	case <-time.After(5 * time.Second):
		t.Fatal("the source never observed the disconnect")
	}
	select {
	case <-served:
	case <-time.After(5 * time.Second):
		t.Fatal("the handler never returned")
	}
}

// Defining a custom element upgrades the ones the parser already inserted,
// synchronously, inside the define call. A callback that runs there and reads a
// module binding declared further down the script reads it before its
// initializer has run and throws — which is silent, because it happens during
// load and leaves a page that simply never updates.
//
// The document marker is always already in the DOM when its element is defined,
// so this is the ordinary path rather than a race, and the ordering is worth a
// test rather than a comment alone.
func TestRuntimeScriptDeclaresItsStateBeforeDefiningElements(t *testing.T) {
	firstDefine := strings.Index(boundaryRuntimeScript, "customElements.define(")
	if firstDefine < 0 {
		t.Fatal("the runtime script defines no custom element")
	}
	for offset := firstDefine; ; {
		index := strings.Index(boundaryRuntimeScript[offset:], "\nlet ")
		if index < 0 {
			return
		}
		offset += index + 1
		line := boundaryRuntimeScript[offset:]
		if end := strings.IndexByte(line, '\n'); end >= 0 {
			line = line[:end]
		}
		t.Errorf("module state is declared after the first customElements.define: %q", line)
	}
}
