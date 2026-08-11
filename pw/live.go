package pw

import (
	"context"
	"crypto/rand"
	"errors"
	"io"
	"math/big"
	"net/http"
	"time"

	"github.com/shibukawa/popcornwave/pwruntime"
	"github.com/shibukawa/tinybind-go/htmlbind"
	"github.com/shibukawa/tinybind-go/htmlbind/delta"
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
const ResponseModeHeader = pwruntime.ResponseModeHeader

// LiveResponseMode is the token that selects a delivery stream.
const LiveResponseMode = pwruntime.LiveResponseMode

// LiveManifestHeader carries the delivery validators a screen already holds, so
// a reconnect transfers what changed rather than everything.
//
// It is the live namespace's counterpart to the update manifest and is a
// separate header for the same reason the modes are separate: boundary ids are
// positional and update instance ids are not, so one header carrying both would
// be two id spaces a parser has to tell apart by shape.
//
// A hint and nothing more. The server executes the page either way, and a
// dropped, truncated, or wrong entry costs a delivery that was already going to
// be sent.
const LiveManifestHeader = "Pw-Live-Manifest"

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
		writeLiveClose(w, nil, liveCloseDone, 0)
		return
	}
	release, admitted := admitLiveResponse(r, config)
	if !admitted {
		logger.Log(ctx, LevelWarn, "live response refused: too many for this client")
		writeLiveHeaders(w)
		writeLiveClose(w, nil, liveCloseRetry, liveRetryHint)
		return
	}
	defer release()

	// The span opens once the response is admitted, so a refusal is one the
	// request span reports and not a live stream that lasted no time.
	ctx, render := startChainRenderTrace(ctx, renderModeLive,
		renderLayers(wrappers), true, true, false)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	watchdog := startLiveWatchdog(cancel, liveLifetime(config), config.LiveIdleTimeout)
	defer watchdog.Stop()

	committed := false
	// target is where every record of this response is written, and it becomes
	// an encoding writer at the moment the response commits rather than here: a
	// stream that ends before its first delivery writes only a close record, and
	// opening a frame around that costs bytes instead of saving them.
	//
	// A reconnect is what makes the coding worth its encoder: the manifest
	// suppresses the boundaries whose bytes the client still holds, and
	// everything left is a boundary that has to be re-transferred whole.
	target := http.ResponseWriter(w)
	finishEncoding := func(bool) {}
	defer func() { finishEncoding(true) }()
	reason := liveCloseDone
	boundaries := make(map[string]struct{})
	// onScreen is the validator of the content each boundary is showing: seeded
	// from what the client claims, and updated by every delivery this response
	// writes. It is what a reconnect exists to consult, and it also covers the
	// ordinary case of a source that produces the same value twice.
	//
	// It is deliberately not the same map as boundaries above. That one bounds
	// what this response serves, and seeding it from a request header would let
	// a client name thirty-two boundaries and have the response close before it
	// delivered any of them.
	digestKey := liveDigestKey(config)
	onScreen := parseLiveManifest(r.Header.Get(LiveManifestHeader), digestKey, liveManifestEntries(config))
	// The head is known before the first delivery, so it rides the opening
	// record rather than arriving after the markup that needs it. A chain whose
	// head cannot be assembled is not a reason to refuse the stream: the tags a
	// live delivery newly needs are the rare case, and the ones it usually needs
	// are already on the page.
	liveHead, err := delta.DeltaStreamHead(wrappers, leaf, renderOptions(ctx, config, false, options)...)
	if err != nil {
		logger.Log(ctx, LevelWarn, "live head could not be assembled", Err(err))
		liveHead = nil
	}
	// scratch is the one record buffer of this response; every record below
	// rebuilds into it and hands back the grown capacity.
	var scratch []byte
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
			// Negotiated before the headers are written, because that is what
			// sets Content-Encoding and Vary, and writeLiveHeaders is what
			// commits them.
			target, finishEncoding = encodedBodyWriter(w, r)
			writeLiveHeaders(w)
			var writeErr error
			if scratch, writeErr = writeLiveHead(target, scratch, liveHead); writeErr != nil {
				logger.Log(ctx, LevelError, "live head record write failed", Err(writeErr))
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
		digest := liveDigest(digestKey, content.HTML)
		if digest != "" && onScreen[content.BoundaryID] == digest {
			// This region is already showing these bytes. The client would
			// discard the record on arrival, so the only thing sending it buys
			// is the bandwidth — which on a reconnect is the whole page's worth
			// of boundaries, every one of them re-rendered and unchanged.
			//
			// The watchdog still counts it. The source produced a value, so the
			// stream is not idle, and closing it would cost a page execution to
			// learn the same thing again.
			render.suppressedContent(len(content.HTML))
			watchdog.Delivered()
			continue
		}
		var writeErr error
		if scratch, writeErr = writeLiveDelivery(target, scratch, content, digest); writeErr != nil {
			// A write failure here is the client going away, which needs no
			// record: there is nobody left to read it.
			logger.Log(ctx, LevelDebug, "live delivery write failed", Err(writeErr))
			return
		}
		if digest != "" {
			onScreen[content.BoundaryID] = digest
		}
		render.wrote(len(content.HTML))
		render.deliveredContent(content.BoundaryID, len(content.HTML))
		watchdog.Delivered()
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
		// A stream that delivered nothing writes one close record, which is
		// smaller than any frame that could hold it, so it goes out as it is.
		writeLiveHeaders(w)
	}
	hint := time.Duration(0)
	if reason == liveCloseRetry {
		hint = liveRetryHint
	}
	if _, err := writeLiveClose(target, scratch, reason, hint); err != nil {
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

// admitLiveResponse takes one slot for this request's client. The count and its
// bound are pwruntime's; what belongs here is naming the client, which is the
// one part that reads the transport.
func admitLiveResponse(r *http.Request, config HTMLConfig) (func(), bool) {
	return pwruntime.AdmitLive(liveClientKey(r), config.LiveMaxResponses)
}

// liveClientKey names the client a bound is counted against: the authenticated
// subject where there is one, and the remote address otherwise, because an
// anonymous screen is still one browser and grouping every anonymous client
// together would refuse the second visitor of the day.
func liveClientKey(r *http.Request) string {
	if r == nil {
		return ""
	}
	if authentication := RequestAuthentication(requestContext(r)); authentication.Authenticated && authentication.Subject != "" {
		return "subject:" + authentication.Subject
	}
	// The resolved caller rather than r.RemoteAddr: behind a terminating proxy
	// that address is the proxy, which would collapse every anonymous visitor
	// into one bucket and refuse the fifth of them at the shipped default.
	return "remote:" + pwruntime.ClientAddress(requestContext(r), r)
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
	// What this response omits depends on what the request claimed, so a cache
	// that ignored the manifest could serve one screen's suppressions to
	// another and leave regions empty. No-store already forbids storing it;
	// this says why, to a cache that reads one header and not the other.
	addVaryHeader(header, LiveManifestHeader)
	w.WriteHeader(http.StatusOK)
	htmlbind.Flush(w)
}

// The record writers, the digests, the manifest parse and the watchdog are
// pwruntime's, so the other transport runtime writes the same wire from the
// same code rather than from a second reading of the same protocol.
var (
	writeLiveDelivery = pwruntime.WriteLiveDelivery
	writeLiveClose    = pwruntime.WriteLiveClose
	writeLiveRecord   = pwruntime.WriteLiveRecord
	startLiveWatchdog = pwruntime.StartLiveWatchdog
	parseLiveManifest = pwruntime.ParseLiveManifest
	liveDigest        = pwruntime.LiveDigest
)

// writeLiveHead passes this build's render version, which is the one value the
// opening record carries that the leaf cannot know.
func writeLiveHead(w io.Writer, scratch []byte, head []string) ([]byte, error) {
	return pwruntime.WriteLiveHead(w, scratch, renderVersion(), head)
}

// liveDigestKey reads the configured validator key, and the leaf supplies the
// per-process fallback that keeps suppression working where nothing is set.
func liveDigestKey(config HTMLConfig) []byte {
	return pwruntime.LiveDigestKey(config.Update.ValidatorKey)
}

// liveManifestEntries bounds the parse at the response's own boundary bound,
// since a claim about a boundary this response will not serve buys nothing.
func liveManifestEntries(config HTMLConfig) int {
	if config.LiveMaxBoundaries > 0 {
		return config.LiveMaxBoundaries
	}
	return pwruntime.DefaultLiveManifestEntries
}
