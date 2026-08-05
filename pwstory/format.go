package pwstory

import "strings"

// voidElements close themselves, so they neither indent what follows nor expect
// a closing tag.
var voidElements = map[string]bool{
	"area": true, "base": true, "br": true, "col": true, "embed": true,
	"hr": true, "img": true, "input": true, "link": true, "meta": true,
	"param": true, "source": true, "track": true, "wbr": true,
}

// prettyHTML re-indents generated markup so it can be read.
//
// Template output is emitted for a browser rather than for a person: it arrives
// as one line, and the escaping context a developer opens this page to check is
// invisible inside it. This adds line breaks and indentation and changes
// nothing else — no attribute is reordered, no whitespace inside a text node is
// touched beyond trimming the run between tags, so what is shown is still what
// was produced.
func prettyHTML(source string) string {
	var out strings.Builder
	depth := 0
	indent := func() {
		if out.Len() > 0 {
			out.WriteByte('\n')
		}
		out.WriteString(strings.Repeat("  ", max(depth, 0)))
	}
	for index := 0; index < len(source); {
		next := strings.IndexByte(source[index:], '<')
		if next < 0 {
			if text := strings.TrimSpace(source[index:]); text != "" {
				indent()
				out.WriteString(text)
			}
			break
		}
		if text := strings.TrimSpace(source[index : index+next]); text != "" {
			indent()
			out.WriteString(text)
		}
		index += next
		end := strings.IndexByte(source[index:], '>')
		if end < 0 {
			indent()
			out.WriteString(source[index:])
			break
		}
		tag := source[index : index+end+1]
		closing := strings.HasPrefix(tag, "</")
		if closing {
			depth--
		}
		indent()
		out.WriteString(tag)
		if !closing && !strings.HasSuffix(tag, "/>") && !isVoid(tag) && !strings.HasPrefix(tag, "<!") {
			depth++
		}
		index += end + 1
	}
	return out.String()
}

func isVoid(tag string) bool {
	name := strings.TrimLeft(tag, "<")
	for cut := 0; cut < len(name); cut++ {
		if name[cut] == ' ' || name[cut] == '>' || name[cut] == '/' {
			name = name[:cut]
			break
		}
	}
	return voidElements[strings.ToLower(name)]
}
