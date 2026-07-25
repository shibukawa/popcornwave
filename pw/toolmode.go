package pw

import (
	"fmt"
	"os"
	"strings"
	"sync"
)

type frameworkActionKind uint8

const (
	frameworkActionNone frameworkActionKind = iota
	frameworkActionGenerateConfig
	frameworkActionPrintDSN
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
		case arg == "--pw-print-dsn":
			action = frameworkAction{kind: frameworkActionPrintDSN}
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
	case frameworkActionPrintDSN:
		// The DSN travels over the pipe to the parent process only; it is never
		// placed in a process argument or a log line.
		dsn, err := configuredDatabaseDSN()
		if err != nil {
			return true, err
		}
		fmt.Fprintln(os.Stdout, dsn)
		return true, nil
	default:
		return true, fmt.Errorf("popcornwave: unsupported framework action")
	}
}
