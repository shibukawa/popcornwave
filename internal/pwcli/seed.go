package pwcli

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/shibukawa/popcornwave/internal/dbseed"
)

func runSeed(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	directory := dbseed.DefaultDir
	var names []string
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch {
		case arg == "--dir":
			if index+1 >= len(args) {
				return fmt.Errorf("seed: --dir requires a directory")
			}
			index++
			directory = args[index]
		case strings.HasPrefix(arg, "--dir="):
			directory = strings.TrimPrefix(arg, "--dir=")
		case strings.HasPrefix(arg, "-"):
			return fmt.Errorf("seed: unknown flag %q", arg)
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
	for _, path := range paths {
		fmt.Fprintln(stdout, "seeding", relativeTo(root, path))
	}
	command := exec.CommandContext(ctx, "go", "run", config.Main, "--pw-seed="+strings.Join(paths, string(filepath.ListSeparator)))
	command.Dir = root
	command.Stdout = stdout
	command.Stderr = stderr
	command.Env = os.Environ()
	if err := command.Run(); err != nil {
		return fmt.Errorf("seed: %w", err)
	}
	return nil
}

func relativeTo(root, path string) string {
	if relative, err := filepath.Rel(root, path); err == nil {
		return relative
	}
	return path
}
