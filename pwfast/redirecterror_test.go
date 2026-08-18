package pwfast

import (
	"net/http"
	"testing"

	"github.com/shibukawa/popcornweb/pwruntime"
	"github.com/shibukawa/tinygodriver/fasthttp"
)

// The other transport answers a returned redirect the same way, because a page
// that loads its own data is written once and served by whichever build runs.
func TestAReturnedRedirectBecomesARedirectResponse(t *testing.T) {
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(http.MethodGet)
	ctx.Request.SetRequestURI("/account")

	WriteProblem(ctx, pwruntime.RedirectError{Location: "/auth/login", Status: http.StatusSeeOther})

	if ctx.Response.StatusCode() != http.StatusSeeOther {
		t.Errorf("status = %d, want 303", ctx.Response.StatusCode())
	}
	if location := string(ctx.Response.Header.Peek("Location")); location != "/auth/login" {
		t.Errorf("Location = %q, want /auth/login", location)
	}
}

// And an ordinary problem is untouched by the recognition above.
func TestAProblemIsStillAProblem(t *testing.T) {
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod(http.MethodGet)
	ctx.Request.SetRequestURI("/missing")

	WriteProblem(ctx, NotFound("no such user"))

	if ctx.Response.StatusCode() != http.StatusNotFound {
		t.Errorf("status = %d, want 404", ctx.Response.StatusCode())
	}
}
