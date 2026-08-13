package handlers

import "testing"

func TestParseAmountCents(t *testing.T) {
	tests := map[string]int{
		"0":       0,
		"10":      1000,
		"10.01":   1001,
		"10.1":    1010,
		"10.10":   1010,
		"0.99":    99,
		"1234.56": 123456,
	}
	for value, want := range tests {
		got, err := parseAmountCents(value)
		if err != nil {
			t.Errorf("parseAmountCents(%q) returned %v", value, err)
			continue
		}
		if got != want {
			t.Errorf("parseAmountCents(%q) = %d, want %d", value, got, want)
		}
	}
}

func TestParseAmountCentsRejectsOversizedFigure(t *testing.T) {
	if _, err := parseAmountCents("99999999999999999999"); err == nil {
		t.Error("parseAmountCents accepted a figure wider than an int")
	}
}
