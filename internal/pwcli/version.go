package pwcli

import (
	"fmt"
	"io"
	"runtime"
	"runtime/debug"
	"strings"
)

// version is written only by the release linker through
// -ldflags "-X github.com/shibukawa/popcornwave/internal/pwcli.version=1.2.3".
var version string

type versionInfo struct {
	Version   string
	Commit    string
	GoVersion string
}

func runVersion(args []string, stdout io.Writer) error {
	if len(args) != 0 {
		return fmt.Errorf("version: unexpected arguments")
	}
	info, ok := debug.ReadBuildInfo()
	fmt.Fprintln(stdout, resolveVersion(version, info, ok).line())
	return nil
}

func (v versionInfo) line() string {
	return fmt.Sprintf("pw %s (%s, %s/%s, go%s)", v.Version, v.Commit, runtime.GOOS, runtime.GOARCH, v.GoVersion)
}

// resolveVersion follows the api:cli-version resolution order: the injected
// value, the module version recorded by go install, the VCS revision of a local
// build, and finally the literal devel.
func resolveVersion(injected string, info *debug.BuildInfo, ok bool) versionInfo {
	resolved := versionInfo{Version: trimVersionPrefix(injected), Commit: "unknown", GoVersion: trimGoPrefix(runtime.Version())}
	if !ok || info == nil {
		if resolved.Version == "" {
			resolved.Version = "devel"
		}
		return resolved
	}
	if info.GoVersion != "" {
		resolved.GoVersion = trimGoPrefix(info.GoVersion)
	}
	revision, modified := vcsSettings(info)
	if revision != "" {
		resolved.Commit = shortRevision(revision)
		if modified {
			resolved.Commit += "-dirty"
		}
	}
	if resolved.Version != "" {
		return resolved
	}
	if module := trimVersionPrefix(info.Main.Version); module != "" && module != "(devel)" {
		resolved.Version = module
		return resolved
	}
	if revision != "" {
		resolved.Version = "devel"
		return resolved
	}
	resolved.Version = "devel"
	return resolved
}

func vcsSettings(info *debug.BuildInfo) (revision string, modified bool) {
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			revision = setting.Value
		case "vcs.modified":
			modified = setting.Value == "true"
		}
	}
	return revision, modified
}

func shortRevision(revision string) string {
	if len(revision) > 12 {
		return revision[:12]
	}
	return revision
}

// trimVersionPrefix drops the tag's leading v so every channel reports the same
// string the Homebrew formula and the Nix derivation were built with.
func trimVersionPrefix(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > 1 && value[0] == 'v' && value[1] >= '0' && value[1] <= '9' {
		return value[1:]
	}
	return value
}

func trimGoPrefix(value string) string { return strings.TrimPrefix(value, "go") }
