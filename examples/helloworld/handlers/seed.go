package handlers

import "github.com/shibukawa/popcornweb/pwconfig"

// SeedCommand is this application's own subcommand: typed CLI-only input,
// named on the command line and parsed like any other binding.
//
// It registers through the shared configuration layer rather than through a
// runtime, because a subcommand is a fact about the command line: this file is
// compiled by both builds, and naming one runtime would put it in only one of
// them.
type SeedCommand struct {
	Count int    `default:"10" help:"rows to write"`
	Table string `default:"users" help:"table to write them into"`
}

// RegisterSeed declares the command. Call it from main before Run, beside the
// configuration registration.
func RegisterSeed() { pwconfig.RegisterSubCommand[SeedCommand]("seed", "write starter rows") }

// Seed returns the parsed command when this invocation named it.
func Seed() (SeedCommand, bool) { return pwconfig.Command[SeedCommand]() }
