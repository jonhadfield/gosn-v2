package gosn

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/google/uuid"
)

func stripLineBreak(input string) string {
	if strings.HasSuffix(input, "\n") {
		return input[:len(input)-1]
	}

	return input
}

func lesserOf(first, second int) int {
	if first < second {
		if first < 0 {
			return 0
		}

		return first
	}

	if second < 0 {
		return 0
	}

	return second
}

// GenUUID generates a unique identifier required when creating a new item.
func GenUUID() string {
	newUUID := uuid.New()
	return newUUID.String()
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
			"SN|File",
		}...)
	}

	for x := range so.Items {
		if !so.Items[x].Deleted && slices.Contains(typesToDelete, so.Items[x].ContentType) {
			so.Items[x].Deleted = true
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
	// fmt.Printf("unmarshallSyncResponse | input: %s\n", string(input))
	err = json.Unmarshal(input, &output)
	if err != nil {
		return
	}

	// check no items keys have an items key
	for _, item := range output.Items {
		if item.ContentType == "SN|ItemsKey" && item.ItemsKeyID != "" {
			err = fmt.Errorf("SN|ItemsKey %s has an ItemsKeyID set", item.UUID)
			return
		}
	}

	return
}
