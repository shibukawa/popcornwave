// Command fastonly is a whole application that never links the net/http
// runtime: it parses a configuration file, reads a setting back, and serves one
// request through the fasthttp chain that parse published the settings for.
//
// It exists to be a real package rather than a fixture. The claim it stands for
// — that binding configuration no longer requires pw — is a property of a
// linked binary, so it has to be asserted against one the toolchain actually
// resolved. pwconfig's split test runs it and reads what it prints, and checks
// its dependency graph for pw at the same time; a fixture written to a temp
// directory would need its own module and would prove less.
//
// It takes the configuration path as its one argument and prints one line.
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/shibukawa/popcornweb/pwconfig"
	"github.com/shibukawa/popcornweb/pwfast"
	"github.com/shibukawa/tinybind-go/configbind"
	"github.com/shibukawa/tinygodriver/fasthttp"
	"github.com/shibukawa/tinygodriver/fasthttp/fasthttputil"
)

func main() {
	if len(os.Args) < 2 {
		fail(fmt.Errorf("usage: fastonly <config.toml>"))
	}
	pwconfig.SetLoadOptions(configbind.LoadOptions{
		Vendor: "popcornweb-fastonly", Tool: "fastonly", FileName: "config.toml",
		ExplicitConfigPath: os.Args[1], Args: []string{}, Environ: []string{"APP_ENV=dev"},
	})
	if err := pwconfig.Parse(); err != nil {
		fail(err)
	}
	server := pwconfig.Value[pwconfig.ServerConfig]()

	// The chain composes from what the parse published. Without those settings
	// Middlewares refuses rather than building a chain out of zero values, so
	// reaching a response at all is half of what this proves.
	handler, err := pwfast.Middlewares(func(r *fasthttp.RequestCtx) {
		_, _ = r.WriteString("application")
	}, pwfast.RuntimeOptions{})
	if err != nil {
		fail(err)
	}

	status, err := probe(handler, server.Health)
	if err != nil {
		fail(err)
	}
	fmt.Printf("port=%d health=%s probe=%d\n", server.Port, server.Health, status)
}

// probe serves one request over an in-memory pipe, so the program needs no port
// and the test that runs it needs no cleanup.
func probe(handler fasthttp.RequestHandler, path string) (int, error) {
	listener := fasthttputil.NewInmemoryListener()
	defer func() { _ = listener.Close() }()
	go func() { _ = (&fasthttp.Server{Handler: handler}).Serve(listener) }()

	connection, err := listener.Dial()
	if err != nil {
		return 0, err
	}
	defer func() { _ = connection.Close() }()
	request := "GET " + path + " HTTP/1.1\r\nHost: fastonly.test\r\nConnection: close\r\n\r\n"
	if _, err := connection.Write([]byte(request)); err != nil {
		return 0, err
	}
	if err := connection.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return 0, err
	}
	raw, err := io.ReadAll(connection)
	if err != nil && !strings.Contains(err.Error(), "closed") {
		return 0, err
	}
	var response fasthttp.Response
	if err := response.Read(bufio.NewReader(strings.NewReader(string(raw)))); err != nil {
		return 0, err
	}
	return response.StatusCode(), nil
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
