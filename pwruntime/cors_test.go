package pwruntime

import (
	"strings"
	"testing"
	"time"
)

func enabledCORS(origins ...string) CORSConfig {
	config := DefaultCORS()
	config.Enabled = true
	config.AllowedOrigins = origins
	return config
}

func resolve(t *testing.T, config CORSConfig, csrfHeader string) ResolvedCORS {
	t.Helper()
	resolved, err := ResolveCORS(config, csrfHeader)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

func header(decision CORSDecision, name string) string {
	for _, entry := range decision.Headers {
		if entry.Name == name {
			return entry.Value
		}
	}
	return ""
}

// Every refusal below is a configuration a browser would answer by dropping the
// response, or a grant wider than the deployment can have meant. They fail at
// startup because the alternative is a network error in somebody else's
// browser, reported to nobody.
func TestCORSValidationRefusesWhatABrowserWould(t *testing.T) {
	credentialed := func(mutate func(*CORSConfig)) CORSConfig {
		config := enabledCORS("https://app.example.com")
		config.AllowCredentials = true
		config.Include = []string{"/api/**"}
		mutate(&config)
		return config
	}
	for name, config := range map[string]CORSConfig{
		"no origin at all": func() CORSConfig {
			config := DefaultCORS()
			config.Enabled = true
			return config
		}(),
		"wildcard with credentials": credentialed(func(c *CORSConfig) {
			c.AllowedOrigins = []string{"*"}
		}),
		"wildcard header with credentials": credentialed(func(c *CORSConfig) {
			c.AllowedHeaders = []string{"*"}
		}),
		"the whole tree with credentials": credentialed(func(c *CORSConfig) {
			c.Include = []string{"/**"}
		}),
		"wildcard mixed with a named origin": enabledCORS("*", "https://app.example.com"),
		"an origin carrying a path":          enabledCORS("https://app.example.com/api"),
		"an origin with a trailing slash":    enabledCORS("https://app.example.com/"),
		"an origin with a query":             enabledCORS("https://app.example.com?a=1"),
		"an origin with userinfo":            enabledCORS("https://user@app.example.com"),
		"the null origin":                    enabledCORS("null"),
		"a non-http scheme":                  enabledCORS("ftp://app.example.com"),
		"a negative max age": func() CORSConfig {
			config := enabledCORS("https://app.example.com")
			config.MaxAge = -time.Second
			return config
		}(),
		"a method that is not a token": func() CORSConfig {
			config := enabledCORS("https://app.example.com")
			config.AllowedMethods = []string{"GET POST"}
			return config
		}(),
	} {
		if err := config.Validate(); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

// Off validates nothing and resolves to a policy that installs nothing, so a
// deployment that never turned this on cannot fail startup on it.
func TestCORSOffAcceptsAnything(t *testing.T) {
	config := DefaultCORS()
	config.AllowedOrigins = []string{"not an origin"}
	resolved := resolve(t, config, "X-CSRF-Token")
	if resolved.Enabled() {
		t.Fatal("a policy that is off installed something")
	}
}

func TestCORSMarksAnAllowedOriginAndSkipsAnUnlistedOne(t *testing.T) {
	resolved := resolve(t, enabledCORS("https://app.example.com"), "X-CSRF-Token")

	allowed := resolved.Decide("/api/things", "", "GET", "https://app.example.com", "", "")
	if got := header(allowed, "Access-Control-Allow-Origin"); got != "https://app.example.com" {
		t.Fatalf("allow-origin = %q", got)
	}
	if allowed.Declined != "" {
		t.Fatalf("declined = %q", allowed.Declined)
	}

	// The framework frames' own headers, none of them safelisted: without this
	// a cross-origin client cannot read the retry metadata on a 429 it was
	// meant to act on.
	expose := header(allowed, "Access-Control-Expose-Headers")
	for _, want := range []string{"X-Request-ID", "Retry-After", "X-RateLimit-Reset"} {
		if !strings.Contains(expose, want) {
			t.Errorf("expose-headers %q omits %s", expose, want)
		}
	}

	unlisted := resolved.Decide("/api/things", "", "GET", "https://evil.example.com", "", "")
	if len(unlisted.Headers) != 0 {
		t.Fatalf("an unlisted origin was marked: %v", unlisted.Headers)
	}
	if unlisted.Declined != CORSDeclinedOrigin {
		t.Fatalf("declined = %q", unlisted.Declined)
	}
}

// The scope is the shared segment grammar. A path outside it is not this
// policy's business, marked or refused.
func TestCORSScopeDecidesNothingOutsideIt(t *testing.T) {
	config := enabledCORS("https://app.example.com")
	config.Include = []string{"/api/**"}
	resolved := resolve(t, config, "X-CSRF-Token")

	if decision := resolved.Decide("/pages/home", "", "GET", "https://app.example.com", "", ""); len(decision.Headers) != 0 || len(decision.Vary) != 0 {
		t.Fatalf("a path outside the scope was decided about: %+v", decision)
	}
	// A path that cannot be matched unambiguously is left alone for the reason
	// the canonical form exists: its target depends on who resolves it.
	if decision := resolved.Decide("/api/../pages/home", "", "GET", "https://app.example.com", "", ""); len(decision.Headers) != 0 {
		t.Fatalf("an ambiguous path was marked: %+v", decision)
	}
}

// Vary is emitted for a response this policy left unmarked, because the
// decision read Origin even when the answer was to write nothing, and a shared
// cache keyed on nothing would hand this response to a caller that would have
// been marked.
func TestCORSVariesOnOriginIncludingWhenItWroteNothing(t *testing.T) {
	allowlist := resolve(t, enabledCORS("https://app.example.com"), "X-CSRF-Token")
	plain := allowlist.Decide("/api/things", "", "GET", "", "", "")
	if len(plain.Headers) != 0 {
		t.Fatalf("a request with no Origin was marked: %v", plain.Headers)
	}
	if len(plain.Vary) != 1 || plain.Vary[0] != "Origin" {
		t.Fatalf("vary = %v", plain.Vary)
	}

	// The wildcard answers identically for every caller, so it needs no Vary
	// and the response stays shared-cacheable.
	wildcard := resolve(t, enabledCORS("*"), "X-CSRF-Token")
	decision := wildcard.Decide("/api/things", "", "GET", "https://anywhere.example.com", "", "")
	if len(decision.Vary) != 0 {
		t.Fatalf("the wildcard policy varied: %v", decision.Vary)
	}
	if got := header(decision, "Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("allow-origin = %q", got)
	}
}

func TestCORSPreflight(t *testing.T) {
	config := enabledCORS("https://app.example.com")
	config.AllowedMethods = []string{"GET", "POST", "DELETE"}
	resolved := resolve(t, config, "X-CSRF-Token")

	admitted := resolved.Decide("/api/things", "", "OPTIONS", "https://app.example.com", "DELETE", "content-type")
	if !admitted.Preflight || admitted.Declined != "" {
		t.Fatalf("preflight = %+v", admitted)
	}
	if got := header(admitted, "Access-Control-Allow-Methods"); !strings.Contains(got, "DELETE") {
		t.Fatalf("allow-methods = %q", got)
	}
	if got := header(admitted, "Access-Control-Max-Age"); got != "600" {
		t.Fatalf("max-age = %q", got)
	}
	// Vary covers the two request headers the answer depends on, not just the
	// origin.
	if len(admitted.Vary) != 3 {
		t.Fatalf("vary = %v", admitted.Vary)
	}

	// A method outside the set still receives the admitted sets, so a developer
	// reading the response sees what was allowed rather than an empty answer.
	refused := resolved.Decide("/api/things", "", "OPTIONS", "https://app.example.com", "PATCH", "")
	if refused.Declined != CORSDeclinedMethod {
		t.Fatalf("declined = %q", refused.Declined)
	}
	if got := header(refused, "Access-Control-Allow-Methods"); got == "" {
		t.Fatal("a refused method was answered with no admitted methods to read")
	}

	// An unlisted origin gets nothing at all: a caller that may not send the
	// request needs no further detail to decide.
	unlisted := resolved.Decide("/api/things", "", "OPTIONS", "https://evil.example.com", "GET", "")
	if !unlisted.Preflight || len(unlisted.Headers) != 0 {
		t.Fatalf("unlisted preflight = %+v", unlisted)
	}

	// An OPTIONS carrying no request-method header is not a preflight and
	// belongs to whatever else answers OPTIONS.
	plain := resolved.Decide("/api/things", "", "OPTIONS", "https://app.example.com", "", "")
	if plain.Preflight {
		t.Fatal("a plain OPTIONS was answered as a preflight")
	}
}

func TestCORSPreflightRefusesAnUnadmittedHeader(t *testing.T) {
	resolved := resolve(t, enabledCORS("https://app.example.com"), "X-CSRF-Token")
	decision := resolved.Decide("/api/things", "", "OPTIONS", "https://app.example.com", "GET", "content-type, x-secret")
	if decision.Declined != CORSDeclinedHeader {
		t.Fatalf("declined = %q", decision.Declined)
	}
	admitted := resolved.Decide("/api/things", "", "OPTIONS", "https://app.example.com", "GET", " Content-Type , authorization ")
	if admitted.Declined != "" {
		t.Fatalf("a configured header was refused: %q", admitted.Declined)
	}
}

// The CSRF header is admitted on its own while credentials are on. Admitting
// the name grants nothing the token check does not still gate, and a header
// that must be remembered in a second place is the one that is forgotten.
func TestCORSAdmitsTheConfiguredCSRFHeaderWithCredentials(t *testing.T) {
	config := enabledCORS("https://app.example.com")
	config.AllowCredentials = true
	config.Include = []string{"/api/**"}
	resolved := resolve(t, config, "X-CSRF-Token")

	decision := resolved.Decide("/api/things", "", "OPTIONS", "https://app.example.com", "POST", "x-csrf-token")
	if decision.Declined != "" {
		t.Fatalf("the configured CSRF header was refused: %q", decision.Declined)
	}
	if got := header(decision, "Access-Control-Allow-Credentials"); got != "true" {
		t.Fatalf("allow-credentials = %q", got)
	}

	config.AllowCredentials = false
	without := resolve(t, config, "X-CSRF-Token")
	if decision := without.Decide("/api/things", "", "OPTIONS", "https://app.example.com", "POST", "x-csrf-token"); decision.Declined != CORSDeclinedHeader {
		t.Fatalf("the header was admitted without credentials: %q", decision.Declined)
	}
}

// The echoed value is the configured one rather than the caller's, so a header
// this frame writes is never assembled from bytes a caller chose.
func TestCORSEchoesTheConfiguredOriginNotTheRequestsOwn(t *testing.T) {
	resolved := resolve(t, enabledCORS("https://app.example.com"), "X-CSRF-Token")
	decision := resolved.Decide("/api/things", "", "GET", "HTTPS://APP.EXAMPLE.COM", "", "")
	if got := header(decision, "Access-Control-Allow-Origin"); got != "https://app.example.com" {
		t.Fatalf("allow-origin = %q", got)
	}
}

// The origin is caller-controlled, so a scan writes one record per request. The
// bound drops past its rate and counts what it dropped.
func TestCORSDeclineRecordsAreBounded(t *testing.T) {
	budget := &corsLogBudget{}
	const second = int64(1700000000)
	written := 0
	for range corsDeclineRecordsPerSecond + 5 {
		if admit, _ := budget.admit(second); admit {
			written++
		}
	}
	if written != corsDeclineRecordsPerSecond {
		t.Fatalf("wrote %d records in one second", written)
	}
	admit, dropped := budget.admit(second + 1)
	if !admit || dropped != 5 {
		t.Fatalf("admit=%v dropped=%d", admit, dropped)
	}
}
