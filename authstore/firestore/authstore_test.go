package firestore

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/shibukawa/popcornweb/internal/firestoretest"
	"github.com/shibukawa/popcornweb/plugin/auth"
	"github.com/shibukawa/tinybind-go/firestorebind"
	"github.com/shibukawa/tinygodriver/cloud/google"
	"github.com/shibukawa/tinygodriver/nosql/datastore"
)

func newContext(t *testing.T) (context.Context, *firestoretest.Server) {
	t.Helper()
	fake := firestoretest.New(t)
	client, err := datastore.New("demo",
		datastore.WithEndpoint(fake.Endpoint()),
		datastore.WithTokenSource(google.StaticTokenSource(google.Token{Value: "test"})),
		datastore.WithRetry(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return firestorebind.WithClient(context.Background(), client), fake
}

func credential(id string, account string) auth.Credential {
	return auth.Credential{
		CredentialID: []byte(id),
		AccountID:    account,
		UserHandle:   []byte("handle-" + account),
		PublicKey:    []byte("cose"),
		PublicKeyX:   []byte("x"),
		PublicKeyY:   []byte("y"),
		Algorithm:    -7,
		SignCount:    1,
		Transports:   []string{"internal", "hybrid"},
		Label:        "laptop",
		CreatedAt:    time.Now().UTC().Truncate(time.Microsecond),
	}
}

func bootstrap(loginID, account string, attempts int) auth.BootstrapCredential {
	now := time.Now().UTC().Truncate(time.Microsecond)
	return auth.BootstrapCredential{
		LoginID:           loginID,
		AccountID:         account,
		SecretDigest:      []byte("digest"),
		Purpose:           auth.PurposeInitialPasskey,
		IssuedAt:          now,
		ExpiresAt:         now.Add(time.Hour),
		AttemptsRemaining: attempts,
	}
}

// --- credentials ---------------------------------------------------------

func TestCredentialRoundTrip(t *testing.T) {
	ctx, _ := newContext(t)
	store := NewCredentials(CredentialOptions{})
	want := credential("cred-1", "account-1")

	if err := store.Save(ctx, want, nil); err != nil {
		t.Fatal(err)
	}
	got, err := store.Find(ctx, want.CredentialID)
	if err != nil {
		t.Fatal(err)
	}
	if string(got.CredentialID) != "cred-1" || got.AccountID != "account-1" {
		t.Errorf("identity: got %q, %q", got.CredentialID, got.AccountID)
	}
	if string(got.PublicKey) != "cose" || got.Algorithm != -7 || got.SignCount != 1 {
		t.Errorf("key material: got %q, %d, %d", got.PublicKey, got.Algorithm, got.SignCount)
	}
	if len(got.Transports) != 2 || got.Transports[0] != "internal" {
		t.Errorf("transports: got %v", got.Transports)
	}
	if string(got.UserHandle) != "handle-account-1" {
		t.Errorf("user handle: got %q", got.UserHandle)
	}
}

func TestFindReportsAnUnknownCredential(t *testing.T) {
	ctx, _ := newContext(t)
	store := NewCredentials(CredentialOptions{})
	if _, err := store.Find(ctx, []byte("absent")); !errors.Is(err, auth.ErrUnknownCredential) {
		t.Fatalf("got %v, want ErrUnknownCredential", err)
	}
}

func TestSaveRefusesASecondCredentialWithTheSameID(t *testing.T) {
	ctx, _ := newContext(t)
	store := NewCredentials(CredentialOptions{})
	if err := store.Save(ctx, credential("cred-1", "account-1"), nil); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(ctx, credential("cred-1", "account-2"), nil); err == nil {
		t.Fatal("a duplicate credential ID was accepted")
	}
	got, err := store.Find(ctx, []byte("cred-1"))
	if err != nil {
		t.Fatal(err)
	}
	if got.AccountID != "account-1" {
		t.Errorf("the refused write overwrote the credential: %q", got.AccountID)
	}
}

// The listing is a single-property equality filter, which the automatic indexes
// already cover. Nothing declares an index and nothing has to apply one.
func TestListByAccountReturnsOnlyThatAccount(t *testing.T) {
	ctx, _ := newContext(t)
	store := NewCredentials(CredentialOptions{})
	for _, entry := range []struct{ id, account string }{
		{"cred-1", "account-1"}, {"cred-2", "account-1"}, {"cred-3", "account-2"},
	} {
		if err := store.Save(ctx, credential(entry.id, entry.account), nil); err != nil {
			t.Fatal(err)
		}
	}
	got, err := store.ListByAccount(ctx, "account-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d credentials, want 2", len(got))
	}
	for _, credential := range got {
		if credential.AccountID != "account-1" {
			t.Errorf("listing leaked %q", credential.AccountID)
		}
		// The handle has to come back: an enrollment reuses the one the account
		// already has, and a listing that could not see it would mint a second.
		if len(credential.UserHandle) == 0 {
			t.Error("the listing dropped the user handle")
		}
	}
}

// A counter that does not move forward is a replayed or cloned authenticator.
// The store refuses it rather than leaving the caller to notice.
func TestUpdateOnAssertionRefusesACounterThatDidNotAdvance(t *testing.T) {
	ctx, _ := newContext(t)
	store := NewCredentials(CredentialOptions{})
	if err := store.Save(ctx, credential("cred-1", "account-1"), nil); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err := store.UpdateOnAssertion(ctx, []byte("cred-1"), 5, true, now); err != nil {
		t.Fatal(err)
	}
	for _, count := range []uint32{5, 4} {
		err := store.UpdateOnAssertion(ctx, []byte("cred-1"), count, true, now)
		if !errors.Is(err, auth.ErrUnknownCredential) {
			t.Errorf("count %d: got %v, want ErrUnknownCredential", count, err)
		}
	}
	got, err := store.Find(ctx, []byte("cred-1"))
	if err != nil {
		t.Fatal(err)
	}
	if got.SignCount != 5 {
		t.Errorf("the refused assertion moved the counter to %d", got.SignCount)
	}
}

// An authenticator that keeps no counter sends zero every time, which is not a
// replay.
func TestUpdateOnAssertionAcceptsAZeroCounter(t *testing.T) {
	ctx, _ := newContext(t)
	store := NewCredentials(CredentialOptions{})
	fresh := credential("cred-1", "account-1")
	fresh.SignCount = 0
	if err := store.Save(ctx, fresh, nil); err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateOnAssertion(ctx, []byte("cred-1"), 0, false, time.Now()); err != nil {
		t.Fatalf("a zero counter was refused: %v", err)
	}
}

func TestDeleteRefusesAnotherAccountsCredential(t *testing.T) {
	ctx, _ := newContext(t)
	store := NewCredentials(CredentialOptions{})
	if err := store.Save(ctx, credential("cred-1", "account-1"), nil); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete(ctx, "account-2", []byte("cred-1")); !errors.Is(err, auth.ErrUnknownCredential) {
		t.Fatalf("got %v, want ErrUnknownCredential", err)
	}
	if _, err := store.Find(ctx, []byte("cred-1")); err != nil {
		t.Errorf("the refused delete removed the credential: %v", err)
	}
	if err := store.Delete(ctx, "account-1", []byte("cred-1")); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Find(ctx, []byte("cred-1")); !errors.Is(err, auth.ErrUnknownCredential) {
		t.Errorf("got %v, want the credential to be gone", err)
	}
}

func TestSaveRefusesAnOverlongLabel(t *testing.T) {
	ctx, fake := newContext(t)
	store := NewCredentials(CredentialOptions{})
	oversized := credential("cred-1", "account-1")
	oversized.Label = string(make([]byte, maxLabelBytes+1))
	if err := store.Save(ctx, oversized, nil); err == nil {
		t.Fatal("an unbounded label was accepted")
	}
	if fake.Calls("commit") != 0 {
		t.Error("the oversized credential was sent")
	}
}

// --- bootstrap credentials ----------------------------------------------

func TestBootstrapIssueAndFind(t *testing.T) {
	ctx, _ := newContext(t)
	store := NewBootstrap()
	want := bootstrap("login-1", "account-1", 3)
	if err := store.Issue(ctx, want); err != nil {
		t.Fatal(err)
	}
	got, err := store.Find(ctx, "login-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.AccountID != "account-1" || got.AttemptsRemaining != 3 {
		t.Errorf("got %q, %d", got.AccountID, got.AttemptsRemaining)
	}
	if string(got.SecretDigest) != "digest" || got.Purpose != auth.PurposeInitialPasskey {
		t.Errorf("digest/purpose: got %q, %q", got.SecretDigest, got.Purpose)
	}
	if !got.ExpiresAt.Equal(want.ExpiresAt) {
		t.Errorf("expires_at: got %s, want %s", got.ExpiresAt, want.ExpiresAt)
	}
}

func TestBootstrapIssueRefusesALiveLoginID(t *testing.T) {
	ctx, _ := newContext(t)
	store := NewBootstrap()
	if err := store.Issue(ctx, bootstrap("login-1", "account-1", 3)); err != nil {
		t.Fatal(err)
	}
	if err := store.Issue(ctx, bootstrap("login-1", "account-2", 9)); err == nil {
		t.Fatal("a live login ID was overwritten")
	}
	got, err := store.Find(ctx, "login-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.AccountID != "account-1" || got.AttemptsRemaining != 3 {
		t.Errorf("the refused issue changed the record: %q, %d", got.AccountID, got.AttemptsRemaining)
	}
}

// An unknown login ID, a consumed credential, and an exhausted budget are one
// error, so a caller cannot enumerate accounts.
func TestBootstrapFindHidesAConsumedCredential(t *testing.T) {
	ctx, _ := newContext(t)
	store := NewBootstrap()
	if err := store.Issue(ctx, bootstrap("login-1", "account-1", 3)); err != nil {
		t.Fatal(err)
	}
	if err := store.Consume(ctx, "login-1", time.Now()); err != nil {
		t.Fatal(err)
	}
	unknown, err := store.Find(ctx, "absent")
	if !errors.Is(err, auth.ErrUnknownBootstrap) {
		t.Fatalf("absent: got %v, %v", unknown, err)
	}
	if _, err := store.Find(ctx, "login-1"); !errors.Is(err, auth.ErrUnknownBootstrap) {
		t.Fatalf("consumed: got %v, want the same error as absent", err)
	}
}

// The contract's sharpest promise: N parallel guesses against a budget of one
// leave exactly one caller with an attempt.
func TestParallelAttemptsCannotSpendTheSameBudgetTwice(t *testing.T) {
	ctx, _ := newContext(t)
	store := NewBootstrap()
	if err := store.Issue(ctx, bootstrap("login-1", "account-1", 1)); err != nil {
		t.Fatal(err)
	}

	const guesses = 8
	var group sync.WaitGroup
	failures := make([]error, guesses)
	start := make(chan struct{})
	for index := range guesses {
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			_, failures[index] = store.RecordAttempt(ctx, "login-1")
		}()
	}
	close(start)
	group.Wait()

	spent := 0
	for index, err := range failures {
		switch {
		case err == nil:
			spent++
		case errors.Is(err, auth.ErrUnknownBootstrap):
		default:
			t.Errorf("guess %d: unexpected %v", index, err)
		}
	}
	if spent != 1 {
		t.Fatalf("%d of %d guesses spent the single attempt; exactly one must", spent, guesses)
	}
}

func TestRecordAttemptCountsDownAndThenRefuses(t *testing.T) {
	ctx, _ := newContext(t)
	store := NewBootstrap()
	if err := store.Issue(ctx, bootstrap("login-1", "account-1", 2)); err != nil {
		t.Fatal(err)
	}
	for want := 1; want >= 0; want-- {
		remaining, err := store.RecordAttempt(ctx, "login-1")
		if err != nil {
			t.Fatal(err)
		}
		if remaining != want {
			t.Errorf("remaining: got %d, want %d", remaining, want)
		}
	}
	if _, err := store.RecordAttempt(ctx, "login-1"); !errors.Is(err, auth.ErrUnknownBootstrap) {
		t.Fatalf("exhausted: got %v, want ErrUnknownBootstrap", err)
	}
}

func TestConsumeIsSingleUse(t *testing.T) {
	ctx, _ := newContext(t)
	store := NewBootstrap()
	if err := store.Issue(ctx, bootstrap("login-1", "account-1", 3)); err != nil {
		t.Fatal(err)
	}
	if err := store.Consume(ctx, "login-1", time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := store.Consume(ctx, "login-1", time.Now()); !errors.Is(err, auth.ErrUnknownBootstrap) {
		t.Fatalf("second consume: got %v, want ErrUnknownBootstrap", err)
	}
}

// --- first enrollment ----------------------------------------------------

// The pair is one commit: the credential cannot be stored without the secret
// that authorized it being spent, and the spend cannot happen without the
// credential.
func TestFirstEnrollmentSpendsAndStoresInOneCommit(t *testing.T) {
	ctx, fake := newContext(t)
	credentials := NewCredentials(CredentialOptions{})
	bootstraps := NewBootstrap()
	if err := bootstraps.Issue(ctx, bootstrap("login-1", "account-1", 3)); err != nil {
		t.Fatal(err)
	}

	commits := fake.Calls("commit")
	activated := 0
	spend := func(ctx context.Context) error { return bootstraps.Consume(ctx, "login-1", time.Now()) }
	activate := func(context.Context) error { activated++; return nil }

	if err := credentials.SaveFirstCredential(ctx, credential("cred-1", "account-1"), spend, activate); err != nil {
		t.Fatal(err)
	}
	if got := fake.Calls("commit") - commits; got != 1 {
		t.Errorf("commits: got %d, want 1; the spend and the insert are one commit", got)
	}
	if activated != 1 {
		t.Errorf("activate ran %d times", activated)
	}
	if _, err := credentials.Find(ctx, []byte("cred-1")); err != nil {
		t.Errorf("the credential was not stored: %v", err)
	}
	if _, err := bootstraps.Find(ctx, "login-1"); !errors.Is(err, auth.ErrUnknownBootstrap) {
		t.Errorf("the bootstrap credential was not spent: %v", err)
	}
}

// A failed spend writes nothing at all — not the credential, and not the spend.
// This is the state the DynamoDB backend cannot avoid and this one can.
func TestAFailedSpendLeavesNeitherWrite(t *testing.T) {
	ctx, _ := newContext(t)
	credentials := NewCredentials(CredentialOptions{})
	bootstraps := NewBootstrap()
	if err := bootstraps.Issue(ctx, bootstrap("login-1", "account-1", 3)); err != nil {
		t.Fatal(err)
	}
	// Already redeemed, so the spend inside the transaction fails.
	if err := bootstraps.Consume(ctx, "login-1", time.Now()); err != nil {
		t.Fatal(err)
	}

	activated := 0
	err := credentials.SaveFirstCredential(ctx, credential("cred-1", "account-1"),
		func(ctx context.Context) error { return bootstraps.Consume(ctx, "login-1", time.Now()) },
		func(context.Context) error { activated++; return nil })
	if !errors.Is(err, auth.ErrUnknownBootstrap) {
		t.Fatalf("got %v, want ErrUnknownBootstrap", err)
	}
	if activated != 0 {
		t.Error("the account was activated by a failed enrollment")
	}
	if _, err := credentials.Find(ctx, []byte("cred-1")); !errors.Is(err, auth.ErrUnknownCredential) {
		t.Error("a credential was stored without a spend")
	}
}

// Retrying the whole enrollment with the same secret is refused at the spend,
// so one issued secret cannot enroll two authenticators.
func TestOneSecretCannotEnrollTwoAuthenticators(t *testing.T) {
	ctx, _ := newContext(t)
	credentials := NewCredentials(CredentialOptions{})
	bootstraps := NewBootstrap()
	if err := bootstraps.Issue(ctx, bootstrap("login-1", "account-1", 3)); err != nil {
		t.Fatal(err)
	}
	spend := func(ctx context.Context) error { return bootstraps.Consume(ctx, "login-1", time.Now()) }

	if err := credentials.SaveFirstCredential(ctx, credential("cred-1", "account-1"), spend, nil); err != nil {
		t.Fatal(err)
	}
	err := credentials.SaveFirstCredential(ctx, credential("cred-2", "account-1"), spend, nil)
	if !errors.Is(err, auth.ErrUnknownBootstrap) {
		t.Fatalf("got %v, want ErrUnknownBootstrap", err)
	}
	if _, err := credentials.Find(ctx, []byte("cred-2")); !errors.Is(err, auth.ErrUnknownCredential) {
		t.Error("a second authenticator was enrolled on one secret")
	}
}

// --- allowlist -----------------------------------------------------------

func TestAllowlistAdmitsARegisteredIdentity(t *testing.T) {
	ctx, _ := newContext(t)
	if _, err := firestorebind.Store(ctx, Entry("https://issuer", "email", "someone@example.com", "the operator's note")); err != nil {
		t.Fatal(err)
	}
	registered, err := NewAllowlist().Registered(ctx, "https://issuer", []auth.AllowlistCandidate{
		{Claim: "sub", Value: "unregistered-subject"},
		{Claim: "email", Value: "someone@example.com"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !registered {
		t.Error("a registered identity was denied")
	}
}

func TestAllowlistDeniesAnUnregisteredIdentity(t *testing.T) {
	ctx, _ := newContext(t)
	registered, err := NewAllowlist().Registered(ctx, "https://issuer", []auth.AllowlistCandidate{
		{Claim: "email", Value: "stranger@example.com"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if registered {
		t.Error("an unregistered identity was admitted")
	}
}

// An entry registered under one issuer must not admit the same claim from
// another, which is what putting the issuer in the key buys.
func TestAllowlistIsScopedToTheIssuer(t *testing.T) {
	ctx, _ := newContext(t)
	if _, err := firestorebind.Store(ctx, Entry("https://issuer-a", "email", "someone@example.com", "")); err != nil {
		t.Fatal(err)
	}
	registered, err := NewAllowlist().Registered(ctx, "https://issuer-b", []auth.AllowlistCandidate{
		{Claim: "email", Value: "someone@example.com"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if registered {
		t.Error("an entry of one issuer admitted a login from another")
	}
}

// A backend failure must be an error and never a denial: reporting an outage as
// "not registered" would turn it into a silent access change.
func TestAllowlistReportsAFailureRatherThanDenying(t *testing.T) {
	ctx, fake := newContext(t)
	fake.FailNext("UNAVAILABLE")
	registered, err := NewAllowlist().Registered(ctx, "https://issuer", []auth.AllowlistCandidate{
		{Claim: "email", Value: "someone@example.com"},
	})
	if err == nil {
		t.Fatal("a backend failure was reported as a clean answer")
	}
	if registered {
		t.Error("a failed lookup admitted the login")
	}
}

func TestAllowlistWithNoCandidatesAsksNothing(t *testing.T) {
	ctx, fake := newContext(t)
	registered, err := NewAllowlist().Registered(ctx, "https://issuer", nil)
	if err != nil || registered {
		t.Fatalf("got %v, %v", registered, err)
	}
	if fake.Calls("lookup") != 0 {
		t.Error("an empty candidate set reached the service")
	}
}

// --- kinds ---------------------------------------------------------------

// The kind list a deployment applies TTL policies from is derived from the
// record types themselves, so a renamed property cannot leave it behind.
func TestTheBootstrapKindDeclaresItsExpiryProperty(t *testing.T) {
	property, expires := bootstrapEntity{}.ExpiryProperty()
	if !expires || property != bootstrapExpiresProperty {
		t.Fatalf("ExpiryProperty: got %q, %v", property, expires)
	}
	stored := bootstrapEntity{credential: bootstrap("login-1", "account-1", 1)}.EncodeEntity()
	if _, held := stored.Get(property); !held {
		t.Errorf("the declared expiry property %q is not written", property)
	}
}

// The credential and allowlist kinds have no expiry at all, and must not claim
// one: a TTL policy pointed at a property nothing maintains deletes nothing,
// and a policy pointed at a property that means something else deletes
// everything.
func TestTheCredentialAndAllowlistKindsDeclareNoExpiry(t *testing.T) {
	for _, record := range []any{credentialEntity{}, allowlistEntry{}} {
		if expirer, claims := record.(firestorebind.Expirer); claims {
			property, expires := expirer.ExpiryProperty()
			if expires {
				t.Errorf("%T claims to expire on %q", record, property)
			}
		}
	}
}

func TestWithoutAClientEveryStoreNamesTheImport(t *testing.T) {
	ctx := context.Background()
	checks := map[string]error{}
	_, checks["credential find"] = NewCredentials(CredentialOptions{}).Find(ctx, []byte("cred-1"))
	checks["credential save"] = NewCredentials(CredentialOptions{}).Save(ctx, credential("cred-1", "account-1"), nil)
	_, checks["bootstrap find"] = NewBootstrap().Find(ctx, "login-1")
	checks["bootstrap issue"] = NewBootstrap().Issue(ctx, bootstrap("login-1", "account-1", 1))
	_, checks["allowlist"] = NewAllowlist().Registered(ctx, "https://issuer",
		[]auth.AllowlistCandidate{{Claim: "email", Value: "someone@example.com"}})

	for name, err := range checks {
		if err == nil {
			t.Errorf("%s: succeeded without a client", name)
			continue
		}
		if !errors.Is(err, firestorebind.ErrNoClient) {
			t.Errorf("%s: got %v, want the no-client error", name, err)
		}
		if want := "database/firestore"; !strings.Contains(err.Error(), want) {
			t.Errorf("%s: %v does not name %s", name, err, want)
		}
	}
}
