package pwcli

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/shibukawa/popcornwave/internal/pwenv"
)

func TestParseInitArgsRejectsAnUnknownAuthMode(t *testing.T) {
	if _, err := parseInitArgs([]string{"demo", "--auth=basic"}); err == nil ||
		!strings.Contains(err.Error(), "--auth must be") {
		t.Fatalf("err = %v", err)
	}
}

func TestScaffoldWiresTheLocalEmulator(t *testing.T) {
	files := scaffoldFiles(initOptions{Name: "demo", TinyGo: true, Auth: authOIDC, AuthEmulator: true})

	project := files["popcornwave.toml"]
	if !strings.Contains(project, "[dev.idp]") || !strings.Contains(project, "enabled = true") {
		t.Fatalf("popcornwave.toml = %q", project)
	}
	roster, ok := files[defaultIdPConfig]
	if !ok {
		t.Fatalf("%s was not scaffolded", defaultIdPConfig)
	}
	if !strings.Contains(roster, "[users.admin]") {
		t.Fatalf("roster = %q", roster)
	}
	config := files[pwenv.FileName(pwenv.Development)]
	if !strings.Contains(config, `mode = "oidc"`) {
		t.Fatalf("config = %q", config)
	}
	// The emulator supplies the provider values through the environment, so the
	// committed file must not carry an issuer or a client credential at all.
	for _, key := range []string{"issuer =", "client_id =", "client_secret ="} {
		if strings.Contains(config, key) {
			t.Fatalf("config declares %s even though pw dev injects it:\n%s", key, config)
		}
	}
	if !strings.Contains(config, "AUTH_OIDC_ISSUER") {
		t.Fatalf("config does not explain the injected environment:\n%s", config)
	}
}

func TestScaffoldWiresAnExternalProvider(t *testing.T) {
	files := scaffoldFiles(initOptions{Name: "demo", TinyGo: true, Auth: authOIDCPasskey})

	if strings.Contains(files["popcornwave.toml"], "[dev.idp]") {
		t.Fatal("an external provider must not enable the development identity provider")
	}
	if _, ok := files[defaultIdPConfig]; ok {
		t.Fatalf("%s must not be scaffolded for an external provider", defaultIdPConfig)
	}
	config := files[pwenv.FileName(pwenv.Development)]
	for _, expected := range []string{`mode = "oidc_passkey"`, `issuer = ""`, `client_id = ""`, `client_secret = ""`} {
		if !strings.Contains(config, expected) {
			t.Fatalf("config is missing %s:\n%s", expected, config)
		}
	}
}

func TestScaffoldWiresPasskeyOnlyAndNone(t *testing.T) {
	passkey := scaffoldFiles(initOptions{Name: "demo", TinyGo: true, Auth: authPasskey})
	config := passkey[pwenv.FileName(pwenv.Development)]
	if !strings.Contains(config, `mode = "passkey_only"`) || strings.Contains(config, "[auth.oidc]") {
		t.Fatalf("passkey config = %q", config)
	}
	if _, ok := passkey[defaultIdPConfig]; ok {
		t.Fatal("passkey-only must not scaffold an identity provider roster")
	}

	none := scaffoldFiles(initOptions{Name: "demo", TinyGo: true, Auth: authNone})
	if strings.Contains(none[pwenv.FileName(pwenv.Development)], "[auth]") {
		t.Fatal("auth none must not write an [auth] section")
	}
}

// The scaffolded keys must be exactly the ones pw.AuthConfig registers,
// otherwise the generated project fails to start on an unknown key.
func TestScaffoldedAuthKeysAreRegistered(t *testing.T) {
	known := map[string]bool{
		// [auth]
		"enabled": true, "mode": true,
		"login_path": true, "callback_path": true, "logout_path": true,
		"post_login_redirect": true, "post_logout_redirect": true,
		// [auth.oidc]
		"issuer": true, "client_id": true, "client_secret": true,
		"redirect_url": true, "scopes": true, "provider_logout": true,
		// [session]
		"ttl": true, "secret": true,
	}
	config := scaffoldFiles(initOptions{Name: "demo", Auth: authOIDC})[pwenv.FileName(pwenv.Development)]
	section := config[strings.Index(config, "[auth]"):]
	for _, line := range strings.Split(section, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "[") {
			continue
		}
		key, _, found := strings.Cut(line, "=")
		if !found {
			t.Fatalf("unparsable line %q", line)
		}
		if !known[strings.TrimSpace(key)] {
			t.Fatalf("scaffolded key %q is not registered by pw.AuthConfig", strings.TrimSpace(key))
		}
	}
}

func TestScaffoldWiresTheFrameworkOwnedEndpoints(t *testing.T) {
	files := scaffoldFiles(initOptions{Name: "demo", TinyGo: true, Auth: authOIDC, AuthEmulator: true})

	main := files["cmd/demo/main.go"]
	if !strings.Contains(main, `_ "github.com/shibukawa/popcornwave/auth"`) {
		t.Fatalf("main.go does not register the authentication endpoints:\n%s", main)
	}
	config := files[pwenv.FileName(pwenv.Development)]
	for _, expected := range []string{
		`login_path = "/auth/login"`, `callback_path = "/auth/callback"`, `logout_path = "/auth/logout"`,
		`post_login_redirect = "/"`, `post_logout_redirect = "/"`, "provider_logout = true",
		"[session]", `ttl = "24h"`,
	} {
		if !strings.Contains(config, expected) {
			t.Fatalf("config is missing %s:\n%s", expected, config)
		}
	}
	// The starter page reads the session and signs out with a POST form.
	handler := files["handlers/home_handler.go"]
	if !strings.Contains(handler, "pw.CurrentUser(r.Context())") {
		t.Fatalf("home handler does not read the session:\n%s", handler)
	}
	if strings.Contains(handler, "oidc") || strings.Contains(handler, "Login") && strings.Contains(handler, "func ") && strings.Contains(handler, "Redirect") {
		t.Fatalf("the application must not implement the login itself:\n%s", handler)
	}
	template := files["handlers/home.pw.html"]
	if !strings.Contains(template, `<form method="post" action={logoutPath}>`) {
		t.Fatalf("logout must be a POST form:\n%s", template)
	}
	if !strings.Contains(template, `<a href={loginPath}>`) {
		t.Fatalf("no login link:\n%s", template)
	}
}

func TestScaffoldGeneratesAUniqueSessionSecret(t *testing.T) {
	first := scaffoldFiles(initOptions{Name: "demo", Auth: authOIDC})[pwenv.FileName(pwenv.Development)]
	second := scaffoldFiles(initOptions{Name: "demo", Auth: authOIDC})[pwenv.FileName(pwenv.Development)]
	secretOf := func(config string) string {
		for _, line := range strings.Split(config, "\n") {
			if value, ok := strings.CutPrefix(line, "secret = "); ok {
				return value
			}
		}
		return ""
	}
	if secretOf(first) == "" || len(secretOf(first)) < 20 {
		t.Fatalf("session secret = %q", secretOf(first))
	}
	if secretOf(first) == secretOf(second) {
		t.Fatal("two projects must not share a session secret")
	}
}

func TestScaffoldLeavesPasskeyOnlyDisabled(t *testing.T) {
	files := scaffoldFiles(initOptions{Name: "demo", Auth: authPasskey})
	config := files[pwenv.FileName(pwenv.Development)]
	// Enabling a mode with no implementation would fail at startup, so the
	// scaffold records the choice without breaking the project.
	if !strings.Contains(config, "enabled = false") || !strings.Contains(config, `mode = "passkey_only"`) {
		t.Fatalf("config = %q", config)
	}
	if strings.Contains(files["cmd/demo/main.go"], "popcornwave/auth") {
		t.Fatal("passkey-only must not import the OIDC authentication package")
	}
}

func TestInitWizardAsksForTheProviderOnlyForOIDC(t *testing.T) {
	t.Chdir(t.TempDir())
	model := feedWizard(t, newTestWizard(defaultInitOptions()),
		typeText("demo"), pressKey(tea.KeyEnter),
		pressKey(tea.KeyEnter), // TinyGo
		pressKey(tea.KeyEnter), // Tailwind
		typeText("2"),          // Authentication: OIDC
	)
	if model.reviewing() {
		t.Fatal("choosing OIDC must ask which provider to use")
	}
	if label := model.steps[model.index].label(); label != "OIDC provider" {
		t.Fatalf("step = %q", label)
	}
	model = feedWizard(t, model, typeText("1")) // Local emulator
	options := wizardResult(model, defaultInitOptions())
	if options.Auth != authOIDC || !options.AuthEmulator {
		t.Fatalf("options = %#v", options)
	}
}

func TestInitWizardSkipsTheProviderStepWithoutOIDC(t *testing.T) {
	t.Chdir(t.TempDir())
	model := feedWizard(t, newTestWizard(initOptions{TinyGo: true, Auth: authOIDC, AuthEmulator: true}),
		typeText("demo"), pressKey(tea.KeyEnter),
		pressKey(tea.KeyEnter), // TinyGo
		pressKey(tea.KeyEnter), // Tailwind
		typeText("4"),          // Authentication: Passkey only
	)
	if !model.reviewing() {
		t.Fatalf("expected the review screen, got step %q", model.steps[model.index].label())
	}
	// The seeded emulator answer must not survive a mode that has no provider.
	options := wizardResult(model, initOptions{TinyGo: true, Auth: authOIDC, AuthEmulator: true})
	if options.Auth != authPasskey || options.AuthEmulator {
		t.Fatalf("options = %#v", options)
	}
}

func TestInitWizardGoesBackPastASkippedStep(t *testing.T) {
	t.Chdir(t.TempDir())
	model := feedWizard(t, newTestWizard(defaultInitOptions()),
		typeText("demo"), pressKey(tea.KeyEnter),
		pressKey(tea.KeyEnter), // TinyGo
		pressKey(tea.KeyEnter), // Tailwind
		pressKey(tea.KeyEnter), // Authentication: None
		pressKey(tea.KeyEsc),   // back from the review screen
	)
	if label := model.steps[model.index].label(); label != "Authentication" {
		t.Fatalf("esc landed on %q, want the last asked question", label)
	}
}
