package pwconfig

import (
	"reflect"
	"testing"
)

func TestParseFrameworkActionRemovesConfigGenerationOption(t *testing.T) {
	previous := selectedFrameworkAction()
	t.Cleanup(func() {
		frameworkActionState.Lock()
		frameworkActionState.action = previous
		frameworkActionState.Unlock()
	})

	args, err := ParseFrameworkAction([]string{"--port", "9090", "--generate-config=toml"})
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
	if _, err := ParseFrameworkAction([]string{"--generate-config", "json"}); err == nil {
		t.Fatal("invalid config format was accepted")
	}
}
