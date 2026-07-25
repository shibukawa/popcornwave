# Hello World

A classic Popcorn Wave example generated with:

```bash
pw init helloworld --tailwind
```

It demonstrates nested HTML templates, Tailwind CSS, and an atomic SQLite page-view counter.

The module always uses the repository checkout through:

```go
replace github.com/shibukawa/popcornwave => ../../
```

Initialize the configured SQLite database, then run the application:

```bash
go run ../../cmd/pw schema-init
go run ./cmd/helloworld
```

For the full regeneration and Tailwind watch loop:

```bash
devbox shell
pw dev
```

Open <http://localhost:8080>. The local `helloworld.db` file is ignored by Git.
