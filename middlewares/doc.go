// Package middlewares contains the net/http middleware shared by Popcorn Web
// applications. Every middleware is an ordinary func(http.Handler) http.Handler
// so it composes with any standard library compatible middleware, and every
// runtime dependency is supplied through options instead of package globals.
package middlewares

import "net/http"

// Middleware is the standard Go HTTP middleware signature.
type Middleware = func(http.Handler) http.Handler
