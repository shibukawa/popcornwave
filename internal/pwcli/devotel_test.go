package pwcli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"
)

const otelProject = `
[project]
name = "app"
main = "./cmd/app"

[generate]
handlers = []
templates = []
queries = []
config = []
`

// One OTLP/HTTP JSON trace export, which the receiver accepts alongside
// protobuf. The identifiers are the hex form protojson expects.
const otlpTraceExport = `{
  "resourceSpans": [{
    "resource": {"attributes": [{"key": "service.name", "value": {"stringValue": "app"}}]},
    "scopeSpans": [{
      "spans": [{
        "traceId": "0102030405060708090a0b0c0d0e0f10",
        "spanId": "0102030405060708",
        "name": "GET /users",
        "kind": 2,
        "startTimeUnixNano": "1700000000000000000",
        "endTimeUnixNano": "1700000000500000000"
      }]
    }]
  }]
}`

type snapshotBody struct {
	Services []string `json:"services"`
	Traces   []struct {
		TraceID string `json:"TraceID"`
		Service string `json:"Service"`
		Name    string `json:"Name"`
	} `json:"traces"`
}

func startTestViewer(t *testing.T, config projectConfig) (*devTelemetryViewer, *bytes.Buffer) {
	t.Helper()
	stdout := &bytes.Buffer{}
	telemetry, err := startDevTelemetryViewer(config, stdout)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(telemetry.close)
	return telemetry, stdout
}

func TestStartDevTelemetryViewerListensOnAFreeLoopbackPort(t *testing.T) {
	root := writeProject(t, map[string]string{"popcornwave.toml": otelProject})
	config, err := loadProjectConfig(root)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	telemetry, stdout := startTestViewer(t, config)

	url := telemetry.url()
	if !strings.HasPrefix(url, "http://127.0.0.1:") {
		t.Fatalf("url = %q, want a loopback address", url)
	}
	if strings.HasSuffix(url, ":0") {
		t.Fatalf("url = %q, want a resolved port", url)
	}
	if !strings.Contains(stdout.String(), url) {
		t.Fatalf("the developer loop never reported the viewer URL:\n%s", stdout)
	}
}

func TestDevTelemetryViewerInjectsTheResolvedEndpoint(t *testing.T) {
	root := writeProject(t, map[string]string{"popcornwave.toml": otelProject})
	config, err := loadProjectConfig(root)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	telemetry, _ := startTestViewer(t, config)

	environ := telemetry.environ(nil)
	want := map[string]string{
		// The enable flag rides along because every other export setting depends
		// on it, and a key whose parent is off is left out of the startup summary.
		envOtelEnabled:  "true",
		envOTLPEndpoint: telemetry.url(),
		envOTLPProtocol: otlpProtocol,
		envOTLPService:  "app",
	}
	got := map[string]string{}
	for _, entry := range environ {
		name, value, _ := strings.Cut(entry, "=")
		got[name] = value
	}
	for name, value := range want {
		if got[name] != value {
			t.Errorf("%s = %q, want %q", name, got[name], value)
		}
	}
}

// A value the developer exported wins, matching the identity provider. Their
// own service name or protocol choice is deliberate and must survive pw dev.
func TestDevTelemetryViewerPreservesADeveloperEnvironmentValue(t *testing.T) {
	t.Setenv(envOTLPService, "chosen-by-hand")
	root := writeProject(t, map[string]string{"popcornwave.toml": otelProject})
	config, err := loadProjectConfig(root)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	telemetry, _ := startTestViewer(t, config)

	for _, entry := range telemetry.environ(nil) {
		if strings.HasPrefix(entry, envOTLPService+"=") {
			t.Fatalf("pw dev overwrote an exported service name: %q", entry)
		}
	}
}

// A developer already pointing the application at their own collector gets no
// viewer, because one nothing exports to is a held port and an empty page.
func TestStartDevTelemetryViewerSkipsAConfiguredEndpoint(t *testing.T) {
	t.Setenv(envOTLPEndpoint, "http://127.0.0.1:4318")
	root := writeProject(t, map[string]string{"popcornwave.toml": otelProject})
	config, err := loadProjectConfig(root)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	stdout := &bytes.Buffer{}
	telemetry, err := startDevTelemetryViewer(config, stdout)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if telemetry != nil {
		telemetry.close()
		t.Fatal("expected no viewer while an endpoint is exported")
	}
	if !strings.Contains(stdout.String(), envOTLPEndpoint) {
		t.Fatalf("the developer loop never explained the skip:\n%s", stdout)
	}
	// A nil viewer must stay usable everywhere the loop touches it.
	if telemetry.url() != "" {
		t.Errorf("url = %q, want empty", telemetry.url())
	}
	telemetry.monitor(nil)
	telemetry.close()
	if got := telemetry.environ([]string{"A=B"}); len(got) != 1 {
		t.Errorf("environ = %v, want the base unchanged", got)
	}
}

// The end-to-end contract: what an exporter posts to the injected endpoint is
// what the UI reads back from the snapshot API.
func TestDevTelemetryViewerReceivesAndServesTelemetry(t *testing.T) {
	root := writeProject(t, map[string]string{"popcornwave.toml": otelProject})
	config, err := loadProjectConfig(root)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	telemetry, _ := startTestViewer(t, config)

	response, err := http.Post(telemetry.url()+"/v1/traces", "application/json", strings.NewReader(otlpTraceExport))
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("export status = %d, want %d", response.StatusCode, http.StatusOK)
	}

	snapshotResponse, err := http.Get(telemetry.url() + "/api/snapshot")
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	defer snapshotResponse.Body.Close()
	var snapshot snapshotBody
	if err := json.NewDecoder(snapshotResponse.Body).Decode(&snapshot); err != nil {
		t.Fatalf("decode snapshot: %v", err)
	}
	if len(snapshot.Traces) != 1 {
		t.Fatalf("traces = %d, want 1", len(snapshot.Traces))
	}
	if snapshot.Traces[0].Name != "GET /users" {
		t.Errorf("trace name = %q", snapshot.Traces[0].Name)
	}
	if snapshot.Traces[0].Service != "app" {
		t.Errorf("trace service = %q, want the project name", snapshot.Traces[0].Service)
	}

	// The UI is mounted on the same listener, so the endpoint pw dev prints is
	// both the export target and the page the developer opens.
	uiResponse, err := http.Get(telemetry.url() + "/")
	if err != nil {
		t.Fatalf("ui: %v", err)
	}
	defer uiResponse.Body.Close()
	if uiResponse.StatusCode != http.StatusOK {
		t.Fatalf("ui status = %d, want %d", uiResponse.StatusCode, http.StatusOK)
	}
	body := &bytes.Buffer{}
	if _, err := body.ReadFrom(uiResponse.Body); err != nil {
		t.Fatalf("read ui: %v", err)
	}
	if !bytes.Contains(body.Bytes(), []byte("<div id=\"root\">")) {
		t.Fatalf("the mounted page is not the viewer UI:\n%s", body)
	}
}

// The whole point of the feature: the process pw dev starts is pointed at the
// viewer without the project committing an endpoint anywhere.
func TestStartApplicationPointsTheProcessAtTheViewer(t *testing.T) {
	root := writeProject(t, map[string]string{
		"popcornwave.toml": otelProject,
		"go.mod":           "module devotelprobe\n\ngo 1.26.0\n",
		"cmd/app/main.go": `package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Println(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"))
	fmt.Println(os.Getenv("OTEL_EXPORTER_OTLP_PROTOCOL"))
	fmt.Println(os.Getenv("OTEL_SERVICE_NAME"))
}
`,
	})
	config, err := loadProjectConfig(root)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	telemetry, _ := startTestViewer(t, config)

	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	app, exited, err := startApplication(ctx, root, "./cmd/app", nil, telemetry, stdout, stderr)
	if err != nil {
		t.Fatalf("start application: %v", err)
	}
	defer stopCommand(app)
	telemetry.monitor(app)
	if err := <-exited; err != nil {
		t.Fatalf("application exited: %v\n%s", err, stderr)
	}

	want := telemetry.url() + "\n" + otlpProtocol + "\napp\n"
	if stdout.String() != want {
		t.Fatalf("application environment =\n%q\nwant\n%q", stdout, want)
	}
}

func TestLoadProjectConfigEnablesTheViewerByDefault(t *testing.T) {
	root := writeProject(t, map[string]string{"popcornwave.toml": otelProject})
	config, err := loadProjectConfig(root)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !config.Otel.Enabled {
		t.Error("expected the telemetry viewer to be enabled by default")
	}
	if config.Otel.Port != 0 {
		t.Errorf("port = %d, want an automatically reserved port", config.Otel.Port)
	}
	if config.Otel.Max != 0 {
		t.Errorf("max = %d, want the viewer default", config.Otel.Max)
	}
}

func TestLoadProjectConfigReadsDevOtel(t *testing.T) {
	root := writeProject(t, map[string]string{"popcornwave.toml": otelProject + `
[dev.otel]
enabled = false
port = 4318
max = 500
`})
	config, err := loadProjectConfig(root)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if config.Otel.Enabled {
		t.Error("expected dev.otel.enabled to be honoured")
	}
	if config.Otel.Port != 4318 {
		t.Errorf("port = %d, want 4318", config.Otel.Port)
	}
	if config.Otel.Max != 500 {
		t.Errorf("max = %d, want 500", config.Otel.Max)
	}
}

func TestLoadProjectConfigRejectsABadOtelPort(t *testing.T) {
	root := writeProject(t, map[string]string{"popcornwave.toml": otelProject + `
[dev.otel]
port = 70000
`})
	if _, err := loadProjectConfig(root); err == nil || !strings.Contains(err.Error(), "dev.otel.port") {
		t.Fatalf("err = %v, want a dev.otel.port range error", err)
	}
}
