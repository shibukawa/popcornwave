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
	root, config, err := buildProject("build")
	if err != nil {
		return err
	}
	progress := newProgressRegion(stdout)
	if err := prepareBuildInputs(ctx, root, config, progress, stdout, stderr); err != nil {
		progress.Done()
		return err
	}
	progress.Phase("compiling")
	// A pw build is the deployable artifact. Strip DWARF and the host linker
	// symbol table, while retaining Go's pclntab so panic stacks still carry
	// function names and line numbers. trimpath removes checkout-specific source
	// prefixes and makes otherwise identical builds reproducible across machines.
	command := exec.CommandContext(ctx, "go", "build", "-trimpath", "-ldflags=-s -w", config.Main)
	command.Dir, command.Stdout, command.Stderr, command.Env = root, stdout, stderr, os.Environ()
	err = command.Run()
	progress.Done()
	if err != nil {
		return fmt.Errorf("go build: %w", err)
	}
	return nil
}

// runPrepare is runBuild without its final compile step. A build the framework
// does not drive — the tinygo invocation in Dockerfile.tinygo, a cross-compiled
// go build with the operator's own flags, an image builder that owns the
// compile step — needs the same tree and has no way to produce it, because
// pw generate reaches only the first of the steps below.
func runPrepare(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	if len(args) != 0 {
		return fmt.Errorf("prepare: unexpected arguments")
	}
	root, config, err := buildProject("prepare")
	if err != nil {
		return err
	}
	progress := newProgressRegion(stdout)
	err = prepareBuildInputs(ctx, root, config, progress, stdout, stderr)
	progress.Done()
	return err
}

// buildProject resolves the project the two commands above run in. The kind is
// read before anything runs. Generation would succeed in a package and the
// link step would then fail on a missing entry point, which is a late error
// about the wrong thing.
func buildProject(command string) (string, projectConfig, error) {
	root, err := projectRoot(".")
	if err != nil {
		return "", projectConfig{}, err
	}
	config, err := loadProjectConfig(root)
	if err != nil {
		return "", projectConfig{}, err
	}
	if err := refuseInPackage(config, command); err != nil {
		return "", projectConfig{}, err
	}
	return root, config, nil
}

// prepareBuildInputs writes everything a compiler needs that is not in version
// control: the generated Go, the production stylesheet, and the derived asset
// tree public.go embeds. It ends with the development-only import check, which
// belongs here rather than beside the compiler because prepare hands the tree
// to a compiler it does not run.
//
// config is taken by value: the Tailwind minify override below is this
// sequence's, not the project's.
func prepareBuildInputs(ctx context.Context, root string, config projectConfig, progress *progressRegion, stdout, stderr io.Writer) error {
	progress.Phase("generating")
	if _, err := generateProject(ctx, false, stdout, false); err != nil {
		return err
	}
	if config.Tailwind.Enabled {
		progress.Phase("building CSS")
		config.Tailwind.Minify = true
		if err := buildTailwind(ctx, root, config.Tailwind, stdout, stderr); err != nil {
			return err
		}
	}
	progress.Phase("building assets")
	report, err := buildDerivedAssets(root, config.Assets)
	if err != nil {
		return err
	}
	reportDerivedAssets(stdout, report)
	return rejectDevelopmentImports(ctx, root, config.Main)
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
