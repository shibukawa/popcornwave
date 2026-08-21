package testutil

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/shibukawa/popcornweb/pwtest"
	"github.com/shibukawa/tinygodriver/fasthttp/fasthttputil"
)

// Harness is one running server and one client, shared by every exchange of a
// test.
//
// Exchange starts a fresh server, listener and goroutine per call and keeps
// all of them alive until the test ends, so a test driving one handler with N
// requests held N listeners and paid N startups. A harness starts the server
// once, and its client reuses connections — which is also what lets a
// sequence of exchanges meet keep-alive the way a browser would.
type Harness struct {
	t      pwtest.TestingT
	client *http.Client
}

// NewHarness starts a real net/http server for handler and returns the
// harness that drives it. The server is shut down by t.Cleanup.
func NewHarness(t pwtest.TestingT, handler http.Handler) *Harness {
	t.Helper()
	listener := fasthttputil.NewInmemoryListener()
	server := &http.Server{Handler: handler}
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = server.Serve(listener)
	}()
	t.Cleanup(func() {
		_ = server.Close()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Errorf("the net/http server did not shut down")
		}
	})
	client := &http.Client{Transport: &http.Transport{
		DialContext: func(context.Context, string, string) (net.Conn, error) { return listener.Dial() },
	}}
	return &Harness{t: t, client: client}
}

// Exchange runs one request through the harness's server and returns what it
// answered.
func (h *Harness) Exchange(request pwtest.Request) pwtest.Response {
	h.t.Helper()
	built, err := http.NewRequest(request.ResolvedMethod(),
		"http://memory.test"+request.ResolvedTarget(), bytes.NewReader(request.Body))
	if err != nil {
		h.t.Fatalf("testutil: unusable request: %v", err)
		return pwtest.Response{}
	}
	for _, header := range request.SortedHeader() {
		built.Header.Add(header.Name, header.Value)
	}
	answered, err := h.client.Do(built)
	if err != nil {
		h.t.Fatalf("testutil: request failed: %v", err)
		return pwtest.Response{}
	}
	defer func() { _ = answered.Body.Close() }()
	body, err := io.ReadAll(answered.Body)
	if err != nil {
		h.t.Fatalf("testutil: unreadable response body: %v", err)
		return pwtest.Response{}
	}
	return pwtest.Response{Status: answered.StatusCode, Header: answered.Header, Body: body}
}

// Exchange runs one request through handler on a real net/http server and
// returns what it answered.
//
// It is the net/http half of the backend-neutral seam. The fasthttp half has
// the same name and the same signature apart from the handler type, so a test
// written against it says the same thing on either transport and only the
// import line moves.
//
// The server is real and the connection is not: an in-memory pipe carries it,
// so the test pays a full request parse and response serialization — which is
// where half the behaviour worth testing lives — without paying a socket. That
// is also what makes it a fair pair with the other half, which runs the same
// way.
//
// Each call starts and keeps a server for the rest of the test; a test making
// several requests against one handler builds a Harness once instead.
func Exchange(t pwtest.TestingT, handler http.Handler, request pwtest.Request) pwtest.Response {
	t.Helper()
	return NewHarness(t, handler).Exchange(request)
}
