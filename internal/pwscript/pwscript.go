// Package pwscript reads a component's script block far enough to answer the
// two questions generation asks of it: which handlers the component publishes,
// and which of its parameters the block asked to be given.
//
// It exists here rather than upstream because the block is authored browser
// code and this framework owns the browser half. The module reports the bytes
// and takes the answer back as a compile option, and reading them is ours.
//
// It is not a JavaScript parser and does not pretend to be one. It scans, which
// means it skips the four things a pattern match gets wrong — a string, a
// template literal, a comment, and a regular expression literal — and then
// reads structure only where the structure is the idiomatic one. Anything else
// is reported as unread rather than guessed at, because a wrong answer here
// publishes a handler that does not exist or a value nobody meant to disclose.
package pwscript

import (
	"fmt"
	"strings"
)

// Block is what one component's script block says about itself.
type Block struct {
	// Handlers are the names the returned object publishes, in source order.
	Handlers []string
	// Parameters are the component parameters the setup pattern destructured.
	Parameters []string
	// Unread says why a question could not be answered, empty when both were.
	//
	// It is a reason rather than a flag because it reaches a build log, where
	// "setup returns a variable" tells an author what to change and "could not
	// read the block" tells them to file a bug.
	Unread string
}

// Read scans one block.
//
// An error is returned only for a source this scanner cannot walk at all, such
// as one whose quoting never closes. A block it walks but cannot understand
// comes back with Unread set and both lists empty, which is the answer that
// makes generation check nothing rather than refuse everything.
func Read(source string) (Block, error) {
	tokens, err := scan(source)
	if err != nil {
		return Block{}, err
	}
	setup, ok := findSetup(tokens)
	if !ok {
		return Block{Unread: "the block exports no setup function"}, nil
	}
	block := Block{}
	parameters, reason := readParameters(tokens, setup)
	if reason != "" {
		return Block{Unread: reason}, nil
	}
	block.Parameters = parameters
	handlers, reason := readHandlers(tokens, setup)
	if reason != "" {
		block.Unread = reason
		return block, nil
	}
	block.Handlers = handlers
	return block, nil
}

// token is one piece of code with the quoting and commentary removed.
//
// Only what the reader below discriminates on is kept: a word, a punctuator,
// and the nesting depth it sits at. A string's contents are not a token at all,
// which is the point — a brace inside one is not a brace.
type token struct {
	text  string
	depth int
	// index is the byte offset, kept so a diagnostic can be positioned later.
	index int
}

// scan turns source into tokens, skipping what quoting hides.
func scan(source string) ([]token, error) {
	var tokens []token
	depth := 0
	for i := 0; i < len(source); {
		c := source[i]
		switch {
		case c == '/' && i+1 < len(source) && source[i+1] == '/':
			for i < len(source) && source[i] != '\n' {
				i++
			}
		case c == '/' && i+1 < len(source) && source[i+1] == '*':
			end := strings.Index(source[i+2:], "*/")
			if end < 0 {
				return nil, fmt.Errorf("pwscript: a block comment is never closed")
			}
			i += end + 4
		case c == '/' && regexCanStart(tokens):
			// A slash is division or a regular expression literal, and which one
			// is decided by what came before it. Getting this wrong is how a
			// scanner reads the rest of a file as a pattern.
			next, err := skipRegex(source, i)
			if err != nil {
				return nil, err
			}
			i = next
		case c == '"' || c == '\'' || c == '`':
			next, err := skipString(source, i)
			if err != nil {
				return nil, err
			}
			i = next
		case isWordByte(c):
			start := i
			for i < len(source) && isWordByte(source[i]) {
				i++
			}
			tokens = append(tokens, token{text: source[start:i], depth: depth, index: start})
		case c == '{' || c == '(' || c == '[':
			depth++
			tokens = append(tokens, token{text: string(c), depth: depth, index: i})
			i++
		case c == '}' || c == ')' || c == ']':
			tokens = append(tokens, token{text: string(c), depth: depth, index: i})
			depth--
			if depth < 0 {
				return nil, fmt.Errorf("pwscript: a closing bracket has no opening one")
			}
			i++
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			i++
		default:
			tokens = append(tokens, token{text: string(c), depth: depth, index: i})
			i++
		}
	}
	if depth != 0 {
		return nil, fmt.Errorf("pwscript: a bracket is never closed")
	}
	return tokens, nil
}

func isWordByte(c byte) bool {
	return c == '_' || c == '$' ||
		(c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

// regexCanStart reports whether a slash here opens a regular expression rather
// than dividing. A literal may only follow something that is not a value, which
// is what makes the test the previous token rather than the character.
func regexCanStart(tokens []token) bool {
	if len(tokens) == 0 {
		return true
	}
	previous := tokens[len(tokens)-1].text
	switch previous {
	case ")", "]", "}":
		return false
	}
	if isWordByte(previous[0]) {
		// A keyword may precede a literal; a name or a number may not.
		switch previous {
		case "return", "typeof", "instanceof", "in", "of", "new", "delete", "void",
			"case", "do", "else", "yield", "await", "throw":
			return true
		}
		return false
	}
	return true
}

// skipString walks past a quoted run, including a template literal, and returns
// the offset after it.
//
// A template's ${} may hold anything including another template, so the nesting
// is walked rather than assumed. Getting this wrong is how a scanner reads code
// as text.
func skipString(source string, i int) (int, error) {
	quote := source[i]
	i++
	for i < len(source) {
		switch source[i] {
		case '\\':
			i += 2
			continue
		case quote:
			return i + 1, nil
		case '$':
			if quote == '`' && i+1 < len(source) && source[i+1] == '{' {
				next, err := skipInterpolation(source, i+2)
				if err != nil {
					return 0, err
				}
				i = next
				continue
			}
		case '\n':
			if quote != '`' {
				return 0, fmt.Errorf("pwscript: a string literal is never closed")
			}
		}
		i++
	}
	return 0, fmt.Errorf("pwscript: a string literal is never closed")
}

// skipInterpolation walks a template's ${ ... } to its closing brace.
func skipInterpolation(source string, i int) (int, error) {
	depth := 1
	for i < len(source) {
		switch source[i] {
		case '"', '\'', '`':
			next, err := skipString(source, i)
			if err != nil {
				return 0, err
			}
			i = next
			continue
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i + 1, nil
			}
		}
		i++
	}
	return 0, fmt.Errorf("pwscript: a template interpolation is never closed")
}

// skipRegex walks past a regular expression literal and its flags.
func skipRegex(source string, i int) (int, error) {
	i++
	inClass := false
	for i < len(source) {
		switch source[i] {
		case '\\':
			i += 2
			continue
		case '[':
			inClass = true
		case ']':
			inClass = false
		case '/':
			if !inClass {
				i++
				for i < len(source) && isWordByte(source[i]) {
					i++
				}
				return i, nil
			}
		case '\n':
			return 0, fmt.Errorf("pwscript: a regular expression literal is never closed")
		}
		i++
	}
	return 0, fmt.Errorf("pwscript: a regular expression literal is never closed")
}

// findSetup returns the index of the token opening setup's parameter list.
//
// Only a top-level declaration counts, because a setup nested inside something
// else is not what the runtime imports. The depth the scanner recorded is what
// says so, with no scope tracking of its own.
func findSetup(tokens []token) (int, bool) {
	for i, current := range tokens {
		if current.text != "setup" || current.depth != 0 {
			continue
		}
		if i == 0 || tokens[i-1].text != "function" {
			continue
		}
		if i+1 >= len(tokens) || tokens[i+1].text != "(" {
			continue
		}
		return i + 1, true
	}
	return 0, false
}

// readParameters reads the component parameters the setup pattern asked for.
//
// They are nested under one key rather than destructured flat, so a component
// taking a parameter named like a capability cannot collide with one. A block
// naming no such key asks for nothing, which is the ordinary case and not a
// failure.
func readParameters(tokens []token, open int) ([]string, string) {
	depth := tokens[open].depth
	for i := open + 1; i < len(tokens); i++ {
		if tokens[i].text == ")" && tokens[i].depth == depth {
			return nil, ""
		}
		if tokens[i].text != PropertyBagKey || tokens[i].depth != depth+1 {
			continue
		}
		// props on its own is the whole bag under another name, which this
		// scanner cannot turn into a list of parameters.
		if i+1 >= len(tokens) || tokens[i+1].text != ":" {
			return nil, "setup takes the parameter bag as a whole rather than destructuring it"
		}
		if i+2 >= len(tokens) || tokens[i+2].text != "{" {
			return nil, "setup renames the parameter bag rather than destructuring it"
		}
		return readNames(tokens, i+2)
	}
	return nil, ""
}

// PropertyBagKey is the key a setup destructures its component's parameters
// from. It is one key rather than flat names so that a parameter cannot collide
// with a capability the bag also carries, and so that a capability added later
// breaks nothing.
const PropertyBagKey = "props"

// readHandlers reads the keys of the object setup returns.
//
// Only a return of an object literal at the function's own top level, which is
// the idiomatic shape and the one a reader can check. A returned variable is
// reported as unread, because following it means evaluating the block.
func readHandlers(tokens []token, open int) ([]string, string) {
	body, ok := findBody(tokens, open)
	if !ok {
		return nil, "setup has no body this scanner can find"
	}
	depth := tokens[body].depth
	for i := body + 1; i < len(tokens); i++ {
		if tokens[i].text == "}" && tokens[i].depth == depth {
			return nil, "setup returns nothing, so it publishes no handler"
		}
		// A return at the function's own top level. One nested in a branch is a
		// shape this scanner declines rather than half-reads.
		if tokens[i].text != "return" || tokens[i].depth != depth {
			continue
		}
		if i+1 >= len(tokens) || tokens[i+1].text != "{" {
			return nil, "setup returns something other than an object literal"
		}
		return readNames(tokens, i+1)
	}
	return nil, "setup returns nothing, so it publishes no handler"
}

// findBody returns the index of the brace opening setup's body.
func findBody(tokens []token, open int) (int, bool) {
	depth := tokens[open].depth
	for i := open + 1; i < len(tokens); i++ {
		if tokens[i].text == ")" && tokens[i].depth == depth {
			if i+1 < len(tokens) && tokens[i+1].text == "{" {
				return i + 1, true
			}
			return 0, false
		}
	}
	return 0, false
}

// readNames reads the identifier keys of one brace-delimited group.
//
// A shorthand name and a name followed by a colon both count, which is what
// covers both the returned object and the destructured pattern. Anything else
// at that level — a spread, a computed key, a nested pattern — makes the whole
// group unread, because a partial list would publish some names and silently
// drop others.
func readNames(tokens []token, open int) ([]string, string) {
	depth := tokens[open].depth
	var names []string
	expect := true
	for i := open + 1; i < len(tokens); i++ {
		current := tokens[i]
		if current.text == "}" && current.depth == depth {
			return names, ""
		}
		if current.depth != depth {
			continue
		}
		switch {
		case current.text == ",":
			expect = true
		case expect && isName(current.text):
			names = append(names, current.text)
			expect = false
			// A value follows a colon, and it is not a name this group publishes.
			if i+1 < len(tokens) && tokens[i+1].text == ":" {
				for i+1 < len(tokens) && tokens[i+1].text != "," && !(tokens[i+1].text == "}" && tokens[i+1].depth == depth) {
					i++
				}
			}
		case expect:
			return nil, fmt.Sprintf("a %q where a name was expected", current.text)
		}
	}
	return nil, "a group is never closed"
}

// isName reports whether a token is an identifier rather than a number or a
// punctuator.
func isName(text string) bool {
	if text == "" || !isWordByte(text[0]) {
		return false
	}
	return text[0] < '0' || text[0] > '9'
}
