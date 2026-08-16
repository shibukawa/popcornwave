package invoices

// LoadInvoices is the page's data. It is an external the template declares and
// binds with {val}, so the component names what it needs rather than matching a
// result list to a parameter list by position.
func LoadInvoices() []Invoice {
	return []Invoice{
		{Number: "INV-2026-0041", Customer: "Kaffeehaus GmbH", Total: "€1,204.00"},
		{Number: "INV-2026-0042", Customer: "Blue Bottle Ltd", Total: "€880.50"},
		{Number: "INV-2026-0043", Customer: "Roasters Co-op", Total: "€2,310.75"},
		{Number: "INV-2026-0044", Customer: "Cafe Nordlys", Total: "€455.20"},
	}
}
