package middlewares

import (
	"fmt"
	"net/http"

	"github.com/shibukawa/popcornwave/pwruntime"
)

// PanicHandler writes the response for a panic recovered by Recover.
type PanicHandler func(w http.ResponseWriter, r *http.Request, err error)

// Recover converts a panic into a response written by handler. A nil handler
// logs the panic and writes a plain 500 when the response is not committed.
func Recover(handler PanicHandler) Middleware {
	if handler == nil {
		handler = writePanicStatus
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if recovered := recover(); recovered != nil {
					handler(w, r, fmt.Errorf("panic: %v", recovered))
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

func writePanicStatus(w http.ResponseWriter, r *http.Request, err error) {
	pwruntime.Logger(r.Context()).ErrorContext(r.Context(), "recovered panic", "error", err)
	if Committed(w) {
		return
	}
	http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}

// Committed reports whether w, or any writer it wraps, already wrote a status
// line. Writers opt in by implementing Committed() bool, as ResponseTracker does.
func Committed(w http.ResponseWriter) bool {
	for w != nil {
		if committed, ok := w.(interface{ Committed() bool }); ok && committed.Committed() {
			return true
		}
		unwrapper, ok := w.(interface{ Unwrap() http.ResponseWriter })
		if !ok {
			return false
		}
		next := unwrapper.Unwrap()
		if next == w {
			return false
		}
		w = next
	}
	return false
}
