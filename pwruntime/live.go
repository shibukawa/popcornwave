package pwruntime

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"hash"
	"io"
	"math/big"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/shibukawa/tinybind-go/htmlbind"
)

// The live delivery protocol, and every decision it rests on, live here rather
// than in a transport runtime.
//
// The wire is one thing: a record stream of a fixed shape, with validators the
// client returns on its next connection, closed by a record that says whether
// the sources finished or a bound ended a healthy response. Two runtimes each
// writing their own would be two chances to disagree about framing, digests and
// close reasons — on the one response nobody watches, because it is open while
// a screen sits idle and nobody reloads a page that looks right.
//
// What each runtime keeps is what genuinely differs: setting response headers,
// obtaining a writer that flushes, naming the client for admission, and
// answering a pre-commit failure. Everything below is shared.

// LiveMediaType frames the record stream.
const LiveMediaType = "application/x-ndjson"

// The close reasons. A clean transport close cannot say whether the sources
// finished or a bound ended a healthy response, and the two deserve opposite
// client behaviour.
const (
	LiveCloseDone  = "done"
	LiveCloseRetry = "retry"
)

// LiveRetryHint is what a retry close suggests waiting.
const LiveRetryHint = 2 * time.Second

// DefaultLiveManifestEntries bounds an unbounded configuration's manifest
// parse. It is not a limit on the response: past it the extra claims are
// dropped and their boundaries are delivered rather than suppressed.
const DefaultLiveManifestEntries = 64

// LiveLifetime spreads a configured maximum around its value.
//
// A fixed lifetime resynchronizes every client on every cycle, so one restart
// produces a herd that then repeats forever; a client cannot fix that with
// backoff, because it never chose the moment it was closed.
func LiveLifetime(maximum time.Duration, jitterPercent int) time.Duration {
	if maximum <= 0 {
		return 0
	}
	spread := jitterPercent
	if spread <= 0 {
		return maximum
	}
	if spread > 100 {
		spread = 100
	}
	span := int64(maximum) * int64(spread) / 100
	if span <= 0 {
		return maximum
	}
	offset, err := rand.Int(rand.Reader, big.NewInt(2*span))
	if err != nil {
		return maximum
	}
	return time.Duration(int64(maximum) - span + offset.Int64())
}

// LiveWatchdog closes a response that has run long enough or delivered nothing
// for long enough. Both bounds cancel the render context, which breaks every
// pull loop and lets each source observe the cancellation through its context.
type LiveWatchdog struct {
	stopOnce sync.Once
	done     chan struct{}
	activity chan struct{}
}

// StartLiveWatchdog begins watching. A zero lifetime and a zero idle bound
// leave it inert, which is the configuration that asks for no bound at all.
func StartLiveWatchdog(cancel context.CancelFunc, lifetime, idle time.Duration) *LiveWatchdog {
	watchdog := &LiveWatchdog{done: make(chan struct{}), activity: make(chan struct{}, 1)}
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

// Delivered reports activity, which restarts the idle bound.
func (d *LiveWatchdog) Delivered() {
	select {
	case d.activity <- struct{}{}:
	default:
	}
}

// Stop ends the watch.
func (d *LiveWatchdog) Stop() { d.stopOnce.Do(func() { close(d.done) }) }

// liveAdmission counts open live responses per client, because a bound on one
// response buys nothing against a client that opens ten.
var liveAdmission = struct {
	sync.Mutex
	open map[string]int
}{open: map[string]int{}}

// AdmitLive takes one slot for a client and returns the release. A maximum of
// zero or less admits everything.
//
// The key is the caller's to choose, and it is the authenticated subject where
// there is one and the remote address otherwise: an anonymous screen is still
// one browser, and grouping every anonymous client together would refuse the
// second visitor of the day.
func AdmitLive(key string, maximum int) (func(), bool) {
	if maximum <= 0 {
		return func() {}, true
	}
	liveAdmission.Lock()
	defer liveAdmission.Unlock()
	if liveAdmission.open[key] >= maximum {
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

// LiveDigest is the validator of one delivery: what a screen holds, and what it
// returns on its next connection so the server can leave it alone.
//
// Keyed, because it travels in a request header on every reconnect, and a
// request header is the most widely logged thing between a browser and a
// handler. An unkeyed digest there is a stable fingerprint of the region's
// content, and a live region with few possible renderings — a status badge, a
// queue depth, a seat count — is enumerable from a proxy log by anyone who can
// render the same page. Keying costs one HMAC per delivery and takes the
// fingerprint away.
//
// Truncated to twelve bytes. The digest decides only whether to skip a transfer
// the client would have discarded, so the wrong answer costs one region one
// stale rendering, and ninety-six bits is far past where that becomes the
// system's most likely failure.
func LiveDigest(key, html []byte) string {
	return NewLiveDigester(key).Digest(html)
}

// LiveDigester is LiveDigest with the HMAC state built once and reset between
// records. A stream digests every boundary it sends, and each hmac.New
// allocates two SHA-256 states plus the key pads, so the loop that owns a
// response builds one of these instead. It is single-goroutine, like that
// loop.
type LiveDigester struct {
	mac hash.Hash
	sum [sha256.Size]byte
}

// NewLiveDigester returns a digester over key, or nil for a nil key — the
// same "suppression off" answer LiveDigest gives, which Digest keeps by
// answering the empty string on a nil receiver.
func NewLiveDigester(key []byte) *LiveDigester {
	if key == nil {
		return nil
	}
	return &LiveDigester{mac: hmac.New(sha256.New, key)}
}

// Digest is the validator of one delivery, from the reused state.
func (d *LiveDigester) Digest(html []byte) string {
	if d == nil {
		return ""
	}
	d.mac.Reset()
	d.mac.Write(html)
	sum := d.mac.Sum(d.sum[:0])
	return base64.RawURLEncoding.EncodeToString(sum[:12])
}

// LiveDigestKey keys the delivery validators, or reports nil where suppression
// has to be off. A configured validator key is used where there is one, and a
// per-process key otherwise, so suppression works in a single-instance
// deployment that configured nothing.
func LiveDigestKey(configured string) []byte {
	if configured != "" {
		return []byte(configured)
	}
	return processDigestKey()
}

// processDigestKey is this process's fallback key. A nil result turns
// suppression off rather than falling back to an unkeyed digest.
var processDigestKey = sync.OnceValue(func() []byte {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil
	}
	return key
})

// ParseLiveManifest reads what the client claims each boundary is showing.
//
// The bound is on entries rather than on bytes, and exceeding it drops the
// extra claims rather than refusing the request: the cost of a dropped claim is
// one region re-transferred, and refusing would cost the whole stream.
func ParseLiveManifest(value string, key []byte, maximum int) map[string]string {
	if value == "" || key == nil || maximum <= 0 {
		return map[string]string{}
	}
	claims := map[string]string{}
	for _, entry := range strings.Split(value, ",") {
		if len(claims) >= maximum {
			break
		}
		id, digest, found := strings.Cut(strings.TrimSpace(entry), ":")
		if !found || id == "" || digest == "" {
			continue
		}
		claims[id] = digest
	}
	return claims
}

// The record writers build into the caller's scratch buffer and return it
// grown, so one live response reuses a single allocation across every delivery
// instead of building a fresh record per boundary. A nil scratch is fine.

// WriteLiveHead opens the stream, on the record every other update response
// opens with. It carries the head a delivery may need, because a delivery whose
// content reaches a component the document never carried would otherwise
// install nothing and paint unstyled.
func WriteLiveHead(w io.Writer, scratch []byte, build string, head []string) ([]byte, error) {
	record := append(scratch[:0], `{"r":"head"`...)
	if build != "" {
		record = append(record, `,"build":`...)
		record = append(record, htmlbind.JSONString(build)...)
	}
	if len(head) > 0 {
		record = append(record, `,"head":[`...)
		for index, tag := range head {
			if index > 0 {
				record = append(record, ',')
			}
			record = append(record, htmlbind.JSONString(tag)...)
		}
		record = append(record, ']')
	}
	return WriteLiveRecord(w, append(record, '}'))
}

// WriteLiveDelivery writes one delivery, carrying the validator the client
// stores and returns on its next connection.
func WriteLiveDelivery(w io.Writer, scratch []byte, content htmlbind.Content, digest string) ([]byte, error) {
	record := append(scratch[:0], `{"r":"await","id":`...)
	record = append(record, htmlbind.JSONString(content.BoundaryID)...)
	record = append(record, `,"html":`...)
	record = append(record, htmlbind.JSONString(string(content.HTML))...)
	if digest != "" {
		record = append(record, `,"v":`...)
		record = append(record, htmlbind.JSONString(digest)...)
	}
	return WriteLiveRecord(w, append(record, '}'))
}

// ReservedSignalPrefix is this framework's signal namespace.
//
// Every layer that produces signals reserves a prefix: the module holds tb.,
// this framework holds pw., and an application uses what is left. The module
// cannot hold this one, because a signal constructor is called at a yield site
// inside a source and is not render-scoped, so it can reach no configured value.
//
// What it protects is trust. The lifecycle names this framework's client runtime
// dispatches — a boundary settled, a live response opened, a delivery applied —
// arrive under this prefix, and a handler believes them precisely because
// application data has no route into the namespace. A source able to emit
// pw.delivery_applied could make a screen believe a render landed that never did.
//
// It lives here rather than beside either live loop because both of them enforce
// it, and a prefix one backend reserved and the other did not would be a
// namespace an application could reach through the second one.
const ReservedSignalPrefix = "pw."

// ReservedSignalName reports a name this framework refuses to put on the wire.
//
// The module refuses its own prefix inside its constructors, where a bad name
// becomes a value that faults when the runtime reads it. This one is enforced
// where a signal is written instead, for two reasons: a constructor cannot carry
// a message of its own, since the fault field is the module's and unexported;
// and a constructor is not a chokepoint at all, because an application calling
// the module's constructor directly bypasses any wrapper this framework offers.
// A live loop is the only path a signal reaches a client through.
func ReservedSignalName(name string) bool {
	return strings.HasPrefix(name, ReservedSignalPrefix)
}

// WriteLiveSignal writes one signal: a name the client looks up in the table it
// registered while the page loaded, and the payload the source encoded.
//
// It carries no boundary id, no validator and no revision, because a signal
// addresses no region. It is dispatched rather than applied, so the suppression
// and manifest bookkeeping every delivery goes through has nothing to say about
// it, and a client that skips a malformed one desynchronizes nothing.
//
// The name is escaped rather than written through: it reaches a client as a
// lookup key, and the payload is appended exactly as the generated encoder
// produced it, which htmlbind already escaped for a script context as well as a
// JSON one.
func WriteLiveSignal(w io.Writer, scratch []byte, signal htmlbind.Signal) ([]byte, error) {
	record := append(scratch[:0], `{"r":"signal","name":`...)
	record = append(record, htmlbind.JSONString(signal.Name())...)
	if payload := signal.Payload(); len(payload) > 0 {
		record = append(record, `,"data":`...)
		record = append(record, payload...)
	}
	return WriteLiveRecord(w, append(record, '}'))
}

// WriteLiveClose is always the last record.
func WriteLiveClose(w io.Writer, scratch []byte, reason string, retryAfter time.Duration) ([]byte, error) {
	record := append(scratch[:0], `{"r":"end","reason":"`...)
	record = append(record, reason...)
	record = append(record, '"')
	if retryAfter > 0 {
		record = append(record, `,"retryMs":`...)
		record = append(record, strconv.FormatInt(retryAfter.Milliseconds(), 10)...)
	}
	return WriteLiveRecord(w, append(record, '}'))
}

// WriteLiveRecord writes one newline-terminated record and flushes it. A
// delivery the client cannot see until a buffer fills is a delivery that did
// not happen.
func WriteLiveRecord(w io.Writer, record []byte) ([]byte, error) {
	record = append(record, '\n')
	if _, err := w.Write(record); err != nil {
		return record, err
	}
	htmlbind.Flush(w)
	return record, nil
}

// ResponseModeHeader is how a client asks for something other than a document
// on a route's own URL, and LiveResponseMode is the one value that does.
//
// They are here rather than on either runtime because they are the wire between
// the browser runtime this framework ships and whichever half is serving. Two
// transports reading two different headers is a client that works against one
// build of an application and not the other, which is what happened.
const (
	ResponseModeHeader = "Pw-Response-Mode"
	LiveResponseMode   = "live"
)
