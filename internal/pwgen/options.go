package pwgen

import "github.com/shibukawa/tinybind-go/generator"

const (
	pwPackage        = "github.com/shibukawa/popcornwave/pw"
	pwRuntimePackage = "github.com/shibukawa/popcornwave/pwruntime"
)

// Options returns TinyBind generator options extended with the stable pw API.
// Options builds the generator configuration. sqlDialect names the target
// database for .pw.sql sources; it is required whenever the run discovers one,
// because a silently assumed dialect emits placeholders the engine rejects.
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
		generator.StreamCreateCall(
			generator.Function(pwPackage, "NewStream"),
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
	options.HTMLTemplatePattern = "*.pw.html"
	options.SQLTemplatePattern = "*.pw.sql"
	options.SQLDialect = sqlDialect
	options.SQLContextOnlyAPI = true
	options.SQLExecutorResolver = &generator.SymbolPattern{
		PackagePath: pwRuntimePackage,
		Name:        "SQLExecutor",
	}
	return options, nil
}
