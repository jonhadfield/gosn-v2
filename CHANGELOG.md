# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html)
starting with `v0.1.0`. Earlier history was consumed only via Go pseudo-versions of
`master`.

## [0.1.0] - 2026-10-10

First tagged release of the post-split library (`auth`, `session`, `items`,
`cache`, `crypto`, `common`).

### Added

- Inline `// ggignore` labels on remaining test fixtures and a `PasswordNonce`
  false positive so GitGuardian treats them as non-leaks.
- `TestReadExportJSONFixtures` covering parse of `test.json` and
  `testuser-encrypted-backup.txt` while `Session.Export` / `Session.Import`
  remain commented out since the package split.

### Fixed

- Session key derivation for keys of 32 bytes or longer (no longer pads a full
  key as if it were a password).
- Rate-limit backoff test no longer depends on wall-clock sleep duration.
- Sync token reset no longer prints to stdout.

### Changed

- Apps can use separate keyring session slots.
- CI test concurrency is keyed by branch.
- Documentation notes tagged releases; install examples pin `v0.1.0`.

[0.1.0]: https://github.com/jonhadfield/gosn-v2/releases/tag/v0.1.0
