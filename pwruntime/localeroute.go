package pwruntime

import (
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// LocaleRoute binds one path prefix to the way its locale is decided. The list
// is registered by the generated message package, because the modes are
// declared in the project's build configuration.
type LocaleRoute struct {
	Prefix string
	Mode   LocaleMode
}

// LocaleChoice is one entry of a language switcher, and one alternate link.
//
// The switcher, the hreflang alternates, and the canonical all read this, so a
// page cannot have its switcher and its alternates disagree. See
// .knowledge requirement:locale-switching-surface.
type LocaleChoice struct {
	Locale Locale
	// Label is the display name written in that locale itself, declared in the
	// project's i18n block. It falls back to the tag, which is visibly wrong in
	// a switcher rather than silently wrong.
	Label string
	// URL is this same page in that locale, empty in a mode with no per-locale
	// URL.
	URL     string
	Current bool
}

var localeRouting struct {
	mu            sync.RWMutex
	routes        []LocaleRoute
	labels        map[string]string
	prefixDefault bool
	registered    bool
}

// RegisterLocaleRouting records the per-prefix modes, the display labels, and
// whether the default locale carries a path prefix.
//
// It is called by the generated message package beside RegisterLocales, because
// all of it is build configuration and none of it is knowable at run time.
func RegisterLocaleRouting(routes []LocaleRoute, labels map[string]string, prefixDefault bool) {
	localeRouting.mu.Lock()
	defer localeRouting.mu.Unlock()
	sorted := append([]LocaleRoute(nil), routes...)
	// Longest first, so a lookup takes the first match and a nested prefix wins
	// over the root the way every other prefix policy resolves.
	sort.SliceStable(sorted, func(i, j int) bool { return len(sorted[i].Prefix) > len(sorted[j].Prefix) })
	localeRouting.routes = sorted
	localeRouting.labels = labels
	localeRouting.prefixDefault = prefixDefault
	localeRouting.registered = true
}

// localeModeFor reports the declared mode of a path, and whether one was
// declared at all.
func localeModeFor(path string) (LocaleMode, bool) {
	localeRouting.mu.RLock()
	defer localeRouting.mu.RUnlock()
	for _, route := range localeRouting.routes {
		if strings.HasPrefix(path, route.Prefix) {
			return route.Mode, true
		}
	}
	return LocaleModeHeader, false
}

func localeLabel(tag string) string {
	localeRouting.mu.RLock()
	defer localeRouting.mu.RUnlock()
	if label := localeRouting.labels[tag]; label != "" {
		return label
	}
	return tag
}

func prefixesDefault() bool {
	localeRouting.mu.RLock()
	defer localeRouting.mu.RUnlock()
	return localeRouting.prefixDefault
}

// LocaleCookieName is the cookie a reader's explicit choice is stored in.
//
// It is separate from the preference cookie on purpose:
// requirement:user-preference-rendering forbids that cookie deciding text
// content, and language is content.
const LocaleCookieName = "pw_lang"

// LocaleCookieMaxAge is how long a recorded choice lives. A year, because the
// choice is a preference rather than a session fact and re-asking a returning
// reader every month is the behaviour the cookie exists to remove.
const LocaleCookieMaxAge = 365 * 24 * 60 * 60

// StripLocalePrefix removes a leading locale segment from a path and reports the
// locale it named.
//
// The prefix position is always read as a locale, so an undeclared tag there is
// not reinterpreted as an ordinary path segment: a route literally named /de/
// would otherwise break on the day German is added.
func StripLocalePrefix(path string) (string, Locale, bool) {
	if !strings.HasPrefix(path, "/") {
		return path, Locale{}, false
	}
	rest := path[1:]
	end := strings.IndexByte(rest, '/')
	segment := rest
	remainder := "/"
	if end >= 0 {
		segment = rest[:end]
		remainder = rest[end:]
	}
	if segment == "" {
		return path, Locale{}, false
	}
	locale, ok := ParseLocale(segment)
	if !ok || locale.Tag() != segment {
		// Only an exact tag is a prefix. A lookup match would make /ja-JP/x and
		// /ja/x two URLs for one representation, which is the duplication the
		// path mode exists to avoid.
		return path, Locale{}, false
	}
	return remainder, locale, true
}

// LocalePath builds the URL of a path in a locale.
//
// It is the Go-side counterpart of the template binding: a redirect Location, a
// mail body, or a push deep link is composed here rather than in markup. The
// empty-prefix case is handled the same way, so a caller never branches on the
// mode.
func LocalePath(locale Locale, mode LocaleMode, path string) string {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	if mode != LocaleModePath || !locale.Valid() {
		return path
	}
	if !prefixesDefault() && locale.Tag() == DefaultLocale().Tag() {
		return path
	}
	if path == "/" {
		return "/" + locale.Tag()
	}
	return "/" + locale.Tag() + path
}

// SetLocale records a reader's explicit choice.
//
// The value is validated against the declared set before it is written, because
// a cookie is client-writable under the plain mode of
// policy:cookie-value-protection and a decoded value is request input.
func SetLocale(w http.ResponseWriter, locale Locale) {
	if !locale.Valid() {
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     LocaleCookieName,
		Value:    locale.Tag(),
		Path:     "/",
		HttpOnly: false,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   LocaleCookieMaxAge,
	})
}

// LocaleChoices reports the locales this page is available in.
//
// A path-mode page carries a URL per locale, which is the same data the
// alternate links are built from. A cookie-mode page carries none, because
// switching there is a server action writing the cookie rather than a link.
func LocaleChoices(r *http.Request) []LocaleChoice {
	ctx := r.Context()
	return LocaleChoicesFor(r.URL.Path, r.URL.RawQuery, LocaleContext(ctx), LocaleModeContext(ctx))
}

// LocaleChoicesFor is LocaleChoices over the values a request carries rather
// than over a request.
//
// The switcher is identical on both transports, so the computation lives here
// and each transport's entry supplies its own path, query, and resolved locale.
// Duplicating it per transport is how the two would drift.
func LocaleChoicesFor(path, rawQuery string, current Locale, mode LocaleMode) []LocaleChoice {
	snapshot := localeSet.snapshot.Load()
	if snapshot == nil {
		return nil
	}
	stripped := path
	if mode == LocaleModePath {
		if rest, _, ok := StripLocalePrefix(path); ok {
			stripped = rest
		}
	}
	choices := make([]LocaleChoice, 0, len(snapshot.tags))
	for i, tag := range snapshot.tags {
		// A declared tag resolves to itself, so the locale is built at its
		// position rather than parsed back out of the set it came from.
		locale := Locale{tag: tag, index: i + 1}
		choice := LocaleChoice{
			Locale:  locale,
			Label:   localeLabel(tag),
			Current: locale.Tag() == current.Tag(),
		}
		if mode == LocaleModePath {
			choice.URL = LocalePath(locale, mode, stripped)
			if rawQuery != "" {
				choice.URL += "?" + rawQuery
			}
		}
		choices = append(choices, choice)
	}
	return choices
}

// negotiateAcceptLanguage picks the best declared locale for an Accept-Language
// header. It reports absence rather than the default, so the caller decides.
//
// The winner is the first entry, in header order, of those with the highest
// quality that resolves against the declared set — the answer collecting,
// sorting, and scanning would give, found in one pass with no slice and no
// sort. The pass keeps both hard caps: a hostile "a,a,a,..." header carries
// ~500k ranges under Go's 1 MiB header limit, and thirty-two well-formed
// ranges is far past what any real client sends, while bounding the parts
// examined stops an all-invalid header from scanning the whole megabyte.
func negotiateAcceptLanguage(header string) (Locale, bool) {
	const maxRanges = 32
	const maxPartsExamined = 256
	var best Locale
	bestQuality := 0.0
	found := false
	ranges := 0
	remainder := header
	for examined := 0; remainder != "" && examined < maxPartsExamined && ranges < maxRanges; examined++ {
		var part string
		part, remainder, _ = strings.Cut(remainder, ",")
		tag := strings.TrimSpace(part)
		quality := 1.0
		if semicolon := strings.IndexByte(tag, ';'); semicolon >= 0 {
			parameters := tag[semicolon+1:]
			tag = strings.TrimSpace(tag[:semicolon])
			for parameter := range splitAccept(parameters, ';') {
				if value, ok := strings.CutPrefix(strings.TrimSpace(parameter), "q="); ok {
					if parsed, err := strconv.ParseFloat(value, 64); err == nil {
						quality = parsed
					}
				}
			}
		}
		if tag == "" || quality <= 0 {
			continue
		}
		ranges++
		// An earlier entry wins a quality tie, so only a strictly better
		// quality displaces the held answer.
		if quality <= bestQuality {
			continue
		}
		var locale Locale
		if tag == "*" {
			locale = DefaultLocale()
		} else if parsed, ok := ParseLocale(tag); ok {
			locale = parsed
		} else {
			continue
		}
		best, bestQuality, found = locale, quality, true
	}
	return best, found
}
