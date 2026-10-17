package items

import (
	"fmt"
	"strings"
	"syscall"

	"github.com/jonhadfield/gosn-v2/common"
	"github.com/jonhadfield/gosn-v2/crypto"
	"github.com/jonhadfield/gosn-v2/log"
	"github.com/jonhadfield/gosn-v2/session"
	"golang.org/x/term"
)

// SessionItemsKeyFromItemsKey converts an ItemsKey model into the session form
// used by encrypt/decrypt helpers.
func SessionItemsKeyFromItemsKey(ik ItemsKey) session.SessionItemsKey {
	return session.SessionItemsKey{
		UUID:               ik.UUID,
		ItemsKey:           ik.ItemsKey,
		Version:            ik.Version,
		Default:            ik.Default,
		CreatedAt:          ik.CreatedAt,
		UpdatedAt:          ik.UpdatedAt,
		CreatedAtTimestamp: ik.CreatedAtTimestamp,
		UpdatedAtTimestamp: ik.UpdatedAtTimestamp,
		Deleted:            ik.Deleted,
	}
}

// Export syncs all items, re-encrypts them with a fresh items key that keeps
// the session default key UUID/timestamps, and writes a Standard Notes 004
// export file to path.
func Export(s *session.Session, path string) error {
	// We must export all items, otherwise we would update the encryption key
	// for non-exported items so they can no longer be decrypted.
	so, err := Sync(SyncInput{
		Session: s,
	})
	if err != nil {
		return err
	}

	ik := s.DefaultItemsKey

	nk, err := CreateItemsKey()
	if err != nil {
		return err
	}

	nk.UUID = ik.UUID
	nk.CreatedAt = ik.CreatedAt
	nk.UpdatedAt = ik.UpdatedAt
	nk.CreatedAtTimestamp = ik.CreatedAtTimestamp
	nk.UpdatedAtTimestamp = ik.UpdatedAtTimestamp

	nei, err := so.Items.ReEncrypt(s, session.SessionItemsKey{}, nk, s.MasterKey)
	if err != nil {
		return err
	}

	eik, err := EncryptItemsKey(SessionItemsKeyFromItemsKey(nk), s, false)
	if err != nil {
		return err
	}

	if eik.UpdatedAtTimestamp != ik.UpdatedAtTimestamp {
		return fmt.Errorf("export: encrypted items key timestamp %d does not match session default %d",
			eik.UpdatedAtTimestamp, ik.UpdatedAtTimestamp)
	}

	eik.UpdatedAtTimestamp = ik.UpdatedAtTimestamp
	eik.UpdatedAt = ik.UpdatedAt

	nei = append(EncryptedItems{eik}, nei...)

	return writeJSON(writeJSONConfig{
		session: *s,
		Path:    path,
		Debug:   s.Debug,
	}, nei)
}

func decryptExport(s *session.Session, path, password string) (items Items, err error) {
	encItemsToImport, keyParams, err := readJSON(path)
	if err != nil {
		return
	}

	log.DebugPrint(s.Debug, fmt.Sprintf("Import | read %d items from export file", len(encItemsToImport)), common.MaxDebugChars)

	mk := s.MasterKey

	// Export from a different account needs that account's master key.
	if keyParams.Identifier != s.KeyParams.Identifier || keyParams.PwNonce != s.KeyParams.PwNonce {
		if password == "" {
			log.DebugPrint(s.Debug, "Import | export is from different account, so prompting for password", common.MaxDebugChars)
			fmt.Print("password: ")

			var bytePassword []byte
			bytePassword, err = term.ReadPassword(int(syscall.Stdin))
			fmt.Println()
			if err != nil {
				return
			}
			password = string(bytePassword)
		} else {
			log.DebugPrint(s.Debug, "Import | export is from different account and using supplied password", common.MaxDebugChars)
		}

		if strings.TrimSpace(password) == "" {
			err = fmt.Errorf("password not defined")
			return
		}

		mk, _, err = crypto.GenerateMasterKeyAndServerPassword004(crypto.GenerateEncryptedPasswordInput{
			UserPassword:  password,
			Identifier:    keyParams.Identifier,
			PasswordNonce: keyParams.PwNonce,
			Debug:         s.Debug,
		})
		if err != nil {
			return
		}
	}

	var exportsEncItemsKeys EncryptedItems
	var exportedEncItems EncryptedItems

	for x := range encItemsToImport {
		if encItemsToImport[x].ContentType == common.SNItemTypeItemsKey {
			log.DebugPrint(s.Debug, fmt.Sprintf("Import | SN|ItemsKey loaded from export %s", encItemsToImport[x].UUID), common.MaxDebugChars)
			exportsEncItemsKeys = append(exportsEncItemsKeys, encItemsToImport[x])
			continue
		}

		exportedEncItems = append(exportedEncItems, encItemsToImport[x])
		log.DebugPrint(s.Debug, fmt.Sprintf("Import | getting exported item %s %s",
			encItemsToImport[x].ContentType,
			encItemsToImport[x].UUID), common.MaxDebugChars)
	}

	if len(exportedEncItems) == 0 {
		err = fmt.Errorf("no items were found in export")
		return
	}

	if len(exportsEncItemsKeys) == 0 {
		err = fmt.Errorf("invalid export: no ItemsKey")
		return
	}

	exportsItemsKeys, err := exportsEncItemsKeys.DecryptAndParseItemsKeys(mk, s.Debug)
	if err != nil {
		err = fmt.Errorf("invalid export: failed to decrypt ItemsKey: %w", err)
		return
	}

	s.ImporterItemsKeys = exportsItemsKeys
	defer func() { s.ImporterItemsKeys = nil }()

	items, err = exportedEncItems.DecryptAndParse(s)
	return
}

// Import decrypts a Standard Notes export, merges with the account's current
// items, re-encrypts under a replacement items key, and syncs the result.
func Import(s *session.Session, path string, syncToken string, password string) (items EncryptedItems, itemsKey ItemsKey, err error) {
	exportItems, err := decryptExport(s, path, password)
	if err != nil {
		return
	}

	log.DebugPrint(s.Debug, fmt.Sprintf("Import | export file returned %d items", len(exportItems)), common.MaxDebugChars)

	so, err := Sync(SyncInput{
		Session:   s,
		SyncToken: "",
	})
	if err != nil {
		return
	}

	log.DebugPrint(s.Debug, fmt.Sprintf("Import | initial sync loaded %d items from SN", len(so.Items)), common.MaxDebugChars)

	existingItems, err := so.Items.DecryptAndParse(s)
	if err != nil {
		return
	}

	var existingToReencrypt Items
	var exportedToReencrypt Items

	for x := range existingItems {
		var match bool

		for y := range exportItems {
			if existingItems[x].GetUUID() == exportItems[y].GetUUID() && exportItems[y].GetContentType() != common.SNItemTypeItemsKey {
				log.DebugPrint(s.Debug, fmt.Sprintf("Import | matching item found %s %s",
					existingItems[x].GetContentType(), existingItems[x].GetUUID()), common.MaxDebugChars)

				match = true

				switch {
				case existingItems[x].GetUpdatedAtTimestamp() > exportItems[y].GetUpdatedAtTimestamp():
					log.DebugPrint(s.Debug, fmt.Sprintf("Import | existing %s %s newer than item to encrypt",
						existingItems[x].GetContentType(),
						existingItems[x].GetUUID()), common.MaxDebugChars)
					existingToReencrypt = append(existingToReencrypt, existingItems[x])

					var identical, unsupported bool
					identical, unsupported, err = compareItems(CompareItemsInput{
						Session:    s,
						FirstItem:  existingItems[x],
						SecondItem: exportItems[y],
					})
					if err != nil {
						return
					}

					if !identical && !unsupported {
						exportedToReencrypt = append(exportedToReencrypt, exportItems[y])
					}
				case existingItems[x].GetUpdatedAtTimestamp() == exportItems[y].GetUpdatedAtTimestamp():
					exportedToReencrypt = append(exportedToReencrypt, exportItems[y])
				default:
					err = fmt.Errorf("exported %s %s found to be newer than server version",
						existingItems[x].GetContentType(),
						existingItems[x].GetUUID())
					return
				}
			}
		}

		if !match && existingItems[x].GetContentType() != common.SNItemTypeItemsKey {
			log.DebugPrint(s.Debug, fmt.Sprintf("Import | no match found for existing item %s %s so add to items to re-encrypt",
				existingItems[x].GetContentType(),
				existingItems[x].GetUUID()), common.MaxDebugChars)

			existingToReencrypt = append(existingToReencrypt, existingItems[x])
		}
	}

	for y := range exportItems {
		var found bool

		for x := range existingItems {
			if exportItems[y].GetUUID() == existingItems[x].GetUUID() {
				found = true
				break
			}
		}

		if !found {
			exportedToReencrypt = append(exportedToReencrypt, exportItems[y])
		}
	}

	nik := NewItemsKey()
	nik.UUID = s.DefaultItemsKey.UUID
	nik.UpdatedAtTimestamp = s.DefaultItemsKey.UpdatedAtTimestamp
	nik.UpdatedAt = s.DefaultItemsKey.UpdatedAt

	f := append(exportedToReencrypt, existingToReencrypt...)

	rf, err := f.Encrypt(s, SessionItemsKeyFromItemsKey(nik))
	if err != nil {
		return
	}

	eNik, err := EncryptItemsKey(SessionItemsKeyFromItemsKey(nik), s, false)
	if err != nil {
		return
	}

	rfa := append(EncryptedItems{eNik}, rf...)

	s.DefaultItemsKey = SessionItemsKeyFromItemsKey(nik)
	s.ItemsKeys = []session.SessionItemsKey{s.DefaultItemsKey}

	token := syncToken
	if token == "" {
		token = so.SyncToken
	}

	so2, err := Sync(SyncInput{
		Session:   s,
		SyncToken: token,
		Items:     rfa,
	})
	if err != nil {
		return
	}

	items = append(so2.SavedItems, so.SavedItems...)
	itemsKey = nik

	return
}
