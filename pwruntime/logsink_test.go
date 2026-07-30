package pwruntime

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"
	"time"
)

func slogRecord(t *testing.T, record Record) map[string]any {
	t.Helper()
	var buffer bytes.Buffer
	NewSlogSink(slog.NewJSONHandler(&buffer, &slog.HandlerOptions{Level: slog.LevelDebug})).Emit(context.Background(), record)
	decoded := map[string]any{}
	if err := json.Unmarshal(buffer.Bytes(), &decoded); err != nil {
		t.Fatalf("record is not valid JSON: %v\n%s", err, buffer.String())
	}
	return decoded
}

// Scalars stay typed rather than being rendered to strings, so a JSON consumer
// still sees a number as a number.
func TestSlogSinkKeepsScalarsTyped(t *testing.T) {
	decoded := slogRecord(t, Record{
		Time:    time.Now(),
		Level:   LevelInfo,
		Message: "request completed",
		Attributes: []Attribute{
			String("path", "/items"), Int("status", 201),
			Bool("cached", false), Float64("duration", 1.5),
		},
	})
	if decoded["msg"] != "request completed" {
		t.Errorf("msg = %v", decoded["msg"])
	}
	if decoded["status"] != float64(201) {
		t.Errorf("status = %#v, want a number", decoded["status"])
	}
	if decoded["cached"] != false || decoded["duration"] != 1.5 {
		t.Errorf("cached = %#v, duration = %#v", decoded["cached"], decoded["duration"])
	}
}

func TestSlogSinkWritesTraceCorrelation(t *testing.T) {
	decoded := slogRecord(t, Record{
		Time: time.Now(), Level: LevelInfo, Message: "correlated",
		TraceID: "0102030405060708090a0b0c0d0e0f10", SpanID: "0102030405060708", TraceFlags: 1,
	})
	if decoded[FieldTraceID] != "0102030405060708090a0b0c0d0e0f10" || decoded[FieldSpanID] != "0102030405060708" {
		t.Fatalf("correlation = %#v", decoded)
	}
}

// A record outside a trace carries no correlation keys at all, rather than
// empty ones a consumer would have to filter.
func TestSlogSinkOmitsAbsentCorrelation(t *testing.T) {
	decoded := slogRecord(t, Record{Time: time.Now(), Level: LevelInfo, Message: "standalone"})
	if _, present := decoded[FieldTraceID]; present {
		t.Fatalf("uncorrelated record carried a trace ID: %#v", decoded)
	}
}

// A user attribute cannot take a reserved name, because a consumer reading the
// stream has to be able to trust those fields.
func TestSlogSinkDropsReservedAttributeNames(t *testing.T) {
	decoded := slogRecord(t, Record{
		Time: time.Now(), Level: LevelWarn, Message: "real message",
		TraceID: "0102030405060708090a0b0c0d0e0f10", SpanID: "0102030405060708",
		Attributes: []Attribute{
			String(FieldMessage, "forged"),
			String(FieldTraceID, "forged"),
			String(FieldSeverity, "forged"),
			String("kept", "yes"),
		},
	})
	if decoded["msg"] != "real message" {
		t.Errorf("msg = %v, want the record message", decoded["msg"])
	}
	if decoded[FieldTraceID] != "0102030405060708090a0b0c0d0e0f10" {
		t.Errorf("%s = %v, want the captured correlation", FieldTraceID, decoded[FieldTraceID])
	}
	if _, present := decoded[FieldSeverity]; present {
		t.Errorf("a reserved name was accepted from a user attribute: %#v", decoded)
	}
	if decoded["kept"] != "yes" {
		t.Errorf("an ordinary attribute was dropped: %#v", decoded)
	}
}

// The handler filters too, so a handler configured above the backend floor
// cannot be bypassed by the backend letting a record through.
func TestSlogSinkRespectsItsHandlerLevel(t *testing.T) {
	var buffer bytes.Buffer
	sink := NewSlogSink(slog.NewJSONHandler(&buffer, &slog.HandlerOptions{Level: slog.LevelError}))
	sink.Emit(context.Background(), Record{Time: time.Now(), Level: LevelInfo, Message: "dropped"})
	if buffer.Len() != 0 {
		t.Fatalf("handler level was ignored: %s", buffer.String())
	}
}

// A nil handler or logger yields a nil sink, which the backend drops, so
// "no destination configured" never becomes a nil dereference at a log site.
func TestNilSinksAreDropped(t *testing.T) {
	if sink := NewSlogSink(nil); sink != nil {
		t.Error("a nil handler produced a sink")
	}
	if sink := NewOtelSink(nil); sink != nil {
		t.Error("a nil logger produced a sink")
	}
	backend := NewLogBackend(LevelInfo, NewSlogSink(nil), NewOtelSink(nil))
	if backend.enabled(LevelError) {
		t.Error("a backend of nil sinks reported a level as enabled")
	}
}

func TestOtelSeverityMapping(t *testing.T) {
	for level, want := range map[Level]uint8{
		LevelTrace: 1, LevelDebug: 5, LevelInfo: 9, LevelWarn: 13, LevelError: 17,
	} {
		if got := uint8(otelSeverity(level)); got != want {
			t.Errorf("otelSeverity(%s) = %d, want %d", level, got, want)
		}
	}
}
