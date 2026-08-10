package pw

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/shibukawa/popcornwave/middlewares"
	"github.com/shibukawa/popcornwave/pwruntime"
)

// TestRateLimitExemptionsCoverWhatTheFrameworkRoutes pins the carve-out that
// cannot be optional. A readiness probe arrives from the proxy on the same
// address as every anonymous caller, and one page view fetches many assets.
func TestRateLimitExemptionsCoverWhatTheFrameworkRoutes(t *testing.T) {
	server := ServerConfig{
		Health:     "/healthz",
		Readiness:  "/readyz",
		OpenAPI:    "/openapi.json",
		APIDoc:     "scalar",
		APIDocPath: "/docs",
		Public:     PublicConfig{Enabled: true, Mount: "/public"},
	}
	exempt := rateLimitExemptions(server)
	for _, want := range []string{"/healthz", "/readyz", "/openapi.json", "/docs", "/docs/**", "/public/**"} {
		if !contains(exempt, want) {
			t.Errorf("%q is not exempt; exemptions were %v", want, exempt)
		}
	}
	// An unset endpoint serves nothing, so it contributes no pattern rather
	// than an empty one that would fail to compile.
	bare := rateLimitExemptions(ServerConfig{Public: PublicConfig{Enabled: false}})
	if len(bare) != 0 {
		t.Errorf("a configuration serving no framework endpoint produced %v", bare)
	}
	// Every produced pattern has to compile, or the limiter refuses to start.
	if _, err := middlewares.RateLimit(enabledPwRateLimit(), middlewares.RateLimitDeps{
		Counter: middlewares.NewMemoryRateLimitCounter(),
		Exempt:  exempt,
	}); err != nil {
		t.Fatalf("the generated exemptions did not compile: %v", err)
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func enabledPwRateLimit() RateLimitConfig {
	config := middlewares.DefaultRateLimit()
	config.Enabled = true
	return config
}

// The two frames must straddle authentication: the identity bucket cannot know
// whether a caller has a subject until authentication resolved one, and the
// ceiling is worth less the further in it sits.
func TestRateLimitSlotsStraddleAuthentication(t *testing.T) {
	if !(SlotRecover < SlotRateLimitProcess && SlotRateLimitProcess < SlotSecurityHeaders) {
		t.Errorf("the process ceiling at %d is not between recover (%d) and security headers (%d)",
			SlotRateLimitProcess, SlotRecover, SlotSecurityHeaders)
	}
	if !(SlotAuthentication < SlotRateLimit && SlotRateLimit < SlotCSRF) {
		t.Errorf("the identity bucket at %d is not between authentication (%d) and CSRF (%d)",
			SlotRateLimit, SlotAuthentication, SlotCSRF)
	}
	// The resolved client address the bucket keys on must already be recorded.
	if !(SlotClientAddress < SlotRateLimit) {
		t.Errorf("the client address is resolved at %d, after the bucket at %d", SlotClientAddress, SlotRateLimit)
	}
}

func TestRateLimitSetupInstallsNothingWhenOff(t *testing.T) {
	ctx := rateLimitContext(RateLimitConfig{}, ServerConfig{})
	for name, setup := range map[string]func(context.Context) (Middleware, error){
		"identity": setupRateLimit,
		"process":  setupRateLimitProcess,
	} {
		middleware, err := setup(ctx)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if middleware != nil {
			t.Errorf("%s: a disabled limiter installed a frame", name)
		}
	}
}

func TestRateLimitSetupRefusesAnIncoherentConfiguration(t *testing.T) {
	config := enabledPwRateLimit()
	// The one count with no off position.
	config.PerAddress = 0
	if _, err := setupRateLimit(rateLimitContext(config, ServerConfig{})); err == nil {
		t.Fatal("a limiter with no address bound started")
	}
}

// End to end through the framework seam: the extension resolves a counter,
// bounds the caller, and answers with the framework's own problem response.
func TestRateLimitSetupBoundsACaller(t *testing.T) {
	t.Cleanup(releaseRateLimitStore)
	config := enabledPwRateLimit()
	config.PerAddress = 2
	server := ServerConfig{Health: "/healthz"}
	middleware, err := setupRateLimit(rateLimitContext(config, server))
	if err != nil {
		t.Fatalf("setupRateLimit: %v", err)
	}
	if middleware == nil {
		t.Fatal("an enabled limiter installed no frame")
	}
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	call := func(path string) int {
		r := httptest.NewRequest(http.MethodGet, path, nil)
		r = r.WithContext(pwruntime.WithClientAddress(r.Context(), "203.0.113.9"))
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, r)
		return response.Code
	}
	if code := call("/orders"); code != http.StatusNoContent {
		t.Fatalf("first request = %d", code)
	}
	if code := call("/orders"); code != http.StatusNoContent {
		t.Fatalf("second request = %d", code)
	}
	if code := call("/orders"); code != http.StatusTooManyRequests {
		t.Fatalf("third request = %d, want the bucket to bind", code)
	}
	// The probe never counted, so it still answers after the bucket is spent.
	if code := call("/healthz"); code != http.StatusNoContent {
		t.Fatalf("the health probe was refused with %d", code)
	}
}

func rateLimitContext(config RateLimitConfig, server ServerConfig) context.Context {
	return pwruntime.WithResources(context.Background(), pwruntime.Resources{
		Configs: map[reflect.Type]any{
			reflect.TypeFor[RateLimitConfig](): config,
			reflect.TypeFor[ServerConfig]():    server,
		},
	})
}
