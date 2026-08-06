package pwruntime

import (
	"context"
	"testing"
)

func TestCapsuleParentChain(t *testing.T) {
	ctx := WithResources(context.Background(), Resources{})
	root := resources(ctx)
	if root.Parent() != nil {
		t.Fatal("request root capsule has a parent")
	}

	ctx = SelectDB(ctx, "reader")
	pinned := resources(ctx)
	if pinned.Parent() != root {
		t.Fatal("SelectDB capsule does not point at the capsule it was derived from")
	}

	ctx = WithLogAttributes(ctx, String("request_id", "r-1"))
	tagged := resources(ctx)
	if tagged.Parent() != pinned || tagged.Parent().Parent() != root {
		t.Fatal("capsule ancestry is not a pointer chain back to the root")
	}
	if tagged.Group != "reader" {
		t.Fatalf("derived capsule lost its state, group = %q", tagged.Group)
	}

	if (*Resources)(nil).Parent() != nil {
		t.Fatal("nil capsule Parent is not nil-safe")
	}
}
