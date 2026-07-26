package pwtestbridge

import (
	"database/sql"
	"fmt"
	"net/http"
	"reflect"
	"sync"

	"github.com/shibukawa/popcornwave/pwruntime"
)

type Configs map[reflect.Type]any

// Options selects optional test runtime behavior.
type Options struct {
	// Transaction wraps every request of the prepared handler in one shared
	// transaction scope that the caller begins and rolls back.
	Transaction bool
	// PrepareDatabase runs against the opened pool before the runtime handler
	// is built. Extensions verify their tables while that handler is
	// assembled, so a test that installs a schema has to do it here rather
	// than after Prepare returns.
	PrepareDatabase func(*sql.DB) error
}

type Prepared struct {
	Handler http.Handler
	DB      *sql.DB
	Driver  string
	// TxScope is non-nil when Options.Transaction was requested and the
	// configured runtime has a database.
	TxScope *pwruntime.TransactionScope
	// Resources is the same runtime state the prepared handler installs, so
	// tests can build a context equivalent to a request context.
	Resources pwruntime.Resources
	Close     func() error
}

type Hooks struct {
	Snapshot func() (Configs, error)
	Prepare  func(http.Handler, Configs, Options) (Prepared, error)
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

func Prepare(handler http.Handler, configs Configs, options Options) (Prepared, error) {
	state.RLock()
	hooks := state.hooks
	state.RUnlock()
	if hooks.Prepare == nil {
		return Prepared{}, fmt.Errorf("popcornwave: test runtime is unavailable")
	}
	return hooks.Prepare(handler, configs, options)
}
