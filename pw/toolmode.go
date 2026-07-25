package pw

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type frameworkActionKind uint8

const (
	frameworkActionNone frameworkActionKind = iota
	frameworkActionGenerateConfig
	frameworkActionSchemaInit
	frameworkActionSeed
)

type frameworkAction struct {
	kind  frameworkActionKind
	value string
}

var frameworkActionState = struct {
	sync.RWMutex
	action frameworkAction
}{}

func parseFrameworkAction(args []string) ([]string, error) {
	filtered := make([]string, 0, len(args))
	var selected frameworkAction
	for index := 0; index < len(args); index++ {
		arg := args[index]
		var action frameworkAction
		switch {
		case arg == "--generate-config":
			if index+1 >= len(args) {
				return nil, fmt.Errorf("popcornwave: --generate-config requires toml or env")
			}
			index++
			action = frameworkAction{kind: frameworkActionGenerateConfig, value: args[index]}
		case strings.HasPrefix(arg, "--generate-config="):
			action = frameworkAction{kind: frameworkActionGenerateConfig, value: strings.TrimPrefix(arg, "--generate-config=")}
		case arg == "--pw-schema-init":
			if index+1 >= len(args) {
				return nil, fmt.Errorf("popcornwave: --pw-schema-init requires a schema directory")
			}
			index++
			action = frameworkAction{kind: frameworkActionSchemaInit, value: args[index]}
		case strings.HasPrefix(arg, "--pw-schema-init="):
			action = frameworkAction{kind: frameworkActionSchemaInit, value: strings.TrimPrefix(arg, "--pw-schema-init=")}
		case arg == "--pw-seed":
			if index+1 >= len(args) {
				return nil, fmt.Errorf("popcornwave: --pw-seed requires at least one dataset path")
			}
			index++
			action = frameworkAction{kind: frameworkActionSeed, value: args[index]}
		case strings.HasPrefix(arg, "--pw-seed="):
			action = frameworkAction{kind: frameworkActionSeed, value: strings.TrimPrefix(arg, "--pw-seed=")}
		default:
			filtered = append(filtered, arg)
			continue
		}
		if selected.kind != frameworkActionNone {
			return nil, fmt.Errorf("popcornwave: multiple framework actions were requested")
		}
		selected = action
	}
	if selected.kind == frameworkActionGenerateConfig {
		switch selected.value {
		case "toml", "env":
		default:
			return nil, fmt.Errorf("popcornwave: unknown config format %q (want toml or env)", selected.value)
		}
	}
	frameworkActionState.Lock()
	frameworkActionState.action = selected
	frameworkActionState.Unlock()
	return filtered, nil
}

func selectedFrameworkAction() frameworkAction {
	frameworkActionState.RLock()
	defer frameworkActionState.RUnlock()
	return frameworkActionState.action
}

func runFrameworkAction() (bool, error) {
	action := selectedFrameworkAction()
	switch action.kind {
	case frameworkActionNone:
		return false, nil
	case frameworkActionGenerateConfig:
		if action.value == "toml" {
			return true, WriteScaffoldTOML(os.Stdout)
		}
		return true, WriteScaffoldEnv(os.Stdout)
	case frameworkActionSchemaInit:
		if err := validateConfiguredRuntime(); err != nil {
			return true, err
		}
		if err := initializeRuntimeDatabase(); err != nil {
			return true, err
		}
		err := initializeSchema(action.value)
		server := Config[ServerConfig](nil)
		return true, closeRuntimeResources(server.ShutdownTimeout, err)
	case frameworkActionSeed:
		if err := validateConfiguredRuntime(); err != nil {
			return true, err
		}
		if err := initializeRuntimeDatabase(); err != nil {
			return true, err
		}
		paths := filepath.SplitList(action.value)
		if len(paths) == 0 {
			return true, fmt.Errorf("popcornwave: --pw-seed requires at least one dataset path")
		}
		err := seedDatabase(paths)
		server := Config[ServerConfig](nil)
		return true, closeRuntimeResources(server.ShutdownTimeout, err)
	default:
		return true, fmt.Errorf("popcornwave: unsupported framework action")
	}
}
