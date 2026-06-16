package models

import "time"

type ShareInvite struct {
	ID             string     `json:"id"`
	OwnerID        string     `json:"ownerId"`
	OwnerEmail     string     `json:"ownerEmail,omitempty"`
	RecipientEmail string     `json:"recipientEmail"`
	BlobID         string     `json:"blobId"`
	EncryptedKey   string     `json:"encryptedKey,omitempty"`
	Permissions    string     `json:"permissions"` // "read" | "write"
	Status         string     `json:"status"`      // "pending" | "accepted" | "revoked"
	ExpiresAt      *time.Time `json:"expiresAt,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
}
