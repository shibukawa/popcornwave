package invoices

// Load takes nothing: this route has no dynamic segment and no query input, so
// its page's parameters are its only result.
func Load() ([]Invoice, error) {
	return []Invoice{
		{Number: "INV-2026-0041", Customer: "Kaffeehaus GmbH", Total: "€1,204.00"},
		{Number: "INV-2026-0042", Customer: "Blue Bottle Ltd", Total: "€880.50"},
		{Number: "INV-2026-0043", Customer: "Roasters Co-op", Total: "€2,310.75"},
		{Number: "INV-2026-0044", Customer: "Cafe Nordlys", Total: "€455.20"},
	}, nil
}
