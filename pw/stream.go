package pw

import (
	"net/http"
	"strings"

	tinybind "github.com/shibukawa/tinybind-go"
)

// Stream is an open typed event stream. It is the module's own type rather
// than a wrapper around it, so the value a callback receives here is the value
// it receives on the other transport runtime, and a handler body is the same
// text on both.
//
// Write sends one value. Close is the runtime's, not the caller's: the entry
// below runs it whichever way the callback ends.
type Stream[T any] = tinybind.Stream[T]

// WriteStream negotiates a typed event stream, runs fn against it, and closes
// it.
//
// The callback is the shape because it is the only one both transports can
// express. A backend that streams by registering a body writer runs that writer
// after the handler has returned, so a stream the handler opened and held has
// nowhere to live there. Moving this surface to the shape that transport
// requires is what lets one handler serve both.
//
// It returns nothing, for the same reason. The callback runs after the handler
// returned on that backend, so an error cannot travel back to handler code;
// offering one here and not there would make the same source behave differently
// per transport, which is what the shared shape exists to prevent.
//
// A failure to negotiate happens before anything is committed and becomes an
// ordinary problem response. Once the stream is open the status has been sent,
// so a failure from the callback reaches SetStreamErrorHandler instead. Close
// runs either way, which is what keeps the trailing bracket of the JSON array
// framing from depending on a caller remembering to write it.
func WriteStream[T any](w http.ResponseWriter, r *http.Request, fn func(*Stream[T]) error) {
	// Checked here rather than left to the module, so an unacceptable Accept
	// reaches this framework's problem path and a browser gets the registered
	// error page instead of a bare problem document.
	if !supportedStreamAccept(r) {
		WriteProblem(w, r, Problem{
			Status:  http.StatusNotAcceptable,
			Title:   "Not Acceptable",
			Code:    "not_acceptable",
			Message: "unsupported streaming representation",
		})
		return
	}
	tinybind.WriteStream(w, r, fn)
}

// SetStreamErrorHandler installs the destination for a stream failure raised
// after the response status has been sent. Passing nil discards them, which is
// the default: a runtime that logged on its own would be writing somewhere the
// application did not choose.
//
// It is shared with the other transport runtime, so installing it once covers
// both.
func SetStreamErrorHandler(fn func(error)) { tinybind.SetStreamErrorHandler(fn) }

func supportedStreamAccept(r *http.Request) bool {
	if r == nil {
		return true
	}
	accept := strings.TrimSpace(strings.ToLower(r.Header.Get("Accept")))
	if accept == "" || accept == "*/*" {
		return true
	}
	for _, item := range strings.Split(accept, ",") {
		media := strings.TrimSpace(strings.SplitN(item, ";", 2)[0])
		switch media {
		case "*/*", "text/event-stream", "application/x-ndjson", "application/ndjson", "application/json", "application/jsonl":
			return true
		}
	}
	return false
}
