package models

type Connection struct {
	ID string `json:"id"`

	FolderID *string `json:"folderId,omitempty"`

	Name string `json:"name"`

	Host string `json:"host"`

	Port int `json:"port"`

	Username string `json:"username"`

	AuthType string `json:"authType"`

	Password *string `json:"password,omitempty"`

	PrivateKeyPath *string `json:"privateKeyPath,omitempty"`

	Color *string `json:"color,omitempty"`
}
