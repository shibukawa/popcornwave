package assetverify

import (
	"strings"
	"testing"
)

const (
	pngMagic  = "\x89PNG\r\n\x1a\n"
	zipMagic  = "PK\x03\x04"
	svgSource = `<?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg"><rect/></svg>`
)

// riff builds a RIFF container with the given form, which is the only way the
// webp and wav signatures differ.
func riff(form string) string { return "RIFF\x00\x00\x00\x00" + form }

// isobmff builds a box header with the given brand.
func isobmff(brand string) string { return "\x00\x00\x00\x20ftyp" + brand }

func TestCheckVerdicts(t *testing.T) {
	for _, testcase := range []struct {
		name     string
		file     string
		content  string
		verdict  Verdict
		declared string
		detected string
	}{
		// The positive rule.
		{"a png that is a png", "logo.png", pngMagic + "rest", Confirmed, "png", "png"},
		{"the motivating case", "logo.png", svgSource, Contradicted, "png", ""},
		{"a png that is a pdf", "logo.png", "%PDF-1.7", Contradicted, "png", "pdf"},
		{"an empty png", "logo.png", "", Contradicted, "png", ""},

		// The negative rule: a signature-less extension is not a hole.
		{"ordinary css", "app.css", "body{color:red}", Unconstrained, "", ""},
		{"a stylesheet that is a zip", "app.css", zipMagic + "junk", Contradicted, "", "zip"},
		{"a script that is a pdf", "app.js", "%PDF-1.7", Contradicted, "", "pdf"},
		{"an ordinary svg", "icon.svg", svgSource, Unconstrained, "", ""},

		// An extension the table never heard of constrains nothing, in either
		// direction, so a project shipping one is not failed over a name.
		{"an unknown extension holding a zip", "data.bin", zipMagic, Unconstrained, "", "zip"},
		{"an unknown extension holding text", "notes.rst", "hello", Unconstrained, "", ""},

		// RIFF says nothing until the form at offset 8.
		{"a webp that is a webp", "a.webp", riff("WEBP"), Confirmed, "webp", "webp"},
		{"a webp that is a wav", "a.webp", riff("WAVE"), Contradicted, "webp", "wav"},

		// Fonts share one signature family, so an .otf holding a TrueType
		// outline is confirmed rather than refused. That is the intent: the
		// container is the same and the extension is a convention.
		{"an otf", "f.otf", "OTTO\x00", Confirmed, "sfnt", "sfnt"},
		{"a ttf", "f.ttf", "\x00\x01\x00\x00", Confirmed, "sfnt", "sfnt"},
		{"a font that is a zip", "f.woff2", zipMagic, Contradicted, "woff2", "zip"},

		{"an extension is matched case-insensitively", "LOGO.PNG", pngMagic, Confirmed, "png", "png"},
	} {
		t.Run(testcase.name, func(t *testing.T) {
			got := Check(testcase.file, []byte(testcase.content))
			if got.Verdict != testcase.verdict {
				t.Errorf("verdict = %v, want %v", got.Verdict, testcase.verdict)
			}
			if got.Declared != testcase.declared {
				t.Errorf("declared = %q, want %q", got.Declared, testcase.declared)
			}
			if got.Detected != testcase.detected {
				t.Errorf("detected = %q, want %q", got.Detected, testcase.detected)
			}
		})
	}
}

// The ISO base media brand is deliberately not read, so every extension in the
// family accepts every brand. Pinning it here means the day someone decides to
// police brands, this test is what states the decision they are reversing.
func TestISOBMFFBrandIsNotPoliced(t *testing.T) {
	for _, name := range []string{"clip.mp4", "photo.avif", "photo.heic", "clip.mov"} {
		for _, brand := range []string{"avif", "isom", "heic", "qt  "} {
			if got := Check(name, []byte(isobmff(brand))); got.Verdict != Confirmed {
				t.Errorf("Check(%q, brand %q) = %v, want Confirmed", name, brand, got.Verdict)
			}
		}
	}
	if got := Check("clip.mp4", []byte(pngMagic)); got.Verdict != Contradicted {
		t.Errorf("an mp4 holding a png = %v, want Contradicted", got.Verdict)
	}
}

// A verdict may not depend on more than the window, because the middleware and
// the doctor walk both hand over a prefix rather than the file.
func TestVerdictFitsTheWindow(t *testing.T) {
	full := []byte(riff("WEBP") + strings.Repeat("payload beyond the window; ", 8))
	if got := Check("a.webp", full[:Window]); got.Verdict != Confirmed {
		t.Errorf("a prefix of the window = %v, want Confirmed", got.Verdict)
	}
}

func TestActiveSVG(t *testing.T) {
	for _, testcase := range []struct {
		name    string
		content string
		found   bool
	}{
		{"a script element", `<svg><script>alert(1)</script></svg>`, true},
		{"an uppercase script element", `<svg><SCRIPT>alert(1)</SCRIPT></svg>`, true},
		{"an event handler", `<svg onload="alert(1)"></svg>`, true},
		{"a smil handler", `<svg><animate onbegin="alert(1)"/></svg>`, true},
		{"a spaced handler", `<svg onload = "alert(1)"></svg>`, true},
		{"a javascript url", `<svg><a href="javascript:alert(1)">x</a></svg>`, true},
		{"an ordinary svg", svgSource, false},

		// The two shapes the pattern is written to leave alone. A build that
		// fails on prose is a build people switch the check off in.
		{"prose containing on =", `<svg><text>version on = 3</text></svg>`, false},
		{"an attribute ending in on", `<svg><text json="1">x</text></svg>`, false},
		{"a word starting with on", `<svg><desc>only once</desc></svg>`, false},
	} {
		t.Run(testcase.name, func(t *testing.T) {
			_, _, found := ActiveSVG([]byte(testcase.content))
			if found != testcase.found {
				t.Errorf("found = %v, want %v", found, testcase.found)
			}
		})
	}
}

func TestActiveSVGReportsWhere(t *testing.T) {
	literal, offset, found := ActiveSVG([]byte(`<svg>` + `<script>x</script></svg>`))
	if !found {
		t.Fatal("no finding")
	}
	if literal != "<script" {
		t.Errorf("literal = %q, want %q", literal, "<script")
	}
	if offset != 5 {
		t.Errorf("offset = %d, want 5", offset)
	}
}

func TestExempt(t *testing.T) {
	for _, testcase := range []struct {
		name  string
		path  string
		globs []string
		want  bool
	}{
		{"an exact path", "vendor/a.png", []string{"vendor/a.png"}, true},
		{"a wildcard in one segment", "vendor/a.png", []string{"vendor/*.png"}, true},
		{"a wildcard does not cross a slash", "vendor/deep/a.png", []string{"vendor/*.png"}, false},
		{"a subtree", "vendor/deep/a.png", []string{"vendor/**"}, true},
		{"a subtree matches its own root", "vendor", []string{"vendor/**"}, true},
		{"a subtree does not match a sibling prefix", "vendored/a.png", []string{"vendor/**"}, false},
		{"no globs", "a.png", nil, false},
	} {
		t.Run(testcase.name, func(t *testing.T) {
			if got := Exempt(testcase.path, testcase.globs); got != testcase.want {
				t.Errorf("Exempt(%q, %v) = %v, want %v", testcase.path, testcase.globs, got, testcase.want)
			}
		})
	}
}

func TestFile(t *testing.T) {
	both := DefaultOptions()

	// A mislabelled file that is also an active SVG reads as mislabelled,
	// because renaming it to .svg is not the fix and reporting the script
	// first would suggest that it was.
	finding, refused := File("logo.png", []byte(`<svg onload="x"></svg>`), both)
	if !refused || finding.Kind != TypeMismatch {
		t.Errorf("kind = %v refused = %v, want TypeMismatch true", finding.Kind, refused)
	}

	finding, refused = File("icon.svg", []byte(`<svg onload="x"></svg>`), both)
	if !refused || finding.Kind != ActiveContent {
		t.Errorf("kind = %v refused = %v, want ActiveContent true", finding.Kind, refused)
	}

	// One list covers both checks.
	if _, refused := File("icon.svg", []byte(`<svg onload="x"></svg>`),
		Options{Signature: true, SVGScan: true, Allow: []string{"icon.svg"}}); refused {
		t.Error("an exempted path was still refused")
	}
	if _, refused := File("logo.png", []byte(svgSource),
		Options{Signature: true, Allow: []string{"logo.png"}}); refused {
		t.Error("an exempted path was still refused by the signature check")
	}

	// Each switch turns off only its own check.
	if _, refused := File("logo.png", []byte(svgSource), Options{SVGScan: true}); refused {
		t.Error("the signature check ran while disabled")
	}
	if _, refused := File("icon.svg", []byte(`<svg onload="x"></svg>`), Options{Signature: true}); refused {
		t.Error("the SVG scan ran while disabled")
	}
}

func TestFindingMessage(t *testing.T) {
	for _, testcase := range []struct {
		name    string
		finding Finding
		want    string
	}{
		{
			"a declared format with nothing behind it",
			Finding{Path: "logo.png", Result: Result{Declared: "png"}},
			"logo.png: the extension declares png, and the bytes carry no png signature",
		},
		{
			"a declared format holding another",
			Finding{Path: "logo.png", Result: Result{Declared: "png", Detected: "pdf"}},
			"logo.png: the extension declares png, and the bytes carry pdf",
		},
		{
			"the negative rule",
			Finding{Path: "app.css", Result: Result{Detected: "zip"}},
			"app.css: the extension declares a format with no signature, and the bytes carry zip",
		},
		{
			"an active svg",
			Finding{Path: "i.svg", Kind: ActiveContent, Literal: "<script", Offset: 5},
			`i.svg: SVG carries "<script" at byte 5`,
		},
	} {
		t.Run(testcase.name, func(t *testing.T) {
			if got := testcase.finding.Message(); got != testcase.want {
				t.Errorf("Message() = %q, want %q", got, testcase.want)
			}
		})
	}
}
