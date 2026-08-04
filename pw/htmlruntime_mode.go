//go:build !pwdev

package pw

// A release build has no development modules and no import of one.
//
// The absence is structural rather than conditional: there is no branch here to
// take at run time and no string naming a module that does not exist, so a
// deployed binary cannot be talked into serving one.

func developmentImport() string { return "" }

func developmentScripts() map[string]string { return nil }
