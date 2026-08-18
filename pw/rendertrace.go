package pw

import (
	"context"
	"io"
	"net/http"
	"time"

	"github.com/shibukawa/popcornweb/contrib/otel/trace"
	"github.com/shibukawa/popcornweb/pwruntime"
	"github.com/shibukawa/tinybind-go/htmlbind"
)

// Render span names.
//
// The mode is part of the parent's name rather than only an attribute, because
// a waterfall is read by its labels: "render stream" and "render redraw" are
// different shapes of work and a reader should not have to open a span to learn
// which one this was. The set is closed and small, so naming it costs no
// cardinality.
const (
	renderSpanPrefix   = "render "
	initialSpanName    = "render initial"
	boundarySpanName   = "render boundary"
	deliverySpanName   = "live delivery"
	signalEventName    = "live signal"
	renderModeBuffered = "buffered"
	renderModeStream   = "stream"
	renderModeLive     = "live"
	renderModeNavigate = "navigate"
	renderModeRedraw   = "redraw"
	renderModeFragment = "fragment"
)

// renderTrace is the span tree of one HTML response.
//
// It is nil whenever framework spans are off, and every method below tolerates
// that, so a call site carries no condition of its own. That is the whole point
// of the type: the render path already branches on streaming, on bots, and on
// three update modes, and an "if tracing" beside each of those would be the
// thing that eventually goes out of step.
//
// Nothing here is synchronized. htmlbind lets only the initial pass and the
// ranging caller write a response, and both are the goroutine serving the
// request; boundary work renders into its own buffer and never reaches this.
type renderTrace struct {
	// ctx carries the render span, so everything a render starts — a statement,
	// a nested span an application opens — lands under it rather than beside it.
	ctx  context.Context
	span *trace.Span
	// initial is the span of the first pass: the shell, the merged head, and
	// every fallback. It exists only where something follows it.
	initial *trace.Span
	// committed is when that pass finished, which is when every await boundary
	// started being waited for and the fallback went on screen.
	committed time.Time
	// boundary reports whether a settled boundary opens a span of its own.
	boundary bool
	// delivered is when each boundary last replaced itself, live only, so a
	// delivery span measures the interval that content held the screen.
	delivered  map[string]time.Time
	boundaries int
	bytes      int64
	// suppressed counts the deliveries this response did not write because the
	// region was already showing them, and suppressedBytes is what they would
	// have cost. Live only.
	suppressed      int
	suppressedBytes int64
	// signals counts the signals this response forwarded, and signalBytes their
	// payloads. Live only. They are counted because a screen driven entirely by
	// signals renders nothing, so every delivery number stays at zero and the
	// response is otherwise indistinguishable from one that did nothing at all.
	signals     int
	signalBytes int64
	// cache is what this response reused from the output cache. It is installed
	// on the context rather than reached from here, because the store is
	// consulted deep inside a generated plan and the only thing that reaches
	// there is the render context.
	cache *renderCacheCounts
	// metrics is the instrument set, nil when metrics are off. It is why this
	// type exists for a response with no span at all: a deployment sampling one
	// trace in ten still measures every render, so span and instrument are
	// installed independently and every method below tolerates either being
	// absent.
	metrics *pwruntime.Metrics
	// mode is the render mode, kept because it is the metric's one attribute and
	// the span carries it where a metric cannot read it.
	mode string
	// started is when the render began, which is what the duration histogram
	// measures. The span keeps its own start, and both come from one call.
	started time.Time
}

// renderInstruments returns the instrument set of ctx when the render group is
// recorded, or nil.
func renderInstruments(ctx context.Context) *pwruntime.Metrics {
	metrics := pwruntime.MetricPolicy(ctx)
	if metrics == nil || metrics.RenderDuration == nil {
		return nil
	}
	return metrics
}

// renderTraced reports whether this request opens render spans.
//
// It exists for the one caller that has to know before it starts a span: a
// redraw is recognized by negotiating the request, and negotiating it is work
// an untraced response should not do twice.
func renderTraced(ctx context.Context) bool { return renderPolicy(ctx) != nil }

// renderPolicy returns the span policy of ctx when it opens render spans.
func renderPolicy(ctx context.Context) *pwruntime.Tracing {
	policy := pwruntime.TracePolicy(ctx)
	if policy == nil || !policy.Render {
		return nil
	}
	return policy
}

// startRenderTrace opens the span of one response and returns the context that
// parents everything below it. It returns ctx unchanged and a nil trace when
// framework render spans are off, which is one nil comparison per response.
func startRenderTrace(ctx context.Context, mode string, attributes ...Attribute) (context.Context, *renderTrace) {
	policy, metrics := renderPolicy(ctx), renderInstruments(ctx)
	if policy == nil && metrics == nil {
		return ctx, nil
	}
	if policy == nil {
		return newRenderTrace(ctx, nil, metrics, mode, nil)
	}
	merged := make([]Attribute, 0, len(attributes)+1)
	merged = append(merged, attributes...)
	merged = append(merged, String("pw.render.mode", mode))
	return newRenderTrace(ctx, policy, metrics, mode, merged)
}

// startChainRenderTrace is startRenderTrace for a composed chain, whose four
// shape attributes every branch shares. Building them here, after the policy
// check, keeps an untraced response from assembling attributes nothing reads,
// and sizes the slice for the mode attribute in one allocation.
//
// Every value is a property of the templates rather than of the request, so
// none of it can carry an instance key, a component input, or anything a user
// supplied, which is what requirement:modern-observability asks of a dimension.
func startChainRenderTrace(ctx context.Context, mode string, layers int, async, live, bot bool) (context.Context, *renderTrace) {
	policy, metrics := renderPolicy(ctx), renderInstruments(ctx)
	if policy == nil && metrics == nil {
		return ctx, nil
	}
	if policy == nil {
		return newRenderTrace(ctx, nil, metrics, mode, nil)
	}
	return newRenderTrace(ctx, policy, metrics, mode, []Attribute{
		Int("pw.render.layers", layers),
		Bool("pw.render.async", async),
		Bool("pw.render.live", live),
		Bool("pw.render.bot", bot),
		String("pw.render.mode", mode),
	})
}

func newRenderTrace(ctx context.Context, policy *pwruntime.Tracing, metrics *pwruntime.Metrics, mode string, attributes []Attribute) (context.Context, *renderTrace) {
	spanCtx, boundary := ctx, false
	var span *trace.Span
	if policy != nil {
		spanCtx, span = trace.Start(ctx, renderSpanPrefix+mode, trace.WithAttributes(attributes...))
		boundary = policy.Boundary
	}
	now := time.Now()
	counts := &renderCacheCounts{}
	spanCtx = withRenderCacheCounts(spanCtx, counts)
	return spanCtx, &renderTrace{
		ctx:       spanCtx,
		span:      span,
		committed: now,
		started:   now,
		boundary:  boundary,
		cache:     counts,
		metrics:   metrics,
		mode:      mode,
	}
}

// initialBuild opens the child span covering everything written before the
// first boundary settles.
//
// Only a response with something after that pass opens it. On a buffered
// branch the whole render is the initial build, and a child span with its
// parent's exact extent would say nothing twice.
func (render *renderTrace) initialBuild() {
	if render == nil {
		return
	}
	_, render.initial = trace.Start(render.ctx, initialSpanName)
}

// commit closes the initial build. It is driven by the first flush rather than
// by a call at the right place, because the moment belongs to htmlbind: the
// initial pass writes the shell and the fallbacks and then flushes, and no
// other signal reaches this side of that call.
//
// Later flushes — one per settled boundary — find the span already ended, and
// Span.End is idempotent, so the first one wins without a flag of its own.
func (render *renderTrace) commit() {
	if render == nil || render.initial == nil {
		return
	}
	render.committed = time.Now()
	render.initial.SetAttributes(Int64("pw.render.bytes", render.bytes))
	render.initial.End()
	render.initial = nil
}

// boundarySettled records one await boundary arriving.
//
// The span runs from the commit to the completion, which is not how long the
// boundary's own work took: it includes waiting for a concurrency slot, and for
// a nested boundary it starts before its parent existed. It is the more useful
// measurement anyway, and the one nothing else reports — it is exactly how long
// that fallback sat on screen.
// The size is the boundary's own fragment and is not added to the response
// total: this branch writes through the wrapper below, which counts the framing
// around the fragment as well.
func (render *renderTrace) boundarySettled(id string, size int) {
	if render == nil {
		return
	}
	render.boundaries++
	// The metric carries no boundary id: positional is safe on a span, where one
	// page is being read, and unbounded across pages, where every placeholder of
	// every route would be its own series.
	if render.metrics != nil && render.metrics.BoundarySettle != nil {
		render.metrics.BoundarySettle.Record(render.ctx, time.Since(render.committed).Seconds())
	}
	if !render.boundary {
		return
	}
	_, span := trace.Start(render.ctx, boundarySpanName,
		trace.WithStartTime(render.committed),
		trace.WithAttributes(String("pw.boundary.id", id), Int("pw.boundary.bytes", size)))
	span.End()
}

// deliveredContent records one live delivery.
//
// Its span runs from the previous delivery of the same boundary, or from the
// stream opening for the first, so the waterfall reads as what a live screen
// actually does: each span is one stretch of content holding a region, ending
// when it was replaced. A delivery is observed only on arrival, so this is the
// one interval around it that is measured rather than guessed.
func (render *renderTrace) deliveredContent(id string, size int) {
	if render == nil {
		return
	}
	render.boundaries++
	since := render.committed
	if previous, ok := render.delivered[id]; ok {
		since = previous
	}
	now := time.Now()
	// The interval is recorded whether or not a boundary span is opened, because
	// the metric answers how often a live region changes and the span answers
	// which one changed. The map is kept for both, since the metric needs the
	// same previous-delivery time the span does.
	if render.metrics != nil && render.metrics.LiveDelivery != nil {
		render.metrics.LiveDelivery.Record(render.ctx, now.Sub(since).Seconds())
	}
	if render.delivered == nil {
		render.delivered = make(map[string]time.Time)
	}
	render.delivered[id] = now
	if !render.boundary {
		return
	}
	_, span := trace.Start(render.ctx, deliverySpanName,
		trace.WithStartTime(since),
		trace.WithAttributes(String("pw.boundary.id", id), Int("pw.boundary.bytes", size)))
	span.End()
}

// liveOpened counts this live response as one open subscription.
func (render *renderTrace) liveOpened() {
	if render == nil || render.metrics == nil || render.metrics.LiveActive == nil {
		return
	}
	render.metrics.LiveActive.Add(render.ctx, 1)
}

// liveClosed releases that count and records why the response ended.
//
// The reason is a closed set of two, and it is the trade policy:live-subscription-bounds
// makes: a bounded lifetime buys back authorization re-checks and deploy
// rollover, and the rate of retry closes is how that trade is watched.
func (render *renderTrace) liveClosed(reason string) {
	if render == nil || render.metrics == nil || render.metrics.LiveActive == nil {
		return
	}
	render.metrics.LiveActive.Add(render.ctx, -1)
	if render.metrics.LiveClosed != nil {
		render.metrics.LiveClosed.Add(render.ctx, 1, String("pw.live.close_reason", reason))
	}
}

// suppressedContent records one delivery this response did not send, because
// the region was already showing those bytes.
//
// It is counted rather than ignored because the two numbers together are the
// only way to read whether the suppression is working: deliveries alone cannot
// distinguish a quiet page from one whose every delivery is being skipped, and
// the bytes are what a reconnect would otherwise have cost.
func (render *renderTrace) suppressedContent(size int) {
	if render == nil {
		return
	}
	render.suppressed++
	render.suppressedBytes += int64(size)
}

// signalled records one signal this response forwarded.
//
// It opens no span. A signal has no duration and holds no region, so the
// interval a delivery span measures — how long that content was on screen — has
// no meaning here; the name is an attribute on a zero-length event instead.
func (render *renderTrace) signalled(name string, size int) {
	if render == nil {
		return
	}
	render.signals++
	render.signalBytes += int64(size)
	if !render.boundary {
		return
	}
	render.span.AddEvent(signalEventName,
		String("pw.signal.name", name), Int("pw.signal.bytes", size))
}

// failed marks the response as having failed, whatever status reached the
// client. A stream that broke after commit still answers 200, so the request
// span cannot report this and the render span is where it is visible.
func (render *renderTrace) failed(err error) {
	if render == nil {
		return
	}
	render.span.RecordError(err)
	render.span.SetStatus(trace.StatusError, "")
}

// wrote counts bytes the framework wrote itself, which is the buffered body and
// the live delivery records. The streamed branch counts through its writer
// instead, because htmlbind writes most of that response.
func (render *renderTrace) wrote(n int) {
	if render == nil {
		return
	}
	render.bytes += int64(n)
}

// end closes the response span with what the render produced.
func (render *renderTrace) end(attributes ...Attribute) {
	if render == nil {
		return
	}
	// A render that failed before its first flush never committed, so the
	// initial build ran until the failure and ends here.
	render.commit()
	attributes = append(attributes,
		Int64("pw.render.bytes", render.bytes),
		Int("pw.render.boundaries", render.boundaries))
	// A response that consulted the cache reports both halves, because a hit
	// count alone cannot distinguish a cache that is working from one nothing
	// is eligible for. A response that consulted it not at all reports neither,
	// rather than two zeros a dashboard would average over.
	if hits, misses := render.cache.hits.Load(), render.cache.misses.Load(); hits+misses > 0 {
		attributes = append(attributes,
			Int64("pw.render.cache_hits", hits),
			Int64("pw.render.cache_misses", misses))
	}
	if render.suppressed > 0 {
		attributes = append(attributes,
			Int("pw.live.suppressed", render.suppressed),
			Int64("pw.live.suppressed_bytes", render.suppressedBytes))
	}
	if render.signals > 0 {
		attributes = append(attributes,
			Int("pw.live.signals", render.signals),
			Int64("pw.live.signal_bytes", render.signalBytes))
	}
	render.measure()
	render.span.SetAttributes(attributes...)
	render.span.End()
}

// measure records what this response cost, keyed by the render mode.
//
// The mode is the whole attribute set: it is a closed set of six, it is the
// branch decision:automatic-async-render-selection took, and nothing outside
// this process can attribute a response time to it. Everything else the span
// carries — the layer count, the boundary ids, the byte totals per boundary —
// stays on the span, where one response is being read rather than all of them.
func (render *renderTrace) measure() {
	if render.metrics == nil {
		return
	}
	mode := String("pw.render.mode", render.mode)
	render.metrics.RenderDuration.Record(render.ctx, time.Since(render.started).Seconds(), mode)
	render.metrics.RenderBytes.Record(render.ctx, float64(render.bytes), mode)
	if render.metrics.RenderCache == nil {
		return
	}
	// Both halves or neither, for the reason the span attributes give: a hit
	// count with no denominator cannot tell a working cache from one nothing is
	// eligible for. A response that consulted no cache produces no series rather
	// than two zeros.
	hits, misses := render.cache.hits.Load(), render.cache.misses.Load()
	if hits+misses == 0 {
		return
	}
	if hits > 0 {
		render.metrics.RenderCache.Add(render.ctx, hits, String("pw.cache.result", pwruntime.CacheResultHit))
	}
	if misses > 0 {
		render.metrics.RenderCache.Add(render.ctx, misses, String("pw.cache.result", pwruntime.CacheResultMiss))
	}
}

// request binds r to the render span, for the one caller whose work reaches
// application code through the request rather than through a context argument.
// An untraced response is handed its own request back, so nothing is copied
// when nothing is traced.
func (render *renderTrace) request(r *http.Request) *http.Request {
	if render == nil {
		return r
	}
	return r.WithContext(render.ctx)
}

// writer returns w wrapped so the render span learns the size of the response
// and the moment the initial pass finished. An untraced render is handed its
// own writer back, so nothing is wrapped when nothing reads the result.
func (render *renderTrace) writer(w io.Writer) io.Writer {
	if render == nil {
		return w
	}
	return &renderWriter{downstream: w, render: render, count: true}
}

// commitWatcher returns w wrapped for the flush alone.
//
// It is what a live response needs: it re-executes the page into io.Discard to
// reach its sources, and those bytes never leave the process, so counting them
// would report a response size nothing was sent at. The flush that ends that
// pass still marks the same moment every delivery below is measured from.
func (render *renderTrace) commitWatcher(w io.Writer) io.Writer {
	if render == nil {
		return w
	}
	return &renderWriter{downstream: w, render: render}
}

// renderWriter counts what a streamed response writes and turns the flush that
// ends the initial pass into the end of a span.
//
// It implements Flush unconditionally rather than only when its downstream can,
// because the flush is a signal this needs whether or not anything acts on it:
// io.Discard flushes nothing and still marks the moment.
type renderWriter struct {
	downstream io.Writer
	render     *renderTrace
	// count says whether these bytes are ones a client receives.
	count bool
}

func (writer *renderWriter) Write(p []byte) (int, error) {
	written, err := writer.downstream.Write(p)
	if writer.count {
		writer.render.bytes += int64(written)
	}
	return written, err
}

func (writer *renderWriter) Flush() {
	writer.render.commit()
	htmlbind.Flush(writer.downstream)
}

// renderLayers counts the templates one chain composes, which is the wrapper
// depth plus the leaf. It bounds nothing and identifies nobody: it says how
// deep the composition that produced this response was.
func renderLayers(wrappers []HTMLWrapper) int { return len(wrappers) + 1 }
