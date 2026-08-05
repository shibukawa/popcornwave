package pw

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/shibukawa/tinybind-go/configbind"
)

func TestResolveLoadOptionsSelectsProjectLocalFilesByEnvironment(t *testing.T) {
	options, env, _, err := resolveLoadOptions(configbind.LoadOptions{
		Vendor: "popcornwave-test", Tool: "pw-test", Environ: []string{"APP_ENV=stg"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if env != EnvStaging {
		t.Fatalf("env = %q, want %q", env, EnvStaging)
	}
	want := []string{"config.stg.toml", filepath.Join("config", "config.stg.toml")}
	if !reflect.DeepEqual(options.ExtraConfigReadPaths, want) {
		t.Fatalf("ExtraConfigReadPaths = %v, want %v", options.ExtraConfigReadPaths, want)
	}
	if options.FileName != "config.toml" {
		t.Fatalf("FileName = %q, want the environment-neutral name", options.FileName)
	}
}

func TestResolveLoadOptionsDefaultsToDevelopment(t *testing.T) {
	options, env, _, err := resolveLoadOptions(configbind.LoadOptions{
		Vendor: "popcornwave-test", Tool: "pw-test", Environ: []string{"PATH=/usr/bin"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if env != EnvDevelopment {
		t.Fatalf("env = %q, want %q", env, EnvDevelopment)
	}
	if options.ExtraConfigReadPaths[0] != "config.dev.toml" {
		t.Fatalf("ExtraConfigReadPaths = %v", options.ExtraConfigReadPaths)
	}
}

func TestResolveLoadOptionsNeverReadsProjectLocalNeutralFile(t *testing.T) {
	options, _, _, err := resolveLoadOptions(configbind.LoadOptions{
		Vendor: "popcornwave-test", Tool: "pw-test", Environ: []string{"APP_ENV=prod"},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range options.ExtraConfigReadPaths {
		if filepath.Base(path) == "config.toml" {
			t.Fatalf("project-local search includes the neutral file: %v", options.ExtraConfigReadPaths)
		}
	}
}

func TestResolveLoadOptionsRejectsAnInvalidEnvironment(t *testing.T) {
	if _, _, _, err := resolveLoadOptions(configbind.LoadOptions{
		Vendor: "popcornwave-test", Tool: "pw-test", Environ: []string{"APP_ENV=../etc"},
	}); err == nil {
		t.Fatal("an environment containing a path separator must fail before load")
	}
}

func TestResolveLoadOptionsKeepsExplicitReadPaths(t *testing.T) {
	explicit := []string{"custom.toml"}
	options, _, _, err := resolveLoadOptions(configbind.LoadOptions{
		Vendor: "popcornwave-test", Tool: "pw-test", Environ: []string{"APP_ENV=stg"},
		ExtraConfigReadPaths: explicit,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(options.ExtraConfigReadPaths, explicit) {
		t.Fatalf("ExtraConfigReadPaths = %v, want %v", options.ExtraConfigReadPaths, explicit)
	}
}

func TestEnvFallsBackToTheDefaultEnvironment(t *testing.T) {
	if got := Env(); got == "" {
		t.Fatal("Env returned an empty environment")
	}
}
