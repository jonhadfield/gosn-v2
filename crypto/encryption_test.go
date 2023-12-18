package crypto

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"
)

// ### server not required for following tests.
func TestGenerateSalt004(t *testing.T) {
	t.Parallel()

	identifier := "sn004@lessknown.co.uk"
	nonce := "2c409996650e46c748856fbd6aa549f89f35be055a8f9bfacdf0c4b29b2152e9"
	decodedHex64, _ := hex.DecodeString("7129955dbbbfb376fdcac49890ef17bc")
	require.Equal(t, decodedHex64, generateSalt(identifier, nonce))
}

func TestGenerateEncryptedPasswordWithValidInput004(t *testing.T) {
	t.Parallel()

	var testInput GenerateEncryptedPasswordInput
	testInput.UserPassword = "debugtest"
	testInput.Identifier = "sn004@lessknown.co.uk"
	testInput.PasswordNonce = "2c409996650e46c748856fbd6aa549f89f35be055a8f9bfacdf0c4b29b2152e9"
	masterKey, serverPassword, err := GenerateMasterKeyAndServerPassword004(testInput)
	require.NoError(t, err)
	require.Equal(t, "2396d6ac0bc70fe45db1d2bcf3daa522603e9c6fcc88dc933ce1a3a31bbc08ed", masterKey)
	require.Equal(t, "a5eb9fbc767eafd6e54fd9d3646b19520e038ba2ccc9cceddf2340b37b788b47", serverPassword)
}

//
// func TestCreateItemsKeyEncryptDecryptSync(t *testing.T) {
// 	defer cleanup()
//
// 	s := testSession
// 	ik := NewItemsKey()
// 	fmt.Printf("ik: %+v\n", ik)
// 	require.False(t, ik.Deleted)
// 	require.NotEmpty(t, ik.ItemsKey)
// 	time.Sleep(time.Millisecond * 1)
// 	require.Greater(t, time.Now().UTC().UnixMicro(), ik.CreatedAtTimestamp)
//
// 	note, _ := NewNote("Note Title", "Note Text", nil)
//
// 	eItem, err := EncryptItem(&note, ik, s)
// 	fmt.Printf("eItem: %+v\n", eItem)
//
// 	require.NoError(t, err)
// 	require.NotEmpty(t, eItem.ItemsKeyID)
// 	require.Equal(t, SNItemTypeNote, eItem.ContentType)
//
// 	s.DefaultItemsKey = ik
// 	s.ItemsKeys = []ItemsKey{ik}
// 	di, err := DecryptAndParseItem(eItem, s)
// 	require.NoError(t, err)
// 	require.NotEmpty(t, di.GetUUID())
//
// 	dn := di.(*Note)
//
// 	require.Equal(t, note.Content.Title, dn.Content.Title)
// 	require.Equal(t, note.Content.Text, dn.Content.Text)
//
// 	eik, err := EncryptItemsKey(ik, testSession, true)
// 	require.NoError(t, err)
// 	require.NotEmpty(t, eik.Content)
//
// 	eItems := EncryptedItems{eItem, eik}
//
// 	require.Len(t, eItems, 2)
//
// 	so, err := Sync(SyncInput{
// 		Session: testSession,
// 		Items:   eItems,
// 	})
// 	require.NoError(t, err)
// 	require.Equal(t, 2, len(so.SavedItems))
//
// 	var foundKey bool
//
// 	var foundNote bool
//
// 	var noteIndex int
//
// 	for x := range so.SavedItems {
// 		if so.SavedItems[x].ContentType == common.SNItemTypeItemsKey {
// 			require.Equal(t, ik.UUID, so.SavedItems[x].UUID)
// 			require.Less(t, int64(0), so.SavedItems[x].UpdatedAtTimestamp)
//
// 			foundKey = true
// 		}
//
// 		if so.SavedItems[x].ContentType == SNItemTypeNote {
// 			noteIndex = x
// 			require.Equal(t, note.UUID, so.SavedItems[x].UUID)
// 			require.Less(t, int64(0), so.SavedItems[x].UpdatedAtTimestamp)
//
// 			foundNote = true
// 		}
// 	}
//
// 	require.True(t, foundKey)
// 	require.True(t, foundNote)
//
// 	var foundSIK bool
//
// 	var numDefaults int
//
// 	for x := range testSession.ItemsKeys {
// 		if testSession.ItemsKeys[x].UUID == ik.UUID {
// 			if testSession.ItemsKeys[x].Default == true {
// 				numDefaults++
// 			}
//
// 			foundSIK = true
// 		}
// 	}
//
// 	require.True(t, foundSIK)
// 	require.Equal(t, 1, numDefaults)
// 	require.Equal(t, ik.UUID, testSession.DefaultItemsKey.UUID)
// 	require.Equal(t, ik.ItemsKey, testSession.DefaultItemsKey.ItemsKey)
// 	require.Equal(t, SNItemTypeNote, so.SavedItems[noteIndex].ContentType)
//
// 	_, err = DecryptAndParseItem(so.SavedItems[noteIndex], testSession)
// 	require.NoError(t, err)
// }
//
// func TestCreateItemsKeyEncryptDecryptItem(t *testing.T) {
// 	defer cleanup()
//
// 	s := testSession
// 	ik := NewItemsKey()
// 	require.False(t, ik.Deleted)
// 	require.NotEmpty(t, ik.ItemsKey)
// 	time.Sleep(time.Millisecond * 1)
// 	require.Greater(t, time.Now().UTC().UnixMicro(), ik.CreatedAtTimestamp)
//
// 	note, _ := NewNote("Note Title", "Note Text", nil)
//
// 	items := Items{&note}
// 	eItems, err := items.Encrypt(s, ik)
// 	require.NoError(t, err)
// 	require.Len(t, eItems, 1)
//
// 	s.DefaultItemsKey = ik
// 	s.ItemsKeys = []ItemsKey{ik}
// 	di, err := eItems.DecryptAndParse(s)
// 	require.NoError(t, err)
// 	require.Len(t, di, 1)
// 	dn := di[0].(*Note)
// 	require.Equal(t, note.Content.Title, dn.Content.Title)
// 	require.Equal(t, note.Content.Text, dn.Content.Text)
//
// 	eik, err := EncryptItemsKey(s.DefaultItemsKey, testSession, true)
// 	require.NoError(t, err)
// 	so, err := Sync(SyncInput{
// 		Session: testSession,
// 		Items:   EncryptedItems{eik},
// 	})
// 	require.NoError(t, err)
// 	require.Equal(t, 1, len(so.SavedItems))
// }

func TestDecryptString(t *testing.T) {
	rawKey := "e73faf921cc265b7a001451d8760a6a6e2270d0dbf1668f9971fd75c8018ffd4"
	cipherText := "kRd2w+7FQBIXaNGze7G28GOIUSngrqtx/t5Jus76z3z+eM18GkJT7Lc/ZpqJiH9I6fdksNdo6uvfip8TCIT458XxcrqIP24Bxk9xaz2Q9IQ="
	nonce := "d211fc5dee400fe54ca04ac43ecac512c9d0dabb6c4ee0f3"
	authData := "eyJrcCI6eyJpZGVudGlmaWVyIjoiZ29zbi12MkBsZXNza25vd24uY28udWsiLCJwd19ub25jZSI6ImIzYjc3Yzc5YzlmZWE5ODY3MWU2NmFmNDczMzZhODhlNWE1MTUyMjI4YjEwMTQ2NDEwM2M1MjJiMWUzYWU0ZGEiLCJ2ZXJzaW9uIjoiMDA0Iiwib3JpZ2luYXRpb24iOiJyZWdpc3RyYXRpb24iLCJjcmVhdGVkIjoiMTYwODEzNDk0NjY5MiJ9LCJ1IjoiNjI3YTg4YTAtY2NkNi00YTY4LWFjZWUtYjM0ODQ5NDZmMjY1IiwidiI6IjAwNCJ9"
	plainText, err := DecryptCipherText(cipherText, rawKey, nonce, authData)

	require.NoError(t, err)
	require.Equal(t, "9381f4ac4371cd9e31c3389442897d5c7de3da3d787927709ab601e28767d18a", string(plainText))
}

func TestEncryptDecryptString(t *testing.T) {
	rawKey := "b396412f690bfb40801c764af7975bc019f3de79b1ed24385e98787aff81c003"
	tempExpectedCipherText := "B+8vUwmSTGZCba6mU2gMSMl55fpt38Wv/yWxAF4pEveX0sjqSYgjT5PA8/yy7LKotF+kjmuiHNvYtH7hB7BaqJrG8Q4G5Sj15tIu8PtlWECJWHnPxHkeiJW1MiS1ypR0t3y+Uc7cRpGPwnQIqJDr/Yl1vp2tZXlaSy0zYtGYlw5GwUnLxXtQBQC1Ml3rzZDpaIT9zIr9Qluv7Q7JXOJ7rAbj95MtsV2CJD4RwjhhJ11fpI3N8+uXqp4="

	nonce := "6045eaf9774a877203b68bb12159f9c5c0c3d19df4949e40"
	plainText := "{\"text\":\"Note Text\",\"title\":\"Note Title\",\"references\":[],\"appData\":{\"org.standardnotes.sn\":{\"client_updated_at\":\"2021-03-20T12:59:46.734Z\"}},\"preview_plain\":\"Note Text\"}"
	uuid := "7eacf350-f4ce-44dd-8525-2457b19047dd"
	authData := "{\"u\":\"" + uuid + "\",\"v\":\"004\"}"
	newCipherText, err := EncryptString(plainText, rawKey, nonce, authData, 32)
	require.NoError(t, err)
	require.NotEmpty(t, newCipherText)
	require.Equal(t, tempExpectedCipherText, newCipherText)

	var ptb []byte
	ptb, err = DecryptCipherText(newCipherText, rawKey, nonce, authData)
	require.NoError(t, err)
	require.Equal(t, plainText, string(ptb))
}
