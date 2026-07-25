package pw

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParseFrameworkActionRemovesConfigGenerationOption(t *testing.T) {
	previous := selectedFrameworkAction()
	t.Cleanup(func() {
		frameworkActionState.Lock()
		frameworkActionState.action = previous
		frameworkActionState.Unlock()
	})

	args, err := parseFrameworkAction([]string{"--port", "9090", "--generate-config=toml"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(args, []string{"--port", "9090"}) {
		t.Fatalf("args = %#v", args)
	}
	if action := selectedFrameworkAction(); action.kind != frameworkActionGenerateConfig || action.value != "toml" {
		t.Fatalf("action = %#v", action)
	}
}

func TestParseFrameworkActionRejectsInvalidConfigFormat(t *testing.T) {
	if _, err := parseFrameworkAction([]string{"--generate-config", "json"}); err == nil {
		t.Fatal("invalid config format was accepted")
	}
}

func TestParseFrameworkActionSelectsSeed(t *testing.T) {
	previous := selectedFrameworkAction()
	t.Cleanup(func() {
		frameworkActionState.Lock()
		frameworkActionState.action = previous
		frameworkActionState.Unlock()
	})

	joined := strings.Join([]string{"testdata/seed/a.yaml", "testdata/seed/b.yaml"}, string(filepath.ListSeparator))
	args, err := parseFrameworkAction([]string{"--port", "9090", "--pw-seed=" + joined})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(args, []string{"--port", "9090"}) {
		t.Fatalf("args = %#v", args)
	}
	action := selectedFrameworkAction()
	if action.kind != frameworkActionSeed {
		t.Fatalf("action = %#v", action)
	}
	if paths := filepath.SplitList(action.value); !reflect.DeepEqual(paths, []string{"testdata/seed/a.yaml", "testdata/seed/b.yaml"}) {
		t.Fatalf("paths = %#v", paths)
	}
}

func TestParseFrameworkActionRejectsSeedWithoutValue(t *testing.T) {
	if _, err := parseFrameworkAction([]string{"--pw-seed"}); err == nil {
		t.Fatal("--pw-seed without a dataset path was accepted")
	}
}

func TestParseFrameworkActionRejectsCombinedActions(t *testing.T) {
	if _, err := parseFrameworkAction([]string{"--pw-schema-init=dbschema", "--pw-seed=testdata/seed/a.yaml"}); err == nil {
		t.Fatal("schema-init combined with seed was accepted")
	}
}
