package pw

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
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
// handler stack used by Run.
func Middlewares(handler http.Handler, option ...Option) (http.Handler, error) {
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
	resources := runtimeResources(slog.Default())
	return buildRuntimeHandler(handler, server, security, middleware, resources, options.publicFS)
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
	wrapped, err := Middlewares(handler, option...)
	if err != nil {
		return err
	}
	serverConfig := Config[ServerConfig](nil)

	signalContext, cancelSignals := notifyShutdownSignals(ctx)
	defer cancelSignals()
	server := newHTTPServer(serverConfig, wrapped)
	serveErr := serveUntilContext(signalContext, server, server.ListenAndServe, serverConfig.ShutdownTimeout)
	return closeRuntimeResources(serverConfig.ShutdownTimeout, serveErr)
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
