package pw

import (
	"io/fs"
	"testing"
	"testing/fstest"
)

func TestRegisterPublicFS(t *testing.T) {
	publicFSState.Lock()
	previous := publicFSState.value
	publicFSState.value = nil
	publicFSState.Unlock()
	t.Cleanup(func() {
		publicFSState.Lock()
		publicFSState.value = previous
		publicFSState.Unlock()
	})

	expected := fstest.MapFS{"app.css": {Data: []byte("body{}")}}
	RegisterPublicFS(expected)
	actual := registeredPublicFS()
	if actual == nil {
		t.Fatal("registered public filesystem is nil")
	}
	if _, err := fs.ReadFile(actual, "app.css"); err != nil {
		t.Fatal(err)
	}
}
