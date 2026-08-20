package pwcli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/shibukawa/popcornweb/internal/pwlsp"
)

const renameUsage = "usage: pw rename <declaration> <new-name> [--apply]"

// runRename previews or applies the requirement:declaration-rename edit set.
//
// The default is a preview. The set reaches handwritten Go and files the
// developer never opened, so seeing it before it happens is the point of the
// command; --apply is the second look.
func runRename(args []string, stdout io.Writer) error {
	from, to, apply, err := parseRenameOptions(args)
	if err != nil {
		return err
	}
	root, err := projectRoot(".")
	if err != nil {
		// A rename resolves names across a project, and there is nothing to
		// resolve against without one.
		return fmt.Errorf("rename: %w", err)
	}
	project, err := loadLSPProject(root)
	if err != nil {
		return fmt.Errorf("rename: %w", err)
	}

	plan, err := pwlsp.PlanRenameIn(project, from, to)
	if err != nil {
		return fmt.Errorf("rename: %w", err)
	}
	if len(plan.Refusals) > 0 {
		return &exitError{command: "rename", message: strings.Join(plan.Refusals, "; ")}
	}
	if plan.Empty() {
		return &exitError{command: "rename", message: "nothing references " + from}
	}

	writeRenamePlan(stdout, root, plan)
	if !apply {
		fmt.Fprintln(stdout, "\nNothing was written. Run again with --apply.")
		return nil
	}
	if err := applyRenamePlan(plan); err != nil {
		return fmt.Errorf("rename: %w", err)
	}
	fmt.Fprintln(stdout, "\nApplied. Run pw generate: the generated Go still carries the old name.")
	return nil
}

func parseRenameOptions(args []string) (from, to string, apply bool, err error) {
	var positional []string
	for _, arg := range args {
		switch {
		case arg == "--apply":
			apply = true
		case strings.HasPrefix(arg, "-"):
			return "", "", false, fmt.Errorf("rename: unknown argument %q; %s", arg, renameUsage)
		default:
			positional = append(positional, arg)
		}
	}
	if len(positional) != 2 {
		return "", "", false, fmt.Errorf("rename: %s", renameUsage)
	}
	return positional[0], positional[1], apply, nil
}

// writeRenamePlan prints what would change, grouped by file, because a set
// spread over a flat list of positions is one nobody reads.
func writeRenamePlan(stdout io.Writer, root string, plan pwlsp.RenamePlan) {
	fmt.Fprintf(stdout, "%s -> %s: %s\n\n", plan.From, plan.To, plan.Summary())
	paths := make([]string, 0, len(plan.Changes))
	for uri := range plan.Changes {
		paths = append(paths, uri)
	}
	sort.Strings(paths)
	for _, uri := range paths {
		path := pwlsp.PathOf(uri)
		if relative, err := filepath.Rel(root, path); err == nil {
			path = filepath.ToSlash(relative)
		}
		lines := make([]string, 0, len(plan.Changes[uri]))
		for _, edit := range plan.Changes[uri] {
			lines = append(lines, itoa(edit.Range.Start.Line+1))
		}
		sort.Strings(lines)
		fmt.Fprintf(stdout, "  %s: %s\n", path, strings.Join(lines, ", "))
	}
}

// applyRenamePlan rewrites each file once, with the edits already ordered
// last-first so an earlier one never moves a later one.
func applyRenamePlan(plan pwlsp.RenamePlan) error {
	for uri, edits := range plan.Changes {
		path := pwlsp.PathOf(uri)
		source, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		text := string(source)
		for _, edit := range edits {
			start, end := pwlsp.OffsetsOf(text, edit.Range)
			text = text[:start] + edit.NewText + text[end:]
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			return err
		}
	}
	return nil
}
