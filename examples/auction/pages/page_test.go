package pages

import "testing"

func TestFormatDollars(t *testing.T) {
	tests := map[int]string{
		0:         "0",
		99:        "0",
		999:       "9",
		100000:    "1,000",
		125000000: "1,250,000",
	}
	for cents, want := range tests {
		if got := FormatDollars(cents); got != want {
			t.Errorf("FormatDollars(%d) = %q, want %q", cents, got, want)
		}
	}
}

func TestFormatCents(t *testing.T) {
	tests := map[int]string{
		0:      "00",
		5:      "05",
		99:     "99",
		100000: "00",
		120501: "01",
	}
	for cents, want := range tests {
		if got := FormatCents(cents); got != want {
			t.Errorf("FormatCents(%d) = %q, want %q", cents, got, want)
		}
	}
}
