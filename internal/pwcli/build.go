package pwcli

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
)

func runBuild(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	if len(args) != 0 {
		return fmt.Errorf("build: unexpected arguments")
	}
	if err := runGenerate(ctx, nil, stdout); err != nil {
		return err
	}
	root, err := projectRoot(".")
	if err != nil {
		return err
	}
	config, err := loadProjectConfig(root)
	if err != nil {
		return err
	}
	if config.Tailwind.Enabled {
		config.Tailwind.Minify = true
		if err := buildTailwind(ctx, root, config.Tailwind, stdout, stderr); err != nil {
			return err
		}
	}
	if err := preparePublicAssets(root); err != nil {
		return err
	}
	command := exec.CommandContext(ctx, "go", "build", config.Main)
	command.Dir, command.Stdout, command.Stderr, command.Env = root, stdout, stderr, os.Environ()
	if err := command.Run(); err != nil {
		return fmt.Errorf("go build: %w", err)
	}
	return nil
}
