package pwcli

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// buildUsage shows the shared backend axis and build's deployment axis.
var buildUsage = "usage: pw build [--debug] [--backend nethttp|fasthttp] [--target lambda|azure-functions|google-cloud-run-functions|vercel-go]  |  pw prepare [--debug] [--backend nethttp|fasthttp]"

func runBuild(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	options, err := buildFlags("build", args)
	if err != nil {
		return err
	}
	root, config, err := buildProject("build")
	if err != nil {
		return err
	}
	if err := options.check(config); err != nil {
		return err
	}
	debug := options.debug
	progress := newProgressRegion(stdout)
	if err := prepareBuildInputs(ctx, root, config, options, progress, stdout, stderr); err != nil {
		progress.Done()
		return err
	}
	if options.target != "" {
		err := buildDeployment(ctx, root, config, options, progress, stdout, stderr)
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
	build = append(build, options.tags()...)
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
	options, err := buildFlags("prepare", args)
	if err != nil {
		return err
	}
	root, config, err := buildProject("prepare")
	if err != nil {
		return err
	}
	if err := options.check(config); err != nil {
		return err
	}
	progress := newProgressRegion(stdout)
	err = prepareBuildInputs(ctx, root, config, options, progress, stdout, stderr)
	progress.Done()
	return err
}

const (
	backendNetHTTP  = "nethttp"
	backendFastHTTP = "fasthttp"

	targetLambda                  = "lambda"
	targetAzureFunctions          = "azure-functions"
	targetGoogleCloudRunFunctions = "google-cloud-run-functions"
	targetVercelGo                = "vercel-go"
)

var deploymentTargets = map[string]bool{
	targetLambda:                  true,
	targetAzureFunctions:          true,
	targetGoogleCloudRunFunctions: true,
	targetVercelGo:                true,
}

// buildOptions are how a build was invoked. Backend selects the HTTP
// implementation; target selects provider packaging and is build-only.
type buildOptions struct {
	debug   bool
	backend string
	target  string
}

// tags is what the compiler is told. A net/http build passes none, so its
// command line is byte for byte what it was before the second target existed.
func (o buildOptions) tags() []string {
	if o.backend != backendFastHTTP {
		return nil
	}
	return []string{"-tags", backendFastHTTP}
}

// check refuses a backend the project did not declare.
//
// Building for fasthttp without project.fasthttp = true would compile the
// authored net/http source with everything it needs tagged out — a package
// with no handlers, no binders and no route registration, which fails as a
// pile of undefined symbols rather than as the one thing that is wrong.
func (o buildOptions) check(config projectConfig) error {
	if o.backend == backendFastHTTP && !config.FastHTTP {
		return fmt.Errorf("--backend %s needs project.fasthttp = true in popcornwave.toml; "+
			"without it nothing generates the half that build compiles", backendFastHTTP)
	}
	return nil
}

// buildFlags reads the build and prepare option set.
//
// It is on prepare as well as on build, and that is the point rather than a
// convenience: prepare exists for a compile this project does not run — the
// TinyGo Dockerfile, a cross-compiled go build, an image builder owning the
// final step — so those are deployments, and a flag only build understood would
// miss the path most likely to become production. What prepare cannot carry is
// the linker half, which belongs to the compile its caller owns.
func buildFlags(command string, args []string) (buildOptions, error) {
	options := buildOptions{backend: backendNetHTTP}
	for index := 0; index < len(args); index++ {
		switch arg := args[index]; {
		case arg == "--debug":
			options.debug = true
		case arg == "--backend":
			index++
			if index >= len(args) {
				return buildOptions{}, fmt.Errorf("%s: --backend needs a value", command)
			}
			options.backend = args[index]
		case strings.HasPrefix(arg, "--backend="):
			options.backend = strings.TrimPrefix(arg, "--backend=")
		case arg == "--target":
			index++
			if index >= len(args) {
				return buildOptions{}, fmt.Errorf("%s: --target needs a value", command)
			}
			options.target = args[index]
		case strings.HasPrefix(arg, "--target="):
			options.target = strings.TrimPrefix(arg, "--target=")
		default:
			return buildOptions{}, fmt.Errorf("%s: unexpected argument %q; %s", command, arg, buildUsage)
		}
		if options.backend != backendNetHTTP && options.backend != backendFastHTTP {
			return buildOptions{}, fmt.Errorf("%s: --backend %q is not a backend", command, options.backend)
		}
		if options.target != "" && !deploymentTargets[options.target] {
			return buildOptions{}, fmt.Errorf("%s: --target %q is not a target", command, options.target)
		}
	}
	if command == "prepare" && options.target != "" {
		return buildOptions{}, fmt.Errorf("prepare: --target is available only on pw build")
	}
	return options, nil
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
func prepareBuildInputs(ctx context.Context, root string, config projectConfig, options buildOptions, progress *progressRegion, stdout, stderr io.Writer) error {
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
	config.Assets.SourceMaps = options.debug
	report, err := buildDerivedAssets(root, config.Assets)
	if err != nil {
		return err
	}
	reportDerivedAssets(stdout, report)
	return rejectDevelopmentImports(ctx, root, config.Main, options)
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

// The graph is listed under the same tags the compile uses. A development-only
// package reached from the second transport's half would be invisible to a list
// taken without them, which is the one build this check would then miss.
func rejectDevelopmentImports(ctx context.Context, root, mainPackage string, options buildOptions) error {
	arguments := append([]string{"list", "-deps", "-f", "{{.ImportPath}}"}, options.tags()...)
	command := exec.CommandContext(ctx, "go", append(arguments, mainPackage)...)
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
