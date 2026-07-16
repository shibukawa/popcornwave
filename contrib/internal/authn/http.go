package authn

import (
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"strings"
)

var (
	ErrInvalidEndpoint  = errors.New("authn: invalid endpoint")
	ErrRedirectRejected = errors.New("authn: redirect rejected")
)

const maxEndpointBytes = 4096

// ReadBounded reads at most maxBytes and reports ErrLimitExceeded when more
// response data is present.
func ReadBounded(reader io.Reader, maxBytes int64) ([]byte, error) {
	if reader == nil || maxBytes <= 0 || maxBytes == math.MaxInt64 {
		return nil, ErrInvalidSize
	}
	limited := io.LimitReader(reader, maxBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, ErrLimitExceeded
	}
	return data, nil
}

// RejectRedirect is suitable for http.Client.CheckRedirect.
func RejectRedirect(_ *http.Request, _ []*http.Request) error {
	return ErrRedirectRejected
}

// ParseEndpoint accepts HTTPS endpoints and, when explicitly enabled, HTTP
// endpoints whose hostname is loopback. User information and fragments are
// rejected so secrets cannot be hidden in endpoint configuration.
func ParseEndpoint(raw string, allowLoopbackHTTP bool) (*url.URL, error) {
	if len(raw) == 0 || len(raw) > maxEndpointBytes {
		return nil, ErrInvalidEndpoint
	}
	endpoint, err := url.Parse(raw)
	if err != nil || endpoint.Host == "" || endpoint.Hostname() == "" || endpoint.User != nil || endpoint.Fragment != "" {
		return nil, ErrInvalidEndpoint
	}
	if endpoint.Scheme == "https" {
		return endpoint, nil
	}
	if endpoint.Scheme != "http" || !allowLoopbackHTTP || !isLoopbackHost(endpoint.Hostname()) {
		return nil, ErrInvalidEndpoint
	}
	return endpoint, nil
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	address := net.ParseIP(host)
	return address != nil && address.IsLoopback()
}

// ValidateResolvedIPs lets a caller reject an endpoint after DNS resolution.
// Every resolved address must be accepted by allow.
func ValidateResolvedIPs(addresses []net.IP, allow func(net.IP) bool) error {
	if len(addresses) == 0 || allow == nil {
		return ErrInvalidEndpoint
	}
	for _, address := range addresses {
		if address == nil || !allow(address) {
			return fmt.Errorf("%w: resolved address rejected", ErrInvalidEndpoint)
		}
	}
	return nil
}
