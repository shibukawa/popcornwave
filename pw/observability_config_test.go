package pw

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/shibukawa/popcornweb/pwconfig"
	"github.com/shibukawa/tinybind-go/configbind"
)

// loadObservability drives the registered binding the way a real start does,
// through TOML and the environment, and restores the process-wide value
// afterwards so the rest of the package still sees its own configuration.
func loadObservability(t *testing.T, toml string, environ ...string) ObservabilityConfig {
	t.Helper()
	bound, ok := pwconfig.Bound[ObservabilityConfig]()
	if !ok {
		t.Fatal("ObservabilityConfig is not registered")
	}
	previous := *bound
	t.Cleanup(func() { *bound = previous })

	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(toml), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := configbind.Load(configbind.LoadOptions{
		Vendor: "popcornweb-test", Tool: "pw-test", FileName: "config.toml",
		ExplicitConfigPath: path, Args: []string{}, Environ: environ,
	}); err != nil {
		t.Fatalf("load: %v", err)
	}
	return *bound
}

// loadResult is loadObservability when the caller wants the provenance rather
// than the bound value. It repeats what ParseConfig does after loading, which
// is where the derived export switch is written.
func loadResult(t *testing.T, toml string, environ ...string) *configbind.LoadResult {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(toml), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := configbind.Load(configbind.LoadOptions{
		Vendor: "popcornweb-test", Tool: "pw-test", FileName: "config.toml",
		ExplicitConfigPath: path, Args: []string{}, Environ: environ,
	})
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	observability := ObservabilityConfig{}
	pwconfig.DeriveExportEnabled(result, &observability)
	return result
}

func TestObservabilityDefaultsComeFromTheRegistration(t *testing.T) {
	config := loadObservability(t, "")
	if config.MinimumLevel != "info" || config.StdoutFormat != StdoutFormatJSON {
		t.Errorf("minimum_level = %q, stdout_format = %q", config.MinimumLevel, config.StdoutFormat)
	}
	if config.Otel.Enabled || config.Otel.Endpoint != "" {
		t.Errorf("export defaults to on: %+v", config.Otel)
	}
	// The literals are the OtelExportConfig tags, which are the only place these
	// defaults are declared. They restate the bounds the exporter and the batch
	// processors apply to a zero value.
	if config.Otel.RequestTimeout != 10*time.Second || config.Otel.FlushInterval != 5*time.Second {
		t.Errorf("timeout = %v, flush = %v", config.Otel.RequestTimeout, config.Otel.FlushInterval)
	}
	if config.Otel.QueueSize != 2048 || config.Otel.MaxExportSize != 512 {
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

// An endpoint from the environment turns the switch on too, and the summary
// records the environment as the source rather than claiming a default did it.
func TestAnEnvironmentEndpointTurnsExportOn(t *testing.T) {
	result := loadResult(t, "", "OTEL_EXPORTER_OTLP_ENDPOINT=http://127.0.0.1:19999")
	for _, entry := range bootEntries(result) {
		if entry.key != "observability.otel.enabled" {
			continue
		}
		if entry.value != "true" {
			t.Fatalf("enabled = %q, want true", entry.value)
		}
		if entry.source != "env" {
			t.Errorf("source = %q, want the place the endpoint came from", entry.source)
		}
		return
	}
	t.Fatal("the enable flag was not reported at all")
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

// Every export setting answers to otel.enabled, so a run that exports nothing
// reports one line in the startup summary instead of seven. The suppression is
// what makes the flag pw dev injects worth injecting.
func TestExportSettingsAreOmittedWhileExportIsOff(t *testing.T) {
	off := bootKeys(t, "")
	if _, present := off["observability.otel.enabled"]; !present {
		t.Fatal("the switch itself must always be reported")
	}
	for _, key := range []string{
		"observability.otel.endpoint", "observability.otel.headers",
		"observability.otel.request_timeout", "observability.otel.queue_size",
		"observability.otel.max_export_size", "observability.otel.flush_interval",
	} {
		if value, present := off[key]; present {
			t.Errorf("%s = %q was reported with export off", key, value)
		}
	}

	// An endpoint alone is enough: pw derives the switch from it, so the summary
	// never hides an address someone just configured.
	on := bootKeys(t, "[observability.otel]\nendpoint = \"https://collector.example:4318\"\n")
	if on["observability.otel.enabled"] != "true" {
		t.Errorf("enabled = %q, want an endpoint to turn export on", on["observability.otel.enabled"])
	}
	if on["observability.otel.endpoint"] != "https://collector.example:4318" {
		t.Errorf("endpoint = %q, want the configured value once export is on", on["observability.otel.endpoint"])
	}
	for _, key := range []string{
		"observability.otel.request_timeout", "observability.otel.queue_size",
		"observability.otel.max_export_size", "observability.otel.flush_interval",
	} {
		if _, present := on[key]; !present {
			t.Errorf("%s was omitted with export on", key)
		}
	}
}

// bootKeys returns the values policy:startup-summary would report, which is the
// surface dependon acts on.
func bootKeys(t *testing.T, toml string) map[string]string {
	t.Helper()
	result := loadResult(t, toml)
	reported := map[string]string{}
	for _, entry := range bootEntries(result) {
		reported[entry.key] = entry.value
	}
	return reported
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
