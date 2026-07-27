package pw

import (
	"context"

	"github.com/shibukawa/tinybind-go/htmlbind"
)

// Pending is one value a handler started before rendering and a template waits
// for in an await clause. A template parameter declared `async T` becomes a
// Pending[T] field in the generated Params struct.
//
// A handler adopts progressive rendering by supplying these and changing
// nothing else: WriteHTML notices that the composed chain can open a boundary
// and streams on its own.
type Pending[T any] = htmlbind.Pending[T]

// AsyncError is the presentation-safe failure a recover clause renders. The
// original Go error stays server-side and reaches the logger instead.
type AsyncError = htmlbind.AsyncError

// PublicError is implemented by an error that supplies its own safe projection.
// Any other error reaches a recover clause as an internal code with no message,
// so error text cannot leak into a page by accident.
type PublicError = htmlbind.PublicError

// UnsetPendingError reports a required async value the handler never set. It is
// raised before any byte commits, so it becomes an ordinary problem response.
type UnsetPendingError = htmlbind.UnsetPendingError

// Go starts work in its own goroutine and returns the handle to pass as a
// template parameter. ctx bounds the work and stays the caller's to cancel; a
// render bounds only how long it waits.
//
// A panic inside work becomes the handle's error, so a boundary reports it
// through its recover clause instead of taking the process down.
func Go[T any](ctx context.Context, work func(context.Context) (T, error)) Pending[T] {
	return htmlbind.Go(ctx, work)
}

// Resolved returns a handle that has already settled to value. It is what a
// handler passes when it computed the value itself, and what a test passes
// instead of starting a goroutine.
func Resolved[T any](value T) Pending[T] { return htmlbind.Resolved(value) }

// Failed returns a handle that has already settled to err. The boundary that
// awaits it renders its recover subtree.
func Failed[T any](err error) Pending[T] { return htmlbind.Failed[T](err) }
