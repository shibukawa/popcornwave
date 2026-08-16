package pwruntime

import (
	"strconv"
	"strings"

	"github.com/shibukawa/tinybind-go/htmlbind"
)

// Segment is one piece of a generated message row: either a literal run of the
// translation, or the position an argument fills.
//
// The table is data and the renderer below is code, which is what makes adding
// a locale cost no generated code at all — only another row. See
// .knowledge decision:message-code-shape.
type Segment struct {
	// Lit is the literal run, used when Arg is zero.
	Lit string
	// Arg is the one-based argument position this segment renders, zero for a
	// literal. It is one-based so the zero value is a literal rather than the
	// first argument.
	Arg uint8
}

// RichSegment is a Segment that may also open a hole. A hole marker carries the
// translated text that belongs inside the template's markup.
type RichSegment struct {
	// Hole names the hole this segment fills, empty for an ordinary run.
	Hole string
	Lit  string
	Arg  uint8
}

// RenderMessage assembles one row into a string.
//
// The result is unescaped: a message is an ordinary string value and the
// template escapes it once, together with its arguments, for the position it
// lands in. Escaping here would double-escape every one. See
// .knowledge decision:message-code-shape escaping_stays_where_it_is.
//
// A row of exactly one literal segment is returned directly, so a message
// carrying no argument costs a table read and no allocation. Generated code for
// a message with no parameters skips this function entirely and indexes a plain
// string table, which is the same saving one call earlier.
func RenderMessage(row []Segment, args ...string) string {
	if len(row) == 1 && row[0].Arg == 0 {
		return row[0].Lit
	}
	size := 0
	for _, segment := range row {
		if segment.Arg == 0 {
			size += len(segment.Lit)
			continue
		}
		if int(segment.Arg) <= len(args) {
			size += len(args[segment.Arg-1])
		}
	}
	var builder strings.Builder
	builder.Grow(size)
	for _, segment := range row {
		if segment.Arg == 0 {
			builder.WriteString(segment.Lit)
			continue
		}
		if int(segment.Arg) <= len(args) {
			builder.WriteString(args[segment.Arg-1])
		}
	}
	return builder.String()
}

// RenderRichMessage assembles one row into the segment list system:tinybind
// interleaves with the markup a template bound to each hole.
//
// Adjacent literal runs are merged so a hole's text arrives as one segment,
// which is what the interleaver writes between a bound element's tags.
//
// This returns an upstream type on purpose. The generated function is the
// argument of htmlbind.Builder.Message, so the shape is fixed by that call
// rather than chosen here; policy:message-rich-text records why the framework
// produces the segments and upstream drives the interleaving.
func RenderRichMessage(row []RichSegment, args ...string) []htmlbind.MessageSegment {
	segments := make([]htmlbind.MessageSegment, 0, len(row))
	for _, segment := range row {
		text := segment.Lit
		if segment.Arg != 0 {
			text = ""
			if int(segment.Arg) <= len(args) {
				text = args[segment.Arg-1]
			}
		}
		if last := len(segments) - 1; last >= 0 && segments[last].Hole == segment.Hole {
			segments[last].Text += text
			continue
		}
		segments = append(segments, htmlbind.MessageSegment{Hole: segment.Hole, Text: text})
	}
	return segments
}

// NumberFormat is how one locale groups digits.
//
// Only grouping and the two separators are modelled. Digit shaping, currency
// placement, and ordinal words each need their own tables, and a message
// argument is a count or a quantity rather than a formatted money value — the
// framework offering half of currency formatting would be worse than offering
// none.
type NumberFormat struct {
	// Group separates thousands, empty for a locale that does not group.
	Group string
	// Decimal separates the fractional part.
	Decimal string
}

var numberFormats = map[string]NumberFormat{
	// The comma-and-period group, which is also the fallback.
	"en": {Group: ",", Decimal: "."}, "ja": {Group: ",", Decimal: "."},
	"ko": {Group: ",", Decimal: "."}, "zh": {Group: ",", Decimal: "."},
	"he": {Group: ",", Decimal: "."}, "th": {Group: ",", Decimal: "."},
	// Period groups, comma decimals.
	"de": {Group: ".", Decimal: ","}, "es": {Group: ".", Decimal: ","},
	"it": {Group: ".", Decimal: ","}, "pt": {Group: ".", Decimal: ","},
	"nl": {Group: ".", Decimal: ","}, "id": {Group: ".", Decimal: ","},
	"tr": {Group: ".", Decimal: ","}, "da": {Group: ".", Decimal: ","},
	// Space groups, comma decimals. The separator is a non-breaking space, so a
	// grouped number never wraps across a line in the middle of itself.
	"fr": {Group: "\u00a0", Decimal: ","}, "ru": {Group: "\u00a0", Decimal: ","},
	"pl": {Group: "\u00a0", Decimal: ","}, "cs": {Group: "\u00a0", Decimal: ","},
	"sk": {Group: "\u00a0", Decimal: ","}, "sv": {Group: "\u00a0", Decimal: ","},
	"fi": {Group: "\u00a0", Decimal: ","}, "uk": {Group: "\u00a0", Decimal: ","},
	"nb": {Group: "\u00a0", Decimal: ","}, "hu": {Group: "\u00a0", Decimal: ","},
	// No grouping.
	"ar": {Group: "", Decimal: "."},
}

// FormatNumber reports the grouping of a locale, falling back to the
// comma-and-period convention for one this build does not tabulate.
//
// The fallback is a convention rather than a guess at the locale: a number that
// groups the wrong way is legible, and one that carries no separators at all is
// legible too, so neither failure is worth an error that stops a page.
func FormatNumber(locale Locale) NumberFormat {
	if format, ok := numberFormats[baseLanguageOf(locale.Tag())]; ok {
		return format
	}
	return NumberFormat{Group: ",", Decimal: "."}
}

func baseLanguageOf(tag string) string {
	if cut := strings.IndexAny(tag, "-_"); cut >= 0 {
		tag = tag[:cut]
	}
	return strings.ToLower(tag)
}

// FormatInt renders an integer argument for a message, grouped for the locale.
//
// Plural selection reads the value and never this string, so a locale whose
// grouping is unknown still selects the right form.
func FormatInt(locale Locale, value int) string {
	digits := strconv.Itoa(value)
	format := FormatNumber(locale)
	if format.Group == "" {
		return digits
	}
	sign := ""
	if strings.HasPrefix(digits, "-") {
		sign, digits = "-", digits[1:]
	}
	if len(digits) <= 3 {
		return sign + digits
	}
	var out strings.Builder
	lead := len(digits) % 3
	if lead == 0 {
		lead = 3
	}
	out.WriteString(digits[:lead])
	for i := lead; i < len(digits); i += 3 {
		out.WriteString(format.Group)
		out.WriteString(digits[i : i+3])
	}
	return sign + out.String()
}
