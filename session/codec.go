package session

import (
	"encoding/json"
	"fmt"
)

// JSONCodec is the default payload codec for durable stores. Record timestamps
// stay in backend columns or fields, so only the typed application payload is
// serialized here.
type JSONCodec[T any] struct{}

func (JSONCodec[T]) Encode(value T) ([]byte, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("%w: encode payload", ErrCodec)
	}
	if len(encoded) == 0 {
		return nil, fmt.Errorf("%w: empty payload", ErrCodec)
	}
	return encoded, nil
}

func (JSONCodec[T]) Decode(encoded []byte) (T, error) {
	// Unmarshal already rejects trailing non-whitespace bytes, without the
	// decoder a trailing-bytes check used to allocate here.
	var value T
	if err := json.Unmarshal(encoded, &value); err != nil {
		var zero T
		return zero, fmt.Errorf("%w: decode payload", ErrCodec)
	}
	return value, nil
}

var _ Codec[string] = JSONCodec[string]{}
