package redis

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/shibukawa/popcornwave/pw"
)

func TestImportingThePackageRegistersTheBackend(t *testing.T) {
	// This package's init is the whole opt-in: an application blank-imports it
	// and session.backend = "redis" resolves.
	registered := false
	for _, name := range pw.SessionBackends() {
		registered = registered || name == pw.SessionBackendRedis
	}
	if !registered {
		t.Fatalf("registered backends = %v", pw.SessionBackends())
	}
}

func TestBackendRefusesAnUnusableServer(t *testing.T) {
	open := func(config pw.SessionRedisConfig) error {
		_, err := pw.OpenSessionBackend(context.Background(),
			pw.SessionConfig{Backend: pw.SessionBackendRedis, Redis: config}, pw.SessionResources{})
		return err
	}

	if err := open(pw.SessionRedisConfig{}); err == nil || !strings.Contains(err.Error(), "session.redis.dsn") {
		t.Fatalf("missing dsn error = %v", err)
	}
	// A malformed DSN is reported by shape only: the URL can carry a password.
	err := open(pw.SessionRedisConfig{DSN: "postgres://app:hunter2@localhost/app"})
	if err == nil || strings.Contains(err.Error(), "hunter2") {
		t.Fatalf("malformed dsn error = %v", err)
	}
	// A server that does not answer stops startup instead of failing the first
	// login.
	err = open(pw.SessionRedisConfig{DSN: "redis://127.0.0.1:1/0", ConnectTimeout: 300 * time.Millisecond})
	if err == nil || !strings.Contains(err.Error(), "session.redis") {
		t.Fatalf("unreachable server error = %v", err)
	}
}

func TestBackendOpensAndClosesItsOwnClient(t *testing.T) {
	address := os.Getenv("PETITWEB_REDIS_ADDR")
	if address == "" {
		t.Skip("PETITWEB_REDIS_ADDR is not set")
	}
	backend, err := pw.OpenSessionBackend(t.Context(), pw.SessionConfig{
		Backend: pw.SessionBackendRedis,
		Redis:   pw.SessionRedisConfig{DSN: "redis://" + address + "/0", KeyPrefix: "pw-backend-test:"},
	}, pw.SessionResources{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if backend.Store == nil {
		t.Fatal("no store")
	}
	// This backend dialed its own client, so it owns the close; the server
	// expires records, so it brings no sweep.
	if backend.Close == nil {
		t.Fatal("backend did not hand back the client it opened")
	}
	if backend.Prune != nil {
		t.Fatal("backend asked to be swept")
	}
	if err := backend.Close(t.Context()); err != nil {
		t.Fatalf("close: %v", err)
	}
}
