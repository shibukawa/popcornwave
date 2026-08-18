// Package jwt strictly parses, signs, and verifies a bounded signed JWT subset.
package jwt

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/shibukawa/popcornweb/contrib/internal/authn"
)

var (
	ErrMalformed            = errors.New("jwt: malformed token")
	ErrLimitExceeded        = errors.New("jwt: limit exceeded")
	ErrUnsupportedAlgorithm = errors.New("jwt: unsupported algorithm")
	ErrInvalidSignature     = errors.New("jwt: invalid signature")
	ErrInvalidClaims        = errors.New("jwt: invalid claims")
	ErrKeyNotFound          = errors.New("jwt: verification key not found")
	ErrAmbiguousKey         = errors.New("jwt: ambiguous verification key")
	ErrInvalidOptions       = errors.New("jwt: invalid options")
)

const (
	defaultMaxTokenBytes   = 16 << 10
	defaultMaxSegmentBytes = 8 << 10
	defaultMaxJSONDepth    = 16
	defaultMaxJSONMembers  = 256
	maxMaxTokenBytes       = 4 << 20
	maxMaxSegmentBytes     = 2 << 20
	maxMaxJSONDepth        = 64
	maxMaxJSONMembers      = 8192
	maxSignerKeyBytes      = 4096
)

type ParseOptions struct {
	MaxTokenBytes   int
	MaxSegmentBytes int
	MaxJSONDepth    int
	MaxJSONMembers  int
}

type Header struct {
	Algorithm string          `json:"alg"`
	Type      string          `json:"typ,omitempty"`
	KeyID     string          `json:"kid,omitempty"`
	Critical  []string        `json:"crit,omitempty"`
	Raw       json.RawMessage `json:"-"`
}

type Token struct {
	Header    Header
	Claims    Claims
	Signature []byte

	signingInput string
}

func Parse(compact string, options ParseOptions) (*Token, error) {
	options, err := normalizeParseOptions(options)
	if err != nil {
		return nil, err
	}
	if len(compact) == 0 || len(compact) > options.MaxTokenBytes {
		return nil, ErrLimitExceeded
	}
	seg0, rest, ok := strings.Cut(compact, ".")
	if !ok {
		return nil, ErrMalformed
	}
	seg1, seg2, ok := strings.Cut(rest, ".")
	if !ok || seg0 == "" || seg1 == "" || seg2 == "" || strings.IndexByte(seg2, '.') >= 0 {
		return nil, ErrMalformed
	}
	if len(seg0) > options.MaxSegmentBytes || len(seg1) > options.MaxSegmentBytes || len(seg2) > options.MaxSegmentBytes {
		return nil, ErrLimitExceeded
	}
	headerJSON, err := authn.DecodeBase64URL(seg0, options.MaxSegmentBytes, options.MaxSegmentBytes)
	if err != nil {
		return nil, classifyDecodeError(err)
	}
	claimsJSON, err := authn.DecodeBase64URL(seg1, options.MaxSegmentBytes, options.MaxSegmentBytes)
	if err != nil {
		return nil, classifyDecodeError(err)
	}
	signature, err := authn.DecodeBase64URL(seg2, options.MaxSegmentBytes, options.MaxSegmentBytes)
	if err != nil {
		return nil, classifyDecodeError(err)
	}
	jsonOptions := authn.JSONOptions{
		MaxBytes: options.MaxSegmentBytes, MaxDepth: options.MaxJSONDepth, MaxMembers: options.MaxJSONMembers,
	}
	if err := authn.ValidateJSON(headerJSON, jsonOptions); err != nil {
		return nil, classifyJSONError(err)
	}
	if err := authn.ValidateJSON(claimsJSON, jsonOptions); err != nil {
		return nil, classifyJSONError(err)
	}
	var header Header
	if err := json.Unmarshal(headerJSON, &header); err != nil || header.Algorithm == "" {
		return nil, ErrMalformed
	}
	if len(header.Critical) != 0 {
		return nil, fmt.Errorf("%w: critical headers", ErrUnsupportedAlgorithm)
	}
	header.Raw = append(json.RawMessage(nil), headerJSON...)
	claims, err := parseClaims(claimsJSON)
	if err != nil {
		return nil, err
	}
	return &Token{
		Header: header, Claims: claims, Signature: signature,
		// The signing input is the token up to the second dot, taken as a
		// substring rather than reassembled.
		signingInput: compact[:len(seg0)+1+len(seg1)],
	}, nil
}

func normalizeParseOptions(options ParseOptions) (ParseOptions, error) {
	if options.MaxTokenBytes < 0 || options.MaxTokenBytes > maxMaxTokenBytes ||
		options.MaxSegmentBytes < 0 || options.MaxSegmentBytes > maxMaxSegmentBytes ||
		options.MaxJSONDepth < 0 || options.MaxJSONDepth > maxMaxJSONDepth ||
		options.MaxJSONMembers < 0 || options.MaxJSONMembers > maxMaxJSONMembers {
		return ParseOptions{}, ErrInvalidOptions
	}
	if options.MaxTokenBytes == 0 {
		options.MaxTokenBytes = defaultMaxTokenBytes
	}
	if options.MaxSegmentBytes == 0 {
		options.MaxSegmentBytes = defaultMaxSegmentBytes
	}
	if options.MaxJSONDepth == 0 {
		options.MaxJSONDepth = defaultMaxJSONDepth
	}
	if options.MaxJSONMembers == 0 {
		options.MaxJSONMembers = defaultMaxJSONMembers
	}
	return options, nil
}

func classifyDecodeError(err error) error {
	if errors.Is(err, authn.ErrLimitExceeded) {
		return ErrLimitExceeded
	}
	return ErrMalformed
}

func classifyJSONError(err error) error {
	if errors.Is(err, authn.ErrLimitExceeded) {
		return ErrLimitExceeded
	}
	return ErrMalformed
}
