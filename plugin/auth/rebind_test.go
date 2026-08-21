package auth

import "testing"

func TestRebindNumbersPostgresPlaceholders(t *testing.T) {
	statement := `UPDATE t SET a = ?, b = ? WHERE c = ? AND d < ?`
	if got, want := rebind("postgres", statement),
		`UPDATE t SET a = $1, b = $2 WHERE c = $3 AND d < $4`; got != want {
		t.Errorf("rebind(postgres) = %q, want %q", got, want)
	}
	for _, dialect := range []string{"sqlite", "mysql", ""} {
		if got := rebind(dialect, statement); got != statement {
			t.Errorf("rebind(%q) = %q, want unchanged", dialect, got)
		}
	}
}
