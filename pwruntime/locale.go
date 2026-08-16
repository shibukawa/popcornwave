package pwruntime

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// Locale identifies one locale the project declared. It is opaque because the
// value that matters is the dense index the generated message tables are
// addressed by; the tag is carried so a document, a URL, and a Content-Language
// header can name it.
//
// A zero Locale is the explicit "no locale resolved" value. It is never a
// silent stand-in for the default, per policy:absent-rather-than-stubbed.
//
// See .knowledge api:locale-accessors.
type Locale struct {
	tag string
	// index is one more than the position in the registered list, so the zero
	// value is distinguishable from the first declared locale.
	index int
}

// Tag returns the BCP 47 tag, empty for the zero value.
func (l Locale) Tag() string { return l.tag }

// Index returns the position of this locale in the declared list. It is the
// subscript of every generated message table.
//
// The zero Locale reports -1 rather than 0, so indexing with an unresolved
// locale panics at the read instead of silently serving the first declared
// language.
func (l Locale) Index() int { return l.index - 1 }

// Valid reports whether this value names a declared locale.
func (l Locale) Valid() bool { return l.index > 0 }

// String makes a Locale printable in a log or a test failure without reaching
// for Tag, and names the zero value rather than rendering as empty.
func (l Locale) String() string {
	if !l.Valid() {
		return "locale(none)"
	}
	return l.tag
}

// NewLocale builds a Locale for a declared tag at a known position.
//
// It exists for generated code, which holds the tables this index addresses and
// therefore already knows every position. Handwritten code has ParseLocale,
// which validates against the registered set; this one does not, because a
// generated constant is initialized before init functions run and so before any
// registration could have happened.
//
// index is the zero-based position in the declared list.
func NewLocale(tag string, index int) Locale { return Locale{tag: tag, index: index + 1} }

// localeSet is the declared locale list, registered once by the generated
// message package.
//
// It is package state because the declared set is a property of the build
// rather than of a request: the segment tables of decision:message-code-shape
// are generated against exactly this list, so a second set would index tables
// that were never built for it.
var localeSet struct {
	mu         sync.RWMutex
	tags       []string
	index      map[string]int
	defaultTag string
	registered bool
}

// RegisterLocales records the declared locale list. The generated message
// package calls it from an init function, because that package is the one
// holding the tables this list indexes.
//
// tags is in declaration order and defaultTag must be a member. Registering a
// second, different set panics: the tables were generated against one list, and
// continuing with another would read them at subscripts they were never built
// for. Registering the identical set again is a no-op, so a test binary linking
// the generated package twice is not a failure.
func RegisterLocales(tags []string, defaultTag string) {
	localeSet.mu.Lock()
	defer localeSet.mu.Unlock()
	if localeSet.registered {
		if sameTags(localeSet.tags, tags) && localeSet.defaultTag == defaultTag {
			return
		}
		panic(fmt.Sprintf("pwruntime: locales already registered as %v with default %q; a second set %v with default %q would index message tables built for the first",
			localeSet.tags, localeSet.defaultTag, tags, defaultTag))
	}
	if len(tags) == 0 {
		panic("pwruntime: RegisterLocales needs at least one locale")
	}
	index := make(map[string]int, len(tags))
	for i, tag := range tags {
		normalized := strings.ToLower(tag)
		if _, duplicate := index[normalized]; duplicate {
			panic(fmt.Sprintf("pwruntime: locale %q declared twice", tag))
		}
		index[normalized] = i + 1
	}
	if _, ok := index[strings.ToLower(defaultTag)]; !ok {
		panic(fmt.Sprintf("pwruntime: default locale %q is not in the declared set %v", defaultTag, tags))
	}
	localeSet.tags = append([]string(nil), tags...)
	localeSet.index = index
	localeSet.defaultTag = defaultTag
	localeSet.registered = true
}

func sameTags(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// DeclaredLocales returns the declared tags in declaration order. It reports nil
// when no generated message package is linked, which is every project that has
// not adopted requirement:application-i18n.
func DeclaredLocales() []string {
	localeSet.mu.RLock()
	defer localeSet.mu.RUnlock()
	return append([]string(nil), localeSet.tags...)
}

// DefaultLocale returns the declared default. It reports the zero Locale when no
// message package is linked.
func DefaultLocale() Locale {
	localeSet.mu.RLock()
	defer localeSet.mu.RUnlock()
	if !localeSet.registered {
		return Locale{}
	}
	return Locale{tag: localeSet.defaultTag, index: localeSet.index[strings.ToLower(localeSet.defaultTag)]}
}

// ParseLocale resolves a tag against the declared set by RFC 4647 lookup: an
// exact match wins, then progressively shorter prefixes, so ja-JP finds ja.
//
// It reports absence rather than substituting the default, per
// policy:absent-rather-than-stubbed. A caller that wants the default asks for it.
//
// This is the entry point for a locale that did not come from a request — a
// value read from a user record before sending mail, or from a job payload.
func ParseLocale(tag string) (Locale, bool) {
	localeSet.mu.RLock()
	defer localeSet.mu.RUnlock()
	if !localeSet.registered {
		return Locale{}, false
	}
	candidate := strings.ToLower(strings.TrimSpace(tag))
	for candidate != "" {
		if i, ok := localeSet.index[candidate]; ok {
			return Locale{tag: localeSet.tags[i-1], index: i}, true
		}
		cut := strings.LastIndexByte(candidate, '-')
		if cut < 0 {
			break
		}
		candidate = candidate[:cut]
	}
	return Locale{}, false
}

// MustParseLocale is ParseLocale for a tag the caller knows is declared, such as
// a constant in generated code. It panics on an undeclared tag.
func MustParseLocale(tag string) Locale {
	locale, ok := ParseLocale(tag)
	if !ok {
		panic(fmt.Sprintf("pwruntime: locale %q is not declared", tag))
	}
	return locale
}

type localeContextKey struct{}

// WithLocale pins a resolved locale onto ctx. Locale resolution runs once per
// request, before the first byte, because policy:locale-vary-correctness needs
// the answer while headers are still open and flow:initial-streaming-render
// closes them before the body renders.
func WithLocale(ctx context.Context, locale Locale) context.Context {
	return context.WithValue(ctx, localeContextKey{}, locale)
}

// LocaleContext returns the locale resolved for this request.
//
// Unlike ParseLocale it always answers: a request that reached a route with no
// declared mode, or a context with no resolution at all, reports the declared
// default. A caller rendering text has no useful branch on absence, and the
// alternative is every message call site handling a case the router already
// decided.
func LocaleContext(ctx context.Context) Locale {
	if locale, ok := ctx.Value(localeContextKey{}).(Locale); ok && locale.Valid() {
		return locale
	}
	return DefaultLocale()
}

// LocaleMode is how a route decides its locale, declared per path prefix in the
// project's i18n block. It is carried on the request because it decides two
// things a handler cannot re-derive: whether a link carries a locale segment,
// and what the response varies on.
//
// See .knowledge decision:locale-url-modes.
type LocaleMode uint8

const (
	// LocaleModePath reads the locale from a URL path prefix. Two languages are
	// two URLs, so the response varies on nothing.
	LocaleModePath LocaleMode = iota
	// LocaleModeCookie reads a stored reader choice, then Accept-Language.
	LocaleModeCookie
	// LocaleModeHeader reads Accept-Language only, which is the mode an API
	// answering a native client uses.
	LocaleModeHeader
)

type localeModeContextKey struct{}

// WithLocaleMode records the mode the matched route declared.
func WithLocaleMode(ctx context.Context, mode LocaleMode) context.Context {
	return context.WithValue(ctx, localeModeContextKey{}, mode)
}

// LocaleModeContext reports the mode of the matched route.
//
// A request that reached no declared route reports LocaleModeHeader, which is
// the mode that puts nothing in a URL. That is the safe default: emitting a
// locale segment for a route that has no prefixed form produces links that 404,
// while omitting one produces links that work and merely do not carry the
// language.
func LocaleModeContext(ctx context.Context) LocaleMode {
	if mode, ok := ctx.Value(localeModeContextKey{}).(LocaleMode); ok {
		return mode
	}
	return LocaleModeHeader
}

// MessageLocale is the provider behind the implicit binding supplying every
// generated message symbol's leading argument.
//
// It returns Locale rather than a tag so the generated catalog takes the type it
// declares. A typed binding cannot be written into markup, which is what keeps
// this value out of the positions LangTag and LangSegment serve.
//
// See .knowledge data:locale-bindings.
func MessageLocale(ctx context.Context) Locale { return LocaleContext(ctx) }

// LangTag is the provider behind the ordinary string binding: the resolved tag,
// never empty, in every mode. It is what a document language attribute and a
// localized asset path are written with.
func LangTag(ctx context.Context) string { return LocaleContext(ctx).Tag() }

// LangSegment is the provider behind the path-segment binding: the tag under
// LocaleModePath and the empty string otherwise.
//
// An empty value collapses the separator before it where it is written into a
// URL attribute, so one template serves every mode: "/{lang}/about" is
// /ja/about where the locale is in the path and /about where it is not.
func LangSegment(ctx context.Context) string {
	if LocaleModeContext(ctx) != LocaleModePath {
		return ""
	}
	return LocaleContext(ctx).Tag()
}

// resetLocalesForTest restores the unregistered state. It exists because
// RegisterLocales is deliberately once-only, and a test exercising several
// declared sets cannot otherwise run in one binary.
func resetLocalesForTest() {
	localeSet.mu.Lock()
	defer localeSet.mu.Unlock()
	localeSet.tags = nil
	localeSet.index = nil
	localeSet.defaultTag = ""
	localeSet.registered = false
}
