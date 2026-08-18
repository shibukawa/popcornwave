//go:build !pwdev

package pw

import "github.com/shibukawa/popcornweb/pwruntime"

// A release build starts no data pane and links none of it. The absence is
// structural rather than conditional: there is no branch here to take at run
// time and no reference to the package that would serve it.
func startDevelopmentData(pwruntime.Resources) {}
