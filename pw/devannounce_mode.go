//go:build !pwdev

package pw

// A release build has no development console to announce to, and links none of
// the code that would reach one. The absence is structural rather than
// conditional: there is no branch here to take at run time.
func announceDevelopmentListener(string) {}
