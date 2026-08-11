package pw

import "github.com/shibukawa/popcornwave/pwconfig"

// The command line is popcornwave/pwconfig's, and so is what the framework's
// own arguments mean. Both builds of one application therefore answer the same
// words: --generate-config, the health probe, and whatever subcommands the
// application registered.
//
// What each runtime keeps is when it answers them, which is a property of its
// own lifecycle rather than of the words.

func parseFrameworkAction(args []string) ([]string, error) {
	return pwconfig.ParseFrameworkAction(args)
}

func refusePendingFrameworkAction() error { return pwconfig.RefusePendingFrameworkAction() }

func runFrameworkAction() (bool, error) { return pwconfig.RunFrameworkAction() }
