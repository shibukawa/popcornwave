package requestorigin

import (
	"crypto/tls"
	"net/http"
	"testing"
)

// request builds a request whose own origin is https://app.example when secure
// is true and http://app.example otherwise.
func request(secure bool, header, value string) *http.Request {
	r, err := http.NewRequest(http.MethodPost, "/action", nil)
	if err != nil {
		panic(err)
	}
	r.Host = "app.example"
	if secure {
		r.TLS = &tls.ConnectionState{}
	}
	if header != "" {
		r.Header.Set(header, value)
	}
	return r
}

func TestMatchesComparesTheWholeOrigin(t *testing.T) {
	// The reason this package exists: an http caller must not pass the check
	// made for an https deployment, which a host-only comparison allowed.
	if Matches(request(true, "Origin", "http://app.example"), nil) {
		t.Fatal("an http origin was admitted to an https deployment")
	}
	if !Matches(request(true, "Origin", "https://app.example"), nil) {
		t.Fatal("the deployment's own origin was refused")
	}
	if Matches(request(true, "Origin", "https://attacker.example"), nil) {
		t.Fatal("a foreign origin was admitted")
	}
}

func TestMatchesRefusesAnOpaqueOrigin(t *testing.T) {
	// A literal null is what a sandboxed frame sends. Treating it as absent
	// would fall through to the Referer branch, which is the weaker check.
	if Matches(request(true, "Origin", "null"), nil) {
		t.Fatal("a null origin was admitted")
	}
}

func TestMatchesRequiresAReferer(t *testing.T) {
	if Matches(request(true, "", ""), nil) {
		t.Fatal("a request carrying neither header was admitted")
	}
	if Matches(request(true, "Referer", "/relative/path"), nil) {
		t.Fatal("a Referer with no scheme or host was admitted")
	}
	if !Matches(request(true, "Referer", "https://app.example/page"), nil) {
		t.Fatal("a same-origin Referer was refused")
	}
	if Matches(request(true, "Referer", "http://app.example/page"), nil) {
		t.Fatal("an http Referer was admitted to an https deployment")
	}
}

func TestMatchesAcceptsADeclaredOrigin(t *testing.T) {
	// The TLS-terminating proxy case: the request arrives without r.TLS, so its
	// own origin reconstructs as http while the browser reports https. The
	// origin the deployment declared is what makes this work, and nothing is
	// read from a forwarded header.
	trusted := Set("https://app.example")
	if !Matches(request(false, "Origin", "https://app.example"), trusted) {
		t.Fatal("a declared origin was refused behind a terminating proxy")
	}
	if Matches(request(false, "Origin", "https://other.example"), trusted) {
		t.Fatal("an undeclared origin was admitted")
	}
	forwarded := request(false, "Origin", "https://app.example")
	forwarded.Header.Set("X-Forwarded-Proto", "https")
	if Matches(forwarded, nil) {
		t.Fatal("a forwarded header was trusted with nothing declared")
	}
}

func TestSetNormalizesAndDropsUnusable(t *testing.T) {
	trusted := Set("https://app.example/login?x=1", " https://spaced.example ", "app.example", "", "https://")
	if !trusted["https://app.example"] {
		t.Fatalf("a path and query were not trimmed to an origin: %v", trusted)
	}
	if !trusted["https://spaced.example"] {
		t.Fatalf("surrounding space was not trimmed: %v", trusted)
	}
	if len(trusted) != 2 {
		t.Fatalf("a value naming no scheme or no host was kept: %v", trusted)
	}
}

func TestOfReportsNothingWithoutAHost(t *testing.T) {
	r := request(true, "", "")
	r.Host = ""
	if got := Of(r); got != "" {
		t.Fatalf("Of with no Host = %q", got)
	}
	// An empty self must not match an empty declared value either.
	if Matches(r, map[string]bool{"": true}) {
		t.Fatal("a request with no Host matched")
	}
}

// proxied builds a request arriving from peer with no TLS, as one does behind
// a terminating proxy.
func proxied(peer string, headers map[string]string) *http.Request {
	r, err := http.NewRequest(http.MethodPost, "/action", nil)
	if err != nil {
		panic(err)
	}
	r.Host = "app.example"
	r.RemoteAddr = peer
	for name, value := range headers {
		r.Header.Set(name, value)
	}
	return r
}

func mustCompile(t *testing.T, values ...string) Proxies {
	t.Helper()
	proxies, err := Compile(values)
	if err != nil {
		t.Fatalf("Compile(%v) = %v", values, err)
	}
	return proxies
}

func TestSchemeReadsForwardedProtoOnlyBehindADeclaredProxy(t *testing.T) {
	headers := map[string]string{"X-Forwarded-Proto": "https"}
	// The whole point of the gate: the same header, from inside and outside.
	if got := mustCompile(t, "10.0.0.0/8").Scheme(proxied("10.0.0.7:5555", headers)); got != "https" {
		t.Fatalf("scheme behind a declared proxy = %q", got)
	}
	if got := mustCompile(t, "10.0.0.0/8").Scheme(proxied("203.0.113.9:5555", headers)); got != "http" {
		t.Fatalf("scheme from an undeclared peer = %q, want the header ignored", got)
	}
	if got := (Proxies{}).Scheme(proxied("10.0.0.7:5555", headers)); got != "http" {
		t.Fatalf("scheme with nothing declared = %q, want the header ignored", got)
	}
}

func TestSchemePrefersTheConnectionOverTheHeader(t *testing.T) {
	r := proxied("10.0.0.7:5555", map[string]string{"X-Forwarded-Proto": "http"})
	r.TLS = &tls.ConnectionState{}
	if got := mustCompile(t, "10.0.0.0/8").Scheme(r); got != "https" {
		t.Fatalf("a direct TLS request reported %q", got)
	}
}

func TestOfBehindAProxyReconstructsTheBrowsersOrigin(t *testing.T) {
	// This is what the CSRF comparison was failing before: the browser reports
	// https and the deployment reconstructed http.
	r := proxied("10.0.0.7:5555", map[string]string{"X-Forwarded-Proto": "https"})
	if got := mustCompile(t, "10.0.0.0/8").Of(r); got != "https://app.example" {
		t.Fatalf("Of behind a declared proxy = %q", got)
	}
	if got := Of(r); got != "http://app.example" {
		t.Fatalf("Of with nothing declared = %q, want the ungated header ignored", got)
	}
}

func TestClientAddressTakesTheFirstUntrustedHop(t *testing.T) {
	proxies := mustCompile(t, "10.0.0.0/8")
	r := proxied("10.0.0.7:5555", map[string]string{"X-Forwarded-For": "203.0.113.9, 10.0.0.3, 10.0.0.7"})
	if got := proxies.ClientAddress(r); got != "203.0.113.9" {
		t.Fatalf("ClientAddress = %q, want the client rather than a relay", got)
	}
}

func TestClientAddressIgnoresAnUngatedHeader(t *testing.T) {
	// A caller asserting a chain from outside the trust set decides nothing.
	r := proxied("203.0.113.9:5555", map[string]string{"X-Forwarded-For": "198.51.100.1"})
	if got := mustCompile(t, "10.0.0.0/8").ClientAddress(r); got != "203.0.113.9" {
		t.Fatalf("ClientAddress from an undeclared peer = %q, want the peer", got)
	}
	if got := (Proxies{}).ClientAddress(r); got != "203.0.113.9" {
		t.Fatalf("ClientAddress with nothing declared = %q, want the peer", got)
	}
}

func TestClientAddressRefusesAMalformedChain(t *testing.T) {
	proxies := mustCompile(t, "10.0.0.0/8")
	r := proxied("10.0.0.7:5555", map[string]string{"X-Forwarded-For": "not-an-address, 10.0.0.7"})
	if got := proxies.ClientAddress(r); got != "10.0.0.7" {
		t.Fatalf("ClientAddress on a malformed chain = %q, want the peer", got)
	}
}

func TestClientAddressFallsBackWhenEveryHopIsTrusted(t *testing.T) {
	proxies := mustCompile(t, "10.0.0.0/8")
	r := proxied("10.0.0.7:5555", map[string]string{"X-Forwarded-For": "10.0.0.3, 10.0.0.7"})
	if got := proxies.ClientAddress(r); got != "10.0.0.3" {
		t.Fatalf("ClientAddress with an all-trusted chain = %q, want the leftmost hop", got)
	}
}

func TestClientAddressReadsRepeatedHeaderLines(t *testing.T) {
	proxies := mustCompile(t, "10.0.0.0/8")
	r := proxied("10.0.0.7:5555", nil)
	r.Header.Add("X-Forwarded-For", "203.0.113.9")
	r.Header.Add("X-Forwarded-For", "10.0.0.3")
	if got := proxies.ClientAddress(r); got != "203.0.113.9" {
		t.Fatalf("ClientAddress across repeated header lines = %q", got)
	}
}

func TestCompileAcceptsBareAddressesAndRejectsJunk(t *testing.T) {
	proxies := mustCompile(t, "10.0.0.7", "2001:db8::/32")
	if !proxies.Trusts("10.0.0.7:9") {
		t.Fatal("a bare address was not trusted as a single host")
	}
	if proxies.Trusts("10.0.0.8:9") {
		t.Fatal("a bare address widened beyond its own host")
	}
	if !proxies.Trusts("[2001:db8::1]:9") {
		t.Fatal("an IPv6 peer inside a declared block was not trusted")
	}
	if _, err := Compile([]string{"nonsense"}); err == nil {
		t.Fatal("an unparseable value compiled")
	}
	if _, err := Compile([]string{" "}); err == nil {
		t.Fatal("an empty value compiled")
	}
}
