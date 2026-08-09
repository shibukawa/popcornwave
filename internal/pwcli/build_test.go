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

// --debug is the one option either command takes, and it has to be on both.
// prepare hands its tree to a compiler this project does not run, which is the
// container path, so a debug artifact built that way would otherwise be
// unreachable.
func TestDebugFlagIsTheOnlyOption(t *testing.T) {
	for _, command := range []string{"build", "prepare"} {
		debug, err := debugFlag(command, nil)
		if err != nil || debug {
			t.Errorf("%s with no argument: debug = %v, err = %v", command, debug, err)
		}
		debug, err = debugFlag(command, []string{"--debug"})
		if err != nil || !debug {
			t.Errorf("%s --debug: debug = %v, err = %v", command, debug, err)
		}
		// A near miss is refused rather than ignored, because a pipeline that
		// meant to ask for a debug artifact and did not would ship the other one.
		if _, err := debugFlag(command, []string{"-debug"}); err == nil {
			t.Errorf("%s accepted -debug", command)
		}
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
