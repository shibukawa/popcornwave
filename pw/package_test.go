package pw

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

// resetPackages clears the registry so one test's packages cannot reach another.
// The asset table is built once per process, so a test that needs it rebuilt
// calls buildPackageAssetTable directly rather than through the cached entry.
func resetPackages(t *testing.T) {
	t.Helper()
	packageState.Lock()
	previous := packageState.registered
	packageState.registered = nil
	packageState.Unlock()
	t.Cleanup(func() {
		packageState.Lock()
		packageState.registered = previous
		packageState.Unlock()
	})
}

func TestRegisterPackageOrdersByModule(t *testing.T) {
	resetPackages(t)
	RegisterPackage(Package{Module: "example.com/zeta"})
	RegisterPackage(Package{Module: "example.com/alpha"})
	packages := Packages()
	if len(packages) != 2 {
		t.Fatalf("packages = %#v", packages)
	}
	// The order is stable so a report of what the binary linked does not move
	// between runs.
	if packages[0].Module != "example.com/alpha" || packages[1].Module != "example.com/zeta" {
		t.Fatalf("packages = %#v", packages)
	}
}

func TestRegisterPackageRejectsDuplicateModule(t *testing.T) {
	resetPackages(t)
	RegisterPackage(Package{Module: "example.com/one"})
	defer func() {
		if recover() == nil {
			t.Fatal("registering one module twice was accepted")
		}
	}()
	RegisterPackage(Package{Module: "example.com/one"})
}

func TestRegisterPackageRejectsSharedMigrationStem(t *testing.T) {
	resetPackages(t)
	stream := fstest.MapFS{"00001_init.sql": &fstest.MapFile{Data: []byte("select 1;")}}
	RegisterPackage(Package{Module: "example.com/one", Migrations: stream, MigrationStem: "shared"})
	defer func() {
		// Two packages sharing a stem would write into one version table, so
		// each would read the other's applied versions as its own.
		if recover() == nil {
			t.Fatal("two packages shared a migration stem")
		}
	}()
	RegisterPackage(Package{Module: "example.com/two", Migrations: stream, MigrationStem: "shared"})
}

func TestRegisterPackageRejectsMigrationsWithoutStem(t *testing.T) {
	resetPackages(t)
	defer func() {
		if recover() == nil {
			t.Fatal("a stream with no stem was accepted")
		}
	}()
	RegisterPackage(Package{
		Module:     "example.com/one",
		Migrations: fstest.MapFS{"00001_init.sql": &fstest.MapFile{Data: []byte("select 1;")}},
	})
}

func TestPackageAssetKeyDedupesIdenticalBytes(t *testing.T) {
	// Identity is the digest, so two packages shipping the same file produce one
	// entry and one URL rather than two copies a browser fetches separately.
	first := packageAssetKey([]byte("body{}"), "app.css")
	second := packageAssetKey([]byte("body{}"), "app.css")
	if first != second {
		t.Fatalf("%q != %q", first, second)
	}
	// A changed byte changes the URL, which is what keeps the immutable cache
	// header honest.
	if changed := packageAssetKey([]byte("body{ }"), "app.css"); changed == first {
		t.Fatal("a changed file kept its URL")
	}
}

func TestPackageAssetURLAndServe(t *testing.T) {
	resetPackages(t)
	RegisterPackage(Package{
		Module: "example.com/widget",
		Assets: fstest.MapFS{"widget.js": &fstest.MapFile{Data: []byte("export const a = 1;")}},
	})
	url, ok := PackageAssetURL("example.com/widget", "widget.js")
	if !ok {
		t.Fatal("the registered asset has no URL")
	}
	if got := url; got[:len(packageAssetPrefix)] != packageAssetPrefix {
		t.Fatalf("url = %q, want the reserved prefix", got)
	}
	if _, ok := PackageAssetURL("example.com/widget", "missing.js"); ok {
		t.Fatal("an unregistered name produced a URL")
	}

	// The table is process-wide and built once, so this exercises the lookup
	// against a table built from this test's registration.
	table := map[string]packageAsset{}
	for key, asset := range buildPackageAssetTable() {
		table[key] = asset
	}
	if len(table) != 1 {
		t.Fatalf("table = %#v", table)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, url, nil)
	if !servePackageAssetFrom(table, recorder, request) {
		t.Fatal("the asset request was not handled")
	}
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}
	if body := recorder.Body.String(); body != "export const a = 1;" {
		t.Fatalf("body = %q", body)
	}
	if control := recorder.Header().Get("Cache-Control"); control != "public, max-age=31536000, immutable" {
		t.Fatalf("cache control = %q", control)
	}
	if mediaType := recorder.Header().Get("Content-Type"); mediaType == "" {
		t.Fatal("no content type")
	}
}

func TestServePackageAssetLeavesUnknownPathsToTheNamespaceClose(t *testing.T) {
	// An unclaimed path under the reserved prefix is closed with a 404 in one
	// place, so this handler declines rather than answering for the namespace.
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, packageAssetPrefix+"deadbeef/missing.js", nil)
	if servePackageAssetFrom(map[string]packageAsset{}, recorder, request) {
		t.Fatal("an unknown package asset path was claimed")
	}
	if !serveReservedPath(httptest.NewRecorder(), request) {
		t.Fatal("the reserved namespace did not close the path")
	}
}

func TestServePackageAssetIgnoresOtherPaths(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/widget.js", nil)
	if servePackageAssetFrom(map[string]packageAsset{}, recorder, request) {
		t.Fatal("a path outside the prefix was claimed")
	}
}
