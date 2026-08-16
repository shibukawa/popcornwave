package pwmsg

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	yaml "github.com/goccy/go-yaml"
)

// Catalog is every message a project declares, read from the directory named by
// i18n.catalog.
//
// One file is one scope, because decision:message-scope-declaration makes the
// scope a template writes the natural sharding key: two translators working on
// separate features do not contend on one file.
type Catalog struct {
	Locales []string
	Default string
	Scopes  []Scope
	// Routing is the locale mode per path prefix, the display labels, and
	// whether the default locale carries a prefix. It is generated into the
	// message package because all of it is build configuration the served
	// process cannot otherwise know. See .knowledge decision:locale-url-modes.
	Routing Routing
}

// Routing is the build-time locale routing declaration.
type Routing struct {
	// Routes are prefix and mode pairs, the mode being "path", "cookie", or
	// "header".
	Routes []Route
	// Labels is the display name of each locale, written in that locale.
	Labels map[string]string
	// PrefixDefault decides whether the default locale carries a path prefix.
	PrefixDefault bool
}

// Route binds one path prefix to a locale mode.
type Route struct {
	Prefix string
	Mode   string
}

// Scope is one catalog file.
type Scope struct {
	Name    string
	Path    string
	Entries []Entry
}

// Entry is one message.
type Entry struct {
	// ID is the bare name within the scope. Qualified() joins it to the scope.
	ID     string
	Params []Param
	// Plural names the parameter driving category selection, empty for a
	// message with no plural variation.
	Plural string
	// Rich marks a message whose translations carry holes, which renders
	// through the segment path of policy:message-rich-text rather than as a
	// string.
	Rich bool
	// Snapshot is the source text recorded when the ID was assigned. It is
	// written by tooling and compared, never edited by hand: a source text
	// differing from it is what marks every other locale stale, per
	// decision:message-id-assignment.
	Snapshot string
	Texts    map[string]Text
	// Line is where the entry starts in its file, for diagnostics.
	Line int
}

// Qualified returns scope.id, which is what a template reference resolves to.
func (e Entry) Qualified(scope string) string {
	if scope == "" {
		return e.ID
	}
	return scope + "." + e.ID
}

// Param is one declared argument.
type Param struct {
	Name string
	Type string
}

// Text is one locale's translation: either a single form, or one per plural
// category the locale distinguishes.
type Text struct {
	Simple   string
	Variants map[Category]string
}

// UnmarshalYAML accepts either a scalar or a mapping of category to scalar, so
// a locale with no plural variation reads as the string it is.
func (t *Text) UnmarshalYAML(data []byte) error {
	var simple string
	if err := yaml.Unmarshal(data, &simple); err == nil {
		t.Simple = simple
		return nil
	}
	var variants map[string]string
	if err := yaml.Unmarshal(data, &variants); err != nil {
		return fmt.Errorf("a translation is either text or a mapping of plural category to text: %w", err)
	}
	t.Variants = make(map[Category]string, len(variants))
	for name, text := range variants {
		category := Category(name)
		if !knownCategory(category) {
			return fmt.Errorf("unknown plural category %q; the CLDR set is zero, one, two, few, many, other", name)
		}
		t.Variants[category] = text
	}
	return nil
}

func knownCategory(c Category) bool {
	for _, known := range categoryOrder {
		if known == c {
			return true
		}
	}
	return false
}

// Forms returns the translation's forms in categoryOrder, and whether it varies.
func (t Text) Forms() ([]Category, bool) {
	if t.Variants == nil {
		return nil, false
	}
	var present []Category
	for _, category := range categoryOrder {
		if _, ok := t.Variants[category]; ok {
			present = append(present, category)
		}
	}
	return present, true
}

// yamlEntry is the on-disk shape. It is separate from Entry so the file format
// can carry the params shorthand and Entry can carry parsed values.
type yamlEntry struct {
	Params   []string        `yaml:"params"`
	Plural   string          `yaml:"plural"`
	Rich     bool            `yaml:"rich"`
	Snapshot string          `yaml:"snapshot"`
	Texts    map[string]Text `yaml:"locales"`
}

// Load reads every catalog file under dir. locales and defaultTag come from
// data:i18n-config and are carried on the result because generation needs the
// declared order, not the order a file happens to list.
func Load(dir string, locales []string, defaultTag string) (*Catalog, error) {
	names, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
	if err != nil {
		return nil, err
	}
	more, err := filepath.Glob(filepath.Join(dir, "*.yml"))
	if err != nil {
		return nil, err
	}
	names = append(names, more...)
	sort.Strings(names)

	catalog := &Catalog{Locales: append([]string(nil), locales...), Default: defaultTag}
	seen := map[string]string{}
	for _, name := range names {
		scope, err := loadScope(name)
		if err != nil {
			return nil, err
		}
		if previous, duplicate := seen[scope.Name]; duplicate {
			return nil, fmt.Errorf("%s and %s both declare scope %q; one scope is one file", previous, name, scope.Name)
		}
		seen[scope.Name] = name
		catalog.Scopes = append(catalog.Scopes, *scope)
	}
	return catalog, nil
}

func loadScope(path string) (*Scope, error) {
	source, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var raw map[string]yamlEntry
	if err := yaml.Unmarshal(source, &raw); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	name := strings.TrimSuffix(strings.TrimSuffix(filepath.Base(path), ".yaml"), ".yml")
	scope := &Scope{Name: name, Path: path}
	for id, entry := range raw {
		if err := validateID(id); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		params, err := parseParams(entry.Params)
		if err != nil {
			return nil, fmt.Errorf("%s: message %q: %w", path, id, err)
		}
		scope.Entries = append(scope.Entries, Entry{
			ID:       id,
			Params:   params,
			Plural:   entry.Plural,
			Rich:     entry.Rich,
			Snapshot: entry.Snapshot,
			Texts:    entry.Texts,
			Line:     lineOf(source, id),
		})
	}
	sort.Slice(scope.Entries, func(i, j int) bool { return scope.Entries[i].ID < scope.Entries[j].ID })
	return scope, nil
}

// lineOf finds the line a top-level key starts on. It is a scan rather than a
// parser position because goccy exposes positions only through its AST, and a
// diagnostic pointing at the right line is worth more than the exactness of
// pointing at the right column.
func lineOf(source []byte, key string) int {
	prefix := key + ":"
	line := 1
	for _, text := range strings.Split(string(source), "\n") {
		if strings.HasPrefix(text, prefix) {
			return line
		}
		line++
	}
	return 0
}

// validateID enforces the lexical form system:tinybind accepts, so a catalog
// cannot declare an ID no template could reference.
func validateID(id string) error {
	if id == "" {
		return errors.New("a message ID may not be empty")
	}
	for _, segment := range strings.Split(id, ".") {
		if segment == "" {
			return fmt.Errorf("message ID %q has an empty segment", id)
		}
		if segment[0] == '-' || segment[len(segment)-1] == '-' {
			return fmt.Errorf("message ID %q has a segment starting or ending in a hyphen, which the template expression lexer reads as subtraction", id)
		}
		for i := 0; i < len(segment); i++ {
			c := segment[i]
			ok := c == '-' || c == '_' ||
				(c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
			if !ok {
				return fmt.Errorf("message ID %q carries %q, which is outside the word characters and hyphens a reference accepts", id, string(c))
			}
		}
	}
	return nil
}

// parseParams reads the "name type" shorthand. The list is ordered because the
// declared order fixes the generated function's argument order, which is what a
// reference's named arguments are put back into.
func parseParams(declared []string) ([]Param, error) {
	var params []Param
	seen := map[string]bool{}
	for _, text := range declared {
		fields := strings.Fields(text)
		if len(fields) != 2 {
			return nil, fmt.Errorf("parameter %q is not in \"name type\" form", text)
		}
		if seen[fields[0]] {
			return nil, fmt.Errorf("parameter %q declared twice", fields[0])
		}
		seen[fields[0]] = true
		params = append(params, Param{Name: fields[0], Type: fields[1]})
	}
	return params, nil
}
