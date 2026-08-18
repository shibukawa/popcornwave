package devidp_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/popcornweb/contrib/devidp"
)

func TestParseConfigReadsTheRoster(t *testing.T) {
	config, err := devidp.ParseConfig([]byte(`
[idp]
issuer = "http://127.0.0.1:18080"
valid_scopes = ["admin"]
token_ttl = "30m"
code_ttl = "2m"

[clients.myapp]
secret = "development-secret"
redirect_uris = ["http://127.0.0.1:8080/auth/callback"]

[users.admin]
subject = "admin-subject"
display_name = "Administrator"
extra_scopes = ["admin"]
[users.admin.claims]
email = "admin@example.com"
employee_id = 42
active = true
teams = ["dev", "ops"]

[users.guest]
`), t.TempDir())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if config.Issuer != "http://127.0.0.1:18080" {
		t.Fatalf("issuer = %q", config.Issuer)
	}
	if config.TokenTTL.Minutes() != 30 || config.CodeTTL.Minutes() != 2 {
		t.Fatalf("ttl = %s / %s", config.TokenTTL, config.CodeTTL)
	}
	if len(config.Users) != 2 {
		t.Fatalf("users = %d", len(config.Users))
	}
	admin := config.Users[0]
	if admin.Subject != "admin-subject" || admin.DisplayName != "Administrator" {
		t.Fatalf("admin = %+v", admin)
	}
	if admin.Claims["employee_id"] != int64(42) || admin.Claims["active"] != true {
		t.Fatalf("claims = %+v", admin.Claims)
	}
	teams, ok := admin.Claims["teams"].([]any)
	if !ok || len(teams) != 2 || teams[0] != "dev" {
		t.Fatalf("teams = %+v", admin.Claims["teams"])
	}
	// A user table with no fields still yields a selectable identity whose
	// subject and display name default to its key.
	guest := config.Users[1]
	if guest.Subject != "guest" || guest.DisplayName != "guest" {
		t.Fatalf("guest = %+v", guest)
	}
	if len(config.Clients) != 1 || config.Clients[0].ID != "myapp" {
		t.Fatalf("clients = %+v", config.Clients)
	}
}

func TestParseConfigRejectsBadRosters(t *testing.T) {
	cases := map[string]struct {
		source   string
		contains string
	}{
		"unknown table": {
			source:   "[reverse_proxy]\nenabled = true\n[users.admin]\n",
			contains: "unknown table [reverse_proxy]",
		},
		"unknown root key": {
			source:   "device_flow = true\n[users.admin]\n",
			contains: "unknown key device_flow",
		},
		"unknown idp key": {
			source:   "[idp]\ndevice_flow = true\n[users.admin]\n",
			contains: "unknown key idp.device_flow",
		},
		"unknown user key": {
			source:   "[users.admin]\npassword = \"hunter2\"\n",
			contains: "unknown key users.admin.password",
		},
		"reserved claim": {
			source:   "[users.admin.claims]\niss = \"https://evil.example\"\n",
			contains: "reserved claim",
		},
		"nested claim table": {
			source:   "[users.admin.claims.address]\ncity = \"Tokyo\"\n",
			contains: "nested claim tables",
		},
		"duplicate subject": {
			source:   "[users.admin]\nsubject = \"shared\"\n[users.guest]\nsubject = \"shared\"\n",
			contains: "share subject",
		},
		"empty client secret": {
			source:   "[users.admin]\n[clients.myapp]\nsecret = \"\"\nredirect_uris = [\"http://127.0.0.1:8080/cb\"]\n",
			contains: "clients.myapp.secret is required",
		},
		"missing redirect uris": {
			source:   "[users.admin]\n[clients.myapp]\nsecret = \"s\"\n",
			contains: "clients.myapp.redirect_uris is required",
		},
		"device-only redirect uris": {
			source:   "[users.admin]\n[clients.device]\ngrants = [\"device_code\"]\nredirect_uris = [\"http://127.0.0.1/cb\"]\n",
			contains: "redirect_uris requires authorization_code",
		},
		"unknown client grant": {
			source:   "[users.admin]\n[clients.device]\ngrants = [\"client_credentials\"]\n",
			contains: "is unsupported",
		},
		"relative redirect uri": {
			source:   "[users.admin]\n[clients.myapp]\nsecret = \"s\"\nredirect_uris = [\"/callback\"]\n",
			contains: "must be an absolute URL",
		},
		"no users": {
			source:   "[idp]\nissuer = \"http://127.0.0.1:1\"\n",
			contains: "at least one user is required",
		},
		"token ttl out of range": {
			source:   "[idp]\ntoken_ttl = \"48h\"\n[users.admin]\n",
			contains: "idp.token_ttl",
		},
		"scope with whitespace": {
			source:   "[idp]\nvalid_scopes = [\"read write\"]\n[users.admin]\n",
			contains: "is not a scope token",
		},
		"issuer with a fragment": {
			source:   "[idp]\nissuer = \"http://127.0.0.1:1#frag\"\n[users.admin]\n",
			contains: "must not carry a query or fragment",
		},
	}
	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := devidp.ParseConfig([]byte(testCase.source), t.TempDir())
			if err == nil {
				t.Fatal("expected an error")
			}
			if !strings.Contains(err.Error(), testCase.contains) {
				t.Fatalf("error %q does not contain %q", err, testCase.contains)
			}
		})
	}
}

func TestParseConfigAcceptsPublicDeviceClient(t *testing.T) {
	config, err := devidp.ParseConfig([]byte("[users.admin]\n[clients.device]\ngrants = [\"device_code\"]\nvalid_scopes = [\"profile\"]\n"), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(config.Clients) != 1 || config.Clients[0].Secret != "" || len(config.Clients[0].RedirectURIs) != 0 ||
		len(config.Clients[0].GrantTypes) != 1 || config.Clients[0].GrantTypes[0] != devidp.GrantDeviceCode {
		t.Fatalf("device client = %#v", config.Clients)
	}
}

func TestLoadConfigResolvesTheSigningKeyRelativeToTheFile(t *testing.T) {
	directory := t.TempDir()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	encoded := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	if err := os.WriteFile(filepath.Join(directory, "signing.pem"), encoded, 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}
	path := filepath.Join(directory, "devidp.toml")
	if err := os.WriteFile(path, []byte("[idp]\nsigning_key = \"signing.pem\"\n[users.admin]\n"), 0o600); err != nil {
		t.Fatalf("write roster: %v", err)
	}
	config, err := devidp.LoadConfig(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if config.SigningKey == nil || config.SigningKey.N.Cmp(key.N) != 0 {
		t.Fatal("expected the configured signing key")
	}
}

func TestLoadConfigRejectsANonRSASigningKey(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "signing.pem"), []byte("not a key"), 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}
	path := filepath.Join(directory, "devidp.toml")
	if err := os.WriteFile(path, []byte("[idp]\nsigning_key = \"signing.pem\"\n[users.admin]\n"), 0o600); err != nil {
		t.Fatalf("write roster: %v", err)
	}
	if _, err := devidp.LoadConfig(path); err == nil {
		t.Fatal("expected an error")
	}
}
