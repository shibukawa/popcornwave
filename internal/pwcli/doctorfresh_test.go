package pwcli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/popcornweb/internal/pwcheck"
	"github.com/shibukawa/popcornweb/internal/pwgen"
)

// A page tree route is a directory, so the two files generated per route
// package come from the directory rather than from a source beside them. Read
// as ordinary generated files they look like artifacts whose source was
// deleted, and pw doctor told every discovered-routing project to delete the
// registrations that serve it — on the day the project was created.
func TestDoctorReadsPageTreeOutputAsAPackageArtifact(t *testing.T) {
	root := t.TempDir()
	tree := filepath.Join(root, "pages", "greet", "name_")
	if err := os.MkdirAll(tree, 0o755); err != nil {
		t.Fatal(err)
	}
	// Beside a page template, which is what makes the directory a route.
	writeTestFile(t, filepath.Join(tree, pwgen.PageFile), "package name_\n")
	for _, name := range []string{pwgen.PageDecoderOutput, pwgen.PageRegistryOutput} {
		writeTestFile(t, filepath.Join(tree, name), "package name_\n")
	}
	scan := &projectScan{root: root, gitTracked: map[string]bool{}, configFileModes: map[string]os.FileMode{}, devboxVersions: map[string]string{}}
	scan.scanGenerated()

	for _, generated := range scan.generated {
		if generated.Orphan {
			t.Errorf("%s is reported as having outlived its source", generated.Path)
		}
	}
}

// devbox pins tailwindcss under a name carrying its major version, which is
// what pw add writes. Asking for the bare name told a project that took the
// scaffold's own answer that its toolchain was missing.
func TestDoctorSeesTheTailwindPinTheScaffoldWrites(t *testing.T) {
	scan := &projectScan{devboxVersions: map[string]string{}}
	scan.scanDevbox(`{"packages": ["go@latest", "` + tailwindDevboxPackage + `"]}`)
	if !scan.devboxPins("tailwindcss") {
		t.Errorf("the scaffolded pin %q is not recognised: %#v", tailwindDevboxPackage, scan.devboxVersions)
	}
	if scan.devboxPins("valkey") {
		t.Error("a package that is not pinned is reported as pinned")
	}
}

// Every session backend pw init offers writes its own import, and the SQL one
// is a package per engine. A check with a table of its own listed rdb-on-SQLite
// and nothing else, so a project taking any other answer was told its backend
// was unregistered while its main package was importing the plugin.
func TestDoctorNamesThePluginEachSessionBackendNeeds(t *testing.T) {
	for _, testcase := range []struct {
		backend, engine, want string
	}{
		{backend: sessionRDB, engine: engineSQLite, want: "sessionstore/sqlite"},
		{backend: sessionRDB, engine: enginePostgres, want: "sessionstore/postgres"},
		{backend: sessionRedis, engine: engineSQLite, want: "sessionstore/redis"},
		{backend: sessionDynamo, engine: engineSQLite, want: "sessionstore/dynamo"},
	} {
		pkg := sessionBackendPackage(testcase.backend, testcase.engine)
		if !strings.HasSuffix(pkg, testcase.want) {
			t.Errorf("backend %q on %q resolves to %q, want one ending %q",
				testcase.backend, testcase.engine, pkg, testcase.want)
		}
		// A project linking that package is wired, and must be reported so.
		run := checkRun{checkContext: checkContext{
			Env:    "dev",
			Config: environmentConfig{Env: "dev", Values: map[string]configValue{"session.enabled": {Raw: "true"}, "session.backend": {Raw: testcase.backend}}},
			Graph:  importGraph{packages: map[string]bool{pkg: true}},
			State:  projectState{config: projectConfig{Database: testcase.engine}},
		}}
		run.checkWiring()
		for _, finding := range run.findings {
			if finding.Check.ID == pwcheck.MissingSessionPlugin {
				t.Errorf("backend %q on %q is linked and still reported: %s",
					testcase.backend, testcase.engine, finding.Evidence)
			}
		}
	}
}

// The development and cookie backends are pw itself. Reported as a plugin that
// nothing registers, the default a scaffolded project starts on reads like a
// fault in the report that is supposed to confirm it.
func TestDoctorReportsTheBuiltInSessionBackendsAsBuiltIn(t *testing.T) {
	for _, backend := range []string{"dev-volatile", "dev-persist", "cookie"} {
		config := environmentConfig{Env: "dev", Values: map[string]configValue{"session.backend": {Raw: backend}}}
		implementation := featureImplementation(
			doctorFeature{Name: "session", State: "on"}, importGraph{}, config, engineSQLite)
		if implementation != "built into pw" {
			t.Errorf("backend %q is described as %q", backend, implementation)
		}
	}
}

// A store declaration is a generation source like a template or a query, and
// its output sits beside it under the same stem. Left out of the source kinds
// the scan knows, the file that serves a declared access pattern is reported as
// one whose source was deleted.
func TestDoctorPairsStoreOutputWithItsDeclaration(t *testing.T) {
	root := t.TempDir()
	for stem, source := range map[string]string{
		"notes":   "notes.pw.dynamo",
		"entries": "entries.pw.firestore",
	} {
		writeTestFile(t, filepath.Join(root, source), "// declaration\n")
		writeTestFile(t, filepath.Join(root, stem+generatedSuffix), "package records\n")
	}
	scan := &projectScan{root: root, gitTracked: map[string]bool{}, configFileModes: map[string]os.FileMode{}, devboxVersions: map[string]string{}}
	scan.scanGenerated()

	if len(scan.generated) == 0 {
		t.Fatal("the scan found no generated files")
	}
	for _, generated := range scan.generated {
		if generated.Orphan {
			t.Errorf("%s is reported as having outlived its source", generated.Path)
		}
	}
}
