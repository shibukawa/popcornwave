# cgosqlite

`cgosqlite` is Petitweb's small native SQLite driver for TinyGo and
`force_tinygo_logic` builds. It implements `database/sql` over a statically
linked copy of the official SQLite amalgamation; it does not use WebAssembly or
load the operating system's SQLite library.

The bundled SQLite is 3.53.3 (`sqlite-amalgamation-3530300.zip`). Loadable
extensions and double-quoted string literals are disabled. URI filenames accept
only `mode`, `cache`, `immutable`, and `busy_timeout` (0–60000 ms).

Import the higher-level `contrib/database/sqlite` package unless a direct test
of this backend is required.
