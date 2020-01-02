package gosn

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEncryptString003(t *testing.T) {
	stringToEncrypt := `{"title":"tagOne","references":[{"uuid":"a5bd62b0-609c-4152-88f9-9d55f5f490f7","content_type":"Note"},{"uuid":"d19be4ac-3cf4-47b9-986f-26d67e9e2b83","content_type":"Note"},{"uuid":"44cc3bc4-c0e9-11e8-86d5-acde48001122","content_type":"Note"}]}`
	encryptionKey := "8b82accf2bae6b1f1183d5398dc46bbb8bc71f019c43e105fef21846ffe7b6be"
	authKey := "aca431d6fc360e46e853e19d70afec26e4825e2609c2e6df9f4431cbd344e1bc"
	uuid := "fa9d5b81-7b2d-4d9b-988d-db09cee3f9ec"
	IV := []byte{181, 126, 90, 99, 56, 249, 177, 105, 14, 215, 154, 75, 62, 86, 66, 227}
	expectedCipher := "003:6d6c7ee899ba8aafa909bf7a13493cf0acad54eaba3963f82ffb4ff15cdd321e:fa9d5b81-7b2d-4d9b-988d-db09cee3f9ec:b57e5a6338f9b1690ed79a4b3e5642e3:5+mzQl3fkiMvJnuu/7nV4mCnJ6bHOn2LJEUHt+TO4vK4s4X3GNly+OAsiycJQ1B72Z+rkWB+TK6YRyiXMoVpDh9R1i+78ZH2wCCzIVZDcihIvY4kzFdr8UuAe7y0nl2GKVAfJv0y+2khZf7/3cwic2HYlwPXEnMdRQWC4vvGh8a0MkBI08uShLF7cmYdhjsBG8DIduh3GSGy8PtY2+iMj+Y6zMbCcHXTZcARuMi+ReNDDY6mnfw6PV0i+FQGj6VE1jQRkhBhKZE4NgL1H79xtKFAwkrWU9Cv2EcFJE0JGHyJV4xvMbFwzFXhcC6xE8aBD7tZ7NhDE+Il1kBIli5QdQ=="

	result, err := encryptString(stringToEncrypt, encryptionKey, authKey, uuid, IV)
	assert.Nil(t, err, err)
	assert.Equal(t, result, expectedCipher, fmt.Sprintf("expected: %s res: %s", expectedCipher, result))
}

func TestDecryptString003(t *testing.T) {
	stringToDecrypt := "003:46b10a5ea73cad1b1252dcbc6abbd616d8ba7ddce359930098506a8e71aea2db:277613b2-f1df-4e95-985f-d23a08172e52:255fa21e7c781e7d904f95764f8c60a9:niiUni0ckcVTsFN+mt7UxwL62nWI9ctn8CtyFisAOG3cF28OVQlQv1QWf9d2wjMFEIiUAEgWzbujbOES0g8Am86/FbSJyvFM6Wp+ox04mhS0cFhCVbjV7wt7yfdf6bc+e2Dx9lvBoP19IOFrBgohCe2yKw3VCSuDsk6IsNWR725oSo6BHHmJSSs+RMEOB+5lpoRIziU0SXWemK3s//NCjNCNA/lkWM3Ry9Vr13kpYXIYmehHfb+cPgKbAB3LiNBxJCQ8MWJbUlBmn3PY1ExIhVzQCnPqwwZ0+pX0+YJXd/lgB5DsYbEWjLAChZWPFid1c9z8e7sad4mXuyiAyKcYsA=="
	encryptionKey := "32bf6c2eceb0a875a17390f34feba0386c641c74d35fb29112a7be4a21cbf974"
	authKey := "530fb9cae9586177d4a00c32332dc151f87a97c9d04cdda6c28e70c2ef747a3f"
	uuid := "277613b2-f1df-4e95-985f-d23a08172e52"
	expectedText := `{"title":"tagOne","references":[{"uuid":"a5bd62b0-609c-4152-88f9-9d55f5f490f7","content_type":"Note"},{"uuid":"d19be4ac-3cf4-47b9-986f-26d67e9e2b83","content_type":"Note"},{"uuid":"44cc3bc4-c0e9-11e8-86d5-acde48001122","content_type":"Note"}]}`

	result, err := decryptString(stringToDecrypt, encryptionKey, authKey, uuid)
	assert.Nil(t, err, err)
	assert.Equal(t, result, expectedText, fmt.Sprintf("expected: %s res: %s", expectedText, result))
}
