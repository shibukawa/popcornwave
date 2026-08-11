package pw

import (
	"os"
	"path/filepath"
	"sync"
)

// boundaryRuntimeScript is the framework's own half of the browser runtime: it
// applies streamed await boundaries and reads the live delivery stream.
//
// The framing it reacts to is written by writeBoundaryCompletion, and the two
// are one design: htmlbind emits the placeholder and the settled fragment, and
// everything about how a fragment travels and lands belongs here.
//
// The trigger is the trailing marker, never the template element. An HTML
// parser inserts an element when it reads its start tag, so code reacting to
// the template's insertion can read one whose content has not arrived yet and
// replace the placeholder with nothing, losing the fallback along with the
// result. Because the marker follows the closing template tag in the byte
// stream, it cannot exist before its template is complete, however a proxy, a
// TLS record, or a compressing encoder split the bytes.
//
// It lives in boundary.js rather than in a Go string literal so a formatter, a
// linter, and an editor can all read it, and that file lives in
// popcornwave/pwbrowser beside the asset it is minified into — both transports
// serve that asset, so its source belongs where the asset does rather than in
// either runtime.
//
// It is read from disk rather than embedded, because an embed cannot reach
// outside its own package and a copy here would be a second source of truth for
// bytes that ship once. Only tests read the unminified source; what a binary
// carries is runtime.min.js, which runtimegen builds from it.
var boundaryRuntimeScript = sync.OnceValue(func() string {
	source, err := os.ReadFile(filepath.Join("..", "pwbrowser", "boundary.js"))
	if err != nil {
		panic("pw: reading the browser runtime source: " + err.Error())
	}
	return string(source)
})()
