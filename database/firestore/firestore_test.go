package firestore

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/shibukawa/tinybind-go/firestorebind"
	"github.com/shibukawa/tinygodriver/cloud/google"
	"github.com/shibukawa/tinygodriver/nosql/datastore"
)

func enabled(edit func(*Config)) Config {
	config := DefaultConfig()
	config.Enabled = true
	config.ProjectID = "demo"
	if edit != nil {
		edit(&config)
	}
	return config
}

func TestValidateAcceptsTheDefaults(t *testing.T) {
	if err := enabled(nil).validate(); err != nil {
		t.Fatal(err)
	}
}

// A disabled section is not validated: a project that never turned Firestore on
// should not have to keep its keys coherent.
func TestValidateSkipsADisabledSection(t *testing.T) {
	config := DefaultConfig()
	config.Timeout = -time.Second
	if err := config.validate(); err != nil {
		t.Fatalf("a disabled section was validated: %v", err)
	}
}

func TestValidateRejectsAnUnknownCredentialSource(t *testing.T) {
	err := enabled(func(c *Config) { c.Credentials = "workload_identity" }).validate()
	if err == nil {
		t.Fatal("an unknown credential source was accepted")
	}
	// The message has to list what is accepted; the value alone leaves a reader
	// guessing which four words are legal.
	for _, want := range []string{CredentialsServiceAccount, CredentialsMetadata, CredentialsOAuth2, CredentialsStatic} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("%v does not name %q", err, want)
		}
	}
}

// A key file named for a source that opens none is a deployment that believes
// it configured something.
func TestValidateRejectsAKeyFileASourceWillNotRead(t *testing.T) {
	for _, source := range []string{CredentialsMetadata, CredentialsStatic} {
		err := enabled(func(c *Config) {
			c.Credentials = source
			c.CredentialsFile = "/etc/key.json"
		}).validate()
		if err == nil {
			t.Errorf("%s: a key file was accepted for a source that reads none", source)
		}
	}
	for _, source := range []string{CredentialsServiceAccount, CredentialsOAuth2} {
		err := enabled(func(c *Config) {
			c.Credentials = source
			c.CredentialsFile = "/etc/key.json"
		}).validate()
		if err != nil {
			t.Errorf("%s: %v", source, err)
		}
	}
}

func TestValidateRejectsAnImpossibleTimeoutOrPool(t *testing.T) {
	if err := enabled(func(c *Config) { c.Timeout = 0 }).validate(); err == nil {
		t.Error("a zero timeout was accepted")
	}
	if err := enabled(func(c *Config) { c.MaxIdleConns = -1 }).validate(); err == nil {
		t.Error("a negative pool size was accepted")
	}
}

// There are no schema keys here, because nothing creates a kind and nothing
// reports one. A key promising either would configure nothing.
func TestTheBindingCarriesNoSchemaKeys(t *testing.T) {
	for _, absent := range []string{"verify_schema", "auto_migrate", "table_prefix", "table_names"} {
		if strings.Contains(configKeys(), absent) {
			t.Errorf("middleware.firestore declares %q, which configures nothing here", absent)
		}
	}
}

// A static credential with no token installed is a startup error rather than an
// anonymous client, which would fail later and further away.
func TestAStaticCredentialNeedsATokenSource(t *testing.T) {
	SetTokenSource(nil)
	_, err := resolveTokenSource(enabled(func(c *Config) { c.Credentials = CredentialsStatic }))
	if err == nil {
		t.Fatal("a static credential with no token source was accepted")
	}
	if !strings.Contains(err.Error(), "SetTokenSource") {
		t.Errorf("the error does not name the fix: %v", err)
	}

	SetTokenSource(google.StaticTokenSource(google.Token{Value: "supplied"}))
	t.Cleanup(func() { SetTokenSource(nil) })
	if _, err := resolveTokenSource(enabled(func(c *Config) { c.Credentials = CredentialsStatic })); err != nil {
		t.Fatalf("an installed token source was refused: %v", err)
	}
}

// An emulator endpoint resolves no real credential: the emulator ignores the
// header, so minting a token would be pretending to exercise the token path.
func TestAnEmulatorEndpointNeedsNoKeyFile(t *testing.T) {
	config := enabled(func(c *Config) { c.Endpoint = "127.0.0.1:8081" })
	if !usingEmulator(config) {
		t.Fatal("a plain-http endpoint was not recognised as an emulator")
	}
	if _, err := resolveTokenSource(config); err != nil {
		t.Fatalf("an emulator endpoint demanded a credential: %v", err)
	}
	if usingEmulator(enabled(func(c *Config) { c.Endpoint = "https://datastore.googleapis.com" })) {
		t.Error("the service endpoint was taken for an emulator")
	}
}

func TestOpenRefusesAProjectItCannotResolve(t *testing.T) {
	t.Setenv("GOOGLE_CLOUD_PROJECT", "")
	t.Setenv("DATASTORE_PROJECT_ID", "")
	config := enabled(func(c *Config) {
		c.ProjectID = ""
		c.Endpoint = "127.0.0.1:8081"
	})
	_, _, err := open(config)
	if err == nil {
		t.Fatal("a client opened with no project")
	}
	if !strings.Contains(err.Error(), "GOOGLE_CLOUD_PROJECT") {
		t.Errorf("the error does not name the fallback: %v", err)
	}
}

// probeServer answers every request with one canonical status.
func probeServer(t *testing.T, status string, httpStatus int) *datastore.Client {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		if status == "" {
			_, _ = writer.Write([]byte(`{"found":[],"missing":[]}`))
			return
		}
		writer.WriteHeader(httpStatus)
		_, _ = writer.Write([]byte(`{"error":{"code":` + strconv.Itoa(httpStatus) +
			`,"status":"` + status + `","message":"from the test"}}`))
	}))
	t.Cleanup(server.Close)

	client, err := datastore.New("demo",
		datastore.WithEndpoint(server.URL),
		datastore.WithTokenSource(google.StaticTokenSource(google.Token{Value: "test"})),
		datastore.WithRetry(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}

// The probe passes on a miss. Finding the entity is not the point: the round
// trip proves the project, the database, the credential and the mode.
func TestTheProbePassesOnAMiss(t *testing.T) {
	client := probeServer(t, "", 0)
	if err := probe(context.Background(), client, enabled(nil)); err != nil {
		t.Fatalf("a clean miss failed the probe: %v", err)
	}
}

// A native-mode database answers with the same status a missing composite index
// produces. The probe carries no filter and no order, so an index cannot be
// what the service is objecting to — which is what lets this name the mode.
func TestTheProbeNamesTheModeOnAPreconditionFailure(t *testing.T) {
	client := probeServer(t, "FAILED_PRECONDITION", http.StatusBadRequest)
	err := probe(context.Background(), client, enabled(func(c *Config) { c.Database = "app" }))
	if err == nil {
		t.Fatal("a precondition failure passed the probe")
	}
	for _, want := range []string{"Datastore mode", "cannot be changed", `"app"`} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("%v does not say %q", err, want)
		}
	}
	if strings.Contains(err.Error(), "index") {
		t.Errorf("the message blames an index: %v", err)
	}
}

// UNAUTHENTICATED points at the credential, and in the field the credential is
// usually fine: a token is signed against this host's clock.
func TestTheProbeNamesTheClockOnAnAuthenticationFailure(t *testing.T) {
	client := probeServer(t, "UNAUTHENTICATED", http.StatusUnauthorized)
	err := probe(context.Background(), client, enabled(nil))
	if err == nil {
		t.Fatal("an authentication failure passed the probe")
	}
	if !strings.Contains(err.Error(), "clock") {
		t.Errorf("the error does not mention the clock: %v", err)
	}
	if !errors.Is(err, datastore.ErrUnauthenticated) {
		t.Errorf("the driver sentinel is unreachable: %v", err)
	}
}

func TestTheProbeReportsAPermissionFailureWithItsTarget(t *testing.T) {
	client := probeServer(t, "PERMISSION_DENIED", http.StatusForbidden)
	err := probe(context.Background(), client, enabled(nil))
	if err == nil {
		t.Fatal("a permission failure passed the probe")
	}
	if !strings.Contains(err.Error(), `project "demo"`) {
		t.Errorf("the error does not name the target: %v", err)
	}
}

// --- kinds ---------------------------------------------------------------

type testKind struct {
	kind   string
	expiry string
}

func (k testKind) Kind() string { return k.kind }

type expiringKind struct{ testKind }

func (k expiringKind) ExpiryProperty() (string, bool) { return k.expiry, k.expiry != "" }

func TestKindsReportsWhatIsLinkedSorted(t *testing.T) {
	registry.Lock()
	registry.kinds = nil
	registry.Unlock()

	RegisterKind(expiringKind{testKind{kind: "zeta", expiry: "dead_at"}})
	RegisterKind(testKind{kind: "alpha"})

	got := Kinds()
	if len(got) != 2 {
		t.Fatalf("got %d kinds, want 2", len(got))
	}
	if got[0].Kind != "alpha" || got[1].Kind != "zeta" {
		t.Errorf("kinds are not sorted: %v", got)
	}
	// A kind that does not expire must not claim a property: a TTL policy
	// pointed at one nothing maintains deletes nothing.
	if got[0].ExpiryProperty != "" {
		t.Errorf("a non-expiring kind reported %q", got[0].ExpiryProperty)
	}
	if got[1].ExpiryProperty != "dead_at" {
		t.Errorf("expiry property: got %q", got[1].ExpiryProperty)
	}
}

// Re-initialization in a test re-runs the same registrations, so registering
// twice keeps the first rather than growing the list.
func TestRegisteringAKindTwiceIsIdempotent(t *testing.T) {
	registry.Lock()
	registry.kinds = nil
	registry.Unlock()

	RegisterKind(testKind{kind: "alpha"})
	RegisterKind(testKind{kind: "alpha"})
	if got := Kinds(); len(got) != 1 {
		t.Fatalf("got %d entries, want 1", len(got))
	}
}

// --- client plumbing -----------------------------------------------------

func TestEnsureClientReportsWhenNothingIsOpen(t *testing.T) {
	state.Lock()
	state.client = nil
	state.Unlock()
	if _, opened := EnsureClient(context.Background()); opened {
		t.Fatal("a client was reported with none open")
	}
}

// A setup context is not a request context, so a store opening at startup finds
// the process client rather than one the middleware has not installed yet.
func TestEnsureClientFindsTheProcessClient(t *testing.T) {
	client, err := datastore.New("demo",
		datastore.WithEndpoint("127.0.0.1:8081"),
		datastore.WithTokenSource(google.StaticTokenSource(google.Token{Value: "test"})))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })

	state.Lock()
	state.client = client
	state.Unlock()
	t.Cleanup(func() {
		state.Lock()
		state.client = nil
		state.Unlock()
	})

	ctx, opened := EnsureClient(context.Background())
	if !opened {
		t.Fatal("the process client was not found")
	}
	if _, err := firestorebind.ClientFromContext(ctx); err != nil {
		t.Errorf("the returned context carries no client: %v", err)
	}
}

// configKeys reads the keys the struct actually declares, so a key added later
// is seen by the test rather than by a list beside it.
func configKeys() string {
	fields := reflect.TypeFor[Config]()
	keys := make([]string, 0, fields.NumField())
	for index := range fields.NumField() {
		keys = append(keys, fields.Field(index).Tag.Get("toml"))
	}
	return strings.Join(keys, " ")
}
