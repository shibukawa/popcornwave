package pwfast

import (
	"slices"
	"strings"
	"testing"

	"github.com/shibukawa/popcornwave/pwruntime"
	"github.com/shibukawa/tinygodriver/fasthttp"
)

// keepRegistrations restores the process list afterwards, because every other
// test in this package composes a chain and would otherwise serve requests
// through frames this one registered.
func keepRegistrations(t *testing.T) {
	t.Helper()
	applicationMiddleware.Lock()
	previous := append([]Frame(nil), applicationMiddleware.frames...)
	applicationMiddleware.Unlock()
	t.Cleanup(func() {
		applicationMiddleware.Lock()
		applicationMiddleware.frames = previous
		applicationMiddleware.Unlock()
	})
}

// marking returns a middleware that records its own entry into order, so a test
// can read the composed chain off one request.
func marking(name string, order *[]string) Middleware {
	return func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
		return func(r *fasthttp.RequestCtx) {
			*order = append(*order, name)
			next(r)
		}
	}
}

func TestRegisteredMiddlewareComposesBySlotThenRegistrationOrder(t *testing.T) {
	keepRegistrations(t)
	publishChainSettings(t, pwruntime.ChainSettings{RequestID: true})

	order := []string{}
	// Registered out of slot order on purpose: the number line decides, and two
	// middlewares at one number keep their registration order.
	RegisterMiddleware(SlotGuard+5, "test-inner", marking("inner", &order))
	RegisterMiddleware(SlotAccessLog-5, "test-outer", marking("outer", &order))
	RegisterMiddleware(SlotGuard+5, "test-inner-second", marking("inner-second", &order))

	handler, err := Middlewares(func(r *fasthttp.RequestCtx) {
		r.SetStatusCode(fasthttp.StatusNoContent)
	}, RuntimeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	status, header, _ := serve(t, handler, "/")
	if status != fasthttp.StatusNoContent {
		t.Fatalf("status = %d", status)
	}
	if want := []string{"outer", "inner", "inner-second"}; !slices.Equal(order, want) {
		t.Fatalf("chain order = %v, want %v", order, want)
	}
	// The outer middleware sits inside the request ID frame, so the ID the
	// response reports was already minted when it ran.
	if !strings.Contains(strings.ToLower(header), "x-request-id:") {
		t.Errorf("the request ID frame did not run:\n%s", header)
	}
}

// A frame sharing a number with a framework frame runs inside it, so an
// application cannot displace the frame it registered beside.
func TestRegisteredMiddlewareRunsInsideTheFrameworkFrameAtItsSlot(t *testing.T) {
	keepRegistrations(t)
	publishChainSettings(t, pwruntime.ChainSettings{RequestID: true})

	var seen string
	RegisterMiddleware(SlotRequestID, "test-shared-slot", func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
		return func(r *fasthttp.RequestCtx) {
			seen = string(r.Response.Header.Peek("X-Request-ID"))
			next(r)
		}
	})

	handler, err := Middlewares(func(*fasthttp.RequestCtx) {}, RuntimeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	serve(t, handler, "/")
	if seen == "" {
		t.Error("the registered frame ran outside the framework frame it shares a slot with")
	}
}

func TestRegisterMiddlewareRefusesFixedFrames(t *testing.T) {
	keepRegistrations(t)
	for _, slot := range []Slot{SlotOperational, SlotAPIDoc} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("registration at fixed frame %d was accepted", slot)
				}
			}()
			RegisterMiddleware(slot, "test-fixed-frame", func(next fasthttp.RequestHandler) fasthttp.RequestHandler { return next })
		}()
	}
}

func TestRegisterMiddlewareRefusesNilMiddleware(t *testing.T) {
	keepRegistrations(t)
	defer func() {
		if recover() == nil {
			t.Fatal("nil middleware was accepted")
		}
	}()
	RegisterMiddleware(SlotAccessLog+5, "test-nil", nil)
}

func TestRegisterMiddlewareRefusesDuplicateName(t *testing.T) {
	keepRegistrations(t)
	RegisterMiddleware(SlotAccessLog+5, "test-duplicate", func(next fasthttp.RequestHandler) fasthttp.RequestHandler { return next })
	defer func() {
		if recover() == nil {
			t.Fatal("duplicate middleware name was accepted")
		}
	}()
	RegisterMiddleware(SlotAccessLog+6, "test-duplicate", func(next fasthttp.RequestHandler) fasthttp.RequestHandler { return next })
}
