package middlewares

import (
	"crypto/tls"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSecurityHeadersAppliesConfiguredPolicies(t *testing.T) {
	config := DefaultSecurityHeaders()
	config.HSTS = HSTSConfig{Enabled: true, MaxAge: 365 * 24 * time.Hour, IncludeSubdomains: true, Preload: true}
	middleware, err := SecurityHeaders(config)
	if err != nil {
		t.Fatal(err)
	}
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Header().Get("X-Content-Type-Options") != "nosniff" ||
		response.Header().Get("X-Frame-Options") != "DENY" ||
		response.Header().Get("Referrer-Policy") != "strict-origin-when-cross-origin" {
		t.Fatalf("headers = %v", response.Header())
	}
	if response.Header().Get("Strict-Transport-Security") != "" {
		t.Fatal("HSTS emitted over plain HTTP")
	}

	secure := httptest.NewRecorder()
	secureRequest := httptest.NewRequest(http.MethodGet, "/", nil)
	secureRequest.TLS = &tls.ConnectionState{}
	handler.ServeHTTP(secure, secureRequest)
	if got := secure.Header().Get("Strict-Transport-Security"); got != "max-age=31536000; includeSubDomains; preload" {
		t.Fatalf("HSTS = %q", got)
	}
}

func TestSecurityHeadersOmitsUnsetPolicies(t *testing.T) {
	middleware, err := SecurityHeaders(SecurityHeadersConfig{FrameOptions: "off"})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).
		ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	for _, name := range []string{"X-Content-Type-Options", "X-Frame-Options", "Referrer-Policy", "Content-Security-Policy", "Permissions-Policy"} {
		if _, ok := response.Header()[name]; ok {
			t.Fatalf("%s was set for an empty policy: %v", name, response.Header())
		}
	}
}

func TestSecurityHeadersTrustsForwardedProtoFromTrustedProxy(t *testing.T) {
	config := DefaultSecurityHeaders()
	config.HSTS = HSTSConfig{Enabled: true, MaxAge: time.Hour, IncludeSubdomains: true}
	_, trusted, err := net.ParseCIDR("10.0.0.0/8")
	if err != nil {
		t.Fatal(err)
	}
	middleware, err := SecurityHeaders(config, WithTrustedProxies([]*net.IPNet{trusted}))
	if err != nil {
		t.Fatal(err)
	}
	handler := middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	forwarded := httptest.NewRequest(http.MethodGet, "/", nil)
	forwarded.RemoteAddr = "10.1.2.3:1234"
	forwarded.Header.Set("X-Forwarded-Proto", "https")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, forwarded)
	if got := response.Header().Get("Strict-Transport-Security"); got != "max-age=3600; includeSubDomains" {
		t.Fatalf("HSTS = %q", got)
	}

	untrusted := httptest.NewRequest(http.MethodGet, "/", nil)
	untrusted.RemoteAddr = "203.0.113.9:1234"
	untrusted.Header.Set("X-Forwarded-Proto", "https")
	spoofed := httptest.NewRecorder()
	handler.ServeHTTP(spoofed, untrusted)
	if spoofed.Header().Get("Strict-Transport-Security") != "" {
		t.Fatal("HSTS trusted a forwarded header from an untrusted remote")
	}
}

func TestSecurityHeadersRejectsInvalidConfiguration(t *testing.T) {
	splitting := DefaultSecurityHeaders()
	splitting.ContentSecurityPolicy = "default-src 'self'\r\nX-Evil: yes"
	for name, config := range map[string]SecurityHeadersConfig{
		"response splitting": splitting,
		"frame options":      {FrameOptions: "allow"},
		"referrer policy":    {ReferrerPolicy: "unsafe-url"},
		"HSTS max age":       {HSTS: HSTSConfig{Enabled: true}},
		"HSTS preload":       {HSTS: HSTSConfig{Enabled: true, MaxAge: time.Hour, Preload: true}},
	} {
		if _, err := SecurityHeaders(config); err == nil {
			t.Fatalf("%s was accepted", name)
		}
	}
}
