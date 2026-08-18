package pw

import (
	"strings"
	"testing"
	"time"

	"github.com/shibukawa/popcornweb/pwconfig"
	"github.com/shibukawa/popcornweb/pwobservability"
)

func queryConfig(mutate func(*pwconfig.QueryLogConfig)) pwconfig.ObservabilityConfig {
	config := pwconfig.ObservabilityConfig{
		MinimumLevel: "info",
		Query: pwconfig.QueryLogConfig{
			Enabled:        QueryToggleAuto,
			Level:          "info",
			SlowThreshold:  pwobservability.DefaultSlowThreshold,
			SlowLevel:      "warn",
			BindValues:     QueryToggleAuto,
			Explain:        true,
			Reproduction:   true,
			MaxSQLLength:   pwobservability.DefaultMaxSQLLength,
			MaxValueLength: pwobservability.DefaultMaxValueLength,
		},
	}
	if mutate != nil {
		mutate(&config.Query)
	}
	return config
}

func TestValidateQueryLogConfig(t *testing.T) {
	if err := validateQueryLogConfig(queryConfig(nil).Query); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		mutate func(*pwconfig.QueryLogConfig)
		want   string
	}{
		{name: "enabled", mutate: func(q *pwconfig.QueryLogConfig) { q.Enabled = "yes" }, want: "observability.query.enabled"},
		{name: "bind values", mutate: func(q *pwconfig.QueryLogConfig) { q.BindValues = "maybe" }, want: "observability.query.bind_values"},
		{name: "level", mutate: func(q *pwconfig.QueryLogConfig) { q.Level = "verbose" }, want: "observability.query.level"},
		{name: "slow level", mutate: func(q *pwconfig.QueryLogConfig) { q.SlowLevel = "off" }, want: "observability.query.slow_level"},
		{name: "threshold", mutate: func(q *pwconfig.QueryLogConfig) { q.SlowThreshold = -time.Second }, want: "slow_threshold"},
		{name: "sql bound", mutate: func(q *pwconfig.QueryLogConfig) { q.MaxSQLLength = -1 }, want: "max_sql_length"},
		{name: "value bound", mutate: func(q *pwconfig.QueryLogConfig) { q.MaxValueLength = -1 }, want: "max_value_length"},
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
	if err := validateQueryLogConfig(queryConfig(func(q *pwconfig.QueryLogConfig) { q.SlowThreshold = 0 }).Query); err != nil {
		t.Fatal(err)
	}
}
