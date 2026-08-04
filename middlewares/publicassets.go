package middlewares

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

// localPublicRoot is the built tree, relative to the working directory. It is
// what the development loop and an explicit local override both read, because
// it is the only tree whose file names match the URLs the pages carry.
const localPublicRoot = "dist/public"

// PublicAssetConfig controls the framework-owned static asset endpoint.
type PublicAssetConfig struct {
	Enabled   bool   `default:"true"`
	Mount     string `default:"/public" dependon:".enabled"`
	ReadLocal bool   `default:"false" dependon:".enabled"`
}

// NormalizePublicMount validates a mount point and returns it with a trailing
// slash. Its errors name the runtime configuration key they come from.
func NormalizePublicMount(value string) (string, error) {
	if value == "" || value[0] != '/' || value == "/" {
		return "", fmt.Errorf("server.public.mount must be an absolute non-root path")
	}
	if strings.ContainsAny(value, `\?#`) || hasControl(value) {
		return "", fmt.Errorf("server.public.mount is invalid")
	}
	trimmed := strings.TrimSuffix(value, "/")
	if path.Clean(trimmed) != trimmed || strings.ContainsAny(trimmed, "{}*") {
		return "", fmt.Errorf("server.public.mount must be canonical and contain no wildcards")
	}
	return trimmed + "/", nil
}

// PublicAssets serves the application's static assets under config.Mount and
// forwards every other request downstream. Assets come from the embedded
// filesystem, optionally overlaid by a local public directory, and are
// negotiated against precompressed zstd sidecars.
//
// A nil embedded filesystem falls back to the one a generated public.go
// registered with RegisterPublicFS. Outside the pwdev build mode, serving
// without either is a startup error.
func PublicAssets(config PublicAssetConfig, embedded fs.FS) (Middleware, error) {
	mount, err := NormalizePublicMount(config.Mount)
	if err != nil {
		return nil, err
	}
	if embedded == nil {
		embedded = registeredPublicFS()
	}
	if embedded == nil && !publicDevelopment {
		return nil, fmt.Errorf("popcornwave: server.public.enabled requires a registered public filesystem")
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == strings.TrimSuffix(mount, "/") {
				location := mount
				if r.URL.RawQuery != "" {
					location += "?" + r.URL.RawQuery
				}
				http.Redirect(w, r, location, http.StatusPermanentRedirect)
				return
			}
			if !strings.HasPrefix(r.URL.Path, mount) {
				next.ServeHTTP(w, r)
				return
			}
			if r.Method != http.MethodGet && r.Method != http.MethodHead {
				w.Header().Set("Allow", "GET, HEAD")
				http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
				return
			}
			name, ok := publicAssetName(strings.TrimPrefix(r.URL.Path, mount))
			if !ok {
				http.NotFound(w, r)
				return
			}
			// A build that produced a manifest already knows every URL, every
			// representation, and every validator, so the request path reads
			// bytes and nothing else.
			if !publicDevelopment && manifestRegistered() {
				servePublicManifest(w, r, name, embedded)
				return
			}
			asset, ok := resolvePublicAsset(name, config, embedded)
			if !ok {
				http.NotFound(w, r)
				return
			}
			if !publicDevelopment {
				w.Header().Set("Vary", "Accept-Encoding")
			}
			representation, encoding, acceptable := selectPublicRepresentation(r, asset)
			if !acceptable {
				http.Error(w, http.StatusText(http.StatusNotAcceptable), http.StatusNotAcceptable)
				return
			}
			contentType := mime.TypeByExtension(path.Ext(asset.name))
			if contentType == "" {
				contentType = http.DetectContentType(asset.identity)
			}
			w.Header().Set("Content-Type", contentType)
			w.Header().Set("Content-Length", strconv.Itoa(len(representation)))
			if encoding != "" {
				w.Header().Set("Content-Encoding", encoding)
			}
			sum := sha256.Sum256(representation)
			w.Header().Set("ETag", fmt.Sprintf(`"%x"`, sum[:16]))
			if r.Header.Get("If-None-Match") == w.Header().Get("ETag") {
				w.WriteHeader(http.StatusNotModified)
				return
			}
			w.WriteHeader(http.StatusOK)
			if r.Method == http.MethodGet {
				_, _ = w.Write(representation)
			}
		})
	}, nil
}

// servePublicManifest answers from data the build computed. Nothing here stats
// a file, digests a body, or infers a media type: a URL the manifest does not
// name is 404 even when bytes for it exist, because serving what a build did
// not declare is how a stale representation reaches a cache.
func servePublicManifest(w http.ResponseWriter, r *http.Request, name string, embedded fs.FS) {
	entry, found := manifestEntry(name)
	if !found {
		http.NotFound(w, r)
		return
	}
	header := w.Header()
	header.Set("Vary", varyForEntry(entry))
	representation, acceptable := selectRepresentation(entry, r.Header.Values("Accept"), r.Header.Values("Accept-Encoding"))
	if !acceptable {
		http.Error(w, http.StatusText(http.StatusNotAcceptable), http.StatusNotAcceptable)
		return
	}
	header.Set("Content-Type", representation.MediaType)
	header.Set("Cache-Control", entry.CacheControl)
	header.Set("ETag", representation.ETag)
	if representation.ContentEncoding != "" {
		header.Set("Content-Encoding", representation.ContentEncoding)
	}
	if r.Header.Get("If-None-Match") == representation.ETag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	body, err := fs.ReadFile(embedded, representation.Path)
	if err != nil {
		// The manifest and the tree ship together, so a missing file means the
		// two came from different builds and no response would be honest.
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	header.Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodGet {
		_, _ = w.Write(body)
	}
}

// varyForEntry names only the headers that can actually change the answer, so a
// cache stores one variant for an asset that negotiates nothing on Accept.
func varyForEntry(entry AssetEntry) string {
	if entryNegotiatesMedia(entry) {
		return "Accept, Accept-Encoding"
	}
	return "Accept-Encoding"
}

func hasControl(value string) bool {
	for _, r := range value {
		if r < 0x20 || r == 0x7f {
			return true
		}
	}
	return false
}

func publicAssetName(name string) (string, bool) {
	if name == "" || strings.ContainsAny(name, "\\\x00") || !fs.ValidPath(name) || path.Clean(name) != name {
		return "", false
	}
	for _, segment := range strings.Split(name, "/") {
		if segment == "" || strings.HasPrefix(segment, ".") {
			return "", false
		}
	}
	if strings.HasSuffix(name, ".zstd") {
		return "", false
	}
	return name, true
}

type publicAsset struct {
	name     string
	identity []byte
	zstd     []byte
}

func resolvePublicAsset(name string, config PublicAssetConfig, embedded fs.FS) (publicAsset, bool) {
	if publicDevelopment || config.ReadLocal {
		asset, found, rejected := readLocalPublicAsset(name)
		if found {
			return asset, true
		}
		if publicDevelopment || rejected {
			return publicAsset{}, false
		}
	}
	return readEmbeddedPublicAsset(embedded, name)
}

func readLocalPublicAsset(name string) (publicAsset, bool, bool) {
	// The built tree is what is served in every mode. The authored tree is an
	// input: a development loop that read it would answer 404 for every
	// reference a conversion moved, since the page names the derived file.
	root := filepath.FromSlash(localPublicRoot)
	info, err := os.Lstat(root)
	if err != nil {
		return publicAsset{}, false, err != nil && !os.IsNotExist(err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return publicAsset{}, false, true
	}
	current := root
	for _, segment := range strings.Split(name, "/") {
		current = filepath.Join(current, filepath.FromSlash(segment))
		info, err = os.Lstat(current)
		if err != nil {
			return publicAsset{}, false, err != nil && !os.IsNotExist(err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return publicAsset{}, false, true
		}
	}
	if info.IsDir() {
		current = filepath.Join(current, "index.html")
		info, err = os.Lstat(current)
		if err != nil {
			return publicAsset{}, false, err != nil && !os.IsNotExist(err)
		}
		name = path.Join(name, "index.html")
	}
	if !info.Mode().IsRegular() {
		return publicAsset{}, false, true
	}
	identity, err := os.ReadFile(current)
	if err != nil {
		return publicAsset{}, false, true
	}
	asset := publicAsset{name: name, identity: identity}
	if !publicDevelopment {
		sidecarInfo, sidecarErr := os.Lstat(current + ".zstd")
		if sidecarErr == nil && sidecarInfo.Mode().IsRegular() && sidecarInfo.Mode()&os.ModeSymlink == 0 {
			asset.zstd, _ = os.ReadFile(current + ".zstd")
		}
	}
	return asset, true, false
}

func readEmbeddedPublicAsset(embedded fs.FS, name string) (publicAsset, bool) {
	if embedded == nil {
		return publicAsset{}, false
	}
	info, err := fs.Stat(embedded, name)
	if err != nil {
		return publicAsset{}, false
	}
	if info.IsDir() {
		name = path.Join(name, "index.html")
		info, err = fs.Stat(embedded, name)
		if err != nil {
			return publicAsset{}, false
		}
	}
	if !info.Mode().IsRegular() {
		return publicAsset{}, false
	}
	identity, err := fs.ReadFile(embedded, name)
	if err != nil {
		return publicAsset{}, false
	}
	asset := publicAsset{name: name, identity: identity}
	if sidecar, sidecarErr := fs.ReadFile(embedded, name+".zstd"); sidecarErr == nil {
		asset.zstd = sidecar
	}
	return asset, true
}

func selectPublicRepresentation(r *http.Request, asset publicAsset) ([]byte, string, bool) {
	if publicDevelopment {
		return asset.identity, "", true
	}
	values, present := r.Header["Accept-Encoding"]
	if !present || strings.TrimSpace(strings.Join(values, ",")) == "" {
		return asset.identity, "", true
	}
	quality := parseEncodingQuality(strings.Join(values, ","))
	zstdQuality, zstdSet := quality["zstd"]
	if !zstdSet {
		zstdQuality = quality["*"]
	}
	if len(asset.zstd) > 0 && zstdQuality > 0 {
		return asset.zstd, "zstd", true
	}
	identityQuality, identitySet := quality["identity"]
	if !identitySet {
		if wildcard, wildcardSet := quality["*"]; wildcardSet {
			identityQuality = wildcard
		} else {
			identityQuality = 1
		}
	}
	if identityQuality > 0 {
		return asset.identity, "", true
	}
	return nil, "", false
}

func parseEncodingQuality(header string) map[string]float64 {
	result := make(map[string]float64)
	for _, item := range strings.Split(header, ",") {
		parts := strings.Split(item, ";")
		token := strings.ToLower(strings.TrimSpace(parts[0]))
		if token == "" {
			continue
		}
		quality := 1.0
		for _, parameter := range parts[1:] {
			key, value, ok := strings.Cut(strings.TrimSpace(parameter), "=")
			if !ok || !strings.EqualFold(strings.TrimSpace(key), "q") {
				continue
			}
			parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
			if err != nil || parsed < 0 || parsed > 1 {
				quality = 0
			} else {
				quality = parsed
			}
		}
		result[token] = quality
	}
	return result
}
