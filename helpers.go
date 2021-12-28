package gosn

import (
	"fmt"
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

func DeleteContent(session *Session) (err error) {
	gnf := Filter{
		Type: "Note",
	}
	gtf := Filter{
		Type: "Tag",
	}
	gcf := Filter{
		Type: "SN|Component",
	}
	f := ItemFilters{
		Filters:  []Filter{gnf, gtf, gcf},
		MatchAny: true,
	}
	si := SyncInput{
		Session: session,
	}

	var so SyncOutput

	so, err = Sync(si)

	if err != nil {
		return
	}

	var items Items

	items, err = so.Items.DecryptAndParse(session)
	if err != nil {
		return
	}

	items.Filter(f)

	var toDel Items

	for x := range items {
		md := items[x]
		switch md.GetContentType() {
		case "Note":
			md.SetContent(*NewNoteContent())
		case "Tag":
			md.SetContent(*NewTagContent())
		case "SN|Component":
			md.SetContent(*NewComponentContent())
		}

		md.SetDeleted(true)
		toDel = append(toDel, md)
	}

	if len(toDel) > 0 {
		eToDel, _ := toDel.Encrypt(*session)
		si := SyncInput{
			Session: session,
			Items:   eToDel,
		}

		_, err = Sync(si)
		if err != nil {
			return fmt.Errorf("PutItems Failed: %v", err)
		}
	}

	return err
}
