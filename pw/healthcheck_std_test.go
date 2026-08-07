//go:build !tinygo

package pw

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

func listenerPort(t *testing.T, server *httptest.Server) int {
	t.Helper()
	_, portText, err := net.SplitHostPort(server.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatal(err)
	}
	return port
}

func TestProbeOperationalEndpointReportsStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s", r.Method)
		}
		switch r.URL.Path {
		case "/healthz":
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	}))
	defer server.Close()
	port := listenerPort(t, server)

	status, err := probeOperationalEndpoint(port, "/healthz", time.Second)
	if err != nil || status != http.StatusOK {
		t.Fatalf("status = %d, err = %v", status, err)
	}
	status, err = probeOperationalEndpoint(port, "/readyz", time.Second)
	if err != nil || status != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, err = %v", status, err)
	}
}

func TestProbeOperationalEndpointConnectionRefused(t *testing.T) {
	// A closed listener's port is the closest stand-in for a crashed server.
	server := httptest.NewServer(http.NotFoundHandler())
	port := listenerPort(t, server)
	server.Close()

	if _, err := probeOperationalEndpoint(port, "/healthz", time.Second); err == nil {
		t.Fatal("probe against a closed port succeeded")
	}
}

func TestProbeOperationalEndpointTimeout(t *testing.T) {
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release
	}))
	// Deferred in this order because Close waits for the handler to return,
	// and the handler returns only once release is closed.
	defer server.Close()
	defer close(release)

	start := time.Now()
	_, err := probeOperationalEndpoint(listenerPort(t, server), "/healthz", 100*time.Millisecond)
	if err == nil {
		t.Fatal("hung endpoint reported healthy")
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("probe took %v against a 100ms deadline", elapsed)
	}
}
