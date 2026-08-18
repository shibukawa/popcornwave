// Package fasttestutil is the fasthttp half of the backend-neutral test seam.
//
// It declares Exchange with the name, the parameter order and the neutral
// request and response types testutil's has, so a test moves between the two by
// changing its import and nothing else — the same relationship pwfast has with
// pw, applied to the harness a test drives them through.
//
// It is a package of its own rather than a file inside testutil because
// testutil is a shipped package that a net/http-only project imports, and
// putting the fasthttp fork behind that import would hand the fork to every
// such project.
package fasttestutil

import (
	"net"
	"time"

	"github.com/shibukawa/popcornweb/pwtest"
	"github.com/shibukawa/tinygodriver/fasthttp"
	"github.com/shibukawa/tinygodriver/fasthttp/fasthttputil"
)

// Exchange runs one request through handler on a real fasthttp server and
// returns what it answered.
//
// Real server, in-memory connection, the same as the other half: the request is
// parsed and the response serialized, so what is tested is what the transport
// does rather than what a hand-built request value was told to say.
func Exchange(t pwtest.TestingT, handler fasthttp.RequestHandler, request pwtest.Request) pwtest.Response {
	t.Helper()
	listener := fasthttputil.NewInmemoryListener()
	server := &fasthttp.Server{Handler: handler}
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = server.Serve(listener)
	}()
	t.Cleanup(func() {
		_ = server.Shutdown()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Errorf("the fasthttp server did not shut down")
		}
	})

	client := &fasthttp.Client{Dial: func(string) (net.Conn, error) { return listener.Dial() }}
	outgoing, incoming := fasthttp.AcquireRequest(), fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(outgoing)
	defer fasthttp.ReleaseResponse(incoming)

	outgoing.SetRequestURI("http://memory.test" + request.ResolvedTarget())
	outgoing.Header.SetMethod(request.ResolvedMethod())
	for _, header := range request.SortedHeader() {
		outgoing.Header.Add(header.Name, header.Value)
	}
	if len(request.Body) > 0 {
		outgoing.SetBody(request.Body)
	}
	if err := client.Do(outgoing, incoming); err != nil {
		t.Fatalf("fasttestutil: request failed: %v", err)
		return pwtest.Response{}
	}

	header := map[string][]string{}
	incoming.Header.VisitAll(func(name, value []byte) {
		key := string(name)
		header[key] = append(header[key], string(value))
	})
	return pwtest.Response{
		Status: incoming.StatusCode(),
		Header: header,
		// The body is copied because the response value goes back to the pool
		// when this function returns, and a caller holding the slice would be
		// reading whatever the next request put there.
		Body: append([]byte(nil), incoming.Body()...),
	}
}
