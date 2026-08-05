package pw

import (
	"strings"
	"testing"
	"time"
)

func queryConfig(mutate func(*QueryLogConfig)) ObservabilityConfig {
	config := ObservabilityConfig{
		MinimumLevel: "info",
		Query: QueryLogConfig{
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
	if diagnostics := resolveQueryDiagnostics(queryConfig(nil), true); diagnostics == nil {
		t.Fatal("auto should enable query diagnostics in dev")
	} else if !diagnostics.BindValues {
		t.Error("auto should enable bind values in dev")
	}
	if diagnostics := resolveQueryDiagnostics(queryConfig(nil), false); diagnostics != nil {
		t.Error("auto enabled query diagnostics where development was not declared")
	}
}

func TestResolveQueryDiagnosticsExplicitToggles(t *testing.T) {
	on := resolveQueryDiagnostics(queryConfig(func(query *QueryLogConfig) {
		query.Enabled = QueryToggleOn
		query.BindValues = QueryToggleOff
	}), false)
	if on == nil {
		t.Fatal("an explicit on should enable query diagnostics outside dev")
	}
	if on.BindValues {
		t.Error("an explicit off should keep bind values out of production records")
	}

	off := resolveQueryDiagnostics(queryConfig(func(query *QueryLogConfig) {
		query.Enabled = QueryToggleOff
	}), true)
	if off != nil {
		t.Error("an explicit off should win over the development default")
	}
}

func TestResolveQueryDiagnosticsMapsLevelsAndBounds(t *testing.T) {
	diagnostics := resolveQueryDiagnostics(queryConfig(func(query *QueryLogConfig) {
		query.Level = "trace"
		query.SlowLevel = "error"
		query.SlowThreshold = 3 * time.Second
		query.MaxSQLLength = 0
		query.MaxValueLength = 0
	}), true)
	if diagnostics == nil {
		t.Fatal("want diagnostics")
	}
	if diagnostics.Level != LevelDebug-4 {
		t.Errorf("trace level = %v", diagnostics.Level)
	}
	if diagnostics.SlowLevel != LevelError {
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

func TestValidateQueryLogConfig(t *testing.T) {
	if err := validateQueryLogConfig(queryConfig(nil).Query); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		mutate func(*QueryLogConfig)
		want   string
	}{
		{name: "enabled", mutate: func(q *QueryLogConfig) { q.Enabled = "yes" }, want: "observability.query.enabled"},
		{name: "bind values", mutate: func(q *QueryLogConfig) { q.BindValues = "maybe" }, want: "observability.query.bind_values"},
		{name: "level", mutate: func(q *QueryLogConfig) { q.Level = "verbose" }, want: "observability.query.level"},
		{name: "slow level", mutate: func(q *QueryLogConfig) { q.SlowLevel = "off" }, want: "observability.query.slow_level"},
		{name: "threshold", mutate: func(q *QueryLogConfig) { q.SlowThreshold = -time.Second }, want: "slow_threshold"},
		{name: "sql bound", mutate: func(q *QueryLogConfig) { q.MaxSQLLength = -1 }, want: "max_sql_length"},
		{name: "value bound", mutate: func(q *QueryLogConfig) { q.MaxValueLength = -1 }, want: "max_value_length"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateQueryLogConfig(queryConfig(test.mutate).Query)
			if err == nil {
				t.Fatalf("want an error naming %s", test.want)
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Errorf("error = %v, want it to name %s", err, test.want)
			}
		})
	}
}

// A zero threshold turns off slow detection, which is what disables EXPLAIN and
// the rerun snippet without touching their own switches.
func TestZeroSlowThresholdIsValid(t *testing.T) {
	if err := validateQueryLogConfig(queryConfig(func(q *QueryLogConfig) { q.SlowThreshold = 0 }).Query); err != nil {
		t.Fatal(err)
	}
}
