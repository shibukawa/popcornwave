package middlewares

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/shibukawa/petitweb-go/contrib/otel/trace"
)

type middlewareCollector struct{ spans []trace.SpanData }

func (c *middlewareCollector) OnEnd(span trace.SpanData)    { c.spans = append(c.spans, span) }
func (*middlewareCollector) Shutdown(context.Context) error { return nil }

func TestOtelCreatesServerAndChildSpans(t *testing.T) {
	collector := &middlewareCollector{}
	provider := trace.NewProvider(collector)
	handler := Otel(WithTracerProvider(provider))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if TraceID(r.Context()) == "" || SpanID(r.Context()) == "" {
			t.Error("missing IDs in request context")
		}
		_, child := StartSpan(r.Context(), "work")
		child.End()
		w.WriteHeader(http.StatusInternalServerError)
	}))
	request := httptest.NewRequest(http.MethodGet, "https://example.com/items?q=1", nil)
	request.Header.Set("traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if len(collector.spans) != 2 {
		t.Fatalf("spans = %d", len(collector.spans))
	}
	server := collector.spans[1]
	if server.Kind != trace.SpanKindServer || server.Status != trace.StatusError {
		t.Fatalf("server kind/status = %d/%d", server.Kind, server.Status)
	}
	if server.SpanContext.TraceID() != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Fatalf("trace ID = %q", server.SpanContext.TraceID())
	}
}

func TestOtelEndsSpanOnPanic(t *testing.T) {
	collector := &middlewareCollector{}
	handler := Otel(WithTracerProvider(trace.NewProvider(collector)))(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	}))
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("panic was swallowed")
			}
		}()
		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	}()
	if len(collector.spans) != 1 || collector.spans[0].Status != trace.StatusError {
		t.Fatalf("completed spans = %#v", collector.spans)
	}
}
