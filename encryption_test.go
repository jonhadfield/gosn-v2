package gosn

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDecryptString(t *testing.T) {
	rawKey := "e73faf921cc265b7a001451d8760a6a6e2270d0dbf1668f9971fd75c8018ffd4"
	cipherText := "kRd2w+7FQBIXaNGze7G28GOIUSngrqtx/t5Jus76z3z+eM18GkJT7Lc/ZpqJiH9I6fdksNdo6uvfip8TCIT458XxcrqIP24Bxk9xaz2Q9IQ="
	nonce := "d211fc5dee400fe54ca04ac43ecac512c9d0dabb6c4ee0f3"
	authData := "eyJrcCI6eyJpZGVudGlmaWVyIjoiZ29zbi12MkBsZXNza25vd24uY28udWsiLCJwd19ub25jZSI6ImIzYjc3Yzc5YzlmZWE5ODY3MWU2NmFmNDczMzZhODhlNWE1MTUyMjI4YjEwMTQ2NDEwM2M1MjJiMWUzYWU0ZGEiLCJ2ZXJzaW9uIjoiMDA0Iiwib3JpZ2luYXRpb24iOiJyZWdpc3RyYXRpb24iLCJjcmVhdGVkIjoiMTYwODEzNDk0NjY5MiJ9LCJ1IjoiNjI3YTg4YTAtY2NkNi00YTY4LWFjZWUtYjM0ODQ5NDZmMjY1IiwidiI6IjAwNCJ9"
	plainText, err := decryptString(cipherText, rawKey, nonce, authData)

	require.NoError(t, err)
	require.Equal(t, "9381f4ac4371cd9e31c3389442897d5c7de3da3d787927709ab601e28767d18a", string(plainText))
}

func TestDecryptItemsKey(t *testing.T) {
	// decrypt encrypted item key
	rawKey := "e73faf921cc265b7a001451d8760a6a6e2270d0dbf1668f9971fd75c8018ffd4"
	cipherText := "kRd2w+7FQBIXaNGze7G28GOIUSngrqtx/t5Jus76z3z+eM18GkJT7Lc/ZpqJiH9I6fdksNdo6uvfip8TCIT458XxcrqIP24Bxk9xaz2Q9IQ="
	nonce := "d211fc5dee400fe54ca04ac43ecac512c9d0dabb6c4ee0f3"
	authData := "eyJrcCI6eyJpZGVudGlmaWVyIjoiZ29zbi12MkBsZXNza25vd24uY28udWsiLCJwd19ub25jZSI6ImIzYjc3Yzc5YzlmZWE5ODY3MWU2NmFmNDczMzZhODhlNWE1MTUyMjI4YjEwMTQ2NDEwM2M1MjJiMWUzYWU0ZGEiLCJ2ZXJzaW9uIjoiMDA0Iiwib3JpZ2luYXRpb24iOiJyZWdpc3RyYXRpb24iLCJjcmVhdGVkIjoiMTYwODEzNDk0NjY5MiJ9LCJ1IjoiNjI3YTg4YTAtY2NkNi00YTY4LWFjZWUtYjM0ODQ5NDZmMjY1IiwidiI6IjAwNCJ9"
	contentItemKeyHexBytes, err := decryptString(cipherText, rawKey, nonce, authData)
	require.NoError(t, err)
	require.Equal(t, "9381f4ac4371cd9e31c3389442897d5c7de3da3d787927709ab601e28767d18a", string(contentItemKeyHexBytes))

	// decrypt item key content with item key
	rawKey = string(contentItemKeyHexBytes)
	nonce = "b0a6519e605db5ecf02cdc225567b61d41f56b41387bc95e"
	cipherText = "GHWwyAayZuu5BKLbHScaJ2e8turXbbcnkNGrmTr9alLQen9UyNRjOtKNH1WcfNb3/kkqabw8XwNxKwrrQwBZmC1wVkIvJpEQc0oI7Nc9F3zHVJyiHqFc8mWRs2jWY+/3IdWm6TTTiJro+QTzFjO5XO9J8KwAx1LizaScjKdTE20p+ryRrrfpp5x8YbbuIWLxpOZRJfF0zUe7wAo/SCI/VuIvSrTK9958VgvPzTagse644pjSo/yvcaSv5XUJhfvaBeqK0JLwiNvNmYZHXt1itfHRE1BFi6/T0fkA30VQb8JmHyHU"
	authData = "eyJrcCI6eyJpZGVudGlmaWVyIjoiZ29zbi12MkBsZXNza25vd24uY28udWsiLCJwd19ub25jZSI6ImIzYjc3Yzc5YzlmZWE5ODY3MWU2NmFmNDczMzZhODhlNWE1MTUyMjI4YjEwMTQ2NDEwM2M1MjJiMWUzYWU0ZGEiLCJ2ZXJzaW9uIjoiMDA0Iiwib3JpZ2luYXRpb24iOiJyZWdpc3RyYXRpb24iLCJjcmVhdGVkIjoiMTYwODEzNDk0NjY5MiJ9LCJ1IjoiNjI3YTg4YTAtY2NkNi00YTY4LWFjZWUtYjM0ODQ5NDZmMjY1IiwidiI6IjAwNCJ9"
	dIKeyContent, err := decryptString(cipherText, rawKey, nonce, authData)
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
	contentItemKeyHexBytes, err := decryptString(cipherText, decryptedItemsKey, nonce, authData)
	require.NoError(t, err)
	require.Equal(t, "b396412f690bfb40801c764af7975bc019f3de79b1ed24385e98787aff81c003", string(contentItemKeyHexBytes))

	rawKey := string(contentItemKeyHexBytes)
	nonce = "6045eaf9774a877203b68bb12159f9c5c0c3d19df4949e40"
	cipherText = "B+8vUwmSTGZCba6mU2gMSMl55fpt38Wv/yWxAF4pEveX0sjqSYgjT5PA8/yy7LKotF+kjmuiHNvYtH7hB7BaqJrG8Q4G5Sj15tIu8PtlWECJWHnPxHkeiJW1MiS1ypR0t3y+Uc7cRpGPwnQIqJDr/Yl1vp2tZXlaSy0zYtGYlw5GwUnLxXtQBQC1Ml3rzZDpaIT9zIr9Qluv7Q7JXOJ7rAbj95MtsV2CJDS33+kXBTUKMqYRbGDWmn0="
	authData = "eyJ1IjoiYmE5MjQ4YWMtOWUxNC00ODcyLTgxNjYtNTkzMjg5ZDg5ODYwIiwidiI6IjAwNCJ9"
	dIKeyContent, err := decryptString(cipherText, rawKey, nonce, authData)
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
	authData := "eyJ1IjoiYmE5MjQ4YWMtOWUxNC00ODcyLTgxNjYtNTkzMjg5ZDg5ODYwIiwidiI6IjAwNCJ9"
	plainText := "{\"text\":\"Note Text\",\"title\":\"Note Title\",\"references\":[],\"appData\":{\"org.standardnotes.sn\":{\"client_updated_at\":\"2021-03-20T12:59:46.734Z\"}},\"preview_plain\":\"Note Text\"}"
	uuid := "7eacf350-f4ce-44dd-8525-2457b19047dd"
	authData = "{\"u\":\"" + uuid + "\",\"v\":\"004\"}"
	newCipherText, err := encryptString(plainText, rawKey, nonce, authData)
	require.NotEmpty(t, newCipherText)
	require.Equal(t, tempExpectedCipherText, newCipherText)

	var ptb []byte
	ptb, err = decryptString(newCipherText, rawKey, nonce, authData)
	require.NoError(t, err)
	require.Equal(t, plainText, string(ptb))
}
