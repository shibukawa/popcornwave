package pwcli

import (
	"bytes"
	"context"
	"flag"
	"os"
	"path"
	"path/filepath"
	"regexp"
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
	runner := generator.New(withExtractedAssetDirs(options, root))
	directories, err = withPageDirectories(directories, pageArtifacts)
	if err != nil {
		return nil, err
	}
	var changes []fileChange
	for _, directory := range directories {
		planned, err := planDirectory(context.Background(), runner, directory,
			directoryPurposes(root, config.Generate, directory), pageArtifacts[directory], config.FastHTTP)
		if err != nil {
			return nil, err
		}
		changes = append(changes, planned...)
	}
	return changes, nil
}

// A project declaring the fasthttp build gets its net/http-bearing generated
// files constrained out of it, and only those.
//
// The page tree fixture is the case that proves the split is per file rather
// than per kind: its route, registry, action, and page files name net/http, and
// its compiled layout does not. The committed files carry no constraint, so
// turning the option on plans exactly the net/http ones — which makes the set of
// planned changes itself the assertion, and the absence of the layout the other
// half of it.
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
		planned[path.Base(filepath.ToSlash(change.path))] = true
		if !bytes.Contains(change.source, []byte(`"net/http"`)) {
			t.Errorf("%s does not import net/http yet the option planned a change to it", change.path)
		}
		if !bytes.HasPrefix(change.source, []byte(netHTTPConstraint)) {
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
	// altogether, so name one that must be there.
	if !planned["route_pw_gen.go"] {
		t.Errorf("expected the route decoder among the constrained files; got %v", planned)
	}
}

// A page tree's own templates extract assets like any other, and the tree run
// returns them in a list of its own. Losing them writes a page that references a
// module answering 404, and losing them into the Go grouping is worse: it names
// the file _pw_gen.go and the next run refuses to parse JavaScript as Go.
//
// Both happened while wiring this, which is why the reference and the file on
// disk are checked against each other rather than either alone.
func TestPageTreeWritesItsExtractedScript(t *testing.T) {
	root, _ := fixtureConfig(t)
	component, err := os.ReadFile(filepath.Join(root, "pages", "users", "id_", "page_pw_gen.go"))
	if err != nil {
		t.Fatal(err)
	}
	reference := regexp.MustCompile(`URL: "([^"]+\.js)"`).FindSubmatch(component)
	if reference == nil {
		t.Skip("the fixture page declares no component script block")
	}
	url := string(reference[1])

	// The URL the page will serve, resolved to where this project writes it.
	if !strings.HasPrefix(url, generator.DefaultPublicURLBase+"/") {
		t.Fatalf("asset URL %q is not under the public base the project serves", url)
	}
	name := path.Base(url)
	written := filepath.Join(root, filepath.FromSlash(extractedAssetDir), name)
	if _, err := os.Stat(written); err != nil {
		t.Fatalf("the page references %s and no file was written for it: %v", url, err)
	}

	// And nothing put JavaScript into a Go file on the way.
	strays, err := filepath.Glob(filepath.Join(root, "*script*_pw_gen.go"))
	if err != nil {
		t.Fatal(err)
	}
	if len(strays) > 0 {
		t.Errorf("an extracted script was written as Go: %v", strays)
	}
}
