package session

import (
	"testing"
	"time"
)

// A zero field means "no such bound", not "the epoch". Reading the absolute
// bound alone returned the zero time when only an idle bound was configured, and
// every caller reads a zero deadline as "no deadline" — so an operator who
// bounded a session purely by inactivity got a session with no bound at all.
func TestDeadlineIsTheEarliestBoundThatIsSet(t *testing.T) {
	base := time.Unix(1_700_000_000, 0).UTC()
	idle := base.Add(15 * time.Minute)
	absolute := base.Add(24 * time.Hour)

	for _, testCase := range []struct {
		name     string
		record   Record[int]
		want     time.Time
		wantZero bool
	}{
		{name: "idle only", record: Record[int]{IdleExpiresAt: idle}, want: idle},
		{name: "absolute only", record: Record[int]{ExpiresAt: absolute}, want: absolute},
		{name: "idle sooner", record: Record[int]{IdleExpiresAt: idle, ExpiresAt: absolute}, want: idle},
		{name: "absolute sooner", record: Record[int]{IdleExpiresAt: absolute, ExpiresAt: idle}, want: idle},
		{name: "neither", record: Record[int]{}, wantZero: true},
	} {
		got := testCase.record.deadline()
		if testCase.wantZero {
			if !got.IsZero() {
				t.Errorf("%s: deadline = %v, want the zero time", testCase.name, got)
			}
			continue
		}
		if !got.Equal(testCase.want) {
			t.Errorf("%s: deadline = %v, want %v", testCase.name, got, testCase.want)
		}
	}
}

// RawRecord answers the same question for a backend, so it has to answer it the
// same way.
func TestRawRecordDeadlineMatches(t *testing.T) {
	base := time.Unix(1_700_000_000, 0).UTC()
	idle := base.Add(15 * time.Minute)

	if got := (RawRecord{IdleExpiresAt: idle}).Deadline(); !got.Equal(idle) {
		t.Errorf("Deadline = %v, want the idle bound %v", got, idle)
	}
	if got := (RawRecord{}).Deadline(); !got.IsZero() {
		t.Errorf("Deadline = %v, want the zero time when nothing is set", got)
	}
}
