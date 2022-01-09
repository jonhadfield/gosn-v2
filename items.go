package gosn

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
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

// AppTagConfig defines expected configuration structure for making Tag related operations.
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
			if e.EncItemKey == "" {
				panic(fmt.Sprintf("DecryptAndParseItemsKeys | item key uuid: %s has no encrypted item key", e.UUID))
			}

			eiks = append(eiks, e)
		}
	}

	if len(eiks) == 0 {
		err = fmt.Errorf("no items keys were retrieved")

		return
	}

	o, err = DecryptAndParseItemKeys(s.MasterKey, eiks)
	if err != nil {
		err = fmt.Errorf("gsDecrypt | %w", err)

		return
	}

	var numDefaultItemKeys int

	for _, ik := range o {
		s.ItemsKeys = append(s.ItemsKeys, ik)

		if ik.Default {
			numDefaultItemKeys++

			debugPrint(s.Debug, fmt.Sprintf("DecryptAndParseItemsKeys | setting default items key uuid: %s", ik.UUID))

			s.DefaultItemsKey = ik
			s.DefaultItemsKey.Default = true
			s.DefaultItemsKey.Content.Default = true
		}
	}

	if numDefaultItemKeys > 1 {
		debugPrint(s.Debug, "DecryptAndParseItemsKeys | more than one default ItemsKey found")

		return
	}

	if s.DefaultItemsKey.UUID == "" {
		err = errors.New("DecryptAndParseItemsKeys | no default items key found")

		return
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
		case enc && i.ItemsKeyID == nil:
			// ignore item in this scenario as the official app does so
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

	// the encrypted items may have been encrypted with a key that is with them
	for x := range ei {
		if ei[x].ContentType != "SN|ItemsKey" {
			var found bool

			for y := range s.ItemsKeys {
				if s.ItemsKeys[y].UUID == *ei[x].ItemsKeyID {
					found = true
				}
			}

			if !found {
				panic(fmt.Sprintf("no items key: %s for %s: %s not found in session", *ei[x].ItemsKeyID, ei[x].ContentType, ei[x].UUID))
			}
		}
	}

	var di DecryptedItems

	di, err = ei.Decrypt(s)
	if err != nil {
		err = fmt.Errorf("DecryptAndParse | Decrypt | %w", err)
		return
	}

	o, err = di.Parse()
	if err != nil {
		err = fmt.Errorf("DecryptAndParse | Parse | %w", err)
	}

	return
}

func (i *Items) Append(x []interface{}) {
	var all Items

	for _, y := range x {
		switch t := y.(type) {
		case Note:
			it := t
			all = append(all, &it)
		case Tag:
			it := t
			all = append(all, &it)
		case Component:
			it := t
			all = append(all, &it)
		}
	}

	*i = all
}

func (i *Items) Encrypt(ik ItemsKey, masterKey string, debug bool) (e EncryptedItems, err error) {
	// return empty if no items provided
	if len(*i) == 0 {
		return
	}

	e, err = encryptItems(i, ik, masterKey, debug)
	if err != nil {
		return
	}

	if err = e.Validate(); err != nil {
		return e, err
	}

	return
}

type EncryptedItem struct {
	UUID        string  `json:"uuid"`
	ItemsKeyID  *string `json:"items_key_id,omitempty"`
	Content     string  `json:"content"`
	ContentType string  `json:"content_type"`
	EncItemKey  string  `json:"enc_item_key"`
	Deleted     bool    `json:"deleted"`
	// Default            bool    `json:"isDefault"`
	CreatedAt          string  `json:"created_at"`
	UpdatedAt          string  `json:"updated_at"`
	CreatedAtTimestamp int64   `json:"created_at_timestamp"`
	UpdatedAtTimestamp int64   `json:"updated_at_timestamp"`
	DuplicateOf        *string `json:"duplicate_of,omitempty"`
}

func (ei EncryptedItem) GetItemsKeyID() string {
	return *ei.ItemsKeyID
}

func (ei EncryptedItem) IsDeleted() bool {
	return ei.Deleted
}

type DecryptedItem struct {
	UUID               string `json:"uuid"`
	ItemsKeyID         string `json:"items_key_id"`
	Content            string `json:"content"`
	ContentType        string `json:"content_type"`
	Deleted            bool   `json:"deleted"`
	Default            bool   `json:"isDefault"`
	CreatedAt          string `json:"created_at"`
	UpdatedAt          string `json:"updated_at"`
	CreatedAtTimestamp int64  `json:"created_at_timestamp"`
	UpdatedAtTimestamp int64  `json:"updated_at_timestamp"`
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

	return responseBody, err
}

// ItemReference defines a reference from one item to another.
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
		case "SN|ItemsKey":
			// TODO: To be implemented separately so we don't parse as a normal item and,
			// most importantly, don't return as a normal Item
			continue
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

func (i Items) Export(s *Session, path string, plainText bool) error {
	// create a new ItemsKey to encrypt the items with
	ik, err := s.CreateItemsKey()
	if err != nil {
		return err
	}

	// encrypt items with the new ItemsKey
	nei, err := i.Encrypt(ik, s.MasterKey, s.Debug)
	if err != nil {
		return err
	}
	// encrypt items key that encrypted the items
	eik, err := ik.Encrypt(s)
	if err != nil {
		return err
	}

	// prepend new items key to the export
	nei = append([]EncryptedItem{eik}, nei...)
	// add existing items keys to the export

	if err = writeJSON(writeJSONConfig{
		session:   *s,
		plainText: false,
		Path:      path,
		Debug:     true,
	}, nei); err != nil {
		return err
	}

	return nil
}

type EncryptedItemExport struct {
	UUID        string  `json:"uuid"`
	ItemsKeyID  *string `json:"items_key_id,omitempty"`
	Content     string  `json:"content"`
	ContentType string  `json:"content_type"`
	// Deleted            bool    `json:"deleted"`
	EncItemKey         string  `json:"enc_item_key"`
	CreatedAt          string  `json:"created_at"`
	UpdatedAt          string  `json:"updated_at"`
	CreatedAtTimestamp int64   `json:"created_at_timestamp"`
	UpdatedAtTimestamp int64   `json:"updated_at_timestamp"`
	DuplicateOf        *string `json:"duplicate_of"`
}

type writeJSONConfig struct {
	session   Session
	plainText bool
	Path      string
	Debug     bool
}

func writeJSON(c writeJSONConfig, items EncryptedItems) error {
	// prepare for export
	var itemsExport []EncryptedItemExport

	for x := range items {
		itemsExport = append(itemsExport, EncryptedItemExport{
			UUID:       items[x].UUID,
			ItemsKeyID: items[x].ItemsKeyID,
			Content:    items[x].Content,
			// Deleted:            items[x].Deleted,
			ContentType:        items[x].ContentType,
			EncItemKey:         items[x].EncItemKey,
			CreatedAt:          items[x].CreatedAt,
			UpdatedAt:          items[x].UpdatedAt,
			CreatedAtTimestamp: items[x].CreatedAtTimestamp,
			UpdatedAtTimestamp: items[x].UpdatedAtTimestamp,
			DuplicateOf:        items[x].DuplicateOf,
		})
	}

	file, err := os.Create(c.Path)
	if err != nil {
		return err
	}

	defer file.Close()

	var jsonExport []byte
	if err == nil {
		jsonExport, err = json.MarshalIndent(itemsExport, "", "  ")
	}

	content := strings.Builder{}
	content.WriteString("{\n  \"version\": \"004\",")
	content.WriteString("\n  \"items\": ")
	content.WriteString(string(jsonExport))
	content.WriteString(",")

	// add keyParams
	content.WriteString("\n  \"keyParams\": {")
	content.WriteString(fmt.Sprintf("\n    \"identifier\": \"%s\",", c.session.KeyParams.Identifier))
	content.WriteString(fmt.Sprintf("\n    \"version\": \"%s\",", c.session.KeyParams.Version))
	content.WriteString(fmt.Sprintf("\n    \"origination\": \"%s\",", c.session.KeyParams.Origination))
	content.WriteString(fmt.Sprintf("\n    \"created\": \"%s\",", c.session.KeyParams.Created))
	content.WriteString(fmt.Sprintf("\n    \"pw_nonce\": \"%s\"", c.session.KeyParams.PwNonce))
	content.WriteString("\n  }")

	content.WriteString("\n}")
	_, err = file.WriteString(content.String())

	return err
}

func (s *Session) Import(path string, persist bool, syncToken string) (items Items, itemsKey ItemsKey, err error) {
	encItemsToImport, err := readJSON(path)
	if err != nil {
		return
	}

	var eik EncryptedItems

	if persist {
		_, err = Sync(SyncInput{
			Session: s,
			Items:   encItemsToImport,
			//SyncToken: syncToken,
		})
		if err != nil {
			return
		}
	}

	// retrieve the ItemsKey used to encrypt the export
	for x := range encItemsToImport {
		if encItemsToImport[x].ContentType == "SN|ItemsKey" {
			eik = append(eik, encItemsToImport[x])

			break
		}
	}

	var mainItemsKeyUUID string
	// getMatchingItemsKey
	for x := range encItemsToImport {
		if encItemsToImport[x].ContentType != "SN|ItemsKey" {
			mainItemsKeyUUID = *encItemsToImport[x].ItemsKeyID
			break
		}
	}

	var mainEncryptedItemsKey EncryptedItem

	for x := range encItemsToImport {
		if encItemsToImport[x].UUID == mainItemsKeyUUID {
			if encItemsToImport[x].UUID == mainItemsKeyUUID {
				mainEncryptedItemsKey = encItemsToImport[x]
				break
			}
		}
	}

	// decrypt the ItemsKey used to encrypt the export
	iks, err := EncryptedItems{mainEncryptedItemsKey}.DecryptAndParseItemsKeys(s)
	iks[0].Default = true
	iks[0].Content.Default = true
	s.DefaultItemsKey = iks[0]
	itemsKey = iks[0]

	s.ItemsKeys = append(s.ItemsKeys, iks...)

	var encFinalList EncryptedItems

	if encItemsToImport != nil {
		for _, item := range encItemsToImport {
			if item.DuplicateOf != nil {
				err = fmt.Errorf("duplicate of item found: %s", *item.DuplicateOf)
			}
		}
		encFinalList = append(encFinalList, encItemsToImport...)
	}

	if len(encFinalList) == 0 {
		err = fmt.Errorf("no items to import were loaded")

		return
	}

	items, err = encFinalList.DecryptAndParse(s)
	if err != nil {
		return
	}

	return
}

func readJSON(filePath string) (items EncryptedItems, err error) {
	file, err := ioutil.ReadFile(filePath)
	if err != nil {
		err = fmt.Errorf("%w failed to open: %s", err, filePath)
		return
	}

	var eif EncryptedItemsFile

	err = json.Unmarshal(file, &eif)
	if err != nil {
		err = fmt.Errorf("failed to unmarshall json: %w", err)
		return
	}

	return eif.Items, err
}

type EncryptedItemsFile struct {
	Items EncryptedItems `json:"items"`
}
