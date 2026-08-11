//go:build tinygo || force_tinygo_logic

package pw

import (
	"compress/gzip"
	"io"
)

const gzipContentEncoding = "gzip"

// gzipResponseLevel is 1 here for a stronger reason than on the host: the
// standard library uses its fast encoder for level 1 alone, so level 2 is
// already half the throughput for 1.7 points of ratio. There is no knee to
// look for above it.
const gzipResponseLevel = 1

// The standard library encoder is used on this target rather than the
// klauspost one, which carries architecture-specific assembly with pure-Go
// fallbacks and would be a validation cost where compress/gzip already works.
type tinyGoResponseGzipEncoder struct {
	*gzip.Writer
	released bool
}

func newResponseGzipEncoder(w io.Writer) (responseEncoder, error) {
	encoder, err := gzip.NewWriterLevel(w, gzipResponseLevel)
	if err != nil {
		return nil, err
	}
	return &tinyGoResponseGzipEncoder{Writer: encoder}, nil
}

func (g *tinyGoResponseGzipEncoder) Close() error {
	if g.released {
		return nil
	}
	g.released = true
	return g.Writer.Close()
}

// Abort discards an uncommitted frame. Nothing is pooled on this target, so the
// encoder becomes collectible without writing a trailer, which preserves the
// problem-response fallback exactly as the zstd encoder does.
func (g *tinyGoResponseGzipEncoder) Abort() { g.released = true }
