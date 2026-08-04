package pwcli

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

// The media variants are converted by the tree walk rather than by a reference
// hook, so the upstream conversion cache never sees them: nothing rewrites a
// URL for a variant, because it is a second representation of one the page
// already names.
//
// Without a cache of their own they were re-encoded on every build, which is
// the one cost that grows with the size of the image tree and never with what
// changed in it. This is that cache: the same shape as the upstream one, keyed
// by the source bytes and everything else the output depends on.
const variantCacheDir = "variants"

// cachedImageEncoder wraps an encoder in a content-addressed store.
//
// The key is the digest of the source bytes together with the format, the axis,
// the quality, and the tool identity. Anything left out of it is something a
// cache hit would silently ignore: an encoder upgrade, a quality change, or a
// switch between the dedicated tool and the platform one all have to miss.
func cachedImageEncoder(cacheRoot, format string, encode imageEncoder) imageEncoder {
	if cacheRoot == "" {
		return encode
	}
	return func(source string, lossless bool, quality int) ([]byte, error) {
		key, err := variantCacheKey(source, format, lossless, quality)
		if err != nil {
			// An unreadable source is the encoder's error to report, not this
			// wrapper's to swallow.
			return encode(source, lossless, quality)
		}
		path := filepath.Join(cacheRoot, filepath.FromSlash(variantCacheDir), format, key+"."+format)
		if cached, readErr := os.ReadFile(path); readErr == nil && len(cached) > 0 {
			return cached, nil
		}
		encoded, err := encode(source, lossless, quality)
		if err != nil {
			// A decline is cached by the caller through its own report; an
			// error is not cached at all, because the next run may have the
			// tool this one was missing.
			return nil, err
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err == nil {
			// A cache that cannot be written is a slow build and nothing worse,
			// so the write is best effort and its failure is not the build's.
			_ = writeScaffoldFile(path, encoded)
		}
		return encoded, nil
	}
}

// variantCacheKey digests the source and everything else the output depends on.
func variantCacheKey(source, format string, lossless bool, quality int) (string, error) {
	content, err := os.ReadFile(source)
	if err != nil {
		return "", err
	}
	digest := sha256.New()
	digest.Write(content)
	fmt.Fprintf(digest, "\x00%s\x00%v\x00%d\x00%s", format, lossless, quality, imageEncoderParams(format, lossless))
	return hex.EncodeToString(digest.Sum(nil))[:32], nil
}
