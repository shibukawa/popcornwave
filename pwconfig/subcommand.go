package pwconfig

import (
	"reflect"
	"sync"

	"github.com/shibukawa/tinybind-go/configbind"
)

var commandState = struct {
	sync.RWMutex
	values map[reflect.Type]any
}{values: make(map[reflect.Type]any)}

// RegisterSubCommand registers typed CLI-only input.
//
// It is here rather than on either runtime because a subcommand is a fact about
// the command line, which is this package's to read. Both builds of one
// application therefore answer the same words — an application that added a
// "migrate" command has it in both, and does not discover on the second that
// its binary understands nothing.
func RegisterSubCommand[T any](name, help string) {
	if name == healthcheckCommandName {
		// Refused at registration rather than shadowed at dispatch: the
		// framework consumes this token before application commands are parsed,
		// so a registration under it could never run and a HEALTHCHECK already
		// written into a Dockerfile must keep meaning the probe.
		panic("popcornwave: subcommand name \"" + healthcheckCommandName + "\" is reserved for the framework health probe")
	}
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
