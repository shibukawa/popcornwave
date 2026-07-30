// Package pwtree lays out dotted configuration keys as an aligned tree.
//
// The startup summary and pw doctor both report resolved configuration, and a
// reader who has learned one layout should not have to learn a second. The
// layout lives here so there is one implementation of it rather than two that
// drift.
package pwtree

import (
	"strings"
	"unicode/utf8"
)

// Entry is one reported key: its dotted name, the value as it should be shown,
// and the layer that won it. Masking is applied before an entry reaches here.
type Entry struct {
	Key    string
	Value  string
	Source string
}

// Line is one rendered row. Label already carries its branch characters, and
// Column and ValueWidth align a value and its source mark with the siblings
// that share a parent rather than with the deepest key in the whole tree.
type Line struct {
	Label      string
	Column     int
	ValueWidth int
	Entry      *Entry
}

type node struct {
	name     string
	entry    *Entry
	children []*node
	index    map[string]*node
}

func (n *node) child(name string) *node {
	if existing, ok := n.index[name]; ok {
		return existing
	}
	created := &node{name: name, index: map[string]*node{}}
	n.index[name] = created
	n.children = append(n.children, created)
	return created
}

// Lines groups entries by their dotted key and returns the rendered rows in
// declaration order. Entries are not sorted: the caller's order is the order a
// reader sees, because provenance order carries meaning.
func Lines(entries []Entry) []Line {
	root := &node{index: map[string]*node{}}
	for index := range entries {
		current := root
		for _, part := range strings.Split(entries[index].Key, ".") {
			current = current.child(part)
		}
		current.entry = &entries[index]
	}
	return appendLines(root, "", nil)
}

func appendLines(parent *node, prefix string, lines []Line) []Line {
	column, valueWidth := 0, 0
	for _, child := range parent.children {
		if child.entry != nil {
			column = max(column, utf8.RuneCountInString(prefix+"├─ "+child.name)+2)
			valueWidth = max(valueWidth, utf8.RuneCountInString(DisplayValue(child.entry.Value)))
		}
	}
	for index, child := range parent.children {
		branch, indent := "├─ ", prefix+"│  "
		if index == len(parent.children)-1 {
			branch, indent = "└─ ", prefix+"   "
		}
		lines = append(lines, Line{
			Label: prefix + branch + child.name, Column: column, ValueWidth: valueWidth, Entry: child.entry,
		})
		lines = appendLines(child, indent, lines)
	}
	return lines
}

// DisplayValue renders an empty value as an explicit pair of quotes, so a key
// that resolved to nothing is distinguishable from one that was not reported.
func DisplayValue(value string) string {
	if value == "" {
		return `""`
	}
	return value
}

// Render writes the rows of Lines, padding each value and source mark into its
// sibling column. dim styles the source mark; pass a function that returns its
// argument unchanged to render without color.
func Render(out *strings.Builder, lines []Line, sourceTag func(string) string, dim func(string) string) {
	for _, line := range lines {
		out.WriteString(line.Label)
		if line.Entry != nil {
			value := DisplayValue(line.Entry.Value)
			out.WriteString(strings.Repeat(" ", line.Column-utf8.RuneCountInString(line.Label)))
			out.WriteString(value)
			if source := sourceTag(line.Entry.Source); source != "" {
				out.WriteString(strings.Repeat(" ", line.ValueWidth-utf8.RuneCountInString(value)))
				out.WriteString(dim("  ← " + source))
			}
		}
		out.WriteByte('\n')
	}
}
