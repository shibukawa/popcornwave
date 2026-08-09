//go:build pw_nozstd

package pw

import (
	"errors"
	"io"
)

// The pw_nozstd build tag removes the zstd response encoder, worth roughly
// 247 KB of binary. A deployment that terminates compression in front of the
// application — a CDN or a reverse proxy — pays for code it never runs
// otherwise, because the runtime middleware.compression toggle cannot unlink
// what is compiled in.
//
// It removes one of two dynamic encoders rather than compression itself: gzip
// survives this tag and is dropped by pw_nogzip. A build passing both has
// nothing left to negotiate.

const zstdContentEncoding = "zstd"

const zstdResponseSupported = false

// newResponseZstdEncoder is unreachable under this tag: availableResponseCodings
// omits the coding, so nothing ever holds this constructor.
func newResponseZstdEncoder(io.Writer) (responseEncoder, error) {
	return nil, errors.New("popcornwave: built with pw_nozstd")
}
