//go:build tinygo

package pw

// frameworkVersion reports nothing under TinyGo. TinyGo builds carry no module
// build information, so the startup summary omits the version instead of
// printing a placeholder.
func frameworkVersion() string { return "" }
