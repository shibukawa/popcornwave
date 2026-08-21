package authn

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

var (
	ErrMalformedJSON = errors.New("authn: malformed JSON")
	ErrDuplicateJSON = errors.New("authn: duplicate JSON member")
)

type JSONOptions struct {
	MaxBytes   int
	MaxDepth   int
	MaxMembers int
}

// ValidateJSON validates exactly one JSON value and rejects duplicate object
// members at every nesting level.
func ValidateJSON(data []byte, options JSONOptions) error {
	if options.MaxBytes <= 0 || options.MaxDepth <= 0 || options.MaxMembers <= 0 {
		return ErrInvalidSize
	}
	if len(data) > options.MaxBytes {
		return ErrLimitExceeded
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	members := 0
	if err := parseJSONValue(decoder, 1, options, &members); err != nil {
		return err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		if err == nil {
			return fmt.Errorf("%w: trailing value", ErrMalformedJSON)
		}
		return fmt.Errorf("%w: %v", ErrMalformedJSON, err)
	}
	return nil
}

// ValidateJSONObject is ValidateJSON for a value that must be a single JSON
// object, and returns that object's members as raw slices of data.
//
// It exists for the token parse path: the validating walk already visits
// every byte, so handing the members back saves the full second decode the
// caller would otherwise run over the same data — twice per bearer token, for
// the header and the claims. Every guarantee of ValidateJSON holds: one
// value, bounded depth and member count, and no duplicate member at any
// nesting level.
func ValidateJSONObject(data []byte, options JSONOptions) (map[string]json.RawMessage, error) {
	if options.MaxBytes <= 0 || options.MaxDepth <= 0 || options.MaxMembers <= 0 {
		return nil, ErrInvalidSize
	}
	if len(data) > options.MaxBytes {
		return nil, ErrLimitExceeded
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	token, err := decoder.Token()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMalformedJSON, err)
	}
	if delimiter, ok := token.(json.Delim); !ok || delimiter != '{' {
		return nil, ErrMalformedJSON
	}
	members := 0
	result := map[string]json.RawMessage{}
	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrMalformedJSON, err)
		}
		key, ok := keyToken.(string)
		if !ok {
			return nil, ErrMalformedJSON
		}
		members++
		if members > options.MaxMembers {
			return nil, ErrLimitExceeded
		}
		if _, duplicate := result[key]; duplicate {
			return nil, ErrDuplicateJSON
		}
		// The offset after the key token sits before the colon; the value's
		// bytes follow it, so the separator and any whitespace are trimmed
		// off the front of the captured slice. No JSON value starts with
		// either.
		start := decoder.InputOffset()
		if err := parseJSONValue(decoder, 2, options, &members); err != nil {
			return nil, err
		}
		result[key] = json.RawMessage(bytes.TrimLeft(data[start:decoder.InputOffset()], ": \t\r\n"))
	}
	if closing, err := decoder.Token(); err != nil || closing != json.Delim('}') {
		return nil, ErrMalformedJSON
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, fmt.Errorf("%w: trailing value", ErrMalformedJSON)
		}
		return nil, fmt.Errorf("%w: %v", ErrMalformedJSON, err)
	}
	return result, nil
}

func parseJSONValue(decoder *json.Decoder, depth int, options JSONOptions, members *int) error {
	if depth > options.MaxDepth {
		return ErrLimitExceeded
	}
	token, err := decoder.Token()
	if err != nil {
		return fmt.Errorf("%w: %v", ErrMalformedJSON, err)
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delimiter {
	case '{':
		seen := make(map[string]struct{})
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return fmt.Errorf("%w: %v", ErrMalformedJSON, err)
			}
			key, ok := keyToken.(string)
			if !ok {
				return ErrMalformedJSON
			}
			(*members)++
			if *members > options.MaxMembers {
				return ErrLimitExceeded
			}
			if _, duplicate := seen[key]; duplicate {
				return ErrDuplicateJSON
			}
			seen[key] = struct{}{}
			if err := parseJSONValue(decoder, depth+1, options, members); err != nil {
				return err
			}
		}
	case '[':
		for decoder.More() {
			(*members)++
			if *members > options.MaxMembers {
				return ErrLimitExceeded
			}
			if err := parseJSONValue(decoder, depth+1, options, members); err != nil {
				return err
			}
		}
	default:
		return ErrMalformedJSON
	}
	closing, err := decoder.Token()
	if err != nil || closing != matchingDelimiter(delimiter) {
		return ErrMalformedJSON
	}
	return nil
}

func matchingDelimiter(open json.Delim) json.Delim {
	if open == '{' {
		return '}'
	}
	return ']'
}
