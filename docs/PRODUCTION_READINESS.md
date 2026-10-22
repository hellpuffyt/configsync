# Production readiness — configsync

Audit of the `audit/production-readiness` branch. Evidence only; the
classification at the end is a proposal for the maintainer to confirm.

- Tested commit: `c364b6a1b0fe9212c63a59ef267fd5b6369eb659` (this file was
  added in the next commit and changes no code)
- Date: 2026-10-22
- Host: macOS 27 (Apple Silicon), Go 1.27.3. `go.mod` declares `go 1.22`.

## Scope

A read-only CLI that parses `.env`, YAML, JSON, TOML (subset) and
`.properties` files into flattened dotted keys, compares two or more
environments, classifies each key (`missing`, `value-differs`, `empty-in`,
`type-differs`, `expected-variance`), applies glob rules, redacts secrets,
and saves / compares snapshots. One dependency (`gopkg.in/yaml.v3`). No
network access.

## Platforms and prerequisites

- Go 1.22+. Verified locally on macOS arm64.
- CI: Go 1.22/1.23 on ubuntu, Windows and macOS, plus a smoke job on the
  examples; not yet run on this branch.

## Clean-clone verification

Fresh clone of the branch; README "Build from source", "Usage" and
"Testing" followed literally (examples now name files that exist):

| Command | Exit | Result |
|---|---|---|
| `go build -o configsync ./cmd/configsync` | 0 | |
| `go test ./...` | 0 | 103 passed, 0 failed, 0 skipped (tests and subtests) |
| `go test -race ./...` | 0 | |
| `configsync compare` on the three examples with `--rules` | 1 | the README table exactly; `7 key(s) shown, 5 unexpected drift` |
| `compare ... --format json` | 0 (via `head`) | valid JSON, summary present |
| grep for the example secret values in JSON output | — | 0 matches |
| `configsync snapshot ... --out baseline.json` | 0 | `9 keys, 3 environments` |
| `configsync drift --snapshot baseline.json ...` | 0 | `no drift detected` |
| grep for the example secret values in `baseline.json` | — | 0 matches |
| `configsync version` | 0 | `configsync 0.1.0` |

## Commands executed (working clone)

| Command | Exit | Outcome |
|---|---|---|
| `go build ./...`, `go vet ./...`, `gofmt -l .` | 0 | clean |
| `go test -race -count=1 -v ./...` (baseline) | 0 | 90 passed, 0 failed, 0 skipped |
| `go test -race -count=1 -v ./...` (after) | 0 | 103 passed, 0 failed, 0 skipped |
| `go run honnef.co/go/tools/cmd/staticcheck@latest ./...` (v0.8.2) | 0 | no findings |
| `go run golang.org/x/vuln/cmd/govulncheck@latest ./...` (v1.9.0) | 0 | No vulnerabilities found |
| `go mod verify` | 0 | all modules verified |
| `go run github.com/zricethezav/gitleaks/v8@latest git --redact .` | 0 | no leaks (the examples hold placeholder values only) |

The parser regression tests failed on the pre-fix code. The CLI defects
were reproduced with the pre-fix binary (the CLI tests need the new
`run` signature): a URL password printed in the table and stored in the
snapshot, an ignored key reported by `drift` (exit 1), and
`--env prod=a --env prod=b` showing the second file in both columns.

## CI status

- Default branch (`main`): last run `success`.
- Audit branch: pending until pushed. The workflow had no `permissions`
  block; it now has `contents: read` and actions pinned to SHAs.

## Bugs found and fixed

| # | Severity | Bug | Fix | Regression test |
|---|---|---|---|---|
| 1 | High (install broken) | Module path `github.com/prabeshsharma/configsync` vs repository `hellpuffyt/configsync`: the README's `go install` fails. | Module path corrected. | build, clean clone |
| 2 | High (secret leak) | Passwords inside URL values (`DATABASE_URL=postgres://app:pw@db`) were printed and stored in snapshots: only key names were checked. | Redact URL passwords in table/JSON; snapshot keeps redacted URL + hash. | `TestURLPasswordsNeverPrintedOrStored`, `TestRedactCredentials` |
| 3 | Medium | `ignore` rules not applied to `snapshot`/`drift`: an ignored, changing key failed every drift run. | Skip in snapshot; filter in drift. | `TestDriftHonoursIgnore` |
| 4 | Medium | Repeated `--env NAME` silently replaced an environment. | Error. | `TestDuplicateEnvironmentNamesRejected` |
| 5 | Medium | JSON integers above 2^53 rounded, so distinct values compared equal; YAML uint64 wrapped negative; float→int64 overflow. | Exact number handling. | `TestJSONKeepsLargeIntegersExact` |
| 6 | Medium | TOML: escaped quote ended strings for comment scanning; hex/octal/binary ints as strings; inline tables, bare words and duplicate keys accepted silently. | Escape-aware scanning, integer prefixes, explicit errors. | `TestTOMLRejectsWhatItCannotParse`, `TestTOMLValuesItSupports` |
| 7 | Low | `.env` `"a\\nb"` decoded to backslash + newline. | Left-to-right unescape. | `TestEnvDoubleQuotedEscapes` |
| 8 | Low | `.properties` `\uXXXX` escapes not decoded. | Decoded in keys and values. | `TestPropertiesUnicodeEscapes` |
| 9 | Docs | README claimed `.env` `DATABASE_URL` and YAML `database.url` are the same key, and that `*_HOST` matches `database.credentials.host`; neither is true. JSON and snapshot examples named nonexistent files; the example table was hand-aligned. | README corrected; behaviour pinned by tests. | `TestRules_FullPathAndLastSegment`, clean clone |

Also: the CLI returned via `os.Exit` from inside subcommands; `run` now
returns an exit code, which made the CLI testable (`cmd/configsync`
previously had no tests).

## Security review

- Read-only on inputs; writes only the `--out` snapshot; no network.
- Secret handling: key-name patterns (built-in + rules) and URL passwords
  are never printed or stored. Remaining risk (documented): secrets under
  innocuous key names; unsalted SHA-256 hashes in snapshots can be
  brute-forced for weak secrets.
- Parsing untrusted files: stdlib/yaml.v3 parsers, line-based custom
  parsers; no code execution or path handling based on content.
- CI: least-privilege token, pinned actions.

## Known limitations

See README "Limitations". Linux and Windows rely on CI.

## Acceptance checklist

| Item | Expected behaviour | Command / test | Result | Actual | Reference |
|---|---|---|---|---|---|
| Install / build | Documented commands work | clean clone | pass | | `go.mod` |
| Five formats | Parsed and flattened; unsupported TOML rejected | `internal/parser` tests | pass | | `internal/parser` |
| Classification | missing / value / empty / type / expected-variance | `internal/diff` tests, `TestCompareExamples` | pass | 7 shown, 5 unexpected | `diff.go` |
| Rules | must_match beats expected_variance; ignore; globs | `internal/rules` tests | pass | | `rules.go` |
| Secret safety | No secret (key-pattern or URL password) in table, JSON or snapshot | report/snapshot tests, `TestURLPasswordsNeverPrintedOrStored`, clean-clone grep | pass | 0 matches | `report`, `snapshot` |
| Snapshot + drift | Round trip clean; changes and secret changes detected; ignore honoured | snapshot tests, `TestDriftHonoursIgnore` | pass | | `snapshot.go` |
| Exit codes | 0 / 1 / 2 | CLI tests | pass | | `main.go` |
| Static analysis / deps | vet, staticcheck, govulncheck clean | above | pass | | |

## Classification proposal

**Production-Ready for Its Documented Scope** — proposed, pending
independent verification and the CI matrix on this branch, with the
secret-detection limits stated in the README.
