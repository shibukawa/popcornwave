package pwobservability

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/shibukawa/popcornwave/internal/pwenv"
	"github.com/shibukawa/popcornwave/pwconfig"
	"github.com/shibukawa/popcornwave/pwruntime"
)

func TestDevelopmentLogSinkCreatesCanonicalJSONLLazily(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".log", "run.jsonl")
	t.Setenv(pwenv.DevLogFileVar, path)
	var diagnostic bytes.Buffer
	sink, closer := developmentSink(pwconfig.ObservabilityConfig{ServiceName: "catalog"}, pwruntime.LevelDebug, pwconfig.EnvDevelopment, &diagnostic)
	if sink == nil || closer == nil {
		t.Fatal("development path did not install a file sink")
	}
	defer closer.Close()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("log file exists before the first record: %v", err)
	}

	sink.Emit(context.Background(), pwruntime.Record{
		Time:  time.Date(2026, 8, 10, 12, 34, 56, 123000000, time.UTC),
		Level: pwruntime.LevelWarn, Message: "request completed",
		TraceID: "0102030405060708090a0b0c0d0e0f10",
		SpanID:  "0102030405060708", TraceFlags: 1,
		Attributes: []pwruntime.Attribute{
			pwruntime.Int("status", 503), pwruntime.Bool("cached", false),
			pwruntime.String(pwruntime.FieldServiceName, "forged"),
		},
	})

	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	directoryInfo, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	fileInfo, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if directoryInfo.Mode().Perm() != 0o700 || fileInfo.Mode().Perm() != 0o600 {
		t.Fatalf("permissions = directory %o, file %o", directoryInfo.Mode().Perm(), fileInfo.Mode().Perm())
	}
	if bytes.Count(source, []byte("\n")) != 1 {
		t.Fatalf("record is not one JSONL line: %q", source)
	}
	decoded := map[string]any{}
	if err := json.Unmarshal(bytes.TrimSpace(source), &decoded); err != nil {
		t.Fatalf("invalid JSONL record: %v\n%s", err, source)
	}
	for key, want := range map[string]any{
		pwruntime.FieldSeverity: "warn", pwruntime.FieldMessage: "request completed",
		pwruntime.FieldServiceName: "catalog", pwruntime.FieldTraceID: "0102030405060708090a0b0c0d0e0f10",
		pwruntime.FieldSpanID: "0102030405060708", pwruntime.FieldTraceFlags: float64(1),
		"status": float64(503), "cached": false,
	} {
		if decoded[key] != want {
			t.Errorf("%s = %#v, want %#v", key, decoded[key], want)
		}
	}
	if _, ok := decoded[pwruntime.FieldTimestamp]; !ok {
		t.Errorf("record has no %s: %#v", pwruntime.FieldTimestamp, decoded)
	}
	for _, old := range []string{"time", "level", "msg"} {
		if _, ok := decoded[old]; ok {
			t.Errorf("slog field %q leaked into canonical record: %#v", old, decoded)
		}
	}
	if diagnostic.Len() != 0 {
		t.Fatalf("unexpected diagnostic: %s", &diagnostic)
	}
}

func TestDevelopmentLogSinkDisablesAfterOneFilesystemFailure(t *testing.T) {
	root := t.TempDir()
	parent := filepath.Join(root, "not-a-directory")
	if err := os.WriteFile(parent, []byte("occupied"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(pwenv.DevLogFileVar, filepath.Join(parent, "run.jsonl"))
	var diagnostic bytes.Buffer
	sink, closer := developmentSink(pwconfig.ObservabilityConfig{}, pwruntime.LevelInfo, pwconfig.EnvDevelopment, &diagnostic)
	defer closer.Close()
	record := pwruntime.Record{Time: time.Now(), Level: pwruntime.LevelInfo, Message: "kept in terminal"}
	sink.Emit(context.Background(), record)
	sink.Emit(context.Background(), record)
	if got := strings.Count(diagnostic.String(), "local log capture disabled"); got != 1 {
		t.Fatalf("diagnostic count = %d, want one: %s", got, &diagnostic)
	}
}

func TestDevelopmentLogSinkRequiresPwDevHandoff(t *testing.T) {
	t.Setenv(pwenv.DevLogFileVar, "")
	if sink, closer := developmentSink(pwconfig.ObservabilityConfig{}, pwruntime.LevelInfo, pwconfig.EnvDevelopment, os.Stderr); sink != nil || closer != nil {
		t.Fatal("development without an injected path installed a file sink")
	}
	t.Setenv(pwenv.DevLogFileVar, filepath.Join(t.TempDir(), "run.jsonl"))
	if sink, closer := developmentSink(pwconfig.ObservabilityConfig{}, pwruntime.LevelInfo, pwconfig.EnvProduction, os.Stderr); sink != nil || closer != nil {
		t.Fatal("production accepted the private development path")
	}
}
