package templates

// The shared leaf rather than a runtime: this file belongs to both builds,
// and the error page registry is one process-wide table either of them reads.
import "github.com/shibukawa/popcornwave/pwruntime"

func init() {
	// The framework renders this in place of a page whose boundary failed with
	// no recover clause. It receives the mapped problem, never the original
	// error, so a template cannot print a cause the server meant to keep.
	pwruntime.RegisterHTMLErrorPage(func(problem pwruntime.Problem) pwruntime.HTMLFragment {
		return Error500(Error500Params{Title: problem.Title})
	})
}
