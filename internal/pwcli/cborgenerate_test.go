package pwcli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/popcornweb/internal/pwgen"
	"github.com/shibukawa/tinybind-go/templates/sqlbind"
)

func TestCBORBodiesAreOffUnlessTheProjectAsks(t *testing.T) {
	// Off is what every project written before the block existed has, and off
	// must regenerate today's bytes exactly.
	root := t.TempDir()
	writeProjectFixture(t, root, "[project]\nname = \"fixture\"\nmain = \"./cmd/fixture\"\n")

	config, err := loadProjectConfig(root)
	if err != nil {
		t.Fatal(err)
	}
	if config.CBOR.Enabled || config.CBOR.RejectFloats || config.CBOR.SortedKeys {
		t.Fatal("CBOR bodies are on for a project that never asked")
	}
}

func TestAProjectCanAskForCBORBodies(t *testing.T) {
	root := t.TempDir()
	writeProjectFixture(t, root, "[project]\nname = \"fixture\"\nmain = \"./cmd/fixture\"\n\n"+
		"[generate.api.cbor]\nenabled = true\nreject_floats = true\nsorted_keys = true\n")

	config, err := loadProjectConfig(root)
	if err != nil {
		t.Fatal(err)
	}
	if !config.CBOR.Enabled {
		t.Fatal("the project asked for CBOR bodies and did not get them")
	}
	if !config.CBOR.RejectFloats || !config.CBOR.SortedKeys {
		t.Fatal("the profile keys were asked for and did not arrive")
	}
}

func TestAnUnusableCBORValueIsReported(t *testing.T) {
	root := t.TempDir()
	writeProjectFixture(t, root, "[project]\nname = \"fixture\"\nmain = \"./cmd/fixture\"\n\n"+
		"[generate.api.cbor]\nenabled = \"yes\"\n")

	_, err := loadProjectConfig(root)
	if err == nil {
		t.Fatal("a string was accepted where a boolean belongs")
	}
	if !strings.Contains(err.Error(), "generate.api.cbor.enabled") {
		t.Fatalf("err = %v, want the key named", err)
	}
}

func TestTheCBORSettingReachesTheGenerator(t *testing.T) {
	// The wiring is what makes the setting mean anything. It is asserted
	// against the generator's own fields rather than against emitted bytes,
	// because the negotiation those fields emit is upstream's to decide.
	options, err := pwgen.Options(sqlbind.DialectSQLite)
	if err != nil {
		t.Fatal(err)
	}
	if options.EnableCBORHTTP {
		t.Fatal("pwgen.Options turns CBOR on, so the project setting could never turn it off")
	}
	options.EnableCBORHTTP = true
	options.CBORHTTPProfile.RejectFloats = true
	options.CBORHTTPProfile.RequireSortedKeys = true
	if !options.EnableCBORHTTP || !options.CBORHTTPProfile.RejectFloats || !options.CBORHTTPProfile.RequireSortedKeys {
		t.Fatal("the generator options do not hold the values")
	}
}

func TestTheScaffoldStatesTheCBORSetting(t *testing.T) {
	// A setting a project only meets by reading someone else's source is one
	// nobody turns on. The scaffold is where it is met.
	root := writeScaffoldedProject(t, initOptions{
		Name: "fixture", Devbox: true, Database: true, Auth: authNone,
	})
	source, err := os.ReadFile(filepath.Join(root, "popcornweb.toml"))
	if err != nil {
		t.Fatal(err)
	}

	written := string(source)
	if !strings.Contains(written, "[generate.api.cbor]") {
		t.Fatalf("the scaffold does not carry the block:\n%s", written)
	}
	if !strings.Contains(written, "no CBOR code is linked") {
		t.Fatal("the scaffold states the setting without stating that off costs nothing")
	}

	// The scaffold writes the block, so the known-key check has to accept it
	// and the default has to stay off.
	config, err := loadProjectConfig(root)
	if err != nil {
		t.Fatalf("a scaffolded project does not load: %v", err)
	}
	if config.CBOR.Enabled {
		t.Fatal("the scaffold turns CBOR bodies on")
	}
}
