package pwruntime

import (
	"context"
	"testing"
)

func withLocales(t *testing.T, tags []string, def string) {
	t.Helper()
	resetLocalesForTest()
	t.Cleanup(resetLocalesForTest)
	RegisterLocales(tags, def)
}

// The zero Locale must be distinguishable from the first declared one, or an
// unresolved locale silently serves whichever language happens to be listed
// first.
func TestZeroLocaleIsNotTheFirstDeclaredOne(t *testing.T) {
	withLocales(t, []string{"ja", "en"}, "ja")

	var zero Locale
	if zero.Valid() {
		t.Error("the zero Locale should not be valid")
	}
	if zero.Index() != -1 {
		t.Errorf("zero index = %d, want -1 so a table read panics instead of serving the first locale", zero.Index())
	}
	if zero.String() != "locale(none)" {
		t.Errorf("zero String() = %q", zero.String())
	}

	first, ok := ParseLocale("ja")
	if !ok || first.Index() != 0 {
		t.Fatalf("ParseLocale(ja) = %v, %v; want index 0", first, ok)
	}
}

func TestParseLocaleFallsBackThroughSubtags(t *testing.T) {
	withLocales(t, []string{"ja", "en", "pt-BR"}, "ja")

	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"ja", "ja", true},
		{"JA", "ja", true},
		{"ja-JP", "ja", true},
		{"en-US-posix", "en", true},
		{"pt-BR", "pt-BR", true},
		// pt alone is not declared, and pt-BR is more specific than the tag
		// asked for, so lookup reports absence rather than widening.
		{"pt", "", false},
		{"de", "", false},
		{"", "", false},
	}
	for _, tc := range cases {
		got, ok := ParseLocale(tc.in)
		if ok != tc.ok || got.Tag() != tc.want {
			t.Errorf("ParseLocale(%q) = %q, %v; want %q, %v", tc.in, got.Tag(), ok, tc.want, tc.ok)
		}
	}
}

// An unresolved tag must not become the default silently: a caller reading a
// stored preference needs to know the stored value is no longer declared.
func TestParseLocaleReportsAbsenceRatherThanDefaulting(t *testing.T) {
	withLocales(t, []string{"ja", "en"}, "ja")

	if got, ok := ParseLocale("de"); ok || got.Valid() {
		t.Errorf("ParseLocale(de) = %v, %v; want the zero value and false", got, ok)
	}
}

// LocaleContext is the opposite case: a request always has an answer, because
// the router resolved one before the handler ran.
func TestLocaleContextAnswersWithTheDefault(t *testing.T) {
	withLocales(t, []string{"ja", "en"}, "ja")

	if got := LocaleContext(context.Background()); got.Tag() != "ja" {
		t.Errorf("LocaleContext with no resolution = %q, want the default", got.Tag())
	}
	en := MustParseLocale("en")
	if got := LocaleContext(WithLocale(context.Background(), en)); got.Tag() != "en" {
		t.Errorf("LocaleContext = %q, want en", got.Tag())
	}
}

func TestUnregisteredProjectHasNoLocales(t *testing.T) {
	resetLocalesForTest()
	t.Cleanup(resetLocalesForTest)

	if got := DeclaredLocales(); len(got) != 0 {
		t.Errorf("DeclaredLocales = %v, want none", got)
	}
	if got := DefaultLocale(); got.Valid() {
		t.Errorf("DefaultLocale = %v, want the zero value", got)
	}
	if _, ok := ParseLocale("ja"); ok {
		t.Error("ParseLocale should report absence with no registration")
	}
}

// Registering a second, different set would index tables generated for the
// first, so it is a panic rather than a silent replacement.
func TestConflictingRegistrationPanics(t *testing.T) {
	withLocales(t, []string{"ja", "en"}, "ja")

	defer func() {
		if recover() == nil {
			t.Error("a conflicting registration should panic")
		}
	}()
	RegisterLocales([]string{"ja", "fr"}, "ja")
}

// The identical set arriving twice is a linked-twice binary, not a conflict.
func TestIdenticalRegistrationIsIdempotent(t *testing.T) {
	withLocales(t, []string{"ja", "en"}, "ja")
	RegisterLocales([]string{"ja", "en"}, "ja")

	if got := DeclaredLocales(); len(got) != 2 {
		t.Errorf("DeclaredLocales = %v", got)
	}
}

func TestDefaultOutsideTheDeclaredSetPanics(t *testing.T) {
	resetLocalesForTest()
	t.Cleanup(resetLocalesForTest)

	defer func() {
		if recover() == nil {
			t.Error("a default outside the set should panic")
		}
	}()
	RegisterLocales([]string{"ja", "en"}, "fr")
}
