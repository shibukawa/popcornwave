package passkeytest

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/binary"
	"fmt"

	"github.com/shibukawa/popcornweb/contrib/cbor"
)

// Authenticator data flags, from the WebAuthn authenticator data layout.
const (
	flagUP byte = 1 << 0
	flagUV byte = 1 << 2
	flagBE byte = 1 << 3
	flagBS byte = 1 << 4
	flagAT byte = 1 << 6
)

// registrationAuthData builds authenticator data carrying attested credential
// data, which a registration response must include.
func registrationAuthData(rpID string, flags byte, signCount uint32, aaguid [16]byte, credentialID, cose []byte) []byte {
	result := assertionAuthData(rpID, flags, signCount)
	result = append(result, aaguid[:]...)
	var length [2]byte
	binary.BigEndian.PutUint16(length[:], uint16(len(credentialID)))
	result = append(result, length[:]...)
	result = append(result, credentialID...)
	return append(result, cose...)
}

// assertionAuthData builds the fixed authenticator data prefix an assertion
// carries: the RP ID hash, the flags, and the signature counter.
func assertionAuthData(rpID string, flags byte, signCount uint32) []byte {
	hash := sha256.Sum256([]byte(rpID))
	result := append([]byte(nil), hash[:]...)
	result = append(result, flags)
	var count [4]byte
	binary.BigEndian.PutUint32(count[:], signCount)
	return append(result, count[:]...)
}

// encodeCOSEKey encodes an EC2 public key as the COSE_Key an authenticator
// reports for the credential.
func encodeCOSEKey(key *ecdsa.PublicKey, algorithm int) (cbor.RawMessage, error) {
	x, err := encodeBytes(key.X.FillBytes(make([]byte, 32)))
	if err != nil {
		return nil, err
	}
	y, err := encodeBytes(key.Y.FillBytes(make([]byte, 32)))
	if err != nil {
		return nil, err
	}
	entries, err := intEntries([][2]int64{{1, 2}, {3, int64(algorithm)}, {-1, 1}})
	if err != nil {
		return nil, err
	}
	negativeTwo, err := encodeInt(-2)
	if err != nil {
		return nil, err
	}
	negativeThree, err := encodeInt(-3)
	if err != nil {
		return nil, err
	}
	entries = append(entries,
		cbor.MapEntry{Key: negativeTwo, Value: x},
		cbor.MapEntry{Key: negativeThree, Value: y},
	)
	return encodeMap(entries)
}

// encodeAttestationObject wraps authenticator data in a none attestation
// object, which is the only statement format requirement:contrib-passkey
// accepts.
func encodeAttestationObject(authData []byte) (cbor.RawMessage, error) {
	format, err := encodeText("none")
	if err != nil {
		return nil, err
	}
	data, err := encodeBytes(authData)
	if err != nil {
		return nil, err
	}
	statement, err := encodeMap(nil)
	if err != nil {
		return nil, err
	}
	keys := make([]cbor.RawMessage, 0, 3)
	for _, name := range []string{"fmt", "authData", "attStmt"} {
		encoded, err := encodeText(name)
		if err != nil {
			return nil, err
		}
		keys = append(keys, encoded)
	}
	return encodeMap([]cbor.MapEntry{
		{Key: keys[0], Value: format},
		{Key: keys[1], Value: data},
		{Key: keys[2], Value: statement},
	})
}

func intEntries(pairs [][2]int64) ([]cbor.MapEntry, error) {
	entries := make([]cbor.MapEntry, 0, len(pairs))
	for _, pair := range pairs {
		key, err := encodeInt(pair[0])
		if err != nil {
			return nil, err
		}
		value, err := encodeInt(pair[1])
		if err != nil {
			return nil, err
		}
		entries = append(entries, cbor.MapEntry{Key: key, Value: value})
	}
	return entries, nil
}

func encodeInt(value int64) (cbor.RawMessage, error) {
	return encodeItem(func(encoder *cbor.Encoder) error { return encoder.WriteInt(value) })
}

func encodeBytes(value []byte) (cbor.RawMessage, error) {
	return encodeItem(func(encoder *cbor.Encoder) error { return encoder.WriteBytes(value) })
}

func encodeText(value string) (cbor.RawMessage, error) {
	return encodeItem(func(encoder *cbor.Encoder) error { return encoder.WriteText(value) })
}

func encodeMap(entries []cbor.MapEntry) (cbor.RawMessage, error) {
	return encodeItem(func(encoder *cbor.Encoder) error { return encoder.WriteMap(entries) })
}

func encodeItem(write func(*cbor.Encoder) error) (cbor.RawMessage, error) {
	var buffer bytes.Buffer
	encoder, err := cbor.NewEncoder(&buffer, cbor.EncoderOptions{})
	if err != nil {
		return nil, fmt.Errorf("passkeytest: cbor encoder: %w", err)
	}
	if err := write(encoder); err != nil {
		return nil, fmt.Errorf("passkeytest: cbor write: %w", err)
	}
	return buffer.Bytes(), nil
}
