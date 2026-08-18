package middlewares

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/shibukawa/popcornweb/pwruntime"
)

func TestAccessLogRecordsTrackedResponse(t *testing.T) {
	var logged bytes.Buffer
	backend := pwruntime.NewLogBackend(pwruntime.LevelInfo, pwruntime.NewSlogSink(slog.NewTextHandler(&logged, nil)))
	handler := Track(InjectResources(pwruntime.Resources{Log: backend})(AccessLog()(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusCreated)
			_, _ = io.WriteString(w, "created")
		}))))
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/items", nil))

	record := logged.String()
	for _, fragment := range []string{"request completed", "method=POST", "path=/items", "status=201", "bytes=7"} {
		if !strings.Contains(record, fragment) {
			t.Fatalf("access log missing %q: %s", fragment, record)
		}
	}
}
