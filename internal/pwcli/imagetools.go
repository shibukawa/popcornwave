package pwcli

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

// The image conversions run as host tools rather than as a Go encoder.
//
// A cgo codec would end the single-runner cross-compilation the CLI release
// depends on, and the cgo-free Go encoders available today lose the size
// comparison against Go's own png encoder on ordinary images, which makes them
// an encoder that produces nothing but declines. libwebp and libavif are the
// implementations these formats are defined by, so they are invoked the way the
// Tailwind executable already is: pinned by the project's tool environment and
// never installed implicitly.
const (
	webpTool = "cwebp"
	avifTool = "avifenc"
	// sipsTool ships with macOS. It writes avif and reads webp without writing
	// it, so it is a fallback for one of the two formats and not the other.
	sipsTool = "sips"
)

// errNoEncoder is what a format nothing on this machine can write returns.
//
// It is not a build failure. An unconverted image is a larger image, not a
// broken page, so the conversion declines with a reason and pw doctor reports
// the missing tool.
var errNoEncoder = errors.New("no encoder is installed for this format")

// toolIdentity is a resolved tool and the version it reported. Both join the
// conversion cache key: the generator executable hash covers a Go encoder and
// sees nothing of an external one, so a silent tool upgrade would otherwise
// serve bytes from a cache that believes it is current.
type toolIdentity struct {
	path    string
	version string
}

var toolIdentities sync.Map

func resolveImageTool(name string) (toolIdentity, error) {
	if cached, ok := toolIdentities.Load(name); ok {
		identity := cached.(toolIdentity)
		if identity.path == "" {
			return identity, fmt.Errorf("%s is not in PATH", name)
		}
		return identity, nil
	}
	path, err := exec.LookPath(name)
	if err != nil {
		toolIdentities.Store(name, toolIdentity{})
		return toolIdentity{}, fmt.Errorf("%s is not in PATH", name)
	}
	identity := toolIdentity{path: path, version: readToolVersion(name, path)}
	toolIdentities.Store(name, identity)
	return identity, nil
}

// readToolVersion asks the tool what it is. An unreadable answer is recorded as
// unknown rather than treated as an error: the key is then weaker, and refusing
// to build over it would be worse than converting.
func readToolVersion(name, path string) string {
	flag := "-version"
	if name == sipsTool {
		// sips carries no version of its own; it moves with the OS, so that is
		// what identifies it.
		flag = "--version"
	}
	command := exec.Command(path, flag)
	var out bytes.Buffer
	command.Stdout, command.Stderr = &out, &out
	if err := command.Run(); err != nil {
		return "unknown"
	}
	return strings.TrimSpace(strings.SplitN(out.String(), "\n", 2)[0])
}

// encoderCandidates lists, in preference order, the tools that can write one
// format on the requested axis. The dedicated encoder comes first everywhere;
// the platform tool is a fallback for a machine that has not installed one.
//
// Two exclusions, both measured on 2026-08-04 rather than assumed:
//
//   - webp has no fallback at all, because sips reads webp and cannot write it.
//     A listed tool that exits zero and writes nothing is worse than none.
//   - sips is excluded from the lossless axis, because it accepts
//     "formatOptions lossless" and produces a lossy file anyway. A png round
//     tripped through it came back with pixels differing by up to 8 across most
//     of the image, where avifenc --lossless came back byte-identical.
//
// The second one matters more than it looks: a png is authored exact, and a
// screenshot, a diagram, or a logo re-encoded lossily is a visible defect that
// nothing downstream would report.
func encoderCandidates(format string, lossless bool) []string {
	switch format {
	case "webp":
		return []string{webpTool}
	case "avif":
		if runtime.GOOS == "darwin" && !lossless {
			return []string{avifTool, sipsTool}
		}
		return []string{avifTool}
	}
	return nil
}

// resolveEncoder picks the first installed candidate that can write the format
// on the axis the source requires.
func resolveEncoder(format string, lossless bool) (string, toolIdentity, error) {
	for _, name := range encoderCandidates(format, lossless) {
		if identity, err := resolveImageTool(name); err == nil {
			return name, identity, nil
		}
	}
	return "", toolIdentity{}, errNoEncoder
}

// encodeWebP converts one authored image. Lossless is the right axis for a png,
// where the source is already exact, and the wrong one for a photograph, where
// it produces a file larger than the jpeg it came from.
func encodeWebP(source string, lossless bool, quality int) ([]byte, error) {
	name, identity, err := resolveEncoder("webp", lossless)
	if err != nil {
		return nil, err
	}
	arguments := []string{"-quiet", "-mt"}
	if lossless {
		arguments = append(arguments, "-lossless", "-z", "9")
	} else {
		arguments = append(arguments, "-q", fmt.Sprint(quality))
	}
	return runImageTool(name, identity, source, ".webp", arguments)
}

// encodeAVIF produces the additional representation policy:public-asset-media-negotiation
// selects from Accept. It is generated from the authored source rather than
// from the webp, because a second lossy pass over lossy bytes is worse than one
// pass over the original.
func encodeAVIF(source string, lossless bool, quality int) ([]byte, error) {
	name, identity, err := resolveEncoder("avif", lossless)
	if err != nil {
		return nil, err
	}
	if name == sipsTool {
		// Only reachable on the lossy axis, per encoderCandidates.
		return runSips(identity, source, ".avif", "avif", quality)
	}
	arguments := []string{"--jobs", "all"}
	if lossless {
		arguments = append(arguments, "--lossless")
	} else {
		arguments = append(arguments, "--qcolor", fmt.Sprint(quality))
	}
	return runImageTool(name, identity, source, ".avif", arguments)
}

// runSips writes through the platform tool, whose argument shape is its own:
// the format is a setting rather than a flag, and quality is formatOptions.
//
// It takes no lossless parameter because it cannot honor one; the axis is
// decided before a caller gets here.
func runSips(identity toolIdentity, source, extension, format string, quality int) ([]byte, error) {
	arguments := []string{"-s", "format", format, "-s", "formatOptions", fmt.Sprint(quality)}
	return runImageToolWith(identity, source, extension, arguments, func(target string) []string {
		return []string{"--out", target}
	}, sipsTool)
}

// runImageTool writes to a temporary file rather than a pipe, because none of
// these tools writes a container to stdout.
func runImageTool(name string, identity toolIdentity, source, extension string, arguments []string) ([]byte, error) {
	return runImageToolWith(identity, source, extension, arguments, func(target string) []string {
		return []string{"-o", target}
	}, name)
}

func runImageToolWith(identity toolIdentity, source, extension string, arguments []string, output func(string) []string, name string) ([]byte, error) {
	target, err := os.CreateTemp("", "pw-asset-*"+extension)
	if err != nil {
		return nil, err
	}
	path := target.Name()
	if err := target.Close(); err != nil {
		return nil, err
	}
	defer os.Remove(path)

	command := exec.Command(identity.path, append(append(append([]string{}, arguments...), source), output(path)...)...)
	var diagnostics bytes.Buffer
	command.Stdout, command.Stderr = &diagnostics, &diagnostics
	if err := command.Run(); err != nil {
		return nil, fmt.Errorf("%s %s: %w: %s", name, filepath.Base(source), err, strings.TrimSpace(diagnostics.String()))
	}
	encoded, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(encoded) == 0 {
		// A tool that exits zero and writes nothing has refused the format
		// without saying so, which must not reach the tree as an empty file.
		return nil, fmt.Errorf("%s %s: wrote no output", name, filepath.Base(source))
	}
	return encoded, nil
}

// imageEncoderParams names the tool identity a conversion depends on, for the
// cache key. A machine with no encoder still produces a stable key, so the
// decision to decline is cached like any other outcome.
func imageEncoderParams(format string, lossless bool) string {
	axis := "lossy"
	if lossless {
		axis = "lossless"
	}
	name, identity, err := resolveEncoder(format, lossless)
	if err != nil {
		return format + "/" + axis + "=none"
	}
	return format + "/" + axis + "=" + name + "@" + identity.version
}

// missingImageEncoders reports the formats this machine cannot write, for the
// diagnostic that says images will ship unconverted.
func missingImageEncoders(avif bool) []string {
	var missing []string
	if _, _, err := resolveEncoder("webp", true); err != nil {
		missing = append(missing, "webp ("+webpTool+")")
	}
	if !avif {
		return missing
	}
	// The two axes resolve separately, so a machine can be able to write a
	// lossy variant and unable to write a lossless one. Reporting the format as
	// present would then be wrong for every png in the tree.
	_, _, losslessErr := resolveEncoder("avif", true)
	_, _, lossyErr := resolveEncoder("avif", false)
	switch {
	case losslessErr != nil && lossyErr != nil:
		missing = append(missing, "avif ("+avifTool+")")
	case losslessErr != nil:
		missing = append(missing, "lossless avif ("+avifTool+"; "+sipsTool+" cannot encode losslessly)")
	}
	return missing
}
