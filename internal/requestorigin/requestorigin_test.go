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
