package session

import (
	"encoding/binary"
	"fmt"
)

// slotMap is the payload of a session record: one encoded value per
// server-placed slot, keyed by its registration key. Each slot is encoded
// independently, so one unreadable slot is cleared rather than failing the
// whole record.
type slotMap map[string][]byte

// slotMapFormat versions the payload layout. A payload written by another
// version is rejected rather than reinterpreted.
const slotMapFormat = 1

// maxSlotsPerRecord bounds how many slots one record may carry, so a malformed
// payload cannot ask for an unbounded allocation.
const maxSlotsPerRecord = 64

// slotMapCodec encodes the record payload as a length-prefixed key and value
// sequence.
//
// JSON would base64 every slot value and inflate the payload by a third, which
// the browser cookie budget of an anonymous session cannot spare.
type slotMapCodec struct {
	// order fixes the encoding order, so one state encodes to one byte string.
	order []string
}

func (c slotMapCodec) Encode(values slotMap) ([]byte, error) {
	blob := []byte{slotMapFormat}
	count := 0
	for _, key := range c.order {
		value, ok := values[key]
		if !ok {
			continue
		}
		if len(key) > 255 {
			return nil, fmt.Errorf("%w: slot key length", ErrCodec)
		}
		blob = append(blob, byte(len(key)))
		blob = append(blob, key...)
		blob = binary.BigEndian.AppendUint32(blob, uint32(len(value)))
		blob = append(blob, value...)
		count++
	}
	if count == 0 {
		// A record with no slot still has to encode to something, because an
		// empty payload is how the stores report a malformed record.
		blob = append(blob, 0)
	}
	return blob, nil
}

func (c slotMapCodec) Decode(encoded []byte) (slotMap, error) {
	if len(encoded) < 1 || encoded[0] != slotMapFormat {
		return nil, fmt.Errorf("%w: record payload layout", ErrCodec)
	}
	values := slotMap{}
	rest := encoded[1:]
	for len(rest) > 0 {
		keyLength := int(rest[0])
		if keyLength == 0 {
			// The empty-record marker written by Encode.
			break
		}
		rest = rest[1:]
		if len(rest) < keyLength+4 {
			return nil, fmt.Errorf("%w: record payload layout", ErrCodec)
		}
		key := string(rest[:keyLength])
		rest = rest[keyLength:]
		// Compared in uint64 before narrowing: on a 32-bit build a length above
		// MaxInt32 would convert to a negative int, pass an int comparison, and
		// panic in the slice expression below.
		encodedLength := binary.BigEndian.Uint32(rest[:4])
		rest = rest[4:]
		if uint64(encodedLength) > uint64(len(rest)) {
			return nil, fmt.Errorf("%w: record payload layout", ErrCodec)
		}
		valueLength := int(encodedLength)
		values[key] = rest[:valueLength]
		rest = rest[valueLength:]
		if len(values) > maxSlotsPerRecord {
			return nil, fmt.Errorf("%w: record slot count", ErrCodec)
		}
	}
	return values, nil
}

var _ Codec[slotMap] = slotMapCodec{}
