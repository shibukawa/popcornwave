package pwcli

import (
	"context"
	"flag"
	"path/filepath"
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
	runner := generator.New(options)
	directories, err = withPageDirectories(directories, pageArtifacts)
	if err != nil {
		return nil, err
	}
	var changes []fileChange
	for _, directory := range directories {
		planned, err := planDirectory(context.Background(), runner, directory,
			directoryPurposes(root, config.Generate, directory), pageArtifacts[directory])
		if err != nil {
			return nil, err
		}
		changes = append(changes, planned...)
	}
	return changes, nil
}
