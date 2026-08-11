package pwfast

import (
	"github.com/shibukawa/popcornwave/pwbrowser"
	"github.com/shibukawa/tinygodriver/fasthttp"
)

// The browser runtime is the shared leaf's — the bytes, the set, the revision
// the URL carries — because a document naming it is rendered by whichever half
// is running. What is here is this transport's half of serving it.
//
// Without this the second build rendered a page whose script tag pointed at a
// 404: both server halves of the update surface worked and nothing client-side
// could reach them.

// RuntimeScriptURL is the absolute path of the boundary runtime module. A
// document template names it through a declared external function rather than
// as a literal, so the template text survives an upgrade that moves the URL.
func RuntimeScriptURL() string { return pwbrowser.RuntimeScriptURL() }

// FrameworkAssets answers the framework's own browser assets, and closes the
// namespace they live in.
//
// Closing it is the second half and not an extra: the prefix is reserved, so an
// unclaimed path inside it — a stale script revision, a probe — answers 404
// here rather than reaching application routing. One reserved prefix is then a
// single routing and access rule instead of a hole an application could
// accidentally serve through.
func FrameworkAssets() Middleware {
	return func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
		return func(r *fasthttp.RequestCtx) {
			path := string(r.Path())
			if !pwbrowser.Claims(path) {
				next(r)
				return
			}
			source, contentType, ok := pwbrowser.Lookup(path)
			if !ok {
				r.Error(fasthttp.StatusMessage(fasthttp.StatusNotFound), fasthttp.StatusNotFound)
				return
			}
			if !operationalMethod(r) {
				return
			}
			r.Response.Header.SetContentType(contentType)
			r.Response.Header.Set("Cache-Control", pwbrowser.CacheControl)
			r.SetStatusCode(fasthttp.StatusOK)
			if string(r.Method()) == "HEAD" {
				// The length is what a HEAD is for, and this transport would
				// otherwise report the body it is not sending.
				r.Response.Header.SetContentLength(len(source))
				r.Response.SkipBody = true
				return
			}
			r.Response.SetBodyString(source)
		}
	}
}
