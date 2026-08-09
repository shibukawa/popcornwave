package pw

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"strconv"
	"time"
)

// Option configures framework lifecycle construction.
type Option func(*lifecycleOptions) error

type lifecycleOptions struct {
	publicFS fs.FS
}

// WithPublicFS supplies the embedded public tree, rooted at its public directory.
func WithPublicFS(publicFS fs.FS) Option {
	return func(options *lifecycleOptions) error {
		if publicFS == nil {
			return errors.New("popcornwave: nil public filesystem")
		}
		options.publicFS = publicFS
		return nil
	}
}

// Middlewares performs framework initialization and returns the same wrapped
// handler stack used by Run. The startup summary is emitted here because the
// application owns the listener and the framework never learns its address.
func Middlewares(handler http.Handler, option ...Option) (http.Handler, error) {
	if err := ParseConfig(); err != nil {
		return nil, err
	}
	if err := refusePendingFrameworkAction(); err != nil {
		return nil, err
	}
	wrapped, err := buildMiddlewares(handler, option...)
	if err != nil {
		return nil, err
	}
	emitBootReport("")
	return wrapped, nil
}

func buildMiddlewares(handler http.Handler, option ...Option) (http.Handler, error) {
	if handler == nil {
		return nil, errors.New("popcornwave: nil handler")
	}
	if err := ParseConfig(); err != nil {
		return nil, err
	}
	options := lifecycleOptions{}
	for _, apply := range option {
		if apply == nil {
			continue
		}
		if err := apply(&options); err != nil {
			return nil, err
		}
	}
	// Updates key their validators with a secret, and an unkeyed digest of
	// low-entropy content lets a guess be confirmed by comparing digests. That
	// is refused before the port is bound rather than degraded at request time.
	if err := validateUpdateConfig(Config[HTMLConfig](nil)); err != nil {
		return nil, err
	}
	// A redraw endpoint is published by a generated init, which has nowhere to
	// return a collision to. This is where one is answered.
	if err := validateReloadableRegistration(); err != nil {
		return nil, err
	}
	server := Config[ServerConfig](nil)
	security := Config[SecurityConfig](nil)
	middleware := Config[MiddlewareConfig](nil)
	if err := validateConfiguredRuntime(); err != nil {
		return nil, err
	}
	if err := initializeRuntimeDatabase(); err != nil {
		return nil, err
	}
	if err := validateOperationalEndpointCollisions(handler, server); err != nil {
		return nil, err
	}
	observability := Config[ObservabilityConfig](nil)
	telemetry, err := buildObservability(observability, Env())
	if err != nil {
		return nil, err
	}
	// A root span is created when export exists, and also when configuration
	// asked for framework spans outright: the children below are only a trace if
	// something roots them.
	rootSpan := telemetry.tracing || traceForced(observability)
	resources := runtimeResources(telemetry.backend, telemetry.tracing)
	reportEnvironment()
	reportCompressionCodings(middleware)
	reportDatabaseConnections(resources.Connections)
	reportQueryDiagnostics(resources.Query, Env(), Development(), resources.DBDriver)
	// The data pane needs the pool, so it starts once the database is open and
	// before the first request. It is a no-op outside the pwdev build mode.
	startDevelopmentData(resources)
	wrapped, err := buildRuntimeHandler(handler, server, security, middleware, resources, rootSpan, options.publicFS)
	if err != nil {
		return nil, err
	}
	// The seed and assert endpoints wrap the finished chain rather than joining
	// it, so they exist only on the served application — the test bridge builds
	// its chain without them — and only in the pwdev build mode.
	return developmentTestEndpoints(wrapped, middleware, resources), nil
}

// Run owns parsing, framework initialization, serving, graceful shutdown, and
// resource cleanup.
func Run(ctx context.Context, handler http.Handler, option ...Option) error {
	if ctx == nil {
		return errors.New("popcornwave: nil context")
	}
	if err := ParseConfig(); err != nil {
		return err
	}
	if handled, err := runFrameworkAction(); handled {
		return err
	}
	wrapped, err := buildMiddlewares(handler, option...)
	if err != nil {
		return err
	}
	serverConfig := Config[ServerConfig](nil)

	signalContext, cancelSignals := notifyShutdownSignals(ctx)
	defer cancelSignals()
	server := newHTTPServer(serverConfig, wrapped)
	// Binding here, instead of inside ListenAndServe, keeps the startup summary
	// honest: it is written once the port is actually accepted, and it reports
	// the resolved port even when the configuration asked for port 0 or a
	// development run moved off a port it could not bind.
	listener, err := listenApplication(serverConfig)
	if err != nil {
		return closeRuntimeResources(serverConfig.ShutdownTimeout, err)
	}
	address := listenURL(listener)
	emitBootReport(address)
	// The development console links the application, and the address it worked
	// out from the project files is not always the one this process bound.
	announceDevelopmentListener(address)
	serve := func() error { return server.Serve(listener) }
	serveErr := serveUntilContext(signalContext, server, serve, serverConfig.ShutdownTimeout)
	return closeRuntimeResources(serverConfig.ShutdownTimeout, serveErr)
}

// listenURL renders the address an operator can open. A wildcard or loopback
// bind is reported as localhost, because that is the host that resolves.
func listenURL(listener net.Listener) string {
	address := listener.Addr()
	tcp, ok := address.(*net.TCPAddr)
	if !ok {
		return "http://" + address.String()
	}
	host := tcp.IP.String()
	switch {
	case tcp.IP == nil || tcp.IP.IsUnspecified() || tcp.IP.IsLoopback():
		host = "localhost"
	case tcp.IP.To4() == nil:
		host = "[" + host + "]"
	}
	return "http://" + host + ":" + strconv.Itoa(tcp.Port)
}

func serveUntilContext(ctx context.Context, server *http.Server, serve func() error, shutdownTimeout time.Duration) error {
	result := make(chan error, 1)
	go func() {
		err := serve()
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		result <- err
	}()

	select {
	case err := <-result:
		return err
	case <-ctx.Done():
		shutdownContext, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		shutdownErr := server.Shutdown(shutdownContext)
		cancel()
		serveErr := <-result
		return errors.Join(shutdownErr, serveErr)
	}
}

func newHTTPServer(config ServerConfig, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              ":" + strconv.Itoa(config.Port),
		Handler:           handler,
		ReadHeaderTimeout: config.ReadHeaderTimeout,
		ReadTimeout:       config.ReadTimeout,
		WriteTimeout:      config.WriteTimeout,
		IdleTimeout:       config.IdleTimeout,
	}
}

func closeRuntimeResources(timeout time.Duration, err error) error {
	configState.RLock()
	cleanups := append([]*runtimeCleanup(nil), configState.cleanups...)
	configState.RUnlock()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return errors.Join(err, runRuntimeCleanups(ctx, cleanups))
}

func runRuntimeCleanups(ctx context.Context, cleanups []*runtimeCleanup) error {
	var result error
	for index := len(cleanups) - 1; index >= 0; index-- {
		cleanupErr := cleanups[index].run(ctx)
		if cleanupErr != nil {
			cleanupErr = fmt.Errorf("close %s: %w", cleanups[index].name, cleanupErr)
		}
		result = errors.Join(result, cleanupErr)
	}
	return result
}
