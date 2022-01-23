package gosn

import (
	"strings"

	"github.com/google/uuid"
)

func stripLineBreak(input string) string {
	if strings.HasSuffix(input, "\n") {
		return input[:len(input)-1]
	}

	return input
}

// GenUUID generates a unique identifier required when creating a new item.
func GenUUID() string {
	newUUID := uuid.New()
	return newUUID.String()
}

func stringInSlice(inStr string, inSlice []string, matchCase bool) bool {
	for i := range inSlice {
		if matchCase && inStr == inSlice[i] {
			return true
		} else if strings.EqualFold(inStr, inSlice[i]) {
			return true
		}
	}

	return false
}

func DeleteContent(session *Session) (deleted int, err error) {
	si := SyncInput{
		Session: session,
	}

	var so SyncOutput

	so, err = Sync(si)
	if err != nil {
		return
	}

	var itemsToPut EncryptedItems

	for _, item := range so.Items {
		if stringInSlice(item.ContentType, []string{"Note", "Tag", "SN|Component"}, true) {
			item.Deleted = true
			itemsToPut = append(itemsToPut, item)
		}
	}

	si.Items = itemsToPut

	_, err = Sync(si)

	return len(itemsToPut), err
}
