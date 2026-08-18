package dynamo

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/shibukawa/popcornweb/pw"
	"github.com/shibukawa/tinybind-go/dynamobind"
	"github.com/shibukawa/tinygodriver/cloud/aws"
	"github.com/shibukawa/tinygodriver/nosql/dynamodb"
)

func TestImportRegistersTheBackend(t *testing.T) {
	// The point of the import is that session.backend = "dynamo" resolves.
	// Registering twice is a panic, so reaching here at all proves the init
	// ran exactly once.
	if pw.SessionBackendDynamo != "dynamo" {
		t.Fatalf("backend name = %q", pw.SessionBackendDynamo)
	}
	// Resolving the backend by name is the whole registration contract. It
	// fails on the missing client rather than on an unknown backend, which is
	// how an unregistered name would have failed.
	_, err := pw.OpenSessionBackend(context.Background(),
		pw.SessionConfig{Backend: pw.SessionBackendDynamo}, pw.SessionResources{})
	if err == nil {
		t.Fatal("opening without a client must fail")
	}
	if strings.Contains(err.Error(), "import") && strings.Contains(err.Error(), "sessionstore/dynamo") {
		t.Fatalf("the backend was not registered by this package's init: %v", err)
	}
}

func TestOpenRefusesWithoutTheClient(t *testing.T) {
	// The client belongs to database/dynamo. Refusing at startup is what keeps
	// the first login from being the thing that discovers a missing import.
	_, err := open(context.Background(), pw.SessionConfig{}, pw.SessionResources{})
	if err == nil {
		t.Fatal("a backend with no client must refuse to open")
	}
	if !strings.Contains(err.Error(), "middleware.dynamo.enabled") {
		t.Fatalf("error must name what to enable, got %v", err)
	}
}

func TestOpenBorrowsTheClientAndSweepsNothing(t *testing.T) {
	server := httptest.NewServer(newFakeItems())
	defer server.Close()
	client, err := dynamodb.New(
		dynamodb.WithEndpoint(server.URL),
		dynamodb.WithRegion("ap-northeast-1"),
		dynamodb.WithCredentials(aws.Credentials{AccessKeyID: "test", SecretAccessKey: "test"}),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = client.Close() }()
	ctx := dynamobind.WithClient(context.Background(), client)

	backend, err := open(ctx, pw.SessionConfig{}, pw.SessionResources{})
	if err != nil {
		t.Fatal(err)
	}
	if backend.Store == nil {
		t.Fatal("the backend must carry a store")
	}
	// What it did not open, it does not close.
	if backend.Close != nil {
		t.Fatal("a borrowed client must not be closed by this backend")
	}
	// Nothing accumulates that a sweep would remove: expiry is decided on read
	// and the bytes go away through TTL.
	if backend.Prune != nil {
		t.Fatal("this backend schedules no sweep")
	}
}
