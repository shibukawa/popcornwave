package pw

import (
	"errors"

	"github.com/shibukawa/popcornwave/pwruntime"
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

// SignalPayload is what a signal carries. Generated encoders satisfy it, which
// is what keeps a payload encoded by the same codec as every other typed value
// this framework sends.
type SignalPayload = htmlbind.SignalPayload

// Signal is one named instruction and its encoded payload.
type Signal = htmlbind.Signal

// ErrSignal matches any signal under errors.Is, for code that wants the
// classification without the value.
var ErrSignal = htmlbind.ErrSignal

// ReservedSignalPrefix is this framework's signal namespace. It and the check
// below are pwruntime's, so both transport runtimes reserve the same one: a
// namespace one backend guarded and the other did not would be reachable through
// the second.
const ReservedSignalPrefix = pwruntime.ReservedSignalPrefix

// ErrReservedSignalName reports a name inside this framework's namespace.
var ErrReservedSignalName = errors.New("pw: signal name uses the reserved " + ReservedSignalPrefix + " prefix")

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

// AsSignal reports whether err is a signal, and returns it. It is what the live
// loop classifies with, and it is exported because an application wrapping a
// source of its own needs the same test.
func AsSignal(err error) (Signal, bool) { return htmlbind.AsSignal(err) }

// ReservedSignalName reports a name this framework refuses to put on the wire.
// It is enforced where a signal is written rather than where one is constructed;
// pwruntime.ReservedSignalName carries why.
func ReservedSignalName(name string) bool { return pwruntime.ReservedSignalName(name) }
