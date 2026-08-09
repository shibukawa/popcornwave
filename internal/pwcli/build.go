package pwcli

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// buildUsage names the one option the two build commands share.
var buildUsage = "usage: pw build [--debug]  |  pw prepare [--debug]"

func runBuild(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	debug, err := debugFlag("build", args)
	if err != nil {
		return err
	}
	root, config, err := buildProject("build")
	if err != nil {
		return err
	}
	progress := newProgressRegion(stdout)
	if err := prepareBuildInputs(ctx, root, config, debug, progress, stdout, stderr); err != nil {
		progress.Done()
		return err
	}
	progress.Phase("compiling")
	// A pw build is the deployable artifact. Strip DWARF and the host linker
	// symbol table, while retaining Go's pclntab so panic stacks still carry
	// function names and line numbers. trimpath removes checkout-specific source
	// prefixes and makes otherwise identical builds reproducible across machines,
	// which is why it is passed either way: it removes no debug information.
	build := []string{"build", "-trimpath"}
	if !debug {
		build = append(build, "-ldflags=-s -w")
	}
	command := exec.CommandContext(ctx, "go", append(build, config.Main)...)
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
	debug, err := debugFlag("prepare", args)
	if err != nil {
		return err
	}
	root, config, err := buildProject("prepare")
	if err != nil {
		return err
	}
	progress := newProgressRegion(stdout)
	err = prepareBuildInputs(ctx, root, config, debug, progress, stdout, stderr)
	progress.Done()
	return err
}

// debugFlag reads the one option these two commands take.
//
// It is on prepare as well as on build, and that is the point rather than a
// convenience: prepare exists for a compile this project does not run — the
// TinyGo Dockerfile, a cross-compiled go build, an image builder owning the
// final step — so those are deployments, and a flag only build understood would
// miss the path most likely to become production. What prepare cannot carry is
// the linker half, which belongs to the compile its caller owns.
func debugFlag(command string, args []string) (bool, error) {
	debug := false
	for _, arg := range args {
		switch arg {
		case "--debug":
			debug = true
		default:
			return false, fmt.Errorf("%s: unexpected argument %q; the only option is --debug", command, arg)
		}
	}
	return debug, nil
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
// sequence's, not the project's, and the source map decision is the same shape.
// It is a property of how the build was invoked, so it is written onto the
// config here rather than read from a file that has no way to say which
// invocation it meant.
func prepareBuildInputs(ctx context.Context, root string, config projectConfig, debug bool, progress *progressRegion, stdout, stderr io.Writer) error {
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
	config.Assets.SourceMaps = debug
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
