package pwfast

import (
	"github.com/shibukawa/tinybind-go/fasthttpbind"
	"github.com/shibukawa/tinybind-go/htmlbind"
	"github.com/shibukawa/tinygodriver/fasthttp"
)

// The composition types are htmlbind's on both transports. They describe
// rendering rather than a transport, so there is nothing here to mirror: pw
// aliases the same declarations, and a fragment built by generated code is the
// same value whichever half receives it.
type (
	HTMLFragment = htmlbind.Fragment
	HTMLWrapper  = htmlbind.Wrapper
	HTMLOption   = htmlbind.Option
)

// The error types are the module's shared leaf, aliased rather than
// redeclared. Two definitions that agree today are two chances to disagree
// later, and the failure is silent: an errors.As that stops matching, a problem
// that no longer unwraps.
type (
	Problem    = fasthttpbind.Problem
	FieldError = fasthttpbind.FieldError
	HTTPError  = fasthttpbind.HTTPError
)

// Parse binds the request into the generated input type.
func Parse[T any](r *fasthttp.RequestCtx) (T, error) { return fasthttpbind.Bind[T](r) }

// WriteAPI writes one typed API response, or the problem that describes why it
// could not.
func WriteAPI[T any](r *fasthttp.RequestCtx, value T) {
	if err := fasthttpbind.Write(r, value); err != nil {
		WriteProblem(r, err)
	}
}

// WriteAPIStatus is WriteAPI with the status the handler chose.
func WriteAPIStatus[T any](r *fasthttp.RequestCtx, status int, value T) {
	if err := fasthttpbind.WriteStatus(r, status, value); err != nil {
		WriteProblem(r, err)
	}
}

// WriteProblem answers with the problem document describing err.
//
// The net/http half also negotiates an HTML error page from Accept, which this
// one does not yet: that page is registered in pw, and reaching it needs the
// same shared-leaf move the document shell needs. The problem body itself is
// byte-identical across the two halves, which is the part a client parses.
func WriteProblem(r *fasthttp.RequestCtx, err error) {
	fasthttpbind.WriteError(r, err)
}

// WriteStream answers with a typed event stream, running fn to produce it.
//
// The callback is the shape rather than a returned stream because this
// transport registers a body writer that runs after the handler returns, so a
// stream the handler held would have nothing to write into. The callback
// returning is the close, and an error from it is post-commit and reaches the
// installed stream error handler.
func WriteStream[T any](r *fasthttp.RequestCtx, fn func(*fasthttpbind.Stream[T]) error) {
	fasthttpbind.WriteStream(r, fn)
}

// SetStreamErrorHandler installs what receives a stream failure raised after
// the response committed. It is process-wide and shared with the net/http half.
func SetStreamErrorHandler(fn func(error)) { fasthttpbind.SetStreamErrorHandler(fn) }
