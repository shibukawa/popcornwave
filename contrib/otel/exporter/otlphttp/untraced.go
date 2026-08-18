package otlphttp

import (
	"net/http"

	"github.com/shibukawa/popcornweb/contrib/otel/otelhttp"
)

// untraced returns client with any tracing frames taken off its transport.
//
// Exporting through a traced client does not merely add noise, it does not
// terminate: the export opens a client span, ending that span enqueues a record,
// the queue flushes by exporting, and that export opens a span. The queue is
// bounded, so the process does not die of it; it simply never empties, and every
// real span waits behind spans about sending spans.
//
// The exclusion lives here rather than as a URL check inside the transport
// because a URL check is defeated by the endpoint being reconfigured, and this
// is not. A caller that hands over an instrumented client — the obvious thing to
// do once one exists and the application has a single shared client — gets a
// working exporter instead of a silent stall.
//
// What it reaches is tracing at the head of the chain, which is where a wrapper
// put there by NewClient or NewTransport sits. A tracing frame buried under
// another wrapper stays, because removing it would mean rebuilding a chain out
// of wrapper types this package does not know how to reassemble. Everything
// that is not tracing is left exactly where it was: an authenticating or
// retrying frame the deployment installed is part of how the export reaches its
// endpoint, and dropping it would break the export to protect it.
func untraced(client *http.Client) *http.Client {
	// Whether anything was removed is tracked rather than compared: a
	// RoundTripper is often a func type, and comparing two interface values
	// holding one panics at run time instead of reporting false.
	stripped, removed := client.Transport, false
	for {
		tracing, ok := stripped.(*otelhttp.Transport)
		if !ok {
			break
		}
		stripped, removed = tracing.Unwrap(), true
	}
	if !removed {
		return client
	}
	// The caller's client is left alone; it is theirs, and it may be the one the
	// application uses for its own outbound calls, which should stay traced.
	copied := *client
	copied.Transport = stripped
	return &copied
}
