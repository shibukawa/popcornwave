//go:build !tinygo

package pw

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"
)

// TestServeUntilContextGracefullyStops covers the sequence a signal produces: a
// server that is already accepting is asked to stop, and reports nothing.
//
// The cancel waits on a completed request rather than on a channel the serve
// func closes before calling Serve. That earlier signal fired while Serve had
// not yet tracked its listener, so a cancel arriving then reached a Shutdown
// with no listener to close — the one ordering where the close-twice error
// cannot happen. The test passed by missing the case it exists for, and failed
// on the runs where the goroutine got there first.
func TestServeUntilContextGracefullyStops(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})}
	ctx, cancel := context.WithCancel(context.Background())
	serve := func() error { return server.Serve(listener) }
	result := make(chan error, 1)
	go func() {
		result <- serveUntilContext(ctx, server, listener, serve, time.Second)
	}()

	// An answered request is proof that serving is underway: the listener is
	// tracked, and a stop from here is the one an operator actually sends.
	response, err := http.Get("http://" + listener.Addr().String() + "/")
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()

	cancel()
	if err := <-result; err != nil {
		t.Fatal(err)
	}
}

// TestServeUntilContextStopsBeforeServingBegins keeps the other ordering
// covered, since it is the one the test above used to take by accident: a stop
// that arrives while Serve has not yet reached its accept loop.
func TestServeUntilContextStopsBeforeServingBegins(t *testing.T) {
	for attempt := 0; attempt < 50; attempt++ {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		server := &http.Server{Handler: http.NotFoundHandler()}
		ctx, cancel := context.WithCancel(context.Background())
		entered := make(chan struct{})
		serve := func() error {
			close(entered)
			return server.Serve(listener)
		}
		result := make(chan error, 1)
		go func() {
			result <- serveUntilContext(ctx, server, listener, serve, time.Second)
		}()
		<-entered
		cancel()
		if err := <-result; err != nil {
			t.Fatalf("attempt %d: a graceful stop reported: %v", attempt, err)
		}
	}
}
