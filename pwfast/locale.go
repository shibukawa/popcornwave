package pwfast

import (
	"github.com/shibukawa/popcornweb/pwruntime"
	"github.com/shibukawa/tinygodriver/fasthttp"
)

// LocaleChoices reports the locales the current page is available in, for a
// language switcher and for the alternate links.
//
// It is the fasthttp half of the same entry the net/http build has. The switcher
// data is the same on both transports because it is derived from the declared
// locale set, the route's mode, and this request's path — none of which is a
// property of the runtime that served it.
func LocaleChoices(r *fasthttp.RequestCtx) []pwruntime.LocaleChoice {
	if r == nil {
		return nil
	}
	return pwruntime.LocaleChoicesFor(
		string(r.Path()), string(r.URI().QueryString()),
		pwruntime.LocaleContext(r), pwruntime.LocaleModeContext(r))
}

// SetLocale records a reader's explicit language choice.
//
// The value is validated against the declared set before it is written, because
// the cookie is client-writable and a decoded value is request input.
func SetLocale(r *fasthttp.RequestCtx, locale pwruntime.Locale) {
	if r == nil || !locale.Valid() {
		return
	}
	cookie := fasthttp.AcquireCookie()
	defer fasthttp.ReleaseCookie(cookie)
	cookie.SetKey(pwruntime.LocaleCookieName)
	cookie.SetValue(locale.Tag())
	cookie.SetPath("/")
	cookie.SetMaxAge(pwruntime.LocaleCookieMaxAge)
	cookie.SetSameSite(fasthttp.CookieSameSiteLaxMode)
	r.Response.Header.SetCookie(cookie)
}
