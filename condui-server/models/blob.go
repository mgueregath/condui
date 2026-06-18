package models

import "time"

type Blob struct {
	ID         string    `json:"id"`
	UserID     string    `json:"userId"`
	BlobType   string    `json:"blobType"`
	Version    int       `json:"version"`
	CipherText string    `json:"cipherText"`
	Nonce      string    `json:"nonce"`
	Checksum   string    `json:"checksum"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

type BlobMeta struct {
	ID        string    `json:"id"`
	BlobType  string    `json:"blobType"`
	Version   int       `json:"version"`
	Checksum  string    `json:"checksum"`
	UpdatedAt time.Time `json:"updatedAt"`
}
