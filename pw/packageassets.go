package pw

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strconv"
	"strings"
	"sync"
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
	bytes     []byte
	mediaType string
}

// packageAssetTable is built once from every registered package, because
// registration completes during package initialization and cannot change
// afterwards.
var packageAssetTable = sync.OnceValue(buildPackageAssetTable)

// buildPackageAssetTable maps the digest path segment plus name to the file.
// Serving is table driven: nothing walks an fs.FS per request, and nothing reads
// a filesystem at all, which is what a TinyGo target with no filesystem needs.
func buildPackageAssetTable() map[string]packageAsset {
	table := make(map[string]packageAsset)
	for _, pkg := range Packages() {
		if pkg.Assets == nil {
			continue
		}
		_ = fs.WalkDir(pkg.Assets, ".", func(name string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() {
				return nil
			}
			data, readErr := fs.ReadFile(pkg.Assets, name)
			if readErr != nil {
				// A file the walk found and the read refuses is a broken embed
				// rather than a request-time condition. Skipping keeps startup
				// from failing over one asset, and the missing URL is what the
				// package's own release check exists to catch.
				return nil
			}
			table[packageAssetKey(data, path.Base(name))] = packageAsset{
				bytes:     data,
				mediaType: packageAssetMediaType(name),
			}
			return nil
		})
	}
	return table
}

// packageAssetKey is the path below the prefix: the content digest, then the
// base name. The name is kept because a browser, a source map, and a developer
// reading a network panel all want to see what the file is, and the digest alone
// would show none of that.
func packageAssetKey(data []byte, name string) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])[:16] + "/" + name
}

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
	for _, pkg := range Packages() {
		if pkg.Module != module || pkg.Assets == nil {
			continue
		}
		data, err := fs.ReadFile(pkg.Assets, name)
		if err != nil {
			return "", false
		}
		return packageAssetPrefix + packageAssetKey(data, path.Base(name)), true
	}
	return "", false
}

// servePackageAsset answers a package asset request and reports whether it
// handled the request. It runs above serveReservedPath, which closes the
// namespace below every handler that owns something inside it.
func servePackageAsset(w http.ResponseWriter, r *http.Request) bool {
	return servePackageAssetFrom(packageAssetTable(), w, r)
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
	header.Set("Content-Length", strconv.Itoa(len(asset.bytes)))
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return true
	}
	_, _ = w.Write(asset.bytes)
	return true
}
