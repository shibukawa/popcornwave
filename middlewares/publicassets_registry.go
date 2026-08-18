package middlewares

import (
	"io/fs"
	"sync"
)

var publicFSState = struct {
	sync.RWMutex
	value fs.FS
}{}

// RegisterPublicFS installs the application's embedded public filesystem.
// Generated project public.go files call this during package initialization.
func RegisterPublicFS(value fs.FS) {
	if value == nil {
		panic("popcornweb: nil public filesystem")
	}
	publicFSState.Lock()
	defer publicFSState.Unlock()
	if publicFSState.value != nil {
		panic("popcornweb: public filesystem is already registered")
	}
	publicFSState.value = value
}

func registeredPublicFS() fs.FS {
	publicFSState.RLock()
	defer publicFSState.RUnlock()
	return publicFSState.value
}
