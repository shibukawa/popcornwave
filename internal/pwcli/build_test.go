package pwcli

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

// pw prepare exists so a build the framework does not drive can still get the
// tree a compiler needs. That only holds if it is reachable: the dispatcher and
// the help table are the two places a command is registered, and a command
// missing from the second is one nobody discovers.
func TestPrepareIsRegisteredInTheCommandList(t *testing.T) {
	var found bool
	for _, command := range commandSummaries {
		if command.name == "prepare" {
			found = true
			if command.summary == "" {
				t.Fatal("prepare has no summary line")
			}
		}
	}
	if !found {
		t.Fatal("prepare is missing from commandSummaries, so pw help omits it")
	}

	var usage bytes.Buffer
	printUsage(&usage)
	if !strings.Contains(usage.String(), "prepare") {
		t.Fatal("pw help does not name prepare")
	}

	var out, errOut bytes.Buffer
	if status := Main([]string{"prepare", "--nonsense"}, &out, &errOut); status == 0 {
		t.Fatal("the dispatcher did not reach prepare: a bad argument exited 0")
	}
}

// --backend selects the HTTP implementation for both commands. --target is a
// deployment destination and therefore belongs to build alone.
func TestBuildFlagsSeparateBackendAndTarget(t *testing.T) {
	for _, command := range []string{"build", "prepare"} {
		options, err := buildFlags(command, nil)
		if err != nil || options.debug || options.backend != "nethttp" || options.target != "" {
			t.Errorf("%s with no argument: %+v, err = %v", command, options, err)
		}
		options, err = buildFlags(command, []string{"--debug"})
		if err != nil || !options.debug {
			t.Errorf("%s --debug: %+v, err = %v", command, options, err)
		}
		// Both spellings, because a pipeline writes one and a person the other.
		for _, spelling := range [][]string{{"--backend", "fasthttp"}, {"--backend=fasthttp"}} {
			options, err = buildFlags(command, spelling)
			if err != nil || options.backend != "fasthttp" {
				t.Errorf("%s %v: %+v, err = %v", command, spelling, options, err)
			}
			if tags := options.tags(); len(tags) != 2 || tags[0] != "-tags" || tags[1] != "fasthttp" {
				t.Errorf("%s %v compiled with %v", command, spelling, tags)
			}
		}
		// A near miss is refused rather than ignored, because a pipeline that
		// meant to ask for a debug artifact and did not would ship the other one.
		if _, err := buildFlags(command, []string{"-debug"}); err == nil {
			t.Errorf("%s accepted -debug", command)
		}
		// An unknown target is refused rather than passed to the compiler as a
		// build tag nothing sets, which would silently produce the first
		// transport's binary under another name.
		if _, err := buildFlags(command, []string{"--backend", "valyala"}); err == nil {
			t.Errorf("%s accepted an unknown backend", command)
		}
		if _, err := buildFlags(command, []string{"--backend"}); err == nil {
			t.Errorf("%s accepted --backend with no value", command)
		}
	}
	for target := range deploymentTargets {
		options, err := buildFlags("build", []string{"--target", target, "--backend=fasthttp"})
		if err != nil || options.target != target || options.backend != "fasthttp" {
			t.Errorf("build target %s: %+v, err = %v", target, options, err)
		}
	}
	if _, err := buildFlags("prepare", []string{"--target=lambda"}); err == nil {
		t.Error("prepare accepted a deployment target")
	}
	if _, err := buildFlags("build", []string{"--target=fasthttp"}); err == nil {
		t.Error("fasthttp is still accepted as a target")
	}
}

// A net/http build passes no tags at all, so its command line is what it was
// before the second target existed.
func TestTheDefaultTargetCompilesWithNoTags(t *testing.T) {
	if tags := (buildOptions{debug: true}).tags(); len(tags) != 0 {
		t.Errorf("the default target compiled with %v", tags)
	}
}

// The second transport's half is emitted by generation, and generation emits it
// only for a project that declared it. Building for a target the project never
// declared would compile the authored source with everything it needs tagged
// out, which fails as a pile of undefined symbols rather than as the one thing
// that is wrong.
func TestTheFastHTTPBackendNeedsTheDeclaration(t *testing.T) {
	if err := (buildOptions{backend: "fasthttp"}).check(projectConfig{}); err == nil {
		t.Error("a fasthttp build was accepted without project.fasthttp")
	} else if !strings.Contains(err.Error(), "project.fasthttp") {
		t.Errorf("the refusal does not name the declaration: %v", err)
	}
	if err := (buildOptions{backend: "fasthttp"}).check(projectConfig{FastHTTP: true}); err != nil {
		t.Errorf("a declared fasthttp build was refused: %v", err)
	}
	if err := (buildOptions{}).check(projectConfig{}); err != nil {
		t.Errorf("the default target was refused: %v", err)
	}
}

// Neither command takes a positional argument. Accepting one silently would let
// a caller believe a package or an output path was honoured.
func TestBuildAndPrepareRejectArguments(t *testing.T) {
	ctx := context.Background()
	var out, errOut bytes.Buffer
	if err := runPrepare(ctx, []string{"./cmd/app"}, &out, &errOut); err == nil {
		t.Error("prepare accepted an argument")
	} else if !strings.Contains(err.Error(), "prepare") {
		t.Errorf("the error does not name the command: %v", err)
	}
	if err := runBuild(ctx, []string{"./cmd/app"}, &out, &errOut); err == nil {
		t.Error("build accepted an argument")
	} else if !strings.Contains(err.Error(), "build") {
		t.Errorf("the error does not name the command: %v", err)
	}
}

// buildProject is what makes the two commands refuse the same projects for the
// same reasons. A prepare that got further than build in a package would report
// the missing entry point instead of the kind.
func TestBuildProjectRefusesAPackageForEitherCommand(t *testing.T) {
	t.Chdir(t.TempDir())
	for _, command := range []string{"build", "prepare"} {
		if _, _, err := buildProject(command); err == nil {
			t.Fatalf("%s: a directory with no project was accepted", command)
		}
	}
}
