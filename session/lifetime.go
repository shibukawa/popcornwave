package session

import (
	"encoding/binary"
	"fmt"
	"time"
)

// BrowserMax is the longest a browser will keep a cookie.
//
// There is no "never expires" in HTTP: a cookie with no Max-Age dies when the
// browser closes, and one with a Max-Age is capped. Current browsers cap it at
// 400 days, so this is what "keep it as long as you can" actually means.
const BrowserMax = 400 * 24 * time.Hour

// SlotOption states how long one registered slot lives, and what ends it.
//
// The two axes are independent, with one constraint: a value may always die
// before its session, but only a value the browser holds can outlive it,
// because a session-placed value is destroyed with the record that holds it.
type SlotOption func(*slot) error

// ExpiresAfter bounds a slot to d, whatever its placement.
//
// It is how a value dies before the session that carries it does: a CSRF
// secret rotated on a schedule, a step-up admission good for seconds, a cached
// decision good for minutes. A record-placed slot carries its own deadline
// inside the record and is dropped on read once it passes; a cookie-placed one
// carries it as the cookie lifetime.
//
// The slot still dies with the session. Use OutlivesSession for a value that
// should not.
func ExpiresAfter(d time.Duration) SlotOption {
	return func(entry *slot) error {
		if d <= 0 {
			return fmt.Errorf("%w: slot %q lifetime must be positive", ErrInvalidOptions, entry.key)
		}
		if entry.expiry > 0 || entry.outlives {
			return fmt.Errorf("%w: slot %q states its lifetime twice", ErrInvalidOptions, entry.key)
		}
		if d > BrowserMax {
			return fmt.Errorf("%w: slot %q lifetime exceeds session.BrowserMax", ErrInvalidOptions, entry.key)
		}
		entry.expiry = d
		return nil
	}
}

// OutlivesSession keeps a slot for d and exempts it from the destruction of the
// session, so a sign-out leaves it alone.
//
// It is what a display language or a density preference wants: state that
// belongs to the browser rather than to whoever is signed in. Pass BrowserMax
// to keep it as long as a browser will.
//
// It is refused for session.Private and session.ServerOnly. Those live in the
// session record, and a record cannot outlive its own destruction; the refusal
// is a registration error rather than a surprise at logout.
func OutlivesSession(d time.Duration) SlotOption {
	return func(entry *slot) error {
		if d <= 0 {
			return fmt.Errorf("%w: slot %q lifetime must be positive", ErrInvalidOptions, entry.key)
		}
		if entry.expiry > 0 || entry.outlives {
			return fmt.Errorf("%w: slot %q states its lifetime twice", ErrInvalidOptions, entry.key)
		}
		if d > BrowserMax {
			return fmt.Errorf("%w: slot %q lifetime exceeds session.BrowserMax", ErrInvalidOptions, entry.key)
		}
		if !entry.placement.cookiePlaced() {
			return fmt.Errorf(
				"%w: slot %q is session.%s, which lives in the session record and is destroyed with it; "+
					"only session.Shared and session.ReadOnly may outlive a session",
				ErrInvalidOptions, entry.key, entry.placement)
		}
		entry.expiry = d
		entry.outlives = true
		return nil
	}
}

// ResetOnRotate drops the slot at a rotation instead of carrying it forward.
//
// A rotation normally preserves every value, which is what lets a login keep
// what the anonymous browser accumulated. A few values must not survive it: a
// CSRF secret is the standing example, because policy:session-security requires
// it to change with the session, so a token minted before a sign-in cannot be
// presented after one.
//
// It states what a rotation does to the value, where ExpiresAfter states what
// time does and OutlivesSession states what a destroy does.
func ResetOnRotate() SlotOption {
	return func(entry *slot) error {
		entry.resetOnRotate = true
		return nil
	}
}

// slotStampBytes is the fixed-width deadline a record-placed slot carries when
// it states a lifetime shorter than its session.
const slotStampBytes = 8

// stampSlot prefixes an encoded slot value with its own deadline.
func stampSlot(deadline time.Time, encoded []byte) []byte {
	stamped := make([]byte, slotStampBytes+len(encoded))
	binary.BigEndian.PutUint64(stamped[:slotStampBytes], milliOf(deadline))
	copy(stamped[slotStampBytes:], encoded)
	return stamped
}

// unstampSlot returns the value and whether it is still live. A slot past its
// own deadline reads as absent, exactly as one that was never written.
func unstampSlot(stamped []byte, now time.Time) ([]byte, bool) {
	if len(stamped) < slotStampBytes {
		return nil, false
	}
	deadline := timeOf(binary.BigEndian.Uint64(stamped[:slotStampBytes]))
	if !deadline.IsZero() && !deadline.After(now) {
		return nil, false
	}
	return stamped[slotStampBytes:], true
}
