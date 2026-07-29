package pw

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/shibukawa/popcornwave/pwruntime"
)

func sampleBootReport() bootReport {
	return bootReport{
		startedAt:   time.Date(2026, 7, 27, 23, 31, 4, 0, time.UTC),
		environment: "dev",
		configPath:  "config.dev.toml",
		configFound: true,
		entries: []bootEntry{
			{key: "html.streaming", value: "true", source: "file_toml"},
			{key: "middleware.rdb.dsn", value: redactedValue, source: "default"},
			{key: "middleware.rdb.enabled", value: "false", source: "default"},
			{key: "server.port", value: "8080", source: "cli"},
			{key: "session.cookie.name", value: "pw_session", source: "default"},
		},
	}
}

func TestRenderBootTreeGroupsKeysBySection(t *testing.T) {
	rendered := renderBootTree(sampleBootReport(), "http://localhost:8080", bootStyle{})
	for _, want := range []string{
		"Popcorn Wave",
		"started at 2026-07-27 23:31:04 UTC",
		"env dev · config.dev.toml",
		"├─ middleware",
		"│  └─ rdb",
		"│     ├─ dsn",
		"└─ session",
		"listening on http://localhost:8080",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("rendered summary missing %q:\n%s", want, rendered)
		}
	}
	if strings.Contains(rendered, "middleware.rdb.enabled") {
		t.Fatalf("dotted key was not grouped:\n%s", rendered)
	}
	if count := strings.Count(rendered, "server.port"); count != 0 {
		t.Fatalf("server.port rendered flat %d times:\n%s", count, rendered)
	}
}

func TestBootBannerRowsShareOneWidth(t *testing.T) {
	// Captions are appended after each art row, so a ragged row would leave the
	// text beside it out of line.
	for _, art := range bootBannerArt {
		if got, want := utf8.RuneCountInString(art), utf8.RuneCountInString(bootBannerArt[0]); got != want {
			t.Fatalf("banner row %q is %d wide, want %d", art, got, want)
		}
	}
	banner := bootBanner(sampleBootReport(), bootStyle{})
	if len(banner) != len(bootBannerArt)+1 {
		t.Fatalf("banner has %d lines, want %d", len(banner), len(bootBannerArt)+1)
	}
	if trailing := banner[len(banner)-1]; trailing != "" {
		t.Fatalf("banner does not end with a blank line: %q", trailing)
	}
}

func TestRenderBootTreeMarksNonDefaultSourcesOnly(t *testing.T) {
	rendered := renderBootTree(sampleBootReport(), "", bootStyle{})
	for _, line := range strings.Split(rendered, "\n") {
		switch {
		case strings.Contains(line, "streaming"):
			if !strings.HasSuffix(line, "true  ← file") {
				t.Fatalf("file source line = %q", line)
			}
		case strings.Contains(line, "port"):
			if !strings.HasSuffix(line, "8080  ← flag") {
				t.Fatalf("flag source line = %q", line)
			}
		case strings.Contains(line, "enabled"):
			if strings.Contains(line, "←") {
				t.Fatalf("default source was marked: %q", line)
			}
		}
	}
	if strings.Contains(rendered, "listening on") {
		t.Fatal("summary announced a listener the framework does not own")
	}
}

func TestRenderBootTreeAlignsSourceMarksWithinAGroup(t *testing.T) {
	report := sampleBootReport()
	report.entries = []bootEntry{
		{key: "app.endpoint", value: "https://example.test", source: "default"},
		{key: "app.mode", value: "on", source: "cli"},
	}
	rendered := renderBootTree(report, "", bootStyle{})
	var marked, longest string
	for _, line := range strings.Split(rendered, "\n") {
		switch {
		case strings.Contains(line, "mode"):
			marked = line
		case strings.Contains(line, "endpoint"):
			longest = line
		}
	}
	// The mark starts past the widest value of the group, so a short value does
	// not drag its mark leftward out of the column.
	markColumn := utf8.RuneCountInString(marked[:strings.Index(marked, "←")])
	if got, want := markColumn, utf8.RuneCountInString(longest)+2; got != want {
		t.Fatalf("mark column = %d, want %d\n%s", got, want, rendered)
	}
}

func TestRenderBootTreeRedactsSecrets(t *testing.T) {
	rendered := renderBootTree(sampleBootReport(), "", bootStyle{})
	if !strings.Contains(rendered, redactedValue) {
		t.Fatalf("secret value not redacted:\n%s", rendered)
	}
}

func TestRenderBootTreeReportsMissingConfigFile(t *testing.T) {
	report := sampleBootReport()
	report.configFound, report.configPath = false, ""
	if !strings.Contains(renderBootTree(report, "", bootStyle{}), "no config file") {
		t.Fatal("missing config file was not reported")
	}
}

func TestBootRecordIsOneStructuredEvent(t *testing.T) {
	var buffer bytes.Buffer
	backend := pwruntime.NewLogBackend(LevelInfo, pwruntime.NewSlogSink(slog.NewJSONHandler(&buffer, nil)))
	pwruntime.NewLogger(context.Background(), backend).Info(bootMessage, bootRecordAttrs(sampleBootReport(), "http://localhost:8080")...)

	if lines := strings.Count(strings.TrimSpace(buffer.String()), "\n"); lines != 0 {
		t.Fatalf("startup emitted %d records, want 1:\n%s", lines+1, buffer.String())
	}
	// Nesting is dotted keys rather than JSON objects, because the same record
	// has to survive OTLP export, where a record attribute is a scalar.
	var record map[string]any
	if err := json.Unmarshal(buffer.Bytes(), &record); err != nil {
		t.Fatalf("record is not valid JSON: %v\n%s", err, buffer.String())
	}
	for key, want := range map[string]string{
		"msg":                          bootMessage,
		"environment":                  "dev",
		"listening":                    "http://localhost:8080",
		"config.server.port":           "8080",
		"config.middleware.rdb.dsn":    redactedValue,
		"config_source.server.port":    "flag",
		"config_source.html.streaming": "file",
	} {
		if got, _ := record[key].(string); got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
	if _, marked := record["config_source.middleware.rdb.enabled"]; marked {
		t.Fatalf("default source was reported as an override: %s", buffer.String())
	}
}
func TestResolveBootLogFormat(t *testing.T) {
	tests := map[string]string{
		BootLogTree:   BootLogTree,
		"  TREE  ":    BootLogTree,
		BootLogRecord: BootLogRecord,
		BootLogOff:    BootLogOff,
		// Tests never run against a terminal, so auto resolves to the record.
		BootLogAuto: BootLogRecord,
		"":          BootLogRecord,
	}
	for setting, want := range tests {
		if got := resolveBootLogFormat(setting); got != want {
			t.Fatalf("resolveBootLogFormat(%q) = %q, want %q", setting, got, want)
		}
	}
}

func TestBootStyleWrapsOnlyWhenColored(t *testing.T) {
	if got := (bootStyle{}).bold("x"); got != "x" {
		t.Fatalf("plain style = %q", got)
	}
	if got := (bootStyle{color: true}).bold("x"); got != "\x1b[1mx\x1b[0m" {
		t.Fatalf("colored style = %q", got)
	}
}

func TestListenURLPrefersLocalhost(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	port := listener.Addr().(*net.TCPAddr).Port
	if got, want := listenURL(listener), "http://localhost:"+strconv.Itoa(port); got != want {
		t.Fatalf("listenURL = %q, want %q", got, want)
	}
}
