package pwgen

import "github.com/shibukawa/tinybind-go/generator"

const (
	pwPackage          = "github.com/shibukawa/popcornwave/pw"
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
		),
		generator.ResponseWriteCall(
			generator.Function(pwPackage, "WriteAPI"),
			generator.GenericType("response", 0),
		),
		generator.ResponseWriteStatusCall(
			generator.Function(pwPackage, "WriteStatus"),
			generator.GenericType("response", 0),
			generator.Argument("status", 2),
		),
		generator.StreamCreateCall(
			generator.Function(pwPackage, "WriteStream"),
			generator.GenericType("stream", 0),
		),
		generator.ConfigBindCall(
			generator.Function(pwPackage, "RegisterConfig"),
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
