package pwobservability

import (
	"testing"
	"time"

	"github.com/shibukawa/popcornweb/pwconfig"
	"github.com/shibukawa/popcornweb/pwruntime"
)

func queryConfig(mutate func(*pwconfig.QueryLogConfig)) pwconfig.ObservabilityConfig {
	config := pwconfig.ObservabilityConfig{
		MinimumLevel: "info",
		Query: pwconfig.QueryLogConfig{
			Enabled:        QueryToggleAuto,
			Level:          "info",
			SlowThreshold:  defaultQuerySlowThreshold,
			SlowLevel:      "warn",
			BindValues:     QueryToggleAuto,
			Explain:        true,
			Reproduction:   true,
			MaxSQLLength:   defaultQueryMaxSQLLength,
			MaxValueLength: defaultQueryMaxValueLength,
		},
	}
	if mutate != nil {
		mutate(&config.Query)
	}
	return config
}

// Auto is what makes the feature a development aid rather than a setting an
// operator has to remember to turn off.
//
// It follows the declared development environment rather than the resolved one.
// A deployment that never set APP_ENV resolves to "dev" and must not be given a
// log of every statement with its bind values on the strength of that default.
func TestResolveQueryDiagnosticsAutoFollowsDeclaredDevelopment(t *testing.T) {
	if diagnostics := QueryDiagnostics(queryConfig(nil), true); diagnostics == nil {
		t.Fatal("auto should enable query diagnostics in dev")
	} else if !diagnostics.BindValues {
		t.Error("auto should enable bind values in dev")
	}
	if diagnostics := QueryDiagnostics(queryConfig(nil), false); diagnostics != nil {
		t.Error("auto enabled query diagnostics where development was not declared")
	}
}

func TestResolveQueryDiagnosticsExplicitToggles(t *testing.T) {
	on := QueryDiagnostics(queryConfig(func(query *pwconfig.QueryLogConfig) {
		query.Enabled = QueryToggleOn
		query.BindValues = QueryToggleOff
	}), false)
	if on == nil {
		t.Fatal("an explicit on should enable query diagnostics outside dev")
	}
	if on.BindValues {
		t.Error("an explicit off should keep bind values out of production records")
	}

	off := QueryDiagnostics(queryConfig(func(query *pwconfig.QueryLogConfig) {
		query.Enabled = QueryToggleOff
	}), true)
	if off != nil {
		t.Error("an explicit off should win over the development default")
	}
}

func TestResolveQueryDiagnosticsMapsLevelsAndBounds(t *testing.T) {
	diagnostics := QueryDiagnostics(queryConfig(func(query *pwconfig.QueryLogConfig) {
		query.Level = "trace"
		query.SlowLevel = "error"
		query.SlowThreshold = 3 * time.Second
		query.MaxSQLLength = 0
		query.MaxValueLength = 0
	}), true)
	if diagnostics == nil {
		t.Fatal("want diagnostics")
	}
	if diagnostics.Level != pwruntime.LevelDebug-4 {
		t.Errorf("trace level = %v", diagnostics.Level)
	}
	if diagnostics.SlowLevel != pwruntime.LevelError {
		t.Errorf("slow level = %v", diagnostics.SlowLevel)
	}
	if diagnostics.SlowThreshold != 3*time.Second {
		t.Errorf("slow threshold = %v", diagnostics.SlowThreshold)
	}
	// An omitted bound must still bound the record.
	if diagnostics.MaxSQLLength != defaultQueryMaxSQLLength || diagnostics.MaxValueLength != defaultQueryMaxValueLength {
		t.Errorf("unset bounds = %d/%d, want the defaults", diagnostics.MaxSQLLength, diagnostics.MaxValueLength)
	}
}
