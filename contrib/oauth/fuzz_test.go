//go:build !tinygo

package oauth

import (
	"testing"
	"time"
)

func FuzzTokenResponse(f *testing.F) {
	f.Add([]byte(`{"access_token":"a","token_type":"Bearer"}`))
	f.Add([]byte(`{"access_token":"a","access_token":"b","token_type":"Bearer"}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = parseTokenSet(data, 64<<10)
	})
}

func FuzzDeviceAuthorizationResponse(f *testing.F) {
	f.Add([]byte(`{"device_code":"secret","user_code":"BCDF-GHJK","verification_uri":"https://issuer.example/device","expires_in":600,"interval":5}`))
	f.Add([]byte(`{"device_code":"a","device_code":"b"}`))
	client := &DeviceClient{clock: func() time.Time { return time.Unix(100, 0) }, maxResponse: 64 << 10}
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = client.parseAuthorization(data)
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
