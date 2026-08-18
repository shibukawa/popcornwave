package bootblock

import (
	"strings"
	"testing"

	"github.com/shibukawa/popcornweb/internal/pwtree"
)

// baseRows is a configuration tree as it reaches a stream. The rows are written
// out rather than rendered, so what these tests read is what an application
// prints and not what this package would have printed for itself. pw's own round
// trip covers the other direction, where the real renderer has to stay readable.
var baseRows = []string{
	"├─ html",
	"│  ├─ streaming              true",
	"│  └─ bot_async_timeout      5s              ← file",
	"└─ server",
	"   └─ port                   8080            ← flag",
}

func blockWith(rows []string, listening string) []string {
	lines := []string{
		Art[0],
		Art[1] + "   Popcorn Web 0.1.0",
		Art[2] + "   started at 2026-08-12 09:00:00 JST",
		Art[3] + "   env dev · config.dev.toml",
		Art[4],
		"",
		"configuration",
	}
	lines = append(lines, rows...)
	if listening != "" {
		lines = append(lines, "", "listening on "+listening)
	}
	return lines
}

func sampleBlock() []string { return blockWith(baseRows, "http://localhost:8080") }

func scan(t *testing.T, lines []string) Report {
	t.Helper()
	scanner := Scanner{}
	for _, line := range lines {
		if taken, _ := scanner.Feed(line); !taken {
			t.Fatalf("scanner refused %q", line)
		}
	}
	report, ok := Parse(scanner.Take())
	if !ok {
		t.Fatalf("summary did not parse:\n%s", strings.Join(lines, "\n"))
	}
	return report
}

func TestScannerHoldsOnlyTheSummary(t *testing.T) {
	scanner := Scanner{}
	if taken, _ := scanner.Feed("go: downloading something"); taken {
		t.Fatal("a line before the summary was taken")
	}
	block := sampleBlock()
	for index, line := range block {
		taken, complete := scanner.Feed(line)
		if !taken {
			t.Fatalf("line %d refused: %q", index, line)
		}
		if complete != (index == len(block)-1) {
			t.Fatalf("line %d reported complete=%v", index, complete)
		}
	}
	if taken, _ := scanner.Feed(`{"msg":"listening"}`); taken {
		t.Fatal("a line after the summary was taken")
	}
}

// A summary emitted by Middlewares has no listening line, so the first line that
// cannot belong to one is what ends it — and that line is still the
// application's, so it has to come back out.
func TestSummaryWithoutAListeningLineEndsAtTheNextLine(t *testing.T) {
	scanner := Scanner{}
	for _, line := range blockWith(baseRows, "") {
		if taken, complete := scanner.Feed(line); !taken || complete {
			t.Fatalf("line %q: taken=%v complete=%v", line, taken, complete)
		}
	}
	taken, complete := scanner.Feed("time=09:00:00 level=INFO msg=ready")
	if taken || !complete {
		t.Fatalf("taken=%v complete=%v, want the line released and the summary complete", taken, complete)
	}
}

// Trailing whitespace is not something to depend on: the first row of the art
// ends in spaces, and anything between the application and this reader is free
// to drop them.
func TestScannerFindsASummaryWhoseArtWasTrimmed(t *testing.T) {
	scanner := Scanner{}
	if taken, _ := scanner.Feed(strings.TrimRight(Art[0], " ")); !taken {
		t.Fatal("a trimmed first row was not recognized")
	}
}

func TestParseReadsTheRowsAndTheFactsAroundThem(t *testing.T) {
	report := scan(t, sampleBlock())
	if report.Version != "Popcorn Web 0.1.0" || report.Environment != "dev" || report.ConfigFile != "config.dev.toml" {
		t.Fatalf("facts read as %+v", report)
	}
	if report.Listening != "http://localhost:8080" {
		t.Fatalf("listening read as %q", report.Listening)
	}
	want := []pwtree.Entry{
		{Key: "html.streaming", Value: "true"},
		{Key: "html.bot_async_timeout", Value: "5s", Source: "file"},
		{Key: "server.port", Value: "8080", Source: "flag"},
	}
	if len(report.Entries) != len(want) {
		t.Fatalf("read %d entries, want %d: %+v", len(report.Entries), len(want), report.Entries)
	}
	for index, entry := range want {
		if report.Entries[index] != entry {
			t.Fatalf("entry %d read as %+v, want %+v", index, report.Entries[index], entry)
		}
	}
}

// A banner that no longer says what this reads is not something to guess at: the
// caller prints the summary unchanged, which is the behavior it replaces.
func TestParseRefusesABannerItDoesNotRecognize(t *testing.T) {
	block := sampleBlock()
	block[3] = Art[3] + "   running in dev"
	if _, ok := Parse(block); ok {
		t.Fatal("an unrecognized banner parsed")
	}
	if _, ok := Parse(block[:3]); ok {
		t.Fatal("a truncated summary parsed")
	}
}

func TestDiffIsEmptyForTheSameSummaryTwice(t *testing.T) {
	first := scan(t, sampleBlock())
	restarted := sampleBlock()
	restarted[2] = Art[2] + "   started at 2026-08-12 09:04:11 JST"
	second := scan(t, restarted)
	if changes := Diff(first, second); len(changes) != 0 {
		t.Fatalf("unchanged summary reported %+v", changes)
	}
	if facts := Facts(first, second); len(facts) != 0 {
		t.Fatalf("a restart alone reported %+v", facts)
	}
}

func TestDiffReportsWhatChangedAppearedAndWentAway(t *testing.T) {
	first := scan(t, sampleBlock())
	second := scan(t, blockWith([]string{
		"└─ html",
		"   ├─ streaming              true",
		"   ├─ bot_async_timeout      10s             ← file",
		"   └─ bot_detection          true            ← file",
	}, "http://localhost:8080"))

	changes := Diff(first, second)
	if len(changes) != 3 {
		t.Fatalf("reported %d changes: %+v", len(changes), changes)
	}
	if changes[0].Key != "server.port" || changes[0].Kind != Removed || changes[0].Old.Value != "8080" {
		t.Fatalf("first change is %+v, want server.port removed", changes[0])
	}
	if changes[1].Key != "html.bot_async_timeout" || changes[1].Kind != Changed {
		t.Fatalf("second change is %+v, want bot_async_timeout changed", changes[1])
	}
	if changes[2].Key != "html.bot_detection" || changes[2].Kind != Added {
		t.Fatalf("third change is %+v, want bot_detection added", changes[2])
	}
}

// The place that won a key is reported for the same reason the value is, so a
// key that stopped being a default changed even though it resolved to what it
// already was.
func TestDiffReportsAPlaceThatChangedWithoutTheValue(t *testing.T) {
	first := scan(t, sampleBlock())
	rows := append([]string{}, baseRows...)
	rows[1] = "│  ├─ streaming              true            ← env"
	changes := Diff(first, scan(t, blockWith(rows, "http://localhost:8080")))
	if len(changes) != 1 || changes[0].Kind != Changed {
		t.Fatalf("reported %+v", changes)
	}
	if mark := changes[0].mark(); mark != "default → env" {
		t.Fatalf("mark is %q, want %q", mark, "default → env")
	}
	if value := changes[0].value(); value != "true" {
		t.Fatalf("value is %q, want the value printed once", value)
	}
}

func TestFactsReportAPortTheApplicationCouldNotBind(t *testing.T) {
	first := scan(t, sampleBlock())
	facts := Facts(first, scan(t, blockWith(baseRows, "http://localhost:8081")))
	if len(facts) != 1 || facts[0].Label != "listening" || facts[0].New != "http://localhost:8081" {
		t.Fatalf("reported %+v", facts)
	}
}

// The shape the whole thing exists for: one changed key, read in the section it
// lives in, with nothing else on screen.
func TestRenderKeepsTheSectionAndDropsEverythingElse(t *testing.T) {
	first := scan(t, sampleBlock())
	rows := append([]string{}, baseRows...)
	rows[2] = "│  └─ bot_async_timeout      10s             ← file"
	second := scan(t, blockWith(rows, "http://localhost:8080"))

	var out strings.Builder
	Render(&out, Facts(first, second), Diff(first, second), func(text string) string { return text })
	rendered := out.String()
	t.Log("\n" + rendered)
	for _, want := range []string{"└─ html", "bot_async_timeout", "5s → 10s"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("rendered report missing %q:\n%s", want, rendered)
		}
	}
	for _, unwanted := range []string{"streaming", "server", "8080", "Popcorn Web"} {
		if strings.Contains(rendered, unwanted) {
			t.Fatalf("rendered report still carries %q:\n%s", unwanted, rendered)
		}
	}
}

func TestRenderMarksWhatAppearedAndWhatWentAway(t *testing.T) {
	first := scan(t, sampleBlock())
	rows := append([]string{}, baseRows...)
	rows[2] = "│  └─ bot_detection          true            ← file"
	second := scan(t, blockWith(rows, "http://localhost:8080"))

	var out strings.Builder
	Render(&out, Facts(first, second), Diff(first, second), func(text string) string { return text })
	rendered := out.String()
	t.Log("\n" + rendered)
	for _, want := range []string{"bot_async_timeout", "← removed", "bot_detection", "← added"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("rendered report missing %q:\n%s", want, rendered)
		}
	}
}
