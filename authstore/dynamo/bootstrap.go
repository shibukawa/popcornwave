package dynamo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/shibukawa/popcornweb/plugin/auth"
	"github.com/shibukawa/tinygodriver/nosql/dynamodb"
)

// Bootstrap attribute names. bootstrapKeyAttribute is named once and used by
// both the table definition and the item, so the two cannot drift.
const (
	bootstrapKeyAttribute = "login_id"

	bootstrapAccountAttribute  = "account_id"
	secretDigestAttribute      = "secret_digest"
	purposeAttribute           = "purpose"
	issuedAtAttribute          = "issued_at"
	bootstrapExpiresAtAttrib   = "expires_at"
	attemptsRemainingAttribute = "attempts_remaining"
	consumedAtAttribute        = "consumed_at"
)

// BootstrapTable is the definition of the issued bootstrap credential table.
//
// A redemption starts from the login ID an operator handed out, so that is the
// partition key and there is no second question to index for. expires_at is the
// attribute a deployment points TTL at; nothing here enables or verifies it,
// and a table without it retains every spent credential forever.
func BootstrapTable(name string) dynamodb.TableDefinition {
	return dynamodb.TableDefinition{
		Name:         name,
		PartitionKey: dynamodb.KeyAttribute{Name: bootstrapKeyAttribute, Type: dynamodb.TypeString},
	}
}

// Bootstrap is the DynamoDB store of issued bootstrap credentials.
type Bootstrap struct{}

var _ auth.BootstrapStore = Bootstrap{}

// NewBootstrap builds the store. It opens nothing: the client comes from the
// request context, installed by the database/dynamo middleware.
func NewBootstrap() Bootstrap { return Bootstrap{} }

// Issue records a new credential. The condition refuses a login ID that is
// already live rather than overwriting it, so re-issuing to the same ID cannot
// silently extend someone else's window.
func (Bootstrap) Issue(ctx context.Context, credential auth.BootstrapCredential) error {
	if credential.LoginID == "" || credential.AccountID == "" || len(credential.SecretDigest) == 0 {
		return errors.New("auth: bootstrap credential needs a login ID, an account, and a secret digest")
	}
	client, table, err := resolve(ctx, DeclaredBootstrapTable)
	if err != nil {
		return err
	}
	item := dynamodb.Item{
		bootstrapKeyAttribute:      dynamodb.S(credential.LoginID),
		bootstrapAccountAttribute:  dynamodb.S(credential.AccountID),
		secretDigestAttribute:      dynamodb.B(credential.SecretDigest),
		purposeAttribute:           dynamodb.S(credential.Purpose),
		issuedAtAttribute:          epoch(credential.IssuedAt),
		bootstrapExpiresAtAttrib:   epoch(credential.ExpiresAt),
		attemptsRemainingAttribute: dynamodb.N(credential.AttemptsRemaining),
	}
	_, err = client.PutItem(ctx, table, item,
		dynamodb.WithCondition("attribute_not_exists(#key)"),
		dynamodb.WithExpressionNames(map[string]string{"#key": bootstrapKeyAttribute}))
	switch {
	case errors.Is(err, dynamodb.ErrConditionalCheck):
		return errors.New("auth: bootstrap login ID is already issued")
	case err != nil:
		return fmt.Errorf("auth: issue bootstrap credential: %w", err)
	}
	return nil
}

// Find returns an unconsumed credential.
//
// An unknown login ID and a consumed one return the same error, so a caller
// cannot tell them apart and enumerate accounts. The read is consistent,
// because a credential issued moments ago must be redeemable immediately.
func (Bootstrap) Find(ctx context.Context, loginID string) (auth.BootstrapCredential, error) {
	if loginID == "" {
		return auth.BootstrapCredential{}, auth.ErrUnknownBootstrap
	}
	client, table, err := resolve(ctx, DeclaredBootstrapTable)
	if err != nil {
		return auth.BootstrapCredential{}, err
	}
	item, err := client.GetItem(ctx, table,
		dynamodb.Key{bootstrapKeyAttribute: dynamodb.S(loginID)},
		dynamodb.WithConsistentRead(true))
	switch {
	case errors.Is(err, dynamodb.ErrItemNotFound):
		return auth.BootstrapCredential{}, auth.ErrUnknownBootstrap
	case err != nil:
		return auth.BootstrapCredential{}, fmt.Errorf("auth: read bootstrap credential: %w", err)
	}
	if consumed := readEpoch(item, consumedAtAttribute); !consumed.IsZero() {
		return auth.BootstrapCredential{}, auth.ErrUnknownBootstrap
	}
	return auth.BootstrapCredential{
		LoginID:           readString(item, bootstrapKeyAttribute),
		AccountID:         readString(item, bootstrapAccountAttribute),
		SecretDigest:      readBytes(item, secretDigestAttribute),
		Purpose:           readString(item, purposeAttribute),
		IssuedAt:          readEpoch(item, issuedAtAttribute),
		ExpiresAt:         readEpoch(item, bootstrapExpiresAtAttrib),
		AttemptsRemaining: int(readInt(item, attemptsRemainingAttribute)),
	}, nil
}

// RecordAttempt spends one attempt and reports what is left.
//
// It is one request. The contract requires that two parallel guesses cannot
// both spend the last attempt, and a read followed by a write cannot promise
// that: both readers would see the same budget and both would decide they had
// one. ADD applies the decrement on the server, and the condition makes it
// conditional on there being something to spend.
func (Bootstrap) RecordAttempt(ctx context.Context, loginID string) (int, error) {
	if loginID == "" {
		return 0, auth.ErrUnknownBootstrap
	}
	client, table, err := resolve(ctx, DeclaredBootstrapTable)
	if err != nil {
		return 0, err
	}
	result, err := client.UpdateItem(ctx, table,
		dynamodb.Key{bootstrapKeyAttribute: dynamodb.S(loginID)},
		"ADD #attempts :spend",
		dynamodb.WithCondition("attribute_exists(#key) AND attribute_not_exists(#consumed) AND #attempts > :zero"),
		dynamodb.WithExpressionNames(map[string]string{
			"#key":      bootstrapKeyAttribute,
			"#attempts": attemptsRemainingAttribute,
			"#consumed": consumedAtAttribute,
		}),
		dynamodb.WithExpressionValues(map[string]dynamodb.AttributeValue{
			":spend": dynamodb.N(-1),
			":zero":  dynamodb.N(0),
		}),
		dynamodb.WithReturnValues("ALL_NEW"))
	switch {
	case errors.Is(err, dynamodb.ErrConditionalCheck):
		// An exhausted budget, a consumed credential, and an unknown login ID
		// are all this one error, per the contract.
		return 0, auth.ErrUnknownBootstrap
	case err != nil:
		return 0, fmt.Errorf("auth: record bootstrap attempt: %w", err)
	}
	if result == nil {
		return 0, errors.New("auth: record bootstrap attempt: empty response")
	}
	remaining, held := result.Attributes[attemptsRemainingAttribute].AsInt()
	if !held {
		return 0, errors.New("auth: record bootstrap attempt: no remaining count returned")
	}
	return int(remaining), nil
}

// Consume marks the credential spent. It is step one of the registration
// sequence, so the condition is what makes that sequence single-use: a second
// redemption of one issued secret fails here, before any credential is written.
func (Bootstrap) Consume(ctx context.Context, loginID string, at time.Time) error {
	if loginID == "" {
		return auth.ErrUnknownBootstrap
	}
	client, table, err := resolve(ctx, DeclaredBootstrapTable)
	if err != nil {
		return err
	}
	_, err = client.UpdateItem(ctx, table,
		dynamodb.Key{bootstrapKeyAttribute: dynamodb.S(loginID)},
		"SET #consumed = :at",
		dynamodb.WithCondition("attribute_exists(#key) AND attribute_not_exists(#consumed)"),
		dynamodb.WithExpressionNames(map[string]string{
			"#key":      bootstrapKeyAttribute,
			"#consumed": consumedAtAttribute,
		}),
		dynamodb.WithExpressionValues(map[string]dynamodb.AttributeValue{":at": epoch(at)}))
	switch {
	case errors.Is(err, dynamodb.ErrConditionalCheck):
		return auth.ErrUnknownBootstrap
	case err != nil:
		return fmt.Errorf("auth: consume bootstrap credential: %w", err)
	}
	return nil
}
