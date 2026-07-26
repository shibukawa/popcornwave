package pw

import (
	"strings"
	"testing"
)

// authPaths fills the endpoint defaults configbind supplies, so each case only
// states the fields it is about.
func authPaths(config AuthConfig) AuthConfig {
	if config.LoginPath == "" {
		config.LoginPath = DefaultLoginPath
	}
	if config.CallbackPath == "" {
		config.CallbackPath = DefaultCallbackPath
	}
	if config.LogoutPath == "" {
		config.LogoutPath = DefaultLogoutPath
	}
	if config.PostLoginRedirect == "" {
		config.PostLoginRedirect = "/"
	}
	if config.PostLogoutRedirect == "" {
		config.PostLogoutRedirect = "/"
	}
	return config
}

func TestValidateAuthConfigAcceptsCompleteConfigurations(t *testing.T) {
	for name, config := range map[string]AuthConfig{
		"disabled": {},
		"disabled with empty provider values": {
			Mode: AuthModeOIDC,
		},
		"oidc": {
			Enabled: true, Mode: AuthModeOIDC,
			OIDC: OIDCConfig{Issuer: "https://issuer.example", ClientID: "client", ClientSecret: "secret"},
		},
		"oidc over loopback http": {
			Enabled: true, Mode: AuthModeOIDCPasskey,
			OIDC: OIDCConfig{
				Issuer: "http://127.0.0.1:18080", ClientID: "client", ClientSecret: "secret",
				RedirectURL: "http://127.0.0.1:8080/auth/callback",
			},
		},
		"passkey only needs no provider": {Enabled: true, Mode: AuthModePasskey},
	} {
		t.Run(name, func(t *testing.T) {
			if err := validateAuthConfig(authPaths(config)); err != nil {
				t.Fatalf("err = %v", err)
			}
		})
	}
}

func TestValidateAuthConfigReportsMissingProviderValues(t *testing.T) {
	err := validateAuthConfig(authPaths(AuthConfig{Enabled: true, Mode: AuthModeOIDC}))
	if err == nil {
		t.Fatal("expected an error")
	}
	for _, expected := range []string{
		"auth.oidc.issuer (AUTH_OIDC_ISSUER)",
		"auth.oidc.client_id (AUTH_OIDC_CLIENT_ID)",
		"auth.oidc.client_secret (AUTH_OIDC_CLIENT_SECRET)",
		"pw dev",
	} {
		if !strings.Contains(err.Error(), expected) {
			t.Fatalf("error %q does not mention %q", err, expected)
		}
	}
}

func TestValidateAuthConfigRejectsPartialAndMalformedValues(t *testing.T) {
	for name, testCase := range map[string]struct {
		config AuthConfig
		want   string
	}{
		"blank secret": {
			config: AuthConfig{Enabled: true, Mode: AuthModeOIDC, OIDC: OIDCConfig{
				Issuer: "https://issuer.example", ClientID: "client", ClientSecret: "   ",
			}},
			want: "auth.oidc.client_secret",
		},
		"unknown mode": {
			config: AuthConfig{Enabled: true, Mode: "basic"},
			want:   "auth.mode must be",
		},
		"relative issuer": {
			config: AuthConfig{Enabled: true, Mode: AuthModeOIDC, OIDC: OIDCConfig{
				Issuer: "issuer.example", ClientID: "client", ClientSecret: "secret",
			}},
			want: "auth.oidc.issuer must be an absolute http or https URL",
		},
		"shared endpoint paths": {
			config: AuthConfig{Enabled: true, Mode: AuthModePasskey, LoginPath: "/auth", LogoutPath: "/auth"},
			want:   "must not share the path",
		},
		"absolute post-login redirect": {
			config: AuthConfig{Enabled: true, Mode: AuthModePasskey, PostLoginRedirect: "https://evil.example/"},
			want:   "auth.post_login_redirect must be an absolute path on this origin",
		},
		"protocol-relative post-logout redirect": {
			config: AuthConfig{Enabled: true, Mode: AuthModePasskey, PostLogoutRedirect: "//evil.example/"},
			want:   "auth.post_logout_redirect must be an absolute path on this origin",
		},
		"relative redirect": {
			config: AuthConfig{Enabled: true, Mode: AuthModeOIDC, OIDC: OIDCConfig{
				Issuer: "https://issuer.example", ClientID: "client", ClientSecret: "secret",
				RedirectURL: "/auth/callback",
			}},
			want: "auth.oidc.redirect_url must be an absolute URL",
		},
	} {
		t.Run(name, func(t *testing.T) {
			err := validateAuthConfig(authPaths(testCase.config))
			if err == nil || !strings.Contains(err.Error(), testCase.want) {
				t.Fatalf("err = %v, want %q", err, testCase.want)
			}
		})
	}
}
