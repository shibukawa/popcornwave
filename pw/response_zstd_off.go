//go:build pw_nozstd

package pw

import (
	"errors"
	"io"
)

// The pw_nozstd build tag removes response compression and with it the zstd
// encoder, which is worth roughly a megabyte of binary. A deployment that
// terminates compression in front of the application — a CDN or a reverse
// proxy — pays for code it never runs otherwise, because the runtime
// middleware.compression toggle cannot unlink what is compiled in.

const zstdContentEncoding = "zstd"

const zstdResponseSupported = false

type responseZstdEncoder interface {
	io.Writer
	Flush() error
	Close() error
	Abort()
}

// newResponseZstdEncoder is unreachable under this tag: prepareHTMLResponse
// tests zstdResponseSupported before calling it.
func newResponseZstdEncoder(io.Writer) (responseZstdEncoder, error) {
	return nil, errors.New("popcornwave: built with pw_nozstd")
}
