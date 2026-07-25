package pw

import (
	"os"
	"reflect"
	"sync"

	"github.com/shibukawa/tinybind-go/configbind"
)

var commandState = struct {
	sync.RWMutex
	values map[reflect.Type]any
}{values: make(map[reflect.Type]any)}

// SubCommand registers typed CLI-only input.
func SubCommand[T any](name, help string) {
	selected := configbind.SubCommand[T](name, help)
	if selected == nil {
		return
	}
	commandState.Lock()
	commandState.values[reflect.TypeFor[T]()] = selected
	commandState.Unlock()
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
	return os.Args[1:]
}
