package pwcli

import (
	"bytes"
	"strings"
	"testing"
)

func TestRenameNeedsBothNames(t *testing.T) {
	for _, args := range [][]string{{}, {"OnlyOne"}, {"A", "B", "C"}} {
		if _, _, _, err := parseRenameOptions(args); err == nil {
			t.Errorf("%v was accepted", args)
		}
	}
}

func TestRenameRejectsAnUnknownFlag(t *testing.T) {
	if _, _, _, err := parseRenameOptions([]string{"A", "B", "--force"}); err == nil {
		t.Fatal("an unknown flag was accepted")
	}
}

func TestRenamePreviewsUnlessAsked(t *testing.T) {
	// The set reaches handwritten Go and files the developer never opened, so
	// the preview is the default and --apply is the second look.
	from, to, apply, err := parseRenameOptions([]string{"Old", "New"})
	if err != nil {
		t.Fatal(err)
	}
	if from != "Old" || to != "New" || apply {
		t.Fatalf("options = %q %q %v", from, to, apply)
	}

	if _, _, apply, err = parseRenameOptions([]string{"Old", "New", "--apply"}); err != nil || !apply {
		t.Fatalf("--apply = %v, err = %v", apply, err)
	}
}

// A command missing from the help table is one nobody discovers.
func TestRenameIsRegisteredInTheCommandList(t *testing.T) {
	var usage bytes.Buffer
	printUsage(&usage)

	found := false
	for _, command := range commandSummaries {
		if command.name == "rename" {
			found = command.summary != ""
		}
	}
	if !found {
		t.Fatal("rename is missing from commandSummaries, so pw help omits it")
	}
	if !strings.Contains(usage.String(), renameUsage) {
		t.Fatal("pw help does not print the rename usage line")
	}

	var out, errOut bytes.Buffer
	if status := Main([]string{"rename", "--nonsense"}, &out, &errOut); status == 0 {
		t.Fatal("the dispatcher did not reach rename: a bad argument exited 0")
	}
}
