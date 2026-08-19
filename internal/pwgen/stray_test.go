package pwgen

import (
	"strings"
	"testing"
)

func TestATemplateInsideItsPurposeIsNotStray(t *testing.T) {
	for _, testCase := range []struct {
		name     string
		purposes SourcePurposes
	}{
		{"card.pw.html", SourcePurposes{Templates: true}},
		{"rooms.pw.sql", SourcePurposes{Queries: true}},
		{"readings.pw.dynamo", SourcePurposes{Dynamo: true}},
		{"orders.pw.firestore", SourcePurposes{Firestore: true}},
		{PageFile, SourcePurposes{Pages: true}},
		{LayoutFile, SourcePurposes{Pages: true}},
		{DocumentFile, SourcePurposes{Pages: true}},
	} {
		if message, stray := StrayTemplateMessage("dir/"+testCase.name, testCase.name, testCase.purposes); stray {
			t.Errorf("%s is reported stray: %s", testCase.name, message)
		}
	}
}

func TestASourceOutsideItsPurposeNamesTheKeyToEdit(t *testing.T) {
	// The reader fixes this by listing the directory, so the message has to
	// name which key to list it under.
	for _, testCase := range []struct{ name, key string }{
		{"card.pw.html", "generate.templates"},
		{"rooms.pw.sql", "generate.queries"},
		{"readings.pw.dynamo", "generate.dynamo"},
		{"orders.pw.firestore", "generate.firestore"},
	} {
		message, stray := StrayTemplateMessage("scratch/"+testCase.name, testCase.name, SourcePurposes{})
		if !stray {
			t.Errorf("%s outside every purpose is not reported", testCase.name)
			continue
		}
		if !strings.Contains(message, testCase.key) {
			t.Errorf("%s: message %q does not name %s", testCase.name, message, testCase.key)
		}
		if !strings.HasPrefix(message, "scratch/"+testCase.name) {
			t.Errorf("%s: message %q does not open with the path", testCase.name, message)
		}
	}
}

func TestAPageTreeCompilesOnlyTheNamesItReserves(t *testing.T) {
	// The directory serves a purpose and the file is still compiled by
	// nothing, which is the case a purpose check alone would miss.
	message, stray := StrayTemplateMessage("pages/rooms/card.pw.html", "card.pw.html", SourcePurposes{Pages: true})

	if !stray {
		t.Fatal("an unreserved template inside a page tree is not reported")
	}
	for _, reserved := range []string{PageFile, LayoutFile, DocumentFile} {
		if !strings.Contains(message, reserved) {
			t.Fatalf("message %q does not name %s", message, reserved)
		}
	}
}

func TestATemplatePurposeCoveringAPageTreeDirectoryStillCompilesTheTemplate(t *testing.T) {
	// A project may list one directory under both keys. The tree rule is the
	// stricter one and is checked first, so this states which answer wins.
	if _, stray := StrayTemplateMessage("pages/card.pw.html", "card.pw.html", SourcePurposes{Pages: true, Templates: true}); !stray {
		t.Fatal("the page tree rule was not applied to a directory serving both purposes")
	}
}

func TestSomethingThatIsNotATemplateIsNotJudged(t *testing.T) {
	// Go files, generated output, and everything else have their own rules in
	// api:cli-generate; this decides templates only.
	for _, name := range []string{"main.go", "handler_pw_gen.go", "README.md", "styles.css"} {
		if _, stray := StrayTemplateMessage("dir/"+name, name, SourcePurposes{}); stray {
			t.Errorf("%s was judged as a template", name)
		}
	}
}
