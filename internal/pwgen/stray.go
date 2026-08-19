package pwgen

import (
	"fmt"
	"strings"
)

// SourcePurposes is the set of generate purposes one directory belongs to,
// restricted to the purposes that read a template source.
//
// It exists so the rule below is stated once. api:cli-generate reports a
// template nothing compiles, and requirement:editor-diagnostics reports the
// same condition in the editor; decision:shared-check-catalog is what makes
// two wordings of one finding a defect rather than a detail.
type SourcePurposes struct {
	Templates bool
	Queries   bool
	Dynamo    bool
	Firestore bool
	// Pages marks a directory inside a page tree root. A tree compiles only
	// the names it reserves, so a template there is stray on stricter terms
	// than one in an ordinary directory.
	Pages bool
}

// ReservedPageTemplate reports whether a base name is one a page tree compiles.
func ReservedPageTemplate(name string) bool {
	switch name {
	case PageFile, LayoutFile, DocumentFile:
		return true
	default:
		return false
	}
}

// StrayTemplateMessage reports why nothing compiles this source, or false when
// something does.
//
// name is the base name and relative is the slash-separated path from the
// project root, which is what the message names: a reader fixes this by editing
// popcornweb.toml or by moving the file, and both need the path as written.
func StrayTemplateMessage(relative, name string, purposes SourcePurposes) (string, bool) {
	switch {
	case purposes.Pages && strings.HasSuffix(name, ".pw.html"):
		if ReservedPageTemplate(name) {
			return "", false
		}
		return fmt.Sprintf(
			"%s is inside a page tree but is not %s, %s, or %s, so nothing compiles it",
			relative, PageFile, LayoutFile, DocumentFile), true
	case strings.HasSuffix(name, ".pw.html") && !purposes.Templates:
		return outsidePurpose(relative, "generate.templates"), true
	case strings.HasSuffix(name, ".pw.sql") && !purposes.Queries:
		return outsidePurpose(relative, "generate.queries"), true
	case strings.HasSuffix(name, ".pw.dynamo") && !purposes.Dynamo:
		return outsidePurpose(relative, "generate.dynamo"), true
	case strings.HasSuffix(name, ".pw.firestore") && !purposes.Firestore:
		return outsidePurpose(relative, "generate.firestore"), true
	default:
		return "", false
	}
}

func outsidePurpose(relative, purpose string) string {
	return fmt.Sprintf("%s is outside %s and is not generated from; list its directory to include it", relative, purpose)
}
