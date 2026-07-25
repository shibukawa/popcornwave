package pwtestbridge

import (
	"database/sql"
	"fmt"
	"net/http"
	"reflect"
	"sync"
)

type Configs map[reflect.Type]any

type Prepared struct {
	Handler http.Handler
	DB      *sql.DB
	Close   func() error
}

type Hooks struct {
	Snapshot func() (Configs, error)
	Prepare  func(http.Handler, Configs) (Prepared, error)
}

var state = struct {
	sync.RWMutex
	hooks Hooks
}{}

func Register(hooks Hooks) {
	state.Lock()
	defer state.Unlock()
	if state.hooks.Snapshot != nil || state.hooks.Prepare != nil {
		panic("popcornwave: test bridge is already registered")
	}
	state.hooks = hooks
}

func Snapshot() (Configs, error) {
	state.RLock()
	hooks := state.hooks
	state.RUnlock()
	if hooks.Snapshot == nil {
		return nil, fmt.Errorf("popcornwave: test runtime is unavailable")
	}
	return hooks.Snapshot()
}

func Prepare(handler http.Handler, configs Configs) (Prepared, error) {
	state.RLock()
	hooks := state.hooks
	state.RUnlock()
	if hooks.Prepare == nil {
		return Prepared{}, fmt.Errorf("popcornwave: test runtime is unavailable")
	}
	return hooks.Prepare(handler, configs)
}
