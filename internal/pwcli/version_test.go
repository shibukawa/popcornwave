package pwcli

import (
	"bytes"
	"runtime"
	"runtime/debug"
	"strings"
	"testing"
)

func buildInfo(mainVersion string, settings ...debug.BuildSetting) *debug.BuildInfo {
	info := &debug.BuildInfo{GoVersion: "go1.26.5", Settings: settings}
	info.Main.Version = mainVersion
	return info
}

func TestResolveVersionPrefersInjectedValue(t *testing.T) {
	resolved := resolveVersion("v1.4.0", buildInfo("v0.9.0", debug.BuildSetting{Key: "vcs.revision", Value: "0123456789abcdef0123"}), true)
	if resolved.Version != "1.4.0" {
		t.Fatalf("version = %q, want 1.4.0", resolved.Version)
	}
	if resolved.Commit != "0123456789ab" {
		t.Fatalf("commit = %q, want the 12 character revision", resolved.Commit)
	}
	if resolved.GoVersion != "1.26.5" {
		t.Fatalf("go version = %q, want 1.26.5", resolved.GoVersion)
	}
}

func TestResolveVersionFallsBackToModuleVersion(t *testing.T) {
	resolved := resolveVersion("", buildInfo("v0.9.0"), true)
	if resolved.Version != "0.9.0" {
		t.Fatalf("version = %q, want 0.9.0", resolved.Version)
	}
	if resolved.Commit != "unknown" {
		t.Fatalf("commit = %q, want unknown", resolved.Commit)
	}
}

func TestResolveVersionMarksModifiedLocalBuild(t *testing.T) {
	resolved := resolveVersion("", buildInfo("(devel)",
		debug.BuildSetting{Key: "vcs.revision", Value: "abcdef0123456789"},
		debug.BuildSetting{Key: "vcs.modified", Value: "true"},
	), true)
	if resolved.Version != "devel" {
		t.Fatalf("version = %q, want devel", resolved.Version)
	}
	if resolved.Commit != "abcdef012345-dirty" {
		t.Fatalf("commit = %q, want the dirty revision", resolved.Commit)
	}
}

func TestResolveVersionWithoutBuildInfo(t *testing.T) {
	resolved := resolveVersion("", nil, false)
	if resolved.Version != "devel" || resolved.Commit != "unknown" {
		t.Fatalf("resolved = %+v, want devel and unknown", resolved)
	}
	if resolved.GoVersion != strings.TrimPrefix(runtime.Version(), "go") {
		t.Fatalf("go version = %q, want the runtime version", resolved.GoVersion)
	}
}

// The packaging install checks of decision:homebrew-tap-channel and
// decision:nix-flake-packaging match on this line, so its shape is a contract.
func TestVersionLineShape(t *testing.T) {
	line := versionInfo{Version: "1.4.0", Commit: "0123456789ab", GoVersion: "1.26.5"}.line()
	want := "pw 1.4.0 (0123456789ab, " + runtime.GOOS + "/" + runtime.GOARCH + ", go1.26.5)"
	if line != want {
		t.Fatalf("line = %q, want %q", line, want)
	}
}

func TestMainVersionCommand(t *testing.T) {
	for _, args := range [][]string{{"version"}, {"--version"}, {"-v"}} {
		var stdout, stderr bytes.Buffer
		if code := Main(args, &stdout, &stderr); code != 0 {
			t.Fatalf("Main(%v) = %d, want 0 (stderr %q)", args, code, stderr.String())
		}
		if !strings.HasPrefix(stdout.String(), "pw ") {
			t.Fatalf("Main(%v) stdout = %q, want a pw version line", args, stdout.String())
		}
	}
}

func TestMainVersionRejectsArguments(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Main([]string{"version", "extra"}, &stdout, &stderr); code != 1 {
		t.Fatalf("Main = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "unexpected arguments") {
		t.Fatalf("stderr = %q, want an argument error", stderr.String())
	}
}
