package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	crand "crypto/rand"
	"encoding/base64"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestEncryptAcceptsKeysOfAnyLength covers the lengths that used to fail:
// padding pushed a key of 32 bytes or more past a valid AES key size, so every
// passphrase of 32 characters or more was rejected.
func TestEncryptAcceptsKeysOfAnyLength(t *testing.T) {
	t.Parallel()

	const plaintext = "a session worth protecting"

	for _, keyLen := range []int{1, 8, 15, 16, 17, 24, 31, 32, 33, 40, 48, 64, 100} {
		key := []byte(strings.Repeat("k", keyLen))

		ciphertext, err := Encrypt(key, plaintext)
		require.NoErrorf(t, err, "encrypting with a %d byte key", keyLen)

		got, err := Decrypt(key, ciphertext)
		require.NoErrorf(t, err, "decrypting with a %d byte key", keyLen)
		require.Equal(t, plaintext, got)
	}
}

func TestEncryptIsNotDeterministic(t *testing.T) {
	t.Parallel()

	key := []byte("a passphrase")

	first, err := Encrypt(key, "same plaintext")
	require.NoError(t, err)

	second, err := Encrypt(key, "same plaintext")
	require.NoError(t, err)

	// a fresh salt and nonce each time, so the same input never looks the same
	require.NotEqual(t, first, second)
}

func TestDecryptRejectsWrongKey(t *testing.T) {
	t.Parallel()

	ciphertext, err := Encrypt([]byte("the right passphrase"), "secret")
	require.NoError(t, err)

	_, err = Decrypt([]byte("the wrong passphrase"), ciphertext)
	require.Error(t, err)
}

// TestDecryptRejectsTamperedCiphertext is the behaviour the old AES-CFB format
// did not have: without authentication it returned altered plaintext instead of
// an error.
func TestDecryptRejectsTamperedCiphertext(t *testing.T) {
	t.Parallel()

	key := []byte("a passphrase")

	ciphertext, err := Encrypt(key, "transfer 10 pounds")
	require.NoError(t, err)

	raw, err := base64.URLEncoding.DecodeString(strings.TrimPrefix(ciphertext, sessionCryptoPrefix))
	require.NoError(t, err)

	// flip a bit in the ciphertext, past the salt and nonce
	raw[len(raw)-1] ^= 0x01

	_, err = Decrypt(key, sessionCryptoPrefix+base64.URLEncoding.EncodeToString(raw))
	require.Error(t, err)
}

// TestDecryptReadsLegacyFormat checks a session stored before this change can
// still be opened. The helper below is the old Encrypt, kept only for the test.
func TestDecryptReadsLegacyFormat(t *testing.T) {
	t.Parallel()

	const plaintext = "a session stored by an older version"

	for _, keyLen := range []int{1, 16, 24, 31} {
		key := []byte(strings.Repeat("k", keyLen))

		legacy, err := legacyEncryptCFB(key, plaintext)
		require.NoErrorf(t, err, "legacy encrypt with a %d byte key", keyLen)

		// no prefix, so Decrypt has to recognise it as the old format
		require.False(t, strings.HasPrefix(legacy, sessionCryptoPrefix))

		got, err := Decrypt(key, legacy)
		require.NoErrorf(t, err, "decrypting legacy text with a %d byte key", keyLen)
		require.Equal(t, plaintext, got)
	}
}

// legacyEncryptCFB reproduces the pre-argon2id Encrypt, so the test can write
// the old format without keeping the old code in the package.
func legacyEncryptCFB(key []byte, text string) (string, error) {
	key = padToAESBlockSize(key)

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	plaintext := []byte(text)
	ciphertext := make([]byte, aes.BlockSize+len(plaintext))

	iv := ciphertext[:aes.BlockSize]
	if _, err = io.ReadFull(crand.Reader, iv); err != nil {
		return "", err
	}

	cipher.NewCFBEncrypter(block, iv).XORKeyStream(ciphertext[aes.BlockSize:], plaintext)

	return base64.URLEncoding.EncodeToString(ciphertext), nil
}
