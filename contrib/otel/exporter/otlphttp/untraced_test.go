package otlphttp

import (
	"net/http"
	"testing"

	"github.com/shibukawa/popcornweb/contrib/otel/otelhttp"
)

type namedTransport struct{ name string }

func (t *namedTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
}

// An authenticating or retrying frame is how the export reaches its endpoint,
// so it survives; only the tracing frame is what would feed itself.
type authTransport struct{ base http.RoundTripper }

func (t *authTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	return t.base.RoundTrip(r)
}

// Exporting through a traced client never terminates: the export opens a span,
// ending it enqueues a record, and flushing the queue exports again.
func TestExporterRefusesATracedClient(t *testing.T) {
	inner := &namedTransport{name: "real"}
	exporter, err := New(Config{
		Endpoint: "https://collector.example.com",
		Client:   otelhttp.NewClient(&http.Client{Transport: inner}),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, traced := exporter.client.Transport.(*otelhttp.Transport); traced {
		t.Fatal("the exporter kept a tracing transport, so exporting a span opens a span")
	}
	if exporter.client.Transport != http.RoundTripper(inner) {
		t.Errorf("transport = %T, want the wrapped one back", exporter.client.Transport)
	}
}

func TestUntracedKeepsEverythingThatIsNotTracing(t *testing.T) {
	inner := &namedTransport{name: "real"}
	auth := &authTransport{base: inner}

	// Tracing above a frame the deployment installed: the tracing goes, the
	// frame stays, because dropping it would break the export to protect it.
	client := &http.Client{Transport: otelhttp.NewTransport(auth)}
	if got := untraced(client).Transport; got != http.RoundTripper(auth) {
		t.Errorf("transport = %T, want the auth frame preserved", got)
	}

	// A chain with no tracing in it is returned untouched, same pointer.
	plain := &http.Client{Transport: auth}
	if untraced(plain) != plain {
		t.Error("a client with no tracing was copied for no reason")
	}

	// Tracing stacked twice is still fully removed.
	doubled := &http.Client{Transport: otelhttp.NewTransport(otelhttp.NewTransport(inner))}
	if got := untraced(doubled).Transport; got != http.RoundTripper(inner) {
		t.Errorf("transport = %T, want both tracing frames gone", got)
	}
}

// The client belongs to the caller and may be the one the application uses for
// its own outbound calls, which should stay traced.
func TestUntracedLeavesTheCallersClientAlone(t *testing.T) {
	traced := otelhttp.NewTransport(&namedTransport{name: "real"})
	client := &http.Client{Transport: traced}
	if _, err := New(Config{Endpoint: "https://collector.example.com", Client: client}); err != nil {
		t.Fatal(err)
	}
	if client.Transport != http.RoundTripper(traced) {
		t.Error("the caller's client lost its tracing transport")
	}
}
