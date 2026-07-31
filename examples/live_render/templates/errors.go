package templates

import "github.com/shibukawa/popcornwave/pw"

func init() {
	// The framework renders this in place of a page whose boundary failed with
	// no recover clause. It receives the mapped problem, never the original
	// error, so a template cannot print a cause the server meant to keep.
	pw.RegisterHTMLErrorPage(func(problem pw.Problem) pw.HTMLFragment {
		return Error500(Error500Params{Title: problem.Title})
	})
}
