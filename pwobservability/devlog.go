package pwobservability

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/shibukawa/popcornwave/internal/pwenv"
	"github.com/shibukawa/popcornwave/pwconfig"
	"github.com/shibukawa/popcornwave/pwruntime"
)

// developmentSink returns the pw-dev-only JSONL destination. The path is
// injected by the parent CLI rather than read from application configuration,
// so a deployed process never starts writing a local file by accident.
func developmentSink(config pwconfig.ObservabilityConfig, minimum pwruntime.Level, env string, diagnostic io.Writer) (pwruntime.Sink, io.Closer) {
	path := strings.TrimSpace(os.Getenv(pwenv.DevLogFileVar))
	if env != pwconfig.EnvDevelopment || path == "" {
		return nil, nil
	}
	writer := &developmentLogWriter{path: path, diagnostic: diagnostic}
	var handler slog.Handler = slog.NewJSONHandler(writer, &slog.HandlerOptions{
		Level:       slog.Level(minimum),
		ReplaceAttr: developmentLogAttribute,
	})
	handler = handler.WithAttrs([]slog.Attr{
		slog.String(pwruntime.FieldServiceName, serviceNameOf(resourceAttributes(config))),
	})
	return pwruntime.NewSlogSink(handler), writer
}

// developmentLogAttribute gives the local file a stable, format-independent
// schema instead of exposing slog's time/level/msg encoder names.
func developmentLogAttribute(_ []string, attribute slog.Attr) slog.Attr {
	switch attribute.Key {
	case slog.TimeKey:
		attribute.Key = pwruntime.FieldTimestamp
	case slog.LevelKey:
		return slog.String(pwruntime.FieldSeverity, strings.ToLower(attribute.Value.String()))
	case slog.MessageKey:
		attribute.Key = pwruntime.FieldMessage
	}
	return attribute
}

// developmentLogWriter opens lazily, appends one handler-produced JSON line at
// a time, and turns a filesystem failure into one diagnostic. Returning success
// after disabling the writer keeps slog, and therefore an application request,
// from inheriting a local tooling failure.
type developmentLogWriter struct {
	mu         sync.Mutex
	path       string
	diagnostic io.Writer
	file       *os.File
	disabled   bool
}

func (writer *developmentLogWriter) Write(record []byte) (int, error) {
	writer.mu.Lock()
	defer writer.mu.Unlock()
	if writer.disabled {
		return len(record), nil
	}
	if writer.file == nil {
		if err := os.MkdirAll(filepath.Dir(writer.path), 0o700); err != nil {
			writer.disable(err)
			return len(record), nil
		}
		file, err := os.OpenFile(writer.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
		if err != nil {
			writer.disable(err)
			return len(record), nil
		}
		writer.file = file
	}
	if _, err := writer.file.Write(record); err != nil {
		writer.disable(err)
	}
	return len(record), nil
}

func (writer *developmentLogWriter) disable(err error) {
	writer.disabled = true
	if writer.file != nil {
		_ = writer.file.Close()
		writer.file = nil
	}
	if writer.diagnostic != nil {
		fmt.Fprintf(writer.diagnostic, "popcornwave: local log capture disabled for %s: %v\n", writer.path, err)
	}
}

func (writer *developmentLogWriter) Close() error {
	writer.mu.Lock()
	defer writer.mu.Unlock()
	if writer.file == nil {
		return nil
	}
	err := writer.file.Close()
	writer.file = nil
	return err
}

var _ io.WriteCloser = (*developmentLogWriter)(nil)
