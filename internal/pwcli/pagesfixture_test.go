package pwcli

import (
	"bytes"
	"context"
	"flag"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/popcornwave/internal/pwgen"
	"github.com/shibukawa/tinybind-go/generator"
)

// updateFixture rewrites the committed generated files of the page tree fixture
// instead of comparing against them, for the one case where an emitter setting
// or a generation template changed on purpose:
//
//	go test ./internal/pwcli -run PagesFixture -update
var updateFixture = flag.Bool("update", false, "rewrite the committed page tree fixture")

// fixtureConfig is the project shape of internal/pagesfixture: one page tree
// and nothing else. The fixture has no popcornwave.toml of its own, because a
// second project root inside this module would be found by any command run from
// a subdirectory.
func fixtureConfig(t *testing.T) (string, projectConfig) {
	t.Helper()
	// Absolute, because that is what projectRoot hands the generation path: a
	// directory reached by two spellings is planned twice, and the plan that
	// resolves no purpose deletes what the other one wrote.
	root, err := filepath.Abs(filepath.Join("..", "pagesfixture"))
	if err != nil {
		t.Fatal(err)
	}
	return root, projectConfig{Generate: generationScope{Pages: []string{"pages"}}}
}

// The generated files of the fixture are committed, so a change to the emitter
// or to an upstream template shows up as a diff a reviewer reads rather than as
// behavior nobody looked at. Generating them again must produce what is already
// there, which is also what makes pw generate --check meaningful.
//
// This drives the production path: the same tree run and the same per-directory
// plan pw generate uses, including the merge that puts a compiled component and
// a request binder deriving one base name into one file.
func TestPagesFixtureGeneratedFilesAreCurrent(t *testing.T) {
	root, config := fixtureConfig(t)
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
		t.Errorf("the page tree fixture is stale; run go test ./internal/pwcli -run PagesFixture -update\n  %v",
			changePaths(root, changes))
	}
}

func planFixture(t *testing.T, root string, config projectConfig) ([]fileChange, error) {
	t.Helper()
	pageArtifacts, err := planPageTrees(root, config)
	if err != nil {
		return nil, err
	}
	directories, err := packageDirectories(root, config.Generate)
	if err != nil {
		return nil, err
	}
	options, err := pwgen.Options(engineFor(config.Database).SQLDialect)
	if err != nil {
		return nil, err
	}
	directories, err = withPageDirectories(directories, pageArtifacts)
	if err != nil {
		return nil, err
	}
	if config.FastHTTP {
		transform := pwgen.FastTransform(options.Calls.Set)
		options.Transform = &transform
	}
	runner := generator.New(options)
	var changes []fileChange
	for _, directory := range directories {
		planned, err := planDirectory(context.Background(), runner, directory,
			directoryPurposes(root, config.Generate, directory), pageArtifacts[directory], config.FastHTTP)
		if err != nil {
			return nil, err
		}
		changes = append(changes, planned...)
	}
	// Plan-only, which is check mode.
	return planSecondBuildPages(root, config, true, changes)
}

// A project declaring the fasthttp build gets two things and nothing else: its
// net/http-bearing generated files constrained out of that build, and the
// handlers it has to supply for itself derived into it.
//
// The page tree fixture is the case that proves the first is per file rather
// than per kind: its route, registry, action, and page files name net/http, and
// its compiled layout does not. The committed files carry no constraint, so
// turning the option on plans exactly the net/http ones plus the derived
// handlers — which makes the set of planned changes itself the assertion, and
// the absence of the layout the other half of it.
func TestFastHTTPBuildConstrainsOnlyTheGeneratedFilesNamingNetHTTP(t *testing.T) {
	root, config := fixtureConfig(t)
	config.FastHTTP = true
	changes, err := planFixture(t, root, config)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) == 0 {
		t.Fatal("turning the option on planned nothing, so this proves nothing")
	}
	planned := map[string]bool{}
	for _, change := range changes {
		if change.remove {
			t.Errorf("%s was planned for removal; the option must not delete anything", change.path)
			continue
		}
		name := path.Base(filepath.ToSlash(change.path))
		planned[name] = true
		// Read the whole header rather than the first line: the generator writes
		// its own constraint below the generated-code header, and this framework
		// writes its above one.
		constraint, _ := buildConstraint(change.source)
		if constraint == strings.TrimSpace(fastHTTPConstraint) {
			// The derived half. It is generated for the second build rather than
			// constrained out of it, so the rule below does not apply.
			//
			// net/http is not the test here: a derived binder names it for the
			// status constants and is still fasthttp-only code. What must not
			// survive is the first transport's runtime and its value types.
			for _, first := range []string{`"github.com/shibukawa/popcornwave/pw"`, "http.ResponseWriter", "*http.Request"} {
				if bytes.Contains(change.source, []byte(first)) {
					t.Errorf("%s is generated for the fasthttp build and names %s", change.path, first)
				}
			}
			continue
		}
		if !bytes.Contains(change.source, []byte(`"net/http"`)) {
			t.Errorf("%s does not import net/http yet the option planned a change to it", change.path)
		}
		if constraint != strings.TrimSpace(netHTTPConstraint) {
			t.Errorf("%s imports net/http and carries no build constraint", change.path)
		}
	}
	// The compiled layout is generated from the same tree in the same run and
	// names no transport, so a change to it would mean the constraint was
	// applied to everything generation touches rather than read from the file.
	if planned["layout_pw_gen.go"] {
		t.Error("layout_pw_gen.go imports no net/http yet was constrained out of the fasthttp build")
	}
	// Nothing above would fail if the emitter stopped producing the route files
	// altogether, so name one of each half that must be there.
	if !planned["route_pw_gen.go"] {
		t.Errorf("expected the route decoder among the constrained files; got %v", planned)
	}
	for _, name := range []string{"tinybind_transport_pw_gen.go", "tinybind_fasthttp_pw_gen.go"} {
		if !planned[name] {
			t.Errorf("%s is missing, so the second build has no %s; got %v", name,
				map[string]string{
					"tinybind_transport_pw_gen.go": "derived server action",
					"tinybind_fasthttp_pw_gen.go":  "binders to serve it",
				}[name], planned)
		}
	}
}
