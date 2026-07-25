package pw

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/shibukawa/popcornwave/pwruntime"
)

var requestSequence atomic.Uint64

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
	observability := Config[ObservabilityConfig](nil)
	if err := validateRuntimeConfig(server, security, middleware, observability); err != nil {
		return nil, err
	}
	if err := validateOperationalEndpointCollisions(handler, server); err != nil {
		return nil, err
	}
	if server.Public.Enabled && !publicDevelopment && options.publicFS == nil {
		return nil, errors.New("popcornwave: server.public.enabled requires pw.WithPublicFS")
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
	wrapped, err := Middlewares(handler, option...)
	if err != nil {
		return err
	}
	serverConfig := Config[ServerConfig](nil)

	signalContext, cancelSignals := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
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

func injectResources(resources pwruntime.Resources) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := pwruntime.WithResources(r.Context(), resources)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if !validRequestID(id) {
			id = fmt.Sprintf("%x-%x", time.Now().UnixNano(), requestSequence.Add(1))
		}
		w.Header().Set("X-Request-ID", id)
		resources := runtimeResources(slog.Default().With("request_id", id))
		ctx := pwruntime.WithResources(r.Context(), resources)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func validRequestID(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for _, r := range value {
		if r < 0x21 || r > 0x7e {
			return false
		}
	}
	return true
}

func recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				err := fmt.Errorf("panic: %v", recovered)
				if responseCommitted(w) {
					Logger(r.Context()).ErrorContext(r.Context(), "panic after response commit", "error", err)
					return
				}
				WriteProblem(w, r, InternalServerError(err))
			}
		}()
		next.ServeHTTP(w, r)
	})
}
