package items

import (
	"fmt"
	"slices"
	"time"

	"github.com/jonhadfield/gosn-v2/common"
)

func parseSmartView(i DecryptedItem) Item {
	c := SmartView{}

	if err := populateItemCommon(&c.ItemCommon, i); err != nil {
		panic(err)
	}

	var err error

	if !c.Deleted {
		var content Content

		content, err = processContentModel(i.ContentType, i.Content)
		if err != nil {
			panic(err)
		}

		c.Content = *content.(*SmartViewContent)
	}

	return &c
}

// SmartViewContent extends TagContent with predicate support for smart filtering
type SmartViewContent struct {
	// Embed TagContent for all standard tag functionality
	TagContent
	// SmartView-specific attributes
	Predicate interface{} `json:"predicate"` // PredicateJsonForm for smart filtering
}

type SmartView struct {
	ItemCommon
	Content SmartViewContent
}

func (c SmartView) IsDefault() bool {
	return false
}

func (i Items) SmartViews() (c SmartViews) {
	for _, x := range i {
		if x.GetContentType() == common.SNItemTypeSmartTag {
			smartView := x.(*SmartView)
			c = append(c, *smartView)
		}
	}

	return c
}

func (c *SmartViews) DeDupe() {
	var encountered []string

	var deDuped SmartViews

	for _, i := range *c {
		if !slices.Contains(encountered, i.UUID) {
			deDuped = append(deDuped, i)
		}

		encountered = append(encountered, i.UUID)
	}

	*c = deDuped
}

// NewSmartView returns an Item of type SmartView without content.
func NewSmartView() SmartView {
	now := time.Now().UTC().Format(common.TimeLayout)

	var c SmartView

	c.ContentType = common.SNItemTypeSmartTag
	c.CreatedAt = now
	c.CreatedAtTimestamp = time.Now().UTC().UnixMicro()
	c.UUID = GenUUID()

	return c
}

// NewSmartViewContent returns an empty SmartView content instance.
func NewSmartViewContent() *SmartViewContent {
	c := &SmartViewContent{}
	c.SetUpdateTime(time.Now().UTC())

	return c
}

type SmartViews []SmartView

func (c SmartViews) Validate() error {
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
			case item.Content.Title == "":
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

func (c SmartView) IsDeleted() bool {
	return c.Deleted
}

func (c *SmartView) SetDeleted(d bool) {
	c.Deleted = d
}

func (c SmartView) GetContent() Content {
	return &c.Content
}

func (c *SmartView) SetContent(cc Content) {
	c.Content = *cc.(*SmartViewContent)
}

func (c SmartView) GetItemsKeyID() string {
	return c.ItemsKeyID
}

func (c SmartView) GetUUID() string {
	return c.UUID
}

func (c SmartView) GetDuplicateOf() string {
	return c.DuplicateOf
}

func (c *SmartView) SetUUID(u string) {
	c.UUID = u
}

func (c SmartView) GetContentType() string {
	return c.ContentType
}

func (c SmartView) GetCreatedAt() string {
	return c.CreatedAt
}

func (c *SmartView) SetCreatedAt(ca string) {
	c.CreatedAt = ca
}

func (c SmartView) GetUpdatedAt() string {
	return c.UpdatedAt
}

func (c *SmartView) SetUpdatedAt(ca string) {
	c.UpdatedAt = ca
}

func (c SmartView) GetCreatedAtTimestamp() int64 {
	return c.CreatedAtTimestamp
}

func (c *SmartView) SetCreatedAtTimestamp(ca int64) {
	c.CreatedAtTimestamp = ca
}

func (c SmartView) GetUpdatedAtTimestamp() int64 {
	return c.UpdatedAtTimestamp
}

func (c *SmartView) SetUpdatedAtTimestamp(ca int64) {
	c.UpdatedAtTimestamp = ca
}

func (c *SmartView) SetContentType(ct string) {
	c.ContentType = ct
}

func (c SmartView) GetContentSize() int {
	return c.ContentSize
}

func (c *SmartView) SetContentSize(s int) {
	c.ContentSize = s
}

// SmartViewContent delegates to embedded TagContent for most methods
func (cc *SmartViewContent) GetUpdateTime() (time.Time, error) {
	return cc.TagContent.GetUpdateTime()
}

func (cc *SmartViewContent) SetUpdateTime(uTime time.Time) {
	cc.TagContent.SetUpdateTime(uTime)
}

func (cc SmartViewContent) GetTitle() string {
	return cc.TagContent.GetTitle()
}

func (cc *SmartViewContent) SetTitle(title string) {
	cc.TagContent.SetTitle(title)
}

func (cc *SmartViewContent) GetAppData() AppDataContent {
	return cc.TagContent.GetAppData()
}

func (cc *SmartViewContent) SetAppData(data AppDataContent) {
	cc.TagContent.SetAppData(data)
}

func (cc SmartViewContent) References() ItemReferences {
	return cc.TagContent.References()
}

func (cc *SmartViewContent) SetReferences(input ItemReferences) {
	cc.TagContent.SetReferences(input)
}

// GetPredicate returns the smart filtering predicate
func (cc SmartViewContent) GetPredicate() interface{} {
	return cc.Predicate
}

// SetPredicate sets the smart filtering predicate
func (cc *SmartViewContent) SetPredicate(predicate interface{}) {
	cc.Predicate = predicate
}