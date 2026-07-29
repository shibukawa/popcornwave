package pw

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/shibukawa/tinybind-go/configbind"
)

// loadObservability drives the registered binding the way a real start does,
// through TOML and the environment, and restores the process-wide value
// afterwards so the rest of the package still sees its own configuration.
func loadObservability(t *testing.T, toml string, environ ...string) ObservabilityConfig {
	t.Helper()
	configState.RLock()
	entry, ok := configState.entries[reflect.TypeFor[ObservabilityConfig]()]
	configState.RUnlock()
	if !ok {
		t.Fatal("ObservabilityConfig is not registered")
	}
	bound, ok := entry.ptr.(*ObservabilityConfig)
	if !ok {
		t.Fatalf("binding = %T, want *ObservabilityConfig", entry.ptr)
	}
	previous := *bound
	t.Cleanup(func() { *bound = previous })

	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(toml), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := configbind.Load(configbind.LoadOptions{
		Vendor: "popcornwave-test", Tool: "pw-test", FileName: "config.toml",
		ExplicitConfigPath: path, Args: []string{}, Environ: environ,
	}); err != nil {
		t.Fatalf("load: %v", err)
	}
	return *bound
}

func TestObservabilityDefaultsComeFromTheRegistration(t *testing.T) {
	config := loadObservability(t, "")
	if config.MinimumLevel != "info" || config.StdoutFormat != StdoutFormatJSON {
		t.Errorf("minimum_level = %q, stdout_format = %q", config.MinimumLevel, config.StdoutFormat)
	}
	if config.Otel.Enabled || config.Otel.Endpoint != "" {
		t.Errorf("export defaults to on: %+v", config.Otel)
	}
	if config.Otel.RequestTimeout != defaultOtelRequestTimeout || config.Otel.FlushInterval != defaultOtelFlushInterval {
		t.Errorf("timeout = %v, flush = %v", config.Otel.RequestTimeout, config.Otel.FlushInterval)
	}
	if config.Otel.QueueSize != defaultOtelQueueSize || config.Otel.MaxExportSize != defaultOtelMaxExportSize {
		t.Errorf("queue = %d, batch = %d", config.Otel.QueueSize, config.Otel.MaxExportSize)
	}
}

// The deployment case the environment alone could not serve: an endpoint and
// its export bounds written in the project's own configuration file.
func TestObservabilityReadsOtelFromTOML(t *testing.T) {
	config := loadObservability(t, `
[observability]
minimum_level = "warn"
stdout_format = "plaintext"
service_name = "billing"
resource_attributes = ["deployment.environment=stg", "service.version=1.4.0"]

[observability.otel]
enabled = true
endpoint = "https://collector.internal:4318"
headers = "authorization=Bearer token,x-tenant=acme"
request_timeout = "3s"
queue_size = 4096
max_export_size = 256
flush_interval = "1s"
`)
	if config.MinimumLevel != "warn" || config.StdoutFormat != StdoutFormatPlaintext {
		t.Errorf("minimum_level = %q, stdout_format = %q", config.MinimumLevel, config.StdoutFormat)
	}
	if config.ServiceName != "billing" {
		t.Errorf("service_name = %q", config.ServiceName)
	}
	if len(config.ResourceAttributes) != 2 || config.ResourceAttributes[0] != "deployment.environment=stg" {
		t.Errorf("resource_attributes = %v", config.ResourceAttributes)
	}
	if !config.Otel.Enabled || config.Otel.Endpoint != "https://collector.internal:4318" {
		t.Errorf("otel = %+v", config.Otel)
	}
	if config.Otel.RequestTimeout != 3*time.Second || config.Otel.FlushInterval != time.Second {
		t.Errorf("timeout = %v, flush = %v", config.Otel.RequestTimeout, config.Otel.FlushInterval)
	}
	if config.Otel.QueueSize != 4096 || config.Otel.MaxExportSize != 256 {
		t.Errorf("queue = %d, batch = %d", config.Otel.QueueSize, config.Otel.MaxExportSize)
	}
	if !strings.Contains(config.Otel.Headers, "x-tenant=acme") {
		t.Errorf("headers = %q", config.Otel.Headers)
	}
}

// The standard OTLP variables bind to the same fields, which is what makes the
// `pw dev` injection work with no configuration file at all.
func TestObservabilityReadsTheStandardOtlpEnvironment(t *testing.T) {
	config := loadObservability(t, "",
		"OTEL_EXPORTER_OTLP_ENDPOINT=http://127.0.0.1:19999",
		"OTEL_EXPORTER_OTLP_HEADERS=x-tenant=acme",
		"OTEL_SERVICE_NAME=helloworld",
	)
	if config.Otel.Endpoint != "http://127.0.0.1:19999" {
		t.Errorf("endpoint = %q", config.Otel.Endpoint)
	}
	if config.Otel.Headers != "x-tenant=acme" {
		t.Errorf("headers = %q", config.Otel.Headers)
	}
	if config.ServiceName != "helloworld" {
		t.Errorf("service_name = %q", config.ServiceName)
	}
}

// Precedence is the ordinary one: the environment outranks the file, so an
// operator can redirect a deployed process without editing what it ships with.
func TestObservabilityEnvironmentOutranksTheFile(t *testing.T) {
	config := loadObservability(t, `
[observability.otel]
endpoint = "https://collector.internal:4318"
`, "OTEL_EXPORTER_OTLP_ENDPOINT=http://127.0.0.1:19999")
	if config.Otel.Endpoint != "http://127.0.0.1:19999" {
		t.Errorf("endpoint = %q, want the environment value", config.Otel.Endpoint)
	}
}

// A scaffolded project has to show the export settings, or the only way to
// discover them is to read the framework source.
func TestScaffoldsIncludeTheExportSettings(t *testing.T) {
	toml, err := ScaffoldTOML()
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{
		`stdout_format = "json"`, `otel.enabled = false`, `otel.endpoint = ""`,
		`otel.request_timeout = "10s"`, "otel.queue_size = 2048",
	} {
		if !strings.Contains(toml, fragment) {
			t.Errorf("TOML scaffold missing %q:\n%s", fragment, toml)
		}
	}
	env, err := ScaffoldEnv()
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{"OTEL_EXPORTER_OTLP_ENDPOINT=", "OTEL_EXPORTER_OTLP_HEADERS="} {
		if !strings.Contains(env, fragment) {
			t.Errorf("env scaffold missing %q:\n%s", fragment, env)
		}
	}
}
