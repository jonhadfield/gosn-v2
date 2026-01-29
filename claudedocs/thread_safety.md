# Thread Safety in gosn-v2

## Overview

The gosn-v2 library handles Standard Notes API authentication and synchronization. This document outlines critical thread-safety considerations when using the library.

## Critical Thread-Safety Issues

### 1. Cookie Jar (http.CookieJar) - NOT THREAD-SAFE ⚠️

**Problem**: Go's `http.CookieJar` implementation is **not thread-safe** for concurrent requests.

**Impact**: Cookie-based authentication (API version 20240226) uses cookies for session management. Concurrent requests using the same HTTP client can cause race conditions.

**Location**: Created in `common/common.go:82`:
```go
jar, err := cookiejar.New(nil)
if err != nil {
    log.Fatalf("Failed to create cookie jar: %v\n", err)
}
c.HTTPClient.Jar = jar
```

**Mitigation**: The library uses `syncMutex` in `items/items.go` to serialize sync requests:
```go
// syncMutex serializes sync requests to prevent race conditions with:
// 1. Cookie jar concurrent access (cookiejar is not thread-safe)
// 2. HTTP connection pool reuse conflicts
// 3. Response body handling races
var syncMutex sync.Mutex
```

**Best Practices**:
- ✅ **DO**: Use separate Session instances for concurrent operations
- ✅ **DO**: Serialize requests to the same session using mutex
- ❌ **DON'T**: Share a single Session across multiple goroutines without synchronization
- ❌ **DON'T**: Assume HTTP client is safe for concurrent use with cookie jar

### 2. Session Sharing Across Goroutines

**Problem**: Session objects contain:
- HTTP client with shared cookie jar (not thread-safe)
- Mutable state (tokens, expiration times)
- Connection pool resources

**Safe Usage Pattern**:
```go
// SAFE: Each goroutine gets its own session
func processItemsConcurrently(email, password string) {
    var wg sync.WaitGroup

    for i := 0; i < 5; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()

            // Create separate session for each goroutine
            session, _, err := cache.GetSession(nil, false, "", server, false)
            if err != nil {
                log.Printf("Failed to get session: %v", err)
                return
            }

            // Process items with dedicated session
            syncOutput, err := items.Sync(items.SyncInput{
                Session: &session.Gosn(),
            })
            // ... handle result
        }()
    }

    wg.Wait()
}
```

**Unsafe Usage Pattern**:
```go
// UNSAFE: Sharing session across goroutines
func processConcurrentlyUnsafe(session *cache.Session) {
    var wg sync.WaitGroup

    for i := 0; i < 5; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()

            // RACE CONDITION: Multiple goroutines use same session
            syncOutput, err := items.Sync(items.SyncInput{
                Session: &session.Gosn(),
            })
            // Cookie jar corruption possible!
        }()
    }

    wg.Wait()
}
```

### 3. HTTP Client and Connection Pool

**Problem**: The HTTP client maintains:
- Connection pool (generally thread-safe)
- Cookie jar (NOT thread-safe)
- Request/response lifecycle state

**Current Implementation**:
The library creates HTTP clients with shared resources. Connection pooling is thread-safe, but cookie jar access is not.

**Location**: `common/common.go` in `NewHTTPClient()`:
```go
t.MaxIdleConns = MaxIdleConnections
t.MaxConnsPerHost = MaxIdleConnections
t.MaxIdleConnsPerHost = MaxIdleConnections
```

**Best Practice**:
- Reuse HTTP clients for connection pooling efficiency
- But ensure cookie jar operations are synchronized

## Authentication-Specific Considerations

### Cookie-Based Authentication (API 20240226)

**Mechanism**:
- Access and refresh tokens stored as HTTP-only cookies
- Tokens also returned in response body with "2:" prefix
- Requires BOTH Cookie header AND Authorization header

**Thread-Safety Impact**:
```go
// Cookie extraction happens in authentication.go:237-272
// Multiple concurrent sign-ins could corrupt cookie jar state
```

**Safe Pattern**:
```go
// Serialize authentication operations per user
var authMutex sync.Mutex

func signIn(email, password string) (*auth.SignInOutput, error) {
    authMutex.Lock()
    defer authMutex.Unlock()

    return auth.SignIn(auth.SignInInput{
        Email:     email,
        Password:  password,
        APIServer: server,
    })
}
```

### Session Refresh

**Critical Section**: `session/session.go:631-686` in `Refresh()`:

The refresh operation:
1. Reads current refresh token (potential race)
2. Makes HTTP request (cookie jar access)
3. Updates session state (potential race)

**Mitigation**: Already protected by `syncMutex` when called through Sync operations.

**Manual Refresh Safety**:
```go
// If calling Refresh() directly, protect with mutex
var sessionMutex sync.Mutex

func refreshSession(s *session.Session) error {
    sessionMutex.Lock()
    defer sessionMutex.Unlock()

    return s.Refresh()
}
```

## Testing for Race Conditions

**Run tests with race detector**:
```bash
go test -race ./...
```

**Known Safe Operations**:
- ✅ Read-only session fields (Server, Debug, SchemaValidation)
- ✅ Independent sessions (no shared resources)

**Known Unsafe Operations**:
- ❌ Concurrent Sync() calls on same session
- ❌ Concurrent Refresh() calls on same session
- ❌ Sharing session.HTTPClient across goroutines with cookie jar

## Recommendations

### For Library Users

1. **One Session Per Goroutine**: Create separate sessions for concurrent operations
2. **Serialize Shared Sessions**: Use mutexes if you must share a session
3. **Run Race Detector**: Test your code with `-race` flag during development
4. **Monitor Cookie State**: Be aware that cookie jar corruption causes silent failures

### For Library Developers

1. **Document Thread-Safety**: Clearly mark which functions are thread-safe
2. **Consider Session Cloning**: Provide `Session.Clone()` for safe concurrent use
3. **Mutex Documentation**: Keep syncMutex comments up-to-date
4. **Test Coverage**: Add race condition tests for critical paths

## Future Improvements

### Potential Solutions

1. **Thread-Safe Cookie Jar Wrapper**:
```go
type SafeCookieJar struct {
    mu  sync.RWMutex
    jar http.CookieJar
}

func (s *SafeCookieJar) SetCookies(u *url.URL, cookies []*http.Cookie) {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.jar.SetCookies(u, cookies)
}

func (s *SafeCookieJar) Cookies(u *url.URL) []*http.Cookie {
    s.mu.RLock()
    defer s.mu.RUnlock()
    return s.jar.Cookies(u)
}
```

2. **Session Cloning**:
```go
func (s *Session) Clone() *Session {
    // Create new HTTP client with separate cookie jar
    // Copy immutable fields
    // Return independent session instance
}
```

3. **Explicit Concurrency Support**:
```go
type ConcurrentSession struct {
    mu sync.RWMutex
    session *Session
}

func (cs *ConcurrentSession) Sync(...) {
    cs.mu.Lock()
    defer cs.mu.Unlock()
    // Safe synchronized access
}
```

## Related Files

- `common/common.go`: HTTP client and cookie jar creation
- `items/items.go`: Sync operations with syncMutex protection
- `session/session.go`: Session management and refresh
- `auth/authentication.go`: Cookie extraction and authentication
- `cache/session.go`: Session persistence and retrieval

## References

- [Go http.CookieJar Documentation](https://pkg.go.dev/net/http#CookieJar)
- [Go Race Detector](https://go.dev/doc/articles/race_detector)
- Standard Notes API v20240226 specification
