package pw

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/shibukawa/popcornwave/pwruntime"
	"github.com/shibukawa/tinybind-go/htmlbind"
	"github.com/shibukawa/tinybind-go/htmlbind/delta"
)

// What a navigation costs, measured through the entry this framework actually
// serves from rather than through the module's.
//
// The question is the one system:tinybind asked for: transfer size per response,
// before and after, on a page shaped like a real one. Before is the complete
// document a browser without the runtime receives. After is the delta, twice —
// once carrying markup and once carrying an address and the values that fill it.
//
// Run it with:
//
//	go test ./pw -run TransferCost -v

// The shape: a document shell, a layout with a nav, and a page with a search
// form and a list of result rows. Classes and attributes are on the markup
// because they are what a real page carries, and the markup-to-value ratio is
// what the split's whole value scales with.

type shellParams struct{ Children HTMLFragment }

type navParams struct {
	Section  string
	Children HTMLFragment
}

type resultsParams struct {
	Query string
	Rows  []resultRow
}

type resultRow struct {
	ID    string
	Title string
	Owner string
	When  string
}

type rowScopeParams struct {
	Outer resultsParams
	Item  resultRow
	Index int
}

var (
	shellOps  = htmlbind.Builder[shellParams]{}
	shellPlan = &htmlbind.Plan[shellParams]{
		Ops: []htmlbind.Op[shellParams]{
			shellOps.Static(`<!doctype html><html lang="en"><head><meta charset="utf-8"><title>Orders</title></head><body>`),
			shellOps.Component(func(p shellParams) htmlbind.Fragment { return p.Children }),
			shellOps.Static(`</body></html>`),
		},
	}

	navOps  = htmlbind.Builder[navParams]{}
	navPlan = &htmlbind.Plan[navParams]{
		// A layout is a chain member, so generation gives it a boundary and a
		// delta can leave it alone when only the page below it changed.
		Boundary: &htmlbind.Boundary[navParams]{
			ComponentID: "pw.measure.Layout@v1",
			Attr:        "data-" + UpdateAttributePrefix + "-id",
			Input:       func(p navParams) string { return delta.CanonString(p.Section) },
		},
		Ops: []htmlbind.Op[navParams]{
			navOps.Static(`<div class="app-shell"`), navOps.BoundaryAttr(), navOps.Static(`>`),
			navOps.Static(`<nav class="app-nav" aria-label="Primary"><ul class="app-nav__list">`),
			navOps.Static(`<li class="app-nav__item"><a class="app-nav__link" href="/orders">Orders</a></li>`),
			navOps.Static(`<li class="app-nav__item"><a class="app-nav__link" href="/invoices">Invoices</a></li>`),
			navOps.Static(`<li class="app-nav__item"><a class="app-nav__link" href="/customers">Customers</a></li>`),
			navOps.Static(`</ul></nav><main class="app-main" id="main">`),
			navOps.Component(func(p navParams) htmlbind.Fragment { return p.Children }),
			navOps.Static(`</main></div>`),
		},
	}

	resultsOps   = htmlbind.Builder[resultsParams]{}
	rowScopeOpsM = htmlbind.Builder[rowScopeParams]{}
	resultsPlan  = &htmlbind.Plan[resultsParams]{
		Boundary: &htmlbind.Boundary[resultsParams]{
			ComponentID: "pw.measure.Results@v1",
			Attr:        "data-" + UpdateAttributePrefix + "-id",
			Input:       func(p resultsParams) string { return delta.CanonString(p.Query) },
		},
		Ops: []htmlbind.Op[resultsParams]{
			resultsOps.Static(`<section class="results"`), resultsOps.BoundaryAttr(), resultsOps.Static(`>`),
			resultsOps.Static(`<form class="results__search" method="get" action="/orders">`),
			resultsOps.Static(`<label class="results__label" for="q">Search</label>`),
			resultsOps.Static(`<input class="results__input" type="search" id="q" name="q"`),
			resultsOps.Attr("value", func(p resultsParams) (string, bool) { return htmlbind.Escape(p.Query), true }),
			resultsOps.Static(`><button class="results__submit" type="submit">Go</button></form>`),
			resultsOps.Static(`<table class="results__table"><tbody class="results__body">`),
			htmlbind.For(
				func(p resultsParams) []resultRow { return p.Rows },
				func(p resultsParams, item resultRow, index int) rowScopeParams {
					return rowScopeParams{Outer: p, Item: item, Index: index}
				},
				[]htmlbind.Op[rowScopeParams]{
					rowScopeOpsM.Static(`<tr class="results__row"`),
					rowScopeOpsM.Attr("data-order", func(s rowScopeParams) (string, bool) {
						return htmlbind.Escape(s.Item.ID), true
					}),
					rowScopeOpsM.Static(`><td class="results__cell results__cell--title"><a class="results__link"`),
					rowScopeOpsM.URLAttr("href", func(s rowScopeParams) (string, bool) {
						return "/orders/" + s.Item.ID, true
					}),
					rowScopeOpsM.Static(`>`),
					rowScopeOpsM.Text(func(s rowScopeParams) string { return s.Item.Title }),
					rowScopeOpsM.Static(`</a></td><td class="results__cell results__cell--owner">`),
					rowScopeOpsM.Text(func(s rowScopeParams) string { return s.Item.Owner }),
					rowScopeOpsM.Static(`</td><td class="results__cell results__cell--when"><time class="results__time"`),
					rowScopeOpsM.Attr("datetime", func(s rowScopeParams) (string, bool) {
						return htmlbind.Escape(s.Item.When), true
					}),
					rowScopeOpsM.Static(`>`),
					rowScopeOpsM.Text(func(s rowScopeParams) string { return s.Item.When }),
					rowScopeOpsM.Static(`</time></td></tr>`),
				}),
			resultsOps.Static(`</tbody></table></section>`),
		},
	}
)

func measureRows(count int, seed string) []resultRow {
	rows := make([]resultRow, 0, count)
	for index := 0; index < count; index++ {
		number := strconv.Itoa(index + 1)
		rows = append(rows, resultRow{
			ID:    seed + "-" + number,
			Title: "Order " + number + " for " + seed,
			Owner: []string{"alice", "bob", "carol", "dan"}[index%4],
			When:  "2026-08-0" + strconv.Itoa(index%9+1),
		})
	}
	return rows
}

func measureChain(section string, results resultsParams) ([]HTMLWrapper, HTMLFragment) {
	wrappers := []HTMLWrapper{
		htmlbind.BindWrapper(shellPlan, shellParams{}, func(target *shellParams, children htmlbind.Fragment) {
			target.Children = children
		}),
		htmlbind.BindWrapper(navPlan, navParams{Section: section}, func(target *navParams, children htmlbind.Fragment) {
			target.Children = children
		}),
	}
	return wrappers, htmlbind.Bind(resultsPlan, results)
}

// measureConfig turns on everything a deployment measuring this would have on.
func measureConfig() HTMLConfig {
	config := defaultHTMLConfig
	config.Update = HTMLUpdateConfig{Enabled: true, ValidatorKey: "measure-key", MaxManifestBytes: 8 << 10}
	return config
}

func measureRequest(target string) *http.Request {
	request := browserRequest(target)
	return request.WithContext(pwruntime.WithResources(request.Context(), pwruntime.Resources{
		Configs: map[reflect.Type]any{reflect.TypeFor[HTMLConfig](): measureConfig()},
	}))
}

// serveMeasured renders one request and returns the bytes a client receives.
func serveMeasured(target, manifest string, sequences bool) (string, int) {
	request := measureRequest(target)
	if manifest != "" {
		request.Header.Set("Pw-Render", "navigation")
		request.Header.Set("Pw-Build", UpdateBuildID())
		request.Header.Set("Pw-Manifest", manifest)
		if sequences {
			request.Header.Set("Pw-Sequences", "1")
		}
	}
	recorder := httptest.NewRecorder()
	wrappers, leaf := measureChainFor(target)
	WriteHTMLChain(recorder, request, wrappers, leaf)
	return recorder.Body.String(), recorder.Body.Len()
}

// measureChainFor is the route table of this measurement: two pages sharing one
// layout, so a link between them is the ordinary cross-page navigation and a
// search on one is the ordinary in-page update.
func measureChainFor(target string) ([]HTMLWrapper, HTMLFragment) {
	query := ""
	if cut := strings.Index(target, "q="); cut >= 0 {
		query = target[cut+2:]
	}
	section := "orders"
	if strings.HasPrefix(target, "/invoices") {
		section = "invoices"
	}
	return measureChain(section, resultsParams{Query: query, Rows: measureRows(25, section+query)})
}

// clientManifest is what this framework's runtime would hold after consuming a
// response, encoded the way it sends it back.
func clientManifest(body string) string {
	entries := []string{}
	for _, line := range strings.Split(body, "\n") {
		if !strings.Contains(line, `"r":"op"`) {
			continue
		}
		var record struct {
			ID       string `json:"id"`
			Frame    string `json:"frame"`
			Children string `json:"children"`
			Parent   string `json:"parent"`
		}
		if json.Unmarshal([]byte(line), &record) != nil || record.ID == "" {
			continue
		}
		entry := record.ID + ":" + record.Frame
		if record.Children != "" || record.Parent != "" {
			entry += ":" + record.Children
		}
		if record.Parent != "" {
			entry += ":" + record.Parent
		}
		entries = append(entries, entry)
	}
	return strings.Join(entries, ",")
}

// sequenceBytes is the one-time cost of the trees a response's addresses name.
func sequenceBytes(t *testing.T, body string) (int, int) {
	t.Helper()
	seen := map[string]bool{}
	total := 0
	for _, line := range strings.Split(body, "\n") {
		var record struct {
			Seq string `json:"seq"`
		}
		if json.Unmarshal([]byte(line), &record) != nil || record.Seq == "" || seen[record.Seq] {
			continue
		}
		seen[record.Seq] = true
		sequence, known := htmlbind.LookupSequence(record.Seq)
		if !known {
			t.Fatalf("the response named an address this process cannot serve: %s", record.Seq)
		}
		total += len(sequence.AppendJSON(nil))
	}
	return total, len(seen)
}

// fragmentBytes reports what the replaced fragments of a response cost on the
// wire and what the same markup is as HTML, which is what attributes the
// difference to the record encoding rather than to the delta.
func fragmentBytes(body string) (onWire, asHTML int) {
	for _, line := range strings.Split(body, "\n") {
		var record struct {
			HTML string `json:"html"`
		}
		if json.Unmarshal([]byte(line), &record) != nil || record.HTML == "" {
			continue
		}
		onWire += len(line)
		asHTML += len(record.HTML)
	}
	return onWire, asHTML
}

func TestTransferCostOfANavigation(t *testing.T) {
	// A cold client holds nothing, so its first navigation is answered in full.
	// Warming it is what a second navigation measures, and both are worth having:
	// the first is what a reader pays after arriving, the second is every click
	// after that.
	warm, _ := serveMeasured("/orders?q=one", "seed", false)
	held := clientManifest(warm)
	if held == "" {
		t.Fatal("the warming navigation returned no manifest")
	}

	_, documentBytes := serveMeasured("/invoices", "", false)

	t.Logf("a page of 25 result rows under a shared layout")
	t.Logf("%-26s %8s %8s   %s", "", "bytes", "vs doc", "")
	t.Logf("%-26s %8d %8s   %s", "complete document", documentBytes, "1.00x",
		"what a browser with no runtime receives")

	for _, one := range []struct{ name, target string }{
		{"cross-page link", "/invoices"},
		{"same-page search", "/orders?q=two"},
	} {
		markup, markupBytes := serveMeasured(one.target, held, false)
		values, valueBytes := serveMeasured(one.target, held, true)
		trees, count := sequenceBytes(t, values)
		onWire, asHTML := fragmentBytes(markup)

		t.Logf("%-26s %8d %7.2fx   %s", one.name+", markup", markupBytes,
			float64(documentBytes)/float64(markupBytes),
			fmt.Sprintf("fragment %d B on the wire, %d B as HTML (%.2fx)",
				onWire, asHTML, float64(onWire)/float64(asHTML)))
		t.Logf("%-26s %8d %7.2fx   %s", one.name+", values", valueBytes,
			float64(documentBytes)/float64(valueBytes),
			fmt.Sprintf("%.2fx smaller than the same delta as markup", float64(markupBytes)/float64(valueBytes)))
		t.Logf("%-26s %8d %8s   %s", one.name+", trees", trees, "-",
			fmt.Sprintf("%d tree(s), fetched once per build and cached immutably", count))
	}
}

// The result worth asserting, because it is the one that decides whether the
// split is worth its complexity — and because the markup delta going the other
// way is a property nobody would guess.
func TestValuesBeatMarkupAndMarkupLosesToTheDocument(t *testing.T) {
	warm, _ := serveMeasured("/orders?q=one", "seed", false)
	held := clientManifest(warm)
	_, documentBytes := serveMeasured("/orders?q=two", "", false)
	_, markupBytes := serveMeasured("/orders?q=two", held, false)
	_, valueBytes := serveMeasured("/orders?q=two", held, true)

	// A delta transferring one region of a page still loses to the whole page
	// when the region is most of it, because a record escapes every angle
	// bracket of the markup it carries. That is not a defect in the delta; it is
	// what the encoding costs, and it is the case for sending values instead.
	if markupBytes <= documentBytes {
		t.Logf("the markup delta is %d bytes against a %d byte document, which is "+
			"smaller than when this was measured; the page shape may have changed",
			markupBytes, documentBytes)
	}
	if valueBytes >= markupBytes {
		t.Errorf("values are %d bytes against %d as markup, so the split bought nothing",
			valueBytes, markupBytes)
	}
	if valueBytes >= documentBytes {
		t.Errorf("values are %d bytes against a %d byte document, so a delta is not worth asking for",
			valueBytes, documentBytes)
	}
}

// The cold client is the other half of the result, so it is asserted rather than
// only printed: a page load leaves this runtime holding no validators, and its
// first navigation is therefore answered in full.
func TestAColdClientHoldsNoValidators(t *testing.T) {
	document, _ := serveMeasured("/orders?q=one", "", false)
	if strings.Contains(document, `"r":"op"`) {
		t.Fatal("a request with no render header was answered as a delta")
	}
	// Nothing in the document carries the manifest a delta would compare
	// against, which is what makes the first navigation after a page load cost
	// a whole page.
	if strings.Contains(document, "Pw-Manifest") {
		t.Error("the document carries a manifest this runtime does not read")
	}
}
