package dynamo

import (
	"context"
	"strings"
	"testing"

	"github.com/shibukawa/tinygodriver/nosql/dynamodb"
)

func TestResolverForAppliesPrefix(t *testing.T) {
	resolver := resolverFor(Config{TablePrefix: "test-a1-"})
	if resolver == nil {
		t.Fatal("a configured prefix must produce a resolver")
	}
	if got := resolver(context.Background(), "reading"); got != "test-a1-reading" {
		t.Fatalf("prefix resolution = %q, want %q", got, "test-a1-reading")
	}
}

func TestResolverForPrefersAnExplicitMapping(t *testing.T) {
	// A CDK physical name shares nothing with the declared one, which is the
	// case a prefix cannot express and this mapping exists for.
	resolver := resolverFor(Config{
		TablePrefix: "test-",
		TableNames:  []TableName{{Declared: "reading", Deployed: "Stack-Readings-A1B2C3"}},
	})
	if got := resolver(context.Background(), "reading"); got != "Stack-Readings-A1B2C3" {
		t.Fatalf("mapped resolution = %q, want the explicit name", got)
	}
	if got := resolver(context.Background(), "other"); got != "test-other" {
		t.Fatalf("unmapped resolution = %q, want the prefixed name", got)
	}
}

func TestResolverForIsNilWhenNothingIsConfigured(t *testing.T) {
	// A deployment named as declared configures nothing, and a nil resolver is
	// what the driver reads as "send the declared name".
	if resolverFor(Config{}) != nil {
		t.Fatal("an unconfigured naming must produce no resolver")
	}
	if got := resolve(context.Background(), nil, "reading"); got != "reading" {
		t.Fatalf("nil resolver = %q, want the declared name", got)
	}
}

func TestValidateTableNamesRejectsAnUnmappedEntry(t *testing.T) {
	// An entry naming a table nothing declares does nothing at run time, and
	// looking correct is the whole problem with it.
	config := Config{TableNames: []TableName{{Declared: "typo", Deployed: "whatever"}}}
	err := validateTableNames(context.Background(), config, resolverFor(config))
	if err == nil {
		t.Fatal("an unmapped entry must be reported")
	}
	if !strings.Contains(err.Error(), `"typo"`) {
		t.Fatalf("error must name the entry, got %v", err)
	}
}

func TestValidateResolvedNameRejectsAnOverlongName(t *testing.T) {
	err := validateResolvedName("reading", strings.Repeat("a", maxTableNameLength+1))
	if err == nil {
		t.Fatal("a name over the DynamoDB limit must be rejected")
	}
	// The declared name has to be in the message: the resolved one is what the
	// service would have complained about, and it appears in no service error.
	if !strings.Contains(err.Error(), `"reading"`) {
		t.Fatalf("error must name the declared table, got %v", err)
	}
}

func TestValidateResolvedNameRejectsAnIllegalCharacter(t *testing.T) {
	if err := validateResolvedName("reading", "bad name"); err == nil {
		t.Fatal("a space is not a legal DynamoDB table name character")
	}
}

func TestKeyScheduleMismatchAcceptsAMatchingSchema(t *testing.T) {
	want := dynamodb.TableDefinition{
		Name:         "reading",
		PartitionKey: dynamodb.KeyAttribute{Name: "sensor", Type: dynamodb.TypeString},
		SortKey:      &dynamodb.KeyAttribute{Name: "at", Type: dynamodb.TypeNumber},
	}
	described := &dynamodb.TableDescription{Keys: []dynamodb.KeyAttribute{{Name: "sensor"}, {Name: "at"}}}
	if detail := keyScheduleMismatch(want, described); detail != "" {
		t.Fatalf("matching schema reported a mismatch: %s", detail)
	}
}

func TestKeyScheduleMismatchReportsBothShapes(t *testing.T) {
	want := dynamodb.TableDefinition{
		Name:         "reading",
		PartitionKey: dynamodb.KeyAttribute{Name: "sensor", Type: dynamodb.TypeString},
	}
	described := &dynamodb.TableDescription{Keys: []dynamodb.KeyAttribute{{Name: "device"}}}
	detail := keyScheduleMismatch(want, described)
	// An operator needs both shapes: knowing only that they differ does not say
	// whether the code or the deployed table is the surprise.
	if !strings.Contains(detail, "sensor") || !strings.Contains(detail, "device") {
		t.Fatalf("mismatch must name both shapes, got %q", detail)
	}
}

func TestKeyScheduleMismatchReportsADifferentArity(t *testing.T) {
	want := dynamodb.TableDefinition{
		Name:         "reading",
		PartitionKey: dynamodb.KeyAttribute{Name: "sensor", Type: dynamodb.TypeString},
		SortKey:      &dynamodb.KeyAttribute{Name: "at", Type: dynamodb.TypeNumber},
	}
	described := &dynamodb.TableDescription{Keys: []dynamodb.KeyAttribute{{Name: "sensor"}}}
	if keyScheduleMismatch(want, described) == "" {
		t.Fatal("a table missing its sort key must be reported")
	}
}
