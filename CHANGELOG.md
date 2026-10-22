# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [Unreleased]

### Security

- Passwords embedded in URL values are redacted in table/JSON output and
  stored in snapshots only as part of a hash (they were printed and stored
  verbatim when the key name was not a secret pattern).

### Fixed

- Module path is `github.com/hellpuffyt/configsync` (the documented
  `go install` failed with a module path mismatch).
- `ignore` rules apply to `snapshot` and `drift`.
- A repeated `--env` name is an error instead of silently replacing an
  environment.
- `.env` double-quoted escapes (`\\n` was decoded as backslash + newline).
- `.properties` `\uXXXX` escapes.
- JSON integers above 2^53 are compared exactly; YAML integers above
  MaxInt64 no longer wrap.
- TOML: escaped quotes no longer end a string when looking for comments;
  hex/octal/binary integers are numbers; inline tables, bare words and
  duplicate keys are errors instead of silent misreads.

### Changed

- README corrected: key names are not normalised across formats; the
  `*_HOST` matching example; unsalted snapshot hashes; exit code 2.
- CI: least-privilege `permissions`, actions pinned to commit SHAs.

## [0.1.0] - 2026-08-30

### Added

- Initial release of `configsync`.
- Parsers for `.env`, YAML, JSON, TOML, and Java-style `.properties`, all
  flattening nested structures to dotted paths for cross-format comparison.
- `compare` command: builds a full drift matrix across two or more
  environments and classifies every difference as `missing`, `value-differs`,
  `empty-in`, `type-differs`, or `expected-variance`.
- Rules file support: `expected_variance`, `must_match`, `ignore`, and
  `secret_patterns` glob patterns.
- Secret redaction: values for keys matching a secret pattern (built-in
  defaults plus rules-file patterns) are never printed in table or JSON
  output.
- `snapshot` and `drift` commands for tracking configuration drift over
  time, with secret values stored only as SHA-256 hashes.
- Table and JSON output formats, with a non-zero exit code when unexpected
  drift is found.
