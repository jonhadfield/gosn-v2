# Authentication Request Comparison: Standard Notes App vs gosn-v2

## Critical Differences Found

### 1. **Code Verifier Length and Encoding**

#### Standard Notes App
```typescript
// Generates 256-byte (2048-bit) random verifier
const codeVerifier = this.crypto.generateRandomKey(256)

// Challenge = base64URL(sha256(verifier))
const codeChallenge = this.crypto.base64URLEncode(
  await this.crypto.sha256(codeVerifier)
)
```

#### gosn-v2
```go
// Generates 64-character alphanumeric verifier (~384 bits)
letterRunes := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
b := make([]rune, 65)
for i := range b {
    b[i] = letterRunes[rnd.Intn(len(letterRunes))]
}
loginCodeVerifier.codeVerifier = string(b)[:64]

// Challenge = base64(hex(sha256(verifier)))[:86]
sha25Hash := fmt.Sprintf("%x", sha256.Sum256([]byte(loginCodeVerifier.codeVerifier)))
loginCodeVerifier.codeChallenge = string(base64.URLEncoding.EncodeToString([]byte(sha25Hash)))[:86]
```

⚠️ **PROBLEM 1**: gosn-v2 is:
1. Hashing the verifier to get binary output
2. Converting to hex string (doubling the length)
3. Base64 encoding the hex string (not the binary hash)
4. Truncating to 86 characters

**App does:** base64URL(sha256_binary(verifier))
**gosn-v2 does:** base64(hex_string(sha256(verifier)))[:86]

This produces **completely different challenge values**!

---

### 2. **Request Body Structure**

#### Standard Notes App - Auth Params Request
```json
{
  "api": "20240226",
  "email": "user@example.com",
  "code_challenge": "TRhvmtjRODs-0fQiXr2uE_0E0JfgRx9RdguyU-EXAMPLE"
}
```

#### gosn-v2 - Auth Params Request
```json
{
  "api": "20240226",
  "email": "user@example.com",
  "code_challenge": "DIFFERENT_VALUE_DUE_TO_ENCODING_ISSUE"
}
```

---

### 3. **Sign-In Request Comparison**

#### Standard Notes App
```typescript
{
  api: "20240226",
  email: dto.email,
  password: dto.serverPassword,  // Already derived via Argon2id
  ephemeral: dto.ephemeral,      // boolean
  code_verifier: codeVerifier,   // Original 256-byte verifier
  hvm_token: dto.hvmToken        // Optional, undefined if not set
}
```

#### gosn-v2
```go
// WITHOUT MFA
{
  "api": "20240226",
  "password": serverPassword,
  "email": email,
  "code_verifier": verifier,
  "ephemeral": false,            // Always false (hardcoded)
  "hvm_token": ""                // Empty string (not omitted)
}

// WITH MFA
{
  "api": "20240226",
  "password": serverPassword,
  "email": email,
  "mfa_token_name": "mfa_key",
  "mfa_token_value": "123456",
  "code_verifier": verifier
}
```

⚠️ **PROBLEM 2**: gosn-v2 sends `"hvm_token": ""` (empty string) instead of omitting the field when undefined

⚠️ **PROBLEM 3**: MFA token fields have different names:
- **App uses**: Dynamic key from error response (e.g., `mfa_code`)
- **gosn-v2 uses**: `tokenName` and `tokenValue` variables but unclear if names match

---

## Root Cause Analysis

### Primary Issue: Code Challenge Generation

The server validates that:
```
sha256(code_verifier_from_signin) === decode(code_challenge_from_params)
```

**gosn-v2's challenge won't validate** because:

1. **Auth params request** sends: `base64(hex(sha256(verifier)))`
2. **Sign-in request** sends: original `verifier`
3. **Server computes**: `sha256(verifier)`
4. **Server compares** to decoded challenge: MISMATCH!

The server expects:
- Challenge = base64URL(binary_sha256(verifier))
- Verifier = original random bytes

gosn-v2 sends:
- Challenge = base64(hex_string(sha256(verifier)))
- Verifier = ASCII string

---

## Required Fixes

### Fix 1: Correct Code Challenge Generation

```go
func generateChallengeAndVerifierForLogin() (loginCodeVerifier generateLoginChallengeCodeVerifier) {
    // Generate random bytes (not string)
    verifierBytes := make([]byte, 64) // or 32, or 256 like the app
    _, err := rand.Read(verifierBytes)
    if err != nil {
        panic(err)
    }

    // Verifier = base64URL encoded bytes for transmission
    loginCodeVerifier.codeVerifier = base64.URLEncoding.EncodeToString(verifierBytes)

    // Challenge = base64URL(sha256(verifierBytes))
    hash := sha256.Sum256(verifierBytes)
    loginCodeVerifier.codeChallenge = base64.URLEncoding.EncodeToString(hash[:])

    return loginCodeVerifier
}
```

### Fix 2: Don't Send Empty hvm_token

```go
// Build request dynamically, only include fields with values
type signInRequest struct {
    API          string `json:"api"`
    Email        string `json:"email"`
    Password     string `json:"password"`
    CodeVerifier string `json:"code_verifier"`
    Ephemeral    bool   `json:"ephemeral"`
    HvmToken     string `json:"hvm_token,omitempty"` // omitempty!
}
```

Or conditionally construct JSON:

```go
if input.tokenName != "" {
    reqBody = fmt.Sprintf(`{"api":"%s","password":"%s","email":"%s","%s":"%s","code_verifier":"%s"}`,
        apiVer, input.encPassword, e, input.tokenName, input.tokenValue, input.codeVerifier)
} else if hvmToken != "" {
    reqBody = fmt.Sprintf(`{"api":"%s","password":"%s","email":"%s","code_verifier":"%s","ephemeral":false,"hvm_token":"%s"}`,
        apiVer, input.encPassword, e, input.codeVerifier, hvmToken)
} else {
    reqBody = fmt.Sprintf(`{"api":"%s","password":"%s","email":"%s","code_verifier":"%s","ephemeral":false}`,
        apiVer, input.encPassword, e, input.codeVerifier)
}
```

---

## Testing Verification

After fixes, verify:

1. ✅ Code verifier is random bytes (base64URL encoded for JSON)
2. ✅ Code challenge = base64URL(sha256(raw_verifier_bytes))
3. ✅ Server can decode challenge and compare with hash of verifier
4. ✅ No empty string fields in JSON (use omitempty or conditional construction)
5. ✅ Ephemeral flag set correctly
6. ✅ MFA token field names match server expectations

---

## Test Cases to Add

```go
func TestCodeChallengeVerification(t *testing.T) {
    verifier := generateChallengeAndVerifierForLogin()

    // Decode challenge
    challengeBytes, err := base64.URLEncoding.DecodeString(verifier.codeChallenge)
    require.NoError(t, err)

    // Decode verifier
    verifierBytes, err := base64.URLEncoding.DecodeString(verifier.codeVerifier)
    require.NoError(t, err)

    // Hash verifier
    hash := sha256.Sum256(verifierBytes)

    // Should match decoded challenge
    require.Equal(t, hash[:], challengeBytes)
}
```
