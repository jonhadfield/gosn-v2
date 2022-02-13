package gosn

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCreateItemsKeyEncryptDecryptSync(t *testing.T) {
	defer cleanup()

	s := testSession
	ik := NewItemsKey()
	require.False(t, ik.Deleted)
	require.NotEmpty(t, ik.ItemsKey)
	time.Sleep(time.Millisecond * 1)
	require.Greater(t, time.Now().UTC().UnixMicro(), ik.CreatedAtTimestamp)

	note, _ := NewNote("Note Title", "Note Text", nil)

	eItem, err := EncryptItem(&note, ik, s)
	require.NoError(t, err)
	require.NotEmpty(t, eItem.ItemsKeyID)
	require.Equal(t, "Note", eItem.ContentType)

	s.DefaultItemsKey = ik
	s.ItemsKeys = []ItemsKey{ik}
	di, err := DecryptAndParseItem(eItem, s)
	require.NoError(t, err)
	require.NotEmpty(t, di.GetUUID())

	dn := di.(*Note)

	require.Equal(t, note.Content.Title, dn.Content.Title)
	require.Equal(t, note.Content.Text, dn.Content.Text)

	eik, err := ik.Encrypt(testSession, true)
	require.NoError(t, err)
	require.NotEmpty(t, eik.Content)

	eItems := EncryptedItems{eItem, eik}

	require.Len(t, eItems, 2)

	so, err := Sync(SyncInput{
		Session: testSession,
		Items:   eItems,
	})
	require.NoError(t, err)
	require.Equal(t, 2, len(so.SavedItems))

	var foundKey bool

	var foundNote bool

	var noteIndex int

	for x := range so.SavedItems {
		if so.SavedItems[x].ContentType == "SN|ItemsKey" {
			require.Equal(t, ik.UUID, so.SavedItems[x].UUID)
			require.Less(t, int64(0), so.SavedItems[x].UpdatedAtTimestamp)

			foundKey = true
		}

		if so.SavedItems[x].ContentType == "Note" {
			noteIndex = x
			require.Equal(t, note.UUID, so.SavedItems[x].UUID)
			require.Less(t, int64(0), so.SavedItems[x].UpdatedAtTimestamp)

			foundNote = true
		}
	}

	require.True(t, foundKey)
	require.True(t, foundNote)

	var foundSIK bool

	var numDefaults int

	for x := range testSession.ItemsKeys {
		if testSession.ItemsKeys[x].UUID == ik.UUID {
			if testSession.ItemsKeys[x].Default == true {
				numDefaults++
			}

			foundSIK = true
		}
	}

	require.True(t, foundSIK)
	require.Equal(t, 1, numDefaults)
	require.Equal(t, ik.UUID, testSession.DefaultItemsKey.UUID)
	require.Equal(t, ik.ItemsKey, testSession.DefaultItemsKey.ItemsKey)
	require.Equal(t, "Note", so.SavedItems[noteIndex].ContentType)

	_, err = DecryptAndParseItem(so.SavedItems[noteIndex], testSession)
	require.NoError(t, err)
}

func TestCreateItemsKeyEncryptDecryptItem(t *testing.T) {
	defer cleanup()

	s := testSession
	ik := NewItemsKey()
	require.False(t, ik.Deleted)
	require.NotEmpty(t, ik.ItemsKey)
	time.Sleep(time.Millisecond * 1)
	require.Greater(t, time.Now().UTC().UnixMicro(), ik.CreatedAtTimestamp)

	note, _ := NewNote("Note Title", "Note Text", nil)

	items := Items{&note}
	eItems, err := items.Encrypt(s, ik)
	require.NoError(t, err)
	require.Len(t, eItems, 1)

	s.DefaultItemsKey = ik
	s.ItemsKeys = []ItemsKey{ik}
	di, err := eItems.DecryptAndParse(s)
	require.NoError(t, err)
	require.Len(t, di, 1)
	dn := di[0].(*Note)
	require.Equal(t, note.Content.Title, dn.Content.Title)
	require.Equal(t, note.Content.Text, dn.Content.Text)

	eik, err := s.DefaultItemsKey.Encrypt(testSession, true)
	require.NoError(t, err)
	so, err := Sync(SyncInput{
		Session: testSession,
		Items:   EncryptedItems{eik},
	})
	require.NoError(t, err)
	require.Equal(t, 1, len(so.SavedItems))
}

func TestEncryptDecryptOfItemsKey(t *testing.T) {
	s := testSession
	ik, err := s.CreateItemsKey()
	require.NoError(t, err)
	require.Equal(t, "SN|ItemsKey", ik.ContentType)
	require.NotEmpty(t, ik.ItemsKey)

	eik, err := ik.Encrypt(testSession, true)
	require.NoError(t, err)
	require.Equal(t, "SN|ItemsKey", eik.ContentType)
	require.NotEmpty(t, eik.EncItemKey)

	dik, err := DecryptAndParseItemKeys(s.MasterKey, []EncryptedItem{eik})
	require.NoError(t, err)
	require.Len(t, dik, 1)
	require.Greater(t, len(dik[0].ItemsKey), 0)
	require.Equal(t, ik.ItemsKey, dik[0].ItemsKey)
	require.NotZero(t, dik[0].CreatedAtTimestamp)
	require.Equal(t, ik.CreatedAtTimestamp, dik[0].CreatedAtTimestamp)
	require.Equal(t, ik.UpdatedAtTimestamp, dik[0].UpdatedAtTimestamp)
	require.Equal(t, ik.CreatedAt, dik[0].CreatedAt)
	require.Equal(t, ik.UpdatedAt, dik[0].UpdatedAt)
}

func TestDecryptString(t *testing.T) {
	rawKey := "e73faf921cc265b7a001451d8760a6a6e2270d0dbf1668f9971fd75c8018ffd4"
	cipherText := "kRd2w+7FQBIXaNGze7G28GOIUSngrqtx/t5Jus76z3z+eM18GkJT7Lc/ZpqJiH9I6fdksNdo6uvfip8TCIT458XxcrqIP24Bxk9xaz2Q9IQ="
	nonce := "d211fc5dee400fe54ca04ac43ecac512c9d0dabb6c4ee0f3"
	authData := "eyJrcCI6eyJpZGVudGlmaWVyIjoiZ29zbi12MkBsZXNza25vd24uY28udWsiLCJwd19ub25jZSI6ImIzYjc3Yzc5YzlmZWE5ODY3MWU2NmFmNDczMzZhODhlNWE1MTUyMjI4YjEwMTQ2NDEwM2M1MjJiMWUzYWU0ZGEiLCJ2ZXJzaW9uIjoiMDA0Iiwib3JpZ2luYXRpb24iOiJyZWdpc3RyYXRpb24iLCJjcmVhdGVkIjoiMTYwODEzNDk0NjY5MiJ9LCJ1IjoiNjI3YTg4YTAtY2NkNi00YTY4LWFjZWUtYjM0ODQ5NDZmMjY1IiwidiI6IjAwNCJ9"
	plainText, err := DecryptCipherText(cipherText, rawKey, nonce, authData)

	require.NoError(t, err)
	require.Equal(t, "9381f4ac4371cd9e31c3389442897d5c7de3da3d787927709ab601e28767d18a", string(plainText))
}

func TestDecryptItemKey(t *testing.T) {
	// decrypt encrypted item key
	rawKey := "e73faf921cc265b7a001451d8760a6a6e2270d0dbf1668f9971fd75c8018ffd4"
	cipherText := "kRd2w+7FQBIXaNGze7G28GOIUSngrqtx/t5Jus76z3z+eM18GkJT7Lc/ZpqJiH9I6fdksNdo6uvfip8TCIT458XxcrqIP24Bxk9xaz2Q9IQ="
	nonce := "d211fc5dee400fe54ca04ac43ecac512c9d0dabb6c4ee0f3"
	authData := "eyJrcCI6eyJpZGVudGlmaWVyIjoiZ29zbi12MkBsZXNza25vd24uY28udWsiLCJwd19ub25jZSI6ImIzYjc3Yzc5YzlmZWE5ODY3MWU2NmFmNDczMzZhODhlNWE1MTUyMjI4YjEwMTQ2NDEwM2M1MjJiMWUzYWU0ZGEiLCJ2ZXJzaW9uIjoiMDA0Iiwib3JpZ2luYXRpb24iOiJyZWdpc3RyYXRpb24iLCJjcmVhdGVkIjoiMTYwODEzNDk0NjY5MiJ9LCJ1IjoiNjI3YTg4YTAtY2NkNi00YTY4LWFjZWUtYjM0ODQ5NDZmMjY1IiwidiI6IjAwNCJ9"
	contentItemKeyHexBytes, err := DecryptCipherText(cipherText, rawKey, nonce, authData)
	require.NoError(t, err)
	require.Equal(t, "9381f4ac4371cd9e31c3389442897d5c7de3da3d787927709ab601e28767d18a", string(contentItemKeyHexBytes))

	// decrypt item key content with item key
	rawKey = string(contentItemKeyHexBytes)
	nonce = "b0a6519e605db5ecf02cdc225567b61d41f56b41387bc95e"
	cipherText = "GHWwyAayZuu5BKLbHScaJ2e8turXbbcnkNGrmTr9alLQen9UyNRjOtKNH1WcfNb3/kkqabw8XwNxKwrrQwBZmC1wVkIvJpEQc0oI7Nc9F3zHVJyiHqFc8mWRs2jWY+/3IdWm6TTTiJro+QTzFjO5XO9J8KwAx1LizaScjKdTE20p+ryRrrfpp5x8YbbuIWLxpOZRJfF0zUe7wAo/SCI/VuIvSrTK9958VgvPzTagse644pjSo/yvcaSv5XUJhfvaBeqK0JLwiNvNmYZHXt1itfHRE1BFi6/T0fkA30VQb8JmHyHU"
	authData = "eyJrcCI6eyJpZGVudGlmaWVyIjoiZ29zbi12MkBsZXNza25vd24uY28udWsiLCJwd19ub25jZSI6ImIzYjc3Yzc5YzlmZWE5ODY3MWU2NmFmNDczMzZhODhlNWE1MTUyMjI4YjEwMTQ2NDEwM2M1MjJiMWUzYWU0ZGEiLCJ2ZXJzaW9uIjoiMDA0Iiwib3JpZ2luYXRpb24iOiJyZWdpc3RyYXRpb24iLCJjcmVhdGVkIjoiMTYwODEzNDk0NjY5MiJ9LCJ1IjoiNjI3YTg4YTAtY2NkNi00YTY4LWFjZWUtYjM0ODQ5NDZmMjY1IiwidiI6IjAwNCJ9"
	dIKeyContent, err := DecryptCipherText(cipherText, rawKey, nonce, authData)
	require.NoError(t, err)

	var ik ItemsKey
	err = json.Unmarshal(dIKeyContent, &ik)
	require.NoError(t, err)
	require.Equal(t, "366df581a789de771a1613d7d0289bbaff7bf4249a7dd15e458a12c361cb7b73", ik.ItemsKey)
}

func TestDecryptNoteText(t *testing.T) {
	// decrypt encrypted items key
	decryptedItemsKey := "366df581a789de771a1613d7d0289bbaff7bf4249a7dd15e458a12c361cb7b73"
	cipherText := "sJhGyLDN4x/wXBcE6TWCsZMaAPfK04ojpsYzjI/zEGvkBsRPGPyihTyQGHvAqcHMWOIZZYZDC2+8YlxVdreF2LblOM8hXz3hwtFDE3DcN5g="
	nonce := "b55df872abe8c97f82bb875a14a9b344584825edef1d0ed7"
	authData := "eyJ1IjoiYmE5MjQ4YWMtOWUxNC00ODcyLTgxNjYtNTkzMjg5ZDg5ODYwIiwidiI6IjAwNCJ9"
	contentItemKeyHexBytes, err := DecryptCipherText(cipherText, decryptedItemsKey, nonce, authData)
	require.NoError(t, err)
	require.Equal(t, "b396412f690bfb40801c764af7975bc019f3de79b1ed24385e98787aff81c003", string(contentItemKeyHexBytes))

	rawKey := string(contentItemKeyHexBytes)
	nonce = "6045eaf9774a877203b68bb12159f9c5c0c3d19df4949e40"
	cipherText = "B+8vUwmSTGZCba6mU2gMSMl55fpt38Wv/yWxAF4pEveX0sjqSYgjT5PA8/yy7LKotF+kjmuiHNvYtH7hB7BaqJrG8Q4G5Sj15tIu8PtlWECJWHnPxHkeiJW1MiS1ypR0t3y+Uc7cRpGPwnQIqJDr/Yl1vp2tZXlaSy0zYtGYlw5GwUnLxXtQBQC1Ml3rzZDpaIT9zIr9Qluv7Q7JXOJ7rAbj95MtsV2CJDS33+kXBTUKMqYRbGDWmn0="
	authData = "eyJ1IjoiYmE5MjQ4YWMtOWUxNC00ODcyLTgxNjYtNTkzMjg5ZDg5ODYwIiwidiI6IjAwNCJ9"
	dIKeyContent, err := DecryptCipherText(cipherText, rawKey, nonce, authData)
	require.NoError(t, err)

	sDiKeyContent := string(dIKeyContent)
	ikc := NoteContent{}
	err = json.Unmarshal([]byte(sDiKeyContent), &ikc)
	require.NoError(t, err)
	require.Equal(t, "Note Title", ikc.Title)
	require.Equal(t, "Note Text", ikc.Text)
}

func TestEncryptDecryptString(t *testing.T) {
	rawKey := "b396412f690bfb40801c764af7975bc019f3de79b1ed24385e98787aff81c003"
	tempExpectedCipherText := "B+8vUwmSTGZCba6mU2gMSMl55fpt38Wv/yWxAF4pEveX0sjqSYgjT5PA8/yy7LKotF+kjmuiHNvYtH7hB7BaqJrG8Q4G5Sj15tIu8PtlWECJWHnPxHkeiJW1MiS1ypR0t3y+Uc7cRpGPwnQIqJDr/Yl1vp2tZXlaSy0zYtGYlw5GwUnLxXtQBQC1Ml3rzZDpaIT9zIr9Qluv7Q7JXOJ7rAbj95MtsV2CJD4RwjhhJ11fpI3N8+uXqp4="

	nonce := "6045eaf9774a877203b68bb12159f9c5c0c3d19df4949e40"
	plainText := "{\"text\":\"Note Text\",\"title\":\"Note Title\",\"references\":[],\"appData\":{\"org.standardnotes.sn\":{\"client_updated_at\":\"2021-03-20T12:59:46.734Z\"}},\"preview_plain\":\"Note Text\"}"
	uuid := "7eacf350-f4ce-44dd-8525-2457b19047dd"
	authData := "{\"u\":\"" + uuid + "\",\"v\":\"004\"}"
	newCipherText, err := encryptString(plainText, rawKey, nonce, authData, 32)
	require.NoError(t, err)
	require.NotEmpty(t, newCipherText)
	require.Equal(t, tempExpectedCipherText, newCipherText)

	var ptb []byte
	ptb, err = DecryptCipherText(newCipherText, rawKey, nonce, authData)
	require.NoError(t, err)
	require.Equal(t, plainText, string(ptb))
}
