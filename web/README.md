# SQLS LSP playground

A local Monaco editor connected to the real `sqls` binary over stdio through a
WebSocket bridge. Each browser connection starts a separate language server.
The bridge creates a temporary SQLite database with `users` and `orders`, so no
external database or credentials are needed. Editor assets are served locally.

Requirements: Go matching `go.mod`, Python 3 with SQLite, and Node.js 22+.

From the repository root:

```sh
go build -o sqls .
npm --prefix web ci
npm --prefix web start
```

Open **http://127.0.0.1:3000**. Use `PORT` to change the port and `SQLS_BINARY` to
select another binary. The bridge binds only to loopback and accepts browser
WebSockets from the same origin. Restart starts a fresh server and reopens the
current document. Query execution runs against the temporary fixture database.

The editor wires SQLS completion, hover, definition, rename, signature help,
document formatting, and selection formatting. Diagnostics appear in the editor
and sidebar. The protocol panel shows actual requests, responses, notifications,
and server stderr.

## Automated checks

The stdio suite needs only Go and Python. It builds SQLS and tests framing,
initialization and shutdown, document synchronization, SQLite schema completion,
hover, definition, rename versions, signature help, diagnostics, formatting,
UTF-16 ranges, multiline strings, SQL quote escaping, and malformed parameters:

```sh
make test-lsp
```

Browser tests drive the editor UI and check that SQLS responses are applied:

```sh
npm --prefix web ci
cd web
npx playwright install chromium
npm test
```

`npm test` rebuilds SQLS, starts and stops its own bridge on port 3001, and runs
nine tests. `SQLS_TEST_PORT` overrides the test port, leaving the playground on
port 3000 available during testing.
If Chromium is already installed, set `CHROMIUM_PATH` to its executable to avoid
downloading another browser. `make test-web` installs the pinned npm dependencies
and runs the same suite. Test results are ignored under `web/test-results/`.

The existing Go checks remain available with `go test -race ./...` and
`go vet ./...`.

## WHERE diagnostics

The SQLite fixture accepts `WHERE u.id`: a nonzero integer is a true value.
An incomplete predicate such as `WHERE u.id = ;` produces a syntax error.
For a configured PostgreSQL connection, SQLS also reports a type mismatch for
known non-boolean columns used directly in `WHERE` or `HAVING`; boolean columns
are valid. Type checks use cached schema information and leave unknown function
return types and user-defined domains to the database.
