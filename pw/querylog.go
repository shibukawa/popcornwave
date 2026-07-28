package pw

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/shibukawa/popcornwave/pwruntime"
)

// Query diagnostics toggles. Auto ties the setting to the runtime environment,
// so a development run is instrumented without configuration and every other
// environment stays silent until someone opts in.
const (
	QueryToggleAuto = "auto"
	QueryToggleOn   = "on"
	QueryToggleOff  = "off"
)

// Query diagnostics defaults.
//
// The record level is info rather than debug because the framework does not yet
// build a handler from observability.minimum_level: a debug record would be
// dropped by the default slog handler, and a development aid that is on by
// default has to be visible by default.
const (
	defaultQuerySlowThreshold  = 200 * time.Millisecond
	defaultQueryMaxSQLLength   = 4096
	defaultQueryMaxValueLength = 256
)

// resolveQueryDiagnostics turns configuration into the runtime setting, or nil
// when query diagnostics are off. Invalid values resolve to nil; validation
// reports them before requests are served.
func resolveQueryDiagnostics(config ObservabilityConfig, env string) *pwruntime.QueryDiagnostics {
	development := env == EnvDevelopment
	enabled, err := resolveQueryToggle(config.Query.Enabled, development)
	if err != nil || !enabled {
		return nil
	}
	level, err := parseQueryLevel(config.Query.Level)
	if err != nil {
		return nil
	}
	slowLevel, err := parseQueryLevel(config.Query.SlowLevel)
	if err != nil {
		return nil
	}
	bindValues, err := resolveQueryToggle(config.Query.BindValues, development)
	if err != nil {
		return nil
	}
	return &pwruntime.QueryDiagnostics{
		Level:          level,
		SlowLevel:      slowLevel,
		SlowThreshold:  config.Query.SlowThreshold,
		BindValues:     bindValues,
		Explain:        config.Query.Explain,
		Reproduction:   config.Query.Reproduction,
		MaxSQLLength:   positiveOr(config.Query.MaxSQLLength, defaultQueryMaxSQLLength),
		MaxValueLength: positiveOr(config.Query.MaxValueLength, defaultQueryMaxValueLength),
	}
}

// positiveOr treats an unset bound as its default, so a configuration that
// names only the keys it cares about still gets bounded records.
func positiveOr(value, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}

// reportQueryDiagnostics states at startup what the query log will do, so a
// missing capability is reported once instead of silently per statement, and a
// non-development run says out loud that records may carry row values.
func reportQueryDiagnostics(diagnostics *pwruntime.QueryDiagnostics, env, driver string) {
	if diagnostics == nil {
		return
	}
	if env != EnvDevelopment {
		slog.Warn("popcornwave query diagnostics enabled",
			"environment", env,
			"bind_values", diagnostics.BindValues,
			"slow_threshold", diagnostics.SlowThreshold,
		)
	}
	if diagnostics.Explain && diagnostics.SlowThreshold > 0 && driver != "" && !pwruntime.SupportsExplain(driver) {
		slog.Warn("popcornwave slow query explain is unavailable",
			"driver", driver,
			"reason", "no known plan-only EXPLAIN form for this driver",
		)
	}
}

func resolveQueryToggle(value string, development bool) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", QueryToggleAuto:
		return development, nil
	case QueryToggleOn:
		return true, nil
	case QueryToggleOff:
		return false, nil
	default:
		return false, fmt.Errorf("must be %s, %s, or %s", QueryToggleAuto, QueryToggleOn, QueryToggleOff)
	}
}

func parseQueryLevel(value string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "trace":
		return pwruntime.LevelTrace, nil
	case "", "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("must be trace, debug, info, warn, or error")
	}
}
