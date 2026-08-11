package pwfast

import (
	"bufio"
	"context"
	"errors"
	"io"

	"github.com/shibukawa/popcornwave/pwruntime"
	"github.com/shibukawa/tinybind-go/fasthttpupdate"
	"github.com/shibukawa/tinybind-go/htmlbind"
	"github.com/shibukawa/tinybind-go/htmlbind/delta"
	"github.com/shibukawa/tinygodriver/fasthttp"
)

// LiveManifestHeader carries what each boundary on the client is showing, so a
// reconnect re-sends only what changed.
const LiveManifestHeader = "Pw-Live-Manifest"

// ResponseModeHeader selects deliveries over a document, and LiveResponseMode
// is the token that does it. They are declared here rather than imported from
// the other half for the reason LiveManifestHeader is: the two transports share
// a wire, not a package.
const ResponseModeHeader = "Pw-Response-Mode"

// LiveResponseMode is the token that selects a delivery stream.
const LiveResponseMode = "live"

// ServeLive answers a live mode request with the deliveries of one chain, and
// reports whether it did.
//
// Every decision it makes is pwruntime's: the admission bound, the watchdog,
// the digest suppression seeded from the client's manifest, the record framing
// and the close reasons. What is written here is the transport — obtaining a
// writer that flushes, and reading the two request values the loop needs — so
// the two runtimes cannot disagree about the wire.
//
// The body writer runs after the handler has returned, which is this
// transport's whole shape, and it changes one thing the other half relies on:
// the request value is pooled and gone by then, so nothing may be read from it
// inside the loop and the render is bounded by the watchdog rather than by the
// request context. A client that goes away is noticed on the next write, which
// is the same signal the other half falls back to once a record fails.
func ServeLive(r *fasthttp.RequestCtx, wrappers []HTMLWrapper, leaf HTMLFragment, options ...HTMLOption) bool {
	settings, ok := pwruntime.ResolvedUpdateSettings()
	if !ok || !settings.Enabled {
		return false
	}
	update, ok := updateOptions()
	if !ok || update.Negotiate(r).Mode != fasthttpupdate.ModeLive {
		return false
	}

	// Everything the loop needs is read here, while the request value is still
	// alive. What crosses into the callback is values.
	manifest := string(r.Request.Header.Peek(LiveManifestHeader))
	clientKey := liveClientKey(r)
	digestKey := pwruntime.LiveDigestKey(settings.ValidatorKey)
	maxBoundaries := settings.LiveMaxBoundaries
	onScreen := pwruntime.ParseLiveManifest(manifest, digestKey, manifestEntries(settings))

	release, admitted := pwruntime.AdmitLive(clientKey, settings.LiveMaxResponses)
	if !admitted {
		writeLiveHeaders(r)
		_, _ = pwruntime.WriteLiveClose(r, nil, pwruntime.LiveCloseRetry, pwruntime.LiveRetryHint)
		return true
	}

	if !htmlbind.HasLiveBlock(wrappers, leaf) {
		// The client should not have asked; the document marker told it so.
		// Answering immediately beats holding a connection open for deliveries
		// that cannot exist.
		release()
		writeLiveHeaders(r)
		_, _ = pwruntime.WriteLiveClose(r, nil, pwruntime.LiveCloseDone, 0)
		return true
	}

	writeLiveHeaders(r)
	head, headErr := delta.DeltaStreamHead(wrappers, leaf, settings.RenderOptions(context.Background())...)
	if headErr != nil {
		// A chain whose head cannot be assembled is not a reason to refuse the
		// stream: the tags a delivery newly needs are the rare case.
		head = nil
	}
	r.SetBodyStreamWriter(func(w *bufio.Writer) {
		defer release()
		runLiveStream(w, wrappers, leaf, settings, digestKey, onScreen, maxBoundaries, head, options)
	})
	return true
}

// runLiveStream is the loop, over the shared protocol.
func runLiveStream(w io.Writer, wrappers []HTMLWrapper, leaf HTMLFragment,
	settings pwruntime.UpdateSettings, digestKey []byte, onScreen map[string]string,
	maxBoundaries int, head []string, options []HTMLOption,
) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	watchdog := pwruntime.StartLiveWatchdog(cancel,
		pwruntime.LiveLifetime(settings.LiveMaxDuration, settings.LiveDurationJitter),
		settings.LiveIdleTimeout)
	defer watchdog.Stop()

	var scratch []byte
	scratch, err := pwruntime.WriteLiveHead(w, scratch, settings.BuildID, head)
	if err != nil {
		return
	}
	reason := pwruntime.LiveCloseDone
	boundaries := map[string]struct{}{}
	render := append(settings.RenderOptions(ctx), options...)
	for content, err := range htmlbind.RenderChainLive(ctx, io.Discard, wrappers, leaf, render...) {
		if err != nil {
			var unrecovered *htmlbind.UnrecoveredError
			if errors.As(err, &unrecovered) {
				// A boundary that failed with no recover clause has nothing
				// left to deliver, and reconnecting would only reproduce it.
				reason = pwruntime.LiveCloseDone
				break
			}
			reason = pwruntime.LiveCloseRetry
			break
		}
		if _, known := boundaries[content.BoundaryID]; !known {
			if maxBoundaries > 0 && len(boundaries) >= maxBoundaries {
				reason = pwruntime.LiveCloseDone
				break
			}
			boundaries[content.BoundaryID] = struct{}{}
		}
		digest := pwruntime.LiveDigest(digestKey, content.HTML)
		if digest != "" && onScreen[content.BoundaryID] == digest {
			// This region is already showing these bytes. The watchdog still
			// counts it: the source produced a value, so the stream is not idle.
			watchdog.Delivered()
			continue
		}
		if scratch, err = pwruntime.WriteLiveDelivery(w, scratch, content, digest); err != nil {
			// A write failure is the client going away, and there is nobody
			// left to send a close record to.
			return
		}
		if digest != "" {
			onScreen[content.BoundaryID] = digest
		}
		watchdog.Delivered()
	}
	if ctx.Err() != nil && reason == pwruntime.LiveCloseDone {
		// A bound this server chose ended a healthy stream, so the client is
		// expected back rather than told to stop.
		reason = pwruntime.LiveCloseRetry
	}
	hint := pwruntime.LiveRetryHint
	if reason != pwruntime.LiveCloseRetry {
		hint = 0
	}
	_, _ = pwruntime.WriteLiveClose(w, scratch, reason, hint)
}

// writeLiveHeaders commits the response. A delivery stream is never shareable,
// and what it omits depends on what the request claimed, so a cache that
// ignored the manifest could serve one screen's suppressions to another.
func writeLiveHeaders(r *fasthttp.RequestCtx) {
	r.Response.Header.SetContentType(pwruntime.LiveMediaType)
	r.Response.Header.Set("Cache-Control", "no-store")
	r.Response.Header.Set("X-Content-Type-Options", "nosniff")
	r.Response.Header.Add("Vary", "Pw-Render")
	r.Response.Header.Add("Vary", LiveManifestHeader)
	r.SetStatusCode(fasthttp.StatusOK)
}

// liveClientKey names the client the admission bound counts against.
func liveClientKey(r *fasthttp.RequestCtx) string {
	if subject := pwruntime.RequestAuthentication(r); subject.Authenticated && subject.Subject != "" {
		return "subject:" + subject.Subject
	}
	return "remote:" + r.RemoteIP().String()
}

func manifestEntries(settings pwruntime.UpdateSettings) int {
	if settings.LiveMaxBoundaries > 0 {
		return settings.LiveMaxBoundaries
	}
	return pwruntime.DefaultLiveManifestEntries
}
