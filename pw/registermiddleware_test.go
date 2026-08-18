package pw

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"testing/fstest"

	"github.com/shibukawa/tinybind-go/configbind"
)

// marking returns a middleware that records its own entry into order, so a
// test can read the composed chain off one request.
func marking(name string, order *[]string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			*order = append(*order, name)
			next.ServeHTTP(w, r)
		})
	}
}

func TestRegisteredMiddlewareComposesBySlotThenRegistrationOrder(t *testing.T) {
	order := []string{}
	// Registered out of slot order on purpose: the number line decides, and
	// two middlewares at one number keep their registration order.
	RegisterMiddleware(SlotGuard+5, "test-inner", marking("inner", &order))
	RegisterMiddleware(SlotAccessLog-5, "test-outer", marking("outer", &order))
	RegisterMiddleware(SlotGuard+5, "test-inner-second", marking("inner-second", &order))

	// Another test may already have parsed the configuration, which freezes
	// the load options; either state serves this test, so the refusal is
	// swallowed rather than coordinated around.
	func() {
		defer func() { _ = recover() }()
		SetConfigLoadOptions(configbind.LoadOptions{
			Vendor: "popcornweb-test", Tool: "pw-middleware-test", FileName: "missing.toml",
		})
	}()
	handler, err := Middlewares(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}), WithPublicFS(fstest.MapFS{".keep": {Data: nil}}))
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d", recorder.Code)
	}
	if want := []string{"outer", "inner", "inner-second"}; !slices.Equal(order, want) {
		t.Fatalf("chain order = %v, want %v", order, want)
	}
	// The outer middleware sits inside the request ID frame, so the ID the
	// response reports was already minted when it ran.
	if recorder.Header().Get("X-Request-ID") == "" {
		t.Fatal("request ID frame did not run")
	}
}

func TestRegisterMiddlewareRefusesFixedFrames(t *testing.T) {
	for _, slot := range []Slot{SlotOperational, SlotAPIDoc} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("registration at fixed frame %d was accepted", slot)
				}
			}()
			RegisterMiddleware(slot, "test-fixed-frame", func(next http.Handler) http.Handler { return next })
		}()
	}
}

func TestRegisterMiddlewareRefusesNilMiddleware(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("nil middleware was accepted")
		}
	}()
	RegisterMiddleware(SlotAccessLog+5, "test-nil", nil)
}

func TestRegisterMiddlewareRefusesDuplicateName(t *testing.T) {
	RegisterMiddleware(SlotAccessLog+5, "test-duplicate", func(next http.Handler) http.Handler { return next })
	defer func() {
		if recover() == nil {
			t.Fatal("duplicate middleware name was accepted")
		}
	}()
	RegisterMiddleware(SlotAccessLog+6, "test-duplicate", func(next http.Handler) http.Handler { return next })
}
