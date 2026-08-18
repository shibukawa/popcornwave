//go:build !tinygo

package pw

import (
	"runtime/debug"
	"sync"
)

const frameworkModule = "github.com/shibukawa/popcornweb"

var frameworkVersionOnce = sync.OnceValue(func() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	if info.Main.Path == frameworkModule {
		return normalizeFrameworkVersion(info.Main.Version)
	}
	for _, dependency := range info.Deps {
		if dependency.Path == frameworkModule {
			return normalizeFrameworkVersion(dependency.Version)
		}
	}
	return ""
})

// frameworkVersion reports the framework module version recorded in the binary,
// or an empty string when the build carries no module information.
func frameworkVersion() string { return frameworkVersionOnce() }

func normalizeFrameworkVersion(version string) string {
	if version == "" || version == "(devel)" {
		return ""
	}
	return version
}
