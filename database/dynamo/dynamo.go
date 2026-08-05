// Package dynamo opens the application's DynamoDB client from configuration and
// installs it into every request context.
//
// Importing it registers the [middleware.dynamo] binding and the middleware, so
// a project that does not use DynamoDB gains no configuration key and links no
// driver:
//
//	import _ "github.com/shibukawa/popcornwave/database/dynamo"
//
// It wraps no operation. There is no database/sql here to hide three engines
// behind, so a handler calls tinybind's dynamobind directly, and everything it
// needs is already in the context:
//
//	reading, err := dynamobind.Load[Reading](ctx, "reading", key)
//	for reading, err := range records.ReadingsSince(ctx, sensor, from) { ... }
//
// Table names in source are the declared ones. The deployed name comes from the
// configured prefix or mapping, applied inside the runtime entry, so no call
// site builds a deployed name.
package dynamo

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/shibukawa/popcornwave/pw"
	"github.com/shibukawa/tinybind-go/dynamobind"
	"github.com/shibukawa/tinygodriver/cloud/aws"
	"github.com/shibukawa/tinygodriver/nosql/dynamodb"
)

func init() {
	pw.RegisterExtension(pw.Extension{
		Name:  "database.dynamo",
		Slot:  pw.SlotStorage,
		Setup: setup,
		Close: closeRuntime,
	})
}

// state is the process client and the naming it was installed with. It is
// rebuilt whenever framework initialization runs.
var state struct {
	sync.RWMutex
	client   *dynamodb.Client
	resolver TableResolver
}

// activeResolver returns the naming the middleware installed, for the migrator
// running outside a request.
func activeResolver() TableResolver {
	state.RLock()
	defer state.RUnlock()
	return state.resolver
}

// EnsureClient returns a context carrying the DynamoDB client, reporting false
// when none can be reached.
//
// It exists for a framework extension that runs after SlotStorage during
// startup and wants to reach the store before serving. A setup context is not
// a request context, so the client the middleware installs per request is not
// in it yet; this finds the one the middleware opened instead. A context that
// already carries a client is returned unchanged.
//
// A handler never needs it. A request context already carries the client.
func EnsureClient(ctx context.Context) (context.Context, bool) {
	if _, err := dynamobind.ClientFromContext(ctx); err == nil {
		return ctx, true
	}
	state.RLock()
	client, resolver := state.client, state.resolver
	state.RUnlock()
	if client == nil {
		return ctx, false
	}
	return dynamobind.WithClient(ctx, client, dynamobind.WithTableNames(resolver)), true
}

// Client returns the process client, for an operation dynamobind does not wrap.
//
// A handler does not need it: every dynamobind entry reads the client from the
// context itself. Reach for this only when calling the driver directly.
func Client(ctx context.Context) (*dynamodb.Client, error) {
	return dynamobind.ClientFromContext(ctx)
}

// setup opens the client, verifies the schema, and returns the middleware that
// installs both into each request.
func setup(ctx context.Context) (pw.Middleware, error) {
	config := pw.Config[Config](ctx)
	if err := config.validate(pw.Development()); err != nil {
		return nil, err
	}
	if !config.Enabled {
		return nil, nil
	}

	resolver := resolverFor(config)
	if err := validateTableNames(ctx, config, resolver); err != nil {
		return nil, err
	}

	client, err := open(config)
	if err != nil {
		return nil, err
	}

	state.Lock()
	state.client = client
	state.resolver = resolver
	state.Unlock()

	// The migrator and the verifier both run against a context carrying the
	// client, so they take the same path a request does and cannot resolve a
	// name differently from one.
	installed := dynamobind.WithClient(ctx, client, dynamobind.WithTableNames(resolver))
	if config.AutoMigrate {
		if _, err := Migrate(installed); err != nil {
			return nil, err
		}
	}
	if config.VerifySchema {
		if err := verify(installed, client, resolver); err != nil {
			return nil, err
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			ctx := dynamobind.WithClient(request.Context(), client, dynamobind.WithTableNames(resolver))
			next.ServeHTTP(writer, request.WithContext(ctx))
		})
	}, nil
}

// open builds the client from configuration. Credentials fall back to the
// environment, which is what a deployment on an instance role wants.
func open(config Config) (*dynamodb.Client, error) {
	options := []dynamodb.Option{dynamodb.WithTimeout(config.Timeout)}
	if config.Region != "" {
		options = append(options, dynamodb.WithRegion(config.Region))
	}
	if config.Endpoint != "" {
		options = append(options, dynamodb.WithEndpoint(config.Endpoint))
	}
	if config.MaxIdleConns > 0 {
		options = append(options, dynamodb.WithMaxIdleConns(config.MaxIdleConns))
	}
	if config.AccessKeyID != "" {
		options = append(options, dynamodb.WithCredentials(aws.Credentials{
			AccessKeyID:     config.AccessKeyID,
			SecretAccessKey: config.SecretAccessKey,
			SessionToken:    config.SessionToken,
		}))
	}
	client, err := dynamodb.New(options...)
	if err != nil {
		// The driver's error names the missing region or credential and holds
		// no secret, so it is wrapped rather than replaced.
		return nil, fmt.Errorf("middleware.dynamo: %w", err)
	}
	return client, nil
}

// closeRuntime releases the pooled connections during shutdown.
func closeRuntime(context.Context) error {
	state.Lock()
	client := state.client
	state.client = nil
	state.resolver = nil
	state.Unlock()
	if client == nil {
		return nil
	}
	return client.Close()
}
