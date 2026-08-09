package middlewares

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
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
// negotiated against the precompressed sidecars the build produced.
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
	// The embedded tree is immutable, so a name resolves to the same bytes,
	// media type, and validator on every request. The local tree can change
	// between requests, and ReadLocal consults it before the embedded one, so
	// that mode keeps resolving fresh. Only names that resolve are cached,
	// which bounds the cache by the tree rather than by what clients request.
	memoize := !publicDevelopment && !config.ReadLocal
	var resolvedAssets sync.Map
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
			var resolved *resolvedPublicAsset
			if memoize {
				if cached, ok := resolvedAssets.Load(name); ok {
					resolved = cached.(*resolvedPublicAsset)
				}
			}
			if resolved == nil {
				asset, ok := resolvePublicAsset(name, config, embedded)
				if !ok {
					http.NotFound(w, r)
					return
				}
				resolved = finishPublicAsset(asset)
				if memoize {
					resolvedAssets.Store(name, resolved)
				}
			}
			if !publicDevelopment {
				w.Header().Set("Vary", "Accept-Encoding")
			}
			representation, rank, acceptable := selectPublicRepresentation(r, resolved.asset)
			if !acceptable {
				http.Error(w, http.StatusText(http.StatusNotAcceptable), http.StatusNotAcceptable)
				return
			}
			w.Header().Set("Content-Type", resolved.contentType)
			w.Header().Set("Content-Length", strconv.Itoa(len(representation)))
			etag := resolved.identityTag
			if rank >= 0 {
				w.Header().Set("Content-Encoding", staticContentCodings[rank].token)
				etag = resolved.encodedTags[rank]
			}
			w.Header().Set("ETag", etag)
			if r.Header.Get("If-None-Match") == etag {
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
	body, err := embedded.Open(representation.Path)
	if err != nil {
		// The manifest and the tree ship together, so a missing file means the
		// two came from different builds and no response would be honest.
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	defer body.Close()
	header.Set("Content-Length", strconv.Itoa(representation.Length))
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodGet {
		_, _ = io.Copy(w, body)
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
	// A hand-rolled walk rather than strings.Split, because this runs on every
	// static asset request and the split slice was its only allocation.
	for rest := name; ; {
		segment, tail, found := strings.Cut(rest, "/")
		if segment == "" || strings.HasPrefix(segment, ".") {
			return "", false
		}
		if !found {
			break
		}
		rest = tail
	}
	if hasStaticCodingSuffix(name) {
		return "", false
	}
	return name, true
}

type publicAsset struct {
	name     string
	identity []byte
	// encoded holds one precompressed sidecar per staticContentCodings rank,
	// nil where the build produced none. A missing coding is ordinary: an
	// encode that came out no smaller than its source is skipped.
	encoded [maxStaticCodings][]byte
}

// resolvedPublicAsset carries one manifest-less asset with everything a
// response derives from its bytes already computed, so a cached asset costs
// no digest and no type sniff while a request waits.
type resolvedPublicAsset struct {
	asset       publicAsset
	contentType string
	identityTag string
	// encodedTags parallels publicAsset.encoded. Two representations of one URL
	// never share a validator, because a cache holding one must not answer a
	// request that asked for another.
	encodedTags [maxStaticCodings]string
}

func finishPublicAsset(asset publicAsset) *resolvedPublicAsset {
	contentType := mime.TypeByExtension(path.Ext(asset.name))
	if contentType == "" {
		contentType = http.DetectContentType(asset.identity)
	}
	resolved := &resolvedPublicAsset{asset: asset, contentType: contentType, identityTag: publicAssetETag(asset.identity)}
	for rank, body := range asset.encoded {
		if len(body) > 0 {
			resolved.encodedTags[rank] = publicAssetETag(body)
		}
	}
	return resolved
}

func publicAssetETag(body []byte) string {
	sum := sha256.Sum256(body)
	return `"` + hex.EncodeToString(sum[:16]) + `"`
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
		for rank := range staticContentCodings {
			sidecar := current + staticContentCodings[rank].suffix
			sidecarInfo, sidecarErr := os.Lstat(sidecar)
			if sidecarErr != nil || !sidecarInfo.Mode().IsRegular() || sidecarInfo.Mode()&os.ModeSymlink != 0 {
				continue
			}
			asset.encoded[rank], _ = os.ReadFile(sidecar)
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
	for rank := range staticContentCodings {
		if sidecar, sidecarErr := fs.ReadFile(embedded, name+staticContentCodings[rank].suffix); sidecarErr == nil {
			asset.encoded[rank] = sidecar
		}
	}
	return asset, true
}

// selectPublicRepresentation picks the smallest stored form the client will
// take, or the identity bytes. The order is staticContentCodings, not the
// client's q-values, which state what can be read rather than what is worth
// sending.
//
// The returned rank indexes staticContentCodings, or is -1 for identity, so the
// caller reads the matching validator without matching on the token again.
func selectPublicRepresentation(r *http.Request, asset publicAsset) ([]byte, int, bool) {
	if publicDevelopment {
		return asset.identity, -1, true
	}
	values, present := r.Header["Accept-Encoding"]
	if !present {
		return asset.identity, -1, true
	}
	header := strings.Join(values, ",")
	if strings.TrimSpace(header) == "" {
		return asset.identity, -1, true
	}
	quality := scanEncodingQuality(header)
	for rank := range staticContentCodings {
		if len(asset.encoded[rank]) > 0 && quality.acceptsCoding(rank) > 0 {
			return asset.encoded[rank], rank, true
		}
	}
	if quality.acceptsIdentity() > 0 {
		return asset.identity, -1, true
	}
	return nil, -1, false
}
