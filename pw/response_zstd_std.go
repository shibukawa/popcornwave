//go:build !tinygo && !force_tinygo_logic && !pw_nozstd

package pw

import (
	"io"
	"sync"

	kzstd "github.com/klauspost/compress/zstd"
)

const zstdContentEncoding = "zstd"

const zstdResponseSupported = true

type responseZstdEncoder interface {
	io.Writer
	Flush() error
	Close() error
	Abort()
}

var responseZstdPool sync.Pool

type pooledResponseZstdEncoder struct {
	encoder  *kzstd.Encoder
	released bool
}

func newResponseZstdEncoder(w io.Writer) (responseZstdEncoder, error) {
	if pooled := responseZstdPool.Get(); pooled != nil {
		encoder := pooled.(*kzstd.Encoder)
		encoder.Reset(w)
		return &pooledResponseZstdEncoder{encoder: encoder}, nil
	}
	encoder, err := kzstd.NewWriter(w,
		kzstd.WithEncoderLevel(kzstd.SpeedDefault),
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
