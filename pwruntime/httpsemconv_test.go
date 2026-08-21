package pwruntime

import (
	"strings"
	"testing"
)

func TestNormalizeHTTPMethodBoundsCardinality(t *testing.T) {
	for _, method := range []string{"GET", "HEAD", "POST", "PUT", "PATCH", "DELETE", "CONNECT", "OPTIONS", "TRACE"} {
		if got := NormalizeHTTPMethod(method); got != method {
			t.Errorf("NormalizeHTTPMethod(%q) = %q, want it unchanged", method, got)
		}
	}
	// Anything outside the known verbs collapses to one label, so a client
	// streaming distinct method tokens cannot mint unbounded metric series.
	for _, method := range []string{"BREW", "get", "FOOBAR", "", "GET ", "\x00"} {
		if got := NormalizeHTTPMethod(method); got != "_OTHER" {
			t.Errorf("NormalizeHTTPMethod(%q) = %q, want _OTHER", method, got)
		}
	}
}

func TestNormalizeSchemeIsTwoValued(t *testing.T) {
	if got := NormalizeScheme("https"); got != "https" {
		t.Errorf("https = %q", got)
	}
	for _, scheme := range []string{"http", "ftp", "javascript", "", "HTTPS"} {
		if got := NormalizeScheme(scheme); got != "http" {
			t.Errorf("NormalizeScheme(%q) = %q, want http", scheme, got)
		}
	}
}

func TestDigestKeyMaterialMeasuresDecodedOrRaw(t *testing.T) {
	// A base64 value is measured decoded: 32 random bytes encode to 44 chars.
	if got := DigestKeyMaterial("nD20aGUfD4QnZMPpu+U89HGvNyRiEo/ts6IDUKca2+E="); got != 32 {
		t.Errorf("32-byte base64 measured as %d, want 32", got)
	}
	// A value that is not valid base64 is measured as its raw bytes.
	const raw = "a-very-long-live-digest-validator-key!"
	if got := DigestKeyMaterial(raw); got != len(raw) {
		t.Errorf("raw key measured as %d, want %d", got, len(raw))
	}
	if got := DigestKeyMaterial(""); got != 0 {
		t.Errorf("empty measured as %d, want 0", got)
	}
	// A short base64 value decodes below the floor even though its string is
	// longer than 32 characters.
	if got := DigestKeyMaterial(strings.Repeat("A", 24)); got >= 32 {
		t.Errorf("24-char base64 measured as %d, want < 32", got)
	}
}
