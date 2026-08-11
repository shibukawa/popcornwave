package pwfast

import (
	"context"
	"net/http"
	"strings"

	"github.com/shibukawa/popcornwave/pwruntime"
	"github.com/shibukawa/popcornwave/session"
	"github.com/shibukawa/tinygodriver/fasthttp"
)

// Carrier carries a session over this transport.
//
// It is three methods, and none of them is a decision: reading the cookies that
// arrived, setting the ones that leave, and handing over the context. Every
// rule about what a session does — when a token rotates, which cookie a stale
// record clears, what separates browser state that is merely old from a backend
// that is down — stays in the session package and is reached through
// Manager.Resolve. That is the point of the seam: a session is the last thing
// that should exist twice, because two implementations of when a session ends
// are two chances to leave one valid that should not be.
type Carrier struct{ ctx *fasthttp.RequestCtx }

// NewCarrier wraps a request value as a session carrier.
func NewCarrier(r *fasthttp.RequestCtx) session.Carrier { return Carrier{ctx: r} }

// Cookies returns the cookies the request carried.
//
// They are translated into net/http's cookie struct, which the session package
// uses as its vocabulary: it is a plain description of a cookie with no
// transport in it. Only the name and the value are read from a request cookie —
// a browser sends nothing else — so nothing is lost in the translation.
func (c Carrier) Cookies() []*http.Cookie {
	var cookies []*http.Cookie
	c.ctx.Request.Header.VisitAllCookie(func(name, value []byte) {
		cookies = append(cookies, &http.Cookie{Name: string(name), Value: string(value)})
	})
	return cookies
}

// SetCookie adds one Set-Cookie to the response.
func (c Carrier) SetCookie(cookie *http.Cookie) {
	if cookie == nil {
		return
	}
	out := fasthttp.AcquireCookie()
	defer fasthttp.ReleaseCookie(out)
	out.SetKey(cookie.Name)
	out.SetValue(cookie.Value)
	if cookie.Path != "" {
		out.SetPath(cookie.Path)
	}
	if cookie.Domain != "" {
		out.SetDomain(cookie.Domain)
	}
	if !cookie.Expires.IsZero() {
		out.SetExpire(cookie.Expires)
	}
	// MaxAge is copied only when it is set, because fasthttp writes Max-Age for
	// any non-zero value and a zero from net/http means "unset" rather than
	// "expire now" — the delete case is negative, which does carry through.
	if cookie.MaxAge != 0 {
		out.SetMaxAge(cookie.MaxAge)
	}
	out.SetSecure(cookie.Secure)
	out.SetHTTPOnly(cookie.HttpOnly)
	out.SetSameSite(sameSite(cookie.SameSite))
	c.ctx.Response.Header.SetCookie(out)
}

// Context is the request's context, which is the request value itself.
func (c Carrier) Context() context.Context { return c.ctx }

// sameSite translates the attribute, which is an enum on both sides and spelled
// differently on each.
func sameSite(mode http.SameSite) fasthttp.CookieSameSite {
	switch mode {
	case http.SameSiteLaxMode:
		return fasthttp.CookieSameSiteLaxMode
	case http.SameSiteStrictMode:
		return fasthttp.CookieSameSiteStrictMode
	case http.SameSiteNoneMode:
		return fasthttp.CookieSameSiteNoneMode
	default:
		return fasthttp.CookieSameSiteDisabled
	}
}

// UnavailableHandler answers a request whose session could not be read because
// the backend failed.
type UnavailableHandler func(r *fasthttp.RequestCtx, err error)

// Session resolves the session of every request and publishes it for the rest
// of the chain.
//
// The resolution is the session package's, so this transport and the other
// apply one set of rules to one browser. What differs is only where the
// resolved session is recorded: the other half derives a context and hands it
// on, and this one writes into the request value.
//
// A nil manager installs nothing, which is how a deployment with session
// storage disabled opts out — and it opts out by having no frame rather than by
// having an inert one, because a session frame that silently does nothing is a
// control that looks installed.
func Session(manager *session.Manager, unavailable UnavailableHandler) Middleware {
	if unavailable == nil {
		unavailable = writeSessionUnavailable
	}
	return func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
		if manager == nil {
			return next
		}
		return func(r *fasthttp.RequestCtx) {
			resolved, err := manager.Resolve(NewCarrier(r))
			if err != nil {
				unavailable(r, err)
				return
			}
			// The key stays the session package's: it writes the value itself
			// rather than exporting the key, because an exported session key is
			// one any code could write, and writing one is presenting one.
			resolved.StoreOn(r)
			next(r)
		}
	}
}

// writeSessionUnavailable answers a backend failure, which is a 503 rather than
// a 500: the request could not be served now and may be served later, and a
// client that retries is doing the right thing.
func writeSessionUnavailable(r *fasthttp.RequestCtx, err error) {
	pwruntime.ReadLogger(r).Log(r, pwruntime.LevelError, "session backend unavailable",
		pwruntime.String("error", strings.TrimSpace(err.Error())))
	r.Response.Header.Set("Cache-Control", "no-store")
	WriteProblem(r, ServiceUnavailable("session storage is unavailable"))
}
