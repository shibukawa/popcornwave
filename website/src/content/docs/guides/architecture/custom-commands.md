---
title: Custom Commands
description: The command line of the binary you build — config flags, scaffold output, and subcommands of your own.
sidebar:
  order: 2
---

`pw` controls development, but it is not the command your users deploy. The
binary produced by `pw build` has its own CLI, generated from the same typed
declarations that drive configuration.

For the development tool itself, see [pw command](/pw/overview/).

## Every setting is a flag

Each registered configuration key is reachable three ways, in increasing
precedence:

```
default  <  TOML file  <  environment variable  <  command-line option
```

The default names derive from the prefix and the field:

| Prefix + field | Key | Option | Environment |
| --- | --- | --- | --- |
| `app` + `EnvLabel` | `app.env_label` | `--app-env_label` | `APP_ENV_LABEL` |
| `middleware.rdb` + `WriteGroup` | `middleware.rdb.write_group` | `--middleware-rdb-write_group` | `MIDDLEWARE_RDB_WRITE_GROUP` |

A field can override any of them:

```go
type ServerConfig struct {
	Port int `key:"listen_port" default:"8080" opt:"port,p" env:"PORT" help:"HTTP listen port"`
}
```

| Tag | Effect |
| --- | --- |
| `default:"value"` | value when nothing else supplies one |
| `key:"name"` | the stable TOML/config key |
| `opt:"long"` or `opt:"long,s"` | the long option, and an optional single-character short one |
| `env:"NAME"` | an exact environment variable name; `env:"-"` disables environment input |
| `help:"text"` | the description shown in usage and in scaffolds |

When `opt` overrides the long option, the environment name derives from that
instead.

```sh
./myapp --port=9090
PORT=9090 ./myapp
```

## Choosing a configuration file

`APP_ENV` selects which project-local file is read, and `--config-path`
overrides the search entirely:

```sh
APP_ENV=stg ./myapp
./myapp --config-path ./deploy/staging.toml
```

See [Configuration](/guides/architecture/configuration/) for the full resolution order.

## Printing a scaffold

```sh
./myapp --generate-config toml > config.dev.toml
./myapp --generate-config env > .env
```

Both formats include **every registered prefix**, whether it belongs to the
framework or the application, and preserve `default` values and `help` text as
comments. The binary reports what its linked packages actually registered. Add
a dependency with configuration, and that configuration appears the next time
you generate a scaffold.

Either form exits after writing; the server does not start.

## Your own subcommands

A subcommand needs only a struct and one registration call. `pw generate` reads
that call site and writes the parser, leaving no parallel flag wiring to
maintain.

```go
package main

type importCommand struct {
	Path   string   `arg:"required" help:"CSV file to import"`
	Label  string   `arg:"optional"`
	Tags   []string `arg:"*"`
	DryRun bool     `default:"false" help:"parse without writing"`
}
```

| Tag | Meaning |
| --- | --- |
| `arg:"required"` | a positional argument that must be supplied |
| `arg:"optional"` | a positional argument that may be omitted |
| `arg:"*"` | variadic positional arguments, zero or more |
| *(no `arg` tag)* | an option, with the same tags as configuration fields |

Register it before configuration is parsed, then dispatch:

```go
func main() {
	handlers.RegisterConfig()
	pw.RegisterSubCommand[importCommand]("import", "import a CSV file")

	if err := pw.ParseConfig(); err != nil {
		log.Fatal(err)
	}
	if command, ok := pw.Command[importCommand](); ok {
		if err := runImport(context.Background(), command); err != nil {
			log.Fatal(err)
		}
		return
	}

	if err := pw.Run(context.Background(), handlers.Handlers()); err != nil {
		log.Fatal(err)
	}
}
```

```sh
./myapp import ./data/users.csv --dry-run
```

`pw.Command[T]` reports `false` unless that subcommand was the one selected, so
the `if` doubles as the dispatch. Only the chosen subcommand receives input.
Calling `pw.ParseConfig` yourself is safe: `pw.Run` calls it too, and it parses
exactly once.

Registration order matters for the same reason it does for configuration —
generated definitions register during package `init`, so `RegisterSubCommand`
must run after every `init` and before `ParseConfig`. Registering after parsing
panics.

The subcommand shares the server's parsed configuration. `pw.Config[T]`
therefore returns the same values the server would use, including the DSN,
without a second settings path to keep in sync.

The database pool is a separate matter: it is opened by `pw.Run` and
`pw.Middlewares`, not by `ParseConfig`. A subcommand that needs a connection
opens one from the configured DSN itself.
