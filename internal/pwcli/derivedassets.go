package pwcli

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"mime"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/andybalholm/brotli"
	"github.com/klauspost/compress/gzip"
	"github.com/klauspost/compress/zstd"
)

// The build writes three directories under dist. Only the first is served; the
// other two exist so a rebuild can be honest about what it did.
const (
	// derivedPublicDir is the tree the application embeds and serves. It is
	// rebuilt from scratch every time, which is what makes a removed source
	// remove its outputs without a cleanup rule.
	derivedPublicDir = "dist/public"
	// derivedStageDir receives what the generation-time hooks produced. It is
	// separate from the served tree so that tree can be cleared without
	// deleting files pw generate wrote in an earlier step.
	derivedStageDir = "dist/derived"
	// conversionCacheDir keeps conversion outcomes across runs, including the
	// decision to decline one. Deleting it costs time and nothing else.
	conversionCacheDir = "dist/cache"
)

// Representation ordering. The build states which media type is worth serving,
// because that is a judgment about the bytes; the request only says what it can
// read. The last one is the fallback every client can take.
const (
	representationPreferenceFirst   = 0
	representationPreferenceDefault = 1
)

// sidecarCoding is one precompressed form the build writes beside a file.
//
// The order is the order a response prefers, and it is pure ratio: every one of
// these is encoded here, so preferring the smallest costs a request nothing.
// That is the whole reason brotli appears at all — at these levels it beats the
// others by a margin no per-request encoder could pay for, and it never has to
// run while a request waits.
type sidecarCoding struct {
	token  string
	suffix string
	encode func([]byte) ([]byte, error)
}

func derivedSidecarCodings() ([]sidecarCoding, error) {
	zstdEncoder, err := zstd.NewWriter(nil,
		zstd.WithEncoderLevel(zstd.SpeedBestCompression),
		zstd.WithEncoderConcurrency(1),
		zstd.WithEncoderCRC(false),
	)
	if err != nil {
		return nil, fmt.Errorf("public assets: create zstd encoder: %w", err)
	}
	return []sidecarCoding{
		{token: "br", suffix: ".br", encode: encodeBrotli},
		{token: "zstd", suffix: ".zstd", encode: func(source []byte) ([]byte, error) {
			return zstdEncoder.EncodeAll(source, nil), nil
		}},
		{token: "gzip", suffix: ".gz", encode: encodeGzip},
	}, nil
}

func encodeBrotli(source []byte) ([]byte, error) {
	var buffer bytes.Buffer
	writer := brotli.NewWriterLevel(&buffer, brotli.BestCompression)
	if _, err := writer.Write(source); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func encodeGzip(source []byte) ([]byte, error) {
	var buffer bytes.Buffer
	writer, err := gzip.NewWriterLevel(&buffer, gzip.BestCompression)
	if err != nil {
		return nil, err
	}
	if _, err := writer.Write(source); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

// sidecarTokens maps a produced suffix to its coding token, in the same order.
// The walks that only recognize a produced file read this rather than build an
// encoder to ask.
var sidecarTokens = [...]struct{ suffix, token string }{
	{suffix: ".br", token: "br"},
	{suffix: ".zstd", token: "zstd"},
	{suffix: ".gz", token: "gzip"},
}

// sidecarTokenFor reports the URL a produced sidecar is a representation of and
// the coding it carries, or false for a path that is an asset rather than a
// sidecar. The suffix is not the token: gzip is stored as .gz.
func sidecarTokenFor(name string) (url string, token string, ok bool) {
	for _, entry := range sidecarTokens {
		if strings.HasSuffix(name, entry.suffix) {
			return strings.TrimSuffix(name, entry.suffix), entry.token, true
		}
	}
	return "", "", false
}

// hasSidecarSuffix reports whether a path names a generated sidecar of any
// coding.
func hasSidecarSuffix(name string) bool {
	_, _, found := sidecarTokenFor(name)
	return found
}

// derivedAsset is one file in the built tree, with the metadata the manifest
// needs. Everything here is decided by the build, never at request time.
type derivedAsset struct {
	url       string
	path      string
	mediaType string
	encoding  string
	length    int
	etag      string
	// immutable marks a URL the build invented, whose name carries the digest
	// of its own bytes. Only such a URL can promise never to change: a name the
	// author wrote serves whatever the next build puts behind it.
	immutable  bool
	preference int
}

// derivedReport is what the build tells the developer. A rewrite is invisible
// in the template, so the build is the only place it can be seen.
type derivedReport struct {
	converted []string
	skipped   []string
	retained  []string
	// unserved names a file the served tree never owed anyone, which is a
	// different statement from a source some conversion replaced.
	unserved []string
	written  int
}

// buildDerivedAssets turns the authored public tree and whatever the generation
// hooks produced into the served tree and its manifest.
//
// The order is load-bearing: the tree is cleared before anything is copied into
// it, sources are dropped only after their replacement exists, sidecars are
// written over final bytes, and the manifest is written last because it
// describes all of it.
func buildDerivedAssets(root string, assets assetsConfig) (derivedReport, error) {
	// A variant is converted by the tree walk, so the upstream conversion cache
	// never sees it. Without one of its own, every build re-encoded every image
	// whether or not anything had changed.
	cache := filepath.Join(root, filepath.FromSlash(conversionCacheDir))
	return buildDerivedAssetsWithEncoder(root, assets, cachedImageEncoder(cache, "avif", encodeAVIF))
}

// buildDerivedAssetsWithEncoder is the body, with the variant encoder stated.
// Whether a variant wins is a property of the image; where it lands is a
// property of this code, and only the second is worth a test running an encoder.
func buildDerivedAssetsWithEncoder(root string, assets assetsConfig, encodeVariant imageEncoder) (derivedReport, error) {
	report := derivedReport{}
	authored := filepath.Join(root, "public")
	output := filepath.Join(root, filepath.FromSlash(derivedPublicDir))
	staged := filepath.Join(root, filepath.FromSlash(derivedStageDir))

	if err := verifyPublicEmbed(root); err != nil {
		return report, err
	}
	rootInfo, err := os.Lstat(authored)
	if err != nil {
		return report, fmt.Errorf("public assets: %w", err)
	}
	if rootInfo.Mode()&os.ModeSymlink != 0 || !rootInfo.IsDir() {
		return report, fmt.Errorf("public assets: public must be a regular directory")
	}
	if err := os.RemoveAll(output); err != nil {
		return report, fmt.Errorf("public assets: clear %s: %w", derivedPublicDir, err)
	}
	if err := os.MkdirAll(output, 0o755); err != nil {
		return report, fmt.Errorf("public assets: create %s: %w", derivedPublicDir, err)
	}
	// The sentinel is committed, because go:embed fails on an absent directory
	// and a fresh clone compiles before it ever runs a build. Clearing the tree
	// removes it, so it is written back rather than left as a deletion.
	if err := writeDerivedFile(filepath.Join(output, ".keep"), nil); err != nil {
		return report, err
	}

	produced, err := stagedProducedFiles(staged)
	if err != nil {
		return report, err
	}
	if !assets.SourceMaps {
		report.unserved = append(report.unserved, dropSourceMaps(produced)...)
	}
	// A conversion is recognized by its output existing, so nothing has to be
	// threaded back from the generator to know which sources were replaced.
	converted := map[string]string{}
	for name := range produced {
		if source, ok := convertedSourceFor(name, authored); ok {
			converted[source] = name
		}
	}

	rewrites := publicURLRewrites(converted)
	err = filepath.WalkDir(authored, func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := os.Lstat(name)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("public assets: symbolic links are not allowed: %s", name)
		}
		if entry.IsDir() {
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("public assets: irregular file is not allowed: %s", name)
		}
		relative, err := filepath.Rel(authored, name)
		if err != nil {
			return err
		}
		slashed := filepath.ToSlash(relative)
		if hasSidecarSuffix(slashed) {
			// A sidecar in the authored tree is left over from the build this
			// one replaces; the served tree gets its own.
			return nil
		}
		if replacement, ok := converted[slashed]; ok {
			retain, reason, err := sourceMustBeRetained(root, slashed)
			if err != nil {
				return err
			}
			report.converted = append(report.converted, slashed+" -> "+replacement)
			if !retain {
				return nil
			}
			report.retained = append(report.retained, slashed+" ("+reason+")")
		} else if scriptBuildInput(slashed, assets) {
			retain, reason, err := sourceMustBeRetained(root, slashed)
			if err != nil {
				return err
			}
			if !retain {
				report.unserved = append(report.unserved, slashed+" (TypeScript is a build input)")
				return nil
			}
			// Reported only as retained. A file that ships is not one the build
			// declined to serve, and saying both would describe two artifacts.
			report.retained = append(report.retained, slashed+" ("+reason+")")
		}
		source, err := os.ReadFile(name)
		if err != nil {
			return err
		}
		content, err := transformAuthoredFile(slashed, source, assets, rewrites)
		if err != nil {
			return err
		}
		return writeDerivedFile(filepath.Join(output, filepath.FromSlash(slashed)), content)
	})
	if err != nil {
		return report, err
	}

	for name, content := range produced {
		target := filepath.Join(output, filepath.FromSlash(name))
		if _, err := os.Lstat(target); err == nil {
			return report, fmt.Errorf("public assets: %s is produced by a conversion and also authored", name)
		}
		if err := writeDerivedFile(target, content); err != nil {
			return report, err
		}
	}

	if assets.AVIF {
		variants, err := writeAVIFVariants(authored, output, variantTargets(authored, output, converted), assets, encodeVariant)
		if err != nil {
			return report, err
		}
		report.skipped = append(report.skipped, variants...)
	}
	if err := writeDerivedSidecars(output); err != nil {
		return report, err
	}
	immutable := map[string]bool{}
	for name := range produced {
		immutable[name] = true
	}
	entries, err := collectDerivedAssets(output, immutable)
	if err != nil {
		return report, err
	}
	report.written = len(entries)
	if err := writeAssetManifest(root, entries); err != nil {
		return report, err
	}
	return report, nil
}

// verifyPublicEmbed refuses to build a tree the application will not embed. The
// scaffolded file is never rewritten by pw, so an older project is told exactly
// what to change rather than silently serving its authored tree.
func verifyPublicEmbed(root string) error {
	name := filepath.Join(root, "public.go")
	info, err := os.Lstat(name)
	if err != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("public.go is required; run pw init to create the public embed scaffold")
	}
	source, err := os.ReadFile(name)
	if err != nil {
		return err
	}
	if strings.Contains(string(source), "//go:embed all:"+derivedPublicDir) {
		return nil
	}
	return fmt.Errorf(`public.go embeds the authored tree, which is no longer what is served.
Change its two references:
  //go:embed all:%s
  fs.Sub(embeddedPublic, "%s")`, derivedPublicDir, derivedPublicDir)
}

// stagedProducedFiles reads what the generation hooks wrote. An empty or absent
// directory is the ordinary case for a project registering no transform.
func stagedProducedFiles(staged string) (map[string][]byte, error) {
	produced := map[string][]byte{}
	info, err := os.Lstat(staged)
	if err != nil || !info.IsDir() {
		return produced, nil
	}
	err = filepath.WalkDir(staged, func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() {
			return walkErr
		}
		relative, err := filepath.Rel(staged, name)
		if err != nil {
			return err
		}
		content, err := os.ReadFile(name)
		if err != nil {
			return err
		}
		produced[filepath.ToSlash(relative)] = content
		return nil
	})
	return produced, err
}

// convertedSourceFor maps a produced file back to the authored file it replaced,
// which is possible because this project owns both naming rules: an image keeps
// its path and changes its extension, and a script entry does the same.
func convertedSourceFor(produced, authored string) (string, bool) {
	extension := path.Ext(produced)
	base := strings.TrimSuffix(produced, extension)
	// A produced name carries the digest of its own bytes, which is what makes
	// its URL immutable. The source it came from is the name without it.
	base = strings.TrimSuffix(base, "."+contentHashOf(base))
	var candidates []string
	switch extension {
	case ".webp":
		candidates = []string{".png", ".jpg", ".jpeg"}
	case ".js":
		candidates = []string{".ts", ".tsx"}
	default:
		return "", false
	}
	for _, candidate := range candidates {
		source := base + candidate
		if info, err := os.Lstat(filepath.Join(authored, filepath.FromSlash(source))); err == nil && info.Mode().IsRegular() {
			return source, true
		}
	}
	return "", false
}

// dropSourceMaps takes the emitted maps out of the produced set and takes the
// comment naming each one out of the bundle it was written into. It reports
// what it dropped, sorted, so the build says so rather than quietly shipping a
// different artifact than the last invocation did.
//
// It runs here rather than in the conversion because the map is decided by how
// the build was invoked, and a conversion is memoized against its inputs and
// replayed. Deciding it here also keeps generation identical between the two
// artifact shapes: the bundle digest is taken over the body without its comment,
// so removing the comment leaves the name, the rewritten reference, and every
// byte of generated Go exactly where they were.
//
// The comment has to go with the file. A bundle still naming a map the tree does
// not hold turns every devtools open into a request for a file that is not
// there, which is a worse artifact than the one this is removing.
func dropSourceMaps(produced map[string][]byte) []string {
	var dropped []string
	for name := range produced {
		if path.Ext(name) != ".map" {
			continue
		}
		dropped = append(dropped, name+" (this artifact carries no source map)")
		delete(produced, name)
	}
	for name, content := range produced {
		if path.Ext(name) != ".js" {
			continue
		}
		if body, comment := splitSourceMapComment(string(content)); comment != "" {
			produced[name] = []byte(body)
		}
	}
	sort.Strings(dropped)
	return dropped
}

// scriptBuildInput reports whether an authored file is one the script build
// consumes rather than one the served tree owes anyone.
//
// No browser runs TypeScript, so with the script build on, a .ts or .tsx under
// public is an input by definition. An entry is already recognized by the bundle
// that replaced it; this is the module that entry imported, which no conversion
// produces a file for and which was therefore copied out beside the bundle it
// had been compiled into. The emitted source map carries its text, so a stack
// trace still names the authored line.
//
// Without the script build nothing consumes it, and a file the build does not
// understand is served as written, as everything else under public is.
func scriptBuildInput(name string, assets assetsConfig) bool {
	if !assets.Scripts {
		return false
	}
	switch strings.ToLower(path.Ext(name)) {
	case ".ts", ".tsx":
		return true
	}
	return false
}

// contentHashOf reports the digest segment a produced name ends with, or the
// empty string when it ends with something else. It is what lets a produced
// file be mapped back to the source it replaced without threading a table out
// of generation.
func contentHashOf(name string) string {
	segment := name[strings.LastIndexByte(name, '.')+1:]
	if len(segment) != contentHashLength {
		return ""
	}
	for index := 0; index < len(segment); index++ {
		character := segment[index]
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return ""
		}
	}
	return segment
}

// publicURLRewrites turns the conversion set into the substitutions a stylesheet
// pass applies, so a background-image follows the file it names.
func publicURLRewrites(converted map[string]string) map[string]string {
	rewrites := make(map[string]string, len(converted))
	for source, target := range converted {
		rewrites[source] = target
	}
	return rewrites
}

// sourceMustBeRetained decides whether dropping an authored file would break a
// reference the build cannot rewrite. Keeping a converted source costs bytes;
// dropping one that a script or a meta tag still names costs a broken page, so
// the scan errs toward keeping it and says so.
func sourceMustBeRetained(root, relative string) (bool, string, error) {
	needle := relative
	var found string
	err := filepath.WalkDir(root, func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", "dist", "node_modules", "public", ".devbox":
				return filepath.SkipDir
			}
			return nil
		}
		if found != "" || !scannableSource(entry.Name()) {
			return nil
		}
		content, err := os.ReadFile(name)
		if err != nil {
			return nil
		}
		if !strings.Contains(string(content), needle) {
			return nil
		}
		// A rewritten reference no longer names the source, so an occurrence
		// that survived generation is one this build could not reach.
		relativeName, relErr := filepath.Rel(root, name)
		if relErr != nil {
			relativeName = name
		}
		found = filepath.ToSlash(relativeName)
		return nil
	})
	if err != nil {
		return false, "", err
	}
	if found == "" {
		return false, "", nil
	}
	return true, "still named by " + found, nil
}

// scannableSource lists the file kinds a reference can hide in. A binary is
// excluded because a byte sequence matching a path inside one is not a
// reference.
func scannableSource(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".go", ".html", ".htm", ".js", ".mjs", ".ts", ".tsx", ".css", ".json", ".md", ".txt", ".xml", ".toml", ".yaml", ".yml":
		return true
	}
	return strings.HasSuffix(name, ".pw.html")
}

func writeDerivedFile(target string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	return writeScaffoldFile(target, content)
}

// variantTargets maps every served image URL back to the authored file it came
// from, whether a conversion moved it or not.
//
// An unconverted image is included on purpose: a machine with an avif encoder
// and no webp one — every stock mac, since sips writes avif and cannot write
// webp — still has something to offer, and it offers it under the URL the page
// already names.
func variantTargets(authored, output string, converted map[string]string) map[string]string {
	targets := map[string]string{}
	for source, target := range converted {
		targets[target] = source
	}
	_ = filepath.WalkDir(output, func(name string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return nil
		}
		relative, relErr := filepath.Rel(output, name)
		if relErr != nil {
			return nil
		}
		slashed := filepath.ToSlash(relative)
		if _, claimed := targets[slashed]; claimed || !convertibleImage(slashed) {
			return nil
		}
		if info, statErr := os.Lstat(filepath.Join(authored, filepath.FromSlash(slashed))); statErr == nil && info.Mode().IsRegular() {
			targets[slashed] = slashed
		}
		return nil
	})
	return targets
}

// writeAVIFVariants adds a second media representation to every served image,
// stored as a sibling of the URL rather than as a URL of its own.
//
// It is a sibling because markup cannot express the choice: the build never
// writes a picture element, and srcset carries density descriptors and no media
// type, so the only place left to choose is the response.
//
// It encodes from the authored source rather than from the webp, because a
// second lossy pass over already-lossy bytes is worse than one pass over the
// original.
func writeAVIFVariants(authored, output string, targets map[string]string, assets assetsConfig, encode imageEncoder) ([]string, error) {
	var declined []string
	served := make([]string, 0, len(targets))
	for target := range targets {
		served = append(served, target)
	}
	sort.Strings(served)
	for _, target := range served {
		source := targets[target]
		primary := filepath.Join(output, filepath.FromSlash(target))
		reference, err := os.Stat(primary)
		if err != nil {
			continue
		}
		lossless := losslessSource(source)
		encoded, err := encode(filepath.Join(authored, filepath.FromSlash(source)),
			lossless, assets.ImageQuality)
		if errors.Is(err, errNoEncoder) {
			// Nothing here can write the format on the axis this source needs,
			// which is a diagnostic rather than a build failure: the primary
			// representation still serves.
			//
			// The axis is named because the two resolve separately: a machine
			// can write a lossy variant of a photograph and be unable to write
			// a lossless one of a png, and only one of the two is a reason to
			// install anything.
			declined = append(declined, target+" (no "+losslessLabel(lossless)+" avif encoder)")
			continue
		}
		if err != nil {
			return nil, err
		}
		// The variant only earns its place by being smaller than the
		// representation it would be chosen over.
		if int64(len(encoded)) >= reference.Size() {
			declined = append(declined, target+" (avif was larger than the webp)")
			continue
		}
		if err := writeDerivedFile(primary+".avif", encoded); err != nil {
			return nil, err
		}
	}
	return declined, nil
}

// losslessLabel names an axis for a report line.
func losslessLabel(lossless bool) string {
	if lossless {
		return "lossless"
	}
	return "lossy"
}

// writeDerivedSidecars compresses the finished bytes. Compressing before a
// conversion would compress bytes nobody serves, which is why this runs last.
// writeDerivedSidecars compresses the finished bytes once per coding.
//
// A coding whose result is no smaller than its source is skipped rather than
// written: a sidecar that saves nothing costs a file in the embed and a
// representation in the manifest for no reason, and the negotiation falls
// through to the next coding on its own.
func writeDerivedSidecars(output string) error {
	codings, err := derivedSidecarCodings()
	if err != nil {
		return err
	}
	return filepath.WalkDir(output, func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() {
			return walkErr
		}
		if hasSidecarSuffix(name) || entry.Name() == ".keep" {
			return nil
		}
		if !publicAssetCompressible(name) {
			return nil
		}
		source, err := os.ReadFile(name)
		if err != nil {
			return err
		}
		for _, coding := range codings {
			encoded, encodeErr := coding.encode(source)
			if encodeErr != nil {
				return fmt.Errorf("public assets: encode %s as %s: %w", name, coding.token, encodeErr)
			}
			if len(encoded) >= len(source) {
				continue
			}
			if writeErr := writeScaffoldFile(name+coding.suffix, encoded); writeErr != nil {
				return writeErr
			}
		}
		return nil
	})
}

// collectDerivedAssets reads the finished tree once and produces one manifest
// entry per URL, with the compressed sibling folded in as a representation of
// the same URL rather than as a URL of its own.
func collectDerivedAssets(output string, immutable map[string]bool) ([]derivedAsset, error) {
	var assets []derivedAsset
	err := filepath.WalkDir(output, func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() {
			return walkErr
		}
		relative, err := filepath.Rel(output, name)
		if err != nil {
			return err
		}
		slashed := filepath.ToSlash(relative)
		if entry.Name() == ".keep" {
			return nil
		}
		content, err := os.ReadFile(name)
		if err != nil {
			return err
		}
		url, encoding := slashed, ""
		if source, token, isSidecar := sidecarTokenFor(slashed); isSidecar {
			url, encoding = source, token
		}
		// A media sibling is a second representation of one URL, never a URL of
		// its own: nothing may link it, and a cache must not learn about it
		// except through the negotiated response.
		mediaType, preference := derivedMediaType(url, content), representationPreferenceDefault
		if strings.HasSuffix(url, ".avif") {
			url = strings.TrimSuffix(url, ".avif")
			mediaType, preference = "image/avif", representationPreferenceFirst
		}
		sum := sha256.Sum256(content)
		assets = append(assets, derivedAsset{
			url:        url,
			immutable:  immutable[url],
			path:       slashed,
			mediaType:  mediaType,
			encoding:   encoding,
			length:     len(content),
			etag:       fmt.Sprintf("%q", fmt.Sprintf("%x", sum[:16])),
			preference: preference,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.SliceStable(assets, func(i, j int) bool {
		if assets[i].url != assets[j].url {
			return assets[i].url < assets[j].url
		}
		if assets[i].preference != assets[j].preference {
			return assets[i].preference < assets[j].preference
		}
		return assets[i].encoding < assets[j].encoding
	})
	return assets, nil
}

func derivedMediaType(url string, content []byte) string {
	if mediaType := mime.TypeByExtension(path.Ext(url)); mediaType != "" {
		return mediaType
	}
	return detectContentType(content)
}
