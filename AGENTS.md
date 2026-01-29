# Repository Guidelines

## Project Structure & Module Organization
- `auth/`, `session/`, and `items/` wrap Standard Notes authentication, session state, and item models.
- `crypto/` centralizes key derivation and encryption helpers; `common/` holds shared utilities.
- `cache/` persists encrypted sync snapshots; tests live alongside code as `*_test.go` files (see `helpers_test.go`).
- CLI helpers reside in `bin/`; reference docs under `docs/`; schemas and fixtures live in `schemas/` and `test.json`.
- Use `go.mod` to manage modules; avoid editing generated assets in `cache.test` or `coverage.txt` by hand.

## Build, Test, and Development Commands
- `go build ./...` verifies the library compiles across modules.
- `go test ./...` runs unit tests; set `SN_SKIP_SESSION_TESTS=true` to skip live-session checks.
- `make test` executes the test suite with coverage aggregation into `coverage.txt`.
- `make fmt` applies `gofmt` and `goimports` to every Go file.
- `make lint` runs `golangci-lint` with the repo’s tuned rule-set; `make critic` adds `gocritic` checks when you need deeper static analysis.
- Run `make ci` locally before opening a PR to mirror the lint + test pipeline.

## Coding Style & Naming Conventions
- Follow idiomatic Go: tabs for indentation, `camelCase` for locals, exported APIs in `PascalCase` with doc comments.
- Prefer small, composable functions; reference existing packages for patterns on encrypt/decrypt flows.
- Let `make fmt` handle formatting; never commit unformatted code. Keep error messages lowercase and without trailing punctuation.

## Testing Guidelines
- Add table-driven tests near the code they validate, naming files `*_test.go` and functions `TestXxx` / `BenchmarkXxx`.
- Integration tests that hit a live Standard Notes server must guard with `testing.Short()` or the `SN_SKIP_SESSION_TESTS` flag.
- Update or generate fixtures under `schemas/` or `cache/` only when behaviour changes; document new datasets in test descriptions.
- Inspect `coverage.txt` after `make test`; target ≥80% coverage when touching core crypto/session logic.

## Commit & Pull Request Guidelines
- Use concise, present-tense commit summaries (`improve error handling.`) and separate logical changes into distinct commits.
- PRs should describe scope, list key commands run (e.g., `make lint`, `make test`), and link relevant issues.
- Add screenshots or curl transcripts if you modify observable client behaviour; note security implications for auth or crypto changes.
- Request at least one reviewer familiar with the touched package; re-run tests after addressing feedback before merging.

## Security & Configuration Tips
- Never commit real Standard Notes credentials; use the encrypted fixtures (`testuser-encrypted-backup.txt`) for reproducible tests.
- Store experimental caches outside the repo; keep local overrides in `.env` or shell exports and out of version control.
