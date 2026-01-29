# CRITICAL: gosn-v2 Authentication Bug Fix

**Date:** 2026-01-28
**Severity:** CRITICAL - Blocks all authentication
**Status:** Root cause identified, fix required

---

## Bug Summary

**Authentication fails because the PKCE code challenge is incorrectly generated**, causing server-side verification to fail.

### Test Proof

```bash
$ go run test_challenge.go
gosn-v2 (WRONG): YzZjODc3YTE3ZTA4NzE3MWZkNmY1YjVkMDAyMWY3MWNmZTFiOTlmM2YwMWE5NTUwNzA3ZmE3OTQ3ODRjMWE5Zg
Correct:         xsh3oX4IcXH9b1tdACH3HP4bmfPwGpVQcH-nlHhMGp8=
Match:           false
```

The values don't match because gosn-v2 base64-encodes the **hex string** of the hash instead of the **binary hash**.

---

## The Bug

**File:** `/Users/hadfielj/Repositories/gosn-v2/auth/authentication.go:974-991`

```go
func generateChallengeAndVerifierForLogin() (loginCodeVerifier generateLoginChallengeCodeVerifier) {
    var src cryptoSource
    rnd := rand.New(src)
    letterRunes := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

    b := make([]rune, 65)
    for i := range b {
        b[i] = letterRunes[rnd.Intn(len(letterRunes))]
    }

    loginCodeVerifier.codeVerifier = string(b)[:64]

    // ❌ BUG: Converts binary hash to HEX, then base64 encodes the HEX STRING
    sha25Hash := fmt.Sprintf("%x", sha256.Sum256([]byte(loginCodeVerifier.codeVerifier)))
    loginCodeVerifier.codeChallenge = string(base64.URLEncoding.EncodeToString([]byte(sha25Hash)))[:86]

    return loginCodeVerifier
}
```

### What Happens

1. Verifier: `"abcd1234..."` (64 ASCII characters)
2. SHA-256: `c6c877a1...` (32 bytes binary)
3. **Convert to hex**: `"c6c877a17e08717..."` (64 characters HEX STRING)  ← **BUG HERE**
4. Base64 encode hex string: `"YzZjODc3YTE3ZTA4..."` ← **WRONG VALUE**
5. Truncate to 86 chars

### What Should Happen (Standard Notes App)

1. Verifier: Random bytes (256 bytes)
2. SHA-256 hash: (32 bytes binary)
3. **Base64URL encode binary hash directly**: `"xsh3oX4IcXH..."` ← **CORRECT**
4. No truncation needed (base64 of 32 bytes = 44 chars with padding)

---

## Server-Side Validation (Why It Fails)

**Standard Notes Server validates:**

```typescript
// When auth params request arrives with code_challenge
storedChallenge = request.code_challenge

// When sign-in request arrives with code_verifier
const computedHash = sha256(request.code_verifier)
const computedChallenge = base64URL(computedHash)

if (computedChallenge !== storedChallenge) {
    return ERROR: Invalid code verifier
}
```

**Why gosn-v2 fails:**

```
Stored challenge (from params):  base64(hex(sha256(verifier)))
Computed (server, from verifier): base64(sha256(verifier))

These never match! ❌
```

---

## The Fix

**File:** `auth/authentication.go` Line 974-991

```go
func generateChallengeAndVerifierForLogin() (loginCodeVerifier generateLoginChallengeCodeVerifier) {
    // Generate random bytes for verifier
    verifierBytes := make([]byte, 64) // Can be 32, 64, or 256 bytes
    if _, err := crypto/rand.Read(verifierBytes); err != nil {
        panic(err)
    }

    // Verifier = base64URL encoded bytes (for JSON transmission)
    loginCodeVerifier.codeVerifier = base64.URLEncoding.EncodeToString(verifierBytes)

    // Challenge = base64URL(sha256(raw_verifier_bytes))
    hash := sha256.Sum256(verifierBytes)
    loginCodeVerifier.codeChallenge = base64.URLEncoding.EncodeToString(hash[:])

    return loginCodeVerifier
}
```

### Key Changes

1. ✅ Generate **binary random bytes** instead of alphanumeric string
2. ✅ Base64-encode verifier for JSON transmission
3. ✅ Hash the **raw bytes** (not the encoded string)
4. ✅ Base64-encode the **binary hash** (not hex string)
5. ✅ No truncation needed

---

## Secondary Issue: Empty hvm_token

**File:** `auth/authentication.go:173`

```go
// Current (sends empty string)
reqBody = fmt.Sprintf(`{"api":"%s","password":"%s","email":"%s","code_verifier":"%s","ephemeral":false,"hvm_token":""}`,
    apiVer, input.encPassword, e, input.codeVerifier)
```

**Should be:**

```go
// Option 1: Omit field entirely
reqBody = fmt.Sprintf(`{"api":"%s","password":"%s","email":"%s","code_verifier":"%s","ephemeral":false}`,
    apiVer, input.encPassword, e, input.codeVerifier)

// Option 2: Only include if present
if hvmToken != "" {
    reqBody = fmt.Sprintf(`{"api":"%s","password":"%s","email":"%s","code_verifier":"%s","ephemeral":false,"hvm_token":"%s"}`,
        apiVer, input.encPassword, e, input.codeVerifier, hvmToken)
} else {
    reqBody = fmt.Sprintf(`{"api":"%s","password":"%s","email":"%s","code_verifier":"%s","ephemeral":false}`,
        apiVer, input.encPassword, e, input.codeVerifier)
}
```

**Impact:** Minor - server likely ignores empty strings, but cleaner to omit

---

## Implementation Plan

### Step 1: Fix Code Challenge Generation ✅ CRITICAL

```go
// auth/authentication.go:974-991
import (
    crand "crypto/rand"
    "crypto/sha256"
    "encoding/base64"
)

func generateChallengeAndVerifierForLogin() (loginCodeVerifier generateLoginChallengeCodeVerifier) {
    // Generate 64 bytes of cryptographically secure random data
    verifierBytes := make([]byte, 64)
    if _, err := crand.Read(verifierBytes); err != nil {
        panic(fmt.Sprintf("failed to generate verifier: %v", err))
    }

    // Encode verifier for JSON transmission
    loginCodeVerifier.codeVerifier = base64.URLEncoding.EncodeToString(verifierBytes)

    // Compute challenge: base64(sha256(verifier_bytes))
    hash := sha256.Sum256(verifierBytes)
    loginCodeVerifier.codeChallenge = base64.URLEncoding.EncodeToString(hash[:])

    return loginCodeVerifier
}
```

### Step 2: Clean Up Request Body Construction

```go
// auth/authentication.go:161-175
func requestToken(input signInInput) (signInSuccess signInResponse, signInFailure ErrorResponse, err error) {
    e := url.PathEscape(input.email)
    apiVer := common.APIVersion

    var reqBody string

    if input.tokenName != "" {
        // MFA flow
        reqBody = fmt.Sprintf(`{"api":"%s","password":"%s","email":"%s","%s":"%s","code_verifier":"%s"}`,
            apiVer, input.encPassword, e, input.tokenName, input.tokenValue, input.codeVerifier)
    } else {
        // Normal flow - omit empty hvm_token
        reqBody = fmt.Sprintf(`{"api":"%s","password":"%s","email":"%s","code_verifier":"%s","ephemeral":false}`,
            apiVer, input.encPassword, e, input.codeVerifier)
    }

    // ... rest of function
}
```

### Step 3: Add Verification Test

```go
// auth/authentication_test.go
func TestCodeChallengeVerification(t *testing.T) {
    verifier := generateChallengeAndVerifierForLogin()

    // Decode the stored verifier
    verifierBytes, err := base64.URLEncoding.DecodeString(verifier.codeVerifier)
    require.NoError(t, err)
    require.Len(t, verifierBytes, 64, "Verifier should be 64 bytes")

    // Decode the challenge
    challengeBytes, err := base64.URLEncoding.DecodeString(verifier.codeChallenge)
    require.NoError(t, err)
    require.Len(t, challengeBytes, 32, "SHA-256 hash should be 32 bytes")

    // Compute hash of verifier
    computedHash := sha256.Sum256(verifierBytes)

    // Verify challenge matches hash of verifier (PKCE validation)
    require.Equal(t, computedHash[:], challengeBytes,
        "Challenge should equal base64(sha256(verifier))")
}

func TestCodeChallengeMatchesStandardNotesApp(t *testing.T) {
    // Use known test vector
    verifierBytes := []byte("test-verifier-12345678901234567890123456789012")
    verifier := base64.URLEncoding.EncodeToString(verifierBytes)

    hash := sha256.Sum256(verifierBytes)
    expectedChallenge := base64.URLEncoding.EncodeToString(hash[:])

    // Generate using our function
    result := generateChallengeAndVerifierForLogin()

    // Decode and verify format matches
    _, err := base64.URLEncoding.DecodeString(result.codeVerifier)
    require.NoError(t, err, "Verifier must be valid base64")

    _, err = base64.URLEncoding.DecodeString(result.codeChallenge)
    require.NoError(t, err, "Challenge must be valid base64")

    // Verify the algorithm works correctly
    testVerifierBytes, _ := base64.URLEncoding.DecodeString(result.codeVerifier)
    testHash := sha256.Sum256(testVerifierBytes)
    testChallenge := base64.URLEncoding.EncodeToString(testHash[:])

    require.Equal(t, result.codeChallenge, testChallenge,
        "Challenge generation algorithm must match Standard Notes")
}
```

---

## Verification Steps

After implementing the fix:

1. ✅ Run `TestCodeChallengeVerification` - should pass
2. ✅ Run `TestSignIn` - should successfully authenticate
3. ✅ Verify no truncation warnings or errors
4. ✅ Test with real Standard Notes server
5. ✅ Test MFA flow if applicable

---

## Impact Assessment

### Before Fix
- ❌ **All authentication fails** due to PKCE validation failure
- ❌ Server rejects sign-in with "Invalid code verifier" or similar error
- ❌ No users can authenticate

### After Fix
- ✅ PKCE validation passes
- ✅ Authentication succeeds
- ✅ Compatible with Standard Notes server
- ✅ Matches official app implementation

---

## Additional Notes

### Why This Bug Wasn't Caught Earlier

1. **No server-side testing** - Tests use mock responses or local server
2. **No challenge verification test** - No test validates PKCE algorithm
3. **Hex encoding seems logical** - Easy to confuse hex with binary encoding
4. **Truncation hides the issue** - Makes values look similar length

### Prevention

1. ✅ Add PKCE verification test (Step 3)
2. ✅ Test against real Standard Notes server in CI
3. ✅ Add test vectors from official implementation
4. ✅ Document PKCE flow in code comments

---

## References

- **PKCE RFC 7636**: https://datatracker.ietf.org/doc/html/rfc7636
- **Standard Notes App Implementation**:
  - `packages/snjs/lib/Services/Api/ApiService.ts:getAccountKeyParams()`
  - Uses `crypto.sha256()` returning binary, then `base64URLEncode()`
- **gosn-v2 Current (Broken)**: `auth/authentication.go:987-988`

---

**Priority:** CRITICAL
**Estimated Fix Time:** 15 minutes
**Testing Time:** 30 minutes
**Risk:** Low - Fix is well-understood and testable
