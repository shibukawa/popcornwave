package pwcli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeProject(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "handlers"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	for name, content := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	return root
}

const idpProject = `
[project]
name = "app"
main = "./cmd/app"

[generate]
handlers = ["handlers"]
templates = ["handlers"]
queries = []
config = []

[dev.idp]
enabled = true
`

const idpRoster = `
[users.admin]
display_name = "Administrator"
[users.admin.claims]
email = "admin@example.com"
`

func TestLoadProjectConfigReadsDevIdP(t *testing.T) {
	root := writeProject(t, map[string]string{"popcornwave.toml": idpProject})
	config, err := loadProjectConfig(root)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !config.IdP.Enabled {
		t.Fatal("expected dev.idp.enabled")
	}
	if config.IdP.Config != defaultIdPConfig {
		t.Fatalf("config = %q", config.IdP.Config)
	}
	if config.IdP.Port != 0 {
		t.Fatalf("port = %d, want an automatically reserved port", config.IdP.Port)
	}
}

func TestLoadProjectConfigRejectsABadIdPPort(t *testing.T) {
	root := writeProject(t, map[string]string{
		"popcornwave.toml": idpProject + "port = 70000\n",
	})
	if _, err := loadProjectConfig(root); err == nil || !strings.Contains(err.Error(), "dev.idp.port") {
		t.Fatalf("err = %v", err)
	}
}

func TestStartDevIdentityProviderInjectsIssuerAndCredentials(t *testing.T) {
	root := writeProject(t, map[string]string{
		"popcornwave.toml": idpProject,
		"devidp.toml":      idpRoster,
	})
	config, err := loadProjectConfig(root)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	output := &bytes.Buffer{}
	idp, err := startDevIdentityProvider(t.Context(), root, config, output)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	defer idp.close()

	environ := idp.environ(nil)
	if len(environ) != 3 {
		t.Fatalf("environ = %v", environ)
	}
	var issuer string
	for _, entry := range environ {
		if value, ok := strings.CutPrefix(entry, envIdPIssuer+"="); ok {
			issuer = value
		}
	}
	if !strings.HasPrefix(issuer, "http://127.0.0.1:") {
		t.Fatalf("%s = %q", envIdPIssuer, issuer)
	}
	if issuer != idp.server.Issuer() {
		t.Fatalf("injected issuer %q does not match the provider %q", issuer, idp.server.Issuer())
	}
	// The secret reaches the application through the environment only.
	report := output.String()
	if !strings.Contains(report, idp.clientID()) || !strings.Contains(report, "admin") {
		t.Fatalf("report = %q", report)
	}
	for _, entry := range environ {
		secret, ok := strings.CutPrefix(entry, envIdPClientSecret+"=")
		if ok && strings.Contains(report, secret) {
			t.Fatal("the client secret must not be printed")
		}
	}
}

func TestDevIdentityProviderKeepsAnExplicitEnvironmentValue(t *testing.T) {
	root := writeProject(t, map[string]string{
		"popcornwave.toml": idpProject,
		"devidp.toml":      idpRoster,
	})
	config, err := loadProjectConfig(root)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	t.Setenv(envIdPIssuer, "https://issuer.example")
	idp, err := startDevIdentityProvider(t.Context(), root, config, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	defer idp.close()

	for _, entry := range idp.environ(nil) {
		if strings.HasPrefix(entry, envIdPIssuer+"=") {
			t.Fatalf("expected the exported %s to win, got %q", envIdPIssuer, entry)
		}
	}
}

func TestStartDevIdentityProviderReportsAMissingRoster(t *testing.T) {
	root := writeProject(t, map[string]string{"popcornwave.toml": idpProject})
	config, err := loadProjectConfig(root)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	_, err = startDevIdentityProvider(t.Context(), root, config, &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected a missing roster to fail")
	}
	if !strings.Contains(err.Error(), "[users.admin]") {
		t.Fatalf("expected a roster sample in %q", err)
	}
}

func TestDevIdentityProviderReloadsAnEditedRoster(t *testing.T) {
	root := writeProject(t, map[string]string{
		"popcornwave.toml": idpProject,
		"devidp.toml":      idpRoster,
	})
	config, err := loadProjectConfig(root)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	idp, err := startDevIdentityProvider(t.Context(), root, config, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	defer idp.close()
	issuer := idp.server.Issuer()
	clientID := idp.clientID()

	if err := os.WriteFile(filepath.Join(root, "devidp.toml"), []byte(idpRoster+"\n[users.guest]\n"), 0o600); err != nil {
		t.Fatalf("edit roster: %v", err)
	}
	idp.reload(&bytes.Buffer{}, &bytes.Buffer{})

	if len(idp.server.Users()) != 2 {
		t.Fatalf("users = %d", len(idp.server.Users()))
	}
	// The issuer and the injected client survive a reload, so the running
	// application keeps working without a restart.
	if idp.server.Issuer() != issuer || idp.clientID() != clientID {
		t.Fatal("a reload must not change the issuer or the registered client")
	}
}

func TestRejectDevelopmentImportsIgnoresAnUnlistableGraph(t *testing.T) {
	if err := rejectDevelopmentImports(context.Background(), t.TempDir(), "./cmd/missing", buildOptions{}); err != nil {
		t.Fatalf("expected the build to report its own diagnostics, got %v", err)
	}
}
