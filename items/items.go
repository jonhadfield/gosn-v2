package items

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/go-retryablehttp"
	"github.com/jonhadfield/gosn-v2/auth"
	"github.com/jonhadfield/gosn-v2/common"
	"github.com/jonhadfield/gosn-v2/crypto"
	"github.com/jonhadfield/gosn-v2/log"
	"github.com/jonhadfield/gosn-v2/session"
	"github.com/santhosh-tekuri/jsonschema/v5"
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

const retryScaleFactor = 0.25

type EncryptedItems []EncryptedItem

func (ei EncryptedItems) DecryptAndParseItemsKeys(mk string, debug bool) (o []session.SessionItemsKey, err error) {
	log.DebugPrint(debug, fmt.Sprintf("DecryptAndParseItemsKeys | encrypted items to check: %d", len(ei)), common.MaxDebugChars)

	if len(ei) == 0 {
		return
	}

	var eiks EncryptedItems

	for _, e := range ei {
		if e.ContentType == common.SNItemTypeItemsKey && !e.Deleted {
			if e.UUID == "" {
				panic("DecryptAndParseItemsKeys | items key has no uuid")
			}

			if e.EncItemKey == "" {
				panic(fmt.Sprintf("DecryptAndParseItemsKeys | items key uuid: %s has no encrypted item key", e.UUID))
			}

			eiks = append(eiks, e)
		}
	}

	if len(eiks) == 0 {
		// err = fmt.Errorf("no items keys were retrieved")

		return
	}

	dpiks, err := DecryptAndParseItemKeys(mk, eiks)
	if err != nil {
		err = fmt.Errorf("DecryptAndParseItemsKeys | %w", err)

		return
	}

	if len(dpiks) == 0 {
		err = fmt.Errorf("failed to decrypt and parse items keys")
		return
	}

	for _, dpik := range dpiks {
		o = append(o, session.SessionItemsKey{
			UUID:     dpik.UUID,
			ItemsKey: dpik.ItemsKey,
			Default:  dpik.Default,
		})
	}

	return
}

func IsEncryptedType(ct string) bool {
	switch {
	case strings.HasPrefix(ct, "SF"):
		return false
	case ct == common.SNItemTypeItemsKey:
		return false
	default:
		return true
	}
}

func (ei *EncryptedItems) Validate() error {
	var err error

	dei := *ei

	for x := range dei {
		enc := IsEncryptedType(dei[x].ContentType)

		switch {
		case dei[x].IsDeleted():
			continue
		case enc && dei[x].ItemsKeyID == "":
			// ignore item in this scenario as the official app does so
		case enc && dei[x].EncItemKey == "":
			err = fmt.Errorf("validation failed for \"%s\" due to missing encrypted item key: \"%s\"",
				dei[x].ContentType, dei[x].UUID)
		}

		if err != nil {
			return err
		}
	}

	return err
}

func ReEncryptItem(ei EncryptedItem, decryptionItemsKey session.SessionItemsKey, newItemsKey ItemsKey, newMasterKey string, s *session.Session) (o EncryptedItem, err error) {
	log.DebugPrint(s.Debug, fmt.Sprintf("ReEncrypt | item to re-encrypt %s %s", ei.ContentType, ei.UUID), common.MaxDebugChars)

	var di DecryptedItem

	di, err = DecryptItem(ei, s, []session.SessionItemsKey{decryptionItemsKey})

	if err != nil {
		err = fmt.Errorf("ReEncryptItem | Decrypt | %w", err)
		return
	}

	return di.Encrypt(newItemsKey, s)
}

func (ei EncryptedItems) ReEncrypt(s *session.Session, decryptionItemsKey session.SessionItemsKey, newItemsKey ItemsKey, newMasterKey string) (o EncryptedItems, err error) {
	log.DebugPrint(s.Debug, fmt.Sprintf("ReEncrypt | items: %d", len(ei)), common.MaxDebugChars)

	var di DecryptedItems

	di, err = DecryptItems(s, ei, []session.SessionItemsKey{decryptionItemsKey})

	if err != nil {
		err = fmt.Errorf("ReEncrypt | Decrypt | %w", err)
		return
	}

	for x := range di {
		// items key handled separately
		if di[x].ContentType == common.SNItemTypeItemsKey {
			continue
		}

		var ri EncryptedItem

		ri, err = di[x].Encrypt(newItemsKey, s)
		if err != nil {
			err = fmt.Errorf("ReEncrypt | Encrypt | %w", err)

			return
		}

		o = append(o, ri)
	}

	return o, err
}

func DecryptAndParseItem(ei EncryptedItem, s *session.Session) (o Item, err error) {
	log.DebugPrint(s.Debug, fmt.Sprintf("DecryptAndParse | items: %s %s", ei.ContentType, ei.UUID), common.MaxDebugChars)

	var di DecryptedItem
	//
	// if len(s.ImporterItemsKeys) > 0 {
	// 	logging.DebugPrint(s.Debug, "DecryptAndParse | using ImportersItemsKey", common.MaxDebugChars)
	// 	ik := GetMatchingItem(ei.ItemsKeyID, s.ImporterItemsKeys)
	//
	// 	di, err = DecryptItem(ei, s, ItemsKeys{ik})
	// } else
	di, err = DecryptItem(ei, s, []session.SessionItemsKey{})
	// }

	if err != nil {
		err = fmt.Errorf("DecryptAndParse | Decrypt | %w", err)
		return
	}

	o, err = ParseItem(di)
	if err != nil {
		err = fmt.Errorf("DecryptAndParse | ParseItem | %w", err)

		return
	}

	if s.SchemaValidation {
		var contentSchema *jsonschema.Schema

		switch it := o.(type) {
		case *Note:
			contentSchema = s.Schemas[noteContentSchemaName]
			if contentSchema == nil {
				err = fmt.Errorf("failed to get schema for %s", noteContentSchemaName)
				return
			}

			if err = validateContentSchema(s.Schemas[noteContentSchemaName], it.Content); err != nil {
				return
			}
		}
	}

	return
}

func DecryptAndParseItems(ei EncryptedItems, s *session.Session) (o Items, err error) {
	log.DebugPrint(s.Debug, fmt.Sprintf("DecryptAndParse | items: %d", len(ei)), common.MaxDebugChars)

	for x := range ei {
		var di Item

		di, err = DecryptAndParseItem(ei[x], s)
		if err != nil {
			return
		}

		o = append(o, di)
	}

	return
}

func (ei EncryptedItems) DecryptAndParse(s *session.Session) (o Items, err error) {
	log.DebugPrint(s.Debug, fmt.Sprintf("DecryptAndParse | items: %d", len(ei)), common.MaxDebugChars)

	var di DecryptedItems

	// if len(s.ImporterItemsKeys) > 0 && s.ImporterItemsKeys.Latest().UUID != "" {
	// 	logging.DebugPrint(s.Debug, "DecryptAndParse | using ImportersItemsKeys", common.MaxDebugChars)
	// 	di, err = DecryptItems(s, ei, s.ImporterItemsKeys)
	// } else {
	log.DebugPrint(s.Debug, "DecryptAndParse | using Session's ItemsKeys", common.MaxDebugChars)
	di, err = DecryptItems(s, ei, []session.SessionItemsKey{})
	// }

	if err != nil {
		err = fmt.Errorf("DecryptAndParse | Decrypt | %w", err)
		return
	}

	o, err = di.Parse()
	if err != nil {
		err = fmt.Errorf("DecryptAndParse | ParseItem | %w", err)

		return
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

func (i *Items) Encrypt(s *session.Session, ik session.SessionItemsKey) (e EncryptedItems, err error) {
	// return empty if no items provided
	if len(*i) == 0 {
		return
	}

	// fmt.Printf("Encrypt | encrypting %d items\n", len(*i))
	// for _, x := range *i {
	// 	fmt.Printf("----- %s %s\n", x.GetContentType(), x.GetUUID())
	// }
	e, err = encryptItems(s, i, ik)
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
	ItemsKeyID  string `json:"items_key_id,omitempty"`
	Content     string `json:"content"`
	ContentType string `json:"content_type"`
	EncItemKey  string `json:"enc_item_key"`
	Deleted     bool   `json:"deleted"`
	// Default            bool    `json:"isDefault"`
	CreatedAt           string  `json:"created_at"`
	UpdatedAt           string  `json:"updated_at"`
	CreatedAtTimestamp  int64   `json:"created_at_timestamp"`
	UpdatedAtTimestamp  int64   `json:"updated_at_timestamp"`
	DuplicateOf         *string `json:"duplicate_of,omitempty"`
	AuthHash            *string `json:"auth_hash,omitempty"`
	UpdatedWithSession  *string `json:"updated_with_session,omitempty"`
	KeySystemIdentifier *string `json:"key_system_identifier,omitempty"`
	SharedVaultUUID     *string `json:"shared_vault_uuid,omitempty"`
	UserUUID            *string `json:"user_uuid,omitempty"`
	LastEditedByUUID    *string `json:"last_edited_by_uuid,omitempty"`
}

func (ei EncryptedItem) GetItemsKeyID() string {
	if ei.ItemsKeyID != "" {
		return ei.ItemsKeyID
	}

	return ""
}

func (ei EncryptedItem) IsDeleted() bool {
	return ei.Deleted
}

type DecryptedItem struct {
	UUID                string  `json:"uuid"`
	ItemsKeyID          string  `json:"items_key_id,omitempty"`
	Content             string  `json:"content"`
	ContentType         string  `json:"content_type"`
	DuplicateOf         string  `json:"duplicate_of,omitempty"`
	Deleted             bool    `json:"deleted"`
	Default             bool    `json:"isDefault"`
	CreatedAt           string  `json:"created_at"`
	UpdatedAt           string  `json:"updated_at"`
	CreatedAtTimestamp  int64   `json:"created_at_timestamp"`
	UpdatedAtTimestamp  int64   `json:"updated_at_timestamp"`
	AuthHash            *string `json:"auth_hash,omitempty"`
	UpdatedWithSession  *string `json:"updated_with_session,omitempty"`
	KeySystemIdentifier *string `json:"key_system_identifier,omitempty"`
	SharedVaultUUID     *string `json:"shared_vault_uuid,omitempty"`
	UserUUID            *string `json:"user_uuid,omitempty"`
	LastEditedByUUID    *string `json:"last_edited_by_uuid,omitempty"`
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
			item.SetContent(ic)
		case *TagContent:
			ic := item.GetContent().(*TagContent)
			ic.UpsertReferences(refs)
			item.SetContent(ic)
		}

		updated = append(updated, item)
	}

	return UpdateItemRefsOutput{
		Items: updated,
	}
}

func makeSyncRequest(session session.Session, reqBody []byte) (responseBody []byte, err error) {
	// fmt.Println(string(reqBody))
	if session.HTTPClient == nil {
		log.DebugPrint(session.Debug, "makeSyncRequest | creating new http client", common.MaxDebugChars)
		session.HTTPClient = common.NewHTTPClient()
	}

	var request *retryablehttp.Request

	request, err = retryablehttp.NewRequest(http.MethodPost, session.Server+common.SyncPath, bytes.NewBuffer(reqBody))
	if err != nil {
		return
	}

	request.Header.Set(common.HeaderContentType, common.SNAPIContentType)
	request.Header.Set("Authorization", "Bearer "+session.AccessToken)
	request.Header.Set("User-Agent", "github.com/jonhadfield/gosn-v2")

	var response *http.Response

	start := time.Now()
	response, err = session.HTTPClient.Do(request)
	elapsed := time.Since(start)

	log.DebugPrint(session.Debug, fmt.Sprintf("makeSyncRequest | request took: %v", elapsed), common.MaxDebugChars)

	if err != nil {
		return
	}

	defer func() {
		if err := response.Body.Close(); err != nil {
			log.DebugPrint(session.Debug, "makeSyncRequest | failed to close body closed", common.MaxDebugChars)
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
		log.DebugPrint(session.Debug, fmt.Sprintf("makeSyncRequest | sync of %d req bytes failed with: %s", len(reqBody), response.Status), common.MaxDebugChars)

		err = errors.New("server returned 401 unauthorized during sync request so most likely throttling due to excessive number of requests")

		return
	}

	if response.StatusCode > 400 {
		log.DebugPrint(session.Debug, fmt.Sprintf("makeSyncRequest | sync of %d req bytes failed with: %s", len(reqBody), response.Status), common.MaxDebugChars)
		return
	}

	if response.StatusCode >= 200 && response.StatusCode < 300 {
		log.DebugPrint(session.Debug, fmt.Sprintf("makeSyncRequest | sync of %d req bytes succeeded with: %s", len(reqBody), response.Status), common.MaxDebugChars)
	}

	// readStart := time.Now()

	responseBody, err = io.ReadAll(response.Body)

	// fmt.Println(string(responseBody))
	// logging.DebugPrint(session.Debug, fmt.Sprintf("makeSyncRequest | response read took %+v", time.Since(readStart)))

	return responseBody, err
}

// ItemReference defines a reference from one item to another.
type ItemReference struct {
	// unique identifier of the item being referenced
	UUID string `json:"uuid"`
	// type of item being referenced
	ContentType string `json:"content_type"`
	// type of reference, notetonote, tagtonote, etc
	ReferenceType string `json:"reference_type,omitempty"`
}

type OrgStandardNotesSNDetail struct {
	ClientUpdatedAt    string `json:"client_updated_at"`
	PrefersPlainEditor bool   `json:"prefersPlainEditor"`
	Pinned             bool   `json:"pinned"`
}

type OrgStandardNotesSNComponentsDetail map[string]interface{}

type AppDataContent struct {
	OrgStandardNotesSN           OrgStandardNotesSNDetail           `json:"org.standardnotes.sn"`
	OrgStandardNotesSNComponents OrgStandardNotesSNComponentsDetail `json:"org.standardnotes.sn.components,omitempty"`
}

type NoteAppDataContent struct {
	OrgStandardNotesSN           OrgStandardNotesSNDetail           `json:"org.standardnotes.sn"`
	OrgStandardNotesSNComponents OrgStandardNotesSNComponentsDetail `json:"org.standardnotes.sn.components,omitempty"`
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

func (i Items) UUIDs() []string {
	var uuids []string

	for _, ii := range i {
		uuids = append(uuids, ii.GetUUID())
	}

	return uuids
}

func ParseItem(di DecryptedItem) (p Item, err error) {
	var pi Item

	switch di.ContentType {
	case common.SNItemTypeItemsKey:
		// TODO: To be implemented separately so we don't parse as a normal item and,
		// most importantly, don't return as a normal Item
	case common.SNItemTypeNote:
		pi = parseNote(di)
	case common.SNItemTypeTag:
		pi = parseTag(di)
	case common.SNItemTypeComponent:
		pi = parseComponent(di)
	case common.SNItemTypeTheme:
		pi = parseTheme(di)
	case common.SNItemTypePrivileges:
		pi = parsePrivileges(di)
	case common.SNItemTypeExtension:
		pi = parseExtension(di)
	case common.SNItemTypeSFExtension:
		pi = parseSFExtension(di)
	case common.SNItemTypeSFMFA:
		pi = parseSFMFA(di)
	case common.SNItemTypeSmartTag:
		pi = parseSmartTag(di)
	case common.SNItemTypeFileSafeFileMetaData:
		pi = parseFileSafeFileMetadata(di)
	case common.SNItemTypeFileSafeIntegration:
		pi = parseFileSafeIntegration(di)
	case common.SNItemTypeUserPreferences:
		pi = parseUserPreferences(di)
	case common.SNItemTypeExtensionRepo:
		pi = parseExtensionRepo(di)
	case common.SNItemTypeFileSafeCredentials:
		pi = parseFileSafeCredentials(di)
	case common.SNItemTypeFile:
		pi = parseFile(di)
	default:
		return nil, fmt.Errorf("unhandled type1 '%s' %s", di.ContentType, di.Content)
	}

	return pi, err
}

func (di *DecryptedItems) Parse() (p Items, err error) {
	for _, i := range *di {
		var pi Item

		switch i.ContentType {
		case common.SNItemTypeItemsKey:
			// TODO: To be implemented separately so we don't parse as a normal item and,
			// most importantly, don't return as a normal Item
			continue
		case common.SNItemTypeNote:
			pi = parseNote(i)
		case common.SNItemTypeTag:
			pi = parseTag(i)
		case common.SNItemTypeComponent:
			pi = parseComponent(i)
		case common.SNItemTypeTheme:
			pi = parseTheme(i)
		case common.SNItemTypePrivileges:
			pi = parsePrivileges(i)
		case common.SNItemTypeExtension:
			pi = parseExtension(i)
		case common.SNItemTypeSFExtension:
			pi = parseSFExtension(i)
		case common.SNItemTypeSFMFA:
			pi = parseSFMFA(i)
		case common.SNItemTypeSmartTag:
			pi = parseSmartTag(i)
		case common.SNItemTypeFileSafeFileMetaData:
			pi = parseFileSafeFileMetadata(i)
		case common.SNItemTypeFileSafeIntegration:
			pi = parseFileSafeIntegration(i)
		case common.SNItemTypeUserPreferences:
			pi = parseUserPreferences(i)
		case common.SNItemTypeExtensionRepo:
			pi = parseExtensionRepo(i)
		case common.SNItemTypeFileSafeCredentials:
			pi = parseFileSafeCredentials(i)
		case common.SNItemTypeFile:
			pi = parseFile(i)
		default:
			return nil, fmt.Errorf("unhandled type2 '%s' %s", i.ContentType, i.Content)
		}

		p = append(p, pi)
	}

	return p, err
}

func processContentModel(contentType, input string) (output Content, err error) {
	// identify content model
	// try and unmarshall Item
	switch contentType {
	case common.SNItemTypeNote:
		var nc NoteContent

		if err = json.Unmarshal([]byte(input), &nc); err != nil {
			err = fmt.Errorf("processContentModel | %w", err)

			return output, err
		}

		return &nc, nil
	case common.SNItemTypeTag:
		var tc TagContent
		if err = json.Unmarshal([]byte(input), &tc); err != nil {
			err = fmt.Errorf("processContentModel | %w", err)

			return output, err
		}

		return &tc, nil
	case common.SNItemTypeComponent:
		var cc ComponentContent
		if err = json.Unmarshal([]byte(input), &cc); err != nil {
			err = fmt.Errorf("processContentModel | %w", err)

			return
		}

		return &cc, nil
	case common.SNItemTypeTheme:
		var tc ThemeContent
		if err = json.Unmarshal([]byte(input), &tc); err != nil {
			err = fmt.Errorf("processContentModel | %w", err)

			return
		}

		return &tc, nil
	case common.SNItemTypePrivileges:
		var pc PrivilegesContent
		if err = json.Unmarshal([]byte(input), &pc); err != nil {
			if err = json.Unmarshal([]byte(input), &pc); err != nil {
				err = fmt.Errorf("processContentModel | %w", err)

				return
			}
		}

		return &pc, nil
	case common.SNItemTypeExtension:
		var ec ExtensionContent
		if err = json.Unmarshal([]byte(input), &ec); err != nil {
			err = fmt.Errorf("processContentModel | %w", err)

			return
		}

		return &ec, nil
	case common.SNItemTypeSFExtension:
		var sfe SFExtensionContent

		if len(input) > 0 {
			if err = json.Unmarshal([]byte(input), &sfe); err != nil {
				err = fmt.Errorf("processContentModel | %w", err)

				return
			}
		}

		return &sfe, nil
	case common.SNItemTypeSFMFA:
		var sfm SFMFAContent

		if len(input) > 0 {
			if err = json.Unmarshal([]byte(input), &sfm); err != nil {
				err = fmt.Errorf("processContentModel | %w", err)

				return
			}
		}

		return &sfm, nil
	case common.SNItemTypeSmartTag:
		var st SmartTagContent

		if len(input) > 0 {
			if err = json.Unmarshal([]byte(input), &st); err != nil {
				err = fmt.Errorf("processContentModel | %w", err)

				return
			}
		}

		return &st, nil

	case common.SNItemTypeFileSafeFileMetaData:
		var fsfm FileSafeFileMetaDataContent

		if len(input) > 0 {
			if err = json.Unmarshal([]byte(input), &fsfm); err != nil {
				err = fmt.Errorf("processContentModel | %w", err)

				return
			}
		}

		return &fsfm, nil

	case common.SNItemTypeFileSafeIntegration:
		var fsi FileSafeIntegrationContent

		if len(input) > 0 {
			if err = json.Unmarshal([]byte(input), &fsi); err != nil {
				err = fmt.Errorf("processContentModel | %w", err)

				return
			}
		}

		return &fsi, nil
	case common.SNItemTypeUserPreferences:
		var upc UserPreferencesContent

		if len(input) > 0 {
			if err = json.Unmarshal([]byte(input), &upc); err != nil {
				err = fmt.Errorf("processContentModel | %w", err)

				return
			}
		}

		return &upc, nil
	case common.SNItemTypeExtensionRepo:
		var erc ExtensionRepoContent

		if len(input) > 0 {
			if err = json.Unmarshal([]byte(input), &erc); err != nil {
				err = fmt.Errorf("processContentModel | %w", err)

				return
			}
		}

		return &erc, nil
	case common.SNItemTypeFileSafeCredentials:
		var fsc FileSafeCredentialsContent

		if len(input) > 0 {
			if err = json.Unmarshal([]byte(input), &fsc); err != nil {
				err = fmt.Errorf("processContentModel | %w", err)

				return
			}
		}

		return &fsc, nil
	case common.SNItemTypeFile:
		var fc FileContent

		if len(input) > 0 {
			if err = json.Unmarshal([]byte(input), &fc); err != nil {
				err = fmt.Errorf("processContentModel | %w", err)

				return
			}
		}

		return &fc, nil
	default:
		return nil, fmt.Errorf("unexpected type '%s'", contentType)
	}
}

func (ei *EncryptedItems) DeDupe() {
	if ei == nil {
		return
	}

	uniqueItems := make(map[string]EncryptedItem)

	var deDuped EncryptedItems

	eis := *ei
	for _, ei1 := range eis {
		if _, ok := uniqueItems[ei1.UUID]; ok {
			if ei1.UpdatedAtTimestamp > uniqueItems[ei1.UUID].UpdatedAtTimestamp {
				uniqueItems[ei1.UUID] = ei1
			}
		} else {
			uniqueItems[ei1.UUID] = ei1
		}
	}

	for _, v := range uniqueItems {
		deDuped = append(deDuped, v)
	}

	*ei = deDuped
}

func (ei *EncryptedItems) RemoveUnsupported() {
	var supported EncryptedItems

	for _, i := range *ei {
		if !slices.Contains([]string{common.SNItemTypeSFExtension}, i.ContentType) && !strings.HasPrefix(i.Content, "003") {
			supported = append(supported, i)
		}
		// if !strings.HasPrefix(i.Content, "003") {
		// 	supported = append(supported, i)
		// }
	}

	*ei = supported
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
		if !slices.Contains(encountered, j.GetUUID()) {
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

type EncryptedItemExport struct {
	UUID        string `json:"uuid"`
	ItemsKeyID  string `json:"items_key_id,omitempty"`
	Content     string `json:"content"`
	ContentType string `json:"content_type"`
	// Deleted            bool    `json:"deleted"`
	EncItemKey         string  `json:"enc_item_key"`
	CreatedAt          string  `json:"created_at"`
	UpdatedAt          string  `json:"updated_at"`
	CreatedAtTimestamp int64   `json:"created_at_timestamp"`
	UpdatedAtTimestamp int64   `json:"updated_at_timestamp"`
	DuplicateOf        *string `json:"duplicate_of"`
}

type writeJSONConfig struct {
	session session.Session
	Path    string
	Debug   bool
}

func writeJSON(c writeJSONConfig, items EncryptedItems) error {
	// prepare for export
	var itemsExport []EncryptedItemExport

	for x := range items {
		itemsExport = append(itemsExport, EncryptedItemExport{
			UUID:               items[x].UUID,
			ItemsKeyID:         items[x].ItemsKeyID,
			Content:            items[x].Content,
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
		if jsonExport, err = json.MarshalIndent(itemsExport, "", "  "); err != nil {
			return fmt.Errorf("writeJSON | %w", err)
		}
	}

	content := strings.Builder{}
	content.WriteString("{\n  \"version\": \"004\",")
	content.WriteString("\n  \"items\": ")
	content.Write(jsonExport)
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
	if err != nil {
		return fmt.Errorf("writeJSON | %w", err)
	}

	return nil
}

type CompareEncryptedItemsInput struct {
	Session        *session.Session
	FirstItem      EncryptedItem
	FirstItemsKey  session.SessionItemsKey
	SecondItem     EncryptedItem
	SecondItemsKey session.SessionItemsKey
}

type CompareItemsInput struct {
	Session    *session.Session
	FirstItem  Item
	SecondItem Item
}

func compareItems(input CompareItemsInput) (same, unsupported bool, err error) {
	if input.FirstItem.GetContentType() != input.SecondItem.GetContentType() {
		return false, unsupported, nil
	}

	first := input.FirstItem
	second := input.SecondItem

	switch first.GetContentType() {
	case common.SNItemTypeNote:
		n1 := first.(*Note)
		n2 := second.(*Note)

		return n1.Content.Title == n2.Content.Title && n1.Content.Text == n2.Content.Text, unsupported, nil
	case common.SNItemTypeTag:
		t1 := first.(*Tag)
		t2 := second.(*Tag)

		// compare references
		var refsDiffer bool

		t1Refs := t1.Content.ItemReferences
		t2Refs := t2.Content.ItemReferences

		if len(t1Refs) == len(t2Refs) {
			for x := range t1Refs {
				if t1Refs[x] != t2Refs[x] {
					refsDiffer = true
					break
				}
			}
		} else {
			refsDiffer = true
		}

		return t1.Content.Title == t2.Content.Title && !refsDiffer, unsupported, nil
	}

	return false, true, nil
}

func compareEncryptedItems(input CompareEncryptedItemsInput) (same, unsupported bool, err error) {
	if input.FirstItem.ContentType != input.SecondItem.ContentType {
		return false, unsupported, nil
	}

	fDec, err := DecryptItems(input.Session, EncryptedItems{input.FirstItem}, []session.SessionItemsKey{input.FirstItemsKey})
	if err != nil {
		return
	}

	fPar, err := fDec.Parse()
	if err != nil {
		return
	}

	sDec, err := DecryptItems(input.Session, EncryptedItems{input.SecondItem}, []session.SessionItemsKey{input.SecondItemsKey})
	if err != nil {
		return
	}

	sPar, err := sDec.Parse()
	if err != nil {
		return
	}

	first := fPar[0]
	second := sPar[0]

	switch first.GetContentType() {
	case common.SNItemTypeNote:
		n1 := first.(*Note)
		n2 := second.(*Note)

		return n1.Content.Title == n2.Content.Title && n1.Content.Text == n2.Content.Text, unsupported, nil
	case common.SNItemTypeTag:
		t1 := first.(*Tag)
		t2 := second.(*Tag)

		// compare references
		var refsDiffer bool

		t1Refs := t1.Content.ItemReferences
		t2Refs := t2.Content.ItemReferences

		if len(t1Refs) == len(t2Refs) {
			for x := range t1Refs {
				if t1Refs[x] != t2Refs[x] {
					refsDiffer = true
					break
				}
			}
		} else {
			refsDiffer = true
		}

		return t1.Content.Title == t2.Content.Title && !refsDiffer, unsupported, nil
	}

	return false, true, nil
}

// func decryptExport(s *session.Session, path, password string) (items Items, err error) {
// 	encItemsToImport, keyParams, err := readJSON(path)
// 	if err != nil {
// 		return
// 	}
//
// 	logging.DebugPrint(s.Debug, fmt.Sprintf("Import | read %d items from export file", len(encItemsToImport)), common.MaxDebugChars)
//
// 	// set master key to session by default, but then check if new one is required
// 	mk := s.MasterKey
//
// 	// if export was for a different user (identifier used to generate salt)
// 	if keyParams.Identifier != s.KeyParams.Identifier || keyParams.PwNonce != s.KeyParams.PwNonce {
// 		if password == "" {
// 			logging.DebugPrint(s.Debug, "Import | export is from different account, so prompting for password", common.MaxDebugChars)
// 			fmt.Print("password: ")
//
// 			var bytePassword []byte
// 			bytePassword, err = term.ReadPassword(int(syscall.Stdin))
//
// 			fmt.Println()
//
// 			if err == nil {
// 				password = string(bytePassword)
// 			} else {
// 				return
// 			}
// 		} else {
// 			logging.DebugPrint(s.Debug, "Import | export is from different account and using supplied password", common.MaxDebugChars)
// 		}
//
// 		if strings.TrimSpace(password) == "" {
// 			err = fmt.Errorf("password not defined")
// 			return
// 		}
//
// 		mk, _, err = crypto.GenerateMasterKeyAndServerPassword004(crypto.GenerateEncryptedPasswordInput{
// 			UserPassword:  password,
// 			Identifier:    keyParams.Identifier,
// 			PasswordNonce: keyParams.PwNonce,
// 			// Version:       keyParams.Version,
// 			Debug: s.Debug,
// 		})
// 		if err != nil {
// 			return
// 		}
// 	}
//
// 	// retrieve items and itemskey from export
// 	var exportsEncItemsKeys EncryptedItems
//
// 	var exportedEncItems EncryptedItems
//
// 	for x := range encItemsToImport {
// 		if encItemsToImport[x].ContentType == common.SNItemTypeItemsKey {
// 			logging.DebugPrint(s.Debug, fmt.Sprintf("Import | SN|ItemsKey loaded from export %s", encItemsToImport[x].UUID), common.MaxDebugChars)
//
// 			exportsEncItemsKeys = append(exportsEncItemsKeys, encItemsToImport[x])
//
// 			continue
// 		}
//
// 		exportedEncItems = append(exportedEncItems, encItemsToImport[x])
// 		logging.DebugPrint(s.Debug, fmt.Sprintf("Import | getting exported item %s %s",
// 			encItemsToImport[x].ContentType,
// 			encItemsToImport[x].UUID), common.MaxDebugChars)
// 	}
//
// 	// re-encrypt items
// 	if len(exportedEncItems) == 0 {
// 		err = fmt.Errorf("no items were found in export")
//
// 		return
// 	}
//
// 	var exportsItemsKeys ItemsKeys
//
// 	if len(exportsEncItemsKeys) == 0 {
// 		err = fmt.Errorf("invalid export: no ItemsKey %w", err)
// 		return
// 	}
//
// 	exportsItemsKeys, err = exportsEncItemsKeys.DecryptAndParseItemsKeys(mk, s.Debug)
// 	if err != nil {
// 		err = fmt.Errorf("invalid export: failed to decrypt ItemsKey %w", err)
// 		return
// 	}
//
// 	// s.ImporterItemsKeys = exportsItemsKeys
// 	items, err = exportedEncItems.DecryptAndParse(s)
// 	// s.ImporterItemsKeys = ItemsKeys{}
//
// 	return
// }

// Import steps are:
// - decrypt items in current file (derive master key based on username, password nonce)
// - create a new items key and reencrypt all items
// - set items key to be same updatedtimestamp in order to replace existing.
// func (s *session.Session) Import(path string, syncToken string, password string) (items EncryptedItems, itemsKey ItemsKey, err error) {
// 	exportItems, err := decryptExport(s, path, password)
// 	if err != nil {
// 		return
// 	}
//
// 	logging.DebugPrint(s.Debug, fmt.Sprintf("Import | export file returned %d items", len(exportItems)), common.MaxDebugChars)
//
// 	// This is already set when decrypting Export
//
// 	// retrieve all existing items from SN
// 	so, err := Sync(SyncInput{
// 		Session:   s,
// 		SyncToken: "",
// 	})
// 	if err != nil {
// 		return
// 	}
//
// 	logging.DebugPrint(s.Debug, fmt.Sprintf("Import | initial sync loaded %d items from SN", len(so.Items)), common.MaxDebugChars)
//
// 	// sync will override the default items key with the initial one found
// 	existingItems, err := so.Items.DecryptAndParse(s)
// 	if err != nil {
// 		return
// 	}
//
// 	// determine whether existing or exported items should be resynced...
// 	// - if export and existing have same last updated time, then just choose exported version
// 	var existingToReencrypt Items
//
// 	var exportedToReencrypt Items
//
// 	for x := range existingItems {
// 		var match bool
//
// 		for y := range exportItems {
// 			// check if we have a match for existing item and exported item
// 			if existingItems[x].GetUUID() == exportItems[y].GetUUID() && exportItems[y].GetContentType() != common.SNItemTypeItemsKey {
// 				logging.DebugPrint(s.Debug, fmt.Sprintf("Import | matching item found %s %s",
// 					existingItems[x].GetContentType(), existingItems[x].GetUUID()), common.MaxDebugChars)
//
// 				match = true
//
// 				if existingItems[x].GetUpdatedAtTimestamp() > exportItems[y].GetUpdatedAtTimestamp() {
// 					logging.DebugPrint(s.Debug, fmt.Sprintf("Import | existing %s %s newer than item to encrypt",
// 						existingItems[x].GetContentType(),
// 						existingItems[x].GetUUID()), common.MaxDebugChars)
// 					// if existing item is newer, then re-encrypt existing and add to list
// 					existingToReencrypt = append(existingToReencrypt, existingItems[x])
//
// 					var identical, unsupported bool
// 					// if exported item's content differs, then add also, and deal with conflict during sync
// 					identical, unsupported, err = compareItems(CompareItemsInput{
// 						Session:   s,
// 						FirstItem: existingItems[x],
// 						// FirstItemsKey:  s.DefaultItemsKey,
// 						SecondItem: exportItems[y],
// 						// SecondItemsKey: exportsItemsKey,
// 					})
// 					if err != nil {
// 						return
// 					}
//
// 					// if we're able to compare items, and they differ, then we'll add this item to intentionally
// 					// conflict on sync and be created as a conflicted copy
// 					if !identical && !unsupported {
// 						exportedToReencrypt = append(exportedToReencrypt, exportItems[y])
// 					}
// 				} else if existingItems[x].GetUpdatedAtTimestamp() == exportItems[y].GetUpdatedAtTimestamp() {
// 					// if existing item same age, then choose exported version that's already encrypted with new key
// 					exportedToReencrypt = append(exportedToReencrypt, exportItems[y])
// 				} else {
// 					// (exported cannot be newer than existing item)
// 					panic(fmt.Sprintf("exported %s %s found to be newer than server version",
// 						existingItems[x].GetContentType(),
// 						existingItems[x].GetUUID()))
// 				}
// 			}
// 		}
//
// 		// if we didn't find a match for the item in the export (and it's not a key) then add to final list
// 		if !match && existingItems[x].GetContentType() != common.SNItemTypeItemsKey {
// 			logging.DebugPrint(s.Debug, fmt.Sprintf("Import | no match found for existing item %s %s so add to items to re-encrypt",
// 				existingItems[x].GetContentType(),
// 				existingItems[x].GetUUID()), common.MaxDebugChars)
//
// 			existingToReencrypt = append(existingToReencrypt, existingItems[x])
// 		}
// 	}
//
// 	// loop through items to import and import any non Items Key (already handled) that doesn't exist in cache
// 	for y := range exportItems {
// 		var found bool
//
// 		for x := range existingItems {
// 			if exportItems[y].GetUUID() == existingItems[x].GetUUID() {
// 				found = true
//
// 				break
// 			}
// 		}
//
// 		if !found {
// 			exportedToReencrypt = append(exportedToReencrypt, exportItems[y])
// 		}
// 	}
//
// 	// create new items key and encrypt using current session's master key
// 	nik := NewItemsKey()
// 	nik.UUID = s.DefaultItemsKey.UUID
// 	nik.UpdatedAtTimestamp = s.DefaultItemsKey.UpdatedAtTimestamp
// 	nik.UpdatedAt = s.DefaultItemsKey.UpdatedAt
//
// 	// combine all items to reencrypt
// 	f := append(exportedToReencrypt, existingToReencrypt...)
//
// 	rf, err := f.Encrypt(s, nik)
// 	if err != nil {
// 		return
// 	}
//
// 	eNik, err := EncryptItemsKey(nik, s, false)
// 	if err != nil {
// 		return
// 	}
//
// 	eNiks := EncryptedItems{
// 		eNik,
// 	}
//
// 	// add existing items (re-encrypted) to the re-encrypted exported items
// 	// preprend new items key to the list of re-encrypted items
// 	rfa := append(eNiks, rf...)
//
// 	// set default items key to new items key
// 	s.DefaultItemsKey = nik
// 	// reset items keys slice to have only new
// 	s.ItemsKeys = ItemsKeys{s.DefaultItemsKey}
//
// 	so2, err := Sync(SyncInput{
// 		Session:   s,
// 		SyncToken: so.SyncToken,
// 		Items:     rfa,
// 	})
// 	if err != nil {
// 		return
// 	}
//
// 	// check initial items key differs from the new
// 	for x := range so.SavedItems {
// 		if so.SavedItems[x].ContentType == common.SNItemTypeItemsKey {
// 			itemsKey, err = so.SavedItems[x].Decrypt(s.MasterKey)
// 			if err != nil {
// 				return
// 			}
// 		}
// 	}
//
// 	items = append(so2.SavedItems, so.SavedItems...)
// 	itemsKey = nik
//
// 	return
// }

func readJSON(filePath string) (items EncryptedItems, kp auth.KeyParams, err error) {
	file, err := os.ReadFile(filePath)
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

	return eif.Items, eif.KeyParams, err
}

type EncryptedItemsFile struct {
	Items     EncryptedItems `json:"items"`
	KeyParams auth.KeyParams `json:"keyParams"`
}

func UpsertReferences(existing, new ItemReferences) ItemReferences {
	res := existing

	if len(existing) == 0 {
		return new
	}

	for _, newRef := range new {
		var found bool

		for _, existingRef := range existing {
			if existingRef.UUID == newRef.UUID {
				found = true
			}
		}

		if !found {
			res = append(res, newRef)
		}
	}

	return res
}

func (iks ItemsKeys) Latest() ItemsKey {
	var l ItemsKey
	for _, ik := range iks {
		if ik.CreatedAtTimestamp > l.CreatedAtTimestamp {
			l = ik
		}
	}

	return l
}

// GenUUID generates a unique identifier required when creating a new item.
func GenUUID() string {
	newUUID := uuid.New()
	return newUUID.String()
}

func DedupeItemsKeys(itemsKeys []ItemsKey) (output []ItemsKey) {
	seen := make(map[string]int)
	for x := range itemsKeys {
		if seen[itemsKeys[x].UUID] > 0 {
			continue
		}

		seen[itemsKeys[x].UUID]++
		output = append(output, itemsKeys[x])
	}

	return output
}

func DecryptEncryptedItemKey(e EncryptedItem, encryptionKey string) (itemKey []byte, err error) {
	_, nonce, cipherText, authData := crypto.SplitContent(e.EncItemKey)
	return crypto.DecryptCipherText(cipherText, encryptionKey, nonce, authData)
}

func DecryptContent(e EncryptedItem, encryptionKey string) (content []byte, err error) {
	_, nonce, cipherText, authData := crypto.SplitContent(e.Content)

	content, err = crypto.DecryptCipherText(cipherText, encryptionKey, nonce, authData)
	if err != nil {
		return
	}

	c := string(content)

	if !slices.Contains([]string{
		common.SNItemTypeFileSafeIntegration,
		common.SNItemTypeFileSafeCredentials,
		common.SNItemTypeComponent,
		common.SNItemTypeTheme,
	}, e.ContentType) && len(c) > 250 {
		return
	}

	return
}

func CreateItemsKey() (ItemsKey, error) {
	ik := NewItemsKey()
	// creating an items key is done during registration or when exporting, in which case it will always be default
	// ik.Default = true
	// ik.Content.Default = true
	ik.CreatedAtTimestamp = time.Now().UTC().UnixMicro()
	ik.CreatedAt = time.Now().UTC().Format(common.TimeLayout)

	return ik, nil
}
