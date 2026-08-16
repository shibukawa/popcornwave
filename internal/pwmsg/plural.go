// Package pwmsg reads message catalogs and generates the typed Go package a
// template's message reference resolves against.
//
// It is a host-side tool: it runs during pw generate and never in a served
// process. See .knowledge decision:message-code-shape.
package pwmsg

import "fmt"

// Category is a CLDR plural category. The set is closed and ordered so a
// generated table row can be addressed by position.
type Category string

const (
	Zero  Category = "zero"
	One   Category = "one"
	Two   Category = "two"
	Few   Category = "few"
	Many  Category = "many"
	Other Category = "other"
)

// categoryOrder is the order a locale's declared variants are written into a
// generated row. It is CLDR's own order, so a reviewer reading a table row and
// a reviewer reading the specification see the same sequence.
var categoryOrder = []Category{Zero, One, Two, Few, Many, Other}

// PluralRule is the cardinal plural behaviour of one locale: which categories it
// distinguishes, and the Go expression selecting one for a value.
//
// Only declared locales are generated, so a single-locale Japanese project emits
// a selector that is a constant. See .knowledge decision:message-code-shape.
type PluralRule struct {
	// Categories are the ones this locale uses, in categoryOrder.
	Categories []Category
	// Selector is Go statements over the parameter n returning the position of
	// the matching category within Categories.
	//
	// It is statements rather than an expression because Go has no conditional
	// expression, and the expression forms that imitate one — a map literal
	// indexed by a bool, a slice indexed by a converted bool — allocate or
	// bounds-check on a path every rendered message crosses.
	//
	// Empty means the locale has one category, where generation writes a
	// constant instead.
	Selector string
}

// pluralRules holds the cardinal rules this build can state correctly.
//
// It is deliberately a table of what is known rather than a general CLDR
// evaluator. Shipping the full rule set means shipping the data that drives it,
// which is the same weight decision:message-id-assignment refused for slugs; and
// a locale whose rule is guessed produces a page that is wrong in a way no test
// written in another language will notice.
//
// A locale absent here is usable — see UnknownPluralLocale — as long as its
// catalog declares no plural variation.
var pluralRules = map[string]PluralRule{
	// One category: the language marks no plural agreement on the noun.
	"ja": onlyOther, "zh": onlyOther, "ko": onlyOther, "th": onlyOther,
	"vi": onlyOther, "id": onlyOther, "ms": onlyOther, "my": onlyOther,
	"km": onlyOther, "lo": onlyOther, "tr": onlyOther,

	// one when the value is exactly 1.
	"en": oneIsSingular, "de": oneIsSingular, "nl": oneIsSingular,
	"sv": oneIsSingular, "da": oneIsSingular, "nb": oneIsSingular,
	"no": oneIsSingular, "fi": oneIsSingular, "et": oneIsSingular,
	"it": oneIsSingular, "es": oneIsSingular, "el": oneIsSingular,
	"he": oneIsSingular, "hu": oneIsSingular, "bg": oneIsSingular,
	"ca": oneIsSingular, "eu": oneIsSingular, "sw": oneIsSingular,

	// one when the value is 0 or 1.
	"fr": zeroOrOneIsSingular, "pt": zeroOrOneIsSingular, "hi": zeroOrOneIsSingular,

	"ru": eastSlavic, "uk": eastSlavic, "be": eastSlavic,
	"pl": polish,
	"cs": czech, "sk": czech,
	"ar": arabic,
}

var (
	onlyOther = PluralRule{Categories: []Category{Other}}

	oneIsSingular = PluralRule{
		Categories: []Category{One, Other},
		Selector:   "if n == 1 {\nreturn 0\n}\nreturn 1",
	}

	zeroOrOneIsSingular = PluralRule{
		Categories: []Category{One, Other},
		Selector:   "if n == 0 || n == 1 {\nreturn 0\n}\nreturn 1",
	}

	// eastSlavic distinguishes one, few, and many by the last digit and the
	// last two digits, which is why the teens are excluded from one and few.
	eastSlavic = PluralRule{
		Categories: []Category{One, Few, Many},
		Selector:   "return pluralEastSlavic(n)",
	}

	polish = PluralRule{
		Categories: []Category{One, Few, Many},
		Selector:   "return pluralPolish(n)",
	}

	czech = PluralRule{
		Categories: []Category{One, Few, Other},
		Selector:   "return pluralCzech(n)",
	}

	arabic = PluralRule{
		Categories: []Category{Zero, One, Two, Few, Many, Other},
		Selector:   "return pluralArabic(n)",
	}
)

// pluralHelpers are the Go functions a Selector may name. Generation emits only
// the ones its declared locales reach, so a project with no Slavic locale
// carries none of them.
var pluralHelpers = map[string]string{
	"pluralEastSlavic": `// pluralEastSlavic selects one, few, or many for Russian, Ukrainian, and
// Belarusian. The teens take many, which is why mod 100 is tested first.
func pluralEastSlavic(n int) int {
	if n < 0 {
		n = -n
	}
	switch {
	case n%10 == 1 && n%100 != 11:
		return 0
	case n%10 >= 2 && n%10 <= 4 && (n%100 < 12 || n%100 > 14):
		return 1
	default:
		return 2
	}
}`,
	"pluralPolish": `// pluralPolish selects one, few, or many.
func pluralPolish(n int) int {
	if n < 0 {
		n = -n
	}
	switch {
	case n == 1:
		return 0
	case n%10 >= 2 && n%10 <= 4 && (n%100 < 12 || n%100 > 14):
		return 1
	default:
		return 2
	}
}`,
	"pluralCzech": `// pluralCzech selects one, few, or other for Czech and Slovak.
func pluralCzech(n int) int {
	switch {
	case n == 1:
		return 0
	case n >= 2 && n <= 4:
		return 1
	default:
		return 2
	}
}`,
	"pluralArabic": `// pluralArabic selects across all six categories.
func pluralArabic(n int) int {
	if n < 0 {
		n = -n
	}
	switch {
	case n == 0:
		return 0
	case n == 1:
		return 1
	case n == 2:
		return 2
	case n%100 >= 3 && n%100 <= 10:
		return 3
	case n%100 >= 11 && n%100 <= 99:
		return 4
	default:
		return 5
	}
}`,
}

// UnknownPluralLocale is reported for a locale whose rule this build cannot
// state. It is an error only when the catalog declares plural variation for that
// locale: a locale using one form throughout needs no rule.
type UnknownPluralLocale struct {
	Tag string
}

func (e UnknownPluralLocale) Error() string {
	return fmt.Sprintf("locale %q has no plural rule in this build, so a message declaring plural variants for it cannot be generated correctly; declare only the %q variant, or add the rule to internal/pwmsg", e.Tag, Other)
}

// RuleFor returns the cardinal rule of a locale, matching the base language
// subtag so pt-BR follows pt.
//
// A locale with no known rule reports the single-category rule and false, which
// is correct for a catalog that declares no variants and is what the caller
// turns into UnknownPluralLocale when it declares some.
func RuleFor(tag string) (PluralRule, bool) {
	if rule, ok := pluralRules[baseLanguage(tag)]; ok {
		return rule, true
	}
	return onlyOther, false
}

func baseLanguage(tag string) string {
	for i := 0; i < len(tag); i++ {
		if tag[i] == '-' || tag[i] == '_' {
			return lower(tag[:i])
		}
	}
	return lower(tag)
}

func lower(s string) string {
	out := []byte(s)
	for i := range out {
		if out[i] >= 'A' && out[i] <= 'Z' {
			out[i] += 'a' - 'A'
		}
	}
	return string(out)
}
