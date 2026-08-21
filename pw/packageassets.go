package pw

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

// packageAssetPrefix is where a component package's embedded browser files are
// served. It sits under the reserved framework prefix rather than under the
// configurable public mount, for the same reason the framework's own scripts do:
// a package asset is no more an application asset than a framework one is, the
// application mount has no say over it, and it stays reachable when public asset
// delivery is switched off entirely.
const packageAssetPrefix = frameworkScriptPrefix + "pkg/"

// packageAsset is one served file. Identity is the digest of the bytes, so two
// packages embedding an identical file produce one URL and one entry, and a
// changed file produces a different URL rather than a stale cache hit.
type packageAsset struct {
	source    fs.FS
	path      string
	size      int64
	mediaType string
}

type packageAssetCatalog struct {
	byKey  map[string]packageAsset
	byName map[string]string
}

// packageAssetSnapshot is one built catalog together with the registration
// generation it was built from, published as a unit so a reader never pairs one
// with the other's moment.
type packageAssetSnapshot struct {
	generation uint64
	catalog    packageAssetCatalog
}

// packageAssetState normally builds once after package initialization. The
// generation also preserves RegisterPackage's existing behavior for programs
// and tests that register before their first asset lookup at a later point.
//
// The current snapshot is read through an atomic pointer rather than under the
// mutex, because the read side is request-serving: the lock's only job is to
// keep concurrent rebuilds from doing the walk twice.
var packageAssetState struct {
	sync.Mutex
	snapshot atomic.Pointer[packageAssetSnapshot]
}

func packageAssets() packageAssetCatalog {
	generation := packageGeneration.Load()
	if snapshot := packageAssetState.snapshot.Load(); snapshot != nil && snapshot.generation == generation {
		return snapshot.catalog
	}
	packageAssetState.Lock()
	defer packageAssetState.Unlock()
	if snapshot := packageAssetState.snapshot.Load(); snapshot != nil && snapshot.generation == generation {
		return snapshot.catalog
	}
	snapshot := &packageAssetSnapshot{generation: generation, catalog: buildPackageAssetCatalog()}
	packageAssetState.snapshot.Store(snapshot)
	return snapshot.catalog
}

// buildPackageAssetTable maps the digest path segment plus name to the file.
// Serving is table driven: nothing walks or hashes an fs.FS per request. The
// selected embedded file is opened only long enough to stream that response,
// which avoids retaining a second copy of every asset in heap memory.
func buildPackageAssetTable() map[string]packageAsset {
	return buildPackageAssetCatalog().byKey
}

func buildPackageAssetCatalog() packageAssetCatalog {
	catalog := packageAssetCatalog{
		byKey:  make(map[string]packageAsset),
		byName: make(map[string]string),
	}
	for _, pkg := range Packages() {
		if pkg.Assets == nil {
			continue
		}
		_ = fs.WalkDir(pkg.Assets, ".", func(name string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() {
				return nil
			}
			file, openErr := pkg.Assets.Open(name)
			if openErr != nil {
				// A file the walk found and the read refuses is a broken embed
				// rather than a request-time condition. Skipping keeps startup
				// from failing over one asset, and the missing URL is what the
				// package's own release check exists to catch.
				return nil
			}
			key, hashErr := packageAssetKeyFromReader(file, path.Base(name))
			closeErr := file.Close()
			if hashErr != nil || closeErr != nil {
				return nil
			}
			info, infoErr := entry.Info()
			if infoErr != nil {
				return nil
			}
			if _, exists := catalog.byKey[key]; !exists {
				catalog.byKey[key] = packageAsset{
					source:    pkg.Assets,
					path:      name,
					size:      info.Size(),
					mediaType: packageAssetMediaType(name),
				}
			}
			catalog.byName[packageAssetName(pkg.Module, name)] = key
			return nil
		})
	}
	return catalog
}

// packageAssetKey is the path below the prefix: the content digest, then the
// base name. The name is kept because a browser, a source map, and a developer
// reading a network panel all want to see what the file is, and the digest alone
// would show none of that.
func packageAssetKey(data []byte, name string) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])[:16] + "/" + name
}

func packageAssetKeyFromReader(reader io.Reader, name string) (string, error) {
	digest := sha256.New()
	if _, err := io.Copy(digest, reader); err != nil {
		return "", err
	}
	return hex.EncodeToString(digest.Sum(nil))[:16] + "/" + name, nil
}

func packageAssetName(module, name string) string { return module + "\x00" + name }

// packageAssetMediaType answers from the file extension only. The bytes are
// chosen at build time and never sniffed, so a package cannot make the framework
// serve one thing as another.
func packageAssetMediaType(name string) string {
	if mediaType := mime.TypeByExtension(path.Ext(name)); mediaType != "" {
		return mediaType
	}
	return "application/octet-stream"
}

// PackageAssetURL returns the absolute path a package asset is served at, and
// reports whether the package registered a file with that name.
//
// A package never chooses its own URL: only the framework knows the reserved
// prefix, and only the bytes decide the digest. This is the lookup an
// application uses while a component cannot yet declare the asset it needs.
func PackageAssetURL(module, name string) (string, bool) {
	key, ok := packageAssets().byName[packageAssetName(module, name)]
	if !ok {
		return "", false
	}
	return packageAssetPrefix + key, true
}

// servePackageAsset answers a package asset request and reports whether it
// handled the request. It runs above serveReservedPath, which closes the
// namespace below every handler that owns something inside it.
//
// The prefix is tested before the catalog is resolved: this runs on every
// request in the chain, and only a request actually under the prefix should
// pay for the lookup.
func servePackageAsset(w http.ResponseWriter, r *http.Request) bool {
	if !strings.HasPrefix(r.URL.Path, packageAssetPrefix) {
		return false
	}
	return servePackageAssetFrom(packageAssets().byKey, w, r)
}

func servePackageAssetFrom(table map[string]packageAsset, w http.ResponseWriter, r *http.Request) bool {
	if !strings.HasPrefix(r.URL.Path, packageAssetPrefix) {
		return false
	}
	asset, ok := table[strings.TrimPrefix(r.URL.Path, packageAssetPrefix)]
	if !ok {
		// Not handled here: an unclaimed path under the reserved prefix is
		// closed with a 404 by serveReservedPath, in one place rather than in
		// each handler that owns part of the namespace.
		return false
	}
	if !operationalMethod(w, r) {
		return true
	}
	header := w.Header()
	header.Set("Content-Type", asset.mediaType)
	// A digest segment never serves different bytes, so this is genuinely
	// immutable rather than merely long-lived.
	header.Set("Cache-Control", "public, max-age=31536000, immutable")
	header.Set("Content-Length", strconv.FormatInt(asset.size, 10))
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return true
	}
	file, err := asset.source.Open(asset.path)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return true
	}
	defer file.Close()
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, file)
	return true
}
