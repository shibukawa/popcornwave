package pwconfig

// The DSN check resolves a scheme onto the engine that opens it, and an engine
// nothing linked is refused by name. These tests write sqlite DSNs, so the test
// binary links the engine an application would.
import _ "github.com/shibukawa/popcornweb/database/sqlite"
