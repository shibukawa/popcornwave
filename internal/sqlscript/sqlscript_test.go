package sqlscript

import (
	"reflect"
	"strings"
	"testing"
)

func TestSplitSeparatesPlainStatements(t *testing.T) {
	got := Split("CREATE TABLE a (id INTEGER);\nINSERT INTO a VALUES (1);\n")
	want := []string{"CREATE TABLE a (id INTEGER);", "INSERT INTO a VALUES (1);"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func TestSplitKeepsSemicolonsInsideLiterals(t *testing.T) {
	got := Split(`INSERT INTO a VALUES ('one; two', "col;name", X'3B');
SELECT 1;`)
	if len(got) != 2 {
		t.Fatalf("got %d statements: %#v", len(got), got)
	}
	if !strings.Contains(got[0], "one; two") || !strings.Contains(got[0], "col;name") {
		t.Fatalf("literal was split: %#v", got[0])
	}
}

func TestSplitHandlesEscapedQuotes(t *testing.T) {
	got := Split(`INSERT INTO a VALUES ('it''s; fine');
SELECT 2;`)
	if len(got) != 2 {
		t.Fatalf("got %d statements: %#v", len(got), got)
	}
	if !strings.Contains(got[0], "it''s; fine") {
		t.Fatalf("escaped quote was mishandled: %#v", got[0])
	}
}

func TestSplitKeepsTriggerBodyIntact(t *testing.T) {
	script := `CREATE TABLE users (id INTEGER, score INTEGER);
CREATE TRIGGER bump AFTER INSERT ON posts BEGIN UPDATE users SET score = score + 1 WHERE id = NEW.user_id; END;
SELECT 3;`
	got := Split(script)
	if len(got) != 3 {
		t.Fatalf("got %d statements: %#v", len(got), got)
	}
	if !strings.HasPrefix(got[1], "CREATE TRIGGER") || !strings.HasSuffix(got[1], "END;") {
		t.Fatalf("trigger statement was split: %#v", got[1])
	}
}

func TestSplitKeepsTemporaryTriggerIntact(t *testing.T) {
	script := `CREATE TEMP TRIGGER t AFTER INSERT ON a BEGIN SELECT 1; SELECT 2; END;
SELECT 4;`
	got := Split(script)
	if len(got) != 2 {
		t.Fatalf("got %d statements: %#v", len(got), got)
	}
}

func TestSplitIgnoresCommentedSemicolons(t *testing.T) {
	script := `-- a comment; not a statement
CREATE TABLE a (id INTEGER); /* another; comment */
SELECT 5;`
	got := Split(script)
	if len(got) != 2 {
		t.Fatalf("got %d statements: %#v", len(got), got)
	}
}

func TestSplitIgnoresEmptyInput(t *testing.T) {
	if got := Split("   \n\n"); len(got) != 0 {
		t.Fatalf("got %#v, want none", got)
	}
}

func TestSplitKeepsTrailingStatementWithoutSemicolon(t *testing.T) {
	got := Split("SELECT 1;\nSELECT 2")
	if len(got) != 2 || got[1] != "SELECT 2" {
		t.Fatalf("got %#v", got)
	}
}
