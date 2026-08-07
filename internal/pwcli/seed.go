package pwcli

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/shibukawa/popcornwave/internal/dbseed"
	"github.com/shibukawa/popcornwave/internal/pwmigrate"
)

const seedUsage = "usage: pw seed [--dir=" + dbseed.DefaultDir + "] [<name>...]"

func runSeed(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	directory := dbseed.DefaultDir
	var names []string
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch {
		case arg == "--dir":
			if index+1 >= len(args) {
				return fmt.Errorf("seed: --dir requires a directory; %s", seedUsage)
			}
			index++
			directory = args[index]
		case strings.HasPrefix(arg, "--dir="):
			directory = strings.TrimPrefix(arg, "--dir=")
		case strings.HasPrefix(arg, "-"):
			return fmt.Errorf("seed: unknown flag %q; %s", arg, seedUsage)
		default:
			names = append(names, arg)
		}
	}
	root, err := projectRoot(".")
	if err != nil {
		return err
	}
	config, err := loadProjectConfig(root)
	if err != nil {
		return err
	}
	if !filepath.IsAbs(directory) {
		directory = filepath.Join(root, filepath.FromSlash(directory))
	}
	paths, err := dbseed.Resolve(directory, names)
	if err != nil {
		return fmt.Errorf("seed: %w", err)
	}
	// The application owns configuration precedence, so it reports the DSN
	// instead of the CLI reimplementing TOML, environment, and flag order.
	dsn, err := resolveApplicationDSN(ctx, root, config.Main, stderr)
	if err != nil {
		return fmt.Errorf("seed: %w", err)
	}
	dialect, err := dbseed.ResolveDialect(dsn)
	if err != nil {
		return redactDSN(fmt.Errorf("seed: %w", err), dsn)
	}
	target, err := pwmigrate.Open(dsn)
	if err != nil {
		return redactDSN(fmt.Errorf("seed: %w", err), dsn)
	}
	defer target.Close()
	for _, path := range paths {
		fmt.Fprintln(stdout, "seeding", relativeTo(root, path))
		if err := dbseed.Apply(ctx, dbseed.FromSQL(target.DB), dialect, false, []string{path}); err != nil {
			return redactDSN(fmt.Errorf("seed: %w", err), dsn)
		}
	}
	return nil
}

func relativeTo(root, path string) string {
	if relative, err := filepath.Rel(root, path); err == nil {
		return relative
	}
	return path
}
