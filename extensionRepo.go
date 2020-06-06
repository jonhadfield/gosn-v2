package gosn

import (
	"fmt"
	"time"
)

func parseExtensionRepo(i DecryptedItem) Item {
	c := ExtensionRepo{}
	c.UUID = i.UUID
	c.ContentType = i.ContentType
	c.Deleted = i.Deleted
	c.UpdatedAt = i.UpdatedAt
	c.CreatedAt = i.CreatedAt
	c.ContentSize = len(i.Content)

	var err error

	if !c.Deleted {
		var content Content

		content, err = processContentModel(i.ContentType, i.Content)
		if err != nil {
			panic(err)
		}

		c.Content = content.(ExtensionRepoContent)
	}

	var cAt, uAt time.Time

	cAt, err = time.Parse(timeLayout, i.CreatedAt)
	if err != nil {
		panic(err)
	}

	c.CreatedAt = cAt.Format(timeLayout)

	uAt, err = time.Parse(timeLayout, i.UpdatedAt)
	if err != nil {
		panic(err)
	}

	c.UpdatedAt = uAt.Format(timeLayout)

	return &c
}


type ExtensionRepoContent struct {
	ItemReferences     ItemReferences `json:"references"`
	AppData            AppDataContent `json:"appData"`
	Name               string         `json:"name"`
	DissociatedItemIds []string       `json:"disassociatedItemIds"`
	AssociatedItemIds  []string       `json:"associatedItemIds"`
	Active             interface{}    `json:"active"`
}

type ExtensionRepo struct {
	ItemCommon
	Content ExtensionRepoContent
}

func (i Items) ExtensionRepo() (c ExtensionRepos) {
	for _, x := range i {
		if x.GetContentType() == "ExtensionRepo" {
			component := x.(*ExtensionRepo)
			c = append(c, *component)
		}
	}

	return c
}

func (c *ExtensionRepos) DeDupe() {
	var encountered []string

	var deDuped ExtensionRepos

	for _, i := range *c {
		if !stringInSlice(i.UUID, encountered, true) {
			deDuped = append(deDuped, i)
		}

		encountered = append(encountered, i.UUID)
	}

	*c = deDuped
}

// NewExtensionRepo returns an Item of type ExtensionRepo without content
func NewExtensionRepo() ExtensionRepo {
	now := time.Now().UTC().Format(timeLayout)

	var c ExtensionRepo

	c.ContentType = "ExtensionRepo"
	c.CreatedAt = now
	c.UpdatedAt = now
	c.UUID = GenUUID()

	return c
}

// NewTagContent returns an empty Tag content instance
func NewExtensionRepoContent() *ExtensionRepoContent {
	c := &ExtensionRepoContent{}
	c.SetUpdateTime(time.Now().UTC())

	return c
}

type ExtensionRepos []ExtensionRepo

func (c ExtensionRepos) Validate() error {
	var updatedTime time.Time

	var err error

	for _, item := range c {
		// validate content if being added
		if !item.Deleted {
			updatedTime, err = item.Content.GetUpdateTime()
			if err != nil {
				return err
			}

			switch {
			case item.Content.Name == "":
				err = fmt.Errorf("failed to create \"%s\" due to missing title: \"%s\"",
					item.ContentType, item.UUID)
			case updatedTime.IsZero():
				err = fmt.Errorf("failed to create \"%s\" due to missing content updated time: \"%s\"",
					item.ContentType, item.Content.GetTitle())
			case item.CreatedAt == "":
				err = fmt.Errorf("failed to create \"%s\" due to missing created at date: \"%s\"",
					item.ContentType, item.Content.GetTitle())
			}

			if err != nil {
				return err
			}
		}
	}

	return err
}

func (c ExtensionRepo) IsDeleted() bool {
	return c.Deleted
}

func (c *ExtensionRepo) SetDeleted(d bool) {
	c.Deleted = d
}

func (c ExtensionRepo) GetContent() Content {
	return &c.Content
}

func (c *ExtensionRepo) SetContent(cc Content) {
	c.Content = cc.(ExtensionRepoContent)
}

func (c ExtensionRepo) GetUUID() string {
	return c.UUID
}

func (c *ExtensionRepo) SetUUID(u string) {
	c.UUID = u
}

func (c ExtensionRepo) GetContentType() string {
	return c.ContentType
}

func (c ExtensionRepo) GetCreatedAt() string {
	return c.CreatedAt
}

func (c *ExtensionRepo) SetCreatedAt(ca string) {
	c.CreatedAt = ca
}

func (c ExtensionRepo) GetUpdatedAt() string {
	return c.UpdatedAt
}

func (c *ExtensionRepo) SetUpdatedAt(ca string) {
	c.UpdatedAt = ca
}

func (c *ExtensionRepo) SetContentType(ct string) {
	c.ContentType = ct
}

func (c ExtensionRepo) GetContentSize() int {
	return c.ContentSize
}

func (c *ExtensionRepo) SetContentSize(s int) {
	c.ContentSize = s
}

func (cc *ExtensionRepoContent) AssociateItems(newItems []string) {
	// add to associated item ids
	for _, newRef := range newItems {
		var existingFound bool

		var existingDFound bool

		for _, existingRef := range cc.AssociatedItemIds {
			if existingRef == newRef {
				existingFound = true
			}
		}

		for _, existingDRef := range cc.DissociatedItemIds {
			if existingDRef == newRef {
				existingDFound = true
			}
		}

		// add reference if it doesn't exist
		if !existingFound {
			cc.AssociatedItemIds = append(cc.AssociatedItemIds, newRef)
		}

		// remove reference (from disassociated) if it does exist in that list
		if existingDFound {
			cc.DissociatedItemIds = removeStringFromSlice(newRef, cc.DissociatedItemIds)
		}
	}
}

func (cc *ExtensionRepoContent) GetItemAssociations() []string {
	return cc.AssociatedItemIds
}

func (cc *ExtensionRepoContent) GetItemDisassociations() []string {
	return cc.DissociatedItemIds
}

func (cc *ExtensionRepoContent) DisassociateItems(itemsToRemove []string) {
	// remove from associated item ids
	for _, delRef := range itemsToRemove {
		var existingFound bool

		for _, existingRef := range cc.AssociatedItemIds {
			if existingRef == delRef {
				existingFound = true
			}
		}

		// remove reference (from disassociated) if it does exist in that list
		if existingFound {
			cc.AssociatedItemIds = removeStringFromSlice(delRef, cc.AssociatedItemIds)
		}
	}
}

func (cc *ExtensionRepoContent) GetUpdateTime() (time.Time, error) {
	if cc.AppData.OrgStandardNotesSN.ClientUpdatedAt == "" {
		return time.Time{}, fmt.Errorf("ClientUpdatedAt not set")
	}

	return time.Parse(timeLayout, cc.AppData.OrgStandardNotesSN.ClientUpdatedAt)
}

func (cc *ExtensionRepoContent) SetUpdateTime(uTime time.Time) {
	cc.AppData.OrgStandardNotesSN.ClientUpdatedAt = uTime.Format(timeLayout)
}

func (cc ExtensionRepoContent) GetTitle() string {
	return ""
}

func (cc *ExtensionRepoContent) GetName() string {
	return cc.Name
}

func (cc *ExtensionRepoContent) GetActive() bool {
	return cc.Active.(bool)
}

func (cc *ExtensionRepoContent) SetTitle(title string) {
}

func (cc *ExtensionRepoContent) GetAppData() AppDataContent {
	return cc.AppData
}

func (cc *ExtensionRepoContent) SetAppData(data AppDataContent) {
	cc.AppData = data
}

func (cc ExtensionRepoContent) References() ItemReferences {
	return cc.ItemReferences
}

func (cc *ExtensionRepoContent) UpsertReferences(input ItemReferences) {
	panic("implement me")
}

func (cc *ExtensionRepoContent) SetReferences(input ItemReferences) {
	panic("implement me")
}
