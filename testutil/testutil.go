// Package testutil runs Popcorn Wave applications from isolated copies of the
// registered runtime configuration.
package testutil

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"github.com/shibukawa/popcornwave/internal/dbschema"
	"github.com/shibukawa/popcornwave/internal/dbseed"
	"github.com/shibukawa/popcornwave/internal/pwtestbridge"
	"github.com/shibukawa/popcornwave/pw"
)

// TestingT is the subset of testing.T used by TestRun.
//
// It stays an interface so this shipped package never imports testing.
// Fatalf reports setup failure that invalidates the test; Errorf reports an
// assertion failure that lets the test continue.
type TestingT interface {
	Helper()
	Cleanup(func())
	Fatalf(string, ...any)
	Errorf(string, ...any)
}

// Config is an isolated copy of all registered framework and application
// configuration values.
type Config struct {
	values pwtestbridge.Configs
}

// Get returns one typed value from a copied configuration.
func Get[T any](config *Config) T {
	if config != nil {
		if value, ok := config.values[reflect.TypeFor[T]()].(T); ok {
			return deepClone(value)
		}
	}
	var zero T
	return zero
}

// Set replaces one typed value in a copied configuration.
func Set[T any](config *Config, value T) {
	if config == nil {
		panic("testutil: nil Config")
	}
	config.values[reflect.TypeFor[T]()] = deepClone(value)
}

// Update edits one typed value in a copied configuration.
func Update[T any](config *Config, edit func(*T)) {
	if edit == nil {
		return
	}
	value := Get[T](config)
	edit(&value)
	Set(config, value)
}

type runSettings struct {
	schemaDir string
	seedDir   string
	seedFiles []string
}

// RunOption configures TestRun resources.
type RunOption func(*runSettings) error

// WithSchemaDir applies lexical-order .sql files from a dbschema directory
// after the copied database configuration has been opened.
func WithSchemaDir(directory string) RunOption {
	return func(settings *runSettings) error {
		if strings.TrimSpace(directory) == "" {
			return fmt.Errorf("testutil: empty schema directory")
		}
		settings.schemaDir = directory
		return nil
	}
}

// Server is a running application created by TestRun.
type Server struct {
	URL     string
	Port    int
	Config  *Config
	DB      *sql.DB
	server  *http.Server
	once    sync.Once
	close   func() error
	seedDir string
}

// Client returns an HTTP client configured for the test server.
func (server *Server) Client() *http.Client {
	return &http.Client{}
}

// Close stops the server and releases its copied runtime resources.
func (server *Server) Close() {
	server.once.Do(func() {
		_ = server.server.Close()
		if server.close != nil {
			_ = server.close()
		}
	})
}

// TestRun copies every registered configuration, defaults the copied server
// port to -1, applies customize, initializes copied runtime resources, and
// starts the application. Port -1 selects an available loopback port.
func TestRun(t TestingT, handler http.Handler, customize func(*Config), options ...RunOption) *Server {
	t.Helper()
	snapshot, err := pwtestbridge.Snapshot()
	if err != nil {
		t.Fatalf("copy Popcorn Wave configuration: %v", err)
		return nil
	}
	config := &Config{values: cloneConfigs(snapshot)}
	Update[pw.ServerConfig](config, func(server *pw.ServerConfig) {
		server.Port = -1
	})
	if customize != nil {
		customize(config)
	}
	settings := runSettings{}
	for _, apply := range options {
		if apply == nil {
			continue
		}
		if err := apply(&settings); err != nil {
			t.Fatalf("configure Popcorn Wave TestRun: %v", err)
			return nil
		}
	}

	serverConfig := Get[pw.ServerConfig](config)
	if serverConfig.Port < -1 || serverConfig.Port > 65535 {
		t.Fatalf("listen for Popcorn Wave TestRun: port must be -1 or between 0 and 65535")
		return nil
	}
	address := "127.0.0.1:0"
	if serverConfig.Port >= 0 {
		address = net.JoinHostPort("127.0.0.1", strconv.Itoa(serverConfig.Port))
	}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		t.Fatalf("listen for Popcorn Wave TestRun: %v", err)
		return nil
	}
	actualPort := listener.Addr().(*net.TCPAddr).Port
	serverConfig.Port = actualPort
	Set(config, serverConfig)

	prepared, err := pwtestbridge.Prepare(handler, config.values)
	if err != nil {
		_ = listener.Close()
		t.Fatalf("initialize Popcorn Wave TestRun: %v", err)
		return nil
	}
	if settings.schemaDir != "" {
		if prepared.DB == nil {
			_ = listener.Close()
			_ = prepared.Close()
			t.Fatalf("initialize Popcorn Wave TestRun schema: configured RDB is disabled")
			return nil
		}
		if err := dbschema.Apply(context.Background(), prepared.DB, settings.schemaDir); err != nil {
			_ = listener.Close()
			_ = prepared.Close()
			t.Fatalf("initialize Popcorn Wave TestRun schema: %v", err)
			return nil
		}
	}
	seedDir := settings.seedDir
	if seedDir == "" {
		seedDir = dbseed.DefaultDir
	}
	if len(settings.seedFiles) > 0 {
		if err := applySeed(config, prepared.DB, seedDir, settings.seedFiles); err != nil {
			_ = listener.Close()
			_ = prepared.Close()
			t.Fatalf("initialize Popcorn Wave TestRun seed: %v", err)
			return nil
		}
	}
	instance := &http.Server{
		Addr:              listener.Addr().String(),
		Handler:           prepared.Handler,
		ReadHeaderTimeout: serverConfig.ReadHeaderTimeout,
		ReadTimeout:       serverConfig.ReadTimeout,
		WriteTimeout:      serverConfig.WriteTimeout,
		IdleTimeout:       serverConfig.IdleTimeout,
	}
	result := &Server{
		URL:     "http://" + listener.Addr().String(),
		Port:    actualPort,
		Config:  config,
		DB:      prepared.DB,
		server:  instance,
		close:   prepared.Close,
		seedDir: seedDir,
	}
	t.Cleanup(result.Close)
	go func() {
		_ = instance.Serve(listener)
	}()
	return result
}

func cloneConfigs(source pwtestbridge.Configs) pwtestbridge.Configs {
	result := make(pwtestbridge.Configs, len(source))
	for typ, value := range source {
		result[typ] = cloneReflect(reflect.ValueOf(value)).Interface()
	}
	return result
}

func deepClone[T any](value T) T {
	cloned := cloneReflect(reflect.ValueOf(value))
	if !cloned.IsValid() {
		var zero T
		return zero
	}
	return cloned.Interface().(T)
}

func cloneReflect(value reflect.Value) reflect.Value {
	if !value.IsValid() {
		return value
	}
	switch value.Kind() {
	case reflect.Interface:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		cloned := cloneReflect(value.Elem())
		result := reflect.New(value.Type()).Elem()
		result.Set(cloned)
		return result
	case reflect.Pointer:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		result := reflect.New(value.Type().Elem())
		result.Elem().Set(cloneReflect(value.Elem()))
		return result
	case reflect.Slice:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		result := reflect.MakeSlice(value.Type(), value.Len(), value.Len())
		for index := 0; index < value.Len(); index++ {
			result.Index(index).Set(cloneReflect(value.Index(index)))
		}
		return result
	case reflect.Map:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		result := reflect.MakeMapWithSize(value.Type(), value.Len())
		iterator := value.MapRange()
		for iterator.Next() {
			result.SetMapIndex(cloneReflect(iterator.Key()), cloneReflect(iterator.Value()))
		}
		return result
	case reflect.Array:
		result := reflect.New(value.Type()).Elem()
		for index := 0; index < value.Len(); index++ {
			result.Index(index).Set(cloneReflect(value.Index(index)))
		}
		return result
	case reflect.Struct:
		result := reflect.New(value.Type()).Elem()
		result.Set(value)
		for index := 0; index < value.NumField(); index++ {
			if result.Field(index).CanSet() && value.Type().Field(index).IsExported() {
				result.Field(index).Set(cloneReflect(value.Field(index)))
			}
		}
		return result
	default:
		return value
	}
}
