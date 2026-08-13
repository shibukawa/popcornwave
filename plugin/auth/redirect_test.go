package auth

import "testing"

func TestResolveOIDCRedirectURIFromLoopbackRequest(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		host string
		want string
	}{
		{"localhost default", "", "localhost:8080", "http://localhost:8080/auth/callback"},
		{"ipv4 path", "/auth/callback", "127.0.0.1:8080", "http://127.0.0.1:8080/auth/callback"},
		{"ipv6 default", "", "[::1]:8080", "http://[::1]:8080/auth/callback"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := resolveOIDCRedirectURI(test.raw, "/auth/callback", true, "http", test.host)
			if err != nil || got != test.want {
				t.Fatalf("resolve = %q, %v; want %q", got, err, test.want)
			}
		})
	}
}

func TestResolveOIDCRedirectURIRefusesNonLoopbackHost(t *testing.T) {
	if _, err := resolveOIDCRedirectURI("", "/auth/callback", true, "https", "app.example"); err == nil {
		t.Fatal("non-loopback Host was accepted")
	}
}

func TestResolveOIDCRedirectURIKeepsAbsoluteConfiguration(t *testing.T) {
	const configured = "https://app.example/auth/callback"
	got, err := resolveOIDCRedirectURI(configured, "/auth/callback", false, "http", "localhost:8080")
	if err != nil || got != configured {
		t.Fatalf("resolve = %q, %v; want configured URL", got, err)
	}
}
