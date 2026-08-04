package pwcli

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/shibukawa/localotelviewer/viewer"
	"github.com/shibukawa/localotelviewer/viewer/webui"
)

// Environment variables the telemetry viewer injects into the application
// process. They are the OTLP conventions rather than pw-specific names, so any
// exporter finds them and no project commits a development endpoint.
//
// The framework's own export switch is not among them: naming an endpoint turns
// export on by itself, and pw derives the switch from it so that every setting
// depending on it still reaches the startup summary.
const (
	envOTLPEndpoint = "OTEL_EXPORTER_OTLP_ENDPOINT"
	envOTLPProtocol = "OTEL_EXPORTER_OTLP_PROTOCOL"
	envOTLPService  = "OTEL_SERVICE_NAME"
)

// otlpProtocol selects the encoding both sides already speak. The receiver also
// accepts OTLP JSON, but protobuf is the exporter default and the smaller wire.
const otlpProtocol = "http/protobuf"

// devTelemetryViewer is the running receiver plus the values pw dev hands to
// the application.
//
// The receiver and the UI are two mounts of one handler, so they share a store
// while answering on different addresses. They want opposite things from a
// port: the receiver publishes an address a process is handed, so a reserved
// port costs nothing, and the UI is an address a person returns to, so it lives
// on the fixed console port instead.
type devTelemetryViewer struct {
	handler    *viewer.Handler
	server     *http.Server
	listener   net.Listener
	env        []string
	stopHealth func()
}

// startDevTelemetryViewer serves the OTLP receiver and the snapshot API on a
// loopback listener of its own. The UI is mounted separately, by the console.
//
// The port defaults to 0 so the operating system chooses a free one: the
// endpoint is injected rather than written down, which makes a fixed number
// unnecessary and lets several projects run pw dev at the same time.
//
// A nil viewer is a normal outcome, not a failure. It means the developer
// already pointed the application at a collector of their own, and a viewer
// nothing exports to would hold a port to show an empty page.
func startDevTelemetryViewer(config projectConfig, stdout io.Writer) (*devTelemetryViewer, error) {
	if value, ok := os.LookupEnv(envOTLPEndpoint); ok && strings.TrimSpace(value) != "" {
		fmt.Fprintf(stdout, "pw dev: telemetry viewer skipped; %s already points at %s\n", envOTLPEndpoint, value)
		return nil, nil
	}
	// The UI comes from the dependency rather than from a build committed here.
	// Importing viewer alone still links no assets, so this import is the
	// explicit choice to take them.
	handler := viewer.NewHandler(config.Otel.Max, viewer.WithWebHandler(webui.Handler()))
	listener, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(config.Otel.Port))
	if err != nil {
		return nil, err
	}
	telemetry := &devTelemetryViewer{
		handler:  handler,
		listener: listener,
		server:   &http.Server{Handler: handler},
		env: []string{
			envOTLPEndpoint + "=http://" + listener.Addr().String(),
			envOTLPProtocol + "=" + otlpProtocol,
			envOTLPService + "=" + config.Name,
		},
	}
	go telemetry.server.Serve(listener)
	telemetry.report(stdout)
	return telemetry, nil
}

// paneHandler is the viewer mounted for the console: the UI it serves for every
// path the receiver and snapshot API do not claim.
//
// The snapshot API is claimed at the console root rather than under the pane
// prefix, because the committed UI bundle resolves its fetch against the
// document origin. See devconsole.Pane.RootPaths.
func (v *devTelemetryViewer) paneHandler() http.Handler {
	if v == nil {
		return nil
	}
	return v.handler
}

// report names the receiver address rather than a page to open. The page is the
// console's telemetry pane; this address is what the application exports to,
// and it is printed because an exporter of the developer's own may have to be
// pointed at it.
func (v *devTelemetryViewer) report(stdout io.Writer) {
	fmt.Fprintf(stdout, "pw dev: telemetry receiver %s\n", v.url())
	fmt.Fprintf(stdout, "pw dev:   traces and logs export there as service %q, and read on the console\n", v.serviceName())
}

func (v *devTelemetryViewer) serviceName() string {
	for _, entry := range v.env {
		if value, ok := strings.CutPrefix(entry, envOTLPService+"="); ok {
			return value
		}
	}
	return ""
}

// url reports where the receiver listens, or the empty string when none runs.
func (v *devTelemetryViewer) url() string {
	if v == nil {
		return ""
	}
	return "http://" + v.listener.Addr().String()
}

// environ returns the process environment for the application, preserving any
// value the developer already exported.
func (v *devTelemetryViewer) environ(base []string) []string {
	if v == nil {
		return base
	}
	for _, entry := range v.env {
		name, _, _ := strings.Cut(entry, "=")
		if value, ok := os.LookupEnv(name); ok && strings.TrimSpace(value) != "" {
			continue
		}
		base = append(base, entry)
	}
	return base
}

// monitor samples the health of the application process into the snapshot the
// UI reads. The previous sampler stops first, because pw dev replaces the
// process on every rebuild and the old pid is gone by then.
func (v *devTelemetryViewer) monitor(command *exec.Cmd) {
	if v == nil {
		return
	}
	if v.stopHealth != nil {
		v.stopHealth()
		v.stopHealth = nil
	}
	if command == nil || command.Process == nil {
		return
	}
	v.stopHealth = v.handler.MonitorProcess(command.Process.Pid)
}

// close stops the viewer with the developer loop. Telemetry is held in memory
// only, so there is nothing to flush and nothing survives the run.
func (v *devTelemetryViewer) close() {
	if v == nil {
		return
	}
	if v.stopHealth != nil {
		v.stopHealth()
		v.stopHealth = nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = v.server.Shutdown(ctx)
	_ = v.listener.Close()
}
