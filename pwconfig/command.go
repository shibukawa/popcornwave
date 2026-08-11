package pwconfig

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/shibukawa/tinybind-go/configbind"
)

type frameworkActionKind uint8

const (
	frameworkActionNone frameworkActionKind = iota
	frameworkActionGenerateConfig
	frameworkActionPrintDSN
	frameworkActionHealthcheck
)

type frameworkAction struct {
	kind        frameworkActionKind
	value       string
	healthcheck healthcheckOptions
}

var frameworkActionState = struct {
	sync.RWMutex
	action frameworkAction
}{}

// ParseFrameworkAction filters the framework's own arguments off the command
// line and records which one was asked for. It is the Args hook a runtime
// installs, so the same words mean the same thing whichever transport is
// linked.
func ParseFrameworkAction(args []string) ([]string, error) {
	filtered := make([]string, 0, len(args))
	var selected frameworkAction
	// The healthcheck token is recognized only in the leading position, which is
	// exactly how HEALTHCHECK CMD ["/app", "healthcheck"] invokes it. Anywhere
	// else the word stays an ordinary argument value, so an application flag can
	// still take "healthcheck" as its value.
	if len(args) > 0 && args[0] == healthcheckCommandName {
		action, err := parseHealthcheckArgs(args[1:])
		if err != nil {
			return nil, err
		}
		frameworkActionState.Lock()
		frameworkActionState.action = action
		frameworkActionState.Unlock()
		return filtered, nil
	}
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
			if !printDSNBuilt {
				// Refusing by name rather than letting it fall through to the
				// application's own arguments: a silent pass-through would make
				// a toolchain that asked the wrong build look like a toolchain
				// with a broken database configuration.
				return nil, fmt.Errorf("popcornwave: --pw-print-dsn needs a build with -tags=pwdev; it prints the database password, so a release build does not carry it")
			}
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

// refusePendingFrameworkAction returns an error naming a framework action the
// caller cannot answer. A framework action invoked against a Middlewares
// application would otherwise fall through into a normal server start — for
// the healthcheck probe that means a competing bind attempt on every
// HEALTHCHECK interval, misreported as the server's own health.
// RefusePendingFrameworkAction returns an error naming an action the caller
// cannot answer.
func RefusePendingFrameworkAction() error {
	name := selectedFrameworkActionName()
	if name == "" {
		return nil
	}
	return fmt.Errorf("popcornwave: %s is answered inside pw.Run; an application that owns its server with pw.Middlewares does not carry it", name)
}

// selectedFrameworkActionName reports the selected action by its CLI spelling,
// or "" when none is pending. Middlewares uses it to refuse an action it
// cannot answer.
func selectedFrameworkActionName() string {
	switch selectedFrameworkAction().kind {
	case frameworkActionGenerateConfig:
		return "--generate-config"
	case frameworkActionPrintDSN:
		return "--pw-print-dsn"
	case frameworkActionHealthcheck:
		return healthcheckCommandName
	default:
		return ""
	}
}

// RunFrameworkAction answers the selected action and reports whether it took
// the invocation. A runtime calls it before it starts serving.
func RunFrameworkAction() (bool, error) {
	action := selectedFrameworkAction()
	switch action.kind {
	case frameworkActionNone:
		return false, nil
	case frameworkActionGenerateConfig:
		if action.value == "toml" {
			return true, configbind.WriteScaffoldTOML(os.Stdout)
		}
		return true, configbind.WriteScaffoldEnv(os.Stdout)
	case frameworkActionPrintDSN:
		// The DSN travels over the pipe to the parent process only; it is never
		// placed in a process argument or a log line.
		dsn, err := Value[MiddlewareConfig]().RDB.MigrationDSN()
		if err != nil {
			return true, err
		}
		fmt.Fprintln(os.Stdout, dsn)
		return true, nil
	case frameworkActionHealthcheck:
		return true, runHealthcheckProbe(action.healthcheck)
	default:
		return true, fmt.Errorf("popcornwave: unsupported framework action")
	}
}
