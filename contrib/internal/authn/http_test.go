package authn

import (
	"bytes"
	"errors"
	"math"
	"net"
	"testing"
)

func TestReadBounded(t *testing.T) {
	if _, err := ReadBounded(nil, 1); !errors.Is(err, ErrInvalidSize) {
		t.Fatalf("nil reader error = %v", err)
	}
	if _, err := ReadBounded(bytes.NewReader(nil), math.MaxInt64); !errors.Is(err, ErrInvalidSize) {
		t.Fatalf("max int reader error = %v", err)
	}
	data, err := ReadBounded(bytes.NewBufferString("1234"), 4)
	if err != nil || string(data) != "1234" {
		t.Fatalf("ReadBounded = %q, %v", data, err)
	}
	if _, err := ReadBounded(bytes.NewBufferString("12345"), 4); !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("limit error = %v", err)
	}
}

func TestParseEndpoint(t *testing.T) {
	valid := []string{
		"https://issuer.example/token",
		"http://localhost:8080/token",
		"http://127.0.0.1/token",
		"http://[::1]/token",
	}
	for index, endpoint := range valid {
		allowLoopback := index > 0
		if _, err := ParseEndpoint(endpoint, allowLoopback); err != nil {
			t.Errorf("ParseEndpoint(%q) error = %v", endpoint, err)
		}
	}
	invalid := []string{
		"http://issuer.example/token",
		"https://user:pass@issuer.example/token",
		"https://issuer.example/token#fragment",
		"/relative",
	}
	for _, endpoint := range invalid {
		if _, err := ParseEndpoint(endpoint, false); !errors.Is(err, ErrInvalidEndpoint) {
			t.Errorf("ParseEndpoint(%q) error = %v", endpoint, err)
		}
	}
}

func TestRejectRedirect(t *testing.T) {
	if err := RejectRedirect(nil, nil); !errors.Is(err, ErrRedirectRejected) {
		t.Fatalf("RejectRedirect error = %v", err)
	}
}

func TestValidateResolvedIPs(t *testing.T) {
	addresses := []net.IP{net.ParseIP("192.0.2.1"), net.ParseIP("192.0.2.2")}
	allowDocumentation := func(address net.IP) bool {
		return address.String() == "192.0.2.1" || address.String() == "192.0.2.2"
	}
	if err := ValidateResolvedIPs(addresses, allowDocumentation); err != nil {
		t.Fatal(err)
	}
	if err := ValidateResolvedIPs(addresses, func(net.IP) bool { return false }); !errors.Is(err, ErrInvalidEndpoint) {
		t.Fatalf("rejection error = %v", err)
	}
	if err := ValidateResolvedIPs(nil, func(net.IP) bool { return true }); !errors.Is(err, ErrInvalidEndpoint) {
		t.Fatalf("empty address error = %v", err)
	}
	if err := ValidateResolvedIPs(addresses, nil); !errors.Is(err, ErrInvalidEndpoint) {
		t.Fatalf("nil validator error = %v", err)
	}
}
