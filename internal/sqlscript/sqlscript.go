// Package sqlscript executes a multi-statement SQL script one statement at a
// time.
//
// Executing a whole script in a single Exec is a convenience of some SQLite
// drivers only. The cgosqlite backend selected for TinyGo builds runs the first
// statement and silently ignores the rest, so every caller must split first.
package sqlscript

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Executor is the subset of *sql.DB and *sql.Tx used to run a script.
type Executor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// Exec runs every statement in script in order.
func Exec(ctx context.Context, executor Executor, script string) error {
	for index, statement := range Split(script) {
		if _, err := executor.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("statement %d (%s): %w", index+1, summarize(statement), err)
		}
	}
	return nil
}

func summarize(statement string) string {
	const limit = 60
	collapsed := strings.Join(strings.Fields(statement), " ")
	if len(collapsed) <= limit {
		return collapsed
	}
	return collapsed[:limit] + "..."
}

// Trigger-prefix states: a statement opens a trigger when its bare words read
// CREATE (TEMP|TEMPORARY|OR|REPLACE|IF|NOT|EXISTS)* TRIGGER, judged one word
// at a time as each word completes — a migration snapshot carries one INSERT
// per seeded row, so the split runs over megabytes and must stay linear.
const (
	triggerStatementStart = iota
	triggerAfterCreate
	triggerIneligible
)

// Split divides a SQL script into individual statements. Semicolons inside
// string literals, quoted identifiers, comments, and CREATE TRIGGER bodies do
// not terminate a statement.
func Split(script string) []string {
	var statements []string
	var current strings.Builder
	// lastWord tracks the most recent bare word so a CREATE TRIGGER body can be
	// terminated at END, matching how the sqlite shell decides completeness.
	var lastWord strings.Builder
	previousWord := ""
	isTrigger := false
	triggerState := triggerStatementStart

	flushWord := func() {
		if lastWord.Len() == 0 {
			return
		}
		previousWord = strings.ToUpper(lastWord.String())
		lastWord.Reset()
		if isTrigger {
			return
		}
		switch triggerState {
		case triggerStatementStart:
			if previousWord == "CREATE" {
				triggerState = triggerAfterCreate
			} else {
				triggerState = triggerIneligible
			}
		case triggerAfterCreate:
			switch previousWord {
			case "TEMP", "TEMPORARY", "OR", "REPLACE", "IF", "NOT", "EXISTS":
			case "TRIGGER":
				isTrigger = true
			default:
				triggerState = triggerIneligible
			}
		}
	}
	endStatement := func() {
		statement := strings.TrimSpace(current.String())
		if statement != "" {
			statements = append(statements, statement)
		}
		current.Reset()
		previousWord = ""
		isTrigger = false
		triggerState = triggerStatementStart
	}

	// Every byte the splitter branches on is ASCII, and UTF-8 continuation
	// bytes cannot collide with any of them, so the walk is byte-indexed and
	// only a non-ASCII lead byte pays for a rune decode.
	for index := 0; index < len(script); index++ {
		char := script[index]
		switch {
		case char == '-' && index+1 < len(script) && script[index+1] == '-':
			flushWord()
			for index < len(script) && script[index] != '\n' {
				current.WriteByte(script[index])
				index++
			}
			if index < len(script) {
				current.WriteByte('\n')
			}
		case char == '/' && index+1 < len(script) && script[index+1] == '*':
			flushWord()
			current.WriteString("/*")
			index += 2
			for index < len(script) {
				if script[index] == '*' && index+1 < len(script) && script[index+1] == '/' {
					current.WriteString("*/")
					index++
					break
				}
				current.WriteByte(script[index])
				index++
			}
		case char == '\'' || char == '"' || char == '`':
			flushWord()
			quote := char
			current.WriteByte(char)
			index++
			for index < len(script) {
				if script[index] == quote {
					// A doubled quote is an escaped quote, not a terminator.
					if index+1 < len(script) && script[index+1] == quote {
						current.WriteByte(quote)
						current.WriteByte(quote)
						index += 2
						continue
					}
					current.WriteByte(quote)
					break
				}
				current.WriteByte(script[index])
				index++
			}
		case char == '[':
			flushWord()
			current.WriteByte(char)
			index++
			for index < len(script) {
				current.WriteByte(script[index])
				if script[index] == ']' {
					break
				}
				index++
			}
		case char == ';':
			flushWord()
			current.WriteByte(char)
			if !isTrigger || previousWord == "END" {
				endStatement()
			}
		case char >= utf8.RuneSelf:
			decoded, size := utf8.DecodeRuneInString(script[index:])
			if isWordRune(decoded) {
				lastWord.WriteRune(decoded)
			} else {
				flushWord()
			}
			current.WriteString(script[index : index+size])
			index += size - 1
		default:
			if isWordByte(char) {
				lastWord.WriteByte(char)
			} else {
				flushWord()
			}
			current.WriteByte(char)
		}
	}
	flushWord()
	endStatement()
	return statements
}

func isWordByte(char byte) bool {
	return char == '_' ||
		('a' <= char && char <= 'z') ||
		('A' <= char && char <= 'Z') ||
		('0' <= char && char <= '9')
}

func isWordRune(char rune) bool {
	return char == '_' || unicode.IsLetter(char) || unicode.IsDigit(char)
}
