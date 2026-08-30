# Contributing

Thanks for considering a contribution to configsync.

## Development setup

Requires Go 1.22 or later.

```
git clone https://github.com/hellpuffyt/configsync.git
cd configsync
go build ./...
```

## Before opening a pull request

Run the full local gate, exactly as CI does:

```
go build ./...
go vet ./...
gofmt -l .        # must print nothing
go test ./...
```

If `gofmt -l .` prints any file names, run `gofmt -w .` and commit the result.

## Adding a new configuration format

1. Add a parser in `internal/parser/` that returns `model.Config` (a flat,
   dotted-path map of `model.Value`). Reuse `parser.Flatten` if your format
   decodes naturally into `map[string]interface{}` / `[]interface{}` trees.
2. Register the new extension in `parser.DetectFormat`.
3. Add parsing tests covering comments, quoting, and nested structures, and
   an example file under `examples/`.

## Adding a new difference classification

Classification logic lives entirely in `internal/diff/diff.go`. Keep
`Compare` deterministic and side-effect free, and add table-driven tests in
`internal/diff/diff_test.go` for both the new base classification and its
interaction with `expected_variance` and `must_match` rules.

## Secret safety

Any change touching `internal/report`, `internal/snapshot`, or
`internal/rules` (secret pattern matching) must keep the guarantee that a
secret value is never written to any output stream, log, or file. When in
doubt, add a test that asserts a specific secret string never appears in
the rendered output, the way `internal/report/report_test.go` and
`internal/snapshot/snapshot_test.go` already do.

## Commit style

Small, focused commits with a clear, imperative subject line (e.g. "Add
TOML array-of-tables support"). Reference the relevant issue if one exists.

## Reporting a security issue

See [SECURITY](README.md#security) in the README.
