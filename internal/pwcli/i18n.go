package pwcli

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/shibukawa/popcornwave/internal/pwgen"
	"github.com/shibukawa/popcornwave/internal/pwmsg"
	templates "github.com/shibukawa/tinybind-go/templates/htmlbind"
)

const i18nUsage = "usage: pw i18n check | extract | rename <from> <to> | export <locale> [file] | import <locale> <file>"

// runI18n reconciles the catalog against the templates that reference it.
//
// It is separate from pw generate because the questions differ: generation asks
// whether every reference resolves, and this asks the reverse as well — whether
// every declared message is still reached. An unused message is not a build
// failure, so a build has no reason to look, and a translator cleaning up needs
// exactly that answer. See .knowledge api:cli-i18n.
func runI18n(args []string, stdout io.Writer) error {
	operation := "check"
	if len(args) > 0 {
		operation = args[0]
	}
	switch operation {
	case "check", "sync":
		// sync is the same reconciliation, named for the direction a translator
		// thinks in. One implementation, because two would drift.
		if len(args) > 1 {
			return fmt.Errorf("i18n: unexpected argument %q; %s", args[1], i18nUsage)
		}
		return runI18nCheck(stdout)
	case "extract":
		return runI18nExtract(args[1:], stdout)
	case "rename":
		if len(args) != 3 {
			return fmt.Errorf("i18n: rename takes the old id and the new one; %s", i18nUsage)
		}
		return runI18nRename(args[1], args[2], stdout)
	case "export":
		return runI18nExport(args[1:], stdout)
	case "import":
		return runI18nImport(args[1:], stdout)
	default:
		return fmt.Errorf("i18n: unknown operation %q; %s", operation, i18nUsage)
	}
}

func runI18nCheck(stdout io.Writer) error {
	root, err := projectRoot(".")
	if err != nil {
		return err
	}
	config, err := loadProjectConfig(root)
	if err != nil {
		return err
	}
	if !config.I18n.Enabled() {
		fmt.Fprintln(stdout, "pw: this project declares no locale, so there is nothing to check")
		return nil
	}
	dir := filepath.Join(root, filepath.FromSlash(config.I18n.Catalog))
	catalog, err := pwmsg.Load(dir, config.I18n.Locales, config.I18n.DefaultLocale)
	if err != nil {
		return fmt.Errorf("pw: %w", err)
	}

	missing := pwmsg.Error
	if config.I18n.Missing == "warn" {
		missing = pwmsg.Warning
	}
	diagnostics := pwmsg.Validate(catalog, missing)

	declared := map[string]bool{}
	for _, scope := range catalog.Scopes {
		for _, entry := range scope.Entries {
			declared[entry.Qualified(scope.Name)] = true
		}
	}

	referenced, err := collectMessageReferences(root, config, stdout)
	if err != nil {
		return err
	}
	for id, where := range referenced {
		if id != "" && !declared[id] {
			diagnostics = append(diagnostics, pwmsg.Diagnostic{
				Severity: pwmsg.Error, Path: where,
				Message: fmt.Sprintf("references message %q, which no catalog declares", id),
			})
		}
	}
	// The reverse direction, which is the one a build never asks. Removal is a
	// human judgement — a message may be referenced from Go, or kept for a page
	// about to land — so it reports rather than fails.
	var unused []string
	for id := range declared {
		if _, ok := referenced[id]; !ok {
			unused = append(unused, id)
		}
	}
	sort.Strings(unused)

	if err := reportCatalogDiagnostics(diagnostics, stdout); err != nil {
		return err
	}
	for _, id := range unused {
		fmt.Fprintf(stdout, "pw: warning: message %q is declared and referenced by no template\n", id)
	}
	fmt.Fprintf(stdout, "pw: %d messages, %d locales, %d referenced\n",
		len(declared), len(config.I18n.Locales), len(referenced))
	return nil
}

// collectMessageReferences reports every message a template references, mapped
// to the file that referenced it.
//
// It reads the upstream reference report rather than the generated output,
// because that report answers before any symbol table exists — which is the
// order this needs, since the table is what the report is used to build.
func collectMessageReferences(root string, config projectConfig, stdout io.Writer) (map[string]string, error) {
	referenced := map[string]string{}
	roots := append([]string(nil), config.Generate.Templates...)
	roots = append(roots, config.Generate.Pages...)

	for _, relative := range roots {
		base := filepath.Join(root, filepath.FromSlash(relative))
		err := filepath.WalkDir(base, func(path string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() || !strings.HasSuffix(entry.Name(), ".pw.html") {
				return err
			}
			source, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			refs, err := templates.MessageRefs(path, source)
			if err != nil {
				// A template that does not parse is pw generate's report to
				// make, with its position and its own message. Reconciliation
				// skips it rather than producing a second, worse rendering of
				// the same failure.
				return nil
			}
			display, relErr := filepath.Rel(root, path)
			if relErr != nil {
				display = path
			}
			for _, ref := range refs {
				if ref.ID == "" {
					fmt.Fprintf(stdout, "pw: %s: reference %q resolves to nothing, because the file declares no message scope\n",
						display, ref.Written)
					continue
				}
				referenced[ref.ID] = display
			}
			return nil
		})
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
	}
	return referenced, nil
}

// i18nBindingNames is what a template author writes. It is exported through the
// help text rather than only living in the generator, because a name nobody can
// look up is a name nobody uses.
var i18nBindingNames = []string{pwgen.LangSegmentBinding, pwgen.LangTagBinding}

// i18nProject loads what every operation needs, refusing early on a project
// that declares no locale.
func i18nProject() (root string, config projectConfig, dir string, err error) {
	root, err = projectRoot(".")
	if err != nil {
		return "", projectConfig{}, "", err
	}
	config, err = loadProjectConfig(root)
	if err != nil {
		return "", projectConfig{}, "", err
	}
	if !config.I18n.Enabled() {
		return "", projectConfig{}, "", fmt.Errorf("i18n: this project declares no locale; add an [i18n] block with locales")
	}
	return root, config, filepath.Join(root, filepath.FromSlash(config.I18n.Catalog)), nil
}

// runI18nExtract turns marked source text into references and catalog entries.
//
// Everything it decides is shown: the ID it proposes for each string, and every
// mark it declined. It never invents a hole name for rich text, because a
// translation whose holes were guessed is one no translator can check.
func runI18nExtract(args []string, stdout io.Writer) error {
	scope := ""
	for _, arg := range args {
		if value, ok := strings.CutPrefix(arg, "--scope="); ok {
			scope = value
			continue
		}
		return fmt.Errorf("i18n: unknown argument %q; extract takes --scope=<name>", arg)
	}
	root, config, dir, err := i18nProject()
	if err != nil {
		return err
	}

	roots := append([]string(nil), config.Generate.Templates...)
	roots = append(roots, config.Generate.Pages...)
	extracted := 0
	for _, relative := range roots {
		base := filepath.Join(root, filepath.FromSlash(relative))
		err := filepath.WalkDir(base, func(path string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() || !strings.HasSuffix(entry.Name(), ".pw.html") {
				return err
			}
			written, err := extractFile(root, dir, config, path, scope, stdout)
			extracted += written
			return err
		})
		if err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	if extracted == 0 {
		fmt.Fprintln(stdout, "pw: nothing marked for extraction; mark an element with i18n, or an attribute with i18n=\"name\"")
		return nil
	}
	fmt.Fprintf(stdout, "pw: extracted %d message(s); run pw generate\n", extracted)
	return nil
}

func extractFile(root, catalogDir string, config projectConfig, path, forcedScope string, stdout io.Writer) (int, error) {
	source, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	display, relErr := filepath.Rel(root, path)
	if relErr != nil {
		display = path
	}
	marks, problems, err := pwmsg.ExtractMarks(path, source)
	if err != nil {
		// A template that does not parse is pw generate's report to make.
		return 0, nil
	}
	for _, problem := range problems {
		fmt.Fprintf(stdout, "pw: %s:%d: <%s> is marked but not extracted: %s\n",
			display, problem.Line, problem.Element, problem.Reason)
	}
	if len(marks) == 0 {
		return 0, nil
	}

	scope := forcedScope
	if scope == "" {
		if declared, err := declaredScope(path, source); err == nil && declared != "" {
			scope = declared
		} else {
			// The file declares none yet, so the scope is named after the
			// directory it sits in, which is the unit a page or a component
			// already groups by.
			scope = catalogPackageName(filepath.Base(filepath.Dir(path)))
		}
	}

	scopeFile := filepath.Join(catalogDir, scope+".yaml")
	existing := map[string]bool{}
	if loaded, err := pwmsg.Load(catalogDir, config.I18n.Locales, config.I18n.DefaultLocale); err == nil {
		for _, s := range loaded.Scopes {
			if s.Name != scope {
				continue
			}
			for _, entry := range s.Entries {
				existing[entry.ID] = true
			}
		}
	}

	ids := make([]string, len(marks))
	var appended strings.Builder
	for i, mark := range marks {
		id, derived := pwmsg.ProposeID(mark.Text, mark.Element, i+1)
		for existing[id] {
			id = fmt.Sprintf("%s-%d", id, i+1)
		}
		existing[id] = true
		ids[i] = id
		appended.WriteString(pwmsg.RenderEntry(id, mark.Text, config.I18n.Locales, config.I18n.DefaultLocale))
		note := ""
		if !derived {
			note = "  (no slug could be derived from this text; rename it with pw i18n rename)"
		}
		fmt.Fprintf(stdout, "pw: %s:%d: %s.%s = %q%s\n", display, mark.Line, scope, id, mark.Text, note)
	}

	rewritten, err := pwmsg.Rewrite(source, marks, ids)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", display, err)
	}
	rewritten = ensureMessagesDeclaration(rewritten, scope)
	if err := os.WriteFile(path, rewritten, 0o644); err != nil {
		return 0, err
	}

	if err := os.MkdirAll(catalogDir, 0o755); err != nil {
		return 0, err
	}
	file, err := os.OpenFile(scopeFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return 0, err
	}
	defer file.Close()
	if _, err := file.WriteString(appended.String()); err != nil {
		return 0, err
	}
	return len(marks), nil
}

// declaredScope reports the file's messages declaration, empty when it has none.
func declaredScope(path string, source []byte) (string, error) {
	refs, err := templates.MessageRefs(path, source)
	if err != nil {
		return "", err
	}
	for _, ref := range refs {
		if ref.Scope != "" {
			return ref.Scope, nil
		}
	}
	return scopeFromSource(source), nil
}

// scopeFromSource reads the declaration textually, for a file that has one and
// no reference yet — which is every file between two extraction runs.
func scopeFromSource(source []byte) string {
	for _, line := range strings.Split(string(source), "\n") {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "messages "); ok {
			return strings.TrimSpace(rest)
		}
	}
	return ""
}

// ensureMessagesDeclaration adds the header declaration when the file has none,
// because a reference with no scope is an error and the rewrite just created
// the first one.
func ensureMessagesDeclaration(source []byte, scope string) []byte {
	if scopeFromSource(source) != "" {
		return source
	}
	lines := strings.Split(string(source), "\n")
	insert := 0
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "package ") {
			insert = i + 1
			break
		}
	}
	declaration := []string{"", "messages " + scope}
	out := append([]string{}, lines[:insert]...)
	out = append(out, declaration...)
	out = append(out, lines[insert:]...)
	return []byte(strings.Join(out, "\n"))
}

// runI18nRename moves an ID, carrying every locale's translation with it.
//
// It is a command rather than an edit because a scope rename orphans every
// translation under it, and doing that by hand loses them silently.
func runI18nRename(from, to string, stdout io.Writer) error {
	_, _, dir, err := i18nProject()
	if err != nil {
		return err
	}
	fromScope, fromID, ok := strings.Cut(from, ".")
	if !ok {
		return fmt.Errorf("i18n: %q is not a qualified id; write scope.id", from)
	}
	toScope, toID, ok := strings.Cut(to, ".")
	if !ok {
		return fmt.Errorf("i18n: %q is not a qualified id; write scope.id", to)
	}
	if fromScope != toScope {
		return fmt.Errorf("i18n: renaming across scopes moves the message to another file; move the entry by hand so the diff shows both sides")
	}
	path := filepath.Join(dir, fromScope+".yaml")
	source, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("i18n: %w", err)
	}
	renamed, ok := pwmsg.RenameEntry(source, fromID, toID)
	if !ok {
		return fmt.Errorf("i18n: %s declares no message %q", path, fromID)
	}
	if err := os.WriteFile(path, renamed, 0o644); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "pw: renamed %s to %s; update the references and run pw generate\n", from, to)
	return nil
}

func runI18nExport(args []string, stdout io.Writer) error {
	if len(args) < 1 {
		return fmt.Errorf("i18n: export takes a locale; %s", i18nUsage)
	}
	locale := args[0]
	_, config, dir, err := i18nProject()
	if err != nil {
		return err
	}
	if !slices.Contains(config.I18n.Locales, locale) {
		return fmt.Errorf("i18n: %q is not a declared locale", locale)
	}
	catalog, err := pwmsg.Load(dir, config.I18n.Locales, config.I18n.DefaultLocale)
	if err != nil {
		return fmt.Errorf("i18n: %w", err)
	}
	data := pwmsg.ExportPO(catalog, locale)
	if len(args) < 2 {
		_, err := stdout.Write(data)
		return err
	}
	if err := os.WriteFile(args[1], data, 0o644); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "pw: wrote %s\n", args[1])
	return nil
}

// runI18nImport writes target translations back, and nothing else.
//
// The source text and the IDs stay the catalog's: a PO file disagreeing about
// either is reporting an edit a translator should not have been able to make,
// so it is reported rather than applied.
func runI18nImport(args []string, stdout io.Writer) error {
	if len(args) != 2 {
		return fmt.Errorf("i18n: import takes a locale and a file; %s", i18nUsage)
	}
	locale, file := args[0], args[1]
	_, config, dir, err := i18nProject()
	if err != nil {
		return err
	}
	if !slices.Contains(config.I18n.Locales, locale) {
		return fmt.Errorf("i18n: %q is not a declared locale", locale)
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	entries, err := pwmsg.ImportPO(data, locale)
	if err != nil {
		return fmt.Errorf("i18n: %s: %w", file, err)
	}
	catalog, err := pwmsg.Load(dir, config.I18n.Locales, config.I18n.DefaultLocale)
	if err != nil {
		return fmt.Errorf("i18n: %w", err)
	}
	declared := map[string]string{}
	for _, scope := range catalog.Scopes {
		for _, entry := range scope.Entries {
			declared[entry.Qualified(scope.Name)] = scope.Path
		}
	}

	byFile := map[string][]pwmsg.POEntry{}
	applied, skipped := 0, 0
	for _, entry := range entries {
		path, ok := declared[entry.ID]
		if !ok {
			fmt.Fprintf(stdout, "pw: warning: %s carries message %q, which this catalog does not declare; skipped\n", file, entry.ID)
			skipped++
			continue
		}
		if entry.Simple == "" && len(entry.Variants) == 0 {
			continue
		}
		byFile[path] = append(byFile[path], entry)
		applied++
	}
	for path, updates := range byFile {
		if err := applyTranslations(path, locale, updates); err != nil {
			return err
		}
	}
	fmt.Fprintf(stdout, "pw: applied %d translation(s) for %s, skipped %d\n", applied, locale, skipped)
	return nil
}

// applyTranslations writes one locale's text into a scope file.
//
// It edits the locale's line under each message rather than re-marshalling, so
// the comments, the ordering, and every other locale's text survive untouched.
func applyTranslations(path, locale string, entries []pwmsg.POEntry) error {
	source, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	scope := strings.TrimSuffix(strings.TrimSuffix(filepath.Base(path), ".yaml"), ".yml")
	lines := strings.Split(string(source), "\n")

	for _, entry := range entries {
		_, id, _ := strings.Cut(entry.ID, ".")
		if scope == "" {
			id = entry.ID
		}
		start := -1
		for i, line := range lines {
			if line == id+":" || strings.HasPrefix(line, id+": ") {
				start = i
				break
			}
		}
		if start < 0 {
			continue
		}
		end := len(lines)
		for i := start + 1; i < len(lines); i++ {
			if lines[i] != "" && !strings.HasPrefix(lines[i], " ") {
				end = i
				break
			}
		}
		replaced := false
		for i := start; i < end; i++ {
			trimmed := strings.TrimSpace(lines[i])
			if !strings.HasPrefix(trimmed, locale+":") {
				continue
			}
			indent := lines[i][:len(lines[i])-len(trimmed)]
			if entry.Variants == nil {
				lines[i] = indent + locale + ": " + strconv.Quote(entry.Simple)
				replaced = true
				break
			}
			var block []string
			block = append(block, indent+locale+":")
			for _, category := range []pwmsg.Category{pwmsg.Zero, pwmsg.One, pwmsg.Two, pwmsg.Few, pwmsg.Many, pwmsg.Other} {
				if text, ok := entry.Variants[category]; ok {
					block = append(block, indent+"  "+string(category)+": "+strconv.Quote(text))
				}
			}
			// A varying translation replaces the locale's whole sub-block,
			// which is the line itself plus every line indented under it.
			stop := i + 1
			for stop < end && strings.HasPrefix(lines[stop], indent+" ") {
				stop++
			}
			lines = append(lines[:i], append(block, lines[stop:]...)...)
			replaced = true
			break
		}
		if !replaced {
			continue
		}
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644)
}
