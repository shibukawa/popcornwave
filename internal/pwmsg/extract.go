package pwmsg

import (
	"fmt"
	"sort"
	"strings"

	templates "github.com/shibukawa/tinybind-go/templates/htmlbind"
)

// Mark is one piece of source text an author flagged for extraction.
//
// The flag is an ordinary HTML attribute, so it needs no grammar of its own and
// is removed by the rewrite. See .knowledge decision:message-id-assignment.
type Mark struct {
	// Text is the source text that becomes the message.
	Text string
	// Start and End are the byte range the reference replaces. The range is
	// source rather than content, so replacing it also replaces any escape the
	// text contained.
	Start, End int
	// AttributeRange is the range of the i18n attribute itself, removed by the
	// same rewrite.
	AttributeStart, AttributeEnd int
	// Element names what carried the mark, used when a slug cannot be derived
	// from the text.
	Element string
	// Attribute names the attribute whose value was marked, empty for element
	// text.
	Attribute string
	// Line is where the mark is, for reporting.
	Line int
}

// MarkProblem is a mark this extractor declines to act on.
type MarkProblem struct {
	Line    int
	Element string
	Reason  string
}

// ExtractMarks reports every marked string in one template.
//
// A mark on an element whose children are not one run of text is declined
// rather than guessed at: that shape is rich text, whose holes name markup the
// template supplies, and a tool inventing hole names would produce translations
// no translator can check. It is reported so the author converts it by hand.
func ExtractMarks(filename string, source []byte) ([]Mark, []MarkProblem, error) {
	module, err := templates.Parse(filename, source)
	if err != nil {
		return nil, nil, err
	}
	var marks []Mark
	var problems []MarkProblem
	for _, declaration := range module.Declarations {
		template, ok := declaration.(*templates.TemplateDecl)
		if !ok {
			continue
		}
		body, ok := template.Body.(templates.Body)
		if !ok {
			continue
		}
		found, declined := walkForMarks(body, source)
		marks = append(marks, found...)
		problems = append(problems, declined...)
	}
	sort.Slice(marks, func(i, j int) bool { return marks[i].Start < marks[j].Start })
	return marks, problems, nil
}

func walkForMarks(nodes []templates.Node, source []byte) ([]Mark, []MarkProblem) {
	var marks []Mark
	var problems []MarkProblem
	for _, node := range nodes {
		element, ok := node.(*templates.ElementNode)
		if !ok {
			continue
		}
		for _, attribute := range element.Attributes {
			if attribute.Name != "i18n" {
				continue
			}
			target := attributeValueText(attribute)
			start, end := attributeRange(source, attribute)
			if target == "" {
				// A bare i18n attribute marks the element's own text.
				mark, problem := markElementText(element, start, end)
				if problem != nil {
					problems = append(problems, *problem)
					continue
				}
				marks = append(marks, *mark)
				continue
			}
			marked, ok := findAttribute(element, target)
			if !ok {
				problems = append(problems, MarkProblem{
					Line: element.Pos.Line, Element: element.Name,
					Reason: fmt.Sprintf("i18n names attribute %q, which this element does not carry", target)})
				continue
			}
			text, literal := attributeLiteral(marked)
			if !literal {
				problems = append(problems, MarkProblem{
					Line: element.Pos.Line, Element: element.Name,
					Reason: fmt.Sprintf("attribute %q is not a literal string, so there is nothing to extract", target)})
				continue
			}
			marks = append(marks, Mark{
				Text: text, Start: marked.Value[0].Start, End: marked.Value[len(marked.Value)-1].End,
				AttributeStart: start, AttributeEnd: end,
				Element: element.Name, Attribute: target, Line: element.Pos.Line,
			})
		}
		nested, declined := walkForMarks(element.Children, source)
		marks = append(marks, nested...)
		problems = append(problems, declined...)
	}
	return marks, problems
}

func markElementText(element *templates.ElementNode, attributeStart, attributeEnd int) (*Mark, *MarkProblem) {
	if len(element.Children) != 1 {
		return nil, &MarkProblem{
			Line: element.Pos.Line, Element: element.Name,
			Reason: "its content is not one run of text; a sentence carrying markup is rich text, whose hole names have to be chosen by hand"}
	}
	text, ok := element.Children[0].(*templates.TextNode)
	if !ok {
		return nil, &MarkProblem{
			Line: element.Pos.Line, Element: element.Name,
			Reason: "its content is not literal text"}
	}
	if strings.TrimSpace(text.Text) == "" {
		return nil, &MarkProblem{
			Line: element.Pos.Line, Element: element.Name, Reason: "it holds no text"}
	}
	return &Mark{
		Text: text.Text, Start: text.Start, End: text.End,
		AttributeStart: attributeStart, AttributeEnd: attributeEnd,
		Element: element.Name, Line: element.Pos.Line,
	}, nil
}

func findAttribute(element *templates.ElementNode, name string) (templates.Attribute, bool) {
	for _, attribute := range element.Attributes {
		if attribute.Name == name {
			return attribute, true
		}
	}
	return templates.Attribute{}, false
}

func attributeValueText(attribute templates.Attribute) string {
	text, _ := attributeLiteral(attribute)
	return text
}

// attributeLiteral reports the literal text of an attribute, and whether every
// part of it was literal. A value carrying an expression has nothing to extract.
func attributeLiteral(attribute templates.Attribute) (string, bool) {
	if len(attribute.Value) == 0 {
		return "", false
	}
	var out strings.Builder
	for _, part := range attribute.Value {
		if part.Expression != nil {
			return "", false
		}
		out.WriteString(part.Text)
	}
	return out.String(), true
}

// attributeRange finds the source range of one attribute, including the
// whitespace before it, so removing it leaves no double space behind.
//
// It is scanned from the attribute's own position rather than derived from its
// value range, because a boolean attribute — which is the bare i18n mark, the
// commonest form — has no value range at all. The position carries a line and a
// column, so the offset is resolved against a line index of the source.
func attributeRange(source []byte, attribute templates.Attribute) (int, int) {
	start := offsetOf(source, attribute.Pos.Line, attribute.Pos.Col)
	if start < 0 || start+len(attribute.Name) > len(source) {
		return -1, -1
	}
	end := start + len(attribute.Name)
	// Past name="value" or name='value', if there is one.
	if rest := end; rest < len(source) && source[rest] == '=' {
		rest++
		if rest < len(source) && (source[rest] == '"' || source[rest] == '\'') {
			quote := source[rest]
			rest++
			for rest < len(source) && source[rest] != quote {
				rest++
			}
			if rest < len(source) {
				rest++
			}
		} else {
			for rest < len(source) && source[rest] != ' ' && source[rest] != '>' && source[rest] != '\n' {
				rest++
			}
		}
		end = rest
	}
	// Take the whitespace before the attribute with it, so the element does not
	// keep a gap where the mark used to be.
	for start > 0 && (source[start-1] == ' ' || source[start-1] == '\t') {
		start--
	}
	return start, end
}

// offsetOf converts a one-based line and column into a byte offset.
func offsetOf(source []byte, line, col int) int {
	if line < 1 || col < 1 {
		return -1
	}
	current, offset := 1, 0
	for current < line {
		next := bytesIndexByteFrom(source, offset, '\n')
		if next < 0 {
			return -1
		}
		offset = next + 1
		current++
	}
	offset += col - 1
	if offset > len(source) {
		return -1
	}
	return offset
}

func bytesIndexByteFrom(source []byte, from int, b byte) int {
	for i := from; i < len(source); i++ {
		if source[i] == b {
			return i
		}
	}
	return -1
}

// ProposeID derives a message ID from source text.
//
// It is best effort by design: a slug from Latin text is mechanical, and a slug
// from kanji needs readings this build deliberately does not ship. A text it
// cannot slug produces a positional name the author renames, which costs what
// naming a variable costs. See .knowledge decision:message-id-assignment.
func ProposeID(text string, element string, ordinal int) (id string, derived bool) {
	var out strings.Builder
	previousDash := true
	for _, r := range strings.ToLower(strings.TrimSpace(text)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			out.WriteRune(r)
			previousDash = false
		case r == ' ' || r == '-' || r == '_':
			if !previousDash && out.Len() > 0 {
				out.WriteByte('-')
				previousDash = true
			}
		}
		if out.Len() >= 40 {
			break
		}
	}
	slug := strings.Trim(out.String(), "-")
	if slug == "" || slug[0] >= '0' && slug[0] <= '9' {
		name := element
		if name == "" {
			name = "message"
		}
		return fmt.Sprintf("%s-%d", name, ordinal), false
	}
	return slug, true
}

// Rewrite replaces each mark with a reference and removes the mark attribute.
//
// Replacements are applied from the end of the file backwards, so an earlier
// range is not shifted by a later edit.
func Rewrite(source []byte, marks []Mark, ids []string) ([]byte, error) {
	if len(marks) != len(ids) {
		return nil, fmt.Errorf("%d marks and %d ids", len(marks), len(ids))
	}
	type edit struct {
		start, end int
		text       string
	}
	var edits []edit
	for i, mark := range marks {
		edits = append(edits, edit{start: mark.Start, end: mark.End, text: "{t " + ids[i] + "}"})
		if mark.AttributeStart >= 0 && mark.AttributeEnd > mark.AttributeStart {
			edits = append(edits, edit{start: mark.AttributeStart, end: mark.AttributeEnd})
		}
	}
	sort.Slice(edits, func(i, j int) bool { return edits[i].start > edits[j].start })
	out := append([]byte(nil), source...)
	for _, e := range edits {
		if e.start < 0 || e.end > len(out) || e.start > e.end {
			return nil, fmt.Errorf("edit range %d:%d is outside the source", e.start, e.end)
		}
		out = append(out[:e.start], append([]byte(e.text), out[e.end:]...)...)
	}
	return out, nil
}

// RenderEntry renders one catalog entry as YAML, ready to append to a scope
// file.
//
// It is appended as text rather than written by re-marshalling the file,
// because re-marshalling drops the comments and the ordering a translator put
// there and this tool has no business rewriting what it did not add.
func RenderEntry(id, text string, locales []string, sourceLocale string) string {
	var out strings.Builder
	fmt.Fprintf(&out, "\n%s:\n", id)
	fmt.Fprintf(&out, "  snapshot: %s\n", quoteYAML(text))
	fmt.Fprintf(&out, "  locales:\n")
	for _, tag := range locales {
		value := `""`
		if tag == sourceLocale {
			value = quoteYAML(text)
		}
		fmt.Fprintf(&out, "    %s: %s\n", tag, value)
	}
	return out.String()
}

func quoteYAML(text string) string {
	escaped := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`, "\t", `\t`).Replace(text)
	return `"` + escaped + `"`
}
