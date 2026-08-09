//go:build pw_nogzip

package pw

import (
	"errors"
	"io"
)

// The pw_nogzip build tag removes the gzip response encoder, as pw_nozstd
// removes the zstd one. It is the cheaper of the two to keep — roughly 148 KB
// beside the zstd encoder's 247 — so it is worth removing only where
// compression is terminated in front of the application and pw_nozstd is being
// passed anyway. Passing both leaves the runtime with nothing to negotiate,
// and middleware.compression becomes a setting with no effect.

const gzipContentEncoding = "gzip"

const gzipResponseSupported = false

// newResponseGzipEncoder is unreachable under this tag: availableResponseCodings
// omits the coding, so nothing ever holds this constructor.
func newResponseGzipEncoder(io.Writer) (responseEncoder, error) {
	return nil, errors.New("popcornwave: built with pw_nogzip")
}
