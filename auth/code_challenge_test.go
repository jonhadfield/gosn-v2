package auth

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestCodeChallengeVerification verifies that the PKCE code challenge generation
// follows the Standard Notes algorithm: challenge = base64(hex(sha256(verifier_string)))
func TestCodeChallengeVerification(t *testing.T) {
	verifier := generateChallengeAndVerifierForLogin()

	// Decode the stored verifier (it's base64 encoded for JSON transmission)
	verifierBytes, err := base64.URLEncoding.DecodeString(verifier.codeVerifier)
	require.NoError(t, err, "Verifier must be valid base64")
	require.Len(t, verifierBytes, 64, "Verifier should be 64 bytes")

	// Standard Notes PKCE algorithm:
	// 1. SHA-256 hash of the code_verifier STRING
	hash := sha256.Sum256([]byte(verifier.codeVerifier))

	// 2. Hex encode the hash
	hashHex := make([]byte, hex.EncodedLen(len(hash)))
	hex.Encode(hashHex, hash[:])

	// 3. Base64-url encode the hex string (without padding)
	expectedChallenge := base64.RawURLEncoding.EncodeToString(hashHex)

	// Verify challenge matches
	require.Equal(t, expectedChallenge, verifier.codeChallenge,
		"Challenge must equal base64(hex(sha256(verifier_string))) for Standard Notes PKCE")
}

// TestCodeChallengeAlgorithm verifies the algorithm matches Standard Notes implementation
func TestCodeChallengeAlgorithm(t *testing.T) {
	result := generateChallengeAndVerifierForLogin()

	// Verifier should be valid base64
	verifierBytes, err := base64.URLEncoding.DecodeString(result.codeVerifier)
	require.NoError(t, err, "Verifier must be valid base64")
	require.Len(t, verifierBytes, 64, "Verifier should be 64 bytes")

	// Challenge should decode to a 64-character hex string (representing 32-byte hash)
	challengeDecoded, err := base64.RawURLEncoding.DecodeString(result.codeChallenge)
	require.NoError(t, err, "Challenge must be valid base64")
	require.Len(t, challengeDecoded, 64, "Challenge should decode to 64 hex characters")

	// Verify the decoded challenge is valid hex
	_, err = hex.DecodeString(string(challengeDecoded))
	require.NoError(t, err, "Challenge should contain valid hex string")
}

// TestCodeChallengeUniqueness verifies each call generates unique verifier/challenge pairs
func TestCodeChallengeUniqueness(t *testing.T) {
	result1 := generateChallengeAndVerifierForLogin()
	result2 := generateChallengeAndVerifierForLogin()

	require.NotEqual(t, result1.codeVerifier, result2.codeVerifier,
		"Each call should generate a unique verifier")
	require.NotEqual(t, result1.codeChallenge, result2.codeChallenge,
		"Each call should generate a unique challenge")
}

// TestCodeChallengeLengths verifies the expected base64 lengths
func TestCodeChallengeLengths(t *testing.T) {
	result := generateChallengeAndVerifierForLogin()

	// Verifier: 64 bytes = 86 base64 characters (with padding) or 85-86 without
	verifierBytes, err := base64.URLEncoding.DecodeString(result.codeVerifier)
	require.NoError(t, err)
	require.Len(t, verifierBytes, 64, "Verifier should be 64 bytes")

	// Challenge: 64 hex characters = 86-88 base64 characters
	// Standard Notes uses 86 characters (no padding)
	challengeDecoded, err := base64.RawURLEncoding.DecodeString(result.codeChallenge)
	require.NoError(t, err)
	require.Len(t, challengeDecoded, 64, "Challenge should decode to 64 hex characters")

	// Verify string lengths
	require.Greater(t, len(result.codeVerifier), 80, "Base64 of 64 bytes should be ~86 chars")
	require.Less(t, len(result.codeVerifier), 90, "Base64 of 64 bytes should be ~86 chars")

	// Challenge should be ~86 chars (base64 of 64 hex chars without padding)
	require.Greater(t, len(result.codeChallenge), 80, "Base64 of 64 hex chars should be ~86 chars")
	require.Less(t, len(result.codeChallenge), 92, "Base64 of 64 hex chars should be ~86-88 chars")
}

// TestCodeVerifierServerValidation simulates server-side validation logic
func TestCodeVerifierServerValidation(t *testing.T) {
	// This test simulates what the Standard Notes server does:
	// 1. Receives code_challenge in /auth/params request
	// 2. Receives code_verifier in /login request
	// 3. Validates: base64(hex(sha256(code_verifier))) == code_challenge

	result := generateChallengeAndVerifierForLogin()

	// Step 1: Server stores the challenge from auth params request
	storedChallenge := result.codeChallenge

	// Step 2: Server receives verifier in sign-in request
	receivedVerifier := result.codeVerifier

	// Step 3: Server validation logic (Standard Notes implementation)
	// Hash the verifier string
	hash := sha256.Sum256([]byte(receivedVerifier))

	// Hex encode the hash
	hashHex := make([]byte, hex.EncodedLen(len(hash)))
	hex.Encode(hashHex, hash[:])

	// Base64-url encode (without padding)
	computedChallenge := base64.RawURLEncoding.EncodeToString(hashHex)

	// Validate
	require.Equal(t, storedChallenge, computedChallenge,
		"Server validation should pass: computed challenge from verifier must match stored challenge")
}

// TestStandardNotesPKCEFormat verifies the exact format matches the Standard Notes app
func TestStandardNotesPKCEFormat(t *testing.T) {
	result := generateChallengeAndVerifierForLogin()

	// The challenge from Standard Notes app is 86 characters (base64 without padding)
	// Example: "MDQwNjE5NGUzMWM3NTFiOWE3MmNhMzM1NTRmOGM1Y2E3ZDAzYjU3NWUyYTZiZWUyYTcwYmJmZmVhMTFlNTEyMg"
	// This decodes to: "0406194e31c751b9a72ca33554f8c5ca7d03b575e2a6bee2a70bbffea11e5122"
	// Which is a 64-character hex string representing a 32-byte SHA-256 hash

	// Verify our challenge follows the same format
	challengeDecoded, err := base64.RawURLEncoding.DecodeString(result.codeChallenge)
	require.NoError(t, err, "Challenge must be valid base64 without padding")

	// Should decode to 64 hex characters
	require.Len(t, challengeDecoded, 64, "Challenge should decode to 64 hex characters")

	// All characters should be valid hex (0-9, a-f)
	for _, b := range challengeDecoded {
		require.True(t, (b >= '0' && b <= '9') || (b >= 'a' && b <= 'f'),
			"Challenge should contain only lowercase hex characters")
	}
}
