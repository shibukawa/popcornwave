package pwruntime

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shibukawa/popcornwave/contrib/otel/trace"
)

func captureBackend(minimum Level) (*LogBackend, *CaptureSink) {
	sink := NewCaptureSink()
	return NewLogBackend(minimum, sink), sink
}

func TestLoggerFiltersBelowTheMinimum(t *testing.T) {
	backend, sink := captureBackend(LevelWarn)
	logger := NewLogger(context.Background(), backend)

	logger.Trace("trace")
	logger.Debug("debug")
	logger.Info("info")
	logger.Warn("warn")
	logger.Error("error")

	records := sink.Records()
	if len(records) != 2 {
		t.Fatalf("want 2 records, got %d: %#v", len(records), records)
	}
	if records[0].Level != LevelWarn || records[1].Level != LevelError {
		t.Errorf("levels = %v, %v", records[0].Level, records[1].Level)
	}
	// Enabled has to agree with what actually happens, because callers use it
	// to skip building attributes.
	if logger.Enabled(LevelInfo) || !logger.Enabled(LevelWarn) {
		t.Error("Enabled disagrees with the emitted records")
	}
}

// A backend with no destination reports nothing as enabled, so an application
// that guards on Enabled skips the work rather than formatting into the void.
func TestLoggerWithNoSinkIsDisabled(t *testing.T) {
	logger := NewLogger(context.Background(), NewLogBackend(LevelTrace))
	if logger.Enabled(LevelError) {
		t.Error("a backend with no sink reported a level as enabled")
	}
	logger.Error("dropped")
}

// The zero Logger is what an accessor returns when nothing is installed. It
// must be callable, because the alternative is a nil check at every log site.
func TestZeroLoggerIsSafe(t *testing.T) {
	var logger Logger
	if logger.Enabled(LevelError) {
		t.Error("the zero logger reported a level as enabled")
	}
	logger.With(String("k", "v")).Error("dropped")
}

func TestLoggerWithDoesNotMutateItsSource(t *testing.T) {
	backend, sink := captureBackend(LevelInfo)
	base := NewLogger(context.Background(), backend, String("service", "app"))
	derived := base.With(String("request_id", "abc"))

	base.Info("base")
	derived.Info("derived")

	records := sink.Records()
	if len(records) != 2 {
		t.Fatalf("want 2 records, got %d", len(records))
	}
	if _, present := records[0].Lookup("request_id"); present {
		t.Error("With mutated the logger it was called on")
	}
	if records[1].Text("service") != "app" || records[1].Text("request_id") != "abc" {
		t.Errorf("derived attributes = %#v", records[1].Attributes)
	}
}

// A later value replaces an earlier one under the same key, so a call site can
// override what middleware bound without producing two entries a consumer has
// to disambiguate.
func TestLoggerCallAttributeOverridesABoundOne(t *testing.T) {
	backend, sink := captureBackend(LevelInfo)
	NewLogger(context.Background(), backend, String("route", "bound")).
		Info("done", String("route", "call"))

	record := sink.Records()[0]
	if record.Text("route") != "call" {
		t.Errorf("route = %q, want the call value", record.Text("route"))
	}
	if count := len(record.Attributes); count != 1 {
		t.Errorf("attributes = %d, want the duplicate resolved to 1", count)
	}
}

func TestLoggerCapturesTraceCorrelationAtAcquisition(t *testing.T) {
	backend, sink := captureBackend(LevelInfo)
	spanContext, err := trace.NewSpanContext("0102030405060708090a0b0c0d0e0f10", "0102030405060708", 1, "", false)
	if err != nil {
		t.Fatal(err)
	}
	ctx := trace.ContextWithSpanContext(context.Background(), spanContext)

	NewLogger(ctx, backend).Info("correlated")

	record := sink.Records()[0]
	if record.TraceID != "0102030405060708090a0b0c0d0e0f10" || record.SpanID != "0102030405060708" {
		t.Fatalf("correlation = %q %q", record.TraceID, record.SpanID)
	}
	if record.TraceFlags != 1 {
		t.Errorf("trace flags = %d, want the sampled bit", record.TraceFlags)
	}
}

func TestLoggerOutsideATraceCarriesNoCorrelation(t *testing.T) {
	backend, sink := captureBackend(LevelInfo)
	NewLogger(context.Background(), backend).Info("standalone")

	if record := sink.Records()[0]; record.TraceID != "" || record.SpanID != "" {
		t.Fatalf("correlation = %q %q, want none", record.TraceID, record.SpanID)
	}
}

// Every sink sees the record. This is what lets a development run keep its
// terminal stream while also feeding a collector.
func TestBackendFansOutToEverySink(t *testing.T) {
	first, second := NewCaptureSink(), NewCaptureSink()
	NewLogger(context.Background(), NewLogBackend(LevelInfo, first, nil, second)).Info("both")

	if len(first.Records()) != 1 || len(second.Records()) != 1 {
		t.Fatalf("records = %d and %d, want 1 each", len(first.Records()), len(second.Records()))
	}
}

func TestErrAttributeAcceptsNil(t *testing.T) {
	backend, sink := captureBackend(LevelInfo)
	NewLogger(context.Background(), backend).Error("failed", Err(nil), Int("attempt", 2))

	record := sink.Records()[0]
	if record.Text("error") != "" {
		t.Errorf("error = %q, want empty for a nil error", record.Text("error"))
	}
	attribute, _ := record.Lookup("attempt")
	if value, ok := attribute.Value.AsInt64(); !ok || value != 2 {
		t.Errorf("attempt = %v %v, want a typed 2", value, ok)
	}
}

func TestErrAttributeCarriesTheMessage(t *testing.T) {
	backend, sink := captureBackend(LevelInfo)
	NewLogger(context.Background(), backend).Error("failed", Err(errors.New("boom")))

	if got := sink.Records()[0].Text("error"); got != "boom" {
		t.Errorf("error = %q", got)
	}
}

// Durations are milliseconds, which is what the OpenTelemetry log model and a
// human reading a terminal both expect.
func TestDurationAttributeIsMilliseconds(t *testing.T) {
	attribute := Duration("elapsed", 1500*time.Microsecond)
	value, ok := attribute.Value.AsFloat64()
	if !ok || value != 1.5 {
		t.Fatalf("elapsed = %v %v, want 1.5", value, ok)
	}
}

func TestLevelStringMatchesConfigurationTokens(t *testing.T) {
	for level, want := range map[Level]string{
		LevelTrace: "trace", LevelDebug: "debug", LevelInfo: "info",
		LevelWarn: "warn", LevelError: "error", LevelOff: "off",
	} {
		if got := level.String(); got != want {
			t.Errorf("%d.String() = %q, want %q", level, got, want)
		}
	}
}
