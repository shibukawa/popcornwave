package middlewares

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestResponseTrackerRecordsStatusAndSize(t *testing.T) {
	var tracker *ResponseTracker
	Track(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		tracker = w.(*ResponseTracker)
		if tracker.Committed() || tracker.Status() != http.StatusOK {
			t.Errorf("uncommitted tracker = %v/%d", tracker.Committed(), tracker.Status())
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, "created")
	})).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	if !tracker.Committed() || tracker.Status() != http.StatusCreated || tracker.BytesWritten() != 7 {
		t.Fatalf("tracker = %v/%d/%d", tracker.Committed(), tracker.Status(), tracker.BytesWritten())
	}
}

func TestResponseTrackerForwardsInformationalStatus(t *testing.T) {
	response := httptest.NewRecorder()
	tracker := &ResponseTracker{ResponseWriter: response}
	tracker.WriteHeader(http.StatusEarlyHints)
	if tracker.Committed() {
		t.Fatal("informational status committed the response")
	}
	tracker.WriteHeader(http.StatusNoContent)
	tracker.WriteHeader(http.StatusTeapot)
	if tracker.Status() != http.StatusNoContent {
		t.Fatalf("status = %d", tracker.Status())
	}
}

func TestResponseTrackerReadFromCountsBytes(t *testing.T) {
	tracker := &ResponseTracker{ResponseWriter: httptest.NewRecorder()}
	count, err := tracker.ReadFrom(strings.NewReader("streamed"))
	if err != nil {
		t.Fatal(err)
	}
	if count != 8 || tracker.BytesWritten() != 8 || tracker.Status() != http.StatusOK {
		t.Fatalf("read from = %d, tracked = %d, status = %d", count, tracker.BytesWritten(), tracker.Status())
	}
}
