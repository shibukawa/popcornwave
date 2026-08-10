// Package requestid mints and checks the correlation ID a request carries.
//
// None of it touches a request. Both middleware chains read the header on their
// own transport and hand the string here, so the rule for what may be echoed
// back is one rule rather than two — which matters more than it sounds, because
// the rule is a security check: an ID arrives from the client and leaves in a
// response header, and a second copy of the check would be a second chance to
// get the escaping wrong.
package requestid

import (
	"crypto/rand"
	"encoding/hex"
	"strconv"
	"sync/atomic"
	"time"
)

// DefaultHeader carries the correlation ID in both directions.
const DefaultHeader = "X-Request-ID"

var sequence atomic.Uint64

// Valid reports whether a client-supplied ID is safe to echo back.
//
// The bound and the character range are the check: anything outside printable
// ASCII could terminate the header or inject another one, and an unbounded
// value would let a client choose how much of every log line is theirs.
func Valid(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for _, r := range value {
		if r < 0x21 || r > 0x7e {
			return false
		}
	}
	return true
}

// Sequential builds an ID from the clock and a process counter.
func Sequential() string {
	// 16 hex digits cover the nanosecond clock until 2554 and the counter for
	// far longer, so the buffer never grows.
	buf := make([]byte, 0, 33)
	buf = strconv.AppendUint(buf, uint64(time.Now().UnixNano()), 16)
	buf = append(buf, '-')
	buf = strconv.AppendUint(buf, sequence.Add(1), 16)
	return string(buf)
}

// Random builds an ID from 16 cryptographically random bytes.
func Random() string {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "popcornwave-request"
	}
	return hex.EncodeToString(bytes[:])
}
