//go:build !tinygo

package oauth

import "testing"

func FuzzTokenResponse(f *testing.F) {
	f.Add([]byte(`{"access_token":"a","token_type":"Bearer"}`))
	f.Add([]byte(`{"access_token":"a","access_token":"b","token_type":"Bearer"}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = parseTokenSet(data, 64<<10)
	})
}

func FuzzScopeGrammar(f *testing.F) {
	f.Add("openid")
	f.Add("openid profile")
	f.Add("open\x00id")
	f.Fuzz(func(t *testing.T, scope string) {
		valid := validScope(scope)
		if valid && len(scope) == 0 {
			t.Fatal("empty scope accepted")
		}
	})
}
