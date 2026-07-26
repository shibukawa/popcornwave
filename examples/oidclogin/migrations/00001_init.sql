-- +goose Up
-- Accounts and their external identities are application-owned. The framework
-- owns only its session table, which it creates during startup.
CREATE TABLE accounts (
    id TEXT PRIMARY KEY,
    display_name TEXT NOT NULL,
    email TEXT NOT NULL
);

-- One verified identity links to exactly one local account. The claim column
-- records which verified claim identified it, because auth.oidc.identity_claim
-- selects that claim: a directory that issues employee numbers is usually
-- keyed on those rather than on the subject.
CREATE TABLE external_identities (
    issuer TEXT NOT NULL,
    claim TEXT NOT NULL,
    value TEXT NOT NULL,
    account_id TEXT NOT NULL REFERENCES accounts(id),
    PRIMARY KEY (issuer, claim, value)
);

-- +goose Down
DROP TABLE external_identities;
DROP TABLE accounts;
