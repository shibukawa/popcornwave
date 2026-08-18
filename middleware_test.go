package petitweb_test

import (
	"context"
	"crypto/tls"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	petitweb "github.com/shibukawa/popcornweb"
	"github.com/shibukawa/popcornweb/pwruntime"
)

func TestRequestIDAndContextAccessors(t *testing.T) {
	sink := pwruntime.NewCaptureSink()
	base := pwruntime.NewLogger(context.Background(), pwruntime.NewLogBackend(pwruntime.LevelInfo, sink))
	handler := petitweb.RequestID("", base)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := petitweb.ReadRequestID(r.Context())
		if !ok || id != "client-id" {
			t.Fatalf("request id = %q, %v", id, ok)
		}
		petitweb.ReadLogger(r.Context()).Info("handled")
		w.WriteHeader(http.StatusNoContent)
	}))
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("X-Request-ID", "client-id")
	handler.ServeHTTP(recorder, request)
	if recorder.Header().Get("X-Request-ID") != "client-id" {
		t.Fatalf("response request ID = %q", recorder.Header().Get("X-Request-ID"))
	}
	records := sink.Records()
	if len(records) != 1 || records[0].Text("request_id") != "client-id" {
		t.Fatalf("the request logger did not carry the correlation ID: %#v", records)
	}
	// A context that never reached the middleware still yields a usable logger.
	if logger := petitweb.ReadLogger(nil); !logger.Enabled(petitweb.LevelError) {
		t.Fatal("fallback logger cannot be called")
	}
}

func TestSecurityHeaders(t *testing.T) {
	config := petitweb.DefaultSecurityHeaders()
	config.HSTS = petitweb.HSTSConfig{Enabled: true, MaxAge: 365 * 24 * time.Hour, IncludeSubdomains: true, Preload: true}
	middleware, err := petitweb.SecurityHeaders(config)
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	if recorder.Header().Get("X-Content-Type-Options") != "nosniff" || recorder.Header().Get("X-Frame-Options") != "DENY" {
		t.Fatalf("security headers = %v", recorder.Header())
	}
	if recorder.Header().Get("Strict-Transport-Security") != "" {
		t.Fatal("HSTS emitted over HTTP")
	}
	httpsRecorder := httptest.NewRecorder()
	httpsRequest := httptest.NewRequest(http.MethodGet, "/", nil)
	httpsRequest.TLS = &tls.ConnectionState{}
	middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {})).ServeHTTP(httpsRecorder, httpsRequest)
	if got := httpsRecorder.Header().Get("Strict-Transport-Security"); got != "max-age=31536000; includeSubDomains; preload" {
		t.Fatalf("HSTS = %q", got)
	}
	bad := petitweb.DefaultSecurityHeaders()
	bad.ContentSecurityPolicy = "default-src 'self'\r\nX-Evil: yes"
	if _, err := petitweb.SecurityHeaders(bad); err == nil {
		t.Fatal("accepted response splitting")
	}
}

func TestJSONDoesNotCommitEncodingFailure(t *testing.T) {
	recorder := httptest.NewRecorder()
	if err := petitweb.JSON(recorder, http.StatusCreated, make(chan int)); err == nil {
		t.Fatal("JSON accepted unsupported value")
	}
	if recorder.Body.Len() != 0 || recorder.Header().Get("Content-Type") != "" {
		t.Fatalf("response was committed: headers=%v body=%q", recorder.Header(), recorder.Body.String())
	}
}

func TestHTMLBuffersBeforeCommit(t *testing.T) {
	recorder := httptest.NewRecorder()
	err := petitweb.HTML(recorder, http.StatusOK, petitweb.RenderFunc(func(w io.Writer) error {
		_, _ = w.Write([]byte("partial"))
		return io.ErrShortWrite
	}))
	if err == nil {
		t.Fatal("HTML returned nil error")
	}
	if recorder.Body.Len() != 0 || recorder.Code != http.StatusOK {
		t.Fatalf("response was committed: %d %q", recorder.Code, recorder.Body.String())
	}
}
