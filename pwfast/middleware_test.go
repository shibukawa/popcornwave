package pwfast

import (
	"strings"
	"testing"

	"github.com/shibukawa/popcornwave/pwruntime"
	"github.com/shibukawa/tinygodriver/fasthttp"
)

func TestChainRunsOutermostFirst(t *testing.T) {
	var order []string
	mark := func(name string) Middleware {
		return func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
			return func(r *fasthttp.RequestCtx) {
				order = append(order, name)
				next(r)
			}
		}
	}
	handler := Chain(func(r *fasthttp.RequestCtx) { order = append(order, "handler") },
		mark("a"), mark("b"))
	serve(t, handler, "/")

	if strings.Join(order, ",") != "a,b,handler" {
		t.Errorf("order = %v, want a,b,handler", order)
	}
}

func TestRequestIDIsMintedAndEchoed(t *testing.T) {
	handler := Chain(func(*fasthttp.RequestCtx) {}, RequestID())
	_, header, _ := serve(t, handler, "/")
	if !strings.Contains(header, "X-Request-Id:") && !strings.Contains(header, "X-Request-ID:") {
		t.Errorf("no request ID on the response:\n%s", header)
	}
}

func TestAUsableClientRequestIDIsKeptAndAnUnusableOneIsReplaced(t *testing.T) {
	handler := Chain(func(*fasthttp.RequestCtx) {}, RequestID())

	_, header, _ := serveRaw(t, handler, "/", "X-Request-ID: abc-123\r\n")
	if !strings.Contains(header, "abc-123") {
		t.Errorf("a usable client ID was not kept:\n%s", header)
	}

	// A value outside printable ASCII could terminate the header or inject
	// another one, so it is replaced rather than echoed.
	_, header, _ = serveRaw(t, handler, "/", "X-Request-ID: bad\tvalue\r\n")
	if strings.Contains(header, "bad") {
		t.Errorf("an unusable client ID was echoed:\n%s", header)
	}
}

// The recording is the half that differs between the transports, so it is the
// half worth asserting: a frame writes into the request value in place, and
// what reads it afterwards is the shared reader.
func TestTheRequestIDReachesTheLoggerThroughTheRequestValue(t *testing.T) {
	handler := Chain(func(r *fasthttp.RequestCtx) {
		for _, attribute := range pwruntime.DeriveResources(r).LogAttributes {
			if attribute.Key == "request_id" {
				value, _ := attribute.Value.AsString()
				_, _ = r.WriteString(value)
				return
			}
		}
		_, _ = r.WriteString("absent")
	}, RequestID())

	_, _, body := serveRaw(t, handler, "/", "X-Request-ID: correlate-me\r\n")
	if body != "correlate-me" {
		t.Errorf("the handler read %q rather than the recorded request ID", body)
	}
}
