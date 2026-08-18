package pwcli

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/shibukawa/popcornweb/internal/pwgen"
	"github.com/shibukawa/tinybind-go/templates/templatefmt"
)

const fmtUsage = "usage: pw fmt [--check] [--stdin=html|sql|dynamo] [--width=n] [<path>...]"

type fmtOptions struct {
	Check bool
	Stdin string
	Width int
	Paths []string
}

func parseFmtOptions(args []string) (fmtOptions, error) {
	options := fmtOptions{}
	for _, arg := range args {
		name, value, hasValue := strings.Cut(arg, "=")
		switch {
		case arg == "--check":
			options.Check = true
		case name == "--stdin" && hasValue:
			options.Stdin = value
		case name == "--width" && hasValue:
			width, err := strconv.Atoi(value)
			if err != nil || width <= 0 {
				return options, fmt.Errorf("fmt: --width needs a positive number, got %q", value)
			}
			options.Width = width
		case strings.HasPrefix(arg, "-"):
			return options, fmt.Errorf("fmt: unknown argument %q; %s", arg, fmtUsage)
		default:
			options.Paths = append(options.Paths, arg)
		}
	}
	if options.Stdin != "" {
		switch options.Stdin {
		case string(templatefmt.HTML), string(templatefmt.SQL), string(templatefmt.Dynamo):
		default:
			return options, fmt.Errorf("fmt: --stdin needs html, sql, or dynamo, got %q", options.Stdin)
		}
		if options.Check || len(options.Paths) > 0 {
			return options, fmt.Errorf("fmt: --stdin formats one stream and takes no path and no --check")
		}
	}
	return options, nil
}

// formatOptions carries the Popcorn Web suffixes into templatefmt, so a
// branded source formats on the same terms it generates. No SQL dialect is
// involved: layout is a token-stream pass and the placeholder style is a
// generation concern.
func (o fmtOptions) formatOptions() templatefmt.Options {
	return templatefmt.Options{
		Width:         o.Width,
		HTMLPattern:   pwgen.HTMLTemplatePattern,
		SQLPattern:    pwgen.SQLTemplatePattern,
		DynamoPattern: pwgen.DynamoTemplatePattern,
	}
}

func runFmt(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	options, err := parseFmtOptions(args)
	if err != nil {
		return err
	}
	if options.Stdin != "" {
		// A stream has no project and no file name, so this path reads no
		// configuration and touches no file. It is what an editor filters through.
		return formatStream(os.Stdin, stdout, templatefmt.Format(options.Stdin), options.formatOptions())
	}

	sources, err := fmtSources(options)
	if err != nil {
		return err
	}
	return formatFiles(ctx, sources, options, stdout, stderr)
}

func formatStream(in io.Reader, out io.Writer, format templatefmt.Format, options templatefmt.Options) error {
	source, err := io.ReadAll(in)
	if err != nil {
		return fmt.Errorf("fmt: read standard input: %w", err)
	}
	formatted, err := templatefmt.SourceAs(format, "<stdin>", source, options)
	if err != nil {
		return fmt.Errorf("fmt: %w", err)
	}
	_, err = out.Write(formatted)
	return err
}

// fmtSources resolves what to format: the named paths when there are any, and
// otherwise every template source under a generate purpose. Naming a path is
// already a decision, so it is not filtered against the purposes.
func fmtSources(options fmtOptions) ([]string, error) {
	if len(options.Paths) > 0 {
		out := make([]string, 0, len(options.Paths))
		for _, path := range options.Paths {
			absolute, err := filepath.Abs(path)
			if err != nil {
				return nil, err
			}
			out = append(out, absolute)
		}
		return out, nil
	}

	root, err := projectRoot(".")
	if err != nil {
		return nil, err
	}
	config, err := loadProjectConfig(root)
	if err != nil {
		return nil, err
	}
	patterns := options.formatOptions()

	found := map[string]bool{}
	scope := config.Generate
	// Handlers and Config are listed too: a purpose owns a directory rather than
	// a file kind, and a template beside a handler is the scaffolded shape.
	for _, sources := range [][]string{
		scope.Handlers, scope.Templates, scope.Queries, scope.Config, scope.Pages, scope.Dynamo,
	} {
		err := walkSources(root, sources, func(path string, entry fs.DirEntry) error {
			if _, err := templatefmt.Identify(entry.Name(), patterns); err != nil {
				return nil
			}
			found[path] = true
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	out := make([]string, 0, len(found))
	for path := range found {
		out = append(out, path)
	}
	sort.Strings(out)
	return out, nil
}

func formatFiles(ctx context.Context, sources []string, options fmtOptions, stdout, stderr io.Writer) error {
	patterns := options.formatOptions()
	var differed, failed int

	for _, path := range sources {
		if err := ctx.Err(); err != nil {
			return err
		}
		source, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintln(stderr, "fmt:", err)
			failed++
			continue
		}
		format, err := templatefmt.Identify(filepath.Base(path), patterns)
		if err != nil {
			// Only reachable for a named path, since discovery already identified
			// everything it collected.
			fmt.Fprintf(stderr, "fmt: %s: %v\n", displayPath(path), err)
			failed++
			continue
		}
		formatted, err := templatefmt.SourceAs(format, path, source, patterns)
		if err != nil {
			// Report and keep going: a formatting run is not a build, and stopping
			// at the first bad file hides every other one.
			fmt.Fprintf(stderr, "fmt: %v\n", err)
			failed++
			continue
		}
		if string(formatted) == string(source) {
			continue
		}
		differed++
		if options.Check {
			fmt.Fprintln(stdout, displayPath(path))
			continue
		}
		// Written only when it differs, so an already formatted file keeps its
		// timestamp and no watcher wakes for it.
		if err := writeFormatted(path, formatted); err != nil {
			fmt.Fprintln(stderr, "fmt:", err)
			failed++
		}
	}

	switch {
	case failed > 0:
		return &exitError{command: "pw fmt", message: plural(failed, "source", "sources") + " could not be formatted"}
	case options.Check && differed > 0:
		return &exitError{command: "pw fmt", message: plural(differed, "source is", "sources are") + " not formatted"}
	}
	return nil
}

// writeFormatted replaces a source in place, preserving its mode.
func writeFormatted(path string, formatted []byte) error {
	mode := fs.FileMode(0o644)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}
	return os.WriteFile(path, formatted, mode)
}

// displayPath prefers a path relative to the working directory, because that is
// what a terminal and an editor problem matcher can both resolve.
func displayPath(path string) string {
	working, err := os.Getwd()
	if err != nil {
		return path
	}
	relative, err := filepath.Rel(working, path)
	if err != nil || strings.HasPrefix(relative, "..") {
		return path
	}
	return relative
}

func plural(count int, one, many string) string {
	if count == 1 {
		return "1 " + one
	}
	return strconv.Itoa(count) + " " + many
}
