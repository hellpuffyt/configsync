# Security policy

## Reporting a vulnerability

Please report security issues — especially any case where a secret value
reaches output or a snapshot — privately through GitHub's advisory form:
https://github.com/hellpuffyt/configsync/security/advisories/new

Include a minimal reproduction that uses fake values. Please do not open a
public issue for a suspected vulnerability.

## Security model

- configsync only reads the files you pass; it never writes to them and
  makes no network connections. It writes a snapshot only to the `--out`
  path you give.
- Values of keys matching a secret pattern (built-in defaults plus
  `secret_patterns`) are never printed or stored; passwords inside URL
  values are shown as `xxxxx`.
- Snapshots keep secret values as unsalted SHA-256 hashes. That detects a
  changed secret without storing it, but a weak secret can be guessed from
  its hash: treat snapshot files as sensitive.
- Secrets stored under key names that match no pattern are not detected.

Only the latest release receives fixes.
