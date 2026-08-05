// Package firestoretest is an in-process Datastore server for the framework's
// own Firestore stores.
//
// It exists because the gcloud Datastore emulator is a Java process inside the
// Cloud SDK image, which is too heavy to require of a unit test, and because
// contention cannot be provoked against it reliably. This server is small
// enough to state what it does and does not implement, which a test can then
// rely on.
//
// It implements lookup, commit, beginTransaction, rollback and runQuery over an
// in-memory map. Transactions are serialized: one is open at a time, and a
// commit whose base versions moved is reported as ABORTED, which is what the
// driver re-runs a closure for. Queries evaluate equality filters over one kind
// and nothing else, because that is every query the framework stores issue.
package firestoretest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"

	"github.com/shibukawa/tinygodriver/nosql/datastore"
)

// Server is a running fake. Close it with the test's cleanup.
type Server struct {
	*httptest.Server

	mu sync.Mutex
	// entities maps a canonical key to what is stored under it.
	entities map[string]stored
	// versions counts commits, which is what a stored version is.
	version int64
	// open is the handle of the transaction currently in flight, if any.
	open map[string]*transaction
	// handles counts issued transaction handles.
	handles int
	// failWith makes the next call fail with this canonical status.
	failWith string
	// counts records how many times each operation was called, which is how a
	// test asserts a round trip was saved rather than assuming it was.
	counts map[string]int
	// beforeCommit runs while the lock is held, so a test can interleave a
	// write between another caller's read and its commit.
	beforeCommit func()
}

type stored struct {
	entity  json.RawMessage
	version int64
}

// transaction is one open handle: what it read, and what it will write.
type transaction struct {
	// readVersions is the version each key held when the transaction read it,
	// which is what the commit checks for contention.
	readVersions map[string]int64
}

// New starts a fake and registers its shutdown with the test.
func New(tb interface {
	Cleanup(func())
	Helper()
}) *Server {
	tb.Helper()
	fake := &Server{
		entities: map[string]stored{},
		open:     map[string]*transaction{},
		counts:   map[string]int{},
	}
	fake.Server = httptest.NewServer(fake)
	tb.Cleanup(fake.Server.Close)
	return fake
}

// Endpoint is the address to configure a client with.
func (fake *Server) Endpoint() string { return fake.Server.URL }

// Calls reports how many times one operation was served.
func (fake *Server) Calls(op string) int {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	return fake.counts[op]
}

// FailNext makes the next operation answer with status, which must be a
// canonical code name such as UNAVAILABLE or ABORTED.
func (fake *Server) FailNext(status string) {
	fake.mu.Lock()
	fake.failWith = status
	fake.mu.Unlock()
}

// BeforeCommit runs fn inside the server lock, just before a commit is applied.
//
// It is how a test provokes the race a version precondition exists to catch:
// the hook writes the entity the caller read, so the commit arrives with a base
// version that has already moved.
func (fake *Server) BeforeCommit(fn func()) {
	fake.mu.Lock()
	fake.beforeCommit = fn
	fake.mu.Unlock()
}

// Len reports how many entities of one kind are stored, over every namespace.
func (fake *Server) Len(kind string) int {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	count := 0
	for key := range fake.entities {
		if strings.Contains(key, "/"+kind+":") {
			count++
		}
	}
	return count
}

// Namespaces reports the namespaces holding at least one entity, which is what
// an isolation test asserts on.
func (fake *Server) Namespaces() []string {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	seen := map[string]bool{}
	for key := range fake.entities {
		seen[key[:strings.Index(key, "/")]] = true
	}
	out := make([]string, 0, len(seen))
	for namespace := range seen {
		out = append(out, namespace)
	}
	return out
}

func (fake *Server) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	op := request.URL.Path[strings.LastIndex(request.URL.Path, ":")+1:]
	body := make([]byte, request.ContentLength)
	if _, err := readFull(request, body); err != nil {
		writeStatus(writer, http.StatusBadRequest, "INVALID_ARGUMENT", err.Error())
		return
	}

	fake.mu.Lock()
	fake.counts[op]++
	if status := fake.failWith; status != "" {
		fake.failWith = ""
		fake.mu.Unlock()
		writeStatus(writer, httpFor(status), status, "injected by the test")
		return
	}
	fake.mu.Unlock()

	switch op {
	case "lookup":
		fake.lookup(writer, body)
	case "commit":
		fake.commit(writer, body)
	case "beginTransaction":
		fake.begin(writer)
	case "rollback":
		fake.rollback(writer, body)
	case "runQuery":
		fake.runQuery(writer, body)
	default:
		writeStatus(writer, http.StatusNotImplemented, "UNIMPLEMENTED", op+" is not served by this fake")
	}
}

type wireReadOptions struct {
	ReadConsistency string          `json:"readConsistency"`
	Transaction     string          `json:"transaction"`
	NewTransaction  json.RawMessage `json:"newTransaction"`
	ReadTime        string          `json:"readTime"`
}

func (fake *Server) lookup(writer http.ResponseWriter, body []byte) {
	var request struct {
		ReadOptions *wireReadOptions  `json:"readOptions"`
		Keys        []json.RawMessage `json:"keys"`
	}
	if err := json.Unmarshal(body, &request); err != nil {
		writeStatus(writer, http.StatusBadRequest, "INVALID_ARGUMENT", err.Error())
		return
	}

	fake.mu.Lock()
	defer fake.mu.Unlock()

	// A read carrying newTransaction starts one and returns the handle, which
	// is what saves the caller a beginTransaction round trip.
	handle := ""
	if request.ReadOptions != nil {
		handle = request.ReadOptions.Transaction
		if request.ReadOptions.NewTransaction != nil {
			handle = fake.newHandle()
		}
	}

	var found, missing []map[string]any
	for _, raw := range request.Keys {
		canonical, err := canonicalKey(raw)
		if err != nil {
			writeStatus(writer, http.StatusBadRequest, "INVALID_ARGUMENT", err.Error())
			return
		}
		entry, held := fake.entities[canonical]
		if handle != "" {
			// Record what the transaction saw, so its commit can tell whether
			// the entity moved underneath it.
			fake.open[handle].readVersions[canonical] = entry.version
		}
		if !held {
			missing = append(missing, map[string]any{
				"entity": map[string]any{"key": json.RawMessage(raw)},
			})
			continue
		}
		found = append(found, map[string]any{
			"entity":  entry.entity,
			"version": strconv.FormatInt(entry.version, 10),
		})
	}

	writeJSON(writer, map[string]any{
		"found":       found,
		"missing":     missing,
		"transaction": handle,
	})
}

type wireMutation struct {
	Insert      json.RawMessage `json:"insert"`
	Update      json.RawMessage `json:"update"`
	Upsert      json.RawMessage `json:"upsert"`
	Delete      json.RawMessage `json:"delete"`
	BaseVersion string          `json:"baseVersion"`
	UpdateTime  string          `json:"updateTime"`
}

func (fake *Server) commit(writer http.ResponseWriter, body []byte) {
	var request struct {
		Mode                 string          `json:"mode"`
		Mutations            []wireMutation  `json:"mutations"`
		Transaction          string          `json:"transaction"`
		SingleUseTransaction json.RawMessage `json:"singleUseTransaction"`
	}
	if err := json.Unmarshal(body, &request); err != nil {
		writeStatus(writer, http.StatusBadRequest, "INVALID_ARGUMENT", err.Error())
		return
	}

	fake.mu.Lock()
	defer fake.mu.Unlock()
	if fake.beforeCommit != nil {
		hook := fake.beforeCommit
		fake.beforeCommit = nil
		// The hook writes through the same map under the same lock, which is
		// what makes the race deterministic instead of timing-dependent.
		fake.mu.Unlock()
		hook()
		fake.mu.Lock()
	}

	tx, transactional := fake.open[request.Transaction]
	if request.Transaction != "" && !transactional {
		writeStatus(writer, http.StatusBadRequest, "INVALID_ARGUMENT", "unknown transaction handle")
		return
	}
	if transactional {
		// A transactional commit fails wholesale when anything it read has
		// moved. That is the contention the driver answers by re-running the
		// closure.
		for canonical, version := range tx.readVersions {
			if fake.entities[canonical].version != version {
				delete(fake.open, request.Transaction)
				writeStatus(writer, http.StatusConflict, "ABORTED", "contention on "+canonical)
				return
			}
		}
	}

	// Every mutation is validated before any is applied, so a refused commit
	// leaves nothing behind — which is what makes the compound writes of the
	// auth stores atomic.
	changes := make([]change, 0, len(request.Mutations))
	applied := map[string]bool{}
	for _, mutation := range request.Mutations {
		raw, remove, verb := mutationBody(mutation)
		if raw == nil {
			writeStatus(writer, http.StatusBadRequest, "INVALID_ARGUMENT", "mutation names no verb")
			return
		}
		canonical, err := canonicalKeyOf(raw, remove)
		if err != nil {
			writeStatus(writer, http.StatusBadRequest, "INVALID_ARGUMENT", err.Error())
			return
		}
		_, held := fake.entities[canonical]
		if applied[canonical] {
			held = !containsRemoval(changes, canonical)
		}
		switch verb {
		case "insert":
			if held {
				writeStatus(writer, http.StatusConflict, "ALREADY_EXISTS", canonical+" exists")
				return
			}
		case "update":
			if !held {
				writeStatus(writer, http.StatusNotFound, "NOT_FOUND", canonical+" does not exist")
				return
			}
		}
		if mutation.BaseVersion != "" {
			want, _ := strconv.ParseInt(mutation.BaseVersion, 10, 64)
			if fake.entities[canonical].version != want {
				writeStatus(writer, http.StatusBadRequest, "FAILED_PRECONDITION",
					fmt.Sprintf("%s is at version %d, not %s",
						canonical, fake.entities[canonical].version, mutation.BaseVersion))
				return
			}
		}
		applied[canonical] = true
		changes = append(changes, change{canonical: canonical, entity: raw, remove: remove})
	}

	fake.version++
	results := make([]map[string]any, 0, len(changes))
	for _, applied := range changes {
		if applied.remove {
			delete(fake.entities, applied.canonical)
			results = append(results, map[string]any{"version": strconv.FormatInt(fake.version, 10)})
			continue
		}
		fake.entities[applied.canonical] = stored{entity: applied.entity, version: fake.version}
		results = append(results, map[string]any{"version": strconv.FormatInt(fake.version, 10)})
	}
	delete(fake.open, request.Transaction)

	writeJSON(writer, map[string]any{
		"mutationResults": results,
		"indexUpdates":    len(changes),
	})
}

// change is one mutation resolved against the store, held back until every
// mutation in the commit has been validated.
type change struct {
	canonical string
	entity    json.RawMessage
	remove    bool
}

// containsRemoval reports whether an earlier mutation in the same commit
// deleted this key, so a later insert of it is legal.
func containsRemoval(changes []change, canonical string) bool {
	for _, change := range changes {
		if change.canonical == canonical && change.remove {
			return true
		}
	}
	return false
}

func mutationBody(mutation wireMutation) (raw json.RawMessage, remove bool, verb string) {
	switch {
	case mutation.Insert != nil:
		return mutation.Insert, false, "insert"
	case mutation.Update != nil:
		return mutation.Update, false, "update"
	case mutation.Upsert != nil:
		return mutation.Upsert, false, "upsert"
	case mutation.Delete != nil:
		return mutation.Delete, true, "delete"
	}
	return nil, false, ""
}

func (fake *Server) begin(writer http.ResponseWriter) {
	fake.mu.Lock()
	handle := fake.newHandle()
	fake.mu.Unlock()
	writeJSON(writer, map[string]any{"transaction": handle})
}

// newHandle issues a transaction. The caller holds the lock.
func (fake *Server) newHandle() string {
	fake.handles++
	handle := "tx-" + strconv.Itoa(fake.handles)
	fake.open[handle] = &transaction{readVersions: map[string]int64{}}
	return handle
}

func (fake *Server) rollback(writer http.ResponseWriter, body []byte) {
	var request struct {
		Transaction string `json:"transaction"`
	}
	_ = json.Unmarshal(body, &request)
	fake.mu.Lock()
	delete(fake.open, request.Transaction)
	fake.mu.Unlock()
	writeJSON(writer, map[string]any{})
}

// runQuery evaluates a kind with equality filters, keys-only, and a limit.
//
// That is every query the framework stores issue. Anything else is refused
// loudly rather than answered approximately, because a fake that silently
// ignored a filter would let a wrong query pass a test.
func (fake *Server) runQuery(writer http.ResponseWriter, body []byte) {
	var request struct {
		PartitionID struct {
			NamespaceID string `json:"namespaceId"`
		} `json:"partitionId"`
		Query struct {
			Kind []struct {
				Name string `json:"name"`
			} `json:"kind"`
			Projection []struct {
				Property struct {
					Name string `json:"name"`
				} `json:"property"`
			} `json:"projection"`
			Filter json.RawMessage `json:"filter"`
			Limit  *int32          `json:"limit"`
		} `json:"query"`
	}
	if err := json.Unmarshal(body, &request); err != nil {
		writeStatus(writer, http.StatusBadRequest, "INVALID_ARGUMENT", err.Error())
		return
	}
	if len(request.Query.Kind) != 1 {
		writeStatus(writer, http.StatusBadRequest, "INVALID_ARGUMENT", "this fake serves one kind per query")
		return
	}
	filters, err := equalityFilters(request.Query.Filter)
	if err != nil {
		writeStatus(writer, http.StatusBadRequest, "INVALID_ARGUMENT", err.Error())
		return
	}

	kind := request.Query.Kind[0].Name
	prefix := request.PartitionID.NamespaceID + "/" + kind + ":"

	fake.mu.Lock()
	defer fake.mu.Unlock()
	var results []map[string]any
	for canonical, entry := range fake.entities {
		if !strings.HasPrefix(canonical, prefix) {
			continue
		}
		var entity datastore.Entity
		if err := json.Unmarshal(entry.entity, &entity); err != nil {
			continue
		}
		if !matches(entity, filters) {
			continue
		}
		results = append(results, map[string]any{
			"entity":  entry.entity,
			"version": strconv.FormatInt(entry.version, 10),
		})
		if request.Query.Limit != nil && len(results) >= int(*request.Query.Limit) {
			break
		}
	}

	writeJSON(writer, map[string]any{
		"batch": map[string]any{
			"entityResults": results,
			"moreResults":   "NO_MORE_RESULTS",
			"endCursor":     "",
		},
	})
}

// equalityFilters flattens the filter tree into property equality pairs, which
// is all this fake evaluates.
func equalityFilters(raw json.RawMessage) (map[string]datastore.Value, error) {
	out := map[string]datastore.Value{}
	if len(raw) == 0 {
		return out, nil
	}
	var filter struct {
		CompositeFilter *struct {
			Op      string            `json:"op"`
			Filters []json.RawMessage `json:"filters"`
		} `json:"compositeFilter"`
		PropertyFilter *struct {
			Property struct {
				Name string `json:"name"`
			} `json:"property"`
			Op    string          `json:"op"`
			Value datastore.Value `json:"value"`
		} `json:"propertyFilter"`
	}
	if err := json.Unmarshal(raw, &filter); err != nil {
		return nil, err
	}
	switch {
	case filter.CompositeFilter != nil:
		if filter.CompositeFilter.Op != "AND" {
			return nil, fmt.Errorf("this fake composes filters with AND only, got %s", filter.CompositeFilter.Op)
		}
		for _, nested := range filter.CompositeFilter.Filters {
			inner, err := equalityFilters(nested)
			if err != nil {
				return nil, err
			}
			for name, value := range inner {
				out[name] = value
			}
		}
	case filter.PropertyFilter != nil:
		if filter.PropertyFilter.Op != "EQUAL" {
			return nil, fmt.Errorf("this fake evaluates EQUAL only, got %s", filter.PropertyFilter.Op)
		}
		out[filter.PropertyFilter.Property.Name] = filter.PropertyFilter.Value
	}
	return out, nil
}

func matches(entity datastore.Entity, filters map[string]datastore.Value) bool {
	for name, want := range filters {
		got, held := entity.Get(name)
		if !held {
			return false
		}
		left, _ := json.Marshal(got)
		right, _ := json.Marshal(want)
		if string(left) != string(right) {
			return false
		}
	}
	return true
}

// canonicalKey renders a key as "namespace/kind:name", which is what the map is
// keyed by. Ancestors join with a dot; nothing here uses one.
func canonicalKey(raw json.RawMessage) (string, error) {
	var key struct {
		PartitionID struct {
			NamespaceID string `json:"namespaceId"`
		} `json:"partitionId"`
		Path []struct {
			Kind string `json:"kind"`
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"path"`
	}
	if err := json.Unmarshal(raw, &key); err != nil {
		return "", err
	}
	if len(key.Path) == 0 {
		return "", fmt.Errorf("key has no path")
	}
	parts := make([]string, 0, len(key.Path))
	for _, element := range key.Path {
		identifier := element.Name
		if identifier == "" {
			identifier = element.ID
		}
		parts = append(parts, element.Kind+":"+identifier)
	}
	return key.PartitionID.NamespaceID + "/" + strings.Join(parts, "."), nil
}

// canonicalKeyOf reads the key out of a mutation body, which is a bare key for
// a delete and an entity for everything else.
func canonicalKeyOf(raw json.RawMessage, isKey bool) (string, error) {
	if isKey {
		return canonicalKey(raw)
	}
	var entity struct {
		Key json.RawMessage `json:"key"`
	}
	if err := json.Unmarshal(raw, &entity); err != nil {
		return "", err
	}
	if entity.Key == nil {
		return "", fmt.Errorf("entity carries no key")
	}
	return canonicalKey(entity.Key)
}

func readFull(request *http.Request, into []byte) (int, error) {
	read := 0
	for read < len(into) {
		n, err := request.Body.Read(into[read:])
		read += n
		if err != nil {
			if read == len(into) {
				return read, nil
			}
			return read, err
		}
	}
	return read, nil
}

func writeJSON(writer http.ResponseWriter, body map[string]any) {
	writer.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(writer).Encode(body)
}

// writeStatus answers with the error envelope the driver parses. The canonical
// status is what it discriminates on, since the HTTP code is ambiguous:
// ABORTED and ALREADY_EXISTS are both 409 and mean opposite things.
func writeStatus(writer http.ResponseWriter, httpStatus int, status, message string) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(httpStatus)
	_ = json.NewEncoder(writer).Encode(map[string]any{
		"error": map[string]any{
			"code":    httpStatus,
			"status":  status,
			"message": message,
		},
	})
}

func httpFor(status string) int {
	switch status {
	case "ABORTED", "ALREADY_EXISTS":
		return http.StatusConflict
	case "NOT_FOUND":
		return http.StatusNotFound
	case "FAILED_PRECONDITION", "INVALID_ARGUMENT":
		return http.StatusBadRequest
	case "UNAUTHENTICATED":
		return http.StatusUnauthorized
	case "PERMISSION_DENIED":
		return http.StatusForbidden
	case "RESOURCE_EXHAUSTED":
		return http.StatusTooManyRequests
	case "UNAVAILABLE":
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}
