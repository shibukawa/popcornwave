package pwcli

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/shibukawa/localotelviewer/viewer"
	"github.com/shibukawa/popcornwave/internal/otelui"
)

// Environment variables the telemetry viewer injects into the application
// process. The first three are the OTLP conventions rather than pw-specific
// names, so any exporter finds them and no project commits a development
// endpoint.
const (
	envOTLPEndpoint = "OTEL_EXPORTER_OTLP_ENDPOINT"
	envOTLPProtocol = "OTEL_EXPORTER_OTLP_PROTOCOL"
	envOTLPService  = "OTEL_SERVICE_NAME"
	// envOtelEnabled is the framework's own switch, and every other export
	// setting depends on it. An endpoint alone already turns export on, but the
	// startup summary hides a key whose parent is off, so injecting this is what
	// keeps the address the developer needs to see in that summary.
	envOtelEnabled = "OBSERVABILITY_OTEL_ENABLED"
)

// otlpProtocol selects the encoding both sides already speak. The receiver also
// accepts OTLP JSON, but protobuf is the exporter default and the smaller wire.
const otlpProtocol = "http/protobuf"

// devTelemetryViewer is the running receiver and UI plus the values pw dev
// hands to the application.
type devTelemetryViewer struct {
	server     *viewer.Server
	env        []string
	stopHealth func()
}

// startDevTelemetryViewer serves the OTLP receiver, the snapshot API, and the
// UI from one loopback listener.
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
	server, err := viewer.New("127.0.0.1:"+strconv.Itoa(config.Otel.Port), config.Otel.Max,
		viewer.WithWebHandler(otelui.Handler()))
	if err != nil {
		return nil, err
	}
	telemetry := &devTelemetryViewer{
		server: server,
		env: []string{
			envOtelEnabled + "=true",
			envOTLPEndpoint + "=" + server.URL(),
			envOTLPProtocol + "=" + otlpProtocol,
			envOTLPService + "=" + config.Name,
		},
	}
	telemetry.report(stdout)
	return telemetry, nil
}

func (v *devTelemetryViewer) report(stdout io.Writer) {
	fmt.Fprintf(stdout, "pw dev: telemetry viewer %s\n", v.server.URL())
	fmt.Fprintf(stdout, "pw dev:   traces and logs export to %s as service %q\n", envOTLPEndpoint, v.serviceName())
}

func (v *devTelemetryViewer) serviceName() string {
	for _, entry := range v.env {
		if value, ok := strings.CutPrefix(entry, envOTLPService+"="); ok {
			return value
		}
	}
	return ""
}

// url reports where the viewer listens, or the empty string when none runs.
func (v *devTelemetryViewer) url() string {
	if v == nil {
		return ""
	}
	return v.server.URL()
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
	v.stopHealth = v.server.MonitorProcess(command.Process.Pid)
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
}
