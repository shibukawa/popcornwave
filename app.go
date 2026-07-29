package petitweb

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/shibukawa/tinygodriver/httpmux"
)

// Middleware is the standard Go HTTP middleware signature.
type Middleware func(http.Handler) http.Handler

// ReadyCheck reports whether a critical application dependency can serve.
type ReadyCheck func(context.Context) error

// Option configures an App before it starts serving.
type Option func(*App) error

// App owns an application's mux, middleware, operational endpoints, and server
// lifecycle. Configuration is frozen the first time Handler or Serve is called.
type App struct {
	mu           sync.Mutex
	mux          *httpmux.ServeMux
	middlewares  []Middleware
	config       ServerConfig
	renderer     ErrorRenderer
	readyChecks  []ReadyCheck
	openAPI      []byte
	closers      []func(context.Context) error
	optionErr    error
	handler      http.Handler
	server       *http.Server
	serving      atomic.Bool
	shutting     atomic.Bool
	shutdownOnce sync.Once
	shutdownErr  error
}

// New constructs an application with safe server defaults.
func New(options ...Option) *App {
	a := &App{mux: httpmux.NewServeMux(), config: DefaultServerConfig()}
	for _, option := range options {
		if option == nil {
			continue
		}
		if err := option(a); err != nil {
			a.optionErr = errors.Join(a.optionErr, err)
		}
	}
	return a
}

// WithMiddleware appends middleware. Middleware executes in the order supplied.
func WithMiddleware(middleware ...Middleware) Option {
	return func(a *App) error {
		for _, m := range middleware {
			if m == nil {
				return errors.New("petitweb: nil middleware")
			}
			a.middlewares = append(a.middlewares, m)
		}
		return nil
	}
}

// WithServerConfig replaces the default server configuration.
func WithServerConfig(config ServerConfig) Option {
	return func(a *App) error { a.config = config; return nil }
}

// WithErrorRenderer installs the HTML error renderer.
func WithErrorRenderer(renderer ErrorRenderer) Option {
	return func(a *App) error { a.renderer = renderer; return nil }
}

// WithReadinessCheck adds a critical dependency readiness check.
func WithReadinessCheck(check ReadyCheck) Option {
	return func(a *App) error {
		if check == nil {
			return errors.New("petitweb: nil readiness check")
		}
		a.readyChecks = append(a.readyChecks, check)
		return nil
	}
}

// WithOpenAPI supplies a generated OpenAPI document for the configured endpoint.
func WithOpenAPI(document []byte) Option {
	return func(a *App) error {
		a.openAPI = append([]byte(nil), document...)
		return nil
	}
}

// WithCloser registers a process-lifetime resource closer. Closers run in
// reverse registration order after active requests have drained.
func WithCloser(closer func(context.Context) error) Option {
	return func(a *App) error {
		if closer == nil {
			return errors.New("petitweb: nil closer")
		}
		a.closers = append(a.closers, closer)
		return nil
	}
}

// Use appends middleware before the application is frozen.
func (a *App) Use(middleware ...Middleware) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.assertMutable()
	for _, m := range middleware {
		if m == nil {
			panic("petitweb: nil middleware")
		}
		a.middlewares = append(a.middlewares, m)
	}
}

// Handle registers a standard net/http handler.
func (a *App) Handle(pattern string, handler http.Handler) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.assertMutable()
	a.mux.Handle(pattern, handler)
}

// HandleFunc registers a standard net/http handler function.
func (a *App) HandleFunc(pattern string, handler http.HandlerFunc) {
	a.Handle(pattern, handler)
}

// Mux returns the application's standard library mux. It must be configured
// before Handler or Serve freezes the application.
func (a *App) Mux() *httpmux.ServeMux { return a.mux }

// Middlewares returns an immutable snapshot of configured middleware.
func (a *App) Middlewares() []Middleware {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]Middleware(nil), a.middlewares...)
}

// SetErrorRenderer installs an HTML error renderer before serving.
func (a *App) SetErrorRenderer(renderer ErrorRenderer) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.assertMutable()
	a.renderer = renderer
}

// WriteError negotiates an error using the renderer configured on the App.
func (a *App) WriteError(w http.ResponseWriter, r *http.Request, err error) {
	a.mu.Lock()
	renderer := a.renderer
	a.mu.Unlock()
	var logger = ReadLogger(nil)
	if r != nil {
		logger = ReadLogger(r.Context())
	}
	ErrorHandler{Renderer: renderer, Logger: logger}.WriteError(w, r, err)
}

// Validate checks all startup invariants without freezing the App.
func (a *App) Validate() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.validateLocked()
}

func (a *App) validateLocked() error {
	if a.shutting.Load() {
		return errors.New("petitweb: application is shutting down")
	}
	if a.optionErr != nil {
		return a.optionErr
	}
	if err := a.config.Validate(); err != nil {
		return err
	}
	if a.config.OpenAPI != "" && len(a.openAPI) == 0 {
		return errors.New("petitweb: openapi endpoint enabled without generated document")
	}
	return nil
}

// Handler freezes configuration and returns the fully composed handler. Invalid
// startup configuration panics; callers that need error handling should call
// Validate first or use Serve/ListenAndServe.
func (a *App) Handler() http.Handler {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.handler != nil {
		return a.handler
	}
	if err := a.validateLocked(); err != nil {
		panic(err)
	}
	a.registerOperationalEndpointsLocked()
	var handler http.Handler = a.mux
	for i := len(a.middlewares) - 1; i >= 0; i-- {
		handler = a.middlewares[i](handler)
	}
	a.handler = handler
	return handler
}

func (a *App) assertMutable() {
	if a.handler != nil || a.server != nil {
		panic("petitweb: application is already frozen")
	}
}

// ListenAndServe validates and serves until the server is shut down.
func (a *App) ListenAndServe(addr string) error {
	if addr == "" {
		addr = a.config.Address
	}
	return a.Serve(&http.Server{Addr: addr})
}

// Serve runs the application using server's listener settings and Petitweb's
// validated timeouts. Supplying a server allows tests and advanced deployments
// to use an existing listener via server.Serve separately after Handler().
func (a *App) Serve(server *http.Server) error {
	if server == nil {
		return errors.New("petitweb: nil server")
	}
	a.mu.Lock()
	if err := a.validateLocked(); err != nil {
		a.mu.Unlock()
		return err
	}
	if a.server != nil {
		a.mu.Unlock()
		return errors.New("petitweb: application already served")
	}
	a.mu.Unlock()
	handler := a.Handler()
	a.mu.Lock()
	server.Handler = handler
	applyServerConfig(server, a.config)
	a.server = server
	a.serving.Store(true)
	a.mu.Unlock()
	err := server.ListenAndServe()
	a.serving.Store(false)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// Run serves until ctx is cancelled, then performs graceful shutdown.
func (a *App) Run(ctx context.Context, addr string) error {
	if ctx == nil {
		return errors.New("petitweb: nil context")
	}
	errCh := make(chan error, 1)
	go func() { errCh <- a.ListenAndServe(addr) }()
	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), a.config.ShutdownTimeout)
		defer cancel()
		shutdownErr := a.Shutdown(shutdownCtx)
		serveErr := <-errCh
		return errors.Join(shutdownErr, serveErr)
	}
}

// Shutdown drains active handlers and closes resources in reverse order.
func (a *App) Shutdown(ctx context.Context) error {
	if ctx == nil {
		return errors.New("petitweb: nil context")
	}
	a.shutdownOnce.Do(func() {
		a.shutting.Store(true)
		a.mu.Lock()
		server := a.server
		closers := append([]func(context.Context) error(nil), a.closers...)
		a.mu.Unlock()
		if server != nil {
			a.shutdownErr = errors.Join(a.shutdownErr, server.Shutdown(ctx))
		}
		for i := len(closers) - 1; i >= 0; i-- {
			a.shutdownErr = errors.Join(a.shutdownErr, closers[i](ctx))
		}
	})
	return a.shutdownErr
}

func applyServerConfig(server *http.Server, config ServerConfig) {
	if server.Addr == "" {
		server.Addr = config.Address
	}
	server.ReadHeaderTimeout = config.ReadHeaderTimeout
	server.ReadTimeout = config.ReadTimeout
	server.WriteTimeout = config.WriteTimeout
	server.IdleTimeout = config.IdleTimeout
	if server.BaseContext == nil {
		server.BaseContext = func(net.Listener) context.Context { return context.Background() }
	}
}

func (a *App) registerOperationalEndpointsLocked() {
	registerStatus := func(path string, ready bool) {
		a.mux.HandleFunc("GET "+path, func(w http.ResponseWriter, r *http.Request) {
			if ready {
				for _, check := range a.readyChecks {
					if err := check(r.Context()); err != nil {
						w.WriteHeader(http.StatusServiceUnavailable)
						return
					}
				}
			}
			if a.shutting.Load() {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			w.WriteHeader(http.StatusOK)
		})
	}
	if a.config.Health != "" {
		registerStatus(a.config.Health, false)
	}
	if a.config.Readiness != "" {
		registerStatus(a.config.Readiness, true)
	}
	if a.config.OpenAPI != "" {
		document := append([]byte(nil), a.openAPI...)
		a.mux.HandleFunc("GET "+a.config.OpenAPI, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			if r.Method != http.MethodHead {
				_, _ = w.Write(document)
			}
		})
	}
}

func (a *App) String() string { return fmt.Sprintf("petitweb.App(%s)", a.config.Address) }
