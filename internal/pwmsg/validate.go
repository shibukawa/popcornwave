package pwmsg

import (
	"fmt"
	"sort"
	"strings"
)

// Severity separates what stops a build from what a reader should see and
// decide about.
type Severity int

const (
	// Warning is reported and does not stop generation.
	Warning Severity = iota
	// Error stops generation.
	Error
)

func (s Severity) String() string {
	if s == Error {
		return "error"
	}
	return "warning"
}

// Diagnostic is one finding against a catalog.
type Diagnostic struct {
	Severity Severity
	Path     string
	Line     int
	Message  string
}

func (d Diagnostic) String() string {
	if d.Line > 0 {
		return fmt.Sprintf("%s:%d: %s: %s", d.Path, d.Line, d.Severity, d.Message)
	}
	return fmt.Sprintf("%s: %s: %s", d.Path, d.Severity, d.Message)
}

// Validate checks a catalog against the declared locale set.
//
// missing is the severity of a locale that supplies no translation for a
// message, which data:i18n-config leaves to the project: a build that must ship
// complete fails, and a build still being translated reports and falls back.
// Every other finding here has a fixed severity, because a placeholder that does
// not match its declaration renders wrong text in a way no fallback repairs.
func Validate(catalog *Catalog, missing Severity) []Diagnostic {
	var diagnostics []Diagnostic
	for _, scope := range catalog.Scopes {
		for _, entry := range scope.Entries {
			diagnostics = append(diagnostics, validateEntry(catalog, scope, entry, missing)...)
		}
	}
	return diagnostics
}

func validateEntry(catalog *Catalog, scope Scope, entry Entry, missing Severity) []Diagnostic {
	var out []Diagnostic
	report := func(severity Severity, format string, args ...any) {
		out = append(out, Diagnostic{
			Severity: severity,
			Path:     scope.Path,
			Line:     entry.Line,
			Message:  fmt.Sprintf("message %q: ", entry.Qualified(scope.Name)) + fmt.Sprintf(format, args...),
		})
	}

	declared := map[string]bool{}
	for _, param := range entry.Params {
		declared[param.Name] = true
	}
	if entry.Plural != "" {
		pluralType := ""
		for _, param := range entry.Params {
			if param.Name == entry.Plural {
				pluralType = param.Type
			}
		}
		switch {
		case pluralType == "":
			report(Error, "plural is driven by %q, which is not a declared parameter", entry.Plural)
		case pluralType != "int":
			report(Error, "plural is driven by %q of type %s; plural category selection reads an int", entry.Plural, pluralType)
		}
	}

	var referenceHoles []string
	haveReference := false

	for _, tag := range catalog.Locales {
		text, ok := entry.Texts[tag]
		if !ok {
			report(missing, "locale %q supplies no translation", tag)
			continue
		}
		forms, varies := text.Forms()
		rule, known := RuleFor(tag)

		switch {
		case varies && entry.Plural == "":
			report(Error, "locale %q declares plural variants, but the message declares no plural parameter", tag)
			continue
		case varies && !known:
			out = append(out, Diagnostic{Severity: Error, Path: scope.Path, Line: entry.Line,
				Message: fmt.Sprintf("message %q: %s", entry.Qualified(scope.Name), UnknownPluralLocale{Tag: tag})})
			continue
		case varies:
			if diff := categoryDiff(rule.Categories, forms); diff != "" {
				report(Error, "locale %q declares %s", tag, diff)
				continue
			}
		case !varies && entry.Plural != "" && len(rule.Categories) > 1:
			report(Error, "locale %q supplies one form, but its plural rules distinguish %s", tag, categoryList(rule.Categories))
		}

		for _, body := range textForms(text) {
			pieces, err := ParseText(body, entry.Rich)
			if err != nil {
				report(Error, "locale %q: %v", tag, err)
				continue
			}
			for _, name := range Placeholders(pieces) {
				if !declared[name] {
					report(Error, "locale %q uses placeholder {%s}, which the message does not declare", tag, name)
				}
			}
			used := map[string]bool{}
			for _, name := range Placeholders(pieces) {
				used[name] = true
			}
			for _, param := range entry.Params {
				if !used[param.Name] {
					report(Error, "locale %q never uses declared parameter %q; a translation that drops a placeholder silently loses the value", tag, param.Name)
				}
			}
			holes := Holes(pieces)
			if !entry.Rich && len(holes) > 0 {
				report(Error, "locale %q opens holes but the message is not marked rich", tag)
			}
			if entry.Rich {
				if !haveReference {
					referenceHoles, haveReference = holes, true
				} else if !sameStrings(referenceHoles, holes) {
					report(Error, "locale %q opens holes %s, and another locale opens %s; a hole names markup the template supplies, so every locale opens the same set",
						tag, strings.Join(holes, ", "), strings.Join(referenceHoles, ", "))
				}
			}
		}
	}

	if entry.Rich && haveReference && len(referenceHoles) == 0 {
		report(Warning, "is marked rich but opens no hole, so it renders as an ordinary message at extra cost")
	}

	if entry.Snapshot != "" {
		if text, ok := entry.Texts[catalog.Default]; ok {
			if source := firstForm(text); source != "" && source != entry.Snapshot {
				report(Warning, "source text changed since the ID was assigned, so every other locale is stale; recorded %q, now %q", entry.Snapshot, source)
			}
		}
	}
	return out
}

// textForms returns every body a translation carries, which is one for a simple
// translation and one per category for a varying one.
func textForms(text Text) []string {
	if text.Variants == nil {
		return []string{text.Simple}
	}
	var bodies []string
	for _, category := range categoryOrder {
		if body, ok := text.Variants[category]; ok {
			bodies = append(bodies, body)
		}
	}
	return bodies
}

func firstForm(text Text) string {
	forms := textForms(text)
	if len(forms) == 0 {
		return ""
	}
	return forms[0]
}

func categoryDiff(required, declared []Category) string {
	have := map[Category]bool{}
	for _, category := range declared {
		have[category] = true
	}
	need := map[Category]bool{}
	for _, category := range required {
		need[category] = true
	}
	var missing, extra []string
	for _, category := range required {
		if !have[category] {
			missing = append(missing, string(category))
		}
	}
	for _, category := range declared {
		if !need[category] {
			extra = append(extra, string(category))
		}
	}
	switch {
	case len(missing) > 0 && len(extra) > 0:
		return fmt.Sprintf("no %s variant and an unused %s variant; its plural rules distinguish %s",
			strings.Join(missing, ", "), strings.Join(extra, ", "), categoryList(required))
	case len(missing) > 0:
		return fmt.Sprintf("no %s variant; its plural rules distinguish %s",
			strings.Join(missing, ", "), categoryList(required))
	case len(extra) > 0:
		return fmt.Sprintf("an unused %s variant; its plural rules distinguish %s",
			strings.Join(extra, ", "), categoryList(required))
	}
	return ""
}

func categoryList(categories []Category) string {
	names := make([]string, len(categories))
	for i, category := range categories {
		names[i] = string(category)
	}
	return strings.Join(names, ", ")
}

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	x := append([]string(nil), a...)
	y := append([]string(nil), b...)
	sort.Strings(x)
	sort.Strings(y)
	for i := range x {
		if x[i] != y[i] {
			return false
		}
	}
	return true
}
