package dynamo

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/shibukawa/tinybind-go/dynamobind"
	"github.com/shibukawa/tinygodriver/cloud/aws"
	"github.com/shibukawa/tinygodriver/nosql/dynamodb"
)

// attribute is one wire-encoded DynamoDB value: {"S": "..."} and friends.
type attribute map[string]any

// fakeTables is the smallest server that exercises these stores: the item
// operations they issue over in-memory tables.
//
// Conditions and update expressions are evaluated rather than matched by name,
// because these stores rest their guarantees on them. The evaluator handles the
// forms the stores actually use — a conjunction of existence checks and numeric
// comparisons, and SET and ADD updates — so a store that started using another
// form would fail here rather than pass by accident.
type fakeTables struct {
	mu sync.Mutex
	// items maps a table name to its items, keyed by the wire form of the
	// partition key.
	items map[string]map[string]dynamodb.Item
	// keyOf names the partition key attribute of each table.
	keyOf map[string]string
	// failWith makes the next operation fail.
	failWith string
	// unread makes the next BatchGetItem leave that many keys unprocessed.
	unread int
	// calls counts operations by name.
	calls map[string]int
}

func newFakeTables() *fakeTables {
	return &fakeTables{
		items: map[string]map[string]dynamodb.Item{},
		keyOf: map[string]string{
			DeclaredAllowlistTable:  allowlistKeyAttribute,
			DeclaredCredentialTable: credentialKeyAttribute,
			DeclaredBootstrapTable:  bootstrapKeyAttribute,
		},
		calls: map[string]int{},
	}
}

type wireRequest struct {
	TableName                 string                 `json:"TableName"`
	Key                       dynamodb.Item          `json:"Key"`
	Item                      dynamodb.Item          `json:"Item"`
	ConsistentRead            bool                   `json:"ConsistentRead"`
	ConditionExpression       string                 `json:"ConditionExpression"`
	UpdateExpression          string                 `json:"UpdateExpression"`
	KeyConditionExpression    string                 `json:"KeyConditionExpression"`
	IndexName                 string                 `json:"IndexName"`
	ReturnValues              string                 `json:"ReturnValues"`
	ExpressionAttributeNames  map[string]string      `json:"ExpressionAttributeNames"`
	ExpressionAttributeValues dynamodb.Item          `json:"ExpressionAttributeValues"`
	RequestItems              map[string]batchTarget `json:"RequestItems"`
}

type batchTarget struct {
	Keys []dynamodb.Item `json:"Keys"`
}

func (fake *fakeTables) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	target := request.Header.Get("X-Amz-Target")
	operation := target[strings.LastIndex(target, ".")+1:]
	body, _ := io.ReadAll(request.Body)
	var req wireRequest
	_ = json.Unmarshal(body, &req)

	fake.mu.Lock()
	defer fake.mu.Unlock()
	fake.calls[operation]++
	if fake.failWith != "" {
		name := fake.failWith
		fake.failWith = ""
		fake.reject(writer, name)
		return
	}
	writer.Header().Set("Content-Type", "application/x-amz-json-1.0")

	switch operation {
	case "PutItem":
		fake.putItem(writer, req)
	case "GetItem":
		fake.getItem(writer, req)
	case "UpdateItem":
		fake.updateItem(writer, req)
	case "DeleteItem":
		fake.deleteItem(writer, req)
	case "Query":
		fake.query(writer, req)
	case "BatchGetItem":
		fake.batchGet(writer, req)
	default:
		fake.reject(writer, "ValidationException")
	}
}

func (fake *fakeTables) reject(writer http.ResponseWriter, name string) {
	writer.WriteHeader(http.StatusBadRequest)
	_ = json.NewEncoder(writer).Encode(map[string]string{
		"__type": "com.amazonaws.dynamodb.v20120810#" + name,
	})
}

func (fake *fakeTables) table(name string) map[string]dynamodb.Item {
	if fake.items[name] == nil {
		fake.items[name] = map[string]dynamodb.Item{}
	}
	return fake.items[name]
}

// keyString renders the partition key value as a comparable string, so binary
// and string keys share one map.
func (fake *fakeTables) keyString(table string, holder dynamodb.Item) string {
	value := holder[fake.keyOf[table]]
	if text, held := value.AsString(); held {
		return "S:" + text
	}
	if raw, held := value.AsBytes(); held {
		return "B:" + string(raw)
	}
	return ""
}

func (fake *fakeTables) putItem(writer http.ResponseWriter, req wireRequest) {
	items := fake.table(req.TableName)
	key := fake.keyString(req.TableName, req.Item)
	if !evaluate(req.ConditionExpression, req.ExpressionAttributeNames, req.ExpressionAttributeValues, items[key]) {
		fake.reject(writer, "ConditionalCheckFailedException")
		return
	}
	items[key] = req.Item
	_ = json.NewEncoder(writer).Encode(map[string]any{})
}

func (fake *fakeTables) getItem(writer http.ResponseWriter, req wireRequest) {
	item, present := fake.table(req.TableName)[fake.keyString(req.TableName, req.Key)]
	if !present {
		// A miss is a 200 with no Item member, which the driver maps to its
		// item-not-found sentinel.
		_ = json.NewEncoder(writer).Encode(map[string]any{})
		return
	}
	_ = json.NewEncoder(writer).Encode(map[string]any{"Item": item})
}

func (fake *fakeTables) updateItem(writer http.ResponseWriter, req wireRequest) {
	items := fake.table(req.TableName)
	key := fake.keyString(req.TableName, req.Key)
	existing, present := items[key]
	if !evaluate(req.ConditionExpression, req.ExpressionAttributeNames, req.ExpressionAttributeValues, existing) {
		fake.reject(writer, "ConditionalCheckFailedException")
		return
	}
	updated := dynamodb.Item{}
	if present {
		for name, value := range existing {
			updated[name] = value
		}
	} else {
		for name, value := range req.Key {
			updated[name] = value
		}
	}
	applyUpdate(req.UpdateExpression, req.ExpressionAttributeNames, req.ExpressionAttributeValues, updated)
	items[key] = updated
	response := map[string]any{}
	if req.ReturnValues == "ALL_NEW" {
		response["Attributes"] = updated
	}
	_ = json.NewEncoder(writer).Encode(response)
}

func (fake *fakeTables) deleteItem(writer http.ResponseWriter, req wireRequest) {
	items := fake.table(req.TableName)
	key := fake.keyString(req.TableName, req.Key)
	existing, present := items[key]
	if !evaluate(req.ConditionExpression, req.ExpressionAttributeNames, req.ExpressionAttributeValues, existing) {
		fake.reject(writer, "ConditionalCheckFailedException")
		return
	}
	response := map[string]any{}
	if present && req.ReturnValues == "ALL_OLD" {
		response["Attributes"] = existing
	}
	delete(items, key)
	_ = json.NewEncoder(writer).Encode(response)
}

// query answers the one key condition these stores issue, over the account
// index, by filtering the table.
//
// It applies the index projection. A projection that omits a field the caller
// reads is a real defect that returns zero values rather than an error, so the
// fake has to drop the same attributes DynamoDB would.
func (fake *fakeTables) query(writer http.ResponseWriter, req wireRequest) {
	matches := []dynamodb.Item{}
	for _, item := range fake.table(req.TableName) {
		if !evaluate(req.KeyConditionExpression, req.ExpressionAttributeNames, req.ExpressionAttributeValues, item) {
			continue
		}
		matches = append(matches, fake.project(req.TableName, req.IndexName, item))
	}
	_ = json.NewEncoder(writer).Encode(map[string]any{"Items": matches, "Count": len(matches)})
}

// project reduces an item to what the named index carries: both key attributes
// and whatever the definition includes.
func (fake *fakeTables) project(table, index string, item dynamodb.Item) dynamodb.Item {
	if index == "" {
		return item
	}
	definition := fake.definitionOf(table)
	for _, secondary := range definition.GlobalIndexes {
		if secondary.Name != index {
			continue
		}
		if secondary.Projection == "" || secondary.Projection == "ALL" {
			return item
		}
		kept := dynamodb.Item{}
		names := append([]string{definition.PartitionKey.Name, secondary.PartitionKey.Name}, secondary.Include...)
		if secondary.SortKey != nil {
			names = append(names, secondary.SortKey.Name)
		}
		for _, name := range names {
			if value, present := item[name]; present {
				kept[name] = value
			}
		}
		return kept
	}
	panic("fake: unknown index " + index)
}

func (fake *fakeTables) definitionOf(table string) dynamodb.TableDefinition {
	switch table {
	case DeclaredAllowlistTable:
		return AllowlistTable(table)
	case DeclaredCredentialTable:
		return CredentialTable(table)
	case DeclaredBootstrapTable:
		return BootstrapTable(table)
	}
	panic("fake: unknown table " + table)
}

func (fake *fakeTables) batchGet(writer http.ResponseWriter, req wireRequest) {
	responses := map[string][]dynamodb.Item{}
	unprocessed := map[string]any{}
	for table, wanted := range req.RequestItems {
		keys := wanted.Keys
		if fake.unread > 0 {
			cut := min(fake.unread, len(keys))
			fake.unread = 0
			unprocessed[table] = map[string]any{"Keys": keys[len(keys)-cut:]}
			keys = keys[:len(keys)-cut]
		}
		for _, key := range keys {
			if item, present := fake.table(table)[fake.keyString(table, key)]; present {
				responses[table] = append(responses[table], item)
			}
		}
	}
	_ = json.NewEncoder(writer).Encode(map[string]any{
		"Responses": responses, "UnprocessedKeys": unprocessed,
	})
}

// evaluate reads a conjunction of the clause forms these stores use. An empty
// expression admits everything, which is what no condition means.
func evaluate(expression string, names map[string]string, values dynamodb.Item, item dynamodb.Item) bool {
	if strings.TrimSpace(expression) == "" {
		return true
	}
	for _, clause := range strings.Split(expression, " AND ") {
		if !evaluateClause(strings.TrimSpace(clause), names, values, item) {
			return false
		}
	}
	return true
}

func evaluateClause(clause string, names map[string]string, values dynamodb.Item, item dynamodb.Item) bool {
	resolveName := func(token string) string {
		if actual, aliased := names[token]; aliased {
			return actual
		}
		return token
	}
	switch {
	case strings.HasPrefix(clause, "attribute_not_exists("):
		name := resolveName(strings.TrimSuffix(strings.TrimPrefix(clause, "attribute_not_exists("), ")"))
		_, present := item[name]
		return !present
	case strings.HasPrefix(clause, "attribute_exists("):
		name := resolveName(strings.TrimSuffix(strings.TrimPrefix(clause, "attribute_exists("), ")"))
		_, present := item[name]
		return present
	}
	for _, operator := range []string{" < ", " > ", " = "} {
		left, right, split := strings.Cut(clause, operator)
		if !split {
			continue
		}
		stored, present := item[resolveName(strings.TrimSpace(left))]
		if !present {
			return false
		}
		wanted := values[strings.TrimSpace(right)]
		if text, held := stored.AsString(); held {
			other, _ := wanted.AsString()
			return strings.TrimSpace(operator) == "=" && text == other
		}
		number, held := stored.AsInt()
		if !held {
			return false
		}
		other, _ := wanted.AsInt()
		switch strings.TrimSpace(operator) {
		case "<":
			return number < other
		case ">":
			return number > other
		default:
			return number == other
		}
	}
	panic("fake: unsupported condition clause " + clause)
}

// applyUpdate performs the SET and ADD forms these stores issue.
func applyUpdate(expression string, names map[string]string, values dynamodb.Item, item dynamodb.Item) {
	resolveName := func(token string) string {
		if actual, aliased := names[token]; aliased {
			return actual
		}
		return token
	}
	body, found := strings.CutPrefix(expression, "SET ")
	if found {
		for _, assignment := range strings.Split(body, ",") {
			left, right, split := strings.Cut(assignment, "=")
			if !split {
				panic("fake: unsupported assignment " + assignment)
			}
			item[resolveName(strings.TrimSpace(left))] = values[strings.TrimSpace(right)]
		}
		return
	}
	if body, found = strings.CutPrefix(expression, "ADD "); found {
		fields := strings.Fields(body)
		if len(fields) != 2 {
			panic("fake: unsupported add " + expression)
		}
		name := resolveName(fields[0])
		current, _ := item[name].AsInt()
		delta, _ := values[fields[1]].AsInt()
		item[name] = dynamodb.N(current + delta)
		return
	}
	panic("fake: unsupported update expression " + expression)
}

func newTestContext(t *testing.T, fake *fakeTables) context.Context {
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
	return dynamobind.WithClient(context.Background(), client)
}

func numberText(value int64) string { return strconv.FormatInt(value, 10) }
