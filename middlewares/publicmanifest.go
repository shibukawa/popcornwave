package middlewares

import (
	"errors"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// AssetRepresentation is one stored form of one public asset: a media type, an
// optional content coding, and the validator for exactly those bytes.
//
// Two representations of one URL never share a validator, because a cache
// holding one must not answer a request that asked for the other.
type AssetRepresentation struct {
	// Path locates the bytes inside the served filesystem.
	Path string
	// MediaType is the Content-Type to answer with. It may disagree with the
	// extension of the URL, which is what lets one URL carry a modern format.
	MediaType string
	// ContentEncoding is empty for the identity form, or a coding token such as
	// zstd for a precompressed sibling.
	ContentEncoding string
	// Length is the stored byte count, so a response needs no stat.
	Length int
	// ETag is the quoted strong tag of these bytes.
	ETag string
	// Preference orders the media types of one URL. The build sets it, because
	// which encoding is worth serving is a judgment about the bytes and the
	// client only states what it can read. Lower sorts first.
	Preference int
	// External marks bytes that ship as their own file rather than inside the
	// binary, so Path is read from the external root at request time.
	//
	// Length and ETag are empty for one of these, and deliberately so: the tree
	// is deployed as its own artifact, so a validator the build computed could
	// outlive the bytes it describes. The file answers for itself instead.
	External bool
}

// AssetEntry is everything the middleware answers with for one URL. A build
// produces it; nothing is discovered by walking the filesystem at request time,
// and no digest is computed while a request waits.
type AssetEntry struct {
	// URL is the mount-relative path, without a leading slash.
	URL string
	// CacheControl is sent verbatim. A name derived from its source is stable
	// rather than immutable, so the default revalidates and the ETag does the
	// work.
	CacheControl string
	// Revision is the digest of this URL's own representations, which is the
	// segment PublicAssetURL puts in front of the name to make it immutable.
	//
	// It is empty for a URL that needs none or may not have one: a name the
	// build invented already carries its digest, and an external file ships as
	// its own artifact with no validator the build may claim.
	Revision string
	// Representations is ordered by Preference, then by content coding.
	Representations []AssetRepresentation
}

// defaultCacheControl revalidates every time. A derived file keeps the name of
// the source it came from, so the same URL can serve different bytes after a
// rebuild and no long max-age would be honest. The validator makes that cheap:
// an unchanged asset costs a 304 and no body.
const defaultCacheControl = "public, no-cache"

var publicManifestState = struct {
	sync.RWMutex
	entries map[string]AssetEntry
}{}

// RegisterPublicManifest installs the build-produced description of the served
// tree. A generated project file calls it during package initialization, beside
// the RegisterPublicFS call that installs the bytes it describes.
//
// The two belong together: a manifest describing a different build than the
// embedded tree would answer with validators for bytes nobody holds.
func RegisterPublicManifest(entries []AssetEntry) {
	if len(entries) == 0 {
		// A build with nothing to declare registers nothing, rather than an
		// empty table that would answer 404 for every URL the tree holds. The
		// clear rather than an early return is what lets a caller that
		// installed a manifest put the process back, which only a test does:
		// an application registers one table, once, from a generated init.
		publicManifestState.Lock()
		defer publicManifestState.Unlock()
		publicManifestState.entries = nil
		return
	}
	indexed := make(map[string]AssetEntry, len(entries))
	for _, entry := range entries {
		if entry.CacheControl == "" {
			entry.CacheControl = defaultCacheControl
		}
		sort.SliceStable(entry.Representations, func(i, j int) bool {
			return entry.Representations[i].Preference < entry.Representations[j].Preference
		})
		indexed[entry.URL] = entry
	}
	publicManifestState.Lock()
	defer publicManifestState.Unlock()
	publicManifestState.entries = indexed
}

func manifestEntry(name string) (AssetEntry, bool) {
	publicManifestState.RLock()
	defer publicManifestState.RUnlock()
	if publicManifestState.entries == nil {
		return AssetEntry{}, false
	}
	entry, ok := publicManifestState.entries[name]
	return entry, ok
}

func manifestRegistered() bool {
	publicManifestState.RLock()
	defer publicManifestState.RUnlock()
	return publicManifestState.entries != nil
}

// selectRepresentation applies media-type negotiation and then content-coding
// negotiation, in that order: a compressed sibling exists per representation
// and not per URL, so the media type has to be settled first.
//
// It reports the chosen representation, or false when every representation the
// entry holds was explicitly refused.
func selectRepresentation(entry AssetEntry, accept []string, acceptEncoding []string) (AssetRepresentation, bool) {
	candidates := acceptableByMediaType(entry, accept)
	if len(candidates) == 0 {
		return AssetRepresentation{}, false
	}
	identity, hasIdentity := AssetRepresentation{}, false
	var encoded [maxStaticCodings]AssetRepresentation
	var hasEncoded [maxStaticCodings]bool
	chosen := candidates[0].MediaType
	for _, candidate := range candidates {
		if candidate.MediaType != chosen {
			continue
		}
		if candidate.ContentEncoding == "" {
			if !hasIdentity {
				identity, hasIdentity = candidate, true
			}
			continue
		}
		// A coding this build does not negotiate is skipped rather than
		// refused: the manifest describes what exists, and what to offer is
		// this middleware's judgment.
		rank := staticCodingRank(candidate.ContentEncoding)
		if rank < 0 || hasEncoded[rank] {
			continue
		}
		encoded[rank], hasEncoded[rank] = candidate, true
	}
	if len(acceptEncoding) > 0 {
		quality := scanEncodingQuality(strings.Join(acceptEncoding, ","))
		// The order is the build's, not the client's q-values: which of two
		// acceptable representations is worth sending is a statement about the
		// bytes, and the header only says what can be read.
		for rank := range encoded {
			if hasEncoded[rank] && quality.acceptsCoding(rank) > 0 {
				return encoded[rank], true
			}
		}
		if !hasIdentity {
			// A URL stored only in encoded forms cannot answer a client that
			// refuses all of them, which is a build mistake rather than a
			// negotiation outcome.
			return AssetRepresentation{}, false
		}
		if quality.acceptsIdentity() <= 0 {
			return AssetRepresentation{}, false
		}
		return identity, true
	}
	if !hasIdentity {
		return AssetRepresentation{}, false
	}
	return identity, true
}

// encodingQuality holds the q-values for the only tokens a negotiation ever
// asks about: the codings this middleware serves, identity, and the wildcard.
// Scanning for exactly these keeps the parse off the heap; a full
// token-to-quality map was allocated per request and then read three times.
type encodingQuality struct {
	coding             [maxStaticCodings]float64
	codingSet          [maxStaticCodings]bool
	identity, wildcard float64
	identitySet        bool
	wildcardSet        bool
}

// acceptsCoding reports the effective q-value for one coding rank, falling back
// to the wildcard the client stated and to zero when it stated neither. A
// coding nobody mentioned is not acceptable, which is what keeps an encoded
// body away from a client that only said gzip.
func (q encodingQuality) acceptsCoding(rank int) float64 {
	if q.codingSet[rank] {
		return q.coding[rank]
	}
	if q.wildcardSet {
		return q.wildcard
	}
	return 0
}

// acceptsIdentity defaults to acceptable, because a client that names no
// coding at all is asking for the bytes as they are.
func (q encodingQuality) acceptsIdentity() float64 {
	if q.identitySet {
		return q.identity
	}
	if q.wildcardSet {
		return q.wildcard
	}
	return 1
}

// scanEncodingQuality reads an Accept-Encoding value for every coding this
// middleware serves, identity, and the wildcard. A duplicated token keeps its
// last q-value, as the map it replaces did.
//
// It scans for the whole coding set rather than for one named coding, because
// an asset now stores several and the alternative was a pass per representation
// over the same header.
func scanEncodingQuality(header string) encodingQuality {
	var result encodingQuality
	for remainder := header; remainder != ""; {
		var item string
		item, remainder, _ = strings.Cut(remainder, ",")
		token, parameters, _ := strings.Cut(item, ";")
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		quality := 1.0
		for parameters != "" {
			var parameter string
			parameter, parameters, _ = strings.Cut(parameters, ";")
			name, value, found := strings.Cut(strings.TrimSpace(parameter), "=")
			if !found || !strings.EqualFold(strings.TrimSpace(name), "q") {
				continue
			}
			parsed, err := parseQuality(strings.TrimSpace(value))
			if err != nil {
				quality = 0
			} else {
				quality = parsed
			}
		}
		switch {
		case strings.EqualFold(token, "identity"):
			result.identity, result.identitySet = quality, true
		case token == "*":
			result.wildcard, result.wildcardSet = quality, true
		default:
			if rank := staticCodingRank(token); rank >= 0 {
				result.coding[rank], result.codingSet[rank] = quality, true
			}
		}
	}
	return result
}

// acceptableByMediaType filters an entry to the representations the request
// will take, keeping the build's preference order rather than the client's
// q-values: the ordering is a statement about which bytes are worth serving,
// and the header only says what can be read.
func acceptableByMediaType(entry AssetEntry, accept []string) []AssetRepresentation {
	if len(entry.Representations) == 0 {
		return nil
	}
	if !entryNegotiatesMedia(entry) {
		return entry.Representations
	}
	ranges := parseMediaRanges(strings.Join(accept, ","))
	if len(ranges) == 0 {
		return defaultMediaRepresentations(entry)
	}
	var acceptable []AssetRepresentation
	for _, representation := range entry.Representations {
		if mediaQuality(ranges, representation.MediaType) > 0 {
			acceptable = append(acceptable, representation)
		}
	}
	if len(acceptable) == 0 {
		fallback := defaultMediaRepresentations(entry)
		if mediaQuality(ranges, fallback[0].MediaType) == 0 && explicitlyRefused(ranges, fallback[0].MediaType) {
			return nil
		}
		return fallback
	}
	return acceptable
}

// entryNegotiatesMedia reports whether an entry holds more than one media type,
// which is the only case where the Accept header can change an answer.
func entryNegotiatesMedia(entry AssetEntry) bool {
	if len(entry.Representations) == 0 {
		return false
	}
	first := entry.Representations[0].MediaType
	for _, representation := range entry.Representations[1:] {
		if representation.MediaType != first {
			return true
		}
	}
	return false
}

// defaultMediaRepresentations returns the least-preferred media type, which the
// build guarantees every client can read.
func defaultMediaRepresentations(entry AssetEntry) []AssetRepresentation {
	fallback := entry.Representations[len(entry.Representations)-1].MediaType
	var result []AssetRepresentation
	for _, representation := range entry.Representations {
		if representation.MediaType == fallback {
			result = append(result, representation)
		}
	}
	return result
}

type mediaRange struct {
	kind    string
	subtype string
	quality float64
}

func parseMediaRanges(header string) []mediaRange {
	var result []mediaRange
	for _, item := range strings.Split(header, ",") {
		parts := strings.Split(item, ";")
		token := strings.ToLower(strings.TrimSpace(parts[0]))
		if token == "" {
			continue
		}
		kind, subtype, ok := strings.Cut(token, "/")
		if !ok {
			continue
		}
		quality := 1.0
		for _, parameter := range parts[1:] {
			name, value, found := strings.Cut(strings.TrimSpace(parameter), "=")
			if !found || !strings.EqualFold(strings.TrimSpace(name), "q") {
				continue
			}
			parsed, err := parseQuality(strings.TrimSpace(value))
			if err != nil {
				quality = 0
			} else {
				quality = parsed
			}
		}
		result = append(result, mediaRange{kind: kind, subtype: subtype, quality: quality})
	}
	return result
}

// mediaQuality scores one media type against the parsed ranges, preferring the
// most specific match, as RFC 9110 requires.
func mediaQuality(ranges []mediaRange, mediaType string) float64 {
	kind, subtype, ok := strings.Cut(strings.ToLower(mediaTypeName(mediaType)), "/")
	if !ok {
		return 0
	}
	best, matched := 0.0, 0
	for _, item := range ranges {
		specificity := 0
		switch {
		case item.kind == kind && item.subtype == subtype:
			specificity = 3
		case item.kind == kind && item.subtype == "*":
			specificity = 2
		case item.kind == "*" && item.subtype == "*":
			specificity = 1
		default:
			continue
		}
		if specificity > matched {
			best, matched = item.quality, specificity
		}
	}
	return best
}

// explicitlyRefused separates "not mentioned" from "mentioned with q=0", since
// only the second is a refusal worth a 406.
func explicitlyRefused(ranges []mediaRange, mediaType string) bool {
	kind, subtype, ok := strings.Cut(strings.ToLower(mediaTypeName(mediaType)), "/")
	if !ok {
		return false
	}
	for _, item := range ranges {
		if item.quality > 0 {
			continue
		}
		if (item.kind == kind && (item.subtype == subtype || item.subtype == "*")) ||
			(item.kind == "*" && item.subtype == "*") {
			return true
		}
	}
	return false
}

// parseQuality reads a q-value, refusing anything outside the range the
// grammar allows rather than clamping it into range.
func parseQuality(value string) (float64, error) {
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil || parsed < 0 || parsed > 1 {
		return 0, errQuality
	}
	return parsed, nil
}

var errQuality = errors.New("invalid q-value")

func mediaTypeName(value string) string {
	if separator := strings.IndexByte(value, ';'); separator >= 0 {
		return strings.TrimSpace(value[:separator])
	}
	return strings.TrimSpace(value)
}
