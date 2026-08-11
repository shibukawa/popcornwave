package pw

import (
	"net/http"
	"net/url"

	tinybind "github.com/shibukawa/tinybind-go"
)

func Parse[T any](r *http.Request) (T, error) { return tinybind.Bind[T](r) }

// The route decoders generation emits read their inputs through these rather
// than off the request directly, which is what keeps a generated decoder from
// naming net/http at all.
//
// A method on the concrete request type is a call the fasthttp source transform
// cannot rewrite, so a decoder written as r.PathValue would take the
// compatibility fallback and refuse the handler around it. A function taking the
// request in a fixed position is a call pattern the transform rewrites
// mechanically, which is the same reasoning that moved every request accessor to
// this shape. Each one therefore has a counterpart in the fasthttp package,
// under the same name and with the transport value in the same position.
//
// They are wrappers and deliberately nothing more: the behaviour is the
// module's, so a generated decoder here and one built against the module read
// their inputs identically.

// Queries parses the request's query string once. A generated decoder calls this
// a single time and resolves each field with QueryLookup, rather than re-parsing
// the raw query per field.
func Queries(r *http.Request) url.Values { return tinybind.Queries(r) }

// QueryLookup returns the first value for key from pre-parsed query values. A
// key present with an empty value reports ("", true), which is what lets a
// decoder tell an empty parameter from an absent one.
func QueryLookup(q url.Values, key string) (string, bool) { return tinybind.QueryLookup(q, key) }

// PathValue returns the routed path value for key.
func PathValue(r *http.Request, key string) string { return tinybind.PathValue(r, key) }
