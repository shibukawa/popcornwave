package pages

import (
	"sort"
	"strings"
)

// Query resolves the route's one optional input.
//
// q is declared optional so an absent query arrives as nil rather than as the
// empty string. Nothing here needs to tell the two apart, but declaring it
// honestly is what keeps the default here rather than in the decoder.
func Query(q *string) string {
	if q == nil {
		return ""
	}
	return *q
}

// The data is a fixed table so the example needs no database and every response
// is reproducible: the same query always produces the same bytes, which is what
// makes the network tab worth reading.
var allOrders = []Order{
	{Id: "o-1041", Title: "Espresso machine", Owner: "alice", At: "2026-08-01"},
	{Id: "o-1042", Title: "Grinder, conical burr", Owner: "bob", At: "2026-08-02"},
	{Id: "o-1043", Title: "Milk frother", Owner: "carol", At: "2026-08-02"},
	{Id: "o-1044", Title: "Filter papers, box of 200", Owner: "dan", At: "2026-08-03"},
	{Id: "o-1045", Title: "Kettle, gooseneck", Owner: "alice", At: "2026-08-04"},
	{Id: "o-1046", Title: "Scale, 0.1g", Owner: "bob", At: "2026-08-05"},
	{Id: "o-1047", Title: "Tamper, 58mm", Owner: "carol", At: "2026-08-05"},
	{Id: "o-1048", Title: "Knock box", Owner: "dan", At: "2026-08-06"},
	{Id: "o-1049", Title: "Cupping bowls, set of 6", Owner: "alice", At: "2026-08-07"},
	{Id: "o-1050", Title: "Storage tin, 1kg", Owner: "bob", At: "2026-08-08"},
}

// SearchOrders filters and orders. A query matching nothing returns an empty
// table rather than an error, because an empty result is an answer.
func SearchOrders(query string) []Order {
	needle := strings.ToLower(strings.TrimSpace(query))
	matched := make([]Order, 0, len(allOrders))
	for _, order := range allOrders {
		if needle == "" ||
			strings.Contains(strings.ToLower(order.Title), needle) ||
			strings.Contains(strings.ToLower(order.Owner), needle) {
			matched = append(matched, order)
		}
	}
	// Sorting by owner rather than by id is deliberate: a query that changes the
	// order without changing the set is what produces a children operation, and
	// there is no other way to see one.
	sort.SliceStable(matched, func(i, j int) bool { return matched[i].Owner < matched[j].Owner })
	return matched
}
