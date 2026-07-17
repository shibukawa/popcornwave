package oauth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/shibukawa/petitweb-go/contrib/authstate"
)

const (
	transactionCodecVersion  = 1
	maxTransactionCodecBytes = 16 << 10
)

// TransactionCodec serializes OAuth correlation state for durable authstate
// stores. Its format is versioned and contains secrets, so encoded values must
// never be logged.
type TransactionCodec struct{}

type transactionRecord struct {
	Version     int    `json:"v"`
	State       string `json:"state"`
	Verifier    string `json:"verifier"`
	Nonce       string `json:"nonce,omitempty"`
	RedirectURI string `json:"redirect_uri"`
	ExpiresAtMS int64  `json:"expires_at_ms"`
}

func (TransactionCodec) Encode(value Transaction) ([]byte, error) {
	if !validTransactionRecord(value.State, value.Verifier, value.Nonce, value.RedirectURI, value.ExpiresAt) {
		return nil, fmt.Errorf("%w: oauth transaction encode", authstate.ErrCodec)
	}
	record := transactionRecord{
		Version: transactionCodecVersion, State: value.State, Verifier: value.Verifier,
		Nonce: value.Nonce, RedirectURI: value.RedirectURI, ExpiresAtMS: value.ExpiresAt.UnixMilli(),
	}
	encoded, err := json.Marshal(record)
	if err != nil || len(encoded) > maxTransactionCodecBytes {
		return nil, fmt.Errorf("%w: oauth transaction encode", authstate.ErrCodec)
	}
	return encoded, nil
}

func (TransactionCodec) Decode(encoded []byte) (Transaction, error) {
	var zero Transaction
	if len(encoded) == 0 || len(encoded) > maxTransactionCodecBytes {
		return zero, fmt.Errorf("%w: oauth transaction decode", authstate.ErrCodec)
	}
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	var record transactionRecord
	if err := decoder.Decode(&record); err != nil {
		return zero, fmt.Errorf("%w: oauth transaction decode", authstate.ErrCodec)
	}
	if err := requireJSONEOF(decoder); err != nil || record.Version != transactionCodecVersion {
		return zero, fmt.Errorf("%w: oauth transaction decode", authstate.ErrCodec)
	}
	expiresAt := time.UnixMilli(record.ExpiresAtMS)
	if !validTransactionRecord(record.State, record.Verifier, record.Nonce, record.RedirectURI, expiresAt) {
		return zero, fmt.Errorf("%w: oauth transaction decode", authstate.ErrCodec)
	}
	return Transaction{
		State: record.State, Verifier: record.Verifier, Nonce: record.Nonce,
		RedirectURI: record.RedirectURI, ExpiresAt: expiresAt,
	}, nil
}

func validTransactionRecord(state, verifier, nonce, redirectURI string, expiresAt time.Time) bool {
	return state != "" && len(state) <= 256 && verifier != "" && len(verifier) <= 256 &&
		len(nonce) <= 256 && redirectURI != "" && len(redirectURI) <= 4096 &&
		!expiresAt.IsZero() && expiresAt.UnixMilli() > 0
}

func requireJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("extra JSON value")
		}
		return err
	}
	return nil
}

var _ authstate.Codec[Transaction] = TransactionCodec{}
