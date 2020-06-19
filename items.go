package gosn

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"
)

type syncResponse struct {
	Items        EncryptedItems `json:"retrieved_items"`
	SavedItems   EncryptedItems `json:"saved_items"`
	Unsaved      EncryptedItems `json:"unsaved"`
	SyncToken    string         `json:"sync_token"`
	CursorToken  string         `json:"cursor_token"`
	LastItemPut  int            // the last item successfully put
	PutLimitUsed int            // the put limit used
}

// AppTagConfig defines expected configuration structure for making Tag related operations
type AppTagConfig struct {
	Email    string
	Token    string
	FindText string
	FindTag  string
	NewTags  []string
	Debug    bool
}

// GetItemsInput defines the input for retrieving items

const retryScaleFactor = 0.25

type EncryptedItems []EncryptedItem

func (ei EncryptedItems) Decrypt(mk, ak string, debug bool) (o DecryptedItems, err error) {
	for _, eItem := range ei {
		var item DecryptedItem

		if eItem.EncItemKey != "" {
			var decryptedEncItemKey string

			decryptedEncItemKey, err = decryptString(eItem.EncItemKey, mk, ak, eItem.UUID)
			if err != nil {
				return
			}

			itemEncryptionKey := decryptedEncItemKey[:len(decryptedEncItemKey)/2]
			itemAuthKey := decryptedEncItemKey[len(decryptedEncItemKey)/2:]

			var decryptedContent string

			decryptedContent, err = decryptString(eItem.Content, itemEncryptionKey, itemAuthKey, eItem.UUID)
			if err != nil {
				return
			}

			item.Content = decryptedContent
		}

		item.UUID = eItem.UUID
		item.Deleted = eItem.Deleted
		item.ContentType = eItem.ContentType
		item.UpdatedAt = eItem.UpdatedAt
		item.CreatedAt = eItem.CreatedAt

		o = append(o, item)
	}

	return o, err
}

func (ei EncryptedItems) DecryptAndParse(mk, ak string, debug bool) (o Items, err error) {
	debugPrint(debug, fmt.Sprintf("DecryptAndParse | items: %d", len(ei)))

	var di DecryptedItems

	di, err = ei.Decrypt(mk, ak, debug)
	if err != nil {
		return
	}

	o, err = di.Parse()

	return
}

func (i *Items) Encrypt(mk, ak string, debug bool) (e EncryptedItems, err error) {
	e, err = encryptItems(i, mk, ak, debug)
	return
}

type EncryptedItem struct {
	UUID        string `json:"uuid"`
	Content     string `json:"content"`
	ContentType string `json:"content_type"`
	EncItemKey  string `json:"enc_item_key"`
	Deleted     bool   `json:"deleted"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type DecryptedItem struct {
	UUID        string `json:"uuid"`
	Content     string `json:"content"`
	ContentType string `json:"content_type"`
	Deleted     bool   `json:"deleted"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type DecryptedItems []DecryptedItem

type UpdateItemRefsInput struct {
	Items Items // Tags
	ToRef Items // Items To Reference
}

type UpdateItemRefsOutput struct {
	Items Items // Tags
}

func UpdateItemRefs(i UpdateItemRefsInput) UpdateItemRefsOutput {
	var updated Items // updated tags

	for _, item := range i.Items {
		var refs ItemReferences

		for _, tr := range i.ToRef {
			ref := ItemReference{
				UUID:        tr.GetUUID(),
				ContentType: tr.GetContentType(),
			}
			refs = append(refs, ref)
		}

		switch item.GetContent().(type) {
		case *NoteContent:
			ic := item.GetContent().(*NoteContent)
			ic.UpsertReferences(refs)
			item.SetContent(*ic)
		case *TagContent:
			ic := item.GetContent().(*TagContent)
			ic.UpsertReferences(refs)
			item.SetContent(*ic)
		}

		updated = append(updated, item)
	}

	return UpdateItemRefsOutput{
		Items: updated,
	}
}

func makeSyncRequest(session Session, reqBody []byte, debug bool) (responseBody []byte, err error) {
	var request *http.Request

	request, err = http.NewRequest(http.MethodPost, session.Server+syncPath, bytes.NewBuffer(reqBody))
	if err != nil {
		return
	}

	request.Header.Set("content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+session.Token)
	request.Header.Set("User-Agent", "github.com/jonhadfield/gosn-v2")

	var response *http.Response

	start := time.Now()
	response, err = httpClient.Do(request)
	elapsed := time.Since(start)

	debugPrint(debug, fmt.Sprintf("makeSyncRequest | request took: %v", elapsed))

	if err != nil {
		return
	}

	defer func() {
		if err := response.Body.Close(); err != nil {
			debugPrint(debug, "makeSyncRequest | failed to close body closed")
		}

		debugPrint(debug, "makeSyncRequest | response body closed")
	}()

	switch response.StatusCode {
	case 413:
		err = errors.New("payload too large")
		return
	}

	if response.StatusCode > 400 {
		debugPrint(debug, fmt.Sprintf("makeSyncRequest | sync of %d req bytes failed with: %s", len(reqBody), response.Status))
		return
	}

	if response.StatusCode >= 200 && response.StatusCode < 300 {
		debugPrint(debug, fmt.Sprintf("makeSyncRequest | sync of %d req bytes succeeded with: %s", len(reqBody), response.Status))
	}

	readStart := time.Now()

	responseBody, err = ioutil.ReadAll(response.Body)

	debugPrint(debug, fmt.Sprintf("makeSyncRequest | response read took %+v", time.Since(readStart)))

	if err != nil {
		return
	}

	debugPrint(debug, fmt.Sprintf("makeSyncRequest | response size %d bytes", len(responseBody)))

	return responseBody, err
}

// ItemReference defines a reference from one item to another
type ItemReference struct {
	// unique identifier of the item being referenced
	UUID string `json:"uuid"`
	// type of item being referenced
	ContentType string `json:"content_type"`
}

type OrgStandardNotesSNDetail struct {
	ClientUpdatedAt string `json:"client_updated_at"`
}
type AppDataContent struct {
	OrgStandardNotesSN OrgStandardNotesSNDetail `json:"org.standardnotes.sn"`
}

type TagContent struct {
	Title          string         `json:"title"`
	ItemReferences ItemReferences `json:"references"`
	AppData        AppDataContent `json:"appData"`
}

func removeStringFromSlice(inSt string, inSl []string) (outSl []string) {
	for _, si := range inSl {
		if inSt != si {
			outSl = append(outSl, si)
		}
	}

	return
}

type ItemReferences []ItemReference

type Items []Item

func (di *DecryptedItems) Parse() (p Items, err error) {
	for _, i := range *di {
		var pi Item

		switch i.ContentType {
		case "Note":
			pi = parseNote(i)
		case "Tag":
			pi = parseTag(i)
		case "SN|Component":
			pi = parseComponent(i)
		case "SN|Theme":
			pi = parseTheme(i)
		case "SN|Privileges":
			pi = parsePrivileges(i)
		case "Extension":
			pi = parseExtension(i)
		case "SF|Extension":
			pi = parseSFExtension(i)
		case "SF|MFA":
			pi = parseSFMFA(i)
		case "SN|SmartTag":
			pi = parseSmartTag(i)
		case "SN|FileSafe|FileMetadata":
			pi = parseFileSafeFileMetadata(i)
		case "SN|FileSafe|Integration":
			pi = parseFileSafeIntegration(i)
		case "SN|UserPreferences":
			pi = parseUserPreferences(i)
		case "SN|ExtensionRepo":
			pi = parseExtensionRepo(i)
		case "SN|FileSafe|Credentials":
			pi = parseFileSafeCredentials(i)
		default:
			return nil, fmt.Errorf("unhandled type '%s'", i.ContentType)
		}

		p = append(p, pi)
	}

	return p, err
}

func processContentModel(contentType, input string) (output Content, err error) {
	// identify content model
	// try and unmarshall Item
	switch contentType {
	case "Note":
		var nc NoteContent
		err = json.Unmarshal([]byte(input), &nc)

		return nc, err
	case "Tag":
		var tc TagContent
		err = json.Unmarshal([]byte(input), &tc)

		return tc, err
	case "SN|Component":
		var cc ComponentContent
		err = json.Unmarshal([]byte(input), &cc)

		return cc, err
	case "SN|Theme":
		var tc ThemeContent
		err = json.Unmarshal([]byte(input), &tc)

		return tc, err
	case "SN|Privileges":
		var pc PrivilegesContent
		err = json.Unmarshal([]byte(input), &pc)

		return pc, err
	case "Extension":
		var ec ExtensionContent
		err = json.Unmarshal([]byte(input), &ec)

		return ec, err
	case "SF|Extension":
		var sfe SFExtensionContent

		if len(input) > 0 {
			err = json.Unmarshal([]byte(input), &sfe)
		}

		return sfe, err
	case "SF|MFA":
		var sfm SFMFAContent

		if len(input) > 0 {
			err = json.Unmarshal([]byte(input), &sfm)
		}

		return sfm, err
	case "SN|SmartTag":
		var st SmartTagContent

		if len(input) > 0 {
			err = json.Unmarshal([]byte(input), &st)
		}

		return st, err

	case "SN|FileSafe|FileMetadata":
		var fsfm FileSafeFileMetaDataContent

		if len(input) > 0 {
			err = json.Unmarshal([]byte(input), &fsfm)
		}

		return fsfm, err

	case "SN|FileSafe|Integration":
		var fsi FileSafeIntegrationContent

		if len(input) > 0 {
			err = json.Unmarshal([]byte(input), &fsi)
		}

		return fsi, err
	case "SN|UserPreferences":
		var upc UserPreferencesContent

		if len(input) > 0 {
			err = json.Unmarshal([]byte(input), &upc)
		}

		return upc, err
	case "SN|ExtensionRepo":
		var erc ExtensionRepoContent

		if len(input) > 0 {
			err = json.Unmarshal([]byte(input), &erc)
		}

		return erc, err
	case "SN|FileSafe|Credentials":
		var fsc FileSafeCredentialsContent

		if len(input) > 0 {
			err = json.Unmarshal([]byte(input), &fsc)
		}

		return fsc, err

	default:
		return nil, fmt.Errorf("unexpected type '%s'", contentType)
	}
}

func (ei *EncryptedItems) DeDupe() {
	var encountered []string

	var deDuped EncryptedItems

	for _, i := range *ei {
		if !stringInSlice(i.UUID, encountered, true) {
			deDuped = append(deDuped, i)
		}

		encountered = append(encountered, i.UUID)
	}

	*ei = deDuped
}

func (ei *EncryptedItems) RemoveDeleted() {
	var clean EncryptedItems

	for _, i := range *ei {
		if !i.Deleted {
			clean = append(clean, i)
		}
	}

	*ei = clean
}

func (i *Items) DeDupe() {
	var encountered []string

	var deDuped Items

	for _, j := range *i {
		if !stringInSlice(j.GetUUID(), encountered, true) {
			deDuped = append(deDuped, j)
		}

		encountered = append(encountered, j.GetUUID())
	}

	*i = deDuped
}

func (i *Items) RemoveDeleted() {
	var clean Items

	for _, j := range *i {
		if !j.IsDeleted() {
			clean = append(clean, j)
		}
	}

	*i = clean
}

func (di *DecryptedItems) RemoveDeleted() {
	var clean DecryptedItems

	for _, j := range *di {
		if !j.Deleted {
			clean = append(clean, j)
		}
	}

	*di = clean
}
