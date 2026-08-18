package pw

import "github.com/shibukawa/popcornweb/pwconfig"

// RegisterSubCommand registers typed CLI-only input.
//
// The registry is popcornweb/pwconfig's, because a subcommand is a fact about
// the command line rather than about a transport: an application that added one
// has it in both of its builds.
func RegisterSubCommand[T any](name, help string) { pwconfig.RegisterSubCommand[T](name, help) }

// SubCommand is retained as a compatibility alias.
// Deprecated: use RegisterSubCommand.
func SubCommand[T any](name, help string) { pwconfig.RegisterSubCommand[T](name, help) }

// Command returns the selected and parsed command after ParseConfig.
func Command[T any]() (T, bool) { return pwconfig.Command[T]() }
