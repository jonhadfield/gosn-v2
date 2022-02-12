package gosn

import (
	"bytes"
	"crypto/aes"
	crand "crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/pbkdf2"
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
)

func splitContent(in string) (version, nonce, cipherText, authenticatedData string) {
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

func DecryptString(cipherText, rawKey, nonce, rawAuthenticatedData string) (result []byte, err error) {
	dct, e1 := base64.StdEncoding.DecodeString(cipherText)
	if e1 != nil {
		fmt.Println("dead ddd")
		panic(e1)
	}

	masterKeyBytes := []byte(rawKey)
	fmt.Printf("Decoding: %s\n", rawKey)
	fmt.Println(rawKey)
	dst1, err := hexDecodeBytes(masterKeyBytes, KeySize)
	if err != nil {
		fmt.Println("dead eee")
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

func generateAuthData(ct, uuid string, kp KeyParams) string {
	var ad string

	if ct == "SN|ItemsKey" {
		ad = "{\"kp\":{\"identifier\":\"" + kp.Identifier + "\",\"pw_nonce\":\"" + kp.PwNonce + "\",\"version\":\"" + kp.Version + "\",\"origination\":\"" + kp.Origination + "\",\"created\":\"" + kp.Created + "\"},\"u\":\"" + uuid + "\",\"v\":\"" + kp.Version + "\"}"
		fmt.Printf("AUTHDATA: %s\n", ad)

		return ad
	}

	ad = "{\"u\":\"" + uuid + "\",\"v\":\"004\"}"
	fmt.Printf("AUTHDATA: %s\n", ad)

	return ad
}

func generateItemKey(returnBytes int) string {
	itemKeyBytes := make([]byte, 64)

	_, err := crand.Read(itemKeyBytes)
	if err != nil {
		panic(err)
	}

	itemKey := hex.EncodeToString(itemKeyBytes)

	return itemKey[:returnBytes]
}

func generateNonce() []byte {
	bNonce := make([]byte, chacha20poly1305.NonceSizeX)

	_, err := crand.Read(bNonce)
	if err != nil {
		panic(err)
	}

	return bNonce
}

func getMatchingItem(uuid string, iks []ItemsKey) ItemsKey {
	for x := range iks {
		if uuid == iks[x].UUID {
			return iks[x]
		}
	}

	return ItemsKey{}
}

func isEncryptedWithMasterKey(t string) bool {
	return t == "SN|ItemsKey"
}

func isUnsupportedType(t string) bool {
	return strings.HasPrefix(t, "SF|")
}

func decryptEncryptedItemKey(e EncryptedItem, encryptionKey string) (itemKey []byte, err error) {
	_, nonce, cipherText, authData := splitContent(e.EncItemKey)

	return DecryptString(cipherText, encryptionKey, nonce, authData)
}

func decryptContent(e EncryptedItem, encryptionKey string) (content []byte, err error) {
	_, nonce, cipherText, authData := splitContent(e.Content)

	content, err = DecryptString(cipherText, encryptionKey, nonce, authData)
	if err != nil {
		return
	}

	c := string(content)
	if !stringInSlice(e.ContentType, []string{"SN|FileSafe|Integration", "SN|FileSafe|Credentials", "SN|Component", "SN|Theme"}, true) && len(c) > 250 {
		return
	}

	return
}

func encryptString(plainText, key, nonce, authenticatedData string, noBytes int) (result string, err error) {
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
	saltLength := 32
	hashSource := fmt.Sprintf("%s:%s", identifier, nonce)
	h := sha256.New()

	if _, err := h.Write([]byte(hashSource)); err != nil {
		panic(err)
	}

	preHash := sha256.Sum256([]byte(hashSource))
	hash := make([]byte, hex.EncodedLen(len(preHash)))
	hex.Encode(hash, preHash[:])

	decodedHex64, err := hexDecodeStrings(string(hash)[:saltLength], SaltSize)
	if err != nil {
		panic(err)
	}

	return decodedHex64
}

func generateMasterKeyAndServerPassword004(input generateEncryptedPasswordInput) (masterKey, serverPassword string, err error) {
	keyLength := uint32(64)
	iterations := uint32(5)
	memory := uint32(64 * 1024)
	parallel := uint8(1)
	salt := generateSalt(input.Identifier, input.PasswordNonce)
	derivedKey := argon2.IDKey([]byte(input.userPassword), salt, iterations, memory, parallel, keyLength)
	derivedKeyHex := make([]byte, hex.EncodedLen(len(derivedKey)))
	hex.Encode(derivedKeyHex, derivedKey)
	masterKey = string(derivedKeyHex[:64])
	serverPassword = string(derivedKeyHex[64:])

	return
}

func generateEncryptedPasswordAndKeys(input generateEncryptedPasswordInput) (pw, mk, ak string, err error) {
	if input.Version == "003" && input.PasswordCost < 100000 {
		err = fmt.Errorf("password cost too low")
		return
	}

	saltSource := input.Identifier + ":" + "SF" + ":" + input.Version + ":" + strconv.Itoa(int(input.PasswordCost)) + ":" + input.PasswordNonce

	h := sha256.New()
	if _, err = h.Write([]byte(saltSource)); err != nil {
		return
	}

	preSalt := sha256.Sum256([]byte(saltSource))
	salt := make([]byte, hex.EncodedLen(len(preSalt)))
	hex.Encode(salt, preSalt[:])
	hashedPassword := pbkdf2.Key([]byte(input.userPassword), salt, int(input.PasswordCost), 96, sha512.New)
	hexedHashedPassword := hex.EncodeToString(hashedPassword)
	splitLength := len(hexedHashedPassword) / 3
	pw = hexedHashedPassword[:splitLength]
	mk = hexedHashedPassword[splitLength : splitLength*2]
	ak = hexedHashedPassword[splitLength*2 : splitLength*3]

	return
}

func padToAESBlockSize(b []byte) []byte {
	n := aes.BlockSize - (len(b) % aes.BlockSize)
	pb := make([]byte, len(b)+n)
	copy(pb, b)
	copy(pb[len(b):], bytes.Repeat([]byte{byte(n)}, n))

	return pb
}
