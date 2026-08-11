package pwgen

import "github.com/shibukawa/tinybind-go/generator"

const (
	pwPackage          = "github.com/shibukawa/popcornwave/pw"
	pwConfigPackage    = "github.com/shibukawa/popcornwave/pwconfig"
	pwRuntimePackage   = "github.com/shibukawa/popcornwave/pwruntime"
	pwDynamoPackage    = "github.com/shibukawa/popcornwave/database/dynamo"
	pwFirestorePackage = "github.com/shibukawa/popcornwave/database/firestore"
	// pwAttributePrefix mirrors pw.UpdateAttributePrefix. It is repeated rather
	// than imported because this package is a host-side tool and pw is the
	// runtime; a test asserts the two agree.
	//
	// It is the module's own default rather than this framework's brand, and
	// that is deliberate: system:tinybind routetree does not thread the prefix
	// into the templates it compiles, so a page tree would keep the default
	// while a registered-router template took the brand, and one document would
	// hold both spellings. One agreed spelling is worth more than the brand
	// until the option reaches both paths.
	pwAttributePrefix = "tb"
)

// Options returns TinyBind generator options extended with the stable pw API.
// Options builds the generator configuration. sqlDialect names the target
// database for .pw.sql sources; it is required whenever the run discovers one,
// because a silently assumed dialect emits placeholders the engine rejects.
// The Popcorn Wave source suffixes. They are the generator's discovery globs
// and the formatter's, so they live here rather than in either caller.
const (
	HTMLTemplatePattern   = "*.pw.html"
	SQLTemplatePattern    = "*.pw.sql"
	DynamoTemplatePattern = "*.pw.dynamo"
	// FirestoreTemplatePattern is the base-name glob of a Firestore query
	// declaration. It follows the .pw. convention every other declaration
	// source here uses rather than the module's own .tb. default.
	FirestoreTemplatePattern = "*.pw.firestore"
)

func Options(sqlDialect string) (generator.Options, error) {
	registry := generator.NewCallRegistry()
	patterns := []generator.CallPattern{
		generator.RequestBindCall(
			generator.Function(pwPackage, "Parse"),
			generator.GenericType("request", 0),
			generator.RequestArgument(0),
		),
		generator.ResponseWriteCall(
			generator.Function(pwPackage, "WriteAPI"),
			generator.GenericType("response", 0),
			generator.WriterArgument(0),
			generator.RequestArgument(1),
		),
		generator.ResponseWriteStatusCall(
			generator.Function(pwPackage, "WriteStatus"),
			generator.GenericType("response", 0),
			generator.Argument("status", 2),
			generator.WriterArgument(0),
			generator.RequestArgument(1),
		),
		generator.StreamCreateCall(
			generator.Function(pwPackage, "WriteStream"),
			generator.GenericType("stream", 0),
			generator.WriterArgument(0),
			generator.RequestArgument(1),
		),
		generator.ConfigBindCall(
			generator.Function(pwPackage, "RegisterConfig"),
			generator.GenericType("config", 0),
			generator.Argument("prefix", 0),
		),
		// The framework's own bindings register through the shared package
		// rather than through pw, because a settings file is not a transport
		// concern and the runtime that binds it need not be the one that
		// serves. An application still writes pw.RegisterConfig above.
		generator.ConfigBindCall(
			generator.Function(pwConfigPackage, "Register"),
			generator.GenericType("config", 0),
			generator.Argument("prefix", 0),
		),
		generator.ConfigSubCommandCall(
			generator.Function(pwPackage, "RegisterSubCommand"),
			generator.GenericType("config", 0),
			generator.Argument("name", 0),
			generator.Argument("help", 1),
		),
		generator.ConfigSubCommandCall(
			generator.Function(pwPackage, "SubCommand"),
			generator.GenericType("config", 0),
			generator.Argument("name", 0),
			generator.Argument("help", 1),
		),
		// The portable spelling, for the same reason pwconfig.Register is
		// registered above: a subcommand is a fact about the command line
		// rather than about a transport, so the file declaring one is compiled
		// by both builds and must not name either runtime.
		generator.ConfigSubCommandCall(
			generator.Function(pwConfigPackage, "RegisterSubCommand"),
			generator.GenericType("config", 0),
			generator.Argument("name", 0),
			generator.Argument("help", 1),
		),
		generator.ErrorResponseCall(
			generator.Function(pwPackage, "BadRequest"),
			generator.Constant("status", 400),
			generator.Constant("error_name", "BadRequest"),
		),
		generator.ErrorResponseCall(
			generator.Function(pwPackage, "Validation"),
			generator.Constant("status", 400),
			generator.Constant("error_name", "Validation"),
		),
		generator.ErrorResponseCall(
			generator.Function(pwPackage, "Unauthorized"),
			generator.Constant("status", 401),
			generator.Constant("error_name", "Unauthorized"),
		),
		generator.ErrorResponseCall(
			generator.Function(pwPackage, "Forbidden"),
			generator.Constant("status", 403),
			generator.Constant("error_name", "Forbidden"),
		),
		generator.ErrorResponseCall(
			generator.Function(pwPackage, "NotFound"),
			generator.Constant("status", 404),
			generator.Constant("error_name", "NotFound"),
		),
		generator.ErrorResponseCall(
			generator.Function(pwPackage, "Conflict"),
			generator.Constant("status", 409),
			generator.Constant("error_name", "Conflict"),
		),
		generator.ErrorResponseCall(
			generator.Function(pwPackage, "PayloadTooLarge"),
			generator.Constant("status", 413),
			generator.Constant("error_name", "PayloadTooLarge"),
		),
		generator.ErrorResponseCall(
			generator.Function(pwPackage, "InternalServerError"),
			generator.Constant("status", 500),
			generator.Constant("error_name", "InternalServerError"),
		),
	}
	// Every remaining pw entry that takes the transport and names no model the
	// generator binds or encodes. Without a pattern each of these looks to the
	// rewriter exactly like an untraceable third-party call, and every handler
	// making one is refused with a remedy only this framework can supply — which
	// is the difference between a second backend an application can adopt and
	// one it cannot.
	//
	// The module's own WriteError needed this shape first, and found it by
	// refusing every handler that reported an error.
	for _, transport := range []struct {
		name    string
		writer  int
		request int
	}{
		// Response writers. The fragment they take is already bound, so there is
		// no model here for the generator to know about — only the two arguments
		// that collapse.
		{name: "WriteProblem", writer: 0, request: 1},
		{name: "WriteHTML", writer: 0, request: 1},
		{name: "WriteHTMLPage", writer: 0, request: 1},
		{name: "WriteHTMLChain", writer: 0, request: 1},
		{name: "WriteHTMLFragment", writer: 0, request: 1},
		// The update surface, whose entries answer with records rather than a
		// value of a declared type.
		{name: "WriteUpdate", writer: 0, request: 1},
		{name: "WriteUpdateNavigate", writer: 0, request: 1},
		{name: "Redraw", writer: 0, request: 1},
		{name: "RedrawComponents", writer: 0, request: 1},
		// Predicates and the specification endpoint, which read the request and
		// write nothing a type describes.
		{name: "Redirect", writer: 0, request: 1},
		{name: "RedirectSeeOther", writer: 0, request: 1},
		{name: "WantsUpdate", writer: -1, request: 0},
		{name: "QueryValue", writer: -1, request: 0},
		{name: "FormValue", writer: -1, request: 0},
		{name: "IsBot", writer: -1, request: 0},
		{name: "OpenAPIJSON", writer: 0, request: 1},
		// The two accessors a generated route decoder reads through. They take
		// the request and no writer, and they exist so a decoder never reaches
		// into the request value itself — which is the read no second transport
		// can follow.
		{name: "PathValue", writer: -1, request: 0},
		{name: "Queries", writer: -1, request: 0},
	} {
		options := []generator.CallPatternOption{generator.RequestArgument(transport.request)}
		if transport.writer >= 0 {
			options = append(options, generator.WriterArgument(transport.writer))
		}
		patterns = append(patterns, generator.TransportCall(
			generator.Function(pwPackage, transport.name), options...))
	}
	if err := registry.Register(patterns...); err != nil {
		return generator.Options{}, err
	}
	options, err := registry.Options(generator.DefaultOptions())
	if err != nil {
		return generator.Options{}, err
	}
	// Nothing this project generates is an input to what it reads. TinyBind
	// recognizes its own header and no other, so the Popcorn Wave prefix is
	// registered here: without it a generated page registry is analyzed as if a
	// developer had written it, and its page registrations become documented API
	// routes.
	options.GeneratedHeaders = []string{GeneratedHeaderPrefix}
	// One prefix names the generated boundary attributes, the placeholder
	// element, and the boundary ids. The browser runtime is built for it, so a
	// document holding the module's default beside this one would address
	// regions the runtime never looks for.
	options.DataAttributePrefix = pwAttributePrefix
	options.HTMLTemplatePattern = HTMLTemplatePattern
	options.SQLTemplatePattern = SQLTemplatePattern
	options.DynamoTemplatePattern = DynamoTemplatePattern
	options.FirestoreTemplatePattern = FirestoreTemplatePattern
	options.SQLDialect = sqlDialect
	options.SQLContextOnlyAPI = true
	options.SQLExecutorResolver = &generator.SymbolPattern{
		PackagePath: pwRuntimePackage,
		Name:        "SQLExecutor",
	}
	// Generated NoSQL queries resolve the process handle through the framework
	// function instead of tinybind's own context key, so a request context
	// carries no tinybind-owned node and a call site pays no context.Value
	// walk, per the same seam SQLExecutorResolver uses.
	options.DynamoHandleResolver = &generator.SymbolPattern{
		PackagePath: pwDynamoPackage,
		Name:        "Handle",
	}
	options.FirestoreHandleResolver = &generator.SymbolPattern{
		PackagePath: pwFirestorePackage,
		Name:        "Handle",
	}
	return options, nil
}
