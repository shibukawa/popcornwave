package dynamo

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/shibukawa/popcornwave/plugin/auth"
	"github.com/shibukawa/tinygodriver/nosql/dynamodb"
)

// Credential attribute names. credentialKeyAttribute and accountIndexAttribute
// are each named once and used by both the table definition and the item, so
// the two cannot drift.
const (
	credentialKeyAttribute = "credential_id"
	accountIndexAttribute  = "account_id"
	accountIndexName       = "account_id_index"

	userHandleAttribute     = "user_handle"
	publicKeyAttribute      = "public_key"
	publicKeyXAttribute     = "public_key_x"
	publicKeyYAttribute     = "public_key_y"
	algorithmAttribute      = "algorithm"
	signCountAttribute      = "sign_count"
	backupEligibleAttribute = "backup_eligible"
	backupStateAttribute    = "backup_state"
	transportsAttribute     = "transports"
	labelAttribute          = "label"
	credentialCreatedAt     = "created_at"
	lastUsedAtAttribute     = "last_used_at"
)

// maxLabelBytes bounds the one credential field an application controls, so a
// long label cannot be the reason a record crosses the item limit.
const maxLabelBytes = 512

// CredentialTable is the definition of the passkey credential table.
//
// A login resolves a credential ID, so that is the partition key. Listing an
// account's credentials is the opposite question, which a global secondary
// index on the account answers. The index is declared here because the driver
// has no UpdateTable: an index missing at creation can never be added, so a
// table created before this definition existed has to be recreated.
func CredentialTable(name string) dynamodb.TableDefinition {
	return dynamodb.TableDefinition{
		Name:         name,
		PartitionKey: dynamodb.KeyAttribute{Name: credentialKeyAttribute, Type: dynamodb.TypeBinary},
		GlobalIndexes: []dynamodb.SecondaryIndex{{
			Name:         accountIndexName,
			PartitionKey: dynamodb.KeyAttribute{Name: accountIndexAttribute, Type: dynamodb.TypeString},
			// A listing needs the credential ID and the transports for
			// excludeCredentials, and the user handle, because an enrollment
			// reuses the handle the account already has rather than minting a
			// second one. A ceremony reads the full record by ID afterwards,
			// so nothing else is projected.
			Projection: "INCLUDE",
			Include:    []string{transportsAttribute, credentialCreatedAt, userHandleAttribute},
		}},
	}
}

// Credentials is the DynamoDB passkey credential store.
type Credentials struct {
	now func() time.Time
}

var (
	_ auth.CredentialStore      = (*Credentials)(nil)
	_ auth.FirstEnrollmentStore = (*Credentials)(nil)
)

// CredentialOptions configures a credential store.
type CredentialOptions struct {
	// Now is injectable for tests.
	Now func() time.Time
}

// NewCredentials builds the store. It opens nothing: the client comes from the
// request context, installed by the database/dynamo middleware.
func NewCredentials(options CredentialOptions) *Credentials {
	now := options.Now
	if now == nil {
		now = time.Now
	}
	return &Credentials{now: now}
}

// Find reads one credential by ID, consistently: a credential enrolled moments
// ago must be findable on the next login, and an eventually consistent read
// would make that a coin flip.
func (s *Credentials) Find(ctx context.Context, credentialID []byte) (auth.Credential, error) {
	if len(credentialID) == 0 {
		return auth.Credential{}, auth.ErrUnknownCredential
	}
	client, table, err := resolve(ctx, DeclaredCredentialTable)
	if err != nil {
		return auth.Credential{}, err
	}
	item, err := client.GetItem(ctx, table,
		dynamodb.Key{credentialKeyAttribute: dynamodb.B(credentialID)},
		dynamodb.WithConsistentRead(true))
	switch {
	case errors.Is(err, dynamodb.ErrItemNotFound):
		return auth.Credential{}, auth.ErrUnknownCredential
	case err != nil:
		return auth.Credential{}, fmt.Errorf("auth: read credential: %w", err)
	}
	return decodeCredential(item), nil
}

// ListByAccount supplies excludeCredentials and allowCredentials, through the
// account index.
func (s *Credentials) ListByAccount(ctx context.Context, accountID string) ([]auth.Credential, error) {
	if accountID == "" {
		return nil, nil
	}
	client, table, err := resolve(ctx, DeclaredCredentialTable)
	if err != nil {
		return nil, err
	}
	var credentials []auth.Credential
	var start dynamodb.Key
	for {
		options := []dynamodb.QueryOption{
			dynamodb.WithIndex(accountIndexName),
			dynamodb.WithExpressionNames(map[string]string{"#account": accountIndexAttribute}),
			dynamodb.WithExpressionValues(map[string]dynamodb.AttributeValue{":account": dynamodb.S(accountID)}),
		}
		if start != nil {
			options = append(options, dynamodb.WithExclusiveStartKey(start))
		}
		page, err := client.Query(ctx, table, "#account = :account", options...)
		if err != nil {
			return nil, fmt.Errorf("auth: list credentials: %w", err)
		}
		for _, item := range page.Items {
			credentials = append(credentials, decodeCredential(item))
		}
		if len(page.LastEvaluatedKey) == 0 {
			break
		}
		start = page.LastEvaluatedKey
	}
	return credentials, nil
}

// Save persists a new credential.
//
// The condition makes it an insert: a retried request that already stored the
// credential is a conditional-check failure rather than a second credential.
//
// A non-nil within runs before the write rather than after it. The framework
// uses SaveFirstCredential for the enrollment that needs a sequence, so this
// path exists for a caller that bundles its own; running the callback first
// keeps the one invariant that matters either way, which is that no credential
// is stored while the bootstrap credential authorizing it is still unspent.
func (s *Credentials) Save(ctx context.Context, credential auth.Credential, within func(context.Context) error) error {
	if within != nil {
		if err := within(ctx); err != nil {
			return err
		}
	}
	return s.put(ctx, credential)
}

// SaveFirstCredential applies a passkey-only registration in the order every
// partial outcome of which is safe.
//
// Spending the bootstrap credential first makes the whole sequence single-use:
// a credential written before it could be written twice by two parallel
// redemptions of one issued secret. The reachable interruptions are therefore
// "bootstrap spent, no credential, account still provisional" and "bootstrap
// spent, credential stored, account still provisional". Neither can create a
// session, because a provisional account cannot, and both are resolved by an
// administrator issuing a new bootstrap credential.
//
// Nothing is rolled back after a failure. A compensating delete that itself
// failed would leave a state nobody specified, which is worse than a state the
// documentation names.
func (s *Credentials) SaveFirstCredential(ctx context.Context, credential auth.Credential, spend, activate func(context.Context) error) error {
	if spend != nil {
		if err := spend(ctx); err != nil {
			return err
		}
	}
	if err := s.put(ctx, credential); err != nil {
		return err
	}
	if activate != nil {
		return activate(ctx)
	}
	return nil
}

func (s *Credentials) put(ctx context.Context, credential auth.Credential) error {
	if len(credential.CredentialID) == 0 || credential.AccountID == "" {
		return errors.New("auth: credential needs an ID and an account")
	}
	if len(credential.Label) > maxLabelBytes {
		return fmt.Errorf("auth: credential label is %d bytes, over the limit of %d",
			len(credential.Label), maxLabelBytes)
	}
	client, table, err := resolve(ctx, DeclaredCredentialTable)
	if err != nil {
		return err
	}
	item := dynamodb.Item{
		credentialKeyAttribute:  dynamodb.B(credential.CredentialID),
		accountIndexAttribute:   dynamodb.S(credential.AccountID),
		userHandleAttribute:     dynamodb.B(credential.UserHandle),
		publicKeyAttribute:      dynamodb.B(credential.PublicKey),
		publicKeyXAttribute:     dynamodb.B(credential.PublicKeyX),
		publicKeyYAttribute:     dynamodb.B(credential.PublicKeyY),
		algorithmAttribute:      dynamodb.N(credential.Algorithm),
		signCountAttribute:      dynamodb.N(credential.SignCount),
		backupEligibleAttribute: dynamodb.Bool(credential.BackupEligible),
		backupStateAttribute:    dynamodb.Bool(credential.BackupState),
		transportsAttribute:     dynamodb.S(strings.Join(credential.Transports, ",")),
		labelAttribute:          dynamodb.S(credential.Label),
		credentialCreatedAt:     epoch(credential.CreatedAt),
	}
	if size := itemBytes(item); size > maxItemBytes {
		return fmt.Errorf("auth: credential is %d bytes, over the DynamoDB item limit of %d",
			size, maxItemBytes)
	}
	_, err = client.PutItem(ctx, table, item,
		dynamodb.WithCondition("attribute_not_exists(#key)"),
		dynamodb.WithExpressionNames(map[string]string{"#key": credentialKeyAttribute}))
	switch {
	case errors.Is(err, dynamodb.ErrConditionalCheck):
		return errors.New("auth: credential already exists")
	case err != nil:
		return fmt.Errorf("auth: save credential: %w", err)
	}
	return nil
}

// UpdateOnAssertion persists the accepted counter and backup state.
//
// The condition requires the stored counter to be below the accepted one. A
// counter that does not move forward is a replayed or cloned authenticator, so
// the store refuses it rather than leaving the caller to notice. A zero stored
// counter means the authenticator does not keep one, which the condition
// tolerates by comparing only when the incoming count is above zero.
func (s *Credentials) UpdateOnAssertion(ctx context.Context, credentialID []byte, signCount uint32, backupState bool, usedAt time.Time) error {
	if len(credentialID) == 0 {
		return auth.ErrUnknownCredential
	}
	client, table, err := resolve(ctx, DeclaredCredentialTable)
	if err != nil {
		return err
	}
	names := map[string]string{
		"#key":    credentialKeyAttribute,
		"#count":  signCountAttribute,
		"#backup": backupStateAttribute,
		"#used":   lastUsedAtAttribute,
	}
	values := map[string]dynamodb.AttributeValue{
		":count":  dynamodb.N(signCount),
		":backup": dynamodb.Bool(backupState),
		":used":   epoch(usedAt),
	}
	condition := "attribute_exists(#key)"
	if signCount > 0 {
		condition += " AND #count < :count"
	}
	_, err = client.UpdateItem(ctx, table,
		dynamodb.Key{credentialKeyAttribute: dynamodb.B(credentialID)},
		"SET #count = :count, #backup = :backup, #used = :used",
		dynamodb.WithCondition(condition),
		dynamodb.WithExpressionNames(names),
		dynamodb.WithExpressionValues(values))
	switch {
	case errors.Is(err, dynamodb.ErrConditionalCheck):
		// Either the credential is gone or its counter did not advance. Both
		// fail the ceremony closed; neither is downgraded to a warning,
		// because a counter that silently fails to persist is exactly what a
		// cloned authenticator needs.
		return auth.ErrUnknownCredential
	case err != nil:
		return fmt.Errorf("auth: update credential: %w", err)
	}
	return nil
}

// Delete removes one credential of an account. The condition keeps the account
// check the relational statement makes in its WHERE clause, so one account
// cannot delete another's credential by ID.
func (s *Credentials) Delete(ctx context.Context, accountID string, credentialID []byte) error {
	if accountID == "" || len(credentialID) == 0 {
		return auth.ErrUnknownCredential
	}
	client, table, err := resolve(ctx, DeclaredCredentialTable)
	if err != nil {
		return err
	}
	_, err = client.DeleteItem(ctx, table,
		dynamodb.Key{credentialKeyAttribute: dynamodb.B(credentialID)},
		dynamodb.WithCondition("#account = :account"),
		dynamodb.WithExpressionNames(map[string]string{"#account": accountIndexAttribute}),
		dynamodb.WithExpressionValues(map[string]dynamodb.AttributeValue{":account": dynamodb.S(accountID)}))
	switch {
	case errors.Is(err, dynamodb.ErrConditionalCheck):
		return auth.ErrUnknownCredential
	case err != nil:
		return fmt.Errorf("auth: delete credential: %w", err)
	}
	return nil
}

func decodeCredential(item dynamodb.Item) auth.Credential {
	credential := auth.Credential{
		CredentialID:   readBytes(item, credentialKeyAttribute),
		AccountID:      readString(item, accountIndexAttribute),
		UserHandle:     readBytes(item, userHandleAttribute),
		PublicKey:      readBytes(item, publicKeyAttribute),
		PublicKeyX:     readBytes(item, publicKeyXAttribute),
		PublicKeyY:     readBytes(item, publicKeyYAttribute),
		Algorithm:      int(readInt(item, algorithmAttribute)),
		SignCount:      uint32(readInt(item, signCountAttribute)),
		BackupEligible: readBool(item, backupEligibleAttribute),
		BackupState:    readBool(item, backupStateAttribute),
		Label:          readString(item, labelAttribute),
		CreatedAt:      readEpoch(item, credentialCreatedAt),
		LastUsedAt:     readEpoch(item, lastUsedAtAttribute),
	}
	if transports := readString(item, transportsAttribute); transports != "" {
		credential.Transports = strings.Split(transports, ",")
	}
	return credential
}

// itemBytes approximates the stored size the way DynamoDB measures it: the
// attribute names plus the values. It is used only to refuse an oversized
// record before the request, so an approximation that never underestimates the
// parts that matter is enough.
func itemBytes(item dynamodb.Item) int {
	total := 0
	for name, value := range item {
		total += len(name)
		if text, held := value.AsString(); held {
			total += len(text)
			continue
		}
		if raw, held := value.AsBytes(); held {
			total += len(raw)
			continue
		}
		if number, held := value.AsNumber(); held {
			total += len(number)
			continue
		}
		total++
	}
	return total
}
