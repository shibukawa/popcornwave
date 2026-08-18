package pwfast

import (
	"github.com/shibukawa/popcornweb/pwconfig"
	"github.com/shibukawa/popcornweb/pwruntime"
	"github.com/shibukawa/tinybind-go/htmlbind"
	"github.com/shibukawa/tinygodriver/fasthttp"
)

// writeHTMLProblem answers a failure with the application's error page, and
// reports whether it did.
//
// It renders through the registered document shell, so the page keeps the
// application's chrome instead of arriving as a bare fragment — a signed-in
// reader's name in the header of a 500 is the ordinary case rather than an
// unusual one. The status is the real one: nothing has committed here, so this
// half can say 500 and show the reader the page at the same time.
//
// Every way of not producing a page returns false and leaves the caller to
// write the document, including the error page's own render failure. An error
// page that recursed into another one would be the worst possible response to
// a failure.
func writeHTMLProblem(r *fasthttp.RequestCtx, problem Problem) bool {
	resolve := pwruntime.RegisteredHTMLErrorPage()
	if resolve == nil || !pwruntime.AcceptsHTML(string(r.Request.Header.Peek("Accept"))) {
		return false
	}
	fragment := resolve(publicProblem(problem))
	if !fragment.Present() {
		return false
	}
	wrappers := pwruntime.RegisteredHTMLDocument()
	var body []byte
	buffer := bytesWriter{buf: &body}
	if err := htmlbind.RenderChain(&buffer, wrappers, fragment); err != nil {
		pwruntime.ReadLogger(r).Log(r, pwruntime.LevelError, "HTML error page render failed",
			pwruntime.String("error", err.Error()))
		return false
	}
	// The representation was chosen from a request header, so a cache keyed on
	// the URL alone must not serve one client's answer to another.
	addVaryHeader(r, "Accept")
	writeChainCachePolicy(r, wrappers, fragment)
	r.Response.Header.SetContentType(htmlContentType)
	r.SetStatusCode(problem.Status)
	r.Response.SetBody(body)
	return true
}

// publicProblem is what an error page is allowed to see.
//
// Outside development it is the status and the title and nothing else, so a
// detail meant for an operator cannot reach a reader through a template that
// happened to render it.
func publicProblem(problem Problem) Problem {
	if pwconfig.Development() {
		return problem
	}
	return Problem{Status: problem.Status, Title: problem.Title}
}
