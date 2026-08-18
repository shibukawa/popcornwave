package pwscript_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/shibukawa/popcornweb/internal/pwscript"
)

// The idiomatic shape, which is the one an author writes and the only one this
// scanner claims to understand.
func TestReadsTheIdiomaticBlock(t *testing.T) {
	block, err := pwscript.Read(`
export function setup({ el, teardown, props: { id, label } }) {
	let count = 0;
	teardown(() => {});
	return {
		increment(event) { count++; },
		reset: () => { count = 0; },
	};
}
`)
	if err != nil {
		t.Fatal(err)
	}
	if block.Unread != "" {
		t.Fatalf("unread: %s", block.Unread)
	}
	if want := []string{"id", "label"}; !slices.Equal(block.Parameters, want) {
		t.Errorf("parameters are %v, want %v", block.Parameters, want)
	}
	if want := []string{"increment", "reset"}; !slices.Equal(block.Handlers, want) {
		t.Errorf("handlers are %v, want %v", block.Handlers, want)
	}
}

// A block asking for no parameter is the ordinary case rather than a failure,
// and it still publishes its handlers.
func TestParametersAreOptional(t *testing.T) {
	block, err := pwscript.Read(`export function setup({ el }) { return { go }; }`)
	if err != nil {
		t.Fatal(err)
	}
	if block.Unread != "" || len(block.Parameters) != 0 {
		t.Fatalf("unread %q, parameters %v", block.Unread, block.Parameters)
	}
	if want := []string{"go"}; !slices.Equal(block.Handlers, want) {
		t.Errorf("handlers are %v, want %v", block.Handlers, want)
	}
}

// The four things a pattern match reads wrong. Each of these holds text that
// would otherwise be read as structure, and none of it is.
func TestQuotingHidesStructure(t *testing.T) {
	block, err := pwscript.Read("" +
		"const brace = \"return { stolen }\";\n" +
		"const template = `${ \"return { alsoStolen }\" } and { more }`;\n" +
		"// return { commented }\n" +
		"/* return { blocked } */\n" +
		"const pattern = /return \\{ pattern \\}/;\n" +
		"export function setup({ el }) {\n" +
		"	return { real };\n" +
		"}\n")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"real"}; !slices.Equal(block.Handlers, want) {
		t.Errorf("handlers are %v, want %v", block.Handlers, want)
	}
}

// A slash after a value is division, and reading it as a literal would swallow
// the rest of the file.
func TestDivisionIsNotARegularExpression(t *testing.T) {
	block, err := pwscript.Read(`
export function setup({ el }) {
	const half = el.offsetWidth / 2;
	const ratio = (half) / 3;
	return { report };
}
`)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"report"}; !slices.Equal(block.Handlers, want) {
		t.Errorf("handlers are %v, want %v (unread %q)", block.Handlers, want, block.Unread)
	}
}

// What the scanner declines rather than half-reads. Each of these is a real
// shape an author may write, and each one publishes nothing rather than
// publishing a guess.
func TestDeclinesWhatItCannotRead(t *testing.T) {
	for _, testCase := range []struct {
		name, source, reason string
	}{
		{
			name:   "a returned variable",
			source: `export function setup({ el }) { const handlers = { a }; return handlers; }`,
			reason: "other than an object literal",
		},
		{
			name:   "a spread in the returned object",
			source: `export function setup({ el }) { return { ...shared, a }; }`,
			reason: "where a name was expected",
		},
		{
			name:   "no setup at all",
			source: `export function other() {}`,
			reason: "exports no setup",
		},
		{
			name:   "a setup that returns nothing",
			source: `export function setup({ el }) { el.hidden = false; }`,
			reason: "returns nothing",
		},
		{
			name:   "the whole bag taken under one name",
			source: `export function setup({ props }) { return { a }; }`,
			reason: "as a whole",
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			block, err := pwscript.Read(testCase.source)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(block.Unread, testCase.reason) {
				t.Errorf("unread is %q, want it to mention %q", block.Unread, testCase.reason)
			}
			if len(block.Handlers) != 0 {
				t.Errorf("it published %v anyway", block.Handlers)
			}
		})
	}
}

// A return nested in a branch is not the function's own answer, and reading one
// as if it were would publish a set that only some visitors get.
func TestANestedReturnIsNotTheAnswer(t *testing.T) {
	block, err := pwscript.Read(`
export function setup({ el }) {
	if (!el) { return {}; }
	el.hidden = false;
}
`)
	if err != nil {
		t.Fatal(err)
	}
	if block.Unread == "" {
		t.Errorf("a nested return was taken as the answer: %v", block.Handlers)
	}
}

// A source this scanner cannot walk at all is an error rather than an empty
// answer, because the two mean different things to whoever is deciding what to
// check.
func TestUnwalkableSourceIsAnError(t *testing.T) {
	if _, err := pwscript.Read("const broken = 'never closed;"); err == nil {
		t.Error("an unterminated string was walked")
	}
}
