package gosn

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
)

type syncResponse struct {
	Items        EncryptedItems  `json:"retrieved_items"`
	SavedItems   EncryptedItems  `json:"saved_items"`
	Unsaved      EncryptedItems  `json:"unsaved"`
	Conflicts    ConflictedItems `json:"conflicts"`
	SyncToken    string          `json:"sync_token"`
	CursorToken  string          `json:"cursor_token"`
	LastItemPut  int             // the last item successfully put
	PutLimitUsed int             // the put limit used
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

func (ei EncryptedItems) Decrypt(s *Session) (o DecryptedItems, err error) {
	o, err = decryptItems(s, ei)

	return o, err
}

func (ei EncryptedItems) DecryptAndParseItemsKeys(s *Session) (o []ItemsKey, err error) {
	debugPrint(s.Debug, fmt.Sprintf("DecryptAndParseItemsKeys | items: %d", len(ei)))

	var eiks EncryptedItems
	for _, e := range ei {
		if e.ContentType == "SN|ItemsKey" {
			eiks = append(eiks, e)
		}
	}

	if len(eiks) == 0 {
		return
	}

	o, err = decryptAndParseItemKeys(s.MasterKey, eiks)
	for _, ik := range o {
		if ik.IsDefault {
			s.DefaultItemsKey = ik
		}
	}

	if len(o) != 0 {
		s.ItemsKeys = o
	}

	return
}

func isEncryptedType(ct string) bool {
	switch {
	case strings.HasPrefix(ct, "SF"):
		return false
	case ct == "SN|ItemsKey":
		return false
	default:
		return true
	}
}

func (ei *EncryptedItems) Validate() error {
	var err error
	for _, i := range *ei {
		enc := isEncryptedType(i.ContentType)
		switch {
		case i.IsDeleted():
			continue
		case enc && i.ItemsKeyID == "":
			// ignore item in this scenario as the official app does so
			//err = fmt.Errorf("validation failed for \"%s\" with uuid: \"%s\" due to missing ItemsKeyID",
			//	i.ContentType, i.UUID)
		case enc && i.EncItemKey == "":
			err = fmt.Errorf("validation failed for \"%s\" due to missing encrypted item key: \"%s\"",
				i.ContentType, i.UUID)
		}
		if err != nil {
			return err
		}
	}
	return err
}

func (ei EncryptedItems) DecryptAndParse(s *Session) (o Items, err error) {
	debugPrint(s.Debug, fmt.Sprintf("DecryptAndParse | items: %d", len(ei)))

	// if no itemsKeys are passed, then items must be items keys to decrypt with mk
	debugPrint(s.Debug, fmt.Sprintf("DecryptAndParseItemsKeys | items: %d", len(ei)))

	var di DecryptedItems
	di, err = ei.Decrypt(s)
	if err != nil {
		return
	}

	o, err = di.Parse()

	return
}

func (i *Items) Encrypt(s Session) (e EncryptedItems, err error) {
	// return empty if no items provided
	if len(*i) == 0 {
		return
	}
	e, err = encryptItems(s, i)
	if err != nil {
		return
	}

	if err = e.Validate(); err != nil {
		return e, err
	}
	return
}

type EncryptedItem struct {
	UUID        string `json:"uuid"`
	ItemsKeyID  string `json:"items_key_id"`
	Content     string `json:"content"`
	ContentType string `json:"content_type"`
	EncItemKey  string `json:"enc_item_key"`
	Deleted     bool   `json:"deleted"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
	DuplicateOf string `json:"duplicate_of"`
}

func (ei EncryptedItem) GetItemsKeyID() string {
	return ei.ItemsKeyID
}

func (ei EncryptedItem) IsDeleted() bool {
	return ei.Deleted
}

type DecryptedItem struct {
	UUID        string `json:"uuid"`
	ItemsKeyID  string `json:"items_key_id"`
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

func makeSyncRequest(session Session, reqBody []byte) (responseBody []byte, err error) {
	var request *http.Request
	request, err = http.NewRequest(http.MethodPost, session.Server+syncPath, bytes.NewBuffer(reqBody))
	if err != nil {
		return
	}

	request.Header.Set("content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+session.AccessToken)
	request.Header.Set("User-Agent", "github.com/jonhadfield/gosn-v2")

	var response *http.Response

	start := time.Now()
	response, err = httpClient.Do(request)
	elapsed := time.Since(start)

	debugPrint(session.Debug, fmt.Sprintf("makeSyncRequest | request took: %v", elapsed))

	if err != nil {
		return
	}

	defer func() {
		if err := response.Body.Close(); err != nil {
			debugPrint(session.Debug, "makeSyncRequest | failed to close body closed")
		}

		debugPrint(session.Debug, "makeSyncRequest | response body closed")
	}()

	if response.StatusCode == 413 {
		err = errors.New("payload too large")
		return
	}

	if response.StatusCode == 498 {
		err = errors.New("session token is invalid or has expired")
		return
	}

	if response.StatusCode == 401 {
		debugPrint(session.Debug, fmt.Sprintf("makeSyncRequest | sync of %d req bytes failed with: %s", len(reqBody), response.Status))
		err = errors.New("server returned 401 unauthorized during sync request so most likely throttling due to excessive number of requests")

		return
	}

	if response.StatusCode > 400 {
		debugPrint(session.Debug, fmt.Sprintf("makeSyncRequest | sync of %d req bytes failed with: %s", len(reqBody), response.Status))
		return
	}

	if response.StatusCode >= 200 && response.StatusCode < 300 {
		debugPrint(session.Debug, fmt.Sprintf("makeSyncRequest | sync of %d req bytes succeeded with: %s", len(reqBody), response.Status))
	}

	readStart := time.Now()

	responseBody, err = ioutil.ReadAll(response.Body)
	debugPrint(session.Debug, fmt.Sprintf("makeSyncRequest | response read took %+v", time.Since(readStart)))
	if err != nil {
		return
	}

	debugPrint(session.Debug, fmt.Sprintf("makeSyncRequest | response size %d bytes", len(responseBody)))
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
	ClientUpdatedAt    string `json:"client_updated_at"`
	PrefersPlainEditor bool   `json:"prefersPlainEditor"`
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
