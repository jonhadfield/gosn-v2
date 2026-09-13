# GitGuardian exclusion plan

Three layers, applied at different points. Layer 1 is committed to this repo;
layer 2 has to be entered in the GitGuardian dashboard by hand; layer 3 is
optional and only affects local/CI runs of `ggshield`.

## Layer 1 — inline `// ggignore` tags (done, in-repo)

Tagged lines carrying test key material or fixture passwords:

| File | Lines | What |
|---|---|---|
| `auth/authentication_test.go` | 34, 202 | `"secretsanta"` fixture password |
| `items/gosn_test.go` | 37 | `"secretsanta"` fixture password |
| `crypto/encryption_test.go` | 26, 28, 31, 32, 169, 176, 180 | derived master key, server password, raw AES keys |
| `items/items_test.go` | 1384, 1390, 1410, 1416, 1429 | items keys and item keys |
| `session/parse_test.go` | 10, 15 | serialised session string, master key |

Deliberately **not** tagged: `nonce` and `authData` locals. A nonce ships in the
clear alongside its ciphertext and `authData` is base64 JSON of public key
params, so neither is credential material. Tagging them also makes gofmt align
the trailing comment against the 300-character `authData` line, which is
unreadable. If GitGuardian does raise them, the layer-2 path exclusions cover it.

## Layer 2 — workspace exclusion rules (paste into the dashboard)

Settings -> Secrets detection -> Exclusions. Scope each of these to the
`jonhadfield/gosn-v2` repository rather than the whole workspace. These apply
retroactively, so existing incidents drop off the table once saved.

### Filepath exclusions

```
crypto/encryption_test.go
items/items_test.go
test.json
testuser-encrypted-backup.txt
claudedocs/**
**/*_TEST_RESULTS.md
**/*_ANALYSIS.md
```

Deliberately **not** `**/*_test.go`. A blanket test glob would silently cover
every test file added in future, including ones that come to hold a real
credential by mistake. That is not hypothetical: while writing
`common/redact_test.go` a real session token lifted from a CI log was very
nearly committed as a fixture, and GitGuardian is what caught it. A
`**/*_test.go` exclusion would have hidden it.

Only two test files are listed, and only because inline tags cannot reach all
of their content: both hold `nonce` and `authData` locals sitting next to
300-character lines, where a trailing `// ggignore` makes gofmt pad the comment
out by ~250 spaces. Every other test file with fixture key material is covered
by inline tags instead:

| File | Covered by |
|---|---|
| `auth/authentication_test.go` | inline tags |
| `items/gosn_test.go` | inline tags |
| `session/parse_test.go` | inline tags |
| `auth/code_challenge_test.go` | nothing needed - two comment lines showing an example PKCE challenge |
| `crypto/encryption_extra_test.go` | nothing needed - `00112233aabbccdd...` style synthetic patterns |

Prefer adding a `// ggignore` tag to excluding a new path. The tags are proven
working against this repository's scanning path, and they fail safe: a new
credential in an untagged line still gets flagged.

Notes on the non-obvious entries:

- `test.json` and `testuser-encrypted-backup.txt` sit at the repo root and match
  no `*test*` directory glob, so they need naming outright. Both hold real
  004-format encrypted payloads for a throwaway account.
- `claudedocs/**` covers five analysis write-ups that quote master keys and
  session strings inline: `credential_test_results.md`,
  `authentication_analysis.md`, `authentication_fix_complete.md`,
  `authentication_investigation_summary.md`, `auth_request_comparison.md`.

### Secret pattern exclusions (regex)

```
^004:[0-9a-f]{48}:
^ramea-[0-9a-f]+$
```

The first matches the Standard Notes 004 ciphertext prefix, which is what makes
the encrypted fixtures look like key material. The second matches the generated
local-test usernames.

## Layer 3 — `.gitguardian.yaml` (optional, CLI only)

Only worth adding if you start running `ggshield` in pre-commit or CI. It will
**not** clear dashboard incidents — GitGuardian's docs are explicit that
"ggshield does not share its ignored secrets with the dashboard".

```yaml
version: 2
secret:
  ignored_paths:
    - 'crypto/encryption_test.go'
    - 'items/items_test.go'
    - 'test.json'
    - 'testuser-encrypted-backup.txt'
    - 'claudedocs/**'
```

## Live credential — removed rather than suppressed

`cache/consecutive_sync_test.go` and `cache/consecutive_sync_isolated_test.go`
used to hardcode a fallback for a live account on lessknown.co.uk when
`SN_EMAIL` / `SN_PASSWORD` were unset. That was a real credential, not a
fixture, so it was deleted rather than tagged or excluded.

Both files now go through a shared helper in `cache/consecutive_sync_test.go`:

```go
func testCredentials(tb testing.TB) (email, password, server string)
```

It reads `SN_EMAIL`, `SN_PASSWORD` and `SN_SERVER`, calls `tb.Skipf` when the
first two are unset, and defaults the server to `common.APIServer`. `tb.Helper()`
means the skip is reported at the caller's line. Taking `testing.TB` lets both
`TestConsecutiveCacheSync` and `BenchmarkConsecutiveSync` share it.

The account itself should still be considered exposed — it was in git history
before this change, so rotate or delete it regardless.
