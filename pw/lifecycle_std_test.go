//go:build !tinygo

package pw

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestServeUntilContextGracefullyStops(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})}
	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{})
	serve := func() error {
		close(started)
		return server.Serve(listener)
	}
	result := make(chan error, 1)
	go func() {
		result <- serveUntilContext(ctx, server, listener, serve, time.Second)
	}()
	<-started
	cancel()
	if err := <-result; err != nil {
		t.Fatal(err)
	}
}
