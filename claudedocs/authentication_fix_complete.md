# Authentication Fix - Complete Summary

## ✅ Status: FIXED AND VERIFIED

Authentication with Standard Notes API now works correctly with Protocol 004 accounts.

## The Bug

The gosn-v2 PKCE (Proof Key for Code Exchange) implementation was generating the code_challenge incorrectly.

### What PKCE Should Be (RFC 7636)
```
code_challenge = BASE64URL(SHA256(code_verifier))
```

### What Standard Notes Actually Uses
```
code_challenge = BASE64URL(HEX(SHA256(code_verifier_string)))
```

Standard Notes adds an extra **hex encoding** step and hashes the **string** (not the original bytes).

## The Fix

### File: `auth/authentication.go`

**Before** (Incorrect - RFC 7636 compliant but not Standard Notes compliant):
```go
hash := sha256.Sum256(verifierBytes)  // Hashing the BYTES
loginCodeVerifier.codeChallenge = base64.URLEncoding.EncodeToString(hash[:])
```

**After** (Correct - Standard Notes compliant):
```go
// 1. Hash the code_verifier STRING (not the original bytes)
hash := sha256.Sum256([]byte(loginCodeVerifier.codeVerifier))

// 2. Hex encode the hash (32 bytes → 64 hex characters)
hashHex := make([]byte, hex.EncodedLen(len(hash)))
hex.Encode(hashHex, hash[:])

// 3. Base64-url encode without padding
loginCodeVerifier.codeChallenge = base64.RawURLEncoding.EncodeToString(hashHex)
```

### Added Import
```go
import "encoding/hex"
```

## Verification

### Test Credentials
- **Email**: gosn-v2-20260128@lessknown.co.uk
- **Password**: gosn-v2-20260128@lessknown.co.uk
- **Protocol**: 004

### Authentication Result
```
✅ AUTHENTICATION SUCCESSFUL!
User UUID: d6439d9e-465c-49a7-8b2c-e1d95b0b5cc1
Email: gosn-v2-20260128@lessknown.co.uk
Protocol Version: 004
```

### Unit Tests
All PKCE tests updated and passing:
- ✅ `TestCodeChallengeVerification` - Verifies the algorithm
- ✅ `TestCodeChallengeAlgorithm` - Verifies hex encoding
- ✅ `TestCodeChallengeUniqueness` - Verifies randomness
- ✅ `TestCodeChallengeLengths` - Verifies format
- ✅ `TestCodeVerifierServerValidation` - Simulates server validation
- ✅ `TestStandardNotesPKCEFormat` - Verifies exact Standard Notes format

## Technical Details

### Standard Notes PKCE Flow

1. **Client generates PKCE pair**:
   - `verifier_bytes` = 64 random bytes
   - `code_verifier` = base64-url-encode(`verifier_bytes`)
   - `code_challenge` = base64-url-encode(hex(SHA-256(`code_verifier`)))

2. **Client requests auth params** (`/v2/login-params`):
   ```json
   {
     "email": "user@example.com",
     "code_challenge": "MDQw...5MTIy",
     "api": "20240226"
   }
   ```

3. **Server stores** `code_challenge` in PKCE repository

4. **Client signs in** (`/v2/login`):
   ```json
   {
     "email": "user@example.com",
     "password": "server_password_from_argon2",
     "code_verifier": "5mY6DrjjhaWl...",
     "api": "20240226",
     "ephemeral": false
   }
   ```

5. **Server validates**:
   ```typescript
   const codeChallenge = base64URLEncode(hex(sha256(code_verifier)))
   const valid = codeChallenge === storedChallenge
   ```

### Code Challenge Format

**Example from Standard Notes app**:
```
code_challenge = "MDQwNjE5NGUzMWM3NTFiOWE3MmNhMzM1NTRmOGM1Y2E3ZDAzYjU3NWUyYTZiZWUyYTcwYmJmZmVhMTFlNTEyMg"
```

**Decoded** (base64):
```
0406194e31c751b9a72ca33554f8c5ca7d03b575e2a6bee2a70bbffea11e5122
```

This is a **64-character hex string** representing the SHA-256 hash.

**Length**: 86 characters (base64 without padding)

## Source Code References

Verified against official Standard Notes repositories:

1. **Server PKCE Validation**:
   - [SignIn.ts](https://github.com/standardnotes/server/blob/main/packages/auth/src/Domain/UseCase/SignIn.ts)
   - Uses: `base64URLEncode(sha256Hash(codeVerifier))`

2. **Server Crypto**:
   - [CrypterNode.ts](https://github.com/standardnotes/server/blob/main/packages/auth/src/Domain/Encryption/CrypterNode.ts)
   - `sha256Hash()` returns **HexString**

3. **Client Crypto**:
   - [crypto.ts](https://github.com/standardnotes/app/blob/main/packages/sncrypto-web/src/crypto.ts)
   - Confirms hex string is base64-encoded

## Other Notes

### Salt Generation (Already Correct)
The salt generation in `crypto/encryption.go` was already correct:
- SHA-256("identifier:pw_nonce") → hex string
- Take first 32 hex characters
- Decode to 16 bytes
- Use as Argon2 salt

### Argon2 Parameters (Already Correct)
- Algorithm: Argon2id
- Time Cost: 5 iterations
- Memory: 65536 KiB (64 MB)
- Parallelism: 1
- Output: 64 bytes

### Password Derivation (Already Correct)
- Derived key from Argon2 → 64 bytes → hex encode → 128 characters
- Master key: first 64 hex characters
- Server password: last 64 hex characters

## Files Modified

1. `auth/authentication.go` - Fixed PKCE code_challenge generation
2. `auth/code_challenge_test.go` - Updated all tests for new algorithm

## Testing Commands

```bash
# Run PKCE tests
SN_SKIP_SESSION_TESTS=true go test -run "^TestCodeChallenge" ./auth -v

# Test authentication with real account
go run test_auth.go
```

## Conclusion

The authentication issue was caused by an incorrect PKCE implementation that followed RFC 7636 instead of Standard Notes' custom variant. The fix adds the missing hex encoding step to match the exact implementation used by the official Standard Notes app and server.

**All authentication functionality is now working correctly with Protocol 004 accounts.**
