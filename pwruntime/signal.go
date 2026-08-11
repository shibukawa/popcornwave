package pwruntime

import (
	"github.com/shibukawa/tinybind-go/htmlbind"
)

// A signal is a named instruction a live source sends to client code, travelling
// beside the deliveries a live boundary renders rather than replacing any of
// them. The client looks the name up in a table the page registered while it
// loaded and calls what it finds; nothing about the instruction is transferred
// but the name and its payload, which is what lets a page keep a script-src with
// no nonce and no unsafe-eval while still being directed by the server.
//
// A source yields one in the error slot of its sequence, the way a walk function
// returns fs.SkipDir: it is not a fault, it renders nothing, and it ends
// nothing. The runtime classifies it before any failure path sees it, so a
// clause with a recover subtree does not render one and a clause without one
// does not end its subscription.
//
//	func WatchJob(ctx context.Context, id string) iter.Seq2[Job, error] {
//	    return func(yield func(Job, error) bool) {
//	        for job := range watch(ctx, id) {
//	            if !yield(job, nil) {
//	                return
//	            }
//	            if job.Done {
//	                yield(Job{}, pw.NewSignal("app.finish", finished{URL: job.ResultURL}))
//	                return
//	            }
//	        }
//	    }
//	}
//
// Delivery is best effort. A signal produced while no response is open is not
// held, a reconnect replays nothing that happened during the outage, and the
// server learns nothing about whether a client dispatched one — so an
// instruction that must be seen exactly once does not belong here, and a screen
// must still be correct for someone who reloads it.
//
// The construction half lives here rather than in pw, beside the prefix both
// live loops reserve and the record both of them write. A source is the one
// piece of a live page that names no transport — it takes no request, writes to
// no response, and returns a sequence — so an application building for both
// backends keeps it in a file no build tag excludes. Reaching pw for a
// constructor is what would put the first transport's runtime into the second
// build, through a file that has nothing to do with either.

// SignalPayload is what a signal carries. Generated encoders satisfy it, which
// is what keeps a payload encoded by the same codec as every other typed value
// this framework sends.
type SignalPayload = htmlbind.SignalPayload

// Signal is one named instruction and its encoded payload.
type Signal = htmlbind.Signal

// ErrSignal matches any signal under errors.Is, for code that wants the
// classification without the value.
var ErrSignal = htmlbind.ErrSignal

// NewSignal builds a signal carrying an encoded payload.
//
// The payload is encoded here, at the call site, so the value is immutable once
// yielded and the runtime holds bytes rather than something it would have to
// reflect on to write. A nil payload is legal and means an instruction with no
// arguments.
func NewSignal[T SignalPayload](name string, payload T) Signal {
	return htmlbind.NewSignal(name, payload)
}

// NamedSignal builds a signal with no payload.
func NamedSignal(name string) Signal { return htmlbind.NamedSignal(name) }

// NewRawSignal builds a signal from a payload that is already encoded JSON.
//
// Nothing validates those bytes. A caller passing something that is not one JSON
// value produces a record a client cannot parse, which is why NewSignal is the
// ordinary path.
func NewRawSignal(name string, payload []byte) Signal {
	return htmlbind.NewRawSignal(name, payload)
}

// AsSignal reports whether err is a signal, and returns it. It is what a live
// loop classifies with, and it is exported because an application wrapping a
// source of its own needs the same test.
func AsSignal(err error) (Signal, bool) { return htmlbind.AsSignal(err) }
