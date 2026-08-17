package middlewares

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
)

// ResponseTracker records the status code and body size of a response while
// forwarding every optional http.ResponseWriter capability to the wrapped
// writer.
type ResponseTracker struct {
	http.ResponseWriter
	status int
	bytes  int64
	wrote  bool
	// route is the pattern the router matched, carried back up the chain.
	//
	// It travels on the writer because it cannot travel on the request: net/http
	// sets Pattern on the copy the mux hands the handler, and every frame that
	// would report it wraps the mux and never sees that copy. The writer is the
	// one value that goes down the chain and is still readable after it returns.
	route string
}

func (w *ResponseTracker) WriteHeader(status int) {
	if w.wrote {
		return
	}
	if status >= 100 && status < 200 && status != http.StatusSwitchingProtocols {
		w.ResponseWriter.WriteHeader(status)
		return
	}
	w.status, w.wrote = status, true
	w.ResponseWriter.WriteHeader(status)
}

func (w *ResponseTracker) Write(body []byte) (int, error) {
	if !w.wrote {
		w.WriteHeader(http.StatusOK)
	}
	count, err := w.ResponseWriter.Write(body)
	w.bytes += int64(count)
	return count, err
}

func (w *ResponseTracker) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// Committed reports whether the response status line has been written.
func (w *ResponseTracker) Committed() bool { return w.wrote }

// Status returns the written status code, defaulting to 200 before commitment.
func (w *ResponseTracker) Status() int {
	if w.status == 0 {
		return http.StatusOK
	}
	return w.status
}

// BytesWritten returns the number of response body bytes written so far.
func (w *ResponseTracker) BytesWritten() int64 { return w.bytes }

// Route returns the matched route pattern, or an empty string when nothing
// reported one.
func (w *ResponseTracker) Route() string { return w.route }

// SetRoute records the matched route pattern.
func (w *ResponseTracker) SetRoute(pattern string) { w.route = pattern }

// SetRoute records the route a response belongs to, so a metric can be keyed by
// it instead of by the raw path, which is unbounded.
//
// It is a function over an http.ResponseWriter rather than a method because the
// caller holds whatever the chain handed it, which may be this tracker or a
// writer wrapping one. A writer that is neither reports no route, and a metric
// with no route attribute is what an unmatched request gets — never the path.
func SetRoute(w http.ResponseWriter, pattern string) {
	if pattern == "" {
		return
	}
	for current := w; current != nil; {
		if tracker, ok := current.(*ResponseTracker); ok {
			tracker.SetRoute(pattern)
			return
		}
		unwrapper, ok := current.(interface{ Unwrap() http.ResponseWriter })
		if !ok {
			return
		}
		current = unwrapper.Unwrap()
	}
}

// RouteOf reports the route recorded on w, following the same unwrap chain.
func RouteOf(w http.ResponseWriter) string {
	for current := w; current != nil; {
		if tracker, ok := current.(*ResponseTracker); ok {
			return tracker.Route()
		}
		unwrapper, ok := current.(interface{ Unwrap() http.ResponseWriter })
		if !ok {
			return ""
		}
		current = unwrapper.Unwrap()
	}
	return ""
}

func (w *ResponseTracker) Flush() {
	if !w.wrote {
		w.WriteHeader(http.StatusOK)
	}
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (w *ResponseTracker) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("popcornwave: response writer does not support hijacking")
	}
	return hijacker.Hijack()
}

func (w *ResponseTracker) Push(target string, options *http.PushOptions) error {
	if pusher, ok := w.ResponseWriter.(http.Pusher); ok {
		return pusher.Push(target, options)
	}
	return http.ErrNotSupported
}

func (w *ResponseTracker) ReadFrom(reader io.Reader) (int64, error) {
	if !w.wrote {
		w.WriteHeader(http.StatusOK)
	}
	if readerFrom, ok := w.ResponseWriter.(io.ReaderFrom); ok {
		count, err := readerFrom.ReadFrom(reader)
		w.bytes += count
		return count, err
	}
	count, err := io.Copy(struct{ io.Writer }{w}, reader)
	return count, err
}

// Track installs a ResponseTracker so downstream middleware and handlers can
// read the response status, size, and commitment state.
func Track(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(&ResponseTracker{ResponseWriter: w}, r)
	})
}
