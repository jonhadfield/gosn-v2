# gosn-v2

[![Tests](https://github.com/jonhadfield/gosn-v2/actions/workflows/tests.yml/badge.svg?branch=master)](https://github.com/jonhadfield/gosn-v2/actions/workflows/tests.yml) [![Go Reference](https://pkg.go.dev/badge/github.com/jonhadfield/gosn-v2.svg)](https://pkg.go.dev/github.com/jonhadfield/gosn-v2) [![Go Report Card](https://goreportcard.com/badge/github.com/jonhadfield/gosn-v2)](https://goreportcard.com/report/github.com/jonhadfield/gosn-v2)

`gosn-v2` is a Go library for building Standard Notes clients. It wraps authentication, sync, encryption, and caching flows while letting you integrate with the official or a self-hosted Standard Notes server.

It is the client library behind [sn-cli](https://github.com/jonhadfield/sn-cli) and [sn-dotfiles](https://github.com/jonhadfield/sn-dotfiles).

## Contents

- [Status](#status)
- [Requirements](#requirements)
- [Installation](#installation)
- [Quick start](#quick-start)
- [Syncing through the local cache](#syncing-through-the-local-cache)
- [Things you need to know](#things-you-need-to-know)
- [Configuration](#configuration)
- [Supported item types](#supported-item-types)
- [Package layout](#package-layout)
- [Performance](#performance)
- [Testing](#testing)
- [Contributing](#contributing)
- [License](#license)

## Status

Pre-1.0 and **unversioned**: this repository has no release tags, so `go get`
resolves to a pseudo-version of `master`, for example:

```
github.com/jonhadfield/gosn-v2 v0.0.0-20260919171233-83a1c125b118
```

There is no semantic versioning, no changelog, and no deprecation policy. The
API has already been reorganised once without notice (the functions that used
to live in the root `gosn` package now live in `auth`, `session` and `items`).
Pin the exact pseudo-version in your `go.mod` and upgrade deliberately.

Standard Notes compatibility:

- API version `20240226` (`common.APIVersion`), which uses cookie-based sessions.
- The `004` encryption protocol (`common.DefaultSNVersion`) for items. Sign-in
  against `003` accounts is handled, but item encryption targets `004`.

## Requirements

- **Go 1.26 or later** — `go.mod` declares `go 1.26.0`. On an older toolchain
  Go will automatically download 1.26 to satisfy this; if you have set
  `GOTOOLCHAIN=local` (or work offline), the build fails instead, so upgrade
  your toolchain first.
- macOS, Linux, or Windows. All three are covered by CI.
- A Standard Notes account only if you want to run the live integration tests
  (see [Testing](#testing)).

## Installation

```bash
go get github.com/jonhadfield/gosn-v2@master
```

There is no importable package at the module root — import the packages you
need:

```go
import (
    "github.com/jonhadfield/gosn-v2/auth"    // sign in, register, refresh tokens
    "github.com/jonhadfield/gosn-v2/session" // session state and persistence
    "github.com/jonhadfield/gosn-v2/items"   // sync, item types, encrypt/decrypt
    "github.com/jonhadfield/gosn-v2/cache"   // local delta-sync database
    "github.com/jonhadfield/gosn-v2/common"  // HTTP client, constants, env config
)
```

## Quick start

Sign in, sync, and print the title of every note:

```go
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/jonhadfield/gosn-v2/auth"
	"github.com/jonhadfield/gosn-v2/common"
	"github.com/jonhadfield/gosn-v2/items"
	"github.com/jonhadfield/gosn-v2/session"
)

func main() {
	httpClient := common.NewHTTPClient()

	// 1. Authenticate. Use common.APIServer for the hosted service, or your own
	//    URL for a self-hosted server.
	so, err := auth.SignIn(auth.SignInInput{
		HTTPClient: httpClient,
		Email:      os.Getenv("SN_EMAIL"),
		Password:   os.Getenv("SN_PASSWORD"),
		APIServer:  common.APIServer,
	})
	if err != nil {
		log.Fatal(err)
	}

	// 2. auth.SignIn returns an auth.SignInResponseDataSession, but items.Sync
	//    needs a *session.Session. Build one from the sign-in result.
	sess := &session.Session{
		HTTPClient:        httpClient,
		Server:            common.APIServer,
		FilesServerUrl:    so.Session.FilesServerUrl,
		MasterKey:         so.Session.MasterKey,
		KeyParams:         so.Session.KeyParams,
		AccessToken:       so.Session.AccessToken,
		RefreshToken:      so.Session.RefreshToken,
		AccessExpiration:  so.Session.AccessExpiration,
		RefreshExpiration: so.Session.RefreshExpiration,
		ReadOnlyAccess:    so.Session.ReadOnlyAccess,
		PasswordNonce:     so.Session.PasswordNonce,
	}

	// 3. Sync. Paging is handled internally; the returned items are encrypted.
	out, err := items.Sync(items.SyncInput{Session: sess})
	if err != nil {
		log.Fatal(err)
	}

	// 4. Decrypt and parse into concrete types (items.Note, items.Tag, ...).
	decrypted, err := out.Items.DecryptAndParse(sess)
	if err != nil {
		log.Fatal(err)
	}

	for _, i := range decrypted {
		if note, ok := i.(*items.Note); ok {
			fmt.Println(note.Content.Title)
		}
	}
}
```

Step 2 is the part that surprises people: `auth` and `session` use separate
session types, and the fields have to be copied across. `cache.ImportSession`
does this for you, which is one reason most callers use the cache package.

## Syncing through the local cache

The `cache` package persists encrypted items to a local
[bbolt](https://github.com/etcd-io/bbolt) key-value store and syncs only deltas
on subsequent calls.

```go
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/jonhadfield/gosn-v2/auth"
	"github.com/jonhadfield/gosn-v2/cache"
	"github.com/jonhadfield/gosn-v2/common"
	"github.com/jonhadfield/gosn-v2/items"
)

func main() {
	so, err := auth.SignIn(auth.SignInInput{
		HTTPClient: common.NewHTTPClient(),
		Email:      os.Getenv("SN_EMAIL"),
		Password:   os.Getenv("SN_PASSWORD"),
		APIServer:  common.APIServer,
	})
	if err != nil {
		log.Fatal(err)
	}

	// ImportSession does the session conversion for you and opens the cache DB.
	// An empty path puts the database under the user's home directory.
	cs, err := cache.ImportSession(&so.Session, "")
	if err != nil {
		log.Fatal(err)
	}

	// Sync deltas into the local store. See "Sync suppression" below before
	// relying on this to pick up changes made elsewhere.
	cso, err := cache.Sync(cache.SyncInput{Session: cs})
	if err != nil {
		log.Fatal(err)
	}
	defer cso.DB.Close()

	var cached cache.Items
	if err = cso.DB.All(&cached); err != nil {
		log.Fatal(err)
	}

	parsed, err := cached.ToItems(cs)
	if err != nil {
		log.Fatal(err)
	}

	for _, i := range parsed {
		if note, ok := i.(*items.Note); ok {
			fmt.Println(note.Content.Title)
		}
	}
}
```

## Things you need to know

### Sync suppression: `cache.Sync` may not call the API at all

`cache.Sync` skips the network request entirely when there is nothing local to
push, a sync token is already stored, and that token is younger than
`common.MinSyncInterval` (**5 minutes**). It returns successfully, having
served you the existing cache contents.

This is the single most common source of confusion: edit a note in the web app,
call `cache.Sync` again, and you will not see the change until the interval has
elapsed. Set `AlwaysSync` when changes made by other clients must be visible
immediately:

```go
cso, err := cache.Sync(cache.SyncInput{Session: cs, AlwaysSync: true})
```

`items.Sync` has no such suppression — it always calls the API.

### Sessions are not safe for concurrent use

`session.Session` is **not** safe for use by multiple goroutines, and neither is
the `*retryablehttp.Client` inside it. Its cookie jar is not thread-safe (the
`20240226` API uses cookie-based sessions), and the token and expiry fields are
mutated in place on refresh.

Safe patterns:

1. Create a separate `Session` — with its own `common.NewHTTPClient()` — per goroutine.
2. Or serialise all access to a shared `Session` behind a mutex.

Never share a client with a cookie jar across goroutines. `items` serialises its
own sync requests internally, but that does not make the surrounding `Session`
safe to share. See the doc comments on `session.Session` and
`common.NewHTTPClient`, and `claudedocs/thread_safety.md`, for detail.

### Errors from the cache package are typed

`cache.Sync` classifies failures and returns a `*cache.SyncError`, which tells
you whether retrying is worthwhile and how long to wait:

```go
cso, err := cache.Sync(cache.SyncInput{Session: cs})
if err != nil {
	var syncErr *cache.SyncError
	if errors.As(err, &syncErr) {
		log.Printf("sync failed (type %d, retryable=%t, backoff=%dms): %s",
			syncErr.Type, syncErr.Retryable, syncErr.BackoffMs, syncErr.Message)
	}
	return err
}
```

The types are `SyncErrorRateLimit`, `SyncErrorValidation`,
`SyncErrorAuthentication`, `SyncErrorItemsKey`, `SyncErrorConflict`,
`SyncErrorNetwork` and `SyncErrorUnknown`.

If you would rather not implement the retry loop yourself, use
`cache.SyncWithRetry`, which applies exponential backoff for rate limiting and
gives up on non-retryable errors:

```go
cso, err := cache.SyncWithRetry(cache.SyncInput{Session: cs}, 3)
```

Rate limiting and transient network failures are already handled below this
layer too: `common.NewHTTPClient` returns a `retryablehttp` client configured
with `common.MaxRequestRetries` attempts and a custom backoff.

### Session persistence

You do not have to authenticate on every run. The `session` package can store a
session in the OS keyring and read it back:

- `session.GetSession` / `session.AddSession` / `session.RemoveSession` / `session.SessionExists`
- `session.SessionStatus` to report on a stored session
- `session.NewFileKeyring(path)` and `session.SetDefaultKeyring` for headless
  environments with no system keyring
- `(*session.Session).Valid()` and `(*session.Session).Refresh()` to check and
  renew an expiring access token

## Configuration

These environment variables are read by the library. All are optional.

| Variable | Read by | Effect |
| --- | --- | --- |
| `SN_REQUEST_TIMEOUT` | `common`, `items` | HTTP request timeout in seconds (default `30`). |
| `SN_RETRY_WAIT_MIN` | `common` | Minimum retry wait in seconds (default `2`). |
| `SN_RETRY_WAIT_MAX` | `common` | Maximum retry wait in seconds (default `5`). |
| `SN_SYNC_TIMEOUT` | `cache` | Overrides the sync timeout. A Go duration, e.g. `30s`, `5m`. |
| `SN_POST_SYNC_REQUEST_DELAY` | `items` | Milliseconds to sleep after each sync request. |
| `SN_POST_SIGN_IN_DELAY` | `auth` | Milliseconds to sleep after sign-in. |
| `SN_SCHEMA_VALIDATION` | `session` | `yes`, `true` or `1` enables JSON Schema validation of item content. |
| `HTTP_PROXY` | `common` | Proxy URL for all API requests. |

`SN_SERVER`, `SN_EMAIL`, `SN_PASSWORD` and `SN_SKIP_SESSION_TESTS` are used by
the test suite, not by the library itself — see [Testing](#testing).

Debug logging is enabled per call, not by environment variable: set the `Debug`
field on `auth.SignInInput`, `session.Session`, and so on.

## Supported item types

`items` has a concrete type and parser for each Standard Notes content type:

```
Note                      Tag
SN|Component              SN|Theme
SN|ItemsKey               SN|SmartTag
SN|File                   SN|FileSafe|FileMetadata
SN|FileSafe|Integration   SN|FileSafe|Credentials
SN|UserPreferences        SN|Privileges
SN|ExtensionRepo          Extension
SF|Extension              SF|MFA
SN|TrustedContact         SN|VaultListing
SN|KeySystemRootKey       SN|KeySystemItemsKey
```

The string constants are in `common` as `common.SNItemType*`.

## Package layout

- `auth/` — sign-in, registration, MFA, token refresh.
- `session/` — session state, validity and refresh, keyring persistence.
- `items/` — item models, sync, encryption and decryption, filtering.
- `crypto/` — key derivation, encryption, and signing helpers.
- `cache/` — local bbolt database for delta syncs.
- `common/` — HTTP client, API constants, environment configuration.
- `schemas/` — embedded JSON Schemas used when `SchemaValidation` is enabled.
- `log/` — debug logging helpers.

Note that `docs/` is a Go package holding a schema string, not a documentation
directory. The `docs/index.md` file it contains predates the split into the
packages above and describes functions that no longer exist (`gosn.GetItems`,
`gosn.PutItems`), so it is deliberately not linked from here. `cache/README.md`
is partly outdated for the same reason. The authoritative reference is
[pkg.go.dev](https://pkg.go.dev/github.com/jonhadfield/gosn-v2).

## Performance

The library is built around a few deliberate choices rather than headline
numbers:

- **Parallel decryption.** Items are decrypted by a worker pool sized to
  `runtime.NumCPU()`, capped at the number of items to decrypt.
- **Conditional sync.** `cache.Sync` skips the API call when there is nothing
  to push and the stored sync token is recent — see
  [Sync suppression](#sync-suppression-cachesync-may-not-call-the-api-at-all).
- **Adaptive batch sizing.** Sync requests are sized towards
  `common.TargetPayloadSize` (256 KB), between `common.MinPageSize` (50) and
  `common.MaxPageSize` (500) items, and shrink automatically on retry.
- **Bounded connection pool.** The shared transport keeps a small number of
  idle connections (`common.MaxIdleConnections`) rather than the Go default of
  100, trading peak concurrency for a much smaller idle footprint.
- **Buffer reuse.** Sync request encoding reuses pooled buffers to reduce GC
  pressure.

If you need figures for your own workload, measure it: `SN_SYNC_TIMEOUT`,
`SN_REQUEST_TIMEOUT` and `items.SyncInput.PageSize` are the tuning knobs worth
starting from.

## Testing

Most of the test suite talks to a real Standard Notes server. Two modes:

```bash
# Offline. Unit tests only — this is what CI runs.
SN_SKIP_SESSION_TESTS=true go test ./...
```

```bash
# Full suite, including live integration tests.
export SN_EMAIL='you@example.com'
export SN_PASSWORD='...'
export SN_SERVER='https://api.standardnotes.com'
go test ./...
```

> **Use a throwaway account.** The integration tests create, modify and delete
> items, and some exercise bulk-deletion paths. Never point them at an account
> whose notes you care about.

Without `SN_SKIP_SESSION_TESTS=true`, `TestMain` signs in during setup and
calls `log.Fatal` if the credentials are missing or wrong, so a bare
`go test ./...` will fail rather than skip.

Other useful commands:

- `go build ./...` verifies every package compiles.
- `make test` aggregates coverage into `coverage.txt`; `make cover` opens the HTML report.
- `make fmt` applies `gofmt` and `goimports` to all Go files.
- `make lint` runs `golangci-lint`; `make critic` adds `gocritic` analysis.
- `make ci` mirrors the lint + test pipeline.

## Contributing

Review the [Repository Guidelines](AGENTS.md) before opening a pull request.
They cover project structure, testing expectations, and the commit/PR workflow.

## License

This project is distributed under the [MIT License](LICENSE).
