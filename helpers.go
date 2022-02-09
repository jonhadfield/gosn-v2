package gosn

import (
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"strings"
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

// DeleteContent will remove all Notes, Tags, and Components from SN.
func DeleteContent(session *Session, everything bool) (deleted int, err error) {
	si := SyncInput{
		Session: session,
	}

	var so SyncOutput

	so, err = Sync(si)
	if err != nil {
		return
	}

	var itemsToPut EncryptedItems

	typesToDelete := []string{
		"Note",
		"Tag",
	}
	if everything {
		typesToDelete = append(typesToDelete, []string{
			"SN|Component",
			"SN|FileSafe|FileMetaData",
			"SN|FileSafe|Credentials",
			"SN|FileSafe|Integration",
			"SN|Theme",
			"SN|ExtensionRepo",
			"SN|Privileges",
			"Extension",
			"SN|UserPreferences",
		}...)
	}

	for x := range so.Items {
		if !so.Items[x].Deleted && stringInSlice(so.Items[x].ContentType, typesToDelete, true) {
			so.Items[x].Deleted = true
			so.Items[x].Content = ""
			itemsToPut = append(itemsToPut, so.Items[x])
		}
	}

	if len(itemsToPut) > 0 {
		debugPrint(session.Debug, fmt.Sprintf("DeleteContent | removing %d items", len(itemsToPut)))
	}

	si.Items = itemsToPut

	so, err = Sync(si)

	return len(so.SavedItems), err
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
