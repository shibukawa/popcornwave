package dynamo

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/shibukawa/popcornweb/authstate"
	"github.com/shibukawa/tinybind-go/dynamobind"
	"github.com/shibukawa/tinygodriver/cloud/aws"
	"github.com/shibukawa/tinygodriver/nosql/dynamodb"
)

// fakeItems is the smallest server that exercises this store: two item
// operations over one in-memory table.
//
// It evaluates no expression language. The store issues exactly one condition,
// so the fake implements that one by name rather than pretending to be
// DynamoDB; a test that changed the condition would have to change this too,
// which is the point.
type fakeItems struct {
	mu    sync.Mutex
	items map[string]map[string]map[string]any
	// failWith makes the next operation fail, for the unavailable path.
	failWith string
	// puts and deletes count what the store actually issued.
	puts    int
	deletes int
}

func newFakeItems() *fakeItems {
	return &fakeItems{items: map[string]map[string]map[string]any{}}
}

func (fake *fakeItems) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	target := request.Header.Get("X-Amz-Target")
	operation := target[strings.LastIndex(target, ".")+1:]
	body, _ := io.ReadAll(request.Body)
	var decoded struct {
		Key                       map[string]map[string]any `json:"Key"`
		Item                      map[string]map[string]any `json:"Item"`
		ConditionExpression       string                    `json:"ConditionExpression"`
		ReturnValues              string                    `json:"ReturnValues"`
		ExpressionAttributeValues map[string]map[string]any `json:"ExpressionAttributeValues"`
	}
	_ = json.Unmarshal(body, &decoded)

	fake.mu.Lock()
	defer fake.mu.Unlock()
	if fake.failWith != "" {
		name := fake.failWith
		fake.failWith = ""
		writer.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(writer).Encode(map[string]string{
			"__type": "com.amazonaws.dynamodb.v20120810#" + name,
		})
		return
	}
	writer.Header().Set("Content-Type", "application/x-amz-json-1.0")

	switch operation {
	case "PutItem":
		fake.puts++
		key := stringOf(decoded.Item[keyAttribute])
		existing, present := fake.items[key]
		// The store's only condition: absent, or already past its deadline.
		if present && numberOf(existing[deadlineAttribute]) > numberOf(decoded.ExpressionAttributeValues[":now"]) {
			writer.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(writer).Encode(map[string]string{
				"__type": "com.amazonaws.dynamodb.v20120810#ConditionalCheckFailedException",
			})
			return
		}
		fake.items[key] = decoded.Item
		_ = json.NewEncoder(writer).Encode(map[string]any{})
	case "DeleteItem":
		fake.deletes++
		key := stringOf(decoded.Key[keyAttribute])
		item, present := fake.items[key]
		delete(fake.items, key)
		response := map[string]any{}
		// Deleting an absent key succeeds and returns no attributes, which is
		// how the store tells a miss from a hit.
		if present && decoded.ReturnValues == "ALL_OLD" {
			response["Attributes"] = item
		}
		_ = json.NewEncoder(writer).Encode(response)
	default:
		writer.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(writer).Encode(map[string]string{
			"__type": "com.amazonaws.dynamodb.v20120810#ValidationException",
		})
	}
}

func stringOf(attribute map[string]any) string {
	value, _ := attribute["S"].(string)
	return value
}

func numberOf(attribute any) int64 {
	typed, ok := attribute.(map[string]any)
	if !ok {
		return 0
	}
	text, _ := typed["N"].(string)
	parsed, _ := strconv.ParseInt(text, 10, 64)
	return parsed
}

// stringCodec is the smallest codec that round trips, so a test asserts store
// behavior rather than encoding behavior.
type stringCodec struct{}

func (stringCodec) Encode(value string) ([]byte, error) { return []byte(value), nil }
func (stringCodec) Decode(encoded []byte) (string, error) {
	if string(encoded) == "corrupt" {
		return "", errors.New("undecodable")
	}
	return string(encoded), nil
}

func newTestStore(t *testing.T, fake *fakeItems, options Options) (authstate.Store[string], context.Context) {
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
	if options.Namespace == "" {
		options.Namespace = "oauth"
	}
	store, err := NewStore[string](stringCodec{}, options)
	if err != nil {
		t.Fatal(err)
	}
	return store, dynamobind.WithClient(context.Background(), client)
}

func TestPutAndTakeRoundTrip(t *testing.T) {
	fake := newFakeItems()
	store, ctx := newTestStore(t, fake, Options{})

	if err := store.Put(ctx, "state-1", "verifier", time.Now().Add(time.Minute)); err != nil {
		t.Fatalf("put = %v", err)
	}
	value, err := store.Take(ctx, "state-1")
	if err != nil || value != "verifier" {
		t.Fatalf("take = %q err = %v", value, err)
	}
}

// The single-use guarantee is the contract's whole point: one removal, one
// answer, whichever caller gets there first.
func TestTakeConsumesTheRecord(t *testing.T) {
	fake := newFakeItems()
	store, ctx := newTestStore(t, fake, Options{})

	if err := store.Put(ctx, "state-1", "verifier", time.Now().Add(time.Minute)); err != nil {
		t.Fatalf("put = %v", err)
	}
	if _, err := store.Take(ctx, "state-1"); err != nil {
		t.Fatalf("first take = %v", err)
	}
	if _, err := store.Take(ctx, "state-1"); !errors.Is(err, authstate.ErrNotFound) {
		t.Fatalf("second take = %v", err)
	}
	// One removal per Take, and no read before it: the atomicity claim rests on
	// there being exactly one request.
	if fake.deletes != 2 {
		t.Fatalf("two takes issued %d deletes", fake.deletes)
	}
}

func TestTakeOfAnUnknownKeyIsNotFound(t *testing.T) {
	fake := newFakeItems()
	store, ctx := newTestStore(t, fake, Options{})

	if _, err := store.Take(ctx, "never-stored"); !errors.Is(err, authstate.ErrNotFound) {
		t.Fatalf("take of an unknown key = %v", err)
	}
}

func TestPutRefusesToOverwriteALiveRecord(t *testing.T) {
	fake := newFakeItems()
	store, ctx := newTestStore(t, fake, Options{})

	expiry := time.Now().Add(time.Minute)
	if err := store.Put(ctx, "state-1", "first", expiry); err != nil {
		t.Fatalf("first put = %v", err)
	}
	if err := store.Put(ctx, "state-1", "second", expiry); !errors.Is(err, authstate.ErrAlreadyExists) {
		t.Fatalf("second put = %v", err)
	}
	// The refusal must not have replaced anything.
	value, err := store.Take(ctx, "state-1")
	if err != nil || value != "first" {
		t.Fatalf("after a refused put = %q err = %v", value, err)
	}
}

// An expired collision is replaced rather than refused, which is the second
// half of the condition and the reason no prune is needed to reuse a key.
func TestPutReplacesAnExpiredRecord(t *testing.T) {
	fake := newFakeItems()
	clock := time.Now()
	store, ctx := newTestStore(t, fake, Options{Now: func() time.Time { return clock }})

	if err := store.Put(ctx, "state-1", "stale", clock.Add(time.Minute)); err != nil {
		t.Fatalf("first put = %v", err)
	}
	clock = clock.Add(2 * time.Minute)
	if err := store.Put(ctx, "state-1", "fresh", clock.Add(time.Minute)); err != nil {
		t.Fatalf("put over an expired record = %v", err)
	}
	value, err := store.Take(ctx, "state-1")
	if err != nil || value != "fresh" {
		t.Fatalf("take = %q err = %v", value, err)
	}
}

// A record past its deadline is never returned, and it stays consumed: the
// removal already happened, and handing back a single-use value would be worse
// than losing it.
func TestTakeAfterExpiryReturnsNothingAndLeavesNothing(t *testing.T) {
	fake := newFakeItems()
	clock := time.Now()
	store, ctx := newTestStore(t, fake, Options{Now: func() time.Time { return clock }})

	if err := store.Put(ctx, "state-1", "verifier", clock.Add(time.Minute)); err != nil {
		t.Fatalf("put = %v", err)
	}
	clock = clock.Add(2 * time.Minute)
	if _, err := store.Take(ctx, "state-1"); !errors.Is(err, authstate.ErrExpired) {
		t.Fatalf("take after expiry = %v", err)
	}
	if _, err := store.Take(ctx, "state-1"); !errors.Is(err, authstate.ErrNotFound) {
		t.Fatalf("expired record survived the take = %v", err)
	}
}

func TestUndecodableRecordStaysConsumed(t *testing.T) {
	fake := newFakeItems()
	store, ctx := newTestStore(t, fake, Options{})

	if err := store.Put(ctx, "state-1", "corrupt", time.Now().Add(time.Minute)); err != nil {
		t.Fatalf("put = %v", err)
	}
	if _, err := store.Take(ctx, "state-1"); !errors.Is(err, authstate.ErrCodec) {
		t.Fatalf("take of an undecodable record = %v", err)
	}
	if _, err := store.Take(ctx, "state-1"); !errors.Is(err, authstate.ErrNotFound) {
		t.Fatalf("undecodable record survived the take = %v", err)
	}
}

// Namespaces share one table, so two protocols using the same correlation key
// must not see each other's record.
func TestNamespacesDoNotCollide(t *testing.T) {
	fake := newFakeItems()
	oauth, ctx := newTestStore(t, fake, Options{Namespace: "oauth"})
	passkey, _ := newTestStore(t, fake, Options{Namespace: "passkey"})

	expiry := time.Now().Add(time.Minute)
	if err := oauth.Put(ctx, "shared", "oauth-value", expiry); err != nil {
		t.Fatalf("oauth put = %v", err)
	}
	if err := passkey.Put(ctx, "shared", "passkey-value", expiry); err != nil {
		t.Fatalf("passkey put under the same key = %v", err)
	}
	value, err := oauth.Take(ctx, "shared")
	if err != nil || value != "oauth-value" {
		t.Fatalf("oauth take = %q err = %v", value, err)
	}
}

func TestOversizedPayloadIsRefusedBeforeTheRequest(t *testing.T) {
	fake := newFakeItems()
	store, ctx := newTestStore(t, fake, Options{MaxValueBytes: 64})

	oversized := strings.Repeat("x", 128)
	if err := store.Put(ctx, "state-1", oversized, time.Now().Add(time.Minute)); !errors.Is(err, authstate.ErrLimitExceeded) {
		t.Fatalf("oversized put = %v", err)
	}
	if fake.puts != 0 {
		t.Fatalf("an oversized record reached the service %d times", fake.puts)
	}
}

func TestInvalidKeyAndExpiryAreRefused(t *testing.T) {
	fake := newFakeItems()
	store, ctx := newTestStore(t, fake, Options{})

	if err := store.Put(ctx, "", "value", time.Now().Add(time.Minute)); !errors.Is(err, authstate.ErrInvalidKey) {
		t.Fatalf("empty key = %v", err)
	}
	if err := store.Put(ctx, "state-1", "value", time.Time{}); !errors.Is(err, authstate.ErrInvalidExpiry) {
		t.Fatalf("zero expiry = %v", err)
	}
	if err := store.Put(ctx, "state-1", "value", time.Now().Add(-time.Minute)); !errors.Is(err, authstate.ErrInvalidExpiry) {
		t.Fatalf("past expiry = %v", err)
	}
	if fake.puts != 0 {
		t.Fatalf("an invalid record reached the service %d times", fake.puts)
	}
}

func TestOperationsWithoutAClientNameTheImport(t *testing.T) {
	store, err := NewStore[string](stringCodec{}, Options{Namespace: "oauth"})
	if err != nil {
		t.Fatal(err)
	}
	err = store.Put(context.Background(), "state-1", "value", time.Now().Add(time.Minute))
	if !errors.Is(err, authstate.ErrUnavailable) || !strings.Contains(err.Error(), "database/dynamo") {
		t.Fatalf("put without a client = %v", err)
	}
	if _, err := store.Take(context.Background(), "state-1"); !errors.Is(err, authstate.ErrUnavailable) {
		t.Fatalf("take without a client = %v", err)
	}
}

func TestDriverFailureMapsToUnavailable(t *testing.T) {
	fake := newFakeItems()
	store, ctx := newTestStore(t, fake, Options{})

	// A validation failure, because the driver retries a throttle by itself and
	// a single injected one would never reach the caller.
	fake.failWith = "ValidationException"
	err := store.Put(ctx, "state-1", "value", time.Now().Add(time.Minute))
	if !errors.Is(err, authstate.ErrUnavailable) {
		t.Fatalf("driver failure = %v", err)
	}
	// The driver sentinel stays reachable through the mapping, so a caller can
	// still tell one failure from another.
	if !errors.Is(err, dynamodb.ErrValidation) {
		t.Fatalf("driver sentinel must survive the mapping, got %v", err)
	}
}

func TestCancelledCallerIsNotReportedAsAnOutage(t *testing.T) {
	fake := newFakeItems()
	store, ctx := newTestStore(t, fake, Options{})
	cancelled, cancel := context.WithCancel(ctx)
	cancel()

	err := store.Put(cancelled, "state-1", "value", time.Now().Add(time.Minute))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled put = %v", err)
	}
	if _, err := store.Take(cancelled, "state-1"); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled take = %v", err)
	}
}

func TestNilContextIsRejectedRatherThanPanicking(t *testing.T) {
	store, err := NewStore[string](stringCodec{}, Options{Namespace: "oauth"})
	if err != nil {
		t.Fatal(err)
	}
	//nolint:staticcheck // the contract requires a stable error rather than a panic
	if err := store.Put(nil, "state-1", "value", time.Now().Add(time.Minute)); err == nil {
		t.Fatal("nil context was accepted")
	}
	//nolint:staticcheck // same
	if _, err := store.Take(nil, "state-1"); err == nil {
		t.Fatal("nil context was accepted")
	}
}

func TestNewStoreRejectsAnUnusableNamespace(t *testing.T) {
	for _, namespace := range []string{"", "with:colon", "with space", strings.Repeat("n", 200)} {
		if _, err := NewStore[string](stringCodec{}, Options{Namespace: namespace}); !errors.Is(err, authstate.ErrInvalidOptions) {
			t.Fatalf("namespace %q was accepted", namespace)
		}
	}
	if _, err := NewStore[string](nil, Options{Namespace: "oauth"}); !errors.Is(err, authstate.ErrInvalidOptions) {
		t.Fatal("a nil codec was accepted")
	}
}

func TestTableDefinitionUsesTheKeyAttribute(t *testing.T) {
	definition := Table("deployed_authstate")
	if definition.Name != "deployed_authstate" {
		t.Fatalf("name = %q", definition.Name)
	}
	if definition.PartitionKey.Name != keyAttribute {
		t.Fatalf("partition key = %q, want %q", definition.PartitionKey.Name, keyAttribute)
	}
	// A sort key would make one namespace one partition, which is the shape
	// this table deliberately does not have.
	if definition.SortKey != nil {
		t.Fatalf("sort key = %#v, want none", definition.SortKey)
	}
}
