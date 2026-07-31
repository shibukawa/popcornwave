package auth

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"

	"github.com/shibukawa/popcornwave/contrib/jwt"

	_ "github.com/shibukawa/tinygodriver/database/sql/sqlite"
)

func allowlistDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "auth.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Exec(allowlistSchemaSQL("sqlite")); err != nil {
		t.Fatal(err)
	}
	return db
}

func testIdentity(issuer, subject string, claims map[string]string) Identity {
	raw := make(map[string]json.RawMessage, len(claims)+1)
	for name, value := range claims {
		encoded, _ := json.Marshal(value)
		raw[name] = encoded
	}
	encodedSubject, _ := json.Marshal(subject)
	raw["sub"] = encodedSubject
	return identityFrom(jwt.Claims{Issuer: issuer, Subject: subject, Raw: raw}, ClaimSubject)
}

func TestRegisteredAdmissionRequiresAnAllowlistedIdentity(t *testing.T) {
	db := allowlistDB(t)
	if _, err := db.Exec(
		`INSERT INTO `+AllowlistTable+`(issuer, claim, value, note) VALUES(?, ?, ?, ?)`,
		"https://issuer.example", "email", "known@example.com", "operator"); err != nil {
		t.Fatal(err)
	}
	allowlist := Allowlist{db: db}
	// The identity claim is the subject here, so recognizing a registration by
	// email is an explicit configuration choice.
	config := OIDCConfig{
		Admission: AdmissionRegistered, AutoProvision: true,
		RegisteredClaims: []string{"email"},
	}

	known := testIdentity("https://issuer.example", "subject-1", map[string]string{
		"email": "known@example.com", "name": "Known",
	})
	account, err := admit(context.Background(), config, allowlist, known)
	if err != nil || account.ID == "" {
		t.Fatalf("registered identity = %#v err = %v", account, err)
	}

	unknown := testIdentity("https://issuer.example", "subject-2", map[string]string{
		"email": "stranger@example.com",
	})
	if _, err := admit(context.Background(), config, allowlist, unknown); !errors.Is(err, ErrAccessDenied) {
		t.Fatalf("unregistered identity = %v", err)
	}

	// The same address registered for another issuer must not admit anyone.
	otherIssuer := testIdentity("https://other.example", "subject-1", map[string]string{
		"email": "known@example.com",
	})
	if _, err := admit(context.Background(), config, allowlist, otherIssuer); !errors.Is(err, ErrAccessDenied) {
		t.Fatalf("identity of another issuer = %v", err)
	}
}

func TestRegisteredAdmissionMatchesTheSubjectClaim(t *testing.T) {
	db := allowlistDB(t)
	if _, err := db.Exec(
		`INSERT INTO `+AllowlistTable+`(issuer, claim, value) VALUES(?, ?, ?)`,
		"https://issuer.example", "sub", "subject-1"); err != nil {
		t.Fatal(err)
	}
	allowlist := Allowlist{db: db}
	config := OIDCConfig{Admission: AdmissionRegistered, AutoProvision: true}

	identity := testIdentity("https://issuer.example", "subject-1", nil)
	if _, err := admit(context.Background(), config, allowlist, identity); err != nil {
		t.Fatalf("subject-registered identity = %v", err)
	}

	// A configured claim list narrows what is compared.
	config.RegisteredClaims = []string{"email"}
	if _, err := admit(context.Background(), config, allowlist, identity); !errors.Is(err, ErrAccessDenied) {
		t.Fatalf("subject match outside the configured claims = %v", err)
	}
}

func TestAllowlistFailureIsNotADenial(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "empty.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	// The table is missing, so the lookup fails. An outage must surface as an
	// error rather than silently denying or admitting.
	allowlist := Allowlist{db: db}
	config := OIDCConfig{Admission: AdmissionRegistered, AutoProvision: true}
	identity := testIdentity("https://issuer.example", "subject-1", nil)
	_, err = admit(context.Background(), config, allowlist, identity)
	if err == nil || errors.Is(err, ErrAccessDenied) {
		t.Fatalf("allowlist backend failure = %v", err)
	}
}

func TestLookupValueRejectsUnusableClaims(t *testing.T) {
	identity := testIdentity("https://issuer.example", "subject-1", map[string]string{"email": ""})
	if _, ok := claimLookupValue("email", identity); ok {
		t.Fatal("empty claim value was compared")
	}
	if _, ok := claimLookupValue("missing", identity); ok {
		t.Fatal("absent claim was compared")
	}
	if _, ok := claimLookupValue("", identity); ok {
		t.Fatal("empty claim name was compared")
	}
	if value, ok := claimLookupValue("sub", identity); !ok || value != "subject-1" {
		t.Fatalf("subject claim = %q %v", value, ok)
	}
}

// TestLookupValueAcceptsIntegerClaims covers a directory that emits its own
// identifier, such as an employee number, as a JSON number.
func TestLookupValueAcceptsIntegerClaims(t *testing.T) {
	identity := Identity{
		Issuer:  "https://issuer.example",
		Subject: "subject-1",
		Claims: Claims{raw: map[string]json.RawMessage{
			"employee_number": json.RawMessage(`10231`),
			"negative":        json.RawMessage(`-42`),
			"fractional":      json.RawMessage(`10231.0`),
			"exponent":        json.RawMessage(`1e5`),
			"object":          json.RawMessage(`{"id":1}`),
			"array":           json.RawMessage(`[1]`),
			"boolean":         json.RawMessage(`true`),
		}},
	}
	if value, ok := claimLookupValue("employee_number", identity); !ok || value != "10231" {
		t.Fatalf("integer claim = %q %v", value, ok)
	}
	if value, ok := claimLookupValue("negative", identity); !ok || value != "-42" {
		t.Fatalf("negative integer claim = %q %v", value, ok)
	}
	// An ambiguous or structured value is refused rather than normalized, so
	// two systems cannot disagree about what the identifier is.
	for _, claim := range []string{"fractional", "exponent", "object", "array", "boolean"} {
		if value, ok := claimLookupValue(claim, identity); ok {
			t.Errorf("claim %q was accepted as %q", claim, value)
		}
	}
}

// TestIdentityKeyUsesTheConfiguredClaim covers the account lookup key a
// deployment selects with auth.oidc.identity_claim.
func TestIdentityKeyUsesTheConfiguredClaim(t *testing.T) {
	claims := jwt.Claims{
		Issuer:  "https://issuer.example",
		Subject: "8f0c4c3e",
		Raw: map[string]json.RawMessage{
			"sub":             json.RawMessage(`"8f0c4c3e"`),
			"employee_number": json.RawMessage(`"E-10231"`),
		},
	}
	identity := identityFrom(claims, "employee_number")
	if identity.KeyClaim != "employee_number" || identity.Key != "E-10231" {
		t.Fatalf("identity key = %q from %q", identity.Key, identity.KeyClaim)
	}
	// The subject stays available even when it does not identify the account.
	if identity.Subject != "8f0c4c3e" {
		t.Fatalf("subject = %q", identity.Subject)
	}

	// An empty configuration keeps the OpenID Connect default.
	if identity := identityFrom(claims, ""); identity.KeyClaim != ClaimSubject || identity.Key != "8f0c4c3e" {
		t.Fatalf("default identity key = %q from %q", identity.Key, identity.KeyClaim)
	}

	// A configured claim the token does not carry denies the login instead of
	// silently falling back to the subject.
	missing := identityFrom(claims, "staff_id")
	if missing.Key != "" {
		t.Fatalf("missing identity claim produced key %q", missing.Key)
	}
	_, err := admit(context.Background(), OIDCConfig{Admission: AdmissionAuthenticated, AutoProvision: true},
		Allowlist{}, missing)
	if !errors.Is(err, ErrAccessDenied) {
		t.Fatalf("login without the configured identity claim = %v", err)
	}
}

// TestDerivedAccountFollowsTheIdentityClaim checks that the default resolver
// keys accounts on the configured claim rather than always on the subject.
func TestDerivedAccountFollowsTheIdentityClaim(t *testing.T) {
	base := jwt.Claims{
		Issuer:  "https://issuer.example",
		Subject: "subject-1",
		Raw: map[string]json.RawMessage{
			"sub":             json.RawMessage(`"subject-1"`),
			"employee_number": json.RawMessage(`"E-10231"`),
		},
	}
	rotated := jwt.Claims{
		Issuer:  "https://issuer.example",
		Subject: "subject-2",
		Raw: map[string]json.RawMessage{
			"sub":             json.RawMessage(`"subject-2"`),
			"employee_number": json.RawMessage(`"E-10231"`),
		},
	}
	first, err := derivedAccount(context.Background(), identityFrom(base, "employee_number"), true)
	if err != nil {
		t.Fatal(err)
	}
	second, err := derivedAccount(context.Background(), identityFrom(rotated, "employee_number"), true)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID {
		t.Fatal("a new subject for the same employee number produced a different account")
	}
	bySubject, err := derivedAccount(context.Background(), identityFrom(base, ClaimSubject), true)
	if err != nil {
		t.Fatal(err)
	}
	if bySubject.ID == first.ID {
		t.Fatal("different identity claims produced the same account identifier")
	}
}
