package pwcli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// fastFixtureConfig is the project shape of internal/fastfixture: one handler
// directory, and the second build declared.
func fastFixtureConfig(t *testing.T) (string, projectConfig) {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "fastfixture"))
	if err != nil {
		t.Fatal(err)
	}
	return root, projectConfig{
		Generate: generationScope{Handlers: []string{"."}},
		FastHTTP: true,
	}
}

// The generated files of both builds are committed, so a change to the
// derivation or to an upstream emitter shows up as a diff a reviewer reads.
//
//	go test ./internal/pwcli -run FastFixture -update
func TestFastFixtureGeneratedFilesAreCurrent(t *testing.T) {
	root, config := fastFixtureConfig(t)
	changes, err := planFixture(t, root, config)
	if err != nil {
		t.Fatal(err)
	}
	if *updateFixture {
		if err := applyFileChanges(changes); err != nil {
			t.Fatal(err)
		}
		for _, path := range changePaths(root, changes) {
			t.Logf("wrote %s", path)
		}
		return
	}
	if len(changes) > 0 {
		t.Errorf("the second build fixture is stale; run go test ./internal/pwcli -run FastFixture -update\n  %v",
			changePaths(root, changes))
	}
}

// One authored package, two builds, each compiled on its own.
//
// Everything else about the second build is a claim about the source that was
// produced. This is the one that would catch two halves that each look right
// and do not fit: a derived handler calling a binder nobody registered, a
// constraint that admits one file into both builds, a shared declaration tagged
// out from under the half that needs it.
func TestBothBuildsOfTheFixtureCompile(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles two configurations of a package")
	}
	root, _ := fastFixtureConfig(t)
	for _, tags := range []string{"", "fasthttp"} {
		args := []string{"build", "-o", os.DevNull}
		if tags != "" {
			args = append(args, "-tags", tags)
		}
		command := exec.Command("go", append(args, ".")...)
		command.Dir = root
		if output, err := command.CombinedOutput(); err != nil {
			t.Errorf("go build -tags %q failed: %v\n%s", tags, err, output)
		}
	}
}

// Both builds are represented, and each generated file belongs to one of them
// or to neither.
//
// The compile test above proves each configuration holds together; this one
// proves there are two of them. Without it a run that emitted nothing for the
// second transport would compile perfectly, because a build with no derived
// half in it is just the first build.
//
// The transport axis is not the only one a generated file can carry — the
// OpenAPI document has its own, and belongs to both builds — so this reads the
// constraint rather than assuming what it says.
func TestBothBuildsAreRepresentedInTheFixture(t *testing.T) {
	root, _ := fastFixtureConfig(t)
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	first, second := 0, 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), "_pw_gen.go") {
			continue
		}
		source, err := os.ReadFile(filepath.Join(root, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		constraint, ok := buildConstraint(source)
		if !ok {
			// A generated file naming no transport belongs to both builds, which
			// is correct and is why the constraint is decided per file.
			continue
		}
		switch constraint {
		case strings.TrimSpace(netHTTPConstraint):
			first++
		case strings.TrimSpace(fastHTTPConstraint):
			second++
		}
	}
	if first == 0 {
		t.Error("no generated file is constrained to the net/http build")
	}
	if second == 0 {
		t.Error("no generated file is constrained to the fasthttp build, so the second build is empty")
	}
}
