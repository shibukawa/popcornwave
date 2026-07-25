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
