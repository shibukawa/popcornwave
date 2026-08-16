package pwruntime

import (
	"net/http"
	"net/url"
	"strings"
)

// LocaleMiddleware resolves the request's locale before the handler runs and
// records the Vary the route's mode implies.
//
// Resolution happens here rather than lazily at the first message because
// headers are final before the body under flow:initial-streaming-render, and a
// Vary decided mid-render is decided too late.
//
// A project declaring no locale is passed through untouched.
func LocaleMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(DeclaredLocales()) == 0 {
			next.ServeHTTP(w, r)
			return
		}
		mode, declared := localeModeFor(r.URL.Path)
		if !declared {
			// A route outside every declared prefix serves the default with
			// nothing in its URL. Header mode is what puts nothing there.
			next.ServeHTTP(w, r.WithContext(
				WithLocaleMode(WithLocale(r.Context(), DefaultLocale()), LocaleModeHeader)))
			return
		}

		switch mode {
		case LocaleModePath:
			if !servePathMode(w, r, next) {
				return
			}
		default:
			locale := negotiate(r, mode)
			// The route negotiates, so it varies on what it negotiates from,
			// on every response and not only the ones that carried the signal.
			// A reader with no cookie would otherwise fill a shared cache with
			// an unvaried default that the next reader is then served.
			// See .knowledge policy:locale-vary-correctness.
			for _, axis := range varyAxes(mode) {
				w.Header().Add("Vary", axis)
			}
			w.Header().Set("Content-Language", locale.Tag())
			next.ServeHTTP(w, r.WithContext(WithLocaleMode(WithLocale(r.Context(), locale), mode)))
		}
	})
}

// servePathMode handles a route whose locale is a URL prefix. It reports whether
// the request should continue to the handler.
func servePathMode(w http.ResponseWriter, r *http.Request, next http.Handler) bool {
	rest, locale, found := StripLocalePrefix(r.URL.Path)
	if found {
		if !prefixesDefault() && locale.Tag() == DefaultLocale().Tag() {
			// The default locale has no prefixed form under this setting, so
			// its prefixed URL is redirected permanently rather than served:
			// one representation, one URL.
			redirect(w, r, rest, http.StatusMovedPermanently)
			return false
		}
		stripped := *r.URL
		stripped.Path = rest
		request := r.Clone(WithLocaleMode(WithLocale(r.Context(), locale), LocaleModePath))
		request.URL = &stripped
		w.Header().Set("Content-Language", locale.Tag())
		next.ServeHTTP(w, request)
		return false
	}

	if !prefixesDefault() {
		// The unprefixed path is the default locale's own page here, so it is
		// served rather than redirected.
		locale := DefaultLocale()
		w.Header().Set("Content-Language", locale.Tag())
		next.ServeHTTP(w, r.WithContext(WithLocaleMode(WithLocale(r.Context(), locale), LocaleModePath)))
		return false
	}

	// Every locale carries a prefix, so an unprefixed path names no
	// representation. It is negotiated and redirected, temporarily: a permanent
	// status would cache one reader's negotiation for the next.
	target := negotiate(r, LocaleModeCookie)
	w.Header().Add("Vary", "Accept-Language")
	w.Header().Add("Vary", "Cookie")
	redirect(w, r, LocalePath(target, LocaleModePath, r.URL.Path), http.StatusFound)
	return false
}

func redirect(w http.ResponseWriter, r *http.Request, path string, status int) {
	target := url.URL{Path: path, RawQuery: r.URL.RawQuery}
	http.Redirect(w, r, target.String(), status)
}

// negotiate resolves a locale from the signals a mode reads.
//
// A cookie outranks a header for the reason decision:preference-signal-precedence
// gives for the same ordering: it holds a choice the reader made on this site,
// and a header reports an environment.
func negotiate(r *http.Request, mode LocaleMode) Locale {
	if mode == LocaleModeCookie {
		if cookie, err := r.Cookie(LocaleCookieName); err == nil {
			if locale, ok := ParseLocale(cookie.Value); ok {
				return locale
			}
		}
	}
	if header := r.Header.Get("Accept-Language"); header != "" {
		if locale, ok := negotiateAcceptLanguage(header); ok {
			return locale
		}
	}
	return DefaultLocale()
}

// varyAxes is what a mode's responses vary on.
func varyAxes(mode LocaleMode) []string {
	switch mode {
	case LocaleModeCookie:
		// Cookie splits a shared cache per session, since HTTP cannot vary on
		// one cookie. That is what private content already is, which is why
		// this mode is for an authenticated application rather than a public
		// page.
		return []string{"Cookie", "Accept-Language"}
	case LocaleModeHeader:
		return []string{"Accept-Language"}
	default:
		// Path mode: two languages are two URLs, so nothing varies.
		return nil
	}
}

// LocaleAlternateLinks renders the hreflang alternates of the current page.
//
// It reads the same data the switcher does, so the two cannot drift. A mode with
// no per-locale URL produces nothing, because there is no URL to point at.
func LocaleAlternateLinks(r *http.Request, origin string) string {
	choices := LocaleChoices(r)
	var out strings.Builder
	defaultTag := DefaultLocale().Tag()
	for _, choice := range choices {
		if choice.URL == "" {
			return ""
		}
		out.WriteString(`<link rel="alternate" hreflang="` + choice.Locale.Tag() + `" href="` + origin + choice.URL + `">`)
		if choice.Locale.Tag() == defaultTag {
			out.WriteString(`<link rel="alternate" hreflang="x-default" href="` + origin + choice.URL + `">`)
		}
	}
	return out.String()
}
