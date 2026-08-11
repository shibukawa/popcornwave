package pwratelimit

import (
	"context"
	"testing"
	"time"
)

// enabledConfig is the shipped default with the limiter switched on, which is
// what every case here varies one field of.
func enabledConfig() Config {
	config := DefaultConfig()
	config.Enabled = true
	config.PerSubject = 3
	config.PerAddress = 2
	return config
}

func TestConfigValidation(t *testing.T) {
	cases := map[string]func(*Config){
		"per_address zero has no off position": func(c *Config) { c.PerAddress = 0 },
		"per_address negative":                 func(c *Config) { c.PerAddress = -1 },
		"per_address above the ceiling":        func(c *Config) { c.Process = 1; c.PerAddress = 2 },
		"per_subject above the ceiling":        func(c *Config) { c.Process = 2; c.PerSubject = 3; c.PerAddress = 1 },
		"unknown backend":                      func(c *Config) { c.Backend = "postgres" },
		"window not positive":                  func(c *Config) { c.Window = 0 },
		"redis dsn without redis backend":      func(c *Config) { c.Redis.DSN = "redis://localhost:6379" },
		"redis backend without dsn":            func(c *Config) { c.Backend = BackendRedis },
	}
	for name, mutate := range cases {
		config := enabledConfig()
		mutate(&config)
		if err := config.Validate(); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	valid := enabledConfig()
	valid.Process = 100
	if err := valid.Validate(); err != nil {
		t.Errorf("a coherent configuration was rejected: %v", err)
	}
	// Nothing is validated while the limiter is off, so a project can carry
	// values it has not switched on yet.
	off := DefaultConfig()
	off.PerAddress = 0
	if err := off.Validate(); err != nil {
		t.Errorf("a disabled limiter was validated: %v", err)
	}
}

func TestMemoryCounterOpensANewWindow(t *testing.T) {
	counter := NewMemoryCounter()
	now := time.Date(2026, 8, 10, 12, 0, 30, 0, time.UTC)
	counter.now = func() time.Time { return now }
	for i := 1; i <= 3; i++ {
		count, err := counter.Increment(context.Background(), "address:203.0.113.9", time.Minute)
		if err != nil {
			t.Fatal(err)
		}
		if count != uint64(i) {
			t.Fatalf("count = %d, want %d", count, i)
		}
	}
	// Past the window boundary the count restarts rather than accumulating.
	now = now.Add(time.Minute)
	count, err := counter.Increment(context.Background(), "address:203.0.113.9", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("count after the window reset = %d, want 1", count)
	}
}
