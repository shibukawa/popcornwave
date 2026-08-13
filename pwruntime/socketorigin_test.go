package pwruntime

import "testing"

// published installs chain settings for one test and restores what was there,
// so a test that needs none is not affected by one that did.
func published(t *testing.T, settings ChainSettings) {
	t.Helper()
	previous, had := ResolvedChainSettings()
	PublishChainSettings(settings)
	t.Cleanup(func() {
		if had {
			PublishChainSettings(previous)
			return
		}
		chainSettingsState.Store(nil)
	})
}

func browserHandshake() SocketHandshake {
	return SocketHandshake{Host: "app.example", RemoteAddress: "203.0.113.9:44321"}
}

func TestAnOriginFromThisDeploymentIsAdmitted(t *testing.T) {
	published(t, ChainSettings{})
	check := SocketOriginCheck(browserHandshake(), false)
	if check == nil {
		t.Fatal("published settings produced no check")
	}
	if !check("http://app.example", "app.example") {
		t.Fatal("an upgrade from this deployment's own origin was refused")
	}
}

func TestAnOriginFromAnotherSiteIsRefused(t *testing.T) {
	published(t, ChainSettings{})
	check := SocketOriginCheck(browserHandshake(), false)
	if check("https://evil.example", "app.example") {
		t.Fatal("an upgrade from another site was admitted, which is the hazard this check exists for")
	}
}

// The scheme is part of the comparison, which is what makes a declared proxy
// load-bearing: nothing terminates TLS in the serving path, so an https
// deployment resolves its own origin as http until it names the peer.
func TestAnHTTPSOriginNeedsTheProxyDeclaredToMatch(t *testing.T) {
	handshake := browserHandshake()
	handshake.ForwardedProto = "https"

	published(t, ChainSettings{})
	if SocketOriginCheck(handshake, false)("https://app.example", "app.example") {
		t.Fatal("a forwarded scheme was believed from a peer this deployment never declared")
	}

	published(t, ChainSettings{TrustedProxies: []string{"203.0.113.0/24"}})
	if !SocketOriginCheck(handshake, false)("https://app.example", "app.example") {
		t.Fatal("a forwarded scheme from a declared proxy was not believed, so every browser upgrade is refused")
	}
}

// The socket reads the allowlist the cross-site check already has, rather than
// asking a deployment to declare its origins twice.
func TestADeclaredTrustedOriginIsAdmitted(t *testing.T) {
	settings := ChainSettings{}
	settings.CSRF.TrustedOrigins = []string{"https://console.example"}
	published(t, settings)
	if !SocketOriginCheck(browserHandshake(), false)("https://console.example", "app.example") {
		t.Fatal("an origin this deployment declared acceptable was refused")
	}
}

// Only a browser is required to send Origin, so its absence is a service or a
// command-line client rather than a page on another site.
func TestAHandshakeWithNoOriginIsAdmitted(t *testing.T) {
	published(t, ChainSettings{})
	if !SocketOriginCheck(browserHandshake(), false)("", "app.example") {
		t.Fatal("a non-browser client was refused for sending no Origin")
	}
}

// An opaque origin sends the literal null, which is a browser, and is not the
// same thing as sending nothing.
func TestANullOriginIsRefused(t *testing.T) {
	published(t, ChainSettings{})
	if SocketOriginCheck(browserHandshake(), false)("null", "app.example") {
		t.Fatal("an opaque origin was treated as an absent one")
	}
}

func TestWithNoPublishedSettingsDevelopmentTakesTheModuleDefault(t *testing.T) {
	previous, had := ResolvedChainSettings()
	chainSettingsState.Store(nil)
	t.Cleanup(func() {
		if had {
			PublishChainSettings(previous)
		}
	})
	if SocketOriginCheck(browserHandshake(), true) != nil {
		t.Fatal("development installed a check where the module's own default should stand")
	}
	check := SocketOriginCheck(browserHandshake(), false)
	if check == nil {
		t.Fatal("a deployment with no settings got no check at all")
	}
	if check("https://app.example", "app.example") {
		t.Fatal("an origin was admitted with nothing to judge it against")
	}
	if !check("", "app.example") {
		t.Fatal("a non-browser client was refused, which no setting decides")
	}
}

func TestAnInstalledPolicyReplacesTheFrameworkResolution(t *testing.T) {
	published(t, ChainSettings{})
	SetSocketOriginPolicy(func(origin, _ string) bool { return origin == "https://evil.example" })
	t.Cleanup(func() { SetSocketOriginPolicy(nil) })

	check := SocketOriginCheck(browserHandshake(), false)
	if !check("https://evil.example", "app.example") {
		t.Fatal("the installed policy did not decide")
	}
	if check("http://app.example", "app.example") {
		t.Fatal("the framework resolution answered over an installed policy")
	}
}
