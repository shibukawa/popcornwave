package auth

import (
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func scopeRuntime(t *testing.T, scope string, allowRequest bool) *runtime {
	t.Helper()
	config := baseConfig(ModeOIDCOnly)
	config.OIDC.LogoutScope = scope
	config.OIDC.AllowGlobalLogoutRequest = allowRequest
	return &runtime{config: config}
}

// A request may escalate toward a global sign-out and may never downgrade:
// escalation costs the user extra sign-outs, while a forced downgrade would
// leave the provider session alive after the user asked to leave it.
func TestLogoutScopeEscalatesButNeverDowngrades(t *testing.T) {
	form := url.Values{logoutScopeField: []string{LogoutScopeGlobal}}
	reconfirmForm := url.Values{logoutScopeField: []string{LogoutScopeReconfirm}}

	cases := []struct {
		name  string
		scope string
		allow bool
		form  url.Values
		want  string
	}{
		{"reconfirm stays reconfirm", LogoutScopeReconfirm, false, nil, LogoutScopeReconfirm},
		{"request ignored when not permitted", LogoutScopeReconfirm, false, form, LogoutScopeReconfirm},
		{"request escalates when permitted", LogoutScopeReconfirm, true, form, LogoutScopeGlobal},
		{"global stays global", LogoutScopeGlobal, false, nil, LogoutScopeGlobal},
		{"a request cannot downgrade global", LogoutScopeGlobal, true, reconfirmForm, LogoutScopeGlobal},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			rt := scopeRuntime(t, testCase.scope, testCase.allow)
			body := ""
			if testCase.form != nil {
				body = testCase.form.Encode()
			}
			request := httptest.NewRequest("POST", "/auth/logout", strings.NewReader(body))
			request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			if got := rt.logoutScope(HTTPExchange(httptest.NewRecorder(), request)); got != testCase.want {
				t.Fatalf("logoutScope = %q, want %q", got, testCase.want)
			}
		})
	}
}

// configbind ignores a key no field declares, so deleting provider_logout
// outright would have left every scaffolded project silently running reconfirm
// while its configuration still read as a global sign-out.
func TestTheRemovedProviderLogoutKeyIsRefused(t *testing.T) {
	config := baseConfig(ModeOIDCOnly)
	config.OIDC.ProviderLogout = true
	err := config.validateShape()
	if err == nil || !strings.Contains(err.Error(), "logout_scope") {
		t.Fatalf("stale provider_logout = %v, want an error naming logout_scope", err)
	}
}

// Any one of the shared-device settings alone achieves nothing, so a
// configuration that asks for the mode while contradicting it is refused rather
// than silently overridden: a file that reads as one behavior while the
// deployment runs another is the failure the mode exists to avoid.
func TestSharedDeviceRefusesAContradictingLogoutScope(t *testing.T) {
	config := baseConfig(ModeOIDCOnly)
	config.SharedDevice = true
	config.OIDC.LogoutScope = LogoutScopeReconfirm
	err := config.validateShape()
	if err == nil || !strings.Contains(err.Error(), "shared_device") {
		t.Fatalf("contradicting shared_device = %v", err)
	}

	config.OIDC.LogoutScope = LogoutScopeGlobal
	if err := config.validateShape(); err != nil {
		t.Fatalf("shared_device with global logout = %v", err)
	}
}
