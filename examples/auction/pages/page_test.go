package pages

import "testing"

func TestFormatAmount(t *testing.T) {
	tests := map[int]string{
		0:       "¥0",
		999:     "¥999",
		1000:    "¥1,000",
		1250000: "¥1,250,000",
	}
	for amount, want := range tests {
		if got := FormatAmount(amount); got != want {
			t.Errorf("FormatAmount(%d) = %q, want %q", amount, got, want)
		}
	}
}
