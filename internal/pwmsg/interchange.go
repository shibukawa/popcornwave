package pwmsg

import (
	"bufio"
	"bytes"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// ExportPO renders one locale as a gettext PO catalog.
//
// PO rather than XLIFF because the mapping is exact and the format is
// line-based: msgctxt carries the message ID, msgid the source text, and msgstr
// the translation. That is the same asymmetry decision:message-source-of-truth
// records — the source is shown to the translator and is not theirs to edit —
// so nothing has to be invented to express it.
//
// The source text comes from the declared source locale, so a translator sees
// what they are translating from rather than an identifier.
func ExportPO(catalog *Catalog, locale string) []byte {
	var out bytes.Buffer
	fmt.Fprintf(&out, "msgid \"\"\nmsgstr \"\"\n")
	fmt.Fprintf(&out, "\"Language: %s\\n\"\n", locale)
	fmt.Fprintf(&out, "\"Content-Type: text/plain; charset=UTF-8\\n\"\n")

	for _, scope := range catalog.Scopes {
		for _, entry := range scope.Entries {
			id := entry.Qualified(scope.Name)
			source, hasSource := entry.Texts[catalog.Default]
			target, hasTarget := entry.Texts[locale]

			fmt.Fprintf(&out, "\n#: %s\n", scope.Path)
			if entry.Snapshot != "" && hasSource && firstForm(source) != entry.Snapshot {
				// The source moved after the ID was assigned, so every
				// translation of it is suspect. Fuzzy is what a PO tool shows a
				// translator for exactly that.
				fmt.Fprintf(&out, "#, fuzzy\n")
			}
			fmt.Fprintf(&out, "msgctxt %s\n", quotePO(id))

			sourceForms := []string{""}
			if hasSource {
				sourceForms = textForms(source)
			}
			if entry.Plural == "" {
				fmt.Fprintf(&out, "msgid %s\n", quotePO(sourceForms[0]))
				body := ""
				if hasTarget {
					body = firstForm(target)
				}
				fmt.Fprintf(&out, "msgstr %s\n", quotePO(body))
				continue
			}
			fmt.Fprintf(&out, "msgid %s\n", quotePO(sourceForms[0]))
			plural := sourceForms[0]
			if len(sourceForms) > 1 {
				plural = sourceForms[len(sourceForms)-1]
			}
			fmt.Fprintf(&out, "msgid_plural %s\n", quotePO(plural))
			rule, _ := RuleFor(locale)
			for index, category := range rule.Categories {
				body := ""
				if hasTarget && target.Variants != nil {
					body = target.Variants[category]
				} else if hasTarget {
					body = target.Simple
				}
				fmt.Fprintf(&out, "msgstr[%d] %s\n", index, quotePO(body))
			}
		}
	}
	return out.Bytes()
}

// POEntry is one translated unit read back from a PO file.
type POEntry struct {
	ID       string
	Simple   string
	Variants map[Category]string
}

// ImportPO reads translations back.
//
// Only msgstr is read. The source text and the ID are the catalog's, and a PO
// file that disagrees about either is reporting an edit a translator should not
// have been able to make — so it is ignored rather than applied.
func ImportPO(data []byte, locale string) ([]POEntry, error) {
	rule, _ := RuleFor(locale)
	var entries []POEntry
	var current *POEntry

	flush := func() {
		if current != nil && current.ID != "" {
			entries = append(entries, *current)
		}
		current = nil
	}

	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		switch {
		case line == "" || strings.HasPrefix(line, "#"):
			if line == "" {
				flush()
			}
		case strings.HasPrefix(line, "msgctxt "):
			flush()
			id, err := unquotePO(strings.TrimPrefix(line, "msgctxt "))
			if err != nil {
				return nil, err
			}
			current = &POEntry{ID: id}
		case strings.HasPrefix(line, "msgstr["):
			if current == nil {
				continue
			}
			close := strings.IndexByte(line, ']')
			if close < 0 {
				return nil, fmt.Errorf("malformed plural index in %q", line)
			}
			index, err := strconv.Atoi(line[len("msgstr["):close])
			if err != nil {
				return nil, fmt.Errorf("malformed plural index in %q", line)
			}
			body, err := unquotePO(strings.TrimSpace(line[close+1:]))
			if err != nil {
				return nil, err
			}
			if index >= len(rule.Categories) {
				return nil, fmt.Errorf("message %q carries plural form %d, but %s distinguishes %d", current.ID, index, locale, len(rule.Categories))
			}
			if current.Variants == nil {
				current.Variants = map[Category]string{}
			}
			current.Variants[rule.Categories[index]] = body
		case strings.HasPrefix(line, "msgstr "):
			if current == nil {
				continue
			}
			body, err := unquotePO(strings.TrimPrefix(line, "msgstr "))
			if err != nil {
				return nil, err
			}
			current.Simple = body
		}
	}
	flush()
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].ID < entries[j].ID })
	return entries, nil
}

func quotePO(text string) string {
	escaped := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`, "\t", `\t`).Replace(text)
	return `"` + escaped + `"`
}

func unquotePO(text string) (string, error) {
	text = strings.TrimSpace(text)
	if len(text) < 2 || text[0] != '"' || text[len(text)-1] != '"' {
		return "", fmt.Errorf("expected a quoted string, got %q", text)
	}
	body := text[1 : len(text)-1]
	var out strings.Builder
	for i := 0; i < len(body); i++ {
		if body[i] != '\\' || i+1 >= len(body) {
			out.WriteByte(body[i])
			continue
		}
		i++
		switch body[i] {
		case 'n':
			out.WriteByte('\n')
		case 't':
			out.WriteByte('\t')
		default:
			out.WriteByte(body[i])
		}
	}
	return out.String(), nil
}

// RenameEntry rewrites one scope file so an ID becomes another.
//
// It edits the key line textually rather than re-marshalling, so the comments,
// the ordering, and the formatting a translator put in the file survive. A
// rename is one line changing; nothing else about the file should move.
func RenameEntry(source []byte, from, to string) ([]byte, bool) {
	lines := strings.Split(string(source), "\n")
	renamed := false
	for i, line := range lines {
		if line == from+":" || strings.HasPrefix(line, from+": ") {
			lines[i] = to + strings.TrimPrefix(line, from)
			renamed = true
		}
	}
	if !renamed {
		return source, false
	}
	return []byte(strings.Join(lines, "\n")), true
}
