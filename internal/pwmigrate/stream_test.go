package pwmigrate

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"testing/fstest"

	_ "github.com/shibukawa/popcornweb/database/sqlite"
)

func streamTarget(t *testing.T) *Target {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return AttachSQLite(db)
}

func streamFS(statements ...string) fstest.MapFS {
	sources := fstest.MapFS{}
	for index, statement := range statements {
		name := "0000" + string(rune('1'+index)) + "_step.sql"
		sources[name] = &fstest.MapFile{Data: []byte(
			"-- +goose Up\n" + statement + "\n",
		)}
	}
	return sources
}

func TestStreamTableCarriesTheFrameworkPrefix(t *testing.T) {
	// An application reading its own schema can tell at a glance which tables it
	// does not own, which is the same rule every framework table follows.
	if table := StreamTable("widget"); table != "popcornweb_migrations_widget" {
		t.Fatalf("table = %q", table)
	}
	// The application's own stream keeps goose's default table, so an existing
	// project's recorded versions stay where they are.
	if table := StreamTable(""); table != "" {
		t.Fatalf("table = %q, want the goose default", table)
	}
}

func TestApplyStreamsKeepsSeparateLedgers(t *testing.T) {
	target := streamTarget(t)
	ctx := context.Background()
	streams := []Stream{
		{Module: "example.com/one", Stem: "one", Sources: streamFS("create table one_thing (id integer primary key);")},
		// Both packages number their first migration 1. Separate ledgers are what
		// keeps the second from reading the first's version as its own and
		// skipping its only migration.
		{Module: "example.com/two", Stem: "two", Sources: streamFS("create table two_thing (id integer primary key);")},
	}
	results, err := ApplyStreams(ctx, target, streams)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("results = %#v", results)
	}
	for _, result := range results {
		if result.Result.Current != 1 || len(result.Result.Applied) != 1 {
			t.Fatalf("%s applied nothing: %#v", result.Module, result.Result)
		}
	}
	for _, table := range []string{"one_thing", "two_thing"} {
		var name string
		err := target.DB.QueryRowContext(ctx,
			"select name from sqlite_master where type='table' and name=?", table).Scan(&name)
		if err != nil {
			t.Fatalf("%s was not created: %v", table, err)
		}
	}
	// Each stream keeps its own version table, named after its stem.
	for _, table := range []string{"popcornweb_migrations_one", "popcornweb_migrations_two"} {
		var name string
		if err := target.DB.QueryRowContext(ctx,
			"select name from sqlite_master where type='table' and name=?", table).Scan(&name); err != nil {
			t.Fatalf("%s is missing: %v", table, err)
		}
	}
}

func TestApplyStreamsIsIdempotent(t *testing.T) {
	target := streamTarget(t)
	ctx := context.Background()
	streams := []Stream{
		{Module: "example.com/one", Stem: "one", Sources: streamFS("create table one_thing (id integer primary key);")},
	}
	if _, err := ApplyStreams(ctx, target, streams); err != nil {
		t.Fatal(err)
	}
	// An upgrade re-runs this, so a stream with nothing new must apply nothing
	// rather than fail on a table that already exists.
	results, err := ApplyStreams(ctx, target, streams)
	if err != nil {
		t.Fatal(err)
	}
	if len(results[0].Result.Applied) != 0 {
		t.Fatalf("a second run applied %#v", results[0].Result.Applied)
	}
}

func TestApplyStreamsAppliesLaterVersionsOnUpgrade(t *testing.T) {
	target := streamTarget(t)
	ctx := context.Background()
	first := []Stream{
		{Module: "example.com/one", Stem: "one", Sources: streamFS("create table one_thing (id integer primary key);")},
	}
	if _, err := ApplyStreams(ctx, target, first); err != nil {
		t.Fatal(err)
	}
	// go get -u then pw migrate: the version added since the install is pending
	// in that package's stream and applies in order, with nothing copied.
	upgraded := []Stream{{
		Module:  "example.com/one",
		Stem:    "one",
		Sources: streamFS("create table one_thing (id integer primary key);", "create table one_more (id integer primary key);"),
	}}
	results, err := ApplyStreams(ctx, target, upgraded)
	if err != nil {
		t.Fatal(err)
	}
	if len(results[0].Result.Applied) != 1 || results[0].Result.Current != 2 {
		t.Fatalf("upgrade applied %#v", results[0].Result)
	}
}

func TestApplyStreamsStopsAtTheFailingPackage(t *testing.T) {
	target := streamTarget(t)
	ctx := context.Background()
	streams := []Stream{
		{Module: "example.com/one", Stem: "one", Sources: streamFS("create table one_thing (id integer primary key);")},
		{Module: "example.com/two", Stem: "two", Sources: streamFS("this is not sql;")},
	}
	results, err := ApplyStreams(ctx, target, streams)
	if err == nil {
		t.Fatal("a broken stream was accepted")
	}
	// The failure names the package, because a project installing several has no
	// other way to tell which one broke.
	if !strings.Contains(err.Error(), "example.com/two") {
		t.Fatalf("err = %v, want the package named", err)
	}
	// The stream that already applied stays applied: a package's schema is its
	// own unit, and rolling it back because a later package failed would leave a
	// working one half removed.
	if len(results) == 0 || results[0].Result.Current != 1 {
		t.Fatalf("results = %#v", results)
	}
}

func TestStreamPendingListsWhatWouldApply(t *testing.T) {
	target := streamTarget(t)
	ctx := context.Background()
	stream := Stream{
		Module:  "example.com/one",
		Stem:    "one",
		Sources: streamFS("create table one_thing (id integer primary key);"),
	}
	// Listing pending statements before applying them is what replaces the
	// review a migration copied into the project used to get.
	pending, err := StreamPending(ctx, target, stream)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 {
		t.Fatalf("pending = %#v", pending)
	}
	if _, err := ApplyStreams(ctx, target, []Stream{stream}); err != nil {
		t.Fatal(err)
	}
	pending, err = StreamPending(ctx, target, stream)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 0 {
		t.Fatalf("pending after apply = %#v", pending)
	}
}
