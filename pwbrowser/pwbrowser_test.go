package pwbrowser

import (
	"strings"
	"testing"
)

// The revision is a function of the bytes, so an upgrade that changes the
// runtime cannot ship under a URL a browser already cached forever.
func TestTheRevisionFollowsTheBytes(t *testing.T) {
	before := Revision()
	if before == "" || len(before) != 16 {
		t.Fatalf("revision = %q", before)
	}
	Publish(map[string]string{"extra.js": "export const x = 1"}, "\nimport './extra.js'")
	t.Cleanup(func() { Publish(nil, "") })

	if after := Revision(); after == before {
		t.Error("adding a module left the revision unchanged, so a browser holding the old URL immutably would receive the new set")
	}
	if _, ok := Scripts()["extra.js"]; !ok {
		t.Error("the published module is not in the set")
	}
	if !strings.Contains(Scripts()[RuntimeName], "import './extra.js'") {
		t.Error("the core does not import what was published beside it")
	}
	Publish(nil, "")
	if Revision() != before {
		t.Error("removing the module did not restore the release revision")
	}
}

// Only the modules of the current set are claimed. The prefix holds more than
// the scripts, so claiming the whole namespace here would swallow every route
// that arrived after this one; a transport closes it below all of them.
func TestLookupClaimsOnlyTheCurrentSet(t *testing.T) {
	if _, _, ok := Lookup(RuntimeScriptURL()); !ok {
		t.Error("the runtime module was not claimed at its own URL")
	}
	for _, path := range []string{
		Prefix + "0000000000000000/" + RuntimeName, // a stale revision
		Prefix + Revision() + "/nested/thing.js",   // not a module name
		Prefix + Revision() + "/",                  // no name at all
		Prefix + "redraw",                          // another owner's route
		"/orders",                                  // outside entirely
	} {
		if _, _, ok := Lookup(path); ok {
			t.Errorf("%s was claimed by the script set", path)
		}
	}
	// The namespace is still recognized, which is what lets a transport close
	// it rather than pass it to application routing.
	if !Claims(Prefix + "redraw") {
		t.Error("the reserved prefix is not recognized")
	}
	if Claims("/orders") {
		t.Error("an ordinary route was read as reserved")
	}
}

// The name is the only thing to read a type from: the bytes are held as
// strings, and sniffing them would be guessing at what the file name states.
func TestContentTypeComesFromTheName(t *testing.T) {
	if got := ContentType(RuntimeName); !strings.HasPrefix(got, "text/javascript") {
		t.Errorf("ContentType(%q) = %q", RuntimeName, got)
	}
	if got := ContentType("devmark.webp"); got != "image/webp" {
		t.Errorf("ContentType(devmark.webp) = %q", got)
	}
}
