package auth

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/shibukawa/tinygodriver/database/sql/sqlite"
)

func storeDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	for _, statement := range []string{credentialSchemaSQL, credentialAccountIndexSQL, bootstrapSchemaSQL} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	return db
}

func testCredential(accountID string, id byte) Credential {
	return Credential{
		CredentialID:   []byte{id, id, id, id},
		AccountID:      accountID,
		UserHandle:     []byte("handle-" + accountID),
		PublicKey:      []byte("cose-key"),
		PublicKeyX:     bytes.Repeat([]byte{id}, 32),
		PublicKeyY:     bytes.Repeat([]byte{id ^ 0xff}, 32),
		Algorithm:      -7,
		BackupEligible: true,
		Transports:     []string{"internal", "hybrid"},
		CreatedAt:      time.Unix(1_700_000_000, 0).UTC(),
	}
}

func TestCredentialRoundTrip(t *testing.T) {
	store := dbStore{db: storeDB(t)}
	ctx := context.Background()
	saved := testCredential("account-1", 0x11)
	if err := store.Save(ctx, saved, nil); err != nil {
		t.Fatalf("Save: %v", err)
	}

	found, err := store.Find(ctx, saved.CredentialID)
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if found.AccountID != saved.AccountID || string(found.UserHandle) != string(saved.UserHandle) {
		t.Fatalf("Find returned %+v, want the saved credential", found)
	}
	if len(found.Transports) != 2 || found.Transports[0] != "internal" {
		t.Fatalf("transports = %v, want them preserved", found.Transports)
	}
	if !found.BackupEligible || found.SignCount != 0 {
		t.Fatalf("Find returned %+v, want backup eligible with a zero counter", found)
	}

	// An assertion persists the accepted counter and backup state together.
	used := time.Unix(1_700_000_500, 0).UTC()
	if err := store.UpdateOnAssertion(ctx, saved.CredentialID, 7, true, used); err != nil {
		t.Fatalf("UpdateOnAssertion: %v", err)
	}
	found, err = store.Find(ctx, saved.CredentialID)
	if err != nil {
		t.Fatalf("Find after assertion: %v", err)
	}
	if found.SignCount != 7 || !found.BackupState || found.LastUsedAt.IsZero() {
		t.Fatalf("after assertion = %+v, want counter 7, backed up, and a last use", found)
	}
}

func TestUnknownCredentialIsReportedAsSuch(t *testing.T) {
	store := dbStore{db: storeDB(t)}
	ctx := context.Background()
	for _, testCase := range []struct {
		name string
		call func() error
	}{
		{"find", func() error { _, err := store.Find(ctx, []byte{0x99}); return err }},
		{"find empty", func() error { _, err := store.Find(ctx, nil); return err }},
		{"update", func() error { return store.UpdateOnAssertion(ctx, []byte{0x99}, 1, false, time.Now()) }},
		{"delete", func() error { return store.Delete(ctx, "account-1", []byte{0x99}) }},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			if err := testCase.call(); !errors.Is(err, ErrUnknownCredential) {
				t.Fatalf("error = %v, want %v", err, ErrUnknownCredential)
			}
		})
	}
}

func TestListByAccountReturnsOnlyThatAccount(t *testing.T) {
	store := dbStore{db: storeDB(t)}
	ctx := context.Background()
	for _, credential := range []Credential{
		testCredential("account-1", 0x11),
		testCredential("account-1", 0x22),
		testCredential("account-2", 0x33),
	} {
		if err := store.Save(ctx, credential, nil); err != nil {
			t.Fatalf("Save: %v", err)
		}
	}

	listed, err := store.ListByAccount(ctx, "account-1")
	if err != nil {
		t.Fatalf("ListByAccount: %v", err)
	}
	if len(listed) != 2 {
		t.Fatalf("listed %d credentials, want 2", len(listed))
	}
	for _, credential := range listed {
		if credential.AccountID != "account-1" {
			t.Fatalf("listed a credential of %q", credential.AccountID)
		}
	}

	empty, err := store.ListByAccount(ctx, "")
	if err != nil || len(empty) != 0 {
		t.Fatalf("ListByAccount(\"\") = %v, %v; want no rows and no error", empty, err)
	}
}

// A first enrollment persists the credential, activates the account, and
// consumes the bootstrap credential as one unit. A failure anywhere in that
// unit must leave no credential behind.
func TestSaveRunsTheCallbackInTheSameTransaction(t *testing.T) {
	db := storeDB(t)
	store := dbStore{db: db}
	bootstrap := bootstrapStore{db: db}
	ctx := context.Background()
	issued := BootstrapCredential{
		LoginID: "login-1", AccountID: "account-1", SecretDigest: []byte("digest"),
		Purpose: PurposeInitialPasskey, IssuedAt: time.Unix(1_700_000_000, 0).UTC(),
		ExpiresAt: time.Unix(1_700_086_400, 0).UTC(), AttemptsRemaining: 5,
	}
	if err := bootstrap.Issue(ctx, issued); err != nil {
		t.Fatalf("Issue: %v", err)
	}

	credential := testCredential("account-1", 0x11)
	err := store.Save(ctx, credential, func(txCtx context.Context) error {
		return bootstrap.Consume(txCtx, "login-1", time.Unix(1_700_000_100, 0).UTC())
	})
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := bootstrap.Find(ctx, "login-1"); !errors.Is(err, ErrUnknownBootstrap) {
		t.Fatalf("consumed credential is still findable: %v", err)
	}
	if _, err := store.Find(ctx, credential.CredentialID); err != nil {
		t.Fatalf("Find after Save: %v", err)
	}
}

func TestSaveRollsBackWhenTheCallbackFails(t *testing.T) {
	db := storeDB(t)
	store := dbStore{db: db}
	ctx := context.Background()
	credential := testCredential("account-1", 0x11)
	activationFailed := errors.New("account activation failed")

	err := store.Save(ctx, credential, func(context.Context) error { return activationFailed })
	if !errors.Is(err, activationFailed) {
		t.Fatalf("Save error = %v, want %v", err, activationFailed)
	}
	// Leaving a credential behind would make an account that cannot log in
	// look enrolled.
	if _, err := store.Find(ctx, credential.CredentialID); !errors.Is(err, ErrUnknownCredential) {
		t.Fatalf("credential survived a failed enrollment: %v", err)
	}
}

func TestSaveRejectsAnIncompleteCredential(t *testing.T) {
	store := dbStore{db: storeDB(t)}
	ctx := context.Background()
	if err := store.Save(ctx, Credential{AccountID: "account-1"}, nil); err == nil {
		t.Fatal("Save accepted a credential with no ID")
	}
	if err := store.Save(ctx, Credential{CredentialID: []byte{0x11}}, nil); err == nil {
		t.Fatal("Save accepted a credential with no account")
	}
}

func TestBootstrapAttemptBudgetIsSpentAtomically(t *testing.T) {
	db := storeDB(t)
	bootstrap := bootstrapStore{db: db}
	ctx := context.Background()
	if err := bootstrap.Issue(ctx, BootstrapCredential{
		LoginID: "login-1", AccountID: "account-1", SecretDigest: []byte("digest"),
		Purpose: PurposeInitialPasskey, IssuedAt: time.Unix(1_700_000_000, 0).UTC(),
		ExpiresAt: time.Unix(1_700_086_400, 0).UTC(), AttemptsRemaining: 2,
	}); err != nil {
		t.Fatalf("Issue: %v", err)
	}

	for want := 1; want >= 0; want-- {
		remaining, err := bootstrap.RecordAttempt(ctx, "login-1")
		if err != nil {
			t.Fatalf("RecordAttempt: %v", err)
		}
		if remaining != want {
			t.Fatalf("remaining = %d, want %d", remaining, want)
		}
	}
	// An exhausted budget is indistinguishable from an unknown login ID, so a
	// guess learns nothing from the response.
	if _, err := bootstrap.RecordAttempt(ctx, "login-1"); !errors.Is(err, ErrUnknownBootstrap) {
		t.Fatalf("exhausted budget error = %v, want %v", err, ErrUnknownBootstrap)
	}
	if _, err := bootstrap.RecordAttempt(ctx, "never-issued"); !errors.Is(err, ErrUnknownBootstrap) {
		t.Fatalf("unknown login error = %v, want %v", err, ErrUnknownBootstrap)
	}
}

func TestBootstrapCredentialIsConsumedOnce(t *testing.T) {
	db := storeDB(t)
	bootstrap := bootstrapStore{db: db}
	ctx := context.Background()
	if err := bootstrap.Issue(ctx, BootstrapCredential{
		LoginID: "login-1", AccountID: "account-1", SecretDigest: []byte("digest"),
		Purpose: PurposeRecoveryPasskey, IssuedAt: time.Unix(1_700_000_000, 0).UTC(),
		ExpiresAt: time.Unix(1_700_086_400, 0).UTC(), AttemptsRemaining: 3,
	}); err != nil {
		t.Fatalf("Issue: %v", err)
	}
	found, err := bootstrap.Find(ctx, "login-1")
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if found.Purpose != PurposeRecoveryPasskey || found.AttemptsRemaining != 3 {
		t.Fatalf("Find returned %+v, want the issued credential", found)
	}

	at := time.Unix(1_700_000_100, 0).UTC()
	if err := bootstrap.Consume(ctx, "login-1", at); err != nil {
		t.Fatalf("Consume: %v", err)
	}
	if err := bootstrap.Consume(ctx, "login-1", at); !errors.Is(err, ErrUnknownBootstrap) {
		t.Fatalf("second Consume error = %v, want %v", err, ErrUnknownBootstrap)
	}
}

func TestBootstrapIssueRejectsAnIncompleteCredential(t *testing.T) {
	bootstrap := bootstrapStore{db: storeDB(t)}
	ctx := context.Background()
	for _, credential := range []BootstrapCredential{
		{AccountID: "account-1", SecretDigest: []byte("digest")},
		{LoginID: "login-1", SecretDigest: []byte("digest")},
		{LoginID: "login-1", AccountID: "account-1"},
	} {
		if err := bootstrap.Issue(ctx, credential); err == nil {
			t.Fatalf("Issue accepted %+v", credential)
		}
	}
}

// The framework asks for a table only when it is the one that will write to it.
func TestRequiredTablesFollowTheModeAndTheInstalledStores(t *testing.T) {
	t.Cleanup(func() {
		SetCredentialStore(nil)
		SetBootstrapStore(nil)
	})

	oidcOnly := names(requiredTables("popcornwave_session", baseConfig(ModeOIDCOnly)))
	if contains(oidcOnly, CredentialTable) || contains(oidcOnly, BootstrapTable) {
		t.Fatalf("oidc_only required %v, want no passkey table", oidcOnly)
	}

	passkeyOnly := names(requiredTables("popcornwave_session", baseConfig(ModePasskeyOnly)))
	if !contains(passkeyOnly, CredentialTable) || !contains(passkeyOnly, BootstrapTable) {
		t.Fatalf("passkey_only required %v, want both passkey tables", passkeyOnly)
	}

	// An application that owns its storage is never asked for the table the
	// framework would have written to.
	SetCredentialStore(dbStore{})
	SetBootstrapStore(bootstrapStore{})
	installed := names(requiredTables("popcornwave_session", baseConfig(ModePasskeyOnly)))
	if contains(installed, CredentialTable) || contains(installed, BootstrapTable) {
		t.Fatalf("required %v after installing stores, want neither passkey table", installed)
	}
}

// A deployment that issues no temporary secret needs no bootstrap table.
func TestBootstrapTableIsRequiredOnlyWhenACredentialIsIssued(t *testing.T) {
	config := baseConfig(ModePasskeyOnly)
	config.Registration.Policy = RegistrationDisabled
	config.Recovery.Policy = RecoveryApplication
	if contains(names(requiredTables("popcornwave_session", config)), BootstrapTable) {
		t.Fatal("a deployment that issues no bootstrap credential was asked for the table")
	}

	config.Recovery.Policy = RecoveryAdministrator
	if !contains(names(requiredTables("popcornwave_session", config)), BootstrapTable) {
		t.Fatal("administrator recovery issues a credential but the table was not required")
	}
}

func names(tables [][2]string) []string {
	result := make([]string, 0, len(tables))
	for _, table := range tables {
		result = append(result, table[0])
	}
	return result
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
