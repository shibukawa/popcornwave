//go:build !tinygo

package passkey

import (
	"testing"
)

func FuzzDecodeAuthenticationCredential(f *testing.F) {
	f.Add([]byte(`{"id":"a","rawId":"a","type":"public-key","response":{}}`))
	f.Add([]byte(`{}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		rp, err := New(Config{RPID: "example.com", RPName: "Example", Origins: []string{"https://example.com"}})
		if err != nil {
			t.Fatal(err)
		}
		_, _ = rp.DecodeAuthenticationCredential(data)
	})
}

func FuzzRPIDConfiguration(f *testing.F) {
	f.Add("example.com")
	f.Add("example..com")
	f.Add("https://example.com")
	f.Fuzz(func(t *testing.T, rpID string) {
		_, _ = New(Config{RPID: rpID, RPName: "Example", Origins: []string{"https://example.com"}})
	})
}
