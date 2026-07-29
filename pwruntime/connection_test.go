package pwruntime

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	_ "github.com/shibukawa/tinygodriver/database/sql/sqlite"
)

// newGroupedDB opens one sqlite file per named connection and returns a context
// carrying the resulting set. Every connection gets the same schema, so a test
// can tell them apart by what it writes into each.
func newGroupedDB(t *testing.T, defaultGroup string, specs ...Connection) (*ConnectionSet, context.Context) {
	t.Helper()
	dir := t.TempDir()
	connections := make([]Connection, 0, len(specs))
	for index, spec := range specs {
		db, err := sql.Open("sqlite", filepath.Join(dir, spec.Label+".db"))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = db.Close() })
		if _, err := db.ExecContext(context.Background(), "CREATE TABLE items (name TEXT NOT NULL)"); err != nil {
			t.Fatal(err)
		}
		spec.DB = db
		if spec.Driver == "" {
			spec.Driver = "sqlite"
		}
		_ = index
		connections = append(connections, spec)
	}
	set, err := NewConnectionSet(defaultGroup, connections)
	if err != nil {
		t.Fatal(err)
	}
	return set, WithResources(context.Background(), Resources{Connections: set})
}

func TestConnectionSetRequiresDefaultGroupWithSeveralGroups(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "a.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := NewConnectionSet("", []Connection{
		{DB: db, Group: "writer"},
		{DB: db, Group: "replica"},
	}); err == nil {
		t.Fatal("two groups without default_group were accepted")
	}
	if _, err := NewConnectionSet("nowhere", []Connection{{DB: db, Group: "writer"}}); !errors.Is(err, ErrUnknownConnectionGroup) {
		t.Fatalf("err = %v, want ErrUnknownConnectionGroup", err)
	}
}

func TestConnectionSetRoundRobinAlternates(t *testing.T) {
	set, _ := newGroupedDB(t, "replica",
		Connection{Group: "replica", Label: "replica#1", ReadOnly: true},
		Connection{Group: "replica", Label: "replica#2", ReadOnly: true},
	)
	var picked []string
	for i := 0; i < 4; i++ {
		connection, err := set.pick("replica")
		if err != nil {
			t.Fatal(err)
		}
		picked = append(picked, connection.Label)
	}
	want := []string{"replica#1", "replica#2", "replica#1", "replica#2"}
	for index := range want {
		if picked[index] != want[index] {
			t.Fatalf("picks = %v, want %v", picked, want)
		}
	}
}

func TestConnectionMemoKeepsOneConnectionPerRequest(t *testing.T) {
	set, ctx := newGroupedDB(t, "replica",
		Connection{Group: "replica", Label: "replica#1", ReadOnly: true},
		Connection{Group: "replica", Label: "replica#2", ReadOnly: true},
	)
	first, err := resources(ctx).connection()
	if err != nil {
		t.Fatal(err)
	}
	second, err := resources(ctx).connection()
	if err != nil {
		t.Fatal(err)
	}
	if first.Label != second.Label {
		t.Fatalf("one request straddled %s and %s", first.Label, second.Label)
	}
	// A different request chain gets the next connection, so the group is still
	// balanced across requests.
	other := WithResources(context.Background(), Resources{Connections: set})
	third, err := resources(other).connection()
	if err != nil {
		t.Fatal(err)
	}
	if third.Label == first.Label {
		t.Fatalf("a second request reused %s instead of advancing", third.Label)
	}
}

func TestUnknownGroupFailsAtTheStatement(t *testing.T) {
	_, ctx := newGroupedDB(t, "replica",
		Connection{Group: "replica", Label: "replica#1", ReadOnly: true},
		Connection{Group: "writer", Label: "writer#1"},
	)
	pinned := SelectDB(ctx, "analytics")
	if _, err := SQLExecutor(pinned); !errors.Is(err, ErrUnknownConnectionGroup) {
		t.Fatalf("err = %v, want ErrUnknownConnectionGroup", err)
	}
	if _, ok := DB(pinned); ok {
		t.Fatal("DB answered for an unknown group")
	}
}

func TestSelectDBRoutesToTheNamedGroup(t *testing.T) {
	_, ctx := newGroupedDB(t, "replica",
		Connection{Group: "replica", Label: "replica#1", ReadOnly: true},
		Connection{Group: "writer", Label: "writer#1"},
	)
	writerCtx := SelectDB(ctx, "writer")
	insert(t, writerCtx, "from-writer")

	writer, _ := DB(writerCtx)
	equal(t, names(t, writer), []string{"from-writer"})
	replica, _ := DB(ctx)
	equal(t, names(t, replica), nil)
}

func TestTransactionOnGroupKeepsUnpinnedStatementsOnThatGroup(t *testing.T) {
	_, ctx := newGroupedDB(t, "replica",
		Connection{Group: "replica", Label: "replica#1", ReadOnly: true},
		Connection{Group: "writer", Label: "writer#1"},
	)
	err := Transaction(ctx, func(ctx context.Context) error {
		// No pin inside: the transaction group has to outrank default_group,
		// otherwise this write would land on the replica.
		insert(t, ctx, "in-writer-tx")
		return nil
	}, OnGroup("writer"))
	if err != nil {
		t.Fatal(err)
	}
	writer, _ := DB(SelectDB(ctx, "writer"))
	equal(t, names(t, writer), []string{"in-writer-tx"})
}

func TestNestedTransactionRejectsAnotherGroup(t *testing.T) {
	_, ctx := newGroupedDB(t, "replica",
		Connection{Group: "replica", Label: "replica#1", ReadOnly: true},
		Connection{Group: "writer", Label: "writer#1"},
	)
	err := Transaction(ctx, func(ctx context.Context) error {
		inner := Transaction(ctx, func(context.Context) error {
			t.Fatal("the cross-group callback ran")
			return nil
		}, OnGroup("replica"))
		if !errors.Is(inner, ErrCrossGroupTransaction) {
			t.Fatalf("inner err = %v, want ErrCrossGroupTransaction", inner)
		}
		// The outer transaction stays usable after the rejection.
		insert(t, ctx, "still-usable")
		return nil
	}, OnGroup("writer"))
	if err != nil {
		t.Fatal(err)
	}
	writer, _ := DB(SelectDB(ctx, "writer"))
	equal(t, names(t, writer), []string{"still-usable"})
}

func TestSelectDBInsideTransactionReadsReplicaAndRejectsWriter(t *testing.T) {
	_, ctx := newGroupedDB(t, "replica",
		Connection{Group: "replica", Label: "replica#1", ReadOnly: true},
		Connection{Group: "writer", Label: "writer#1"},
		Connection{Group: "reporting", Label: "reporting#1"},
	)
	err := Transaction(ctx, func(ctx context.Context) error {
		// A readonly group is reachable: the read simply happens outside the
		// transaction.
		if _, err := SQLExecutor(SelectDB(ctx, "replica")); err != nil {
			t.Fatalf("readonly escape failed: %v", err)
		}
		// A writable group is not, because the write would look atomic and
		// would not be.
		if _, err := SQLExecutor(SelectDB(ctx, "reporting")); err == nil {
			t.Fatal("a writable group was selectable inside a transaction")
		}
		return nil
	}, OnGroup("writer"))
	if err != nil {
		t.Fatal(err)
	}
}

func TestReadOnlyConnectionBeginsReadOnlyTransaction(t *testing.T) {
	_, ctx := newGroupedDB(t, "replica",
		Connection{Group: "replica", Label: "replica#1", ReadOnly: true},
		Connection{Group: "writer", Label: "writer#1"},
	)
	err := Transaction(ctx, func(ctx context.Context) error {
		if !resources(ctx).TxScope.ReadOnly() {
			t.Fatal("a readonly connection opened a writable transaction")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCollapsedSetAnswersEveryGroupName(t *testing.T) {
	// A single-database configuration, and a test, must run code that selects a
	// replica group without a dedicated branch.
	db, ctx := newTestDB(t, "sqlite")
	insert(t, SelectDB(ctx, "replica"), "collapsed")
	err := Transaction(SelectDB(ctx, "writer"), func(ctx context.Context) error {
		insert(t, SelectDB(ctx, "replica"), "still-collapsed")
		return nil
	}, OnGroup("writer"))
	if err != nil {
		t.Fatal(err)
	}
	equal(t, names(t, db), []string{"collapsed", "still-collapsed"})
}
