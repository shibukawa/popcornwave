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
