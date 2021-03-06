package gosn

import (
	"bytes"
	"crypto/aes"
	crand "crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"

	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/pbkdf2"
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

const (
	// KeySize is the size of the key used by this AEAD, in bytes.
	KeySize = 32

	// NonceSize is the size of the nonce used with the standard variant of this
	// AEAD, in bytes.
	//
	// Note that this is too short to be safely generated at random if the same
	// key is reused more than 2³² times.
	NonceSize = 12

	// NonceSizeX is the size of the nonce used with the XChaCha20-Poly1305
	// variant of this AEAD, in bytes.
	NonceSizeX = 24
)

// Encryption - Specifics
//
//An encrypted payload consists of:
//
//items_key_id: The UUID of the itemsKey used to encrypt enc_item_key.
//enc_item_key: An encrypted protocol string joined by colons : of the following components:
// - protocol version
// - encryption nonce
// - ciphertext
// - authenticated_data
//content: An encrypted protocol string joined by colons : of the following components:
// - protocol version
// - encryption nonce
// - ciphertext
// - authenticated_data

func decryptString(cipherText, rawKey, nonce, rawAuthenticatedData string) (result []byte, err error) {
	dct, e1 := base64.StdEncoding.DecodeString(cipherText)
	if e1 != nil {
		panic(e1)
	}

	masterKeyBytes := []byte(rawKey)

	// hex decode masterkey
	dst1 := make([]byte, 32)
	_, err = hex.Decode(dst1, masterKeyBytes)
	if err != nil {
		return
	}

	aead, err := chacha20poly1305.NewX(dst1)
	if err != nil {
		return nil, err
	}

	var dst []byte

	hexDecodedNonce := make([]byte, 24)
	_, err = hex.Decode(hexDecodedNonce, []byte(nonce))
	if err != nil {
		return nil, err
	}
	plaintext, err := aead.Open(dst, hexDecodedNonce, dct, []byte(rawAuthenticatedData))
	if err != nil {
		return
	}
	return plaintext, err
}

// decryptItemsKeys takes the master key and a list of EncryptedItemKeys
// and returns a list of items keys
func decryptAndParseItemKeys(mk string, eiks EncryptedItems) (iks []ItemsKey, err error) {
	for _, eik := range eiks {
		// decrypt enc_item_key
		_, encNonce, cipherText, authenticatedData := splitContent(eik.EncItemKey)
		var pt []byte
		pt, err = decryptString(cipherText, mk, encNonce, authenticatedData)
		if err != nil {
			return
		}
		// decrypt content with item_key
		_, encNonce1, cipherText1, authenticatedData1 := splitContent(eik.Content)
		var pt1 []byte
		pt1, err = decryptString(cipherText1, string(pt), encNonce1, authenticatedData1)
		if err != nil {
			return
		}

		var f ItemsKey
		err = json.Unmarshal(pt1, &f)
		if err != nil {
			return
		}

		f.UUID = eik.UUID
		f.ContentType = eik.ContentType
		f.UpdatedAt = eik.UpdatedAt
		iks = append(iks, f)
	}

	return

}

func getMatchingItem(uuid string, iks []ItemsKey) ItemsKey {
	for _, i := range iks {
		if uuid == i.UUID {
			return i
		}
	}

	return ItemsKey{}
}

// decryptItems takes the itemsKeys and the EncryptedItems to decrypt
// and returns a list of items keys uuid (string) and key (bytes)
func decryptItems(s *Session, eis EncryptedItems) (items DecryptedItems, err error) {
	for _, ei := range eis {
		if ! isEncryptedType(ei.ContentType) || ei.Deleted {
			continue
		}

		// decrypt item key with itemsKey
		if ei.ItemsKeyID == "" {
			debugPrint(s.Debug, fmt.Sprintf("ignoring invalid item: %+v\n", ei))

			continue
		}

		ik := getMatchingItem(ei.ItemsKeyID, s.ItemsKeys)
		if ik.UUID == "" {
			err = errors.New("wrong items key passed for decrypting item key")
			return
		}

		version, nonce, cipherText, authData := splitContent(ei.EncItemKey)
		if version != "004" {
			err = errors.New(fmt.Sprintf("your account contains items encrypted with an earlier version of Standard Notes\nto upgrade your encryption, perform a backup and restore via the official app"))
			return
		}
		var itemKey []byte
		itemKey, err = decryptString(cipherText, ik.ItemsKey, nonce, authData)
		if err != nil {
			return
		}
		version, nonce, cipherText, authData = splitContent(ei.Content)
		var content []byte
		content, err = decryptString(cipherText, string(itemKey), nonce, authData)
		if err != nil {
			return
		}
		var di DecryptedItem
		di.UUID = ei.UUID
		di.ItemsKeyID = ei.ItemsKeyID
		di.ContentType = ei.ContentType
		di.Deleted = ei.Deleted
		di.UpdatedAt = ei.UpdatedAt
		di.CreatedAt = ei.CreatedAt
		di.Content = string(content)
		items = append(items, di)
	}

	return
}

func encryptItems(s Session, decItems *Items) (encryptedItems EncryptedItems, err error) {
	debugPrint(s.Debug, fmt.Sprintf("encryptItems | encrypting %d items", len(*decItems)))
	d := *decItems
	for _, decItem := range d {
		var e EncryptedItem
		e, err = encryptItem(decItem, s.DefaultItemsKey)
		encryptedItems = append(encryptedItems, e)
	}

	return
}

func encryptString(plainText, key, nonce, authenticatedData string) (result string, err error) {
	// TODO: expecting authenticatedData to be pre base64 encoded?
	if len(nonce) == 0 {
		panic("empty nonce")
	}

	// Re-use previous item key (cheating for now)
	itemKey := make([]byte, 32)
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

	hexDecodedNonce := make([]byte, 24)
	_, err = hex.Decode(hexDecodedNonce, []byte(nonce))
	if err != nil {
		return
	}

	// Encrypt the message and append the ciphertext to the nonce.
	encryptedMsg = aead.Seal(nil, hexDecodedNonce, msg, []byte(authenticatedData))

	return base64.StdEncoding.EncodeToString(encryptedMsg), err
}

func encryptItem(item Item, ik ItemsKey) (encryptedItem EncryptedItem, err error) {
	encryptedItem.UUID = item.GetUUID()
	encryptedItem.ItemsKeyID = ik.GetUUID()
	encryptedItem.ContentType = item.GetContentType()
	encryptedItem.UpdatedAt = item.GetUpdatedAt()
	encryptedItem.CreatedAt = item.GetCreatedAt()
	encryptedItem.Deleted = item.IsDeleted()
	// Generate Item Key
	itemKeyBytes := make([]byte, 64)

	_, err = crand.Read(itemKeyBytes)
	if err != nil {
		panic(err)
	}

	itemKey := hex.EncodeToString(itemKeyBytes)
	// get Item Encryption Key
	itemEncryptionKey := itemKey[:len(itemKey)/2]
	// encrypt Item content
	var encryptedContent string

	mContent, _ := json.Marshal(item.GetContent())

	authData := "{\"u\":\"" + item.GetUUID() + "\",\"v\":\"004\"}"
	b64AuthData := base64.StdEncoding.EncodeToString([]byte(authData))

	// generate nonce
	bNonce := make([]byte, chacha20poly1305.NonceSizeX)
	_, err = crand.Read(bNonce)
	if err != nil {
		panic(err)
	}
	nonce := hex.EncodeToString(bNonce)

	encryptedContent, err = encryptString(string(mContent), itemEncryptionKey, nonce, b64AuthData)
	if err != nil {
		panic(err)
		return
	}
	content := fmt.Sprintf("004:%s:%s:%s", nonce, encryptedContent, b64AuthData)

	encryptedItem.Content = content

	// generate nonce
	_, err = crand.Read(bNonce)
	if err != nil {
		panic(err)
	}
	nonce = hex.EncodeToString(bNonce)

	// encrypt content encryption key with default Items Key
	var encryptedContentKey string
	encryptedContentKey, err = encryptString(itemEncryptionKey, ik.ItemsKey, nonce, b64AuthData)
	encItemKey := fmt.Sprintf("004:%s:%s:%s", nonce, encryptedContentKey, b64AuthData)
	encryptedItem.EncItemKey = encItemKey

	return encryptedItem, err
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
	hashHexString := string(hash)
	decodedHex64, _ := hex.DecodeString(hashHexString[:saltLength])
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

func getBodyContent(input []byte) (output syncResponse, err error) {
	err = json.Unmarshal(input, &output)
	if err != nil {
		return
	}

	return
}

func padToAESBlockSize(b []byte) []byte {
	n := aes.BlockSize - (len(b) % aes.BlockSize)
	pb := make([]byte, len(b)+n)
	copy(pb, b)
	copy(pb[len(b):], bytes.Repeat([]byte{byte(n)}, n))

	return pb
}
