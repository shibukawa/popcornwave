package pw

import (
	"context"
	"net/http"

	"github.com/shibukawa/popcornwave/pwruntime"
)

// Locale identifies one locale the project declared in its i18n block.
//
// It is the first argument of every generated message function, so the same
// function serves a request handler, a batch job, a mail renderer, and a push
// notification. There is no separate library mode: the generated surface
// already is one. See .knowledge api:locale-accessors.
type Locale = pwruntime.Locale

// LocaleContext returns the locale resolved for this request.
//
// Resolution happens once, at routing, before the first byte. A template never
// calls this — a message reference reads the framework's implicit binding
// instead, which is what lets a cached component key on the locale without any
// rule about messages. See .knowledge data:locale-bindings.
func LocaleContext(ctx context.Context) Locale { return pwruntime.LocaleContext(ctx) }

// RequestLocale is LocaleContext for a handler. It is not spelled Locale
// because that name is the locale type itself.
func RequestLocale(r *http.Request) Locale { return pwruntime.LocaleContext(r.Context()) }

// ParseLocale resolves a BCP 47 tag against the declared set by RFC 4647
// lookup, so ja-JP finds ja. It reports absence rather than substituting the
// default.
//
// This is the entry point for a locale that did not come from a request: a
// value read from a user record before sending mail, or from a job payload.
func ParseLocale(tag string) (Locale, bool) { return pwruntime.ParseLocale(tag) }

// DefaultLocale returns the locale named by i18n.default_locale. It is the zero
// Locale in a project with no generated message package.
func DefaultLocale() Locale { return pwruntime.DefaultLocale() }

// DeclaredLocales returns the declared tags in declaration order, nil in a
// project that declared none.
func DeclaredLocales() []string { return pwruntime.DeclaredLocales() }

// LocaleChoice is one entry of a language switcher, and one alternate link.
type LocaleChoice = pwruntime.LocaleChoice

// LocaleChoices reports the locales the current page is available in.
//
// The switcher control, the hreflang alternates, and the canonical all read
// this one call, so a page cannot have them disagree. A mode with no per-locale
// URL reports entries whose URL is empty, because switching there is a server
// action rather than a link. See .knowledge requirement:locale-switching-surface.
func LocaleChoices(r *http.Request) []LocaleChoice { return pwruntime.LocaleChoices(r) }

// SetLocale records a reader's explicit language choice.
//
// The framework owns the cookie and its validation; the control that offers the
// choice, and the action handler that calls this, stay with the application, on
// the same terms requirement:user-preference-rendering divides an override.
func SetLocale(w http.ResponseWriter, locale Locale) { pwruntime.SetLocale(w, locale) }

// LocalePath builds the URL of a path in a locale.
//
// It is for URLs composed outside a template: a redirect Location, a mail body,
// a push deep link. Inside a template the locale is written with the framework's
// binding instead, per .knowledge decision:explicit-locale-in-links.
func LocalePath(r *http.Request, locale Locale, path string) string {
	return LocalePathContext(r.Context(), locale, path)
}

// LocalePathContext is LocalePath for code below the handler — a mail body or
// a job payload composed after the response.
func LocalePathContext(ctx context.Context, locale Locale, path string) string {
	return pwruntime.LocalePath(locale, pwruntime.LocaleModeContext(ctx), path)
}
