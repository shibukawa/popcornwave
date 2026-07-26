package pwcli

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
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
	if err := rejectDevelopmentImports(ctx, root, config.Main); err != nil {
		return err
	}
	command := exec.CommandContext(ctx, "go", "build", config.Main)
	command.Dir, command.Stdout, command.Stderr, command.Env = root, stdout, stderr, os.Environ()
	if err := command.Run(); err != nil {
		return fmt.Errorf("go build: %w", err)
	}
	return nil
}

// developmentOnlyPackages must never reach a built application. The
// development identity provider authenticates nobody, so linking it into a
// deployable binary is a security defect rather than a configuration mistake.
var developmentOnlyPackages = []string{
	"github.com/shibukawa/popcornwave/contrib/devidp",
}

func rejectDevelopmentImports(ctx context.Context, root, mainPackage string) error {
	command := exec.CommandContext(ctx, "go", "list", "-deps", "-f", "{{.ImportPath}}", mainPackage)
	command.Dir, command.Env = root, os.Environ()
	output, err := command.Output()
	if err != nil {
		// A dependency graph that cannot be listed fails the build below with
		// a compiler diagnostic, which is more useful than this check's error.
		return nil
	}
	for _, line := range strings.Split(string(output), "\n") {
		imported := strings.TrimSpace(line)
		for _, forbidden := range developmentOnlyPackages {
			if imported == forbidden {
				return fmt.Errorf("pw build: %s imports %s, which is development-only and must not ship in an application", mainPackage, forbidden)
			}
		}
	}
	return nil
}
