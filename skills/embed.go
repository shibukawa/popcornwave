// Package skills carries the agent skill sources this repository publishes.
//
// The tree is authored here so it is installable straight from the repository
// by skill managers, and it is embedded so pw init can place a copy into a new
// project without reaching for the network.
package skills

import "embed"

// PopcornWeb is the framework-usage skill: the guideline an AI coding agent
// loads to write templates, queries, and configuration in a Popcorn Web
// project, and the pw commands it checks its work with.
//
//go:embed all:popcornweb-skill
var PopcornWeb embed.FS
