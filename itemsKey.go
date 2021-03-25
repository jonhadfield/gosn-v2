package gosn

// pt1: {"itemsKey":"366df581a789de771a1613d7d0289bbaff7bf4249a7dd15e458a12c361cb7b73","version":"004","references":[],"appData":{"org.standardnotes.sn":{"client_updated_at":"2020-12-15T20:18:39.334Z"}},"isDefault":true}

type ItemsKey struct {
	ItemCommon
	ItemsKey       string         `json:"itemsKey"`
	IsDefault      bool           `json:"isDefault"`
	Version        string         `json:"version"`
	ItemReferences ItemReferences `json:"references"`
	AppData        AppDataContent `json:"appData"`
}

func (k ItemsKey) GetUUID() string {
	return k.UUID
}