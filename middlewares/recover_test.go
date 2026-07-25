package middlewares

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRecoverPassesPanicToHandler(t *testing.T) {
	var received error
	handler := Recover(func(w http.ResponseWriter, _ *http.Request, err error) {
		received = err
		w.WriteHeader(http.StatusBadGateway)
	})(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("boom") }))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if received == nil || !strings.Contains(received.Error(), "panic: boom") {
		t.Fatalf("recovered error = %v", received)
	}
	if response.Code != http.StatusBadGateway {
		t.Fatalf("status = %d", response.Code)
	}
}

func TestRecoverDefaultHandlerLeavesCommittedResponseAlone(t *testing.T) {
	response := httptest.NewRecorder()
	Track(Recover(nil)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		_, _ = io.WriteString(w, "partial")
		panic("late")
	}))).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != http.StatusAccepted || response.Body.String() != "partial" {
		t.Fatalf("committed response was rewritten: %d %q", response.Code, response.Body)
	}

	fresh := httptest.NewRecorder()
	Track(Recover(nil)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("early")
	}))).ServeHTTP(fresh, httptest.NewRequest(http.MethodGet, "/", nil))
	if fresh.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", fresh.Code)
	}
}

func TestCommittedUnwrapsWrappedWriters(t *testing.T) {
	tracker := &ResponseTracker{ResponseWriter: httptest.NewRecorder()}
	wrapped := &ResponseTracker{ResponseWriter: tracker}
	if Committed(wrapped) {
		t.Fatal("uncommitted response reported as committed")
	}
	tracker.WriteHeader(http.StatusOK)
	if !Committed(wrapped) {
		t.Fatal("committed response was not detected through the wrapper")
	}
}
