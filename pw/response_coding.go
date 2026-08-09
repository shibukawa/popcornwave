package pw

import (
	"io"
	"strconv"
	"strings"
)

// responseEncoder is the shape every dynamic content coding presents to the
// response path. All of them supply the same four operations, so negotiation
// holds a constructor rather than a type: a coding is a registry entry, and
// adding one touches nothing that renders.
//
// Flush emits everything written so far without ending the stream, which is
// what the streaming branch is built on. It deliberately does not flush the
// destination; flushingEncoder chains that.
//
// Abort discards a frame that was never committed, so a problem response can
// still replace the body.
type responseEncoder interface {
	io.Writer
	Flush() error
	Close() error
	Abort()
}

// responseCoding pairs a content-coding token with the encoder producing it.
type responseCoding struct {
	token      string
	newEncoder func(io.Writer) (responseEncoder, error)
}

// maxResponseCodings bounds the negotiation scratch space, so a request reads
// its Accept-Encoding without touching the heap. It counts the codings this
// framework can encode, not the tokens a header may name.
const maxResponseCodings = 2

// availableResponseCodings is every coding this build can encode, in the
// framework's default preference order.
//
// zstd leads because it stays ahead of gzip on ratio at the levels both run,
// and because a client already receiving zstd should keep receiving it, which
// makes gzip purely additive. brotli is absent on purpose: it lives in the
// asset build, where its cost lands on a machine that is not answering a
// request.
//
// A build tag that removes an encoder removes its entry here, which is what
// makes middleware.compression a silent no-op on such a build rather than a
// link error.
var availableResponseCodings = func() []responseCoding {
	codings := make([]responseCoding, 0, maxResponseCodings)
	if zstdResponseSupported {
		codings = append(codings, responseCoding{token: zstdContentEncoding, newEncoder: newResponseZstdEncoder})
	}
	if gzipResponseSupported {
		codings = append(codings, responseCoding{token: gzipContentEncoding, newEncoder: newResponseGzipEncoder})
	}
	return codings
}()

// orderedResponseCodings resolves a configured order against what the build can
// encode, writing into caller-owned scratch so a request allocates nothing.
//
// A coding left out of the list is not offered even when the client asks for
// it, which is what lets one field express removal as well as ordering. A
// coding named but absent from the build is dropped rather than refused,
// because the build-time decision has to win over the configuration one.
func orderedResponseCodings(names []string, scratch *[maxResponseCodings]responseCoding) []responseCoding {
	if len(names) == 0 {
		return availableResponseCodings
	}
	ordered := scratch[:0]
	for token := range codingTokens(names) {
		for _, coding := range availableResponseCodings {
			if !strings.EqualFold(token, coding.token) {
				continue
			}
			if codingListed(ordered, coding.token) {
				break
			}
			ordered = append(ordered, coding)
			break
		}
	}
	if len(ordered) == 0 && !namesAnyKnown(names) {
		// A list holding nothing this framework understands is the same as no
		// list. It cannot reach here past validation, but a value assembled at
		// runtime should degrade to the default rather than to identity.
		return availableResponseCodings
	}
	return ordered
}

// codingTokens yields the coding names a configured list holds.
//
// One entry may carry several comma-separated names, because a list set from a
// single environment variable or a scalar arrives as one string while the same
// list written as a TOML array arrives as several. Both spellings mean the same
// thing, and the difference should not reach anything that negotiates.
func codingTokens(names []string) func(func(string) bool) {
	return func(yield func(string) bool) {
		for _, name := range names {
			for token := range splitSeq(name, ',') {
				token = strings.TrimSpace(token)
				if token == "" {
					continue
				}
				if !yield(token) {
					return
				}
			}
		}
	}
}

func namesAnyKnown(names []string) bool {
	for token := range codingTokens(names) {
		if knownResponseCoding(token) {
			return true
		}
	}
	return false
}

func codingListed(codings []responseCoding, token string) bool {
	for _, coding := range codings {
		if coding.token == token {
			return true
		}
	}
	return false
}

// knownResponseCoding reports whether a token names a dynamic coding this
// framework understands, whatever this build can encode.
//
// A build tag removes an encoder, not the vocabulary. A configuration file
// naming zstd has to keep starting on a binary built without it, or a smaller
// target would need its own configuration file to say the same thing.
func knownResponseCoding(name string) bool {
	return strings.EqualFold(name, zstdContentEncoding) || strings.EqualFold(name, gzipContentEncoding)
}

// unavailableResponseCodings names the configured codings this build cannot
// produce, so startup can say so rather than leave a compression setting that
// quietly does nothing.
func unavailableResponseCodings(names []string) []string {
	var missing []string
	for token := range codingTokens(names) {
		available := false
		for _, coding := range availableResponseCodings {
			if strings.EqualFold(token, coding.token) {
				available = true
				break
			}
		}
		if !available {
			missing = append(missing, token)
		}
	}
	return missing
}

// negotiateResponseCoding picks the first offered coding the client will take.
//
// The order is the deployment's, never the client's q-values: a q-value says
// what can be read, and which of two readable codings is worth spending CPU on
// is not the client's judgment to make. A q-value still excludes, which is the
// half of the header that is a statement about capability.
func negotiateResponseCoding(values []string, order []responseCoding) (responseCoding, bool) {
	if len(order) == 0 {
		return responseCoding{}, false
	}
	var quality [maxResponseCodings]float64
	var named [maxResponseCodings]bool
	wildcard, wildcardNamed := 0.0, false
	for _, value := range values {
		// Cut loops rather than Split, so a header line parses without
		// allocating.
		for entry := range splitSeq(value, ',') {
			token, parameters, _ := strings.Cut(entry, ";")
			token = strings.TrimSpace(token)
			if token == "" {
				continue
			}
			if token == "*" {
				wildcard, wildcardNamed = codingQuality(parameters), true
				continue
			}
			for i := range order {
				if strings.EqualFold(token, order[i].token) {
					quality[i], named[i] = codingQuality(parameters), true
					break
				}
			}
		}
	}
	for i := range order {
		acceptable := quality[i]
		if !named[i] {
			if !wildcardNamed {
				continue
			}
			acceptable = wildcard
		}
		if acceptable > 0 {
			return order[i], true
		}
	}
	return responseCoding{}, false
}

// codingQuality reads the q parameter of one Accept-Encoding entry. A malformed
// or out-of-range value counts as unacceptable rather than as the default, so a
// header this parser cannot agree with never wins a coding.
func codingQuality(parameters string) float64 {
	quality := 1.0
	for parameter := range splitSeq(parameters, ';') {
		name, raw, ok := strings.Cut(parameter, "=")
		if !ok || !strings.EqualFold(strings.TrimSpace(name), "q") {
			continue
		}
		parsed, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
		if err != nil || parsed < 0 || parsed > 1 {
			quality = 0
		} else {
			quality = parsed
		}
	}
	return quality
}
