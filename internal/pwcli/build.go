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
	progress := newProgressRegion(stdout)
	progress.Phase("generating")
	if _, err := generateProject(ctx, false, stdout, false); err != nil {
		progress.Done()
		return err
	}
	root, err := projectRoot(".")
	if err != nil {
		progress.Done()
		return err
	}
	config, err := loadProjectConfig(root)
	if err != nil {
		progress.Done()
		return err
	}
	if config.Tailwind.Enabled {
		progress.Phase("building CSS")
		config.Tailwind.Minify = true
		if err := buildTailwind(ctx, root, config.Tailwind, stdout, stderr); err != nil {
			progress.Done()
			return err
		}
	}
	if err := preparePublicAssets(root); err != nil {
		progress.Done()
		return err
	}
	if err := rejectDevelopmentImports(ctx, root, config.Main); err != nil {
		progress.Done()
		return err
	}
	progress.Phase("compiling")
	command := exec.CommandContext(ctx, "go", "build", config.Main)
	command.Dir, command.Stdout, command.Stderr, command.Env = root, stdout, stderr, os.Environ()
	err = command.Run()
	progress.Done()
	if err != nil {
		return fmt.Errorf("go build: %w", err)
	}
	return nil
}

// developmentOnlyPackages must never reach a built application. Each one is a
// security defect in a deployable binary rather than a configuration mistake:
// the development identity provider authenticates nobody, the virtual
// authenticator holds a signing key that mints assertions a relying party
// accepts, and the authentication test seam builds a request context that is
// already logged in.
var developmentOnlyPackages = []string{
	"github.com/shibukawa/popcornwave/contrib/devidp",
	"github.com/shibukawa/popcornwave/contrib/passkey/passkeytest",
	"github.com/shibukawa/popcornwave/plugin/auth/authtest",
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
