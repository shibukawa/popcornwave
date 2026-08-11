package pwcli

import (
	"strings"
	"testing"
)

const verifyFixtureHead = `[project]
name = "fixture"
main = "./cmd/fixture"
`

// Both checks read bytes the asset walk already holds, so a project that says
// nothing gets both. Anything else would make the default the weaker one.
func TestLoadProjectConfigVerifiesByDefault(t *testing.T) {
	root := t.TempDir()
	writeProjectFixture(t, root, verifyFixtureHead)
	config, err := loadProjectConfig(root)
	if err != nil {
		t.Fatal(err)
	}
	if !config.Assets.Verify || !config.Assets.VerifySVG {
		t.Errorf("verify = %v, svg_scan = %v, want both true", config.Assets.Verify, config.Assets.VerifySVG)
	}
	if len(config.Assets.VerifyAllow) != 0 {
		t.Errorf("allow = %v, want empty", config.Assets.VerifyAllow)
	}
}

func TestLoadProjectConfigVerifyKeys(t *testing.T) {
	root := t.TempDir()
	writeProjectFixture(t, root, verifyFixtureHead+`
[assets.verify]
enabled = false
svg_scan = false
allow = ["vendor/**", "legacy.png"]
`)
	config, err := loadProjectConfig(root)
	if err != nil {
		t.Fatal(err)
	}
	if config.Assets.Verify || config.Assets.VerifySVG {
		t.Errorf("verify = %v, svg_scan = %v, want both false", config.Assets.Verify, config.Assets.VerifySVG)
	}
	if strings.Join(config.Assets.VerifyAllow, ",") != "vendor/**,legacy.png" {
		t.Errorf("allow = %v, want the two configured globs", config.Assets.VerifyAllow)
	}
}

// A glob that cannot match is a mistake worth naming rather than a value to
// ignore: every path it is compared against is relative to the public tree, so
// an absolute or escaping one would silently exempt nothing.
func TestLoadProjectConfigRejectsAnUnmatchableGlob(t *testing.T) {
	for _, glob := range []string{"/etc/passwd", "../outside/*.png", ""} {
		root := t.TempDir()
		writeProjectFixture(t, root, verifyFixtureHead+"\n[assets.verify]\nallow = [\""+glob+"\"]\n")
		if _, err := loadProjectConfig(root); err == nil {
			t.Errorf("allow = %q was accepted", glob)
		}
	}
}
