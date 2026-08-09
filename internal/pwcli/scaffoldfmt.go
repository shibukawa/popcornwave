package pwcli

import (
	"path"

	"github.com/shibukawa/tinybind-go/templates/templatefmt"
)

// A scaffolded template source is written the way pw fmt would leave it, so
// the first pw fmt --check in a new project passes instead of reporting the
// framework's own files. The literals in this package stay authored for the
// reader of this source; what canonical looks like is the formatter's answer,
// asked here at scaffold time rather than copied into every literal — a
// formatter release then moves the scaffold with it instead of contradicting it.

// canonicalScaffoldSource returns the bytes pw fmt (with no flags) would leave
// at this path. A path the formatter does not recognize passes through, which
// is every scaffolded file that is not a template source.
//
// So does a source the formatter cannot parse: the pw generate run that
// follows every scaffold parses the same bytes and reports a broken literal
// with its position, which is a better failure than this pass could produce.
// The scaffold tests format every variant, so an unparsable literal still
// cannot reach a release quietly.
func canonicalScaffoldSource(sourcePath, content string) string {
	options := fmtOptions{}.formatOptions()
	format, err := templatefmt.Identify(path.Base(sourcePath), options)
	if err != nil {
		return content
	}
	formatted, err := templatefmt.SourceAs(format, sourcePath, []byte(content), options)
	if err != nil {
		return content
	}
	return string(formatted)
}

// canonicalScaffoldSources rewrites every template source in a scaffold map in
// place.
func canonicalScaffoldSources(files map[string]string) {
	for sourcePath, content := range files {
		files[sourcePath] = canonicalScaffoldSource(sourcePath, content)
	}
}
