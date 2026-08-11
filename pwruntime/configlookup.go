package pwruntime

import (
	"context"
	"reflect"
	"sync/atomic"
)

// The resolved configuration is reached through a published lookup rather than
// moved here.
//
// Registration, defaults, environment overlay, scaffold emission and the boot
// report all live around the registry in pw and none of them is transport
// shaped, so moving the registry would move a great deal of unrelated code to
// share one read. Publishing the read shares exactly the read.
//
// The shape is a function over reflect.Type because the values are of types
// this package cannot name: an application registers its own configuration
// structs, and the registry is keyed by them.
var configLookup atomic.Pointer[func(reflect.Type) (any, bool)]

// PublishConfigLookup records how to read the resolved configuration. Whichever
// runtime owns configuration parsing calls it once.
//
// It returns whatever was published before, so a caller installing one
// temporarily can put that back rather than leaving nothing behind. Publishing
// nil is how a caller takes its own lookup away, and a process left with none
// answers every binding with its zero value.
func PublishConfigLookup(lookup func(reflect.Type) (any, bool)) func(reflect.Type) (any, bool) {
	previous := configLookup.Swap(&lookup)
	if previous == nil {
		return nil
	}
	return *previous
}

// RegisteredConfig returns the resolved value for T, and whether one was
// registered and parsed.
func RegisteredConfig[T any]() (T, bool) {
	var zero T
	lookup := configLookup.Load()
	// Both halves: nothing has published, or something published nil to take
	// its lookup back. The second is how a test undoes one, and storing a
	// pointer to a nil function makes the pointer check alone pass.
	if lookup == nil || *lookup == nil {
		return zero, false
	}
	value, ok := (*lookup)(reflect.TypeFor[T]())
	if !ok {
		return zero, false
	}
	pointer, ok := value.(*T)
	if !ok || pointer == nil {
		return zero, false
	}
	return *pointer, true
}

// ResolveConfig answers the way both runtimes' Config accessors do: the value
// carried on the request if a middleware put one there, and otherwise the
// process-wide resolved value.
//
// The per-request value comes first because that is what lets a test or a
// development mode serve one request with configuration that is not the
// process's. Falling back rather than requiring it is what lets a caller
// outside a request — startup validation, or a runtime with no middleware of
// its own — read the same settings.
func ResolveConfig[T any](ctx context.Context) T {
	if value, ok := Config[T](ctx); ok {
		return value
	}
	if value, ok := RegisteredConfig[T](); ok {
		return value
	}
	var zero T
	return zero
}
