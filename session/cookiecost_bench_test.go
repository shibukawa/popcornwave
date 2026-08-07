package session

import (
	"crypto/rand"
	"fmt"
	"testing"
)

// What one sealed cookie costs, in absolute time.
//
// A percentage of a benchmark's CPU is only meaningful for that benchmark's
// workload: put a real query in front of it and the same work becomes a
// different percentage. Microseconds compose, so these are the numbers a reader
// can add to their own request budget.
//
// Sizes span an empty record through one that is close to the 4 KB a browser
// will carry, because the AEAD cost is linear in the plaintext and a reader
// needs to know where on that line they sit.
func benchKeyring(b *testing.B) *Keyring {
	b.Helper()
	secret := make([]byte, cookieSecretBytes)
	if _, err := rand.Read(secret); err != nil {
		b.Fatal(err)
	}
	ring, err := NewKeyring(secret)
	if err != nil {
		b.Fatal(err)
	}
	return ring
}

func benchPayload(n int) []byte {
	out := make([]byte, n)
	for i := range out {
		out[i] = byte('a' + i%26)
	}
	return out
}

func BenchmarkKeyringSeal(b *testing.B) {
	ring := benchKeyring(b)
	for _, size := range []int{64, 256, 1024, 3072} {
		plaintext := benchPayload(size)
		b.Run(fmt.Sprintf("bytes=%d", size), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, err := ring.seal("pw_session_data", plaintext, rand.Reader); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkKeyringOpen(b *testing.B) {
	ring := benchKeyring(b)
	for _, size := range []int{64, 256, 1024, 3072} {
		sealed, err := ring.seal("pw_session_data", benchPayload(size), rand.Reader)
		if err != nil {
			b.Fatal(err)
		}
		b.Run(fmt.Sprintf("bytes=%d", size), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, ok := ring.open("pw_session_data", sealed); !ok {
					b.Fatal("open failed")
				}
			}
		})
	}
}

// A signed slot pays HMAC rather than AEAD, which is the cheaper half of the
// keyring and worth separating so a reader choosing a placement can compare.
func BenchmarkKeyringSign(b *testing.B) {
	ring := benchKeyring(b)
	body := string(benchPayload(256))
	b.ReportAllocs()
	for b.Loop() {
		_ = ring.sign("pw_session", body)
	}
}

func BenchmarkKeyringVerify(b *testing.B) {
	ring := benchKeyring(b)
	body := string(benchPayload(256))
	mac := ring.sign("pw_session", body)
	b.ReportAllocs()
	for b.Loop() {
		if !ring.verify("pw_session", body, mac) {
			b.Fatal("verify failed")
		}
	}
}
