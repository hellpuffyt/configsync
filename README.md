# configsync

Detect configuration drift between environments, and know which values
differ, which are missing, and which are merely empty -- because those
three have completely different consequences and a plain diff conflates
them all.

## What

`configsync` compares two or more environments' configuration -- dev,
staging, prod, or any set you name -- across `.env`, YAML, JSON, TOML, and
Java-style `.properties` files. It flattens nested structures to dotted
paths so a YAML file and a flat `.env` file can be compared key-for-key,
classifies every difference it finds, and lets you declare which keys are
*expected* to differ (URLs, hostnames, credentials) versus which must be
identical everywhere (feature flags, timeouts).

## Why

"It works in staging" is usually a config difference nobody can see.
Comparing two files by eye, or with `diff`, misses the distinction between:

- a key that's **present in both but holds a different value**,
- a key that's **missing entirely** from one environment, and
- a key that's **present but empty** in one environment.

Those are three different bugs with three different fixes, and a line-based
diff shows them all as the same kind of noise. `configsync` separates them,
adds a fourth category for type mismatches (`"true"` the string vs. `true`
the boolean is a real source of production bugs), and knows -- via a rules
file -- which differences you already expect and which ones should fail a
build.

## How it differs from a plain diff

| | `diff a.env b.env` | `configsync compare` |
|---|---|---|
| Formats | Same format only, line-for-line | `.env`, YAML, JSON, TOML, `.properties`, compared cross-format |
| Nesting | N/A | Nested YAML/JSON/TOML flattened to dotted paths |
| Missing vs. empty vs. different | All look like a changed line | Classified separately: `missing`, `empty-in`, `value-differs`, `type-differs` |
| Expected variance | No concept of it -- every difference is noise | Rules file suppresses differences you've declared expected |
| N environments | Two files at a time | Any number of environments in one matrix |
| Secrets | Prints the full line, value included | Never prints a secret value, in any output |
| Drift over time | No memory between runs | `snapshot` + `drift` track what changed since a saved baseline |

## Features

- **Five formats**, auto-detected by extension: `.env`, `.yaml`/`.yml`,
  `.json`, `.toml`, `.properties`.
- **Flattening**: nested maps become dotted paths (`database.host`), arrays
  become indexed paths (`hosts.0`), so structurally different formats can
  still be compared key-for-key.
- **Multi-environment matrix**: compare 2, 3, or more environments in a
  single run.
- **Five-way classification** for every key:
  - `missing` -- absent in at least one environment.
  - `value-differs` -- present everywhere, values differ.
  - `empty-in` -- present everywhere, empty in at least one.
  - `type-differs` -- present everywhere with the same string value but a
    different native type (a YAML/JSON/TOML-only distinction: `"true"` vs.
    `true`, or `"8080"` vs. `8080`).
  - `expected-variance` -- differs, but matches a rule saying it should.
- **Rules file**: `expected_variance`, `must_match`, `ignore`, and
  `secret_patterns` glob lists. A `must_match` rule always wins over an
  `expected_variance` rule on the same key.
- **Secret redaction by default**: values for keys matching a secret
  pattern are never printed, in table output, JSON output, or snapshots --
  only the fact that they differ.
- **Drift over time**: `snapshot` saves a point-in-time baseline; `drift`
  compares a later run against it and reports newly diverged keys, with
  secret values compared by hash so the baseline file itself is never a
  leak.
- **CI-friendly**: table or JSON output, non-zero exit code when unexpected
  drift is found.

## Formats

| Format | Extension | Notes |
|---|---|---|
| dotenv | `.env` | `KEY=value`, `export KEY=value`, single/double-quoted values with escapes, `#` comments |
| YAML | `.yaml`, `.yml` | Full nested maps and sequences, native types preserved |
| JSON | `.json` | Nested objects and arrays, native types preserved |
| TOML | `.toml` | `[table]` headers, `[[array.of.tables]]`, dotted keys, inline arrays of scalars |
| Java properties | `.properties` | `key=value`, `key: value`, `key value`, `\` line continuation, `#`/`!` comments |

All formats flatten to the same internal representation
(`internal/model.Config`, a `map[string]Value`), which is what makes
cross-format comparison possible: an `.env` file's `DATABASE_URL` and a
YAML file's `database: {url: ...}` are just different ways of writing the
same flattened key, once you dot-path the nesting away.

## Architecture

```
cmd/configsync/        CLI entry point and subcommands
internal/model/        Config, Value, Environment -- the shared flattened representation
internal/parser/       One file per format, all producing model.Config
internal/rules/        Glob-pattern rule matching (expected_variance, must_match, ignore, secrets)
internal/diff/         Compare(): builds the drift matrix and classifies every key
internal/report/       Table and JSON rendering, with secret redaction
internal/snapshot/     Point-in-time snapshots and drift-since-snapshot comparison
```

The pipeline is: parse each environment file into a `model.Config` -> take
the union of keys -> classify each key against every environment's cells ->
apply rules to reclassify or escalate -> render.

## Installation

```
go install github.com/hellpuffyt/configsync/cmd/configsync@latest
```

Or build from source:

```
git clone https://github.com/hellpuffyt/configsync.git
cd configsync
go build -o configsync ./cmd/configsync
```

## Usage

### Compare environments

```
configsync compare \
  --env dev=examples/dev.env \
  --env staging=examples/staging.yaml \
  --env prod=examples/prod.json \
  --rules examples/rules.yaml
```

```
KEY                dev                                      staging                                       prod                                       STATUS
API_KEY            [REDACTED]                               [REDACTED]                                    [REDACTED]                                 value-differs
DATABASE_URL       postgres://dev-db.internal:5432/widgets   postgres://staging-db.internal:5432/widgets   postgres://prod-db.internal:5432/widgets   expected-variance
DB_HOST            dev-db.internal                           staging-db.internal                           prod-db.internal                           expected-variance
DB_PASSWORD        [REDACTED]                                [REDACTED]                                    [REDACTED]                                 value-differs
FEATURE_NEW_UI     true                                      true                                          true                                       type-differs (must-match)
LOG_LEVEL          debug                                     info                                          warn                                       value-differs
STAGING_ONLY_FLAG  <missing>                                 true                                          <missing>                                  missing

7 key(s) shown, 5 unexpected drift
```

By default only differences are shown; pass `--all` to see the full matrix
including matching keys. Exit code is non-zero whenever unexpected drift is
found; pass `--exit-zero` to always exit 0 (useful for a report-only run).

### JSON output

```
configsync compare --env dev=dev.env --env prod=prod.yaml --format json
```

Produces a document with a `summary` (counts per classification) and an
`entries` array, one object per key, each environment's cell marked
`redacted: true` instead of carrying a `value` when the key is a secret.

### Save and compare against a snapshot

```
configsync snapshot --env dev=dev.env --env staging=staging.yaml --env prod=prod.json --out baseline.json

# ... time passes, configs change ...

configsync drift --snapshot baseline.json --env dev=dev.env --env staging=staging.yaml --env prod=prod.json
```

`drift` reports every key/environment pair whose presence or value changed
since the snapshot was taken. Secret values are stored in the snapshot only
as a `sha256:...` hash, so a changed secret is detected and reported
without the snapshot file ever holding the plaintext.

## Rules file

```yaml
expected_variance:
  - "*_URL"
  - "*_HOST"
  - "*_PORT"

must_match:
  - "FEATURE_*"
  - "*_TIMEOUT_MS"

ignore:
  - "GENERATED_AT"

secret_patterns:
  - "*_DSN"
  - "*SIGNING*"
```

- Patterns use `*` as the only wildcard, matched case-insensitively against
  both a key's full dotted path and its final segment -- so `*_HOST`
  matches both `DB_HOST` and `database.credentials.host`.
- `must_match` takes precedence over `expected_variance` when a key matches
  both: a feature flag named `FEATURE_URL_OVERRIDE` that also looks like a
  URL is still held to "must be identical everywhere."
- `expected_variance` does **not** suppress `type-differs`: a key changing
  native type across environments is treated as a real bug regardless of
  which rule it matches, because `"true"` vs. `true` is very rarely
  intentional.
- See `examples/rules.yaml` for a complete, commented example.

## Secret safety

configsync reads real configuration, potentially production configuration.
Dumping a secret value into a CI log or a snapshot file committed to a
repository would be exactly the leak this tool exists to prevent, so:

- A built-in set of secret patterns (`*PASSWORD*`, `*SECRET*`, `*TOKEN*`,
  `*_KEY`, `*CREDENTIAL*`, `*PRIVATE*`, and more) is always active, even
  with no rules file.
- A key matching any secret pattern **never** has its value printed, in
  table output, JSON output, or a saved snapshot -- only whether it is
  present, and whether it differs.
- Snapshots store a SHA-256 hash for secret values instead of the value
  itself, which is enough to detect that a secret changed without ever
  persisting the plaintext to disk.
- This is the default behavior, not an opt-in flag. Add more patterns via
  `secret_patterns` in your rules file; you cannot turn the built-in
  defaults off.

## Examples

The `examples/` directory contains a working three-environment setup:

- `dev.env` -- dotenv format
- `staging.yaml` -- YAML format
- `prod.json` -- JSON format
- `rules.yaml` -- expected-variance, must-match, ignore, and secret-pattern
  rules

Together they exercise every classification: a matching key (`APP_NAME`), a
plain unrules value difference (`LOG_LEVEL`), two expected-variance
suppressions (`DATABASE_URL`, `DB_HOST`), a must-match type violation
(`FEATURE_NEW_UI`), a missing key (`STAGING_ONLY_FLAG`), and two redacted
secret differences (`DB_PASSWORD`, `API_KEY`). Run the command under
[Usage](#usage) to see it end to end.

## Testing

```
go test ./...
```

The suite covers each format's parser (quoting, comments, nested
flattening, arrays/tables), every difference classification, rule
interactions (`expected_variance` suppressing a difference, `must_match`
escalating one even when `expected_variance` also matches), three-way and
N-way comparison, snapshot save/load/diff, and -- the most important tests
in the suite -- that a secret value never appears in table output, JSON
output, or a saved snapshot file, under any circumstance.

## Security

configsync is a read-only comparison tool: it never writes back to your
configuration files and never transmits configuration data anywhere. The
one thing it must never do is print a secret value, which is covered above
under [Secret safety](#secret-safety) and enforced by dedicated tests. If
you find a case where a secret value leaks into output, please open an
issue with a minimal reproduction (using fake values).

## License

MIT. See [LICENSE](LICENSE).
