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

// Split divides a SQL script into individual statements. Semicolons inside
// string literals, quoted identifiers, comments, and CREATE TRIGGER bodies do
// not terminate a statement.
func Split(script string) []string {
	var statements []string
	var current strings.Builder
	runes := []rune(script)
	// lastWord tracks the most recent bare word so a CREATE TRIGGER body can be
	// terminated at END, matching how the sqlite shell decides completeness.
	var lastWord strings.Builder
	previousWord := ""
	isTrigger := false

	flushWord := func() {
		if lastWord.Len() > 0 {
			previousWord = strings.ToUpper(lastWord.String())
			lastWord.Reset()
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
	}

	for index := 0; index < len(runes); index++ {
		char := runes[index]
		switch {
		case char == '-' && index+1 < len(runes) && runes[index+1] == '-':
			flushWord()
			for index < len(runes) && runes[index] != '\n' {
				current.WriteRune(runes[index])
				index++
			}
			if index < len(runes) {
				current.WriteRune('\n')
			}
		case char == '/' && index+1 < len(runes) && runes[index+1] == '*':
			flushWord()
			current.WriteString("/*")
			index += 2
			for index < len(runes) {
				if runes[index] == '*' && index+1 < len(runes) && runes[index+1] == '/' {
					current.WriteString("*/")
					index++
					break
				}
				current.WriteRune(runes[index])
				index++
			}
		case char == '\'' || char == '"' || char == '`':
			flushWord()
			quote := char
			current.WriteRune(char)
			index++
			for index < len(runes) {
				if runes[index] == quote {
					// A doubled quote is an escaped quote, not a terminator.
					if index+1 < len(runes) && runes[index+1] == quote {
						current.WriteRune(quote)
						current.WriteRune(quote)
						index += 2
						continue
					}
					current.WriteRune(quote)
					break
				}
				current.WriteRune(runes[index])
				index++
			}
		case char == '[':
			flushWord()
			current.WriteRune(char)
			index++
			for index < len(runes) {
				current.WriteRune(runes[index])
				if runes[index] == ']' {
					break
				}
				index++
			}
		case char == ';':
			flushWord()
			current.WriteRune(char)
			if !isTrigger || previousWord == "END" {
				endStatement()
			}
		default:
			if isWordRune(char) {
				lastWord.WriteRune(char)
			} else {
				flushWord()
			}
			current.WriteRune(char)
			if !isTrigger && startsTrigger(current.String()) {
				isTrigger = true
			}
		}
	}
	flushWord()
	endStatement()
	return statements
}

func isWordRune(char rune) bool {
	return char == '_' || unicode.IsLetter(char) || unicode.IsDigit(char)
}

// startsTrigger reports whether the accumulated statement opens a trigger, whose
// body contains semicolons that must not split it.
func startsTrigger(statement string) bool {
	fields := strings.Fields(strings.ToUpper(statement))
	if len(fields) < 2 || fields[0] != "CREATE" {
		return false
	}
	for _, field := range fields[1:] {
		switch field {
		case "TEMP", "TEMPORARY", "OR", "REPLACE", "IF", "NOT", "EXISTS":
			continue
		case "TRIGGER":
			return true
		default:
			return false
		}
	}
	return false
}
