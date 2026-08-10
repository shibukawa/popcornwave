package pwfast

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/shibukawa/popcornwave/pwruntime"
	"github.com/shibukawa/tinygodriver/fasthttp"
	"github.com/shibukawa/tinygodriver/fasthttp/fasthttputil"
)

func publishChainSettings(t *testing.T, settings pwruntime.ChainSettings) {
	t.Helper()
	previous, had := pwruntime.ResolvedChainSettings()
	pwruntime.PublishChainSettings(settings)
	t.Cleanup(func() {
		if had {
			pwruntime.PublishChainSettings(previous)
		}
	})
}

// A chain built from nothing would have no recovery frame, no request ID and no
// security headers, and would still serve requests. Refusing is the difference
// between a missing configuration and a silently reduced one.
func TestMiddlewaresRefusesWithoutPublishedSettings(t *testing.T) {
	if _, err := pwruntime.ResolvedChainSettings(); err {
		t.Skip("settings are already published in this binary")
	}
	_, err := Middlewares(func(*fasthttp.RequestCtx) {}, RuntimeOptions{})
	if err == nil {
		t.Fatal("a chain was built with no published settings")
	}
	if !strings.Contains(err.Error(), "chain settings") {
		t.Errorf("the refusal did not name the cause: %v", err)
	}
}

func TestMiddlewaresInstallsTheConfiguredFrames(t *testing.T) {
	publishChainSettings(t, pwruntime.ChainSettings{
		RequestID:       true,
		Recovery:        true,
		SecurityHeaders: pwruntime.DefaultSecurityHeaders(),
	})

	handler, err := Middlewares(func(r *fasthttp.RequestCtx) {
		_, _ = r.WriteString("served")
	}, RuntimeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	_, header, body := serve(t, handler, "/")

	if body != "served" {
		t.Errorf("body = %q", body)
	}
	if !strings.Contains(strings.ToLower(header), "x-request-id:") {
		t.Errorf("the request ID frame was not installed:\n%s", header)
	}
	if !strings.Contains(header, "X-Frame-Options: DENY") {
		t.Errorf("the security header frame was not installed:\n%s", header)
	}
}

// A frame a deployment turned off must be absent rather than present and inert,
// because an inert frame is one nobody can tell is not working.
func TestAFrameTurnedOffIsNotInstalled(t *testing.T) {
	publishChainSettings(t, pwruntime.ChainSettings{RequestID: false})

	handler, err := Middlewares(func(*fasthttp.RequestCtx) {}, RuntimeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	_, header, _ := serve(t, handler, "/")
	if strings.Contains(strings.ToLower(header), "x-request-id:") {
		t.Errorf("a disabled frame still ran:\n%s", header)
	}
}

// The slot numbers are shared so both transports compose in one order, and the
// order is what a guard's correctness rests on.
func TestFramesComposeInSlotOrder(t *testing.T) {
	var order []string
	mark := func(name string) Middleware {
		return func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
			return func(r *fasthttp.RequestCtx) {
				order = append(order, name)
				next(r)
			}
		}
	}
	handler := Compose(func(*fasthttp.RequestCtx) { order = append(order, "handler") },
		Frame{Slot: SlotGuard, Name: "guard", Middleware: mark("guard")},
		Frame{Slot: SlotResources, Name: "resources", Middleware: mark("resources")},
		Frame{Slot: SlotSession, Name: "session", Middleware: mark("session")},
	)
	serve(t, handler, "/")

	// Ascending slot, smallest outermost: resources 20, session 120, guard 150.
	if got := strings.Join(order, ","); got != "resources,session,guard,handler" {
		t.Errorf("order = %s", got)
	}
}

func TestServeStopsWhenTheContextIsCancelled(t *testing.T) {
	listener := fasthttputil.NewInmemoryListener()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Serve(ctx, listener, func(*fasthttp.RequestCtx) {}) }()

	// Reach it once so the server is certainly up before the cancel.
	conn, err := listener.Dial()
	if err != nil {
		t.Fatal(err)
	}
	_, _ = conn.Write([]byte("GET / HTTP/1.1\r\nHost: t\r\nConnection: close\r\n\r\n"))
	_ = conn.Close()

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Serve returned %v, want a clean shutdown", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Serve did not return after its context was cancelled")
	}
}

// The resolved caller is what every downstream bound counts against, so it has
// to be recorded where the shared readers look for it.
func TestResolveClientAddressRecordsThePeer(t *testing.T) {
	_, everything, _ := net.ParseCIDR("0.0.0.0/0")
	handler := Compose(func(r *fasthttp.RequestCtx) {
		_, _ = r.WriteString(ClientAddress(r))
	}, Frame{Slot: SlotClientAddress, Middleware: ResolveClientAddress([]*net.IPNet{everything})})

	_, _, body := serveRaw(t, handler, "/", "X-Forwarded-For: 203.0.113.9\r\n")
	if body != "203.0.113.9" {
		t.Errorf("client address = %q, want the forwarded caller", body)
	}
}

func TestTheProbesAnswerAboveTheApplication(t *testing.T) {
	publishChainSettings(t, pwruntime.ChainSettings{Health: "/healthz", Readiness: "/readyz"})
	handler, err := Middlewares(func(r *fasthttp.RequestCtx) {
		_, _ = r.WriteString("application")
	}, RuntimeOptions{})
	if err != nil {
		t.Fatal(err)
	}

	status, header, body := serve(t, handler, "/healthz")
	if status != fasthttp.StatusOK || body != "ok\n" {
		t.Errorf("liveness answered %d %q", status, body)
	}
	if !strings.Contains(header, "Cache-Control: no-store") {
		t.Errorf("a probe answer was cacheable:\n%s", header)
	}
	// No connections configured means ready, which is the same answer the other
	// transport gives for the same process.
	if status, _, body := serve(t, handler, "/readyz"); status != fasthttp.StatusOK || body != "ok\n" {
		t.Errorf("readiness answered %d %q", status, body)
	}
	if _, _, body := serve(t, handler, "/"); body != "application" {
		t.Errorf("an ordinary path was taken by a probe: %q", body)
	}
}

// A probe that accepts any method is one an arbitrary caller can POST to, and
// on the readiness path that costs a database round trip per request.
func TestAProbeRefusesAMethodItDoesNotAnswer(t *testing.T) {
	publishChainSettings(t, pwruntime.ChainSettings{Health: "/healthz"})
	handler, err := Middlewares(func(*fasthttp.RequestCtx) {}, RuntimeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	status, header, _ := serveForm(t, handler, "/healthz", "x=1")
	if status != fasthttp.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", status)
	}
	if !strings.Contains(header, "Allow:") {
		t.Errorf("405 carried no Allow header:\n%s", header)
	}
}

func TestAnEmptyProbePathInstallsNothing(t *testing.T) {
	publishChainSettings(t, pwruntime.ChainSettings{})
	handler, err := Middlewares(func(r *fasthttp.RequestCtx) {
		_, _ = r.WriteString("application")
	}, RuntimeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, body := serve(t, handler, "/healthz"); body != "application" {
		t.Errorf("a disabled probe still answered: %q", body)
	}
}
