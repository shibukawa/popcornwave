package pwruntime

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// reproName is the prepared statement name used by a rerun snippet. It is
// deallocated by the snippet itself so pasting it twice still works.
const reproName = "pw_repro"

// reproductionSnippet renders a paste-able rerun of the observed statement.
//
// The snippet binds the arguments and then runs the statement text unchanged.
// Values are never inlined into the SQL: constant folding and literal-driven
// index selection can give a literal rewrite a different plan than the
// parameterized execution being diagnosed.
//
// An empty result means the statement cannot be reproduced honestly, which is
// the case for an unknown dialect, an unrecognized placeholder style, or a
// value this package cannot quote for the target CLI.
func reproductionSnippet(driver, query string, args []any) string {
	if len(args) == 0 {
		// The logged statement text already is the complete rerun.
		return ""
	}
	dialect, ok := reproDialects[driver]
	if !ok {
		return ""
	}
	dollars, questions := scanPlaceholders(query)
	switch dialect {
	case reproSQLite:
		if !coversDollars(dollars, len(args)) {
			return ""
		}
		return sqliteSnippet(query, args, dialect)
	case reproPostgres:
		if !coversDollars(dollars, len(args)) {
			return ""
		}
		return postgresSnippet(query, args, dialect)
	case reproMySQL:
		if questions != len(args) {
			return ""
		}
		return mysqlSnippet(query, args, dialect)
	}
	return ""
}

type reproDialect int

const (
	reproSQLite reproDialect = iota + 1
	reproPostgres
	reproMySQL
)

var reproDialects = map[string]reproDialect{
	"sqlite":     reproSQLite,
	"sqlite3":    reproSQLite,
	"postgres":   reproPostgres,
	"postgresql": reproPostgres,
	"pgx":        reproPostgres,
	"mysql":      reproMySQL,
}

// sqliteSnippet uses the sqlite3 shell parameter table, which binds by the
// parameter name as it appears in the statement. Dot commands must each own a
// line, so this snippet is multi-line by necessity.
func sqliteSnippet(query string, args []any, dialect reproDialect) string {
	var snippet strings.Builder
	for i, arg := range args {
		literal, ok := sqlLiteral(arg, dialect)
		if !ok {
			return ""
		}
		fmt.Fprintf(&snippet, ".parameter set $%d %s\n", i+1, literal)
	}
	snippet.WriteString(statementText(query))
	return snippet.String()
}

// postgresSnippet prepares the statement so the server plans it with the same
// parameter placeholders it used during the observed execution.
func postgresSnippet(query string, args []any, dialect reproDialect) string {
	values := make([]string, len(args))
	for i, arg := range args {
		literal, ok := sqlLiteral(arg, dialect)
		if !ok {
			return ""
		}
		values[i] = literal
	}
	return fmt.Sprintf("PREPARE %s AS %s EXECUTE %s(%s); DEALLOCATE %s;",
		reproName, statementText(query), reproName, strings.Join(values, ", "), reproName)
}

// mysqlSnippet routes values through user variables, which is how the MySQL
// client binds parameters to a prepared statement.
func mysqlSnippet(query string, args []any, dialect reproDialect) string {
	assignments := make([]string, len(args))
	names := make([]string, len(args))
	for i, arg := range args {
		literal, ok := sqlLiteral(arg, dialect)
		if !ok {
			return ""
		}
		names[i] = "@p" + strconv.Itoa(i+1)
		assignments[i] = fmt.Sprintf("SET %s = %s;", names[i], literal)
	}
	prepared, ok := sqlLiteral(strings.TrimSuffix(statementText(query), ";"), dialect)
	if !ok {
		return ""
	}
	return fmt.Sprintf("%s PREPARE %s FROM %s; EXECUTE %s USING %s; DEALLOCATE PREPARE %s;",
		strings.Join(assignments, " "), reproName, prepared, reproName, strings.Join(names, ", "), reproName)
}

// statementText normalizes the statement into one terminated line.
func statementText(query string) string {
	text := strings.Join(strings.Fields(query), " ")
	if !strings.HasSuffix(text, ";") {
		text += ";"
	}
	return text
}

// sqlLiteral quotes one value for the target CLI. It refuses anything it cannot
// render exactly, because a snippet that runs a different query is worse than
// no snippet at all.
func sqlLiteral(value any, dialect reproDialect) (string, bool) {
	switch typed := value.(type) {
	case nil:
		return "NULL", true
	case string:
		return quoteString(typed)
	case []byte:
		if dialect == reproPostgres {
			return "'\\x" + hex.EncodeToString(typed) + "'::bytea", true
		}
		return "X'" + hex.EncodeToString(typed) + "'", true
	case bool:
		if dialect == reproPostgres {
			return strings.ToUpper(strconv.FormatBool(typed)), true
		}
		if typed {
			return "1", true
		}
		return "0", true
	case time.Time:
		return "'" + typed.Format(time.RFC3339Nano) + "'", true
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return fmt.Sprint(typed), true
	case float32, float64:
		return fmt.Sprint(typed), true
	default:
		return "", false
	}
}

// quoteString renders a SQL string literal. A control character would break the
// single-line snippet, so such a value makes the whole snippet unavailable.
func quoteString(value string) (string, bool) {
	for _, r := range value {
		if unicode.IsControl(r) {
			return "", false
		}
	}
	return "'" + strings.ReplaceAll(value, "'", "''") + "'", true
}

// coversDollars reports whether the statement uses exactly $1..$count.
func coversDollars(found map[int]bool, count int) bool {
	if len(found) != count {
		return false
	}
	for i := 1; i <= count; i++ {
		if !found[i] {
			return false
		}
	}
	return true
}

// scanPlaceholders collects the placeholders of a statement while skipping
// string literals, quoted identifiers, and comments, so a '?' inside a literal
// is not mistaken for a bind parameter.
func scanPlaceholders(query string) (map[int]bool, int) {
	dollars := make(map[int]bool)
	questions := 0
	for i := 0; i < len(query); {
		switch c := query[i]; c {
		case '\'', '"', '`':
			i = skipQuoted(query, i, c)
		case '-':
			if i+1 < len(query) && query[i+1] == '-' {
				i = skipLine(query, i)
				continue
			}
			i++
		case '/':
			if i+1 < len(query) && query[i+1] == '*' {
				i = skipBlock(query, i)
				continue
			}
			i++
		case '?':
			questions++
			i++
		case '$':
			start := i + 1
			end := start
			for end < len(query) && query[end] >= '0' && query[end] <= '9' {
				end++
			}
			if end > start {
				if number, err := strconv.Atoi(query[start:end]); err == nil {
					dollars[number] = true
				}
				i = end
				continue
			}
			i++
		default:
			i++
		}
	}
	return dollars, questions
}

// skipQuoted returns the index after the closing quote, treating a doubled
// quote as an escaped one.
func skipQuoted(query string, start int, quote byte) int {
	for i := start + 1; i < len(query); i++ {
		if query[i] != quote {
			continue
		}
		if i+1 < len(query) && query[i+1] == quote {
			i++
			continue
		}
		return i + 1
	}
	return len(query)
}

func skipLine(query string, start int) int {
	if index := strings.IndexByte(query[start:], '\n'); index >= 0 {
		return start + index + 1
	}
	return len(query)
}

func skipBlock(query string, start int) int {
	if index := strings.Index(query[start+2:], "*/"); index >= 0 {
		return start + 2 + index + 2
	}
	return len(query)
}
