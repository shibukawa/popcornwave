package pw

import (
	"os"
	"reflect"
	"strings"
	"sync"

	"github.com/shibukawa/tinybind-go/configbind"
)

var commandState = struct {
	sync.RWMutex
	values map[reflect.Type]any
}{values: make(map[reflect.Type]any)}

// RegisterSubCommand registers typed CLI-only input.
func RegisterSubCommand[T any](name, help string) {
	selected := configbind.SubCommand[T](name, help)
	if selected == nil {
		return
	}
	commandState.Lock()
	commandState.values[reflect.TypeFor[T]()] = selected
	commandState.Unlock()
}

// SubCommand is retained as a compatibility alias.
// Deprecated: use RegisterSubCommand.
func SubCommand[T any](name, help string) {
	RegisterSubCommand[T](name, help)
}

// Command returns the selected and parsed command after ParseConfig.
func Command[T any]() (T, bool) {
	var zero T
	commandState.RLock()
	value, ok := commandState.values[reflect.TypeFor[T]()]
	commandState.RUnlock()
	if !ok {
		return zero, false
	}
	ptr, ok := value.(*T)
	if !ok || ptr == nil {
		return zero, false
	}
	return *ptr, true
}

func commandArgs(configured []string) []string {
	if configured != nil {
		return configured
	}
	args := os.Args[1:]
	if !strings.HasSuffix(os.Args[0], ".test") {
		return args
	}
	filtered := make([]string, 0, len(args))
	for _, arg := range args {
		if !strings.HasPrefix(arg, "-test.") {
			filtered = append(filtered, arg)
		}
	}
	return filtered
}
