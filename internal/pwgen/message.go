package pwgen

import (
	"github.com/shibukawa/popcornwave/internal/pwmsg"
	"github.com/shibukawa/tinybind-go/generator"
	templates "github.com/shibukawa/tinybind-go/templates/htmlbind"
)

// Binding names the implicit bindings this framework puts in every template's
// scope. They are constants rather than strings spelled at each use, because a
// template author writes them and a mismatch between the declaration and the
// documentation is a name that resolves to nothing.
const (
	// MessageLocaleBinding carries the resolved locale to every message call.
	// It is typed, so it is unwritable in markup: a template reading it is a
	// generation error naming the binding, which is the behaviour that keeps
	// the value out of positions LangTagBinding serves.
	MessageLocaleBinding = "pwlocale"
	// LangSegmentBinding is the URL path segment, empty in every mode that does
	// not carry the locale in the path.
	LangSegmentBinding = "lang"
	// LangTagBinding is the resolved tag, never empty.
	LangTagBinding = "langtag"
)

// varyAxisFor is the response header a binding's value depends on.
//
// It is empty here for every binding, because the axis a route needs is decided
// by that route's mode and a binding declaration is per compilation. A path-mode
// route recovers the locale from its own URL and declares nothing; a negotiated
// route varies, and policy:locale-vary-correctness has that emitted from the
// route's mode rather than from the binding.
//
// Declaring a non-empty axis here would put Accept-Language on every response of
// every project, including the path-mode ones two URLs already separate, which
// is the opposite_failure policy:preference-vary-correctness names.
const varyAxisFor = ""

// MessageBindings returns the implicit bindings every HTML template compiles
// against, whether or not the project declares any locale.
//
// They are unconditional because a binding no template reads generates
// byte-identical Go: making the list depend on the catalog would make the
// generated output of a project differ by whether it had adopted i18n yet, for
// no saving. See .knowledge data:locale-bindings.
func MessageBindings() []templates.ImplicitBinding {
	provider := func(name, result string) templates.BindingProvider {
		return templates.BindingProvider{Package: pwRuntimePackage, Name: name, Result: result}
	}
	return []templates.ImplicitBinding{{
		Name:     MessageLocaleBinding,
		Provider: provider("MessageLocale", pwRuntimePackage+".Locale"),
		VaryAxis: varyAxisFor,
	}, {
		Name:        LangSegmentBinding,
		Provider:    provider("LangSegment", ""),
		PathSegment: true,
		VaryAxis:    varyAxisFor,
	}, {
		Name:     LangTagBinding,
		Provider: provider("LangTag", ""),
		VaryAxis: varyAxisFor,
	}}
}

// ApplyMessages fills the message half of the generator options from a catalog
// that has already been generated.
//
// The symbol table is data rather than a naming convention because an ID may
// carry a hyphen and is therefore not a Go identifier; the catalog decides how a
// slug becomes a symbol, and this hands over the result. See
// .knowledge decision:message-code-shape id_to_symbol_is_a_table.
//
// importPath is where the generated message package lives. A run with no
// catalog passes an empty table, which leaves every reference an unknown ID and
// is what makes a template referencing a message the project never declared fail
// generation rather than compile into a call to nothing.
func ApplyMessages(options *generator.Options, symbols map[string]pwmsg.Symbol, importPath string) {
	options.ImplicitBindings = MessageBindings()
	options.MessageContextBinding = MessageLocaleBinding
	options.Messages = MessageSymbolTable(symbols, importPath)
}

// MessageSymbolTable converts a catalog's symbols into the table the template
// compiler resolves against.
//
// It is separate from ApplyMessages because a page tree is compiled through its
// own options type rather than through the generator's, and both paths must be
// handed the same table. A seam reaching one compile path and not the other is
// how a feature ships absent on filesystem routes.
//
// An empty catalog produces a nil table, which leaves every reference an unknown
// ID: a template naming a message the project never declared fails generation
// rather than compiling into a call to nothing.
func MessageSymbolTable(symbols map[string]pwmsg.Symbol, importPath string) map[string]templates.MessageSymbol {
	if len(symbols) == 0 {
		return nil
	}
	table := make(map[string]templates.MessageSymbol, len(symbols))
	for id, symbol := range symbols {
		table[id] = templates.MessageSymbol{
			Package: importPath,
			Name:    symbol.Name,
			Params:  symbol.Params,
		}
	}
	return table
}
