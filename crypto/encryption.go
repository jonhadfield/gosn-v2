package crypto

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	crand "crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/chacha20poly1305"
)

// Encryption - Specifics
//
// An encrypted payload consists of:
//
// items_key_id: The UUID of the itemsKey used to encrypt enc_item_key.
// enc_item_key: An encrypted protocol string joined by colons : of the following components:
// - protocol version
// - encryption nonce
// - ciphertext
// - authenticated_data
// content: An encrypted protocol string joined by colons : of the following components:
// - protocol version
// - encryption nonce
// - ciphertext
// - authenticated_data

const (
	// KeySize is the size of the key used by this AEAD, in bytes.
	KeySize = 32

	// NonceSizeX is the size of the nonce used with the XChaCha20-Poly1305
	// variant of this AEAD, in bytes.
	NonceSizeX = 24

	SaltSize = 16

	MaxPlaintextSize = 10000000
)

func SplitContent(in string) (version, nonce, cipherText, authenticatedData string) {
	components := strings.Split(in, ":")
	if len(components) < 3 {
		panic(components)
	}

	version = components[0]           // protocol version
	nonce = components[1]             // encryption nonce
	cipherText = components[2]        // ciphertext
	authenticatedData = components[3] // authenticated data

	return
}

func hexDecodeStrings(in string, noBytes int) (dn []byte, err error) {
	return hexDecodeBytes([]byte(in), noBytes)
}

func hexDecodeBytes(in []byte, noBytes int) (dn []byte, err error) {
	dn = make([]byte, noBytes)

	if _, err = hex.Decode(dn, in); err != nil {
		return
	}

	return
}

func DecryptCipherText(cipherText, rawKey, nonce, rawAuthenticatedData string) (result []byte, err error) {
	dct, err := base64.StdEncoding.DecodeString(cipherText)
	if err != nil {
		return nil, err
	}

	masterKeyBytes := []byte(rawKey)

	dst1, err := hexDecodeBytes(masterKeyBytes, KeySize)
	if err != nil {
		return
	}

	aead, err := chacha20poly1305.NewX(dst1)
	if err != nil {
		return nil, err
	}

	var dst []byte

	hexDecodedNonce, err := hexDecodeStrings(nonce, NonceSizeX)
	if err != nil {
		return nil, err
	}

	plaintext, err := aead.Open(dst, hexDecodedNonce, dct, []byte(rawAuthenticatedData))
	if err != nil {
		err = fmt.Errorf("decryptString: %w", err)
	}

	return plaintext, err
}

func GenerateItemKey(returnBytes int) string {
	itemKeyBytes := make([]byte, 64)

	_, err := crand.Read(itemKeyBytes)
	if err != nil {
		panic(err)
	}

	itemKey := hex.EncodeToString(itemKeyBytes)

	return itemKey[:returnBytes]
}

func GenerateNonce() []byte {
	bNonce := make([]byte, chacha20poly1305.NonceSizeX)

	_, err := crand.Read(bNonce)
	if err != nil {
		panic(err)
	}

	return bNonce
}

func EncryptString(plainText, key, nonce, authenticatedData string, noBytes int) (result string, err error) {
	// TODO: expecting authenticatedData to be pre base64 encoded?
	if len(nonce) == 0 {
		panic("empty nonce")
	}

	itemKey := make([]byte, noBytes)

	_, err = hex.Decode(itemKey, []byte(key))
	if err != nil {
		return
	}

	aead, err := chacha20poly1305.NewX(itemKey)
	if err != nil {
		panic(err)
	}

	var encryptedMsg []byte

	msg := []byte(plainText)

	hexDecodedNonce, err := hexDecodeStrings(nonce, NonceSizeX)
	if err != nil {
		return
	}

	// Encrypt the message and append the ciphertext to the nonce.
	encryptedMsg = aead.Seal(nil, hexDecodedNonce, msg, []byte(authenticatedData))

	return base64.StdEncoding.EncodeToString(encryptedMsg), err
}

func generateSalt(identifier, nonce string) []byte {
	// Per Standard Notes Protocol 004, salt is generated from SHA-256(identifier:nonce)
	// truncated to 32 hex characters (representing 128 bits), then decoded to 16 bytes
	// Reference: https://github.com/standardnotes/app
	//   - truncateHexString returns first 32 hex chars
	//   - argon2() calls Utils.hexStringToArrayBuffer(salt) to decode hex to binary
	const saltLengthChars = 32 // 128 bits / 4 bits-per-hex-char = 32 characters
	hashSource := fmt.Sprintf("%s:%s", identifier, nonce)

	// Compute SHA-256 hash
	preHash := sha256.Sum256([]byte(hashSource))

	// Convert to hex string (64 characters)
	hash := make([]byte, hex.EncodedLen(len(preHash)))
	hex.Encode(hash, preHash[:])

	// Take first 32 hex characters and decode to 16 bytes
	// This matches the Standard Notes app's implementation
	salt, err := hex.DecodeString(string(hash[:saltLengthChars]))
	if err != nil {
		panic(fmt.Sprintf("failed to decode salt: %v", err))
	}

	return salt
}

type GenerateEncryptedPasswordInput struct {
	UserPassword  string
	Identifier    string
	PasswordNonce string
	Debug         bool
}

func GenerateMasterKeyAndServerPassword004(input GenerateEncryptedPasswordInput) (masterKey, serverPassword string, err error) {
	keyLength := uint32(64)
	iterations := uint32(5)
	memory := uint32(64 * 1024)
	parallel := uint8(1)
	salt := generateSalt(input.Identifier, input.PasswordNonce)
	derivedKey := argon2.IDKey([]byte(input.UserPassword), salt, iterations, memory, parallel, keyLength)
	derivedKeyHex := make([]byte, hex.EncodedLen(len(derivedKey)))
	hex.Encode(derivedKeyHex, derivedKey)
	masterKey = string(derivedKeyHex[:64])
	serverPassword = string(derivedKeyHex[64:])

	return
}

func padToAESBlockSize(b []byte) []byte {
	n := aes.BlockSize - (len(b) % aes.BlockSize)
	pb := make([]byte, len(b)+n)
	copy(pb, b)
	copy(pb[len(b):], bytes.Repeat([]byte{byte(n)}, n))

	return pb
}

// Encrypt string to base64 crypto using AES.
func Encrypt(key []byte, text string) string {
	key = padToAESBlockSize(key)
	plaintext := []byte(text)

	block, err := aes.NewCipher(key)
	if err != nil {
		panic(err)
	}

	// fail if plaintext is over 10MB
	if len(plaintext) > MaxPlaintextSize {
		panic("plaintext too long. please report this issue at https://github.com/jonhadfield/gosn-v2/issues")
	}

	ciphertext := make([]byte, aes.BlockSize+len(plaintext))

	iv := ciphertext[:aes.BlockSize]
	if _, err = io.ReadFull(crand.Reader, iv); err != nil {
		panic(err)
	}

	stream := cipher.NewCFBEncrypter(block, iv)
	stream.XORKeyStream(ciphertext[aes.BlockSize:], plaintext)

	// convert to base64
	return base64.URLEncoding.EncodeToString(ciphertext)
}

// decodeCryptoText decodes URL-safe base64 encoded cipher text.
func decodeCryptoText(s string) ([]byte, error) {
	return base64.URLEncoding.DecodeString(s)
}

// Decrypt from base64 to decrypted string.
func Decrypt(key []byte, cryptoText string) (pt string, err error) {
	ciphertext, err := decodeCryptoText(cryptoText)
	if err != nil {
		return
	}

	key = padToAESBlockSize(key)

	var block cipher.Block

	if block, err = aes.NewCipher(key); err != nil {
		return
	}

	if len(ciphertext) < aes.BlockSize {
		return "", errors.New("ciphertext too short")
	}

	iv := ciphertext[:aes.BlockSize]
	ciphertext = ciphertext[aes.BlockSize:]

	stream := cipher.NewCFBDecrypter(block, iv)
	stream.XORKeyStream(ciphertext, ciphertext)

	pt = string(ciphertext)

	return
}
