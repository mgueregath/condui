package sftp

type FileItem struct {
	Name string `json:"name"`
	Path string `json:"path"`

	IsDirectory bool `json:"isDirectory"`

	Size int64 `json:"size"`

	Mode string `json:"mode"`
}
