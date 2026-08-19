//go:build go1.27

package testutil

// The configuration operations as methods on the isolated copy.
//
// Each is the matching package function with the configuration as the receiver,
// so a test reads config.Update rather than naming this package twice on one
// line. The functions keep the bodies until they are retired; see
// pwruntime/cache_go127.go for why the whole set is tagged rather than shipped,
// and for what has to happen if Go 1.27 arrives without the feature.
//
// Two of the three infer their type from an argument and still had to wait: a
// method may not declare a type parameter even when the call site writes none.

// Get returns one typed value from a copied configuration. It is the one entry
// with nothing to infer from, so the type is written at the call site:
// config.Get[pw.ServerConfig]().
func (c *Config) Get[T any]() T { return Get[T](c) }

// Set replaces one typed value in a copied configuration.
func (c *Config) Set[T any](value T) { Set[T](c, value) }

// Update edits one typed value in a copied configuration.
func (c *Config) Update[T any](edit func(*T)) { Update[T](c, edit) }
