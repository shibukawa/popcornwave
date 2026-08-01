package dynamo

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/shibukawa/tinybind-go/dynamobind"
	"github.com/shibukawa/tinygodriver/cloud/aws"
	"github.com/shibukawa/tinygodriver/nosql/dynamodb"
)

// fakeDynamo answers the three table operations the migrator uses. It is not a
// DynamoDB implementation: it is the smallest server that lets the plan, the
// apply and the verify paths be exercised without an emulator.
type fakeDynamo struct {
	mu sync.Mutex
	// tables maps a table name to its key attribute names, in key schema
	// order. A table present here exists.
	tables map[string][]string
	// calls counts operations by name, so a test can assert that a matching
	// plan sends no write.
	calls map[string]int
}

func newFakeDynamo() *fakeDynamo {
	return &fakeDynamo{tables: map[string][]string{}, calls: map[string]int{}}
}

func (fake *fakeDynamo) countOf(operation string) int {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	return fake.calls[operation]
}

func (fake *fakeDynamo) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	target := request.Header.Get("X-Amz-Target")
	operation := target[strings.LastIndex(target, ".")+1:]
	body, _ := io.ReadAll(request.Body)
	var decoded struct {
		TableName string `json:"TableName"`
		KeySchema []struct {
			AttributeName string `json:"AttributeName"`
		} `json:"KeySchema"`
	}
	_ = json.Unmarshal(body, &decoded)

	fake.mu.Lock()
	fake.calls[operation]++
	keys, exists := fake.tables[decoded.TableName]
	fake.mu.Unlock()

	writer.Header().Set("Content-Type", "application/x-amz-json-1.0")
	switch operation {
	case "DescribeTable":
		if !exists {
			fake.writeError(writer, "ResourceNotFoundException")
			return
		}
		fake.writeTable(writer, "Table", decoded.TableName, keys)
	case "CreateTable":
		if exists {
			fake.writeError(writer, "ResourceInUseException")
			return
		}
		created := make([]string, 0, len(decoded.KeySchema))
		for _, element := range decoded.KeySchema {
			created = append(created, element.AttributeName)
		}
		fake.mu.Lock()
		fake.tables[decoded.TableName] = created
		fake.mu.Unlock()
		fake.writeTable(writer, "TableDescription", decoded.TableName, created)
	default:
		fake.writeError(writer, "ValidationException")
	}
}

func (fake *fakeDynamo) writeTable(writer http.ResponseWriter, member, name string, keys []string) {
	schema := make([]map[string]string, 0, len(keys))
	for index, key := range keys {
		keyType := "HASH"
		if index > 0 {
			keyType = "RANGE"
		}
		schema = append(schema, map[string]string{"AttributeName": key, "KeyType": keyType})
	}
	_ = json.NewEncoder(writer).Encode(map[string]any{
		member: map[string]any{
			"TableName":   name,
			"TableStatus": "ACTIVE",
			"KeySchema":   schema,
		},
	})
}

func (fake *fakeDynamo) writeError(writer http.ResponseWriter, name string) {
	writer.WriteHeader(http.StatusBadRequest)
	_ = json.NewEncoder(writer).Encode(map[string]string{
		"__type": "com.amazonaws.dynamodb.v20120810#" + name,
	})
}

// withRegisteredTables installs a table registry for one test and restores the
// previous one afterwards, so the package-level registry stays test-local.
func withRegisteredTables(t *testing.T, factories map[string]TableFactory) {
	t.Helper()
	tableState.Lock()
	previous := tableState.factories
	tableState.factories = factories
	tableState.Unlock()
	t.Cleanup(func() {
		tableState.Lock()
		tableState.factories = previous
		tableState.Unlock()
	})
}

func readingTable(name string) dynamodb.TableDefinition {
	return dynamodb.TableDefinition{
		Name:         name,
		PartitionKey: dynamodb.KeyAttribute{Name: "sensor", Type: dynamodb.TypeString},
		SortKey:      &dynamodb.KeyAttribute{Name: "at", Type: dynamodb.TypeNumber},
	}
}

// testContext starts the fake server and returns a context carrying a client
// pointed at it, installed exactly as the middleware installs one.
func testContext(t *testing.T, fake *fakeDynamo, resolver TableResolver) context.Context {
	t.Helper()
	server := httptest.NewServer(fake)
	t.Cleanup(server.Close)
	client, err := dynamodb.New(
		dynamodb.WithEndpoint(server.URL),
		dynamodb.WithRegion("ap-northeast-1"),
		dynamodb.WithCredentials(aws.Credentials{AccessKeyID: "test", SecretAccessKey: "test"}),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return dynamobind.WithClient(context.Background(), client, dynamobind.WithTableNames(resolver))
}

func TestPlanReportsAMissingTable(t *testing.T) {
	withRegisteredTables(t, map[string]TableFactory{"reading": readingTable})
	fake := newFakeDynamo()
	ctx := testContext(t, fake, resolverFor(Config{TablePrefix: "test-"}))

	plan, err := planWith(ctx, mustClient(t, ctx), resolverFor(Config{TablePrefix: "test-"}))
	if err != nil {
		t.Fatal(err)
	}
	if len(plan) != 1 {
		t.Fatalf("plan has %d entries, want 1", len(plan))
	}
	if plan[0].Change != ChangeCreate {
		t.Fatalf("change = %q, want %q", plan[0].Change, ChangeCreate)
	}
	// The plan reports both names: the declared one is what source says, and
	// the deployed one is what an operator would look for in the console.
	if plan[0].Declared != "reading" || plan[0].Deployed != "test-reading" {
		t.Fatalf("plan names = %q/%q", plan[0].Declared, plan[0].Deployed)
	}
	if writes := fake.countOf("CreateTable"); writes != 0 {
		t.Fatalf("planning sent %d writes, want 0", writes)
	}
}

func TestMigrateCreatesAndIsIdempotent(t *testing.T) {
	withRegisteredTables(t, map[string]TableFactory{"reading": readingTable})
	fake := newFakeDynamo()
	resolver := resolverFor(Config{TablePrefix: "test-"})
	ctx := testContext(t, fake, resolver)
	client := mustClient(t, ctx)

	plan, err := planWith(ctx, client, resolver)
	if err != nil {
		t.Fatal(err)
	}
	result, err := applyPlan(ctx, client, plan)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Created) != 1 || result.Created[0] != "test-reading" {
		t.Fatalf("created = %v, want [test-reading]", result.Created)
	}

	// A second run is a no-op by construction rather than by bookkeeping:
	// there is no version table, so this is the whole idempotence claim.
	plan, err = planWith(ctx, client, resolver)
	if err != nil {
		t.Fatal(err)
	}
	before := fake.countOf("CreateTable")
	result, err = applyPlan(ctx, client, plan)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Created) != 0 {
		t.Fatalf("second run created %v, want nothing", result.Created)
	}
	if after := fake.countOf("CreateTable"); after != before {
		t.Fatalf("second run sent %d writes, want none", after-before)
	}
}

func TestMigrateRefusesAKeySchemaMismatch(t *testing.T) {
	withRegisteredTables(t, map[string]TableFactory{"reading": readingTable})
	fake := newFakeDynamo()
	// A table deployed with a different partition key. DynamoDB cannot alter a
	// key, so this is reported and never performed.
	fake.tables["reading"] = []string{"device", "at"}
	ctx := testContext(t, fake, nil)
	client := mustClient(t, ctx)

	plan, err := planWith(ctx, client, nil)
	if err != nil {
		t.Fatal(err)
	}
	if plan[0].Change != ChangeMismatch {
		t.Fatalf("change = %q, want %q", plan[0].Change, ChangeMismatch)
	}
	before := fake.countOf("CreateTable")
	if _, err := applyPlan(ctx, client, plan); err == nil {
		t.Fatal("a mismatch must stop the run")
	} else if !strings.Contains(err.Error(), "sensor") || !strings.Contains(err.Error(), "device") {
		t.Fatalf("error must name both shapes, got %v", err)
	}
	if after := fake.countOf("CreateTable"); after != before {
		t.Fatal("a refused plan must send no write")
	}
}

func TestVerifyRefusesToServeWithoutTheTable(t *testing.T) {
	withRegisteredTables(t, map[string]TableFactory{"reading": readingTable})
	fake := newFakeDynamo()
	ctx := testContext(t, fake, nil)
	client := mustClient(t, ctx)

	err := verify(ctx, client, nil)
	if err == nil {
		t.Fatal("a missing table must refuse startup")
	}
	// The message has to say what would fix it; deployment tooling owns the
	// table in production and pw migrate owns it in development.
	if !strings.Contains(err.Error(), "pw migrate up") {
		t.Fatalf("error must name the remedy, got %v", err)
	}
	if writes := fake.countOf("CreateTable"); writes != 0 {
		t.Fatal("verification must create nothing")
	}
}

func TestVerifyPassesOnAMatchingSchema(t *testing.T) {
	withRegisteredTables(t, map[string]TableFactory{"reading": readingTable})
	fake := newFakeDynamo()
	fake.tables["test-reading"] = []string{"sensor", "at"}
	resolver := resolverFor(Config{TablePrefix: "test-"})
	ctx := testContext(t, fake, resolver)

	if err := verify(ctx, mustClient(t, ctx), resolver); err != nil {
		t.Fatalf("a matching schema must verify: %v", err)
	}
}

func TestPlanWithoutAClientNamesTheImport(t *testing.T) {
	withRegisteredTables(t, map[string]TableFactory{"reading": readingTable})
	_, err := Plan(context.Background())
	if err == nil {
		t.Fatal("a context without a client must fail")
	}
	if !strings.Contains(err.Error(), "middleware.dynamo") {
		t.Fatalf("error must name the configuration to enable, got %v", err)
	}
}

func mustClient(t *testing.T, ctx context.Context) *dynamodb.Client {
	t.Helper()
	client, err := dynamobind.ClientFromContext(ctx)
	if err != nil {
		t.Fatal(err)
	}
	return client
}
