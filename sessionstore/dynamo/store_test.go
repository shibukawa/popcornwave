package dynamo

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/shibukawa/popcornweb/session"
	"github.com/shibukawa/tinybind-go/dynamobind"
	"github.com/shibukawa/tinygodriver/cloud/aws"
	"github.com/shibukawa/tinygodriver/nosql/dynamodb"
)

// fakeItems is the smallest server that exercises this store: four item
// operations over one in-memory table.
//
// It evaluates no expression language. The store issues exactly one condition
// and one update, so the fake implements those two by name rather than
// pretending to be DynamoDB; a test that changed either would have to change
// this too, which is the point.
type fakeItems struct {
	mu sync.Mutex
	// items maps a key hash to its stored attributes.
	items map[string]map[string]map[string]any
	// failWith makes the next operation fail, for the unavailable path.
	failWith string
	// consistentReads counts reads asking for strong consistency.
	consistentReads int
	// reads counts every GetItem.
	reads int
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
		ConsistentRead            bool                      `json:"ConsistentRead"`
		ConditionExpression       string                    `json:"ConditionExpression"`
		UpdateExpression          string                    `json:"UpdateExpression"`
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
		fake.items[stringOf(decoded.Item[keyAttribute])] = decoded.Item
		_ = json.NewEncoder(writer).Encode(map[string]any{})
	case "GetItem":
		fake.reads++
		if decoded.ConsistentRead {
			fake.consistentReads++
		}
		item, present := fake.items[stringOf(decoded.Key[keyAttribute])]
		if !present {
			// A miss is a 200 with no Item member, which the driver maps to
			// its item-not-found sentinel.
			_ = json.NewEncoder(writer).Encode(map[string]any{})
			return
		}
		_ = json.NewEncoder(writer).Encode(map[string]any{"Item": item})
	case "DeleteItem":
		delete(fake.items, stringOf(decoded.Key[keyAttribute]))
		_ = json.NewEncoder(writer).Encode(map[string]any{})
	case "UpdateItem":
		key := stringOf(decoded.Key[keyAttribute])
		item, present := fake.items[key]
		now := numberOf(decoded.ExpressionAttributeValues[":now"])
		// The store's only condition: the item exists and is not yet dead.
		if !present || numberOf(item[deadAtAttribute]) <= now {
			writer.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(writer).Encode(map[string]string{
				"__type": "com.amazonaws.dynamodb.v20120810#ConditionalCheckFailedException",
			})
			return
		}
		item[lastSeenAtAttribute] = numberValue(decoded.ExpressionAttributeValues[":last_seen"])
		if idle, renews := decoded.ExpressionAttributeValues[":idle"]; renews {
			item[idleExpiresAtAttribute] = numberValue(idle)
			item[deadAtAttribute] = numberValue(idle)
		}
		_ = json.NewEncoder(writer).Encode(map[string]any{})
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
	var parsed int64
	_, _ = fmtSscan(text, &parsed)
	return parsed
}

func numberValue(attribute map[string]any) map[string]any {
	return map[string]any{"N": attribute["N"]}
}

// fmtSscan keeps the import list of this file to what the store itself uses.
func fmtSscan(text string, out *int64) (int, error) {
	var value int64
	var negative bool
	for index, r := range text {
		if index == 0 && r == '-' {
			negative = true
			continue
		}
		if r < '0' || r > '9' {
			return 0, errors.New("not a number")
		}
		value = value*10 + int64(r-'0')
	}
	if negative {
		value = -value
	}
	*out = value
	return 1, nil
}

func newTestStore(t *testing.T, fake *fakeItems, options Options) (*Store, context.Context) {
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
	return NewStore(options), dynamobind.WithClient(context.Background(), client)
}

func sampleRecord(now time.Time) session.RawRecord {
	return session.RawRecord{
		Payload:         []byte(`{"user_id":"u-1"}`),
		CreatedAt:       now,
		AuthenticatedAt: now,
		LastSeenAt:      now,
		ExpiresAt:       now.Add(time.Hour),
		IdleExpiresAt:   now.Add(15 * time.Minute),
		Method:          "oidc",
		Version:         1,
	}
}

func TestPutAndGetRoundTrip(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	store, ctx := newTestStore(t, newFakeItems(), Options{Now: func() time.Time { return now }})

	want := sampleRecord(now)
	if err := store.Put(ctx, "hash-1", want); err != nil {
		t.Fatal(err)
	}
	got, err := store.Get(ctx, "hash-1")
	if err != nil {
		t.Fatal(err)
	}
	if string(got.Payload) != string(want.Payload) {
		t.Fatalf("payload = %q, want %q", got.Payload, want.Payload)
	}
	if !got.ExpiresAt.Equal(want.ExpiresAt) {
		t.Fatalf("expiry = %s, want %s", got.ExpiresAt, want.ExpiresAt)
	}
	if got.Method != "oidc" || got.Version != 1 {
		t.Fatalf("method/version = %q/%d", got.Method, got.Version)
	}
}

func TestGetRetriesAMissConsistently(t *testing.T) {
	// The hazard is a false miss right after a login rotation. A hit must not
	// pay for it, and a miss must not be believed on the first, cheap read.
	now := time.Now().UTC()
	fake := newFakeItems()
	store, ctx := newTestStore(t, fake, Options{Now: func() time.Time { return now }})

	if _, err := store.Get(ctx, "absent"); !errors.Is(err, session.ErrNotFound) {
		t.Fatalf("missing key = %v, want ErrNotFound", err)
	}
	if fake.reads != 2 || fake.consistentReads != 1 {
		t.Fatalf("a miss made %d reads (%d consistent), want 2 with 1 consistent",
			fake.reads, fake.consistentReads)
	}

	fake.reads, fake.consistentReads = 0, 0
	if err := store.Put(ctx, "hash-1", sampleRecord(now)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(ctx, "hash-1"); err != nil {
		t.Fatal(err)
	}
	if fake.reads != 1 || fake.consistentReads != 0 {
		t.Fatalf("a hit made %d reads (%d consistent), want 1 eventually consistent",
			fake.reads, fake.consistentReads)
	}
}

func TestGetWithConsistentReadDoesNotRetry(t *testing.T) {
	now := time.Now().UTC()
	fake := newFakeItems()
	store, ctx := newTestStore(t, fake, Options{ConsistentRead: true, Now: func() time.Time { return now }})

	if _, err := store.Get(ctx, "absent"); !errors.Is(err, session.ErrNotFound) {
		t.Fatal(err)
	}
	// With the first read already consistent there is nothing for a retry to
	// catch, so a second one would be pure cost.
	if fake.reads != 1 {
		t.Fatalf("consistent mode made %d reads, want 1", fake.reads)
	}
}

func TestGetReportsAnExpiredRecord(t *testing.T) {
	// Expiry is decided here rather than waiting for TTL, which is documented
	// as taking up to two days.
	stored := time.Now().UTC()
	clock := stored
	store, ctx := newTestStore(t, newFakeItems(), Options{Now: func() time.Time { return clock }})
	if err := store.Put(ctx, "hash-1", sampleRecord(stored)); err != nil {
		t.Fatal(err)
	}
	clock = stored.Add(2 * time.Hour)
	if _, err := store.Get(ctx, "hash-1"); !errors.Is(err, session.ErrExpired) {
		t.Fatalf("expired record = %v, want ErrExpired", err)
	}
}

func TestTouchRenewsAndRefusesToRevive(t *testing.T) {
	stored := time.Now().UTC().Truncate(time.Second)
	clock := stored
	store, ctx := newTestStore(t, newFakeItems(), Options{Now: func() time.Time { return clock }})
	if err := store.Put(ctx, "hash-1", sampleRecord(stored)); err != nil {
		t.Fatal(err)
	}

	clock = stored.Add(5 * time.Minute)
	renewed := clock.Add(15 * time.Minute)
	if err := store.Touch(ctx, "hash-1", clock, renewed); err != nil {
		t.Fatal(err)
	}
	got, err := store.Get(ctx, "hash-1")
	if err != nil {
		t.Fatal(err)
	}
	if !got.IdleExpiresAt.Equal(renewed) {
		t.Fatalf("idle expiry = %s, want %s", got.IdleExpiresAt, renewed)
	}

	// Past the renewed idle expiry the condition fails, so the renewal cannot
	// bring the record back.
	clock = renewed.Add(time.Minute)
	if err := store.Touch(ctx, "hash-1", clock, clock.Add(time.Minute)); !errors.Is(err, session.ErrNotFound) {
		t.Fatalf("renewing a dead record = %v, want ErrNotFound", err)
	}
}

func TestTouchOnAMissingRecordDoesNotCreateOne(t *testing.T) {
	now := time.Now().UTC()
	store, ctx := newTestStore(t, newFakeItems(), Options{Now: func() time.Time { return now }})
	if err := store.Touch(ctx, "absent", now, now.Add(time.Minute)); !errors.Is(err, session.ErrNotFound) {
		t.Fatalf("touching an absent key = %v, want ErrNotFound", err)
	}
	if _, err := store.Get(ctx, "absent"); !errors.Is(err, session.ErrNotFound) {
		t.Fatal("a refused renewal must leave nothing behind")
	}
}

func TestDeleteIsIdempotent(t *testing.T) {
	now := time.Now().UTC()
	store, ctx := newTestStore(t, newFakeItems(), Options{Now: func() time.Time { return now }})
	if err := store.Put(ctx, "hash-1", sampleRecord(now)); err != nil {
		t.Fatal(err)
	}
	for attempt := range 2 {
		if err := store.Delete(ctx, "hash-1"); err != nil {
			t.Fatalf("delete %d: %v", attempt, err)
		}
	}
	if _, err := store.Get(ctx, "hash-1"); !errors.Is(err, session.ErrNotFound) {
		t.Fatalf("after delete = %v, want ErrNotFound", err)
	}
}

func TestOperationsWithoutAClientNameTheImport(t *testing.T) {
	store := NewStore(Options{})
	err := store.Put(context.Background(), "hash-1", sampleRecord(time.Now()))
	if !errors.Is(err, session.ErrUnavailable) {
		t.Fatalf("no client = %v, want ErrUnavailable", err)
	}
	if !strings.Contains(err.Error(), "database/dynamo") {
		t.Fatalf("error must name the import to add, got %v", err)
	}
}

func TestDriverFailureMapsToUnavailable(t *testing.T) {
	now := time.Now().UTC()
	fake := newFakeItems()
	store, ctx := newTestStore(t, fake, Options{Now: func() time.Time { return now }})
	// A validation failure, because the driver retries a throttle by itself and
	// a single injected one would never reach the caller.
	fake.failWith = "ValidationException"
	err := store.Put(ctx, "hash-1", sampleRecord(now))
	if !errors.Is(err, session.ErrUnavailable) {
		t.Fatalf("rejected write = %v, want ErrUnavailable", err)
	}
	// The driver sentinel stays reachable through the mapping, so a caller can
	// still tell one failure from another.
	if !errors.Is(err, dynamodb.ErrValidation) {
		t.Fatalf("driver sentinel must survive the mapping, got %v", err)
	}
}

func TestTableDefinitionUsesTheKeyAttribute(t *testing.T) {
	// One constant names the key, so the definition and the item cannot drift.
	definition := Table("deployed-sessions")
	if definition.Name != "deployed-sessions" {
		t.Fatalf("name = %q", definition.Name)
	}
	if definition.PartitionKey.Name != keyAttribute {
		t.Fatalf("partition key = %q, want %q", definition.PartitionKey.Name, keyAttribute)
	}
	if definition.SortKey != nil {
		t.Fatal("the session table has no sort key")
	}
}
