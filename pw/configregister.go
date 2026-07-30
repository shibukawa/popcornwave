package pw

// Binding the framework's own configuration has to happen after the generated
// definitions exist, because configbind.Bind panics on a type it has no
// definition for. Go initializes a package's files in lexical file name order,
// and this file sorts after configbind_gen.go; keep it that way when renaming
// either file.
func init() {
	RegisterConfig[ServerConfig]("server")
	RegisterConfig[SecurityConfig]("security")
	RegisterConfig[SessionConfig]("session")
	RegisterConfig[ObservabilityConfig]("observability")
	RegisterConfig[MiddlewareConfig]("middleware")
	RegisterConfig[HTMLConfig]("html")
	seedConfigDefaults(defaultHTMLConfig)
}
