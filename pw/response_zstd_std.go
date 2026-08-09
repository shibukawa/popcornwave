//go:build !tinygo && !force_tinygo_logic && !pw_nozstd

package pw

import (
	"io"
	"sync"

	kzstd "github.com/klauspost/compress/zstd"
)

const zstdContentEncoding = "zstd"

const zstdResponseSupported = true

// The dynamic path encodes at SpeedFastest rather than SpeedDefault. A
// response body is compressed while a request waits, so the level is a
// throughput budget: fastest puts zstd within a tenth of gzip level 1 on
// throughput and keeps it ahead on ratio, which is the whole reason it leads
// the preference order. The public asset build spends the deep levels instead,
// where the cost is a build and not a request.
var responseZstdPool sync.Pool

type pooledResponseZstdEncoder struct {
	encoder  *kzstd.Encoder
	released bool
}

func newResponseZstdEncoder(w io.Writer) (responseEncoder, error) {
	if pooled := responseZstdPool.Get(); pooled != nil {
		encoder := pooled.(*kzstd.Encoder)
		encoder.Reset(w)
		return &pooledResponseZstdEncoder{encoder: encoder}, nil
	}
	encoder, err := kzstd.NewWriter(w,
		kzstd.WithEncoderLevel(kzstd.SpeedFastest),
		kzstd.WithEncoderConcurrency(1),
		kzstd.WithWindowSize(128<<10),
		kzstd.WithLowerEncoderMem(true),
		kzstd.WithEncoderCRC(false),
	)
	if err != nil {
		return nil, err
	}
	return &pooledResponseZstdEncoder{encoder: encoder}, nil
}

func (z *pooledResponseZstdEncoder) Write(p []byte) (int, error) { return z.encoder.Write(p) }
func (z *pooledResponseZstdEncoder) Flush() error                { return z.encoder.Flush() }

func (z *pooledResponseZstdEncoder) Close() error {
	if z.released {
		return nil
	}
	err := z.encoder.Close()
	z.release()
	return err
}

func (z *pooledResponseZstdEncoder) Abort() {
	if z.released {
		return
	}
	z.release()
}

func (z *pooledResponseZstdEncoder) release() {
	z.encoder.Reset(io.Discard)
	z.released = true
	responseZstdPool.Put(z.encoder)
}
