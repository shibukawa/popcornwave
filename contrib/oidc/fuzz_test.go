//go:build !tinygo

package oidc

import (
	"testing"

	"github.com/shibukawa/popcornweb/contrib/jwt"
)

func FuzzIDTokenParsing(f *testing.F) {
	f.Add("eyJhbGciOiJSUzI1NiIsImtpZCI6ImsxIn0.eyJpc3MiOiJodHRwczovL2lzc3Vlci5leGFtcGxlIn0.AA")
	f.Add("..")
	f.Fuzz(func(t *testing.T, raw string) {
		// Parsing is bounded before any provider or key lookup is attempted.
		_, _ = parseBoundedToken(raw)
	})
}

func FuzzBearerTokenValidation(f *testing.F) {
	f.Add("access-token")
	f.Add("access\r\nX-Injected: yes")
	f.Add("日本語")
	f.Fuzz(func(t *testing.T, value string) {
		valid := validBearerToken(value)
		if valid && value == "" {
			t.Fatal("empty bearer token accepted")
		}
	})
}

func parseBoundedToken(raw string) (*jwt.Token, error) {
	return jwt.Parse(raw, jwt.ParseOptions{MaxTokenBytes: 16 << 10, MaxSegmentBytes: 8 << 10})
}
