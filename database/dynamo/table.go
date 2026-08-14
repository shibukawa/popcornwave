package dynamo

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/shibukawa/tinybind-go/dynamobind"
	"github.com/shibukawa/tinygodriver/nosql/dynamodb"
)

// TableFactory builds one table definition for a deployed name. It is the
// shape tinybind generates beside an item codec, so a registered entry is the
// generated constructor itself rather than a copy of what it returns.
type TableFactory func(name string) dynamodb.TableDefinition

// TableResolver maps a declared table name onto the deployed one.
//
// It is an alias rather than a type of its own: this is the seam tinybind runs
// inside every entry, and a second named type would only need converting back.
type TableResolver = dynamobind.TableResolver

var tableState struct {
	sync.RWMutex
	factories map[string]TableFactory
}

// RegisterTable records the definition of one declared table. Generated code
// registers an application table and an imported framework package registers
// its own, so the desired schema is exactly what the binary links.
//
// Registering the same declared name twice is a programming error rather than a
// configuration one: two definitions of one table cannot both be right.
func RegisterTable(declared string, factory TableFactory) {
	if strings.TrimSpace(declared) == "" {
		panic("popcornwave/database/dynamo: empty declared table name")
	}
	if factory == nil {
		panic("popcornwave/database/dynamo: table " + declared + " has no definition")
	}
	tableState.Lock()
	defer tableState.Unlock()
	if tableState.factories == nil {
		tableState.factories = make(map[string]TableFactory)
	}
	if _, taken := tableState.factories[declared]; taken {
		panic("popcornwave/database/dynamo: table " + declared + " is already registered")
	}
	tableState.factories[declared] = factory
}

// registeredTables lists the declared names in a stable order, so a plan, a
// startup summary and a printed schema never reorder between runs.
func registeredTables() []string {
	tableState.RLock()
	defer tableState.RUnlock()
	names := make([]string, 0, len(tableState.factories))
	for name := range tableState.factories {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// tableFactory returns the registered definition of one declared name.
func tableFactory(declared string) (TableFactory, bool) {
	tableState.RLock()
	defer tableState.RUnlock()
	factory, ok := tableState.factories[declared]
	return factory, ok
}

// resolverFor composes the configured naming into the one function every
// runtime entry and the migrator both use. An explicit mapping wins, then the
// prefix, and otherwise the declared name stands.
//
// It takes no context of its own: the framework installs one process-level
// resolver, because the migrator has no request to read and a name it could not
// reproduce would create a table no handler finds.
func resolverFor(config Config) TableResolver {
	names := make(map[string]string, len(config.TableNames))
	for _, mapping := range config.TableNames {
		names[mapping.Declared] = mapping.Deployed
	}
	prefix := config.TablePrefix
	if len(names) == 0 && prefix == "" {
		return nil
	}
	return func(_ context.Context, declared string) string {
		if deployed, mapped := names[declared]; mapped {
			return deployed
		}
		return prefix + declared
	}
}

// resolve applies a resolver, tolerating the nil one that means "as declared".
func resolve(ctx context.Context, resolver TableResolver, declared string) string {
	if resolver == nil {
		return declared
	}
	return resolver(ctx, declared)
}

// validateTableNames checks every registered table against the configured
// naming, and reports a mapping that names a table nothing declares. Such an
// entry does nothing, and looking correct is the whole problem with it.
func validateTableNames(ctx context.Context, config Config, resolver TableResolver) error {
	declaredNames := registeredTables()
	known := make(map[string]bool, len(declaredNames))
	for _, declared := range declaredNames {
		known[declared] = true
		if err := validateResolvedName(declared, resolve(ctx, resolver, declared)); err != nil {
			return err
		}
	}
	unmapped := make([]string, 0, len(config.TableNames))
	for _, mapping := range config.TableNames {
		if !known[mapping.Declared] {
			unmapped = append(unmapped, mapping.Declared)
		}
	}
	if len(unmapped) > 0 {
		sort.Strings(unmapped)
		return fmt.Errorf(
			"middleware.dynamo.table_names names %s, which no registered table declares",
			strings.Join(quoteAll(unmapped), ", "))
	}
	return nil
}

func quoteAll(values []string) []string {
	quoted := make([]string, len(values))
	for i, value := range values {
		quoted[i] = fmt.Sprintf("%q", value)
	}
	return quoted
}

