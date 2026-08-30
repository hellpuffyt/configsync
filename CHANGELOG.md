# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

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
