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
		err = fmt.Errorf("decryptString: %w", err)
	}

	return plaintext, err
}

func (ik ItemsKey) Encrypt(session *Session, new bool) (encryptedItem EncryptedItem, err error) {
	if ik.ItemsKey == "" {
		debugPrint(session.Debug, fmt.Sprintf("ItemsKey Encrypt | skipping %s due to missing Content.ItemsKey", ik.UUID))

		return
	}

	if ik.UUID == "" {
		panic("ik.UUID is empty")
	}

	// updated at is set by SN so will be zero for a new key
	//if !new {
	//	if ik.UpdatedAt == "" {
	//		panic("ik.UpdatedAt is empty")
	//	}
	//
	//	if ik.UpdatedAtTimestamp == 0 {
	//		panic("ik.UpdatedAtTimestamp is empty")
	//	}
	//}

	encryptedItem.UUID = ik.UUID

	if ik.ContentType == "" {
		panic("ik.ContentType is empty")
	}

	encryptedItem.ContentType = ik.ContentType

	// updatedat is set by SN so will be zero for a new key
	if !new {
		encryptedItem.UpdatedAt = ik.UpdatedAt
		encryptedItem.UpdatedAtTimestamp = ik.UpdatedAtTimestamp
	}

	encryptedItem.CreatedAt = ik.CreatedAt
	encryptedItem.Deleted = ik.Deleted

	if ik.CreatedAtTimestamp == 0 {
		panic("ik.CreatedAtTimeStamp is 0")
	}

	encryptedItem.CreatedAtTimestamp = ik.CreatedAtTimestamp

	itemKeyBytes := make([]byte, 64)

	_, err = crand.Read(itemKeyBytes)
	if err != nil {
		panic(err)
	}

	itemKey := hex.EncodeToString(itemKeyBytes)
	// Create Item Encryption Key (that will encrypt the items key content itself)
	itemEncryptionKey := itemKey[:len(itemKey)/2]

	var encryptedContent string

	if ik.ItemsKey == "" {
		panic("attempting to encrypt empty items key")
	}
	// Marshall the ItemsKey plaintext content
	mContent, err := json.Marshal(ik.Content)
	if err != nil {
		return
	}

	// Create the auth data that will be used to authenticate the encrypted content
	authData := "{\"kp\":{\"identifier\":\"" + session.KeyParams.Identifier + "\",\"pw_nonce\":\"" + session.KeyParams.PwNonce + "\",\"version\":\"" + session.KeyParams.Version + "\",\"origination\":\"" + session.KeyParams.Origination + "\",\"created\":\"" + session.KeyParams.Created + "\"},\"u\":\"" + encryptedItem.UUID + "\",\"v\":\"" + session.KeyParams.Version + "\"}"

	b64AuthData := base64.StdEncoding.EncodeToString([]byte(authData))
	// Generate nonce
	bNonce := make([]byte, chacha20poly1305.NonceSizeX)

	_, err = crand.Read(bNonce)
	if err != nil {
		panic(err)
	}

	nonce := hex.EncodeToString(bNonce)

	// Encrypt the marshaled JSON with the item encryption key, nonce, and auth data
	encryptedContent, err = encryptString(string(mContent), itemEncryptionKey, nonce, b64AuthData)
	if err != nil {
		return
	}

	// Create the Encrypted Items Key content element
	content := fmt.Sprintf("004:%s:%s:%s", nonce, encryptedContent, b64AuthData)

	encryptedItem.Content = content
	// Generate another nonce for the content element to be encrypted with
	_, err = crand.Read(bNonce)
	if err != nil {
		panic(err)
	}

	nonce = hex.EncodeToString(bNonce)

	// Encrypt the Encrypted Items Key content element with the master key
	var encryptedContentKey string
	encryptedContentKey, err = encryptString(itemEncryptionKey, session.MasterKey, nonce, b64AuthData)
	encItemKey := fmt.Sprintf("004:%s:%s:%s", nonce, encryptedContentKey, b64AuthData)
	encryptedItem.EncItemKey = encItemKey

	switch {
	case encryptedItem.EncItemKey == "":
		panic("produced encrypted ItemsKey with empty enc_item_key")
	case encryptedItem.UUID == "":
		panic("produced encrypted ItemsKey with empty uuid")
	case encryptedItem.Content == "":
		panic("produced encrypted ItemsKey with empty content")
	case encryptedItem.ItemsKeyID != nil:
		panic("produced encrypted ItemsKey non nil ItemsKeyID")
	case encryptedItem.CreatedAtTimestamp == 0:
		panic("encrypted items key has CreatedAtTimestamp set to 0")
	}

	return encryptedItem, err
}

func (ei EncryptedItem) Decrypt(mk string) (ik ItemsKey, err error) {
	if ei.ContentType != "SN|ItemsKey" {
		return ik, fmt.Errorf("item passed to decrypt is of type %s, expected SN|ItemsKey", ik.ContentType)
	}
	// decrypt enc_item_key
	_, encNonce, cipherText, authenticatedData := splitContent(ei.EncItemKey)

	var pt []byte

	pt, err = decryptString(cipherText, mk, encNonce, authenticatedData)
	if err != nil {
		err = fmt.Errorf("DecryptAndParseItemKeys: %w", err)

		return
	}

	// decrypt content with item key
	_, encNonce1, cipherText1, authenticatedData1 := splitContent(ei.Content)

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

	f.UUID = ei.UUID
	f.ContentType = ei.ContentType
	f.UpdatedAt = ei.UpdatedAt
	f.UpdatedAtTimestamp = ei.UpdatedAtTimestamp
	f.CreatedAtTimestamp = ei.CreatedAtTimestamp
	f.CreatedAt = ei.CreatedAt

	ik = f

	return
}

func EncryptItem(item Item, ik ItemsKey, session *Session) (encryptedItem EncryptedItem, err error) {
	var contentEncryptionKey string

	if ik.UUID == "" {
		panic("in EncryptItem with invalid items key (missing UUID)")
	}

	ikid := ik.UUID

	encryptedItem.ItemsKeyID = &ikid
	contentEncryptionKey = ik.ItemsKey
	encryptedItem.UUID = item.GetUUID()
	encryptedItem.ContentType = item.GetContentType()
	encryptedItem.UpdatedAt = item.GetUpdatedAt()
	encryptedItem.CreatedAt = item.GetCreatedAt()
	encryptedItem.Deleted = item.IsDeleted()
	encryptedItem.UpdatedAtTimestamp = item.GetUpdatedAtTimestamp()
	encryptedItem.CreatedAtTimestamp = item.GetCreatedAtTimestamp()
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

	var authData string
	if item.GetContentType() == "SN|ItemsKey" {
		authData = "{\"kp\":{\"identifier\":\"" + session.KeyParams.Identifier + "\",\"pw_nonce\":\"" + session.KeyParams.PwNonce + "\",\"version\":\"" + session.KeyParams.Version + "\",\"origination\":\"" + session.KeyParams.Origination + "\",\"created\",\"" + session.KeyParams.Created + "\"},\"u\":\"" + item.GetUUID() + "\",\"v\":\"" + session.KeyParams.Version + "\"}"
	} else {
		authData = "{\"u\":\"" + item.GetUUID() + "\",\"v\":\"004\"}"
	}

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

	// encrypt content encryption key
	var encryptedContentKey string
	encryptedContentKey, err = encryptString(itemEncryptionKey, contentEncryptionKey, nonce, b64AuthData)
	encItemKey := fmt.Sprintf("004:%s:%s:%s", nonce, encryptedContentKey, b64AuthData)
	encryptedItem.EncItemKey = encItemKey

	return encryptedItem, err
}

type AuthData struct {
	Kp struct {
		Identifier  string `json:"identifier"`
		PwNonce     string `json:"pw_nonce"`
		Version     string `json:"version"`
		Origination string `json:"origination"`
		Created     string `json:"created"`
	} `json:"kp"`

	U string `json:"u"`
	V string `json:"v"`
}

// decryptItemsKeys takes the master key and a list of EncryptedItemKeys
// and returns a list of items keys.
func DecryptAndParseItemKeys(mk string, eiks EncryptedItems) (iks []ItemsKey, err error) {
	for _, eik := range eiks {
		if eik.ContentType != "SN|ItemsKey" {
			continue
		}
		// decrypt enc_item_key
		_, encNonce, cipherText, authenticatedData := splitContent(eik.EncItemKey)
		var pt []byte

		pt, err = decryptString(cipherText, mk, encNonce, authenticatedData)
		if err != nil {
			err = fmt.Errorf("DecryptAndParseItemKeys: %w", err)

			return
		}

		dct, e1 := base64.StdEncoding.DecodeString(authenticatedData)
		if e1 != nil {
			panic(e1)
		}

		var ad AuthData

		err = json.Unmarshal(dct, &ad)
		if err != nil {
			return
		}

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
		f.UpdatedAtTimestamp = eik.UpdatedAtTimestamp
		f.CreatedAtTimestamp = eik.CreatedAtTimestamp
		f.CreatedAt = eik.CreatedAt

		if f.ItemsKey == "" {
			continue
		}

		iks = append(iks, f)
	}

	return
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

func DecryptItems(eis EncryptedItems, s *Session, ik ItemsKey) (dis DecryptedItems, err error) {
	for x := range eis {
		var di DecryptedItem
		di, err = DecryptItem(eis[x], s, ik)
		if err != nil {
			return
		}

		dis = append(dis, di)
	}

	return
}

func DecryptItem(e EncryptedItem, s *Session, ik ItemsKey) (o DecryptedItem, err error) {
	debugPrint(s.Debug, fmt.Sprintf("Decrypt | decrypting %s %s", e.ContentType, e.UUID))

	if e.Deleted {
		err = fmt.Errorf(fmt.Sprintf("cannot decrypt deleted item: %s %s", e.ContentType, e.UUID))

		return
	}

	var contentEncryptionKey string

	switch {
	case ik.ItemsKey != "":
		// passed when re-encrypting an export
		contentEncryptionKey = ik.ItemsKey
	case e.ItemsKeyID == nil && e.ContentType != "SN|ItemsKey":
		err = fmt.Errorf(fmt.Sprintf("cannot decrypt item without items key id specified: %s %s", e.ContentType, e.UUID))

		return
	case isEncryptedWithMasterKey(e.ContentType):
		contentEncryptionKey = s.MasterKey
	default:
		if e.ItemsKeyID == nil {
			debugPrint(s.Debug, fmt.Sprintf("decryptItems | missing ItemsKeyID for content type: %s", e.ContentType))
			err = fmt.Errorf("encountered deleted: %t item %s of type %s without ItemsKeyID", e.Deleted, e.UUID, e.ContentType)
			return
		}

		contentEncryptionKey = getMatchingItem(*e.ItemsKeyID, s.ItemsKeys).ItemsKey
		if contentEncryptionKey == "" {
			err = fmt.Errorf("deleted: %t item %s of type %s cannot be decrypted as we're missing ItemsKey %s", e.Deleted, e.UUID, e.ContentType, *e.ItemsKeyID)
			return
		}
	}

	version, nonce, cipherText, authData := splitContent(e.EncItemKey)
	if version != "004" {
		err = fmt.Errorf("your account contains an item (uuid: \"%s\" type: \"%s\" encryption version: \"%s\") encrypted with an earlier version of Standard Notes\nto upgrade your encryption, perform a backup and restore via the official app", e.UUID, e.ContentType, version)
		return
	}

	var itemKey []byte

	itemKey, err = decryptString(cipherText, contentEncryptionKey, nonce, authData)
	if err != nil {
		return
	}

	_, nonce, cipherText, authData = splitContent(e.Content)

	var content []byte

	content, err = decryptString(cipherText, string(itemKey), nonce, authData)
	if err != nil {
		return
	}

	var di DecryptedItem
	di.UUID = e.UUID
	di.ContentType = e.ContentType
	di.Deleted = e.Deleted

	if e.ItemsKeyID != nil {
		di.ItemsKeyID = *e.ItemsKeyID
	}

	di.UpdatedAt = e.UpdatedAt
	di.CreatedAt = e.CreatedAt
	di.CreatedAtTimestamp = e.CreatedAtTimestamp
	di.UpdatedAtTimestamp = e.UpdatedAtTimestamp
	di.Content = string(content)

	return di, err
}

// Decrypt.
func (ei EncryptedItems) Decrypt(s *Session, ik ItemsKey) (o DecryptedItems, err error) {
	debugPrint(s.Debug, fmt.Sprintf("Decrypt | decrypting %d items", len(ei)))

	for _, e := range ei {
		if e.Deleted {
			continue
		}

		var contentEncryptionKey string

		switch {
		case ik.ItemsKey != "":
			// passed when re-encrypting an export
			contentEncryptionKey = ik.ItemsKey
		case e.ItemsKeyID == nil && e.ContentType != "SN|ItemsKey":
			continue
		case isEncryptedWithMasterKey(e.ContentType):
			contentEncryptionKey = s.MasterKey
		default:
			if e.ItemsKeyID == nil {
				debugPrint(s.Debug, fmt.Sprintf("decryptItems | missing ItemsKeyID for content type: %s", e.ContentType))
				err = fmt.Errorf("encountered deleted: %t item %s of type %s without ItemsKeyID", e.Deleted, e.UUID, e.ContentType)
				return
			}

			contentEncryptionKey = getMatchingItem(*e.ItemsKeyID, s.ItemsKeys).ItemsKey
			if contentEncryptionKey == "" {
				err = fmt.Errorf("deleted: %t item %s of type %s cannot be decrypted as we're missing ItemsKey %s", e.Deleted, e.UUID, e.ContentType, *e.ItemsKeyID)
				return
			}
		}

		version, nonce, cipherText, authData := splitContent(e.EncItemKey)
		if version != "004" {
			err = fmt.Errorf("your account contains an item (uuid: \"%s\" type: \"%s\" encryption version: \"%s\") encrypted with an earlier version of Standard Notes\nto upgrade your encryption, perform a backup and restore via the official app", e.UUID, e.ContentType, version)
			return
		}

		var itemKey []byte

		itemKey, err = decryptString(cipherText, contentEncryptionKey, nonce, authData)
		if err != nil {
			return
		}

		_, nonce, cipherText, authData = splitContent(e.Content)

		var content []byte

		content, err = decryptString(cipherText, string(itemKey), nonce, authData)
		if err != nil {
			return
		}

		var di DecryptedItem
		di.UUID = e.UUID
		di.ContentType = e.ContentType
		di.Deleted = e.Deleted

		if e.ItemsKeyID != nil {
			di.ItemsKeyID = *e.ItemsKeyID
		}

		di.UpdatedAt = e.UpdatedAt
		di.CreatedAt = e.CreatedAt
		di.CreatedAtTimestamp = e.CreatedAtTimestamp
		di.UpdatedAtTimestamp = e.UpdatedAtTimestamp
		di.Content = string(content)
		o = append(o, di)
	}

	return
}

func encryptItems(decItems *Items, ik ItemsKey, debug bool) (encryptedItems EncryptedItems, err error) {
	debugPrint(debug, fmt.Sprintf("encryptItems | encrypting %d items", len(*decItems)))
	d := *decItems

	for _, decItem := range d {
		var e EncryptedItem
		e, err = EncryptItem(decItem, ik, nil)
		encryptedItems = append(encryptedItems, e)
	}

	return
}

func encryptString(plainText, key, nonce, authenticatedData string) (result string, err error) {
	// TODO: expecting authenticatedData to be pre base64 encoded?
	if len(nonce) == 0 {
		panic("empty nonce")
	}

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

func unmarshallSyncResponse(input []byte) (output syncResponse, err error) {
	// TODO: There should be an IsValid method on each item that includes this check if SN|ItemsKey
	err = json.Unmarshal(input, &output)
	if err != nil {
		return
	}

	// check no items keys have an items key
	for _, item := range output.Items {
		if item.ContentType == "SN|ItemsKey" && item.ItemsKeyID != nil {
			err = fmt.Errorf("SN|ItemsKey %s has an ItemsKeyID set", item.UUID)
			return
		}
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
