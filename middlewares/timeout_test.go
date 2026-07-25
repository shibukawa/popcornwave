package middlewares

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRequestTimeoutInstallsDeadline(t *testing.T) {
	var hasDeadline bool
	RequestTimeout(time.Second)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_, hasDeadline = r.Context().Deadline()
	})).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	if !hasDeadline {
		t.Fatal("request deadline was not installed")
	}
}

func TestRequestTimeoutWithoutTimeoutPassesThrough(t *testing.T) {
	var hasDeadline bool
	RequestTimeout(0)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_, hasDeadline = r.Context().Deadline()
	})).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	if hasDeadline {
		t.Fatal("deadline was installed for a disabled timeout")
	}
}
