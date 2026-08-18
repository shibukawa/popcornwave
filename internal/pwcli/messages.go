package pwcli

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/shibukawa/popcornweb/internal/pwmsg"
)

// messagesGeneratedName is the file the message package is written to. It
// follows the suffix policy:generated-artifacts uses for every other emitted
// Go file, so the same sweep and the same version-control rule reach it.
const messagesGeneratedName = "messages_pw_gen.go"

// messagePlan is what one catalog read produced.
type messagePlan struct {
	// Symbols maps a resolved message ID to the Go symbol a reference calls.
	// It is nil for a project declaring no locale, which is what makes any
	// reference in such a project fail generation rather than resolve to
	// nothing.
	Symbols map[string]pwmsg.Symbol
	// ImportPath is where the generated package lives.
	ImportPath string
}

// planMessages reads the catalog and plans the generated message package.
//
// It runs before any template is compiled, because a reference is type-checked
// against these symbols and the diagnostic degrades to an unresolved name if
// they are not there yet. See .knowledge decision:message-code-shape.
func planMessages(root string, config projectConfig, changes []fileChange, stdout io.Writer) ([]fileChange, messagePlan, error) {
	if !config.I18n.Enabled() {
		return changes, messagePlan{}, nil
	}
	dir := filepath.Join(root, filepath.FromSlash(config.I18n.Catalog))
	info, err := os.Stat(dir)
	switch {
	case os.IsNotExist(err):
		return nil, messagePlan{}, fmt.Errorf("pw: i18n.catalog %q does not exist; a project declaring locales has a catalog directory", config.I18n.Catalog)
	case err != nil:
		return nil, messagePlan{}, err
	case !info.IsDir():
		return nil, messagePlan{}, fmt.Errorf("pw: i18n.catalog %q is not a directory", config.I18n.Catalog)
	}

	catalog, err := pwmsg.Load(dir, config.I18n.Locales, config.I18n.DefaultLocale)
	if err != nil {
		return nil, messagePlan{}, fmt.Errorf("pw: %w", err)
	}
	// The routing declaration is generated into the message package because a
	// served process cannot read popcornweb.toml, and the mode of a route
	// decides both what a link carries and what the response varies on.
	catalog.Routing = pwmsg.Routing{
		Labels:        config.I18n.Labels,
		PrefixDefault: config.I18n.PrefixDefault,
	}
	for _, route := range config.I18n.Routes {
		catalog.Routing.Routes = append(catalog.Routing.Routes, pwmsg.Route{Prefix: route.Prefix, Mode: route.Mode})
	}

	missing := pwmsg.Error
	if config.I18n.Missing == "warn" {
		missing = pwmsg.Warning
	}
	if err := reportCatalogDiagnostics(pwmsg.Validate(catalog, missing), stdout); err != nil {
		return nil, messagePlan{}, err
	}
	if err := checkLocaleLabels(config.I18n, stdout); err != nil {
		return nil, messagePlan{}, err
	}

	packageName := catalogPackageName(filepath.Base(dir))
	generated, err := pwmsg.Generate(catalog, packageName)
	if err != nil {
		return nil, messagePlan{}, fmt.Errorf("pw: %w", err)
	}

	changes, err = appendIfChanged(changes, filepath.Join(dir, messagesGeneratedName), generated.Source)
	if err != nil {
		return nil, messagePlan{}, err
	}

	module, moduleDir, err := moduleImportPath(root)
	if err != nil {
		return nil, messagePlan{}, err
	}
	relative, err := filepath.Rel(moduleDir, dir)
	if err != nil {
		return nil, messagePlan{}, err
	}
	return changes, messagePlan{
		Symbols:    generated.Symbols,
		ImportPath: path.Join(module, filepath.ToSlash(relative)),
	}, nil
}

// reportCatalogDiagnostics prints warnings and fails on the first error.
//
// Every finding is printed before failing, rather than stopping at the first:
// a translator fixing a catalog wants the whole list, and the run is going to
// fail either way.
func reportCatalogDiagnostics(diagnostics []pwmsg.Diagnostic, stdout io.Writer) error {
	sort.SliceStable(diagnostics, func(i, j int) bool {
		if diagnostics[i].Path != diagnostics[j].Path {
			return diagnostics[i].Path < diagnostics[j].Path
		}
		return diagnostics[i].Line < diagnostics[j].Line
	})
	errors := 0
	for _, diagnostic := range diagnostics {
		fmt.Fprintf(stdout, "pw: %s\n", diagnostic)
		if diagnostic.Severity == pwmsg.Error {
			errors++
		}
	}
	if errors > 0 {
		return fmt.Errorf("pw: %d message catalog error(s)", errors)
	}
	return nil
}

// checkLocaleLabels reports a locale with no display label.
//
// It is a warning rather than an error because a project mid-translation has a
// working site without it, and falling back to the raw tag is visible in the
// switcher rather than silent: a reader sees "en" where "English" belongs.
func checkLocaleLabels(config i18nConfig, stdout io.Writer) error {
	var missing []string
	for _, tag := range config.Locales {
		if config.Labels[tag] == "" {
			missing = append(missing, tag)
		}
	}
	if len(missing) > 0 {
		fmt.Fprintf(stdout, "pw: warning: no i18n.label for %s, so a language switcher shows the tag instead of a name\n",
			strings.Join(missing, ", "))
	}
	return nil
}

// catalogPackageName turns a directory name into a legal package identifier, so
// a catalog directory named messages-ja does not emit a file that will not
// compile.
//
// It is derived from the name rather than read from the directory's existing Go
// files, the way goPackageName does elsewhere, because the catalog directory
// usually holds only YAML the first time this runs.
func catalogPackageName(name string) string {
	var out strings.Builder
	for i, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r == '_':
			out.WriteRune(r)
		case r >= '0' && r <= '9' && i > 0:
			out.WriteRune(r)
		}
	}
	if out.Len() == 0 {
		return "messages"
	}
	return out.String()
}
