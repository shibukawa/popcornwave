package middlewares

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestLifecycleHeadersWritesBothDateFormatsAndLink(t *testing.T) {
	deprecated := time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC)
	sunset := time.Date(2026, time.August, 31, 0, 0, 0, 0, time.FixedZone("JST", 9*60*60))
	middleware, err := LifecycleHeaders(Lifecycle{
		DeprecatedAt:     deprecated,
		SunsetAt:         sunset,
		DocumentationURL: "https://example.com/migrations/v2",
	})
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/v1/items", nil))

	if got, want := recorder.Header().Get("Deprecation"), "@"+strconv.FormatInt(deprecated.Unix(), 10); got != want {
		t.Errorf("Deprecation = %q, want %q", got, want)
	}
	if got, want := recorder.Header().Get("Sunset"), "Sun, 30 Aug 2026 15:00:00 GMT"; got != want {
		t.Errorf("Sunset = %q, want %q", got, want)
	}
	if got, want := recorder.Header().Get("Link"), `<https://example.com/migrations/v2>; rel="deprecation"`; got != want {
		t.Errorf("Link = %q, want %q", got, want)
	}
}

func TestLifecycleHeadersAllowsIndependentSignals(t *testing.T) {
	tests := []struct {
		name         string
		lifecycle    Lifecycle
		present      string
		absent       string
		linkRelation string
	}{
		{"deprecation", Lifecycle{DeprecatedAt: time.Unix(1, 0), DocumentationURL: "https://example.com/deprecation"}, "Deprecation", "Sunset", "deprecation"},
		{"sunset", Lifecycle{SunsetAt: time.Unix(2, 0), DocumentationURL: "https://example.com/sunset"}, "Sunset", "Deprecation", "sunset"},
		{"policy link", Lifecycle{DocumentationURL: "https://example.com/policy"}, "Link", "Deprecation", "deprecation"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			middleware, err := LifecycleHeaders(tc.lifecycle)
			if err != nil {
				t.Fatal(err)
			}
			recorder := httptest.NewRecorder()
			middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
			if recorder.Header().Get(tc.present) == "" {
				t.Errorf("%s header is absent", tc.present)
			}
			if recorder.Header().Get(tc.absent) != "" {
				t.Errorf("%s header = %q", tc.absent, recorder.Header().Get(tc.absent))
			}
			if !strings.Contains(recorder.Header().Get("Link"), `rel="`+tc.linkRelation+`"`) {
				t.Errorf("Link = %q", recorder.Header().Get("Link"))
			}
		})
	}
}

func TestLifecycleHeadersRejectsInvalidConfiguration(t *testing.T) {
	deprecated := time.Unix(20, 0)
	tests := []Lifecycle{
		{},
		{DeprecatedAt: deprecated, SunsetAt: deprecated.Add(-time.Second)},
		{DocumentationURL: "relative/path"},
		{DocumentationURL: "https://example.com/>\r\nBad: value"},
	}
	for _, lifecycle := range tests {
		if _, err := LifecycleHeaders(lifecycle); err == nil {
			t.Errorf("LifecycleHeaders(%#v) succeeded", lifecycle)
		}
	}
}
