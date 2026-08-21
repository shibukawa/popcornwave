package pwruntime

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func withRouting(t *testing.T, tags []string, def string, routes []LocaleRoute, prefixDefault bool) {
	t.Helper()
	withLocales(t, tags, def)
	RegisterLocaleRouting(routes, map[string]string{"ja": "日本語", "en": "English"}, prefixDefault)
	t.Cleanup(func() { RegisterLocaleRouting(nil, nil, true) })
}

func serve(t *testing.T, request *http.Request) (*httptest.ResponseRecorder, Locale, LocaleMode) {
	t.Helper()
	recorder := httptest.NewRecorder()
	var locale Locale
	var mode LocaleMode
	LocaleMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		locale, mode = LocaleContext(r.Context()), LocaleModeContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(recorder, request)
	return recorder, locale, mode
}

func TestPathModeStripsThePrefixAndVariesOnNothing(t *testing.T) {
	withRouting(t, []string{"ja", "en"}, "ja", []LocaleRoute{{Prefix: "/", Mode: LocaleModePath}}, true)

	request := httptest.NewRequest(http.MethodGet, "/en/about", nil)
	var seenPath string
	recorder := httptest.NewRecorder()
	LocaleMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.Path
		if got := LocaleContext(r.Context()).Tag(); got != "en" {
			t.Errorf("locale = %q, want en", got)
		}
	})).ServeHTTP(recorder, request)

	if seenPath != "/about" {
		t.Errorf("handler saw %q, want the prefix stripped before routing", seenPath)
	}
	// Two languages are two URLs, so nothing varies. This is the whole reason
	// public content uses this mode.
	if vary := recorder.Header().Values("Vary"); len(vary) != 0 {
		t.Errorf("Vary = %v, want none in path mode", vary)
	}
	if got := recorder.Header().Get("Content-Language"); got != "en" {
		t.Errorf("Content-Language = %q", got)
	}
}

func TestPathModeRedirectsAnUnprefixedPath(t *testing.T) {
	withRouting(t, []string{"ja", "en"}, "ja", []LocaleRoute{{Prefix: "/", Mode: LocaleModePath}}, true)

	request := httptest.NewRequest(http.MethodGet, "/about?x=1", nil)
	request.Header.Set("Accept-Language", "en-US,en;q=0.9")
	recorder, _, _ := serve(t, request)

	// Temporary, because the target depends on the request: a permanent status
	// would cache one reader's negotiation for the next.
	if recorder.Code != http.StatusFound {
		t.Errorf("status = %d, want 302", recorder.Code)
	}
	if got := recorder.Header().Get("Location"); got != "/en/about?x=1" {
		t.Errorf("Location = %q", got)
	}
	if !strings.Contains(strings.Join(recorder.Header().Values("Vary"), ","), "Accept-Language") {
		t.Errorf("a negotiated redirect must vary on what it negotiated from: %v", recorder.Header().Values("Vary"))
	}
}

// An undeclared tag in the prefix position is not reinterpreted as a path
// segment, or a route literally named /de/ would break the day German is added.
func TestUndeclaredPrefixIsNotAPathSegment(t *testing.T) {
	withRouting(t, []string{"ja", "en"}, "ja", []LocaleRoute{{Prefix: "/", Mode: LocaleModePath}}, true)

	recorder, _, _ := serve(t, httptest.NewRequest(http.MethodGet, "/de/about", nil))
	if recorder.Code != http.StatusFound {
		t.Fatalf("status = %d, want a negotiation redirect", recorder.Code)
	}
	if got := recorder.Header().Get("Location"); got != "/ja/de/about" {
		t.Errorf("Location = %q; /de/ is an ordinary path here, not a locale", got)
	}
}

func TestPrefixDefaultFalseServesTheRootAndRedirectsThePrefixedForm(t *testing.T) {
	withRouting(t, []string{"ja", "en"}, "ja", []LocaleRoute{{Prefix: "/", Mode: LocaleModePath}}, false)

	_, locale, _ := serve(t, httptest.NewRequest(http.MethodGet, "/about", nil))
	if locale.Tag() != "ja" {
		t.Errorf("unprefixed path served %q, want the default locale", locale.Tag())
	}

	recorder, _, _ := serve(t, httptest.NewRequest(http.MethodGet, "/ja/about", nil))
	if recorder.Code != http.StatusMovedPermanently {
		t.Errorf("status = %d, want 301 so one representation has one URL", recorder.Code)
	}
	if got := recorder.Header().Get("Location"); got != "/about" {
		t.Errorf("Location = %q", got)
	}
}

// The read-driven Vary rule does not transfer to language: a reader with no
// cookie must still get a varying response, or a shared cache stores the
// default and serves it to a reader whose cookie says otherwise.
func TestCookieModeVariesEvenWithNoCookiePresent(t *testing.T) {
	withRouting(t, []string{"ja", "en"}, "ja", []LocaleRoute{{Prefix: "/", Mode: LocaleModeCookie}}, true)

	recorder, locale, _ := serve(t, httptest.NewRequest(http.MethodGet, "/admin/", nil))
	if locale.Tag() != "ja" {
		t.Errorf("locale = %q, want the default", locale.Tag())
	}
	vary := strings.Join(recorder.Header().Values("Vary"), ",")
	for _, axis := range []string{"Cookie", "Accept-Language"} {
		if !strings.Contains(vary, axis) {
			t.Errorf("Vary = %q, want it to include %s", vary, axis)
		}
	}
}

func TestCookieOutranksTheHeader(t *testing.T) {
	withRouting(t, []string{"ja", "en"}, "ja", []LocaleRoute{{Prefix: "/", Mode: LocaleModeCookie}}, true)

	request := httptest.NewRequest(http.MethodGet, "/admin/", nil)
	request.Header.Set("Accept-Language", "en")
	request.AddCookie(&http.Cookie{Name: LocaleCookieName, Value: "ja"})
	_, locale, _ := serve(t, request)
	if locale.Tag() != "ja" {
		t.Errorf("locale = %q; a stored choice outranks an environment report", locale.Tag())
	}
}

// An API reads the header only. A body that changed with cookie state would be
// answering the same request two ways for reasons the client never stated.
func TestHeaderModeIgnoresTheCookie(t *testing.T) {
	withRouting(t, []string{"ja", "en"}, "ja", []LocaleRoute{{Prefix: "/api/", Mode: LocaleModeHeader}}, true)

	request := httptest.NewRequest(http.MethodGet, "/api/items", nil)
	request.Header.Set("Accept-Language", "en")
	request.AddCookie(&http.Cookie{Name: LocaleCookieName, Value: "ja"})
	recorder, locale, _ := serve(t, request)

	if locale.Tag() != "en" {
		t.Errorf("locale = %q, want the header's answer", locale.Tag())
	}
	if vary := strings.Join(recorder.Header().Values("Vary"), ","); strings.Contains(vary, "Cookie") {
		t.Errorf("Vary = %q; a header-mode route reads no cookie", vary)
	}
}

func TestLongestPrefixWins(t *testing.T) {
	withRouting(t, []string{"ja", "en"}, "ja", []LocaleRoute{
		{Prefix: "/", Mode: LocaleModePath},
		{Prefix: "/api/", Mode: LocaleModeHeader},
	}, true)

	_, _, mode := serve(t, httptest.NewRequest(http.MethodGet, "/api/items", nil))
	if mode != LocaleModeHeader {
		t.Errorf("mode = %v, want the nested prefix to win", mode)
	}
}

func TestAcceptLanguageQualityOrdering(t *testing.T) {
	withLocales(t, []string{"ja", "en"}, "ja")

	locale, ok := negotiateAcceptLanguage("de;q=1.0, en;q=0.8, ja;q=0.9")
	if !ok || locale.Tag() != "ja" {
		t.Errorf("negotiated %q, %v; want ja as the highest declared match", locale.Tag(), ok)
	}
	if _, ok := negotiateAcceptLanguage("de, fr"); ok {
		t.Error("no declared match should report absence rather than the default")
	}
}

// A hostile Accept-Language header packed with ranges must not build and sort an
// unbounded candidate slice: the scan is capped, and a real preferred language
// within the cap is still negotiated correctly.
func TestAcceptLanguageIsBoundedAgainstAmplification(t *testing.T) {
	withLocales(t, []string{"ja", "en"}, "ja")

	// ~200k ranges under a realistic header size. The cap must keep this cheap;
	// the test simply completing rather than exploding is the assertion.
	flood := strings.Repeat("en;q=0.1,", 200_000) + "ja"
	locale, ok := negotiateAcceptLanguage(flood)
	// The real preferred tag sits past the cap here, so the bounded scan settles
	// on the best range it examined rather than reading the whole megabyte.
	if !ok {
		t.Fatal("a bounded scan reported no match at all")
	}
	if locale.Tag() != "en" && locale.Tag() != "ja" {
		t.Errorf("negotiated %q, want a declared locale", locale.Tag())
	}

	// A normal header with the preferred tag early is negotiated exactly.
	if locale, ok := negotiateAcceptLanguage("en;q=0.4, ja;q=0.9"); !ok || locale.Tag() != "ja" {
		t.Errorf("negotiated %q, %v; want ja", locale.Tag(), ok)
	}
}

func TestLocaleChoicesFeedTheSwitcherAndTheAlternates(t *testing.T) {
	withRouting(t, []string{"ja", "en"}, "ja", []LocaleRoute{{Prefix: "/", Mode: LocaleModePath}}, true)

	request := httptest.NewRequest(http.MethodGet, "/en/about", nil)
	request = request.WithContext(WithLocaleMode(WithLocale(request.Context(), MustParseLocale("en")), LocaleModePath))

	choices := LocaleChoices(request)
	if len(choices) != 2 {
		t.Fatalf("choices = %d, want one per declared locale", len(choices))
	}
	byTag := map[string]LocaleChoice{}
	for _, choice := range choices {
		byTag[choice.Locale.Tag()] = choice
	}
	if got := byTag["ja"].URL; got != "/ja/about" {
		t.Errorf("ja URL = %q, want the same page with the prefix swapped", got)
	}
	if got := byTag["ja"].Label; got != "日本語" {
		t.Errorf("label = %q, want the name written in that language", got)
	}
	if !byTag["en"].Current {
		t.Error("the active locale should be marked current")
	}

	links := LocaleAlternateLinks(request, "https://example.com")
	for _, want := range []string{`hreflang="ja" href="https://example.com/ja/about"`, `hreflang="x-default"`} {
		if !strings.Contains(links, want) {
			t.Errorf("alternates missing %q:\n%s", want, links)
		}
	}
}

// A negotiated mode has no per-locale URL, so it produces no alternates rather
// than pointing every language at one address.
func TestNegotiatedModeProducesNoAlternates(t *testing.T) {
	withRouting(t, []string{"ja", "en"}, "ja", []LocaleRoute{{Prefix: "/", Mode: LocaleModeCookie}}, true)

	request := httptest.NewRequest(http.MethodGet, "/admin/", nil)
	request = request.WithContext(WithLocaleMode(request.Context(), LocaleModeCookie))
	if links := LocaleAlternateLinks(request, "https://example.com"); links != "" {
		t.Errorf("alternates = %q, want none", links)
	}
}

func TestLocalePathHandlesEveryMode(t *testing.T) {
	withRouting(t, []string{"ja", "en"}, "ja", nil, true)
	en := MustParseLocale("en")

	cases := []struct {
		mode LocaleMode
		path string
		want string
	}{
		{LocaleModePath, "/about", "/en/about"},
		{LocaleModePath, "/", "/en"},
		{LocaleModeCookie, "/about", "/about"},
		{LocaleModeHeader, "/about", "/about"},
	}
	for _, tc := range cases {
		if got := LocalePath(en, tc.mode, tc.path); got != tc.want {
			t.Errorf("LocalePath(%v, %q) = %q, want %q", tc.mode, tc.path, got, tc.want)
		}
	}

	withRouting(t, []string{"ja", "en"}, "ja", nil, false)
	if got := LocalePath(MustParseLocale("ja"), LocaleModePath, "/about"); got != "/about" {
		t.Errorf("the default locale carries no prefix under prefix_default false, got %q", got)
	}
}

func TestSetLocaleWritesOnlyADeclaredTag(t *testing.T) {
	withLocales(t, []string{"ja", "en"}, "ja")

	recorder := httptest.NewRecorder()
	SetLocale(recorder, MustParseLocale("en"))
	if got := recorder.Header().Get("Set-Cookie"); !strings.Contains(got, LocaleCookieName+"=en") {
		t.Errorf("Set-Cookie = %q", got)
	}

	recorder = httptest.NewRecorder()
	SetLocale(recorder, Locale{})
	if got := recorder.Header().Get("Set-Cookie"); got != "" {
		t.Errorf("an invalid locale should write nothing, got %q", got)
	}
}

func TestProjectWithoutLocalesPassesThrough(t *testing.T) {
	resetLocalesForTest()
	t.Cleanup(resetLocalesForTest)

	called := false
	recorder := httptest.NewRecorder()
	LocaleMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.URL.Path != "/en/about" {
			t.Errorf("path = %q, want it untouched", r.URL.Path)
		}
	})).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/en/about", nil))

	if !called {
		t.Fatal("the handler should still run")
	}
	if len(recorder.Header().Values("Vary")) != 0 {
		t.Error("a project with no locales should carry no locale Vary")
	}
}
