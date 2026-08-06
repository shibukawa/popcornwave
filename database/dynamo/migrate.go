package dynamo

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/shibukawa/tinybind-go/dynamobind"
	"github.com/shibukawa/tinygodriver/nosql/dynamodb"
)

// Change is what one table needs, decided by comparing the registered
// definition against what the account reports.
type Change string

const (
	// ChangeNone means the deployed table already matches.
	ChangeNone Change = "none"
	// ChangeCreate means the table is absent.
	ChangeCreate Change = "create"
	// ChangeMismatch means the table exists with a key schema this build does
	// not expect. DynamoDB cannot alter a key, so it is reported and never
	// performed.
	ChangeMismatch Change = "mismatch"
)

// TableChange is one entry of a plan.
type TableChange struct {
	// Declared is the name source uses.
	Declared string
	// Deployed is the name the resolver produced.
	Deployed string
	// Change is what this table needs.
	Change Change
	// Detail explains a mismatch. It is empty otherwise.
	Detail string
}

// Result reports what an apply did.
type Result struct {
	// Created lists the deployed names created, in plan order.
	Created []string
	// Unchanged lists the deployed names that already matched.
	Unchanged []string
}

// activePollInterval bounds how often a create waits on DescribeTable. The
// driver ships no waiter, so polling is the caller's loop.
const activePollInterval = 200 * time.Millisecond

// Plan reports what schema application would do, without sending a write.
//
// It is the production-relevant entry: deployment tooling knows what it
// created, and only the application knows what it expects, so comparing the two
// is a question nothing else can answer.
func Plan(ctx context.Context) ([]TableChange, error) {
	client, resolver, err := planInputs(ctx)
	if err != nil {
		return nil, err
	}
	return planWith(ctx, client, resolver)
}

// Migrate creates the tables that are missing and leaves everything else alone.
//
// It never deletes, never alters, and never reads the operational surface a
// deployment owns, so the only outcome besides success is a report.
func Migrate(ctx context.Context) (Result, error) {
	client, resolver, err := planInputs(ctx)
	if err != nil {
		return Result{}, err
	}
	plan, err := planWith(ctx, client, resolver)
	if err != nil {
		return Result{}, err
	}
	return applyPlan(ctx, client, plan)
}

// planInputs resolves the client and the naming from the process handle, or
// from a handle a test installed on ctx.
func planInputs(ctx context.Context) (*dynamodb.Client, TableResolver, error) {
	handle, err := Handle(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf(
			"popcornwave/database/dynamo: no client available; import the package and enable middleware.dynamo: %w", err)
	}
	return handle.Client(), tableNaming(handle), nil
}

// tableNaming adapts the naming carried by handle to the TableResolver shape
// the planner uses.
func tableNaming(handle dynamobind.Handle) TableResolver {
	return func(ctx context.Context, declared string) string {
		if _, deployed, err := handle.Table(ctx, declared); err == nil {
			return deployed
		}
		return declared
	}
}

// planWith compares every registered table against the account.
func planWith(ctx context.Context, client *dynamodb.Client, resolver TableResolver) ([]TableChange, error) {
	declaredNames := registeredTables()
	plan := make([]TableChange, 0, len(declaredNames))
	for _, declared := range declaredNames {
		factory, ok := tableFactory(declared)
		if !ok {
			continue
		}
		deployed := resolve(ctx, resolver, declared)
		if err := validateResolvedName(declared, deployed); err != nil {
			return nil, err
		}
		want := factory(deployed)
		change, err := compareTable(ctx, client, want)
		if err != nil {
			return nil, err
		}
		change.Declared = declared
		change.Deployed = deployed
		plan = append(plan, change)
	}
	return plan, nil
}

// compareTable reads one table and classifies it.
func compareTable(ctx context.Context, client *dynamodb.Client, want dynamodb.TableDefinition) (TableChange, error) {
	described, err := client.DescribeTable(ctx, want.Name)
	switch {
	case errors.Is(err, dynamodb.ErrTableNotFound), errors.Is(err, dynamodb.ErrResourceNotFound):
		return TableChange{Change: ChangeCreate}, nil
	case err != nil:
		return TableChange{}, fmt.Errorf("describing table %q: %w", want.Name, err)
	}
	if detail := keyScheduleMismatch(want, described); detail != "" {
		return TableChange{Change: ChangeMismatch, Detail: detail}, nil
	}
	return TableChange{Change: ChangeNone}, nil
}

// keyScheduleMismatch compares the key attributes and returns a description of
// the difference, or the empty string when they agree.
//
// Only the names are compared, and only positionally. DescribeTable reports a
// key attribute's name and nothing else: the driver drops the HASH and RANGE
// roles it receives, and DynamoDB reports attribute types in a part of the
// reply the driver does not decode at all. So a table whose partition key is
// named as expected but typed differently reads as matching here. That is a
// gap in what can be checked, not a decision to check less.
func keyScheduleMismatch(want dynamodb.TableDefinition, described *dynamodb.TableDescription) string {
	expected := []string{want.PartitionKey.Name}
	if want.SortKey != nil {
		expected = append(expected, want.SortKey.Name)
	}
	observed := make([]string, 0, len(described.Keys))
	for _, key := range described.Keys {
		observed = append(observed, key.Name)
	}
	if len(expected) != len(observed) {
		return fmt.Sprintf("this build expects key %s, and the deployed table has key %s",
			joinKeys(expected), joinKeys(observed))
	}
	for i := range expected {
		if expected[i] != observed[i] {
			return fmt.Sprintf("this build expects key %s, and the deployed table has key %s",
				joinKeys(expected), joinKeys(observed))
		}
	}
	return ""
}

func joinKeys(names []string) string {
	if len(names) == 0 {
		return "(none)"
	}
	return "(" + strings.Join(names, ", ") + ")"
}

// applyPlan creates what the plan says is missing. A mismatch stops the run
// before any write, and every offending table is named rather than only the
// first, so one run shows the whole problem.
func applyPlan(ctx context.Context, client *dynamodb.Client, plan []TableChange) (Result, error) {
	var blocked []string
	for _, change := range plan {
		if change.Change == ChangeMismatch {
			blocked = append(blocked, fmt.Sprintf("%s: %s", change.Deployed, change.Detail))
		}
	}
	if len(blocked) > 0 {
		return Result{}, fmt.Errorf(
			"popcornwave/database/dynamo: %d table(s) cannot be reconciled, and DynamoDB cannot alter a key schema:\n  %s",
			len(blocked), strings.Join(blocked, "\n  "))
	}

	var result Result
	for _, change := range plan {
		if change.Change != ChangeCreate {
			result.Unchanged = append(result.Unchanged, change.Deployed)
			continue
		}
		factory, ok := tableFactory(change.Declared)
		if !ok {
			continue
		}
		definition := factory(change.Deployed)
		// The definition carries the keys and nothing else. Billing and
		// capacity are a deployment's, so the driver default of on-demand is
		// what a development table gets.
		if err := client.CreateTable(ctx, definition); err != nil {
			if !errors.Is(err, dynamodb.ErrTableInUse) {
				return result, fmt.Errorf("creating table %q: %w", change.Deployed, err)
			}
			// Another migrator won the race. Creating is the only mutation
			// here and it is idempotent, so this is "already applied".
		}
		if err := waitActive(ctx, client, change.Deployed); err != nil {
			return result, err
		}
		result.Created = append(result.Created, change.Deployed)
	}
	return result, nil
}

// waitActive polls until the table is usable. The driver ships no waiter.
func waitActive(ctx context.Context, client *dynamodb.Client, table string) error {
	ticker := time.NewTicker(activePollInterval)
	defer ticker.Stop()
	for {
		described, err := client.DescribeTable(ctx, table)
		if err == nil && described.Active() {
			return nil
		}
		if err != nil && !errors.Is(err, dynamodb.ErrTableNotFound) && !errors.Is(err, dynamodb.ErrResourceNotFound) {
			return fmt.Errorf("waiting for table %q: %w", table, err)
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("waiting for table %q: %w", table, ctx.Err())
		case <-ticker.C:
		}
	}
}

// verify reports the first table that is missing or mismatched, for the startup
// check. It creates nothing.
func verify(ctx context.Context, client *dynamodb.Client, resolver TableResolver) error {
	plan, err := planWith(ctx, client, resolver)
	if err != nil {
		return err
	}
	var problems []string
	for _, change := range plan {
		switch change.Change {
		case ChangeCreate:
			problems = append(problems, fmt.Sprintf(
				"%s is absent; create it with your deployment tooling, or with pw migrate up in development",
				change.Deployed))
		case ChangeMismatch:
			problems = append(problems, fmt.Sprintf("%s: %s", change.Deployed, change.Detail))
		}
	}
	if len(problems) > 0 {
		return fmt.Errorf("popcornwave/database/dynamo: schema does not match this build:\n  %s",
			strings.Join(problems, "\n  "))
	}
	return nil
}
