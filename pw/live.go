package pw

import (
	"context"
	"crypto/rand"
	"errors"
	"io"
	"math/big"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/shibukawa/tinybind-go/htmlbind"
)

// ResponseModeHeader names what one route writes. Absent, the route answers the
// document every bookmark, crawler, and cross-site link must receive; carrying
// the live token, it answers deliveries instead.
//
// The mode is a header rather than a path or a query parameter because the page
// renders from its own URL: a token among the search parameters would reach
// template scope, and a second path would be a second route whose authorization
// and binding could drift from the one already generated and already tested. A
// custom header also cannot be set by a simple cross-origin form or link, which
// is the class of request policy:csrf-protection worries about.
const ResponseModeHeader = "Pw-Response-Mode"

// LiveResponseMode is the token that selects a delivery stream.
const LiveResponseMode = "live"

// liveMediaType frames the stream as newline-delimited records. Past the
// initial document no parser is reading, so the template-and-marker framing of
// writeBoundaryCompletion has nothing to trigger and a record is the ordinary
// shape.
const liveMediaType = "application/x-ndjson"

// liveRetryHint is how long a client is asked to wait after a response the
// server closed for its own reasons. It is a hint on the record rather than a
// rule, because only the server knows whether it is shedding load, and only the
// client knows whether the tab is still worth updating.
const liveRetryHint = 2 * time.Second

// live close reasons. done means nothing more is coming and the client stops;
// retry means this response ended healthy and reconnecting is expected. A bare
// close cannot say which, because a lifetime bound closes a working stream.
const (
	liveCloseDone  = "done"
	liveCloseRetry = "retry"
)

// liveModeRequested reports whether this request asked for deliveries instead
// of a document. An unknown mode token is not an error: it answers the document,
// so an older client meeting a newer server stays functional.
func liveModeRequested(r *http.Request) bool {
	return r != nil && r.Header.Get(ResponseModeHeader) == LiveResponseMode
}

// renderVersion identifies the generated code behind the boundary ids. A client
// holding ids from an older deployment is told to reload rather than handed
// deliveries that address a document that no longer exists.
//
// It comes from the build's own VCS stamp, so two instances of one deployment
// agree and a restart of the same binary does not evict anybody. A build with
// no stamp reports nothing, which disables the check rather than inventing a
// value that would differ per process and reload every client on every restart.
var renderVersion = updateBuildID

// serveLive answers a live mode request with the deliveries of one chain.
//
// The route reached here has already run: its handler, its layouts, and its
// parameter binding are the ones that produced the document, which is what
// makes a reconnect need no continuation token and no server-held subscription
// state. Boundary ids are positional, so this render hands out the same ids the
// document render did and a delivery addresses a placeholder already on screen.
func serveLive(w http.ResponseWriter, r *http.Request, wrappers []HTMLWrapper, leaf HTMLFragment, config HTMLConfig, options ...HTMLOption) {
	ctx := requestContext(r)
	logger := Logger(ctx)
	// A page with nothing live answers immediately rather than holding a
	// connection open for deliveries that cannot exist. The client should not
	// have asked: the document marker told it so.
	if !liveEnabled(config) || !htmlbind.HasLiveBlock(wrappers, leaf) {
		writeLiveHeaders(w)
		writeLiveClose(w, liveCloseDone, 0)
		return
	}
	release, admitted := admitLiveResponse(r, config)
	if !admitted {
		logger.Log(ctx, LevelWarn, "live response refused: too many for this client")
		writeLiveHeaders(w)
		writeLiveClose(w, liveCloseRetry, liveRetryHint)
		return
	}
	defer release()

	// The span opens once the response is admitted, so a refusal is one the
	// request span reports and not a live stream that lasted no time.
	ctx, render := startRenderTrace(ctx, renderModeLive,
		chainRenderAttributes(wrappers, true, true, false)...)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	watchdog := startLiveWatchdog(cancel, liveLifetime(config), config.LiveIdleTimeout)
	defer watchdog.stop()

	committed := false
	reason := liveCloseDone
	boundaries := make(map[string]struct{})
	// The close reason is read at return rather than captured here, because
	// every branch below decides it and only the last one is true.
	defer func() { render.end(String("pw.live.close_reason", reason)) }()
	// The chain renders into io.Discard, so the wrapper is here for the flush
	// alone: it marks the same moment a document response commits, which is
	// where every delivery interval below is measured from.
	render.initialBuild()
	for content, err := range htmlbind.RenderChainLive(ctx, render.commitWatcher(io.Discard), wrappers, leaf, renderOptions(ctx, config, false, options)...) {
		if err != nil {
			render.failed(err)
			// A boundary that failed with no recover clause has nothing left to
			// deliver, and reconnecting would only reproduce it, so this closes
			// done however far the stream got.
			var unrecovered *htmlbind.UnrecoveredError
			if errors.As(err, &unrecovered) {
				logger.Log(ctx, LevelError, "live boundary failed with no recover clause",
					String("boundary", unrecovered.BoundaryID), Err(unrecovered.Err))
				reason = liveCloseDone
				break
			}
			if !committed {
				// Nothing has been written, so this failure still carries its
				// real status and becomes an ordinary problem response.
				WriteProblem(w, r, InternalServerError(err))
				return
			}
			logger.Log(ctx, LevelError, "live stream failed after commit", Err(err))
			reason = liveCloseRetry
			break
		}
		if !committed {
			writeLiveHeaders(w)
			if err := writeLiveOpen(w); err != nil {
				logger.Log(ctx, LevelError, "live open record write failed", Err(err))
				return
			}
			committed = true
		}
		if _, known := boundaries[content.BoundaryID]; !known {
			if config.LiveMaxBoundaries > 0 && len(boundaries) >= config.LiveMaxBoundaries {
				// Reported rather than truncated silently: a screen quietly
				// missing one panel's updates is worse than one that stops and
				// says so in a log.
				logger.Log(ctx, LevelError, "live response exceeded its boundary bound",
					String("boundary", content.BoundaryID), Int("bound", config.LiveMaxBoundaries))
				reason = liveCloseDone
				break
			}
			boundaries[content.BoundaryID] = struct{}{}
		}
		if err := writeLiveDelivery(w, content); err != nil {
			// A write failure here is the client going away, which needs no
			// record: there is nobody left to read it.
			logger.Log(ctx, LevelDebug, "live delivery write failed", Err(err))
			return
		}
		render.wrote(len(content.HTML))
		render.deliveredContent(content.BoundaryID, len(content.HTML))
		watchdog.delivered()
	}
	if ctx.Err() != nil {
		if requestContext(r).Err() != nil {
			// The client disconnected. Every subscription is already cancelled
			// by the same context, and no record can reach a closed connection.
			return
		}
		// A bound this server chose ended a healthy stream, so the client is
		// expected back rather than told to stop.
		reason = liveCloseRetry
	}
	if !committed {
		writeLiveHeaders(w)
	}
	hint := time.Duration(0)
	if reason == liveCloseRetry {
		hint = liveRetryHint
	}
	if err := writeLiveClose(w, reason, hint); err != nil {
		logger.Log(ctx, LevelDebug, "live close record write failed", Err(err))
	}
}

// liveEnabled reports whether this configuration answers live requests at all.
// Streaming carries live: a buffered document settles its live boundaries in
// place and writes no placeholder, so it holds nothing a delivery could replace.
func liveEnabled(config HTMLConfig) bool { return config.Live && config.Streaming }

// liveLifetime spreads the configured maximum around its value. A fixed
// lifetime resynchronizes every client on every cycle, so one restart produces
// a herd that then repeats forever; a client cannot fix that with backoff,
// because it never chose the moment it was closed.
func liveLifetime(config HTMLConfig) time.Duration {
	if config.LiveMaxDuration <= 0 {
		return 0
	}
	spread := config.LiveDurationJitter
	if spread <= 0 {
		return config.LiveMaxDuration
	}
	if spread > 100 {
		spread = 100
	}
	span := int64(config.LiveMaxDuration) * int64(spread) / 100
	if span <= 0 {
		return config.LiveMaxDuration
	}
	offset, err := rand.Int(rand.Reader, big.NewInt(2*span))
	if err != nil {
		return config.LiveMaxDuration
	}
	return time.Duration(int64(config.LiveMaxDuration) - span + offset.Int64())
}

// liveWatchdog closes a response that has run long enough or delivered nothing
// for long enough. Both bounds cancel the render context, which breaks every
// pull loop and lets each source observe the cancellation through the context
// decision:live-external-signature makes mandatory.
type liveWatchdog struct {
	stopOnce sync.Once
	done     chan struct{}
	activity chan struct{}
}

func startLiveWatchdog(cancel context.CancelFunc, lifetime, idle time.Duration) *liveWatchdog {
	watchdog := &liveWatchdog{done: make(chan struct{}), activity: make(chan struct{}, 1)}
	if lifetime <= 0 && idle <= 0 {
		return watchdog
	}
	go func() {
		var deadline <-chan time.Time
		if lifetime > 0 {
			timer := time.NewTimer(lifetime)
			defer timer.Stop()
			deadline = timer.C
		}
		idleTimer := &time.Timer{}
		var idleC <-chan time.Time
		if idle > 0 {
			idleTimer = time.NewTimer(idle)
			defer idleTimer.Stop()
			idleC = idleTimer.C
		}
		for {
			select {
			case <-watchdog.done:
				return
			case <-deadline:
				cancel()
				return
			case <-idleC:
				cancel()
				return
			case <-watchdog.activity:
				if idle > 0 {
					if !idleTimer.Stop() {
						select {
						case <-idleTimer.C:
						default:
						}
					}
					idleTimer.Reset(idle)
				}
			}
		}
	}()
	return watchdog
}

func (d *liveWatchdog) delivered() {
	select {
	case d.activity <- struct{}{}:
	default:
	}
}

func (d *liveWatchdog) stop() { d.stopOnce.Do(func() { close(d.done) }) }

// liveAdmission counts open live responses per client, because a bound on one
// response buys nothing against a client that opens ten. The key is the
// authenticated subject where there is one, and the remote address otherwise:
// an anonymous screen is still one browser, and grouping every anonymous client
// together would refuse the second visitor of the day.
var liveAdmission = struct {
	sync.Mutex
	open map[string]int
}{open: map[string]int{}}

func admitLiveResponse(r *http.Request, config HTMLConfig) (func(), bool) {
	if config.LiveMaxResponses <= 0 {
		return func() {}, true
	}
	key := liveClientKey(r)
	liveAdmission.Lock()
	defer liveAdmission.Unlock()
	if liveAdmission.open[key] >= config.LiveMaxResponses {
		return func() {}, false
	}
	liveAdmission.open[key]++
	return func() {
		liveAdmission.Lock()
		defer liveAdmission.Unlock()
		if liveAdmission.open[key] <= 1 {
			delete(liveAdmission.open, key)
			return
		}
		liveAdmission.open[key]--
	}, true
}

func liveClientKey(r *http.Request) string {
	if r == nil {
		return ""
	}
	if authentication := RequestAuthentication(requestContext(r)); authentication.Authenticated && authentication.Subject != "" {
		return "subject:" + authentication.Subject
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	return "remote:" + host
}

// writeLiveHeaders commits the response. A delivery stream is never shareable,
// and the mode header has to reach any cache between here and the client, or a
// stream stored under the page URL is served where a document was expected.
func writeLiveHeaders(w http.ResponseWriter) {
	header := w.Header()
	header.Set("Content-Type", liveMediaType)
	header.Set("Cache-Control", "no-store")
	header.Set("X-Content-Type-Options", "nosniff")
	addVaryHeader(header, ResponseModeHeader)
	w.WriteHeader(http.StatusOK)
	htmlbind.Flush(w)
}

// writeLiveOpen names the generated version this stream belongs to, so a client
// holding ids from another deployment reloads instead of applying deliveries to
// a document that no longer matches them.
func writeLiveOpen(w http.ResponseWriter) error {
	record := []byte(`{"control":"open","version":`)
	record = append(record, htmlbind.JSONString(renderVersion())...)
	return writeLiveRecord(w, append(record, '}'))
}

func writeLiveDelivery(w http.ResponseWriter, content htmlbind.Content) error {
	return writeLiveRecord(w, content.AppendJSON(nil))
}

// writeLiveClose is always the last record. A clean transport close cannot say
// whether the sources finished or a bound ended a healthy response, and the two
// deserve opposite client behaviour.
func writeLiveClose(w http.ResponseWriter, reason string, retryAfter time.Duration) error {
	record := []byte(`{"control":"closed","reason":"` + reason + `"`)
	if retryAfter > 0 {
		record = append(record, `,"retry_after_ms":`...)
		record = append(record, strconv.FormatInt(retryAfter.Milliseconds(), 10)...)
	}
	return writeLiveRecord(w, append(record, '}'))
}

func writeLiveRecord(w io.Writer, record []byte) error {
	if _, err := w.Write(append(record, '\n')); err != nil {
		return err
	}
	htmlbind.Flush(w)
	return nil
}
