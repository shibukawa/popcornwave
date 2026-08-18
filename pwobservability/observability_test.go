package pwobservability

import (
	"strings"
	"testing"

	"github.com/shibukawa/popcornweb/pwconfig"
	"github.com/shibukawa/popcornweb/pwruntime"
)

func TestBuildObservabilityWithoutOtelUsesStdoutOnly(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	resolved, err := Build(pwconfig.ObservabilityConfig{}, pwconfig.EnvProduction)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.logs != nil || resolved.traces != nil || resolved.Tracing() {
		t.Fatal("export was configured with no endpoint anywhere")
	}
	if resolved.Backend().Minimum() != pwruntime.LevelInfo {
		t.Errorf("minimum = %v, want info by default", resolved.Backend().Minimum())
	}
	if !resolved.Backend().Enabled(pwruntime.LevelInfo) {
		t.Error("stdout is the only destination and it was not installed")
	}
}

// The environment variable alone turns export on. This is the whole `pw dev`
// integration: the viewer injects an endpoint and the application finds it
// without a line of configuration.
func TestBuildObservabilityEnablesExportFromTheEnvironment(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://127.0.0.1:19999")
	resolved, err := Build(pwconfig.ObservabilityConfig{}, pwconfig.EnvDevelopment)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.logs == nil || resolved.traces == nil {
		t.Fatal("the injected endpoint did not enable export")
	}
	if !resolved.Tracing() {
		t.Error("tracing stayed off with an exporter configured")
	}
	t.Cleanup(func() { _ = resolved.Shutdown(t.Context()) })
}

// Routing is exclusive outside development: a record goes to the collector or
// to stdout, not both.
func TestBuildObservabilityRoutesExclusivelyOutsideDevelopment(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	resolved, err := Build(pwconfig.ObservabilityConfig{
		Otel: pwconfig.OtelExportConfig{Endpoint: "https://collector.example:4318"},
	}, pwconfig.EnvProduction)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = resolved.Shutdown(t.Context()) })
	if got := resolved.SinkCount(); got != 1 {
		t.Fatalf("sinks = %d, want the collector alone", got)
	}
}

// Development is the exception, because the terminal is the surface the
// developer is watching and a viewer must not empty it.
func TestBuildObservabilityKeepsStdoutInDevelopment(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	resolved, err := Build(pwconfig.ObservabilityConfig{
		Otel: pwconfig.OtelExportConfig{Endpoint: "http://127.0.0.1:19999"},
	}, pwconfig.EnvDevelopment)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = resolved.Shutdown(t.Context()) })
	if got := resolved.SinkCount(); got != 2 {
		t.Fatalf("sinks = %d, want the collector and stdout", got)
	}
}

func TestBuildObservabilityRejectsBadSettings(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	for name, testCase := range map[string]struct {
		config pwconfig.ObservabilityConfig
		want   string
	}{
		"level":    {pwconfig.ObservabilityConfig{MinimumLevel: "verbose"}, "minimum_level"},
		"format":   {pwconfig.ObservabilityConfig{StdoutFormat: "yaml"}, "stdout_format"},
		"headers":  {pwconfig.ObservabilityConfig{Otel: pwconfig.OtelExportConfig{Endpoint: "https://c.example", Headers: "broken"}}, "headers"},
		"endpoint": {pwconfig.ObservabilityConfig{Otel: pwconfig.OtelExportConfig{Endpoint: "ftp://c.example"}}, "endpoint"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := Build(testCase.config, pwconfig.EnvProduction)
			if err == nil || !strings.Contains(err.Error(), testCase.want) {
				t.Fatalf("err = %v, want one naming %s", err, testCase.want)
			}
		})
	}
}

func TestOffSilencesEverySeverity(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	resolved, err := Build(pwconfig.ObservabilityConfig{MinimumLevel: "off"}, pwconfig.EnvProduction)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Backend().Enabled(pwruntime.LevelError) {
		t.Error("a level survived observability.minimum_level = off")
	}
}

// The service name identifies the process to the collector; the environment
// value is the one `pw dev` injects.
func TestResourceAttributesPreferConfigurationOverTheEnvironment(t *testing.T) {
	t.Setenv("OTEL_SERVICE_NAME", "from-env")
	if got := serviceNameOf(resourceAttributes(pwconfig.ObservabilityConfig{ServiceName: "from-config"})); got != "from-config" {
		t.Errorf("service.name = %q, want the configured value", got)
	}
	if got := serviceNameOf(resourceAttributes(pwconfig.ObservabilityConfig{})); got != "from-env" {
		t.Errorf("service.name = %q, want the injected value", got)
	}
}

func TestResourceAttributesCarryExtraIdentifiers(t *testing.T) {
	attributes := resourceAttributes(pwconfig.ObservabilityConfig{
		ServiceName:        "app",
		ResourceAttributes: []string{"deployment.environment=stg", "ignored"},
	})
	if len(attributes) != 2 {
		t.Fatalf("attributes = %d, want the service name and one extra", len(attributes))
	}
	if attributes[1].Key != "deployment.environment" {
		t.Errorf("extra attribute = %q", attributes[1].Key)
	}
}
