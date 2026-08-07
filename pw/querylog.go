package pw

import (
	"fmt"
	"strings"
	"time"

	"github.com/shibukawa/popcornwave/pwruntime"
)

// Diagnostic toggles. Auto ties the setting to something the process already
// knows, so a run that wants the ordinary answer configures nothing: query
// diagnostics read the runtime environment, and framework spans read whether
// anything exports them.
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
// development is whether the development relaxations apply, which is narrower
// than the environment being "dev": a deployment that never set APP_ENV is not
// asking for a log of every statement with its bind values.
func resolveQueryDiagnostics(config ObservabilityConfig, development bool) *pwruntime.QueryDiagnostics {
	enabled, err := resolveToggle(config.Query.Enabled, development)
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
	bindValues, err := resolveToggle(config.Query.BindValues, development)
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
// run that did not ask for development says out loud that records may carry row
// values.
//
// The warning keys off development rather than off the environment token so
// that the one case most in need of it — a deployment that never set APP_ENV and
// landed on "dev" by default — is the case that gets it. Suppressing it there
// would leave the only signal about row values in logs behind the very
// condition that produced them.
func reportQueryDiagnostics(diagnostics *pwruntime.QueryDiagnostics, env string, development bool, driver string) {
	if diagnostics == nil {
		return
	}
	if !development {
		processLogger().Warn("popcornwave query diagnostics enabled",
			String("environment", env),
			Bool("bind_values", diagnostics.BindValues),
			Duration("slow_threshold", diagnostics.SlowThreshold),
		)
	}
	if diagnostics.Explain && diagnostics.SlowThreshold > 0 && driver != "" && !pwruntime.SupportsExplain(driver) {
		processLogger().Warn("popcornwave slow query explain is unavailable",
			String("driver", driver),
			String("reason", "no known plan-only EXPLAIN form for this driver"),
		)
	}
}

// resolveToggle reads the auto/on/off vocabulary shared by every diagnostic
// switch. auto is what the caller passes as the resolved automatic answer,
// which differs per setting: the runtime environment for query diagnostics,
// and whether anything exports traces for framework spans.
func resolveToggle(value string, auto bool) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", QueryToggleAuto:
		return auto, nil
	case QueryToggleOn:
		return true, nil
	case QueryToggleOff:
		return false, nil
	default:
		return false, fmt.Errorf("must be %s, %s, or %s", QueryToggleAuto, QueryToggleOn, QueryToggleOff)
	}
}

func parseQueryLevel(value string) (Level, error) {
	if strings.TrimSpace(value) == "" {
		return LevelDebug, nil
	}
	level, err := parseLevel(value, LevelDebug)
	if err != nil || level == LevelOff {
		return 0, fmt.Errorf("must be trace, debug, info, warn, or error")
	}
	return level, nil
}
