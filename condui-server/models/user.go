package models

import "time"

const (
	TierFree = "free"
	TierPro  = "pro"
)

type User struct {
	ID             string     `json:"id"`
	Email          string     `json:"email"`
	PasswordHash   string     `json:"-"`
	Tier           string     `json:"tier"`
	TierExpiresAt  *time.Time `json:"tierExpiresAt,omitempty"`
	PublicKey      string     `json:"publicKey,omitempty"`
	IdentityBlob   string     `json:"identityBlob,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
}

// EffectiveTier returns "free" if the pro tier has expired, otherwise Tier.
func (u *User) EffectiveTier() string {
	if u.TierExpiresAt != nil && time.Now().After(*u.TierExpiresAt) {
		return TierFree
	}
	return u.Tier
}

type RefreshToken struct {
	ID         string
	UserID     string
	TokenHash  string
	DeviceName string
	ExpiresAt  time.Time
	CreatedAt  time.Time
}
