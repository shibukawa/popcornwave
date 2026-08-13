// Package assetverify decides whether a static file is the kind of file its
// name claims, and whether an SVG carries anything that executes.
//
// It lives here rather than in the build because two callers need it and they
// sit on opposite sides of the module: pw build walks the authored tree, and
// the public asset middleware answers from a tree no build declared whenever
// server.public.read_local is set. One table read by both is what keeps a file
// refused at build time from being served at request time.
//
// The whole package reads bytes. It parses no text format, opens no file, and
// holds no state, because every caller already has the content in hand for
// another reason.
package assetverify

import (
	"bytes"
	"path"
	"regexp"
	"strings"
)

// Verdict is what the signature table concluded about one file.
type Verdict int

const (
	// Unconstrained means the extension declares a format with no signature
	// and the bytes carry no signature of their own. Most text files land
	// here, and nothing is reported.
	Unconstrained Verdict = iota
	// Confirmed means the extension declares a signature and the bytes carry
	// it.
	Confirmed
	// Contradicted means the bytes and the name disagree.
	Contradicted
)

// Window is how many leading bytes a verdict can depend on. Every signature in
// the table fits, including the ones at a non-zero offset, so a caller holding
// only a prefix gets the same answer as one holding the file.
const Window = 64

// Result carries the verdict and enough to say why in one line.
type Result struct {
	Verdict Verdict
	// Declared is the format name the extension claims, empty when the
	// extension is not in the table at all.
	Declared string
	// Detected is the format name the bytes carry, empty when they carry no
	// signature this package knows.
	Detected string
}

// Check reports whether content matches what name's extension declares.
//
// The name is the file's own name on disk, never the URL it is served under.
// The build deliberately puts AVIF bytes under a .webp URL, so judging by the
// URL would refuse the build's own output; the file behind it is called
// .avif and agrees with itself.
func Check(name string, content []byte) Result {
	declared, known := extensionFormats[strings.ToLower(path.Ext(name))]
	detected := detect(content)
	switch {
	// declared must be non-empty here: a signature-less extension over bytes
	// with no signature agrees with itself trivially, and calling that
	// Confirmed would claim the table had checked something it cannot.
	case known && declared != "" && declared == detected:
		return Result{Verdict: Confirmed, Declared: declared, Detected: detected}
	case known && signatureBearing[declared]:
		// The positive rule. A .png that is not a PNG is contradicted whether
		// or not the bytes are anything else recognisable, which is what
		// catches the SVG-in-a-PNG case: SVG is text and matches nothing.
		return Result{Verdict: Contradicted, Declared: declared, Detected: detected}
	case known && detected != "":
		// The negative rule. CSS, JS, JSON, and SVG assert nothing on their
		// own, so without this the extensions a browser treats as executable
		// or as same-origin markup would be exactly the ones exempt.
		return Result{Verdict: Contradicted, Declared: declared, Detected: detected}
	default:
		// An extension nobody put in the table constrains nothing. The table
		// is not a claim to know every format, and failing a build over a
		// name it was never taught would make it the first thing switched off.
		return Result{Verdict: Unconstrained, Declared: declared, Detected: detected}
	}
}

// detect names the format the leading bytes carry, or returns empty.
func detect(content []byte) string {
	for _, candidate := range signatures {
		for _, alternative := range candidate.alternatives {
			if matches(content, alternative) {
				return candidate.format
			}
		}
	}
	return ""
}

func matches(content []byte, parts []part) bool {
	for _, piece := range parts {
		end := piece.offset + len(piece.magic)
		if len(content) < end || !bytes.Equal(content[piece.offset:end], piece.magic) {
			return false
		}
	}
	return true
}

type part struct {
	offset int
	magic  []byte
}

type signature struct {
	format string
	// alternatives are the encodings of one format. Any one matching is the
	// format; a format with a single encoding has one entry.
	alternatives [][]part
}

func at(offset int, magic string) part { return part{offset: offset, magic: []byte(magic)} }

func one(magic string) [][]part { return [][]part{{at(0, magic)}} }

// signatures is ordered only for determinism; no two entries can match the same
// bytes, so nothing here depends on which is tried first.
//
// A format is in this table when its signature is unambiguous. MP3 is the one
// deliberately left out: a frame header is a bit pattern rather than a string,
// and an ID3 tag is three printable characters that a text file could open
// with, so including it would trade a real false positive for a rare catch.
var signatures = []signature{
	{format: "png", alternatives: one("\x89PNG\r\n\x1a\n")},
	{format: "jpeg", alternatives: one("\xff\xd8\xff")},
	{format: "gif", alternatives: [][]part{{at(0, "GIF87a")}, {at(0, "GIF89a")}}},
	{format: "bmp", alternatives: one("BM")},
	{format: "tiff", alternatives: [][]part{{at(0, "II*\x00")}, {at(0, "MM\x00*")}}},
	{format: "ico", alternatives: one("\x00\x00\x01\x00")},
	{format: "pdf", alternatives: one("%PDF-")},
	// The three ZIP records: a local file header, and the two an empty or
	// spanned archive opens with.
	{format: "zip", alternatives: [][]part{{at(0, "PK\x03\x04")}, {at(0, "PK\x05\x06")}, {at(0, "PK\x07\x08")}}},
	{format: "gzip", alternatives: one("\x1f\x8b")},
	{format: "zstd", alternatives: one("\x28\xb5\x2f\xfd")},
	{format: "wasm", alternatives: one("\x00asm")},
	{format: "woff", alternatives: one("wOFF")},
	{format: "woff2", alternatives: one("wOF2")},
	{format: "sfnt", alternatives: [][]part{{at(0, "\x00\x01\x00\x00")}, {at(0, "true")}, {at(0, "ttcf")}, {at(0, "OTTO")}}},
	{format: "ogg", alternatives: one("OggS")},
	{format: "flac", alternatives: one("fLaC")},
	// RIFF says nothing on its own; the form at offset 8 is the format.
	{format: "webp", alternatives: [][]part{{at(0, "RIFF"), at(8, "WEBP")}}},
	{format: "wav", alternatives: [][]part{{at(0, "RIFF"), at(8, "WAVE")}}},
	{format: "avi", alternatives: [][]part{{at(0, "RIFF"), at(8, "AVI ")}}},
	// One entry for the whole ISO base media family. The brand at offset 8
	// would separate MP4 from AVIF from HEIC, and is deliberately not read:
	// the brand registry keeps growing, so policing it would refuse a valid
	// file the day a new one ships, while the box itself already answers the
	// question worth asking, which is whether this is a media container at all.
	{format: "isobmff", alternatives: [][]part{{at(4, "ftyp")}}},
}

// extensionFormats maps an extension to the format its name declares. An
// extension absent here is unconstrained in both directions.
var extensionFormats = map[string]string{
	".png":   "png",
	".jpg":   "jpeg",
	".jpeg":  "jpeg",
	".gif":   "gif",
	".bmp":   "bmp",
	".tif":   "tiff",
	".tiff":  "tiff",
	".ico":   "ico",
	".pdf":   "pdf",
	".zip":   "zip",
	".gz":    "gzip",
	".zst":   "zstd",
	".zstd":  "zstd",
	".wasm":  "wasm",
	".woff":  "woff",
	".woff2": "woff2",
	".ttf":   "sfnt",
	".ttc":   "sfnt",
	".otf":   "sfnt",
	".ogg":   "ogg",
	".oga":   "ogg",
	".ogv":   "ogg",
	".flac":  "flac",
	".webp":  "webp",
	".wav":   "wav",
	".avi":   "avi",
	".mp4":   "isobmff",
	".m4v":   "isobmff",
	".m4a":   "isobmff",
	".mov":   "isobmff",
	".avif":  "isobmff",
	".heic":  "isobmff",
	".heif":  "isobmff",

	// Signature-less by design. Listing them is what turns "the table has
	// never heard of this" into "this format asserts nothing", which is the
	// difference between silence and the negative rule.
	".css":         "",
	".js":          "",
	".mjs":         "",
	".cjs":         "",
	".ts":          "",
	".json":        "",
	".map":         "",
	".webmanifest": "",
	".txt":         "",
	".md":          "",
	".csv":         "",
	".html":        "",
	".htm":         "",
	".xml":         "",
	".svg":         "",
	".vtt":         "",
}

// signatureBearing is the set of format names that have a signature, derived
// from the table so the two cannot disagree.
var signatureBearing = func() map[string]bool {
	bearing := map[string]bool{}
	for _, candidate := range signatures {
		bearing[candidate.format] = true
	}
	return bearing
}()

// activeSVG matches the three constructs a scan can find without parsing.
//
// The handler pattern demands at least two letters after "on" and a following
// equals sign, so a sentence containing " on = " in a text element does not
// match while onload, onerror, and the SMIL onbegin all do.
//
// Everything a parse would be needed for -- a SMIL set assigning href, an
// entity-encoded handler, a namespace prefix, foreignObject holding HTML -- is
// deliberately not here. Those are answered by the sandbox header on the
// response, which does not depend on the build having understood the file, so
// the gap this leaves is a missing warning rather than a missing defence.
var activeSVG = regexp.MustCompile(`(?i)(<script|\son[a-z]{2,}\s*=|javascript:)`)

// ActiveSVG reports the first executable-looking construct in an SVG, with the
// offset so the build can say where rather than only that.
func ActiveSVG(content []byte) (literal string, offset int, found bool) {
	span := activeSVG.FindIndex(content)
	if span == nil {
		return "", 0, false
	}
	return string(content[span[0]:span[1]]), span[0], true
}

// IsSVG reports whether the name is one this package scans for active content.
func IsSVG(name string) bool { return strings.EqualFold(path.Ext(name), ".svg") }

// Exempt reports whether a slash-separated path is covered by one of the
// configured globs.
//
// A trailing /** matches the subtree, which path.Match cannot express and is
// the shape an exemption actually wants: a vendored directory is exempted
// once rather than file by file.
func Exempt(name string, globs []string) bool {
	for _, glob := range globs {
		if subtree, ok := strings.CutSuffix(glob, "/**"); ok {
			if name == subtree || strings.HasPrefix(name, subtree+"/") {
				return true
			}
			continue
		}
		if matched, err := path.Match(glob, name); err == nil && matched {
			return true
		}
	}
	return false
}
