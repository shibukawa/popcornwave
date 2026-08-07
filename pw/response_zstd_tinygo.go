//go:build tinygo || force_tinygo_logic

package pw

import (
	"io"

	"github.com/shibukawa/tinygodriver/compress/zstd"
)

const zstdContentEncoding = zstd.ContentEncoding

type responseZstdEncoder interface {
	io.Writer
	Flush() error
	Close() error
	Abort()
}

type tinyGoResponseZstdEncoder struct {
	*zstd.Writer
}

func newResponseZstdEncoder(w io.Writer) (responseZstdEncoder, error) {
	encoder, err := zstd.NewWriter(w, zstd.WithETag(false))
	if err != nil {
		return nil, err
	}
	return &tinyGoResponseZstdEncoder{Writer: encoder}, nil
}

// The bounded TinyGo encoder has no Reset operation. An aborted pre-commit
// frame is intentionally discarded and becomes collectible without being
// closed, preserving the problem-response fallback semantics.
func (z *tinyGoResponseZstdEncoder) Abort() {}
