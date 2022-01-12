package postman2

import (
	"encoding/json"
	"strings"

	"github.com/grokify/mogo/errors/errorsutil"
	"github.com/grokify/mogo/net/httputilmore"
)

const (
	SchemaURL210 = "https://schema.getpostman.com/json/collection/v2.1.0/collection.json"
	SchemaURL200 = "https://schema.getpostman.com/json/collection/v2.0.0/collection.json"
)

type Collection struct {
	Info  CollectionInfo `json:"info"`
	Item  []*Item        `json:"item"`
	Event []Event        `json:"event,omitempty"`
}

func NewCollectionFromBytes(data []byte) (Collection, error) {
	col := Collection{}
	err := json.Unmarshal(data, &col)
	if err != nil {
		err = errorsutil.Wrap(err, "spectrum.postman2.NewCollectionFromBytes << json.Unmarshal")
		return col, err
	}
	col.Inflate()
	return col, nil
}

func (col *Collection) GetOrNewFolder(folderName string) *Item {
	for _, folder := range col.Item {
		if folder.Name == folderName {
			return folder
		}
	}
	folder := &Item{Name: folderName, Item: []*Item{}}
	col.Item = append(col.Item, folder)
	return folder
}

func (col *Collection) SetFolder(newFolder *Item) {
	if newFolder == nil || len(strings.TrimSpace(newFolder.Name)) == 0 {
		return
	}
	for i, folder := range col.Item {
		if newFolder.Name == folder.Name {
			col.Item[i] = newFolder
			return
		}
	}
	col.Item = append(col.Item, newFolder)
}

func (col *Collection) Inflate() {
	col.Info.Schema = strings.TrimSpace(col.Info.Schema)
	if len(col.Info.Schema) == 0 {
		col.Info.Schema = SchemaURL210
	}
	col.InflateRawURLs()
}

func (col *Collection) InflateRawURLs() {
	for _, folder := range col.Item {
		for j, api := range folder.Item {
			if api.Request.URL.IsRawOnly() &&
				len(strings.TrimSpace(api.Request.URL.Raw)) > 0 {
				url := NewURL(strings.TrimSpace(api.Request.URL.Raw))
				url.Auth = api.Request.URL.Auth
				url.Variable = api.Request.URL.Variable
				folder.Item[j].Request.URL = &url
			}
		}
	}
}

type CollectionInfo struct {
	Name        string `json:"name,omitempty"`
	PostmanID   string `json:"_postman_id,omitempty"`
	Description string `json:"description,omitempty"`
	Schema      string `json:"schema,omitempty"`
}

// Item can represent a folder or an API
type Item struct {
	Name        string       `json:"name,omitempty"`                 // Folder,Operation
	Description *Description `json:"description,omitempty"`          // Folder
	Item        []*Item      `json:"item,omitempty"`                 // Folder
	IsSubFolder bool         `json:"_postman_isSubFolder,omitempty"` // Folder
	Event       []Event      `json:"event,omitempty"`                // Operation
	Request     *Request     `json:"request,omitempty"`              // Operation
}

func (item *Item) UpsertSubItem(newItem *Item) {
	if newItem == nil || len(strings.TrimSpace(newItem.Name)) == 0 {
		return
	}
	for i, itemTry := range item.Item {
		if itemTry.Name == newItem.Name {
			item.Item[i] = newItem
			return
		}
	}
	item.Item = append(item.Item, newItem)
	return
}

type Description struct {
	Content string `json:"content,omitempty"`
	Type    string `json:"type,omitempty"`
	Version string `json:"version,omitempty"`
}

func (desc *Description) Inflate() {
	desc.Content = strings.TrimSpace(desc.Content)
	desc.Type = strings.TrimSpace(desc.Type)
	if len(desc.Content) > 0 && len(desc.Type) == 0 {
		desc.Type = httputilmore.ContentTypeTextPlain
	}
}

type Event struct {
	Listen string `json:"listen"`
	Script Script `json:"script"`
}

type Script struct {
	Type string   `json:"type,omitempty"`
	Exec []string `json:"exec,omitmpety"`
}
