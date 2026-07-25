// Package redis provides a Redis and Valkey backed authstate.Store.
package redis

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"github.com/shibukawa/popcornwave/contrib/authstate"
)

const (
	recordVersion        = 1
	recordHeaderBytes    = 9
	defaultMaxKeyBytes   = 256
	defaultMaxValueBytes = 64 << 10
	hardMaxKeyBytes      = 4096
	hardMaxValueBytes    = 1 << 20
	maxPrefixBytes       = 128
	maxNamespaceBytes    = 128
)

// Options controls key isolation, resource bounds, and expiry behavior.
type Options struct {
	Prefix        string
	Namespace     string
	Now           func() time.Time
	MaxKeyBytes   int
	MaxValueBytes int
}

type commandClient interface {
	SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) *goredis.BoolCmd
	GetDel(ctx context.Context, key string) *goredis.StringCmd
}

// Store persists expiring, single-use authentication state in Redis or Valkey.
type Store[T any] struct {
	client        commandClient
	codec         authstate.Codec[T]
	prefix        string
	now           func() time.Time
	maxKeyBytes   int
	maxValueBytes int
}

// NewStore constructs a Redis or Valkey backed Store. The caller retains
// ownership of client and is responsible for closing it.
func NewStore[T any](client goredis.UniversalClient, codec authstate.Codec[T], options Options) (*Store[T], error) {
	return newStore[T](client, codec, options)
}

func newStore[T any](client commandClient, codec authstate.Codec[T], options Options) (*Store[T], error) {
	if client == nil || codec == nil || !validPrefix(options.Prefix) ||
		!validNamespace(options.Namespace) || options.MaxKeyBytes < 0 ||
		options.MaxKeyBytes > hardMaxKeyBytes || options.MaxValueBytes < 0 ||
		options.MaxValueBytes > hardMaxValueBytes ||
		(options.MaxValueBytes > 0 && options.MaxValueBytes <= recordHeaderBytes) {
		return nil, authstate.ErrInvalidOptions
	}
	maxKeyBytes := options.MaxKeyBytes
	if maxKeyBytes == 0 {
		maxKeyBytes = defaultMaxKeyBytes
	}
	maxValueBytes := options.MaxValueBytes
	if maxValueBytes == 0 {
		maxValueBytes = defaultMaxValueBytes
	}
	now := options.Now
	if now == nil {
		now = time.Now
	}
	return &Store[T]{
		client: client, codec: codec, prefix: options.Prefix + options.Namespace + ":", now: now,
		maxKeyBytes: maxKeyBytes, maxValueBytes: maxValueBytes,
	}, nil
}

func (s *Store[T]) Put(ctx context.Context, key string, value T, expiresAt time.Time) error {
	if !s.ready() || ctx == nil {
		return authstate.ErrInvalidOptions
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if key == "" || len(key) > s.maxKeyBytes {
		return authstate.ErrInvalidKey
	}
	now := s.now()
	if expiresAt.IsZero() || !expiresAt.After(now) || expiresAt.UnixMilli() <= 0 {
		return authstate.ErrInvalidExpiry
	}
	payload, err := s.codec.Encode(value)
	if err != nil {
		return fmt.Errorf("%w: encode", authstate.ErrCodec)
	}
	if len(payload) == 0 {
		return fmt.Errorf("%w: encode", authstate.ErrCodec)
	}
	if len(payload) > s.maxValueBytes-recordHeaderBytes {
		return authstate.ErrLimitExceeded
	}
	record := make([]byte, recordHeaderBytes+len(payload))
	record[0] = recordVersion
	binary.BigEndian.PutUint64(record[1:recordHeaderBytes], uint64(expiresAt.UnixMilli()))
	copy(record[recordHeaderBytes:], payload)
	created, err := s.client.SetNX(ctx, s.prefix+key, record, ceilMillisecond(expiresAt.Sub(now))).Result()
	if err != nil {
		return unavailable(ctx)
	}
	if !created {
		return authstate.ErrAlreadyExists
	}
	return nil
}

func (s *Store[T]) Take(ctx context.Context, key string) (T, error) {
	var zero T
	if !s.ready() || ctx == nil {
		return zero, authstate.ErrInvalidOptions
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	if key == "" || len(key) > s.maxKeyBytes {
		return zero, authstate.ErrInvalidKey
	}
	encoded, err := s.client.GetDel(ctx, s.prefix+key).Result()
	if errors.Is(err, goredis.Nil) {
		return zero, authstate.ErrNotFound
	}
	if err != nil {
		return zero, unavailable(ctx)
	}
	if len(encoded) < recordHeaderBytes || len(encoded) > s.maxValueBytes || encoded[0] != recordVersion {
		return zero, fmt.Errorf("%w: malformed record", authstate.ErrCodec)
	}
	expiresAtMS := int64(binary.BigEndian.Uint64([]byte(encoded[1:recordHeaderBytes])))
	if expiresAtMS <= 0 {
		return zero, fmt.Errorf("%w: malformed record", authstate.ErrCodec)
	}
	if !time.UnixMilli(expiresAtMS).After(s.now()) {
		return zero, authstate.ErrExpired
	}
	value, err := s.codec.Decode([]byte(encoded[recordHeaderBytes:]))
	if err != nil {
		return zero, fmt.Errorf("%w: decode", authstate.ErrCodec)
	}
	return value, nil
}

func (s *Store[T]) ready() bool {
	return s != nil && s.client != nil && s.codec != nil && s.now != nil && s.prefix != "" &&
		s.maxKeyBytes > 0 && s.maxValueBytes >= recordHeaderBytes
}

func validPrefix(value string) bool {
	if value == "" || len(value) > maxPrefixBytes {
		return false
	}
	for i := range len(value) {
		if value[i] < 0x21 || value[i] > 0x7e {
			return false
		}
	}
	return true
}

func validNamespace(value string) bool {
	if value == "" || len(value) > maxNamespaceBytes {
		return false
	}
	for i := range len(value) {
		if value[i] < 0x21 || value[i] > 0x7e || value[i] == ':' {
			return false
		}
	}
	return true
}

func ceilMillisecond(duration time.Duration) time.Duration {
	if remainder := duration % time.Millisecond; remainder != 0 {
		if duration <= time.Duration(1<<63-1)-(time.Millisecond-remainder) {
			duration += time.Millisecond - remainder
		}
	}
	return duration
}

func unavailable(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return fmt.Errorf("%w: redis", authstate.ErrUnavailable)
}

var _ authstate.Store[string] = (*Store[string])(nil)
