package pw

import "github.com/shibukawa/popcornwave/middlewares"

// Lifecycle describes the deprecation and expected shutdown dates of the
// routes wrapped by LifecycleHeaders.
type Lifecycle = middlewares.Lifecycle

// LifecycleHeaders returns middleware that announces an API resource's
// lifecycle using RFC 9745 Deprecation and RFC 8594 Sunset fields.
func LifecycleHeaders(lifecycle Lifecycle) (Middleware, error) {
	return middlewares.LifecycleHeaders(lifecycle)
}
