//go:build !tinygo

package authn

import "testing"

func FuzzValidateJSON(f *testing.F) {
	f.Add([]byte(`{"state":"value","nested":[1,true,null]}`))
	f.Add([]byte(`{"key":1,"key":2}`))
	f.Add([]byte(`[[[[[[[]]]]]]]`))
	f.Add([]byte(`{} trailing`))
	f.Fuzz(func(t *testing.T, data []byte) {
		_ = ValidateJSON(data, JSONOptions{MaxBytes: 64 << 10, MaxDepth: 16, MaxMembers: 256})
	})
}

func FuzzDecodeBase64URL(f *testing.F) {
	f.Add("AQIDBA")
	f.Add("YQ==")
	f.Add("\x00")
	f.Fuzz(func(t *testing.T, value string) {
		_, _ = DecodeBase64URL(value, 4096, 2048)
	})
}

func FuzzParseEndpoint(f *testing.F) {
	f.Add("https://issuer.example/.well-known")
	f.Add("http://127.0.0.1:8080/callback")
	f.Add("https://:443/token")
	f.Fuzz(func(t *testing.T, value string) {
		_, _ = ParseEndpoint(value, true)
	})
}
