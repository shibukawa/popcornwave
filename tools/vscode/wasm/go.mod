// A module of its own, so `go build ./...` and the TinyGo matrix at the
// repository root never compile the extension's WebAssembly entry.
//
// The tinybind-go version here is the formatter the extension ships and is
// deliberately independent of the version the framework pins: an extension
// release must not require a framework release. decision:formatter-delivery
// carries what that costs and how the delegated path removes it.
module github.com/shibukawa/popcornwave/tools/vscode/wasm

go 1.26.0

require github.com/shibukawa/tinybind-go v0.3.2
