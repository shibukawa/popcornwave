// Package pwbrowser is the browser runtime this framework ships, and where a
// document finds it.
//
// It is a leaf because both transports serve the same bytes under the same URL,
// and they have to: a document naming the runtime is rendered by whichever half
// is running, and an application's own template names that URL once. Two
// runtimes each embedding their own copy would be two revisions of one asset,
// and a page rendered by one build would load a script the other does not
// serve.
//
// What is not here is the serving. Reading a path and writing a response is a
// transport's work, so each supplies that over Lookup.
package pwbrowser

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
)

//go:generate go run ../internal/runtimegen

//go:embed runtime.min.js
var minifiedRuntimeSource string

// RuntimeSource is the core module's bytes, before whatever a build mode
// appends. It is exported for a test that reads what shipped rather than what
// a set was assembled into.
func RuntimeSource() string { return minifiedRuntimeSource }

// Prefix is reserved for framework-owned browser assets. It is a fixed absolute
// path rather than a subtree of the configurable public mount, because these
// belong to the framework rather than to the application, and because the
// document shell has to be able to name one without reading configuration.
const Prefix = "/_pw/"

// RuntimeName is the module a document loads.
const RuntimeName = "popcornweb-runtime.js"

// state is the module set this process serves. A runtime with modules of its
// own publishes them before anything reads a URL; nothing does at init time,
// which is what makes that ordering ordinary rather than delicate.
//
// The resolved set travels through an atomic pointer because the readers are
// per-request and per-render — every document naming the runtime URL and every
// framework-asset request pass through here — so the mutex guards only
// publication and the build of a new snapshot.
var state struct {
	sync.Mutex
	extra       map[string]string
	extraImport string
	resolved    atomic.Pointer[setState]
}

// Publish adds the modules a build serves beyond the core, and the import line
// the core carries to reach them.
//
// It exists for the development build mode, which adds a module and an import
// and must therefore land on a different revision — with no constant for
// anybody to bump, and no chance of a browser holding the release URL
// immutably and receiving development bytes under it.
//
// A build that adds nothing calls nothing, which is every deployed build.
func Publish(extra map[string]string, extraImport string) {
	state.Lock()
	defer state.Unlock()
	state.extra, state.extraImport = extra, extraImport
	// The memo is cleared rather than computed once, so a build that publishes
	// after something read a URL still serves one consistent answer afterwards.
	state.resolved.Store(nil)
}

type setState struct {
	scripts  map[string]string
	revision string
	// urlPrefix and runtimeURL are finished strings rather than concatenations
	// per call, because every rendered document asks for the runtime URL.
	urlPrefix  string
	runtimeURL string
}

// Scripts returns the module set this build serves, by file name.
//
// It is a set rather than one file because the core loads its capabilities by
// dynamic import. Everything here is framework source: nothing is derived from
// application code, and nothing is written into the project.
func Scripts() map[string]string { return current().scripts }

// Revision digests the script set so a changed dependency changes every URL.
//
// Deriving it from the bytes rather than from a release constant means an
// upgrade that changes the runtime cannot ship under a URL a browser already
// cached forever.
func Revision() string { return current().revision }

func current() *setState {
	if resolved := state.resolved.Load(); resolved != nil {
		return resolved
	}
	state.Lock()
	defer state.Unlock()
	if resolved := state.resolved.Load(); resolved != nil {
		return resolved
	}
	scripts := map[string]string{RuntimeName: minifiedRuntimeSource + state.extraImport}
	for name, source := range state.extra {
		scripts[name] = source
	}
	names := make([]string, 0, len(scripts))
	for name := range scripts {
		names = append(names, name)
	}
	sort.Strings(names)
	digest := sha256.New()
	for _, name := range names {
		// The separators keep the digest a function of the set rather than of
		// the concatenation, so no rename can collide with a content change.
		digest.Write([]byte(name))
		digest.Write([]byte{0})
		digest.Write([]byte(scripts[name]))
		digest.Write([]byte{0})
	}
	resolved := &setState{scripts: scripts, revision: hex.EncodeToString(digest.Sum(nil))[:16]}
	resolved.urlPrefix = Prefix + resolved.revision + "/"
	resolved.runtimeURL = resolved.urlPrefix + RuntimeName
	state.resolved.Store(resolved)
	return resolved
}

// ScriptURL is the absolute path of one module in the set. Modules share one
// revision segment so that imports between them stay ordinary relative
// specifiers and nothing has to rewrite them.
func ScriptURL(name string) string { return current().urlPrefix + name }

// RuntimeScriptURL is the absolute path of the boundary runtime module. A
// document template names it through a declared external function rather than
// as a literal, so the template text survives an upgrade that moves the URL.
func RuntimeScriptURL() string { return current().runtimeURL }

// Lookup answers what a request for path should be served, and whether this
// build claims it at all.
//
// Only the modules of the current set are claimed. The prefix holds more than
// the script assets — a package asset lives here too — so claiming the whole
// namespace here would swallow every route that arrived after this one. A
// transport closes the namespace below all of them.
//
// A stale revision is therefore not found rather than served from the current
// set, which is what makes the immutable caching sound.
func Lookup(path string) (source, contentType string, ok bool) {
	set := current()
	name, ok := scriptNameIn(set, path)
	if !ok {
		return "", "", false
	}
	source, ok = set.scripts[name]
	if !ok {
		return "", "", false
	}
	return source, ContentType(name), true
}

// Claims reports whether path is inside the reserved namespace at all, which is
// what a transport uses to close it.
func Claims(path string) bool { return strings.HasPrefix(path, Prefix) }

// CacheControl is what an asset under a revision segment carries. The segment
// never serves different bytes, so this is genuinely immutable rather than
// merely long-lived.
const CacheControl = "public, max-age=31536000, immutable"

// ContentType picks a type from the name a module was registered under.
//
// The set held nothing but JavaScript until the development launcher brought
// its mark along, and the name is the only thing to read it from: the bytes are
// held as strings, and sniffing them would be guessing at what the file name
// already states. The set is closed and this build wrote every name in it, so
// an unrecognised one is a module rather than something to refuse.
func ContentType(name string) string {
	if strings.HasSuffix(name, ".webp") {
		return "image/webp"
	}
	return "text/javascript; charset=utf-8"
}

func scriptName(path string) (string, bool) {
	return scriptNameIn(current(), path)
}

func scriptNameIn(set *setState, path string) (string, bool) {
	rest, ok := strings.CutPrefix(path, set.urlPrefix)
	if !ok || rest == "" || strings.Contains(rest, "/") {
		return "", false
	}
	return rest, true
}
