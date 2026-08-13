package assetverify

import "fmt"

// Kind separates the two conditions. They share a walk and a switch section but
// not an identifier: one asks whether the bytes match the name and is decidable
// from the bytes, the other asks whether an honest file carries something that
// executes and is best effort behind a response header that does not need it.
type Kind int

const (
	// TypeMismatch is the signature verdict.
	TypeMismatch Kind = iota
	// ActiveContent is the SVG scan.
	ActiveContent
)

// Finding is one refused file.
type Finding struct {
	// Path is slash-separated and relative to the authored tree, which is what
	// an exemption glob is written against and what a report should print.
	Path   string
	Kind   Kind
	Result Result
	// Literal and Offset carry the ActiveContent match.
	Literal string
	Offset  int
}

// Options is what the project configured.
type Options struct {
	// Signature enables the type check.
	Signature bool
	// SVGScan enables the best-effort active-content scan.
	SVGScan bool
	// Allow exempts paths from both, because a file kept on purpose is kept
	// for reasons the two checks cannot tell apart.
	Allow []string
}

// DefaultOptions is what a project that configures nothing gets. Both checks
// read bytes the caller already holds, so there is no cost to default them on.
func DefaultOptions() Options { return Options{Signature: true, SVGScan: true} }

// File runs the enabled checks over one file and returns the first finding.
//
// The order matters: a mislabelled file is reported as mislabelled even when it
// is also an SVG carrying script, because renaming it to .svg is not the fix
// and reporting the script first would suggest it was.
func File(name string, content []byte, options Options) (Finding, bool) {
	if Exempt(name, options.Allow) {
		return Finding{}, false
	}
	if options.Signature {
		if result := Check(name, content); result.Verdict == Contradicted {
			return Finding{Path: name, Kind: TypeMismatch, Result: result}, true
		}
	}
	if options.SVGScan && IsSVG(name) {
		if literal, offset, found := ActiveSVG(content); found {
			return Finding{Path: name, Kind: ActiveContent, Literal: literal, Offset: offset}, true
		}
	}
	return Finding{}, false
}

// Message is the one line a build, a dev server, and pw doctor all print, so
// the same file reads the same way wherever it was caught.
func (f Finding) Message() string {
	switch {
	case f.Kind == ActiveContent:
		return fmt.Sprintf("%s: SVG carries %q at byte %d", f.Path, f.Literal, f.Offset)
	case f.Result.Declared == "":
		return fmt.Sprintf("%s: the extension declares a format with no signature, and the bytes carry %s",
			f.Path, f.Result.Detected)
	case f.Result.Detected == "":
		return fmt.Sprintf("%s: the extension declares %s, and the bytes carry no %s signature",
			f.Path, f.Result.Declared, f.Result.Declared)
	default:
		return fmt.Sprintf("%s: the extension declares %s, and the bytes carry %s",
			f.Path, f.Result.Declared, f.Result.Detected)
	}
}

// Remedy is the fixed advice, which differs by kind and is what separates a
// useful refusal from one that only says no.
func (f Finding) Remedy() string {
	if f.Kind == ActiveContent {
		return "remove the script, or list the path in assets.verify.allow when the SVG is interactive on purpose"
	}
	return "rename the file to the type it actually is, or list the path in assets.verify.allow"
}

func (f Finding) Error() string { return f.Message() + "; " + f.Remedy() }
