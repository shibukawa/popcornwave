//go:build pwdev

package pw

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/shibukawa/popcornwave/pwdata"
	"github.com/shibukawa/popcornwave/pwruntime"
)

// DevAttachTokenVar carries the per-run token pw dev generates. The console
// accepts an announcement only when it matches, so an attachment is not taken
// from anything that merely reached the port.
const DevAttachTokenVar = "PW_DEV_ATTACH_TOKEN"

// startDevelopmentData serves the data pane beside the application.
//
// It listens on a loopback address of its own rather than on the application's
// listener, so nothing here is reachable from the port the application serves,
// and it tells the console where it is rather than waiting to be found.
//
// A failure is reported and dropped. The pane is a reader of the application,
// and an application that runs without one is still an application.
func startDevelopmentData(resources pwruntime.Resources) {
	console := strings.TrimSpace(os.Getenv(DevConsoleURLVar))
	if console == "" || resources.DB == nil {
		return
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fmt.Fprintln(os.Stderr, "pw: development data pane:", err)
		return
	}
	server := pwdata.New(developmentConnections(resources), Env())
	go func() {
		_ = (&http.Server{Handler: server.Handler()}).Serve(listener)
	}()
	announceDevelopmentData(console, listener.Addr().String())
}

// developmentConnections describes every pool the application opened.
//
// A configuration that declares a connection set gets one entry per connection
// rather than one per group: selection inside a group is round robin, so a pane
// addressing the group could not say which replica answered, and whether a
// replica has caught up is the one question replicas raise.
//
// The driver travels with each connection, because nothing forbids two groups
// on two engines and the dialect is resolved from it.
func developmentConnections(resources pwruntime.Resources) []pwdata.Connection {
	if resources.Connections == nil {
		return []pwdata.Connection{
			pwdata.NewConnection("default", "default", resources.DBDriver, false, resources.DB),
		}
	}
	var connections []pwdata.Connection
	for _, connection := range resources.Connections.Connections() {
		connections = append(connections, pwdata.NewConnection(
			connection.Label, connection.Group, connection.Driver, connection.ReadOnly, connection.DB))
	}
	if len(connections) == 0 {
		return []pwdata.Connection{
			pwdata.NewConnection("default", "default", resources.DBDriver, false, resources.DB),
		}
	}
	return connections
}

// announceDevelopmentData tells the console the address to proxy to.
//
// The application dials out; the console never dials in. That is what keeps the
// pane off the application's own listener while still letting one page reach
// it, and it is the direction the telemetry exporter already uses.
func announceDevelopmentData(console, address string) {
	request, err := http.NewRequest(http.MethodPost, console+"/api/attach", strings.NewReader(address))
	if err != nil {
		return
	}
	request.Header.Set("Content-Type", "text/plain")
	request.Header.Set("X-Pw-Attach-Token", os.Getenv(DevAttachTokenVar))
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		// The console may not be running, which is ordinary: the developer can
		// have disabled it, and the application does not depend on it.
		return
	}
	_ = response.Body.Close()
}
