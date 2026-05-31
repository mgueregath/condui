package models

type Folder struct {
	ID string `json:"id"`

	Name string `json:"name"`

	ParentID *string `json:"parentId,omitempty"`
}
