package session

import (
	"bytes"
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
	var zero T
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	var value T
	if err := decoder.Decode(&value); err != nil {
		return zero, fmt.Errorf("%w: decode payload", ErrCodec)
	}
	if decoder.More() {
		return zero, fmt.Errorf("%w: trailing payload bytes", ErrCodec)
	}
	return value, nil
}

var _ Codec[string] = JSONCodec[string]{}
