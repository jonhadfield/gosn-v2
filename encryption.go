package gosn

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	crand "crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/pbkdf2"
)

func unPad(cipherText []byte) []byte {
	c := cipherText[len(cipherText)-1]
	n := int(c)

	return cipherText[:len(cipherText)-n]
}

func decryptString(stringToDecrypt, encryptionKey, authKey, uuid string) (output string, err error) {
	components := strings.Split(stringToDecrypt, ":")
	version := components[0]
	authHash := components[1]
	localUUID := components[2]
	IV := components[3]
	cipherText := components[4]

	if components[2] != uuid {
		err = fmt.Errorf("aborting as uuid in string to decrypt: \"%s\" is not equal to passed uuid: \"%s\"",
			localUUID, uuid)
		return
	}

	stringToAuth := fmt.Sprintf("%s:%s:%s:%s", version, localUUID, IV, cipherText)

	var deHexedAuthKey []byte

	deHexedAuthKey, err = hex.DecodeString(authKey)
	if err != nil {
		return
	}

	localAuthHasher := hmac.New(sha256.New, deHexedAuthKey)

	_, err = localAuthHasher.Write([]byte(stringToAuth))
	if err != nil {
		return
	}

	localAuthHash := hex.EncodeToString(localAuthHasher.Sum(nil))

	if localAuthHash != authHash {
		err = fmt.Errorf("auth hash does not match. possible tampering or server issue")
		return
	}

	var deHexedEncKey []byte

	deHexedEncKey, err = hex.DecodeString(encryptionKey)
	if err != nil {
		return
	}

	var aesCipher cipher.Block

	aesCipher, err = aes.NewCipher(deHexedEncKey)
	if err != nil {
		return
	}

	unHexedIv, _ := hex.DecodeString(IV)
	mode := cipher.NewCBCDecrypter(aesCipher, unHexedIv)

	var b64DecodedCipherText []byte

	b64DecodedCipherText, err = base64.StdEncoding.DecodeString(cipherText)
	if err != nil {
		return
	}

	mode.CryptBlocks(b64DecodedCipherText, b64DecodedCipherText)

	b64DecodedCipherText = unPad(b64DecodedCipherText)

	output = string(b64DecodedCipherText)

	return output, err
}

func encryptString(stringToEncrypt, encryptionKey, authKey, uuid string, IVOverride []byte) (result string, err error) {
	bytesToEncrypt := []byte(stringToEncrypt)
	bytesToEncrypt = padToAESBlockSize(bytesToEncrypt)

	// hex decode encryption key
	var deHexedEncKey []byte

	deHexedEncKey, err = hex.DecodeString(encryptionKey)
	if err != nil {
		return
	}

	var IV []byte
	if IVOverride != nil {
		IV = IVOverride
	} else {
		IV = make([]byte, 16)
		_, err = crand.Read(IV)
		if err != nil {
			return
		}
	}

	// create cipher block
	var aesCipher cipher.Block

	aesCipher, err = aes.NewCipher(deHexedEncKey)
	if err != nil {
		return
	}

	cipherText := make([]byte, len(bytesToEncrypt))

	mode := cipher.NewCBCEncrypter(aesCipher, IV)
	mode.CryptBlocks(cipherText, bytesToEncrypt)
	b64EncodedCipher := base64.StdEncoding.EncodeToString(cipherText)
	cipherText = []byte(b64EncodedCipher)

	var deHexedAuthKey []byte

	deHexedAuthKey, err = hex.DecodeString(authKey)
	if err != nil {
		return
	}

	IVString := hex.EncodeToString(IV)

	stringToAuth := fmt.Sprintf("003:%s:%s:%s", uuid, IVString, string(cipherText))

	localAuthHasher := hmac.New(sha256.New, deHexedAuthKey)

	_, err = localAuthHasher.Write([]byte(stringToAuth))
	if err != nil {
		return
	}

	localAuthHash := hex.EncodeToString(localAuthHasher.Sum(nil))

	result = fmt.Sprintf("003:%s:%s:%s:%s", localAuthHash, uuid, IVString, cipherText)

	return result, err
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
	hashedPassword := pbkdf2.Key([]byte(input.userPassword), []byte(string(salt)), int(input.PasswordCost), 96, sha512.New)
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

func encryptItems(decItems *Items, mk, ak string, debug bool) (encryptedItems EncryptedItems, err error) {
	debugPrint(debug, fmt.Sprintf("encryptItems | encrypting %d items", len(*decItems)))
	d := *decItems
	for _, decItem := range d {
		var e EncryptedItem
		e, err = encryptItem(decItem, mk, ak)
		encryptedItems = append(encryptedItems, e)
	}

	return
}

func encryptItem(item Item, mk, ak string) (encryptedItem EncryptedItem, err error) {
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
	// get Item Auth Key
	itemAuthKey := itemKey[len(itemKey)/2:]
	// encrypt Item content
	var encryptedContent string

	mContent, _ := json.Marshal(item.GetContent())

	encryptedContent, err = encryptString(string(mContent), itemEncryptionKey, itemAuthKey, item.GetUUID(), nil)
	if err != nil {
		return
	}

	encryptedItem.Content = encryptedContent

	var encryptedKey string

	encryptedKey, err = encryptString(itemKey, mk, ak, item.GetUUID(), nil)
	if err != nil {
		return
	}

	encryptedItem.EncItemKey = encryptedKey
	encryptedItem.UUID = item.GetUUID()
	encryptedItem.ContentType = item.GetContentType()

	return encryptedItem, err
}
