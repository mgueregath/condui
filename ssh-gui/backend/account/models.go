package account

type AccountStatus struct {
	LoggedIn  bool           `json:"loggedIn"`
	Email     string         `json:"email"`
	Tier      string         `json:"tier"`
	ServerURL string         `json:"serverUrl"`
	LastSync  string         `json:"lastSync"`
	// Limits mirrors condui-server's tier_limits table for the user's
	// current tier (e.g. {"blobs":5,"devices":1,"connections":5}), fetched
	// from /auth/me. It's the only source of truth for these numbers —
	// nothing in this app hardcodes a limit of its own.
	Limits map[string]int `json:"limits,omitempty"`
}

type ShareInfo struct {
	ID             string `json:"id"`
	OwnerEmail     string `json:"ownerEmail"`
	RecipientEmail string `json:"recipientEmail"`
	BlobID         string `json:"blobId"`
	EncryptedKey   string `json:"encryptedKey"`
	Permissions    string `json:"permissions"`
	Status         string `json:"status"`
}

type BlobMeta struct {
	ID        string `json:"id"`
	BlobType  string `json:"blobType"`
	Version   int    `json:"version"`
	Checksum  string `json:"checksum"`
	UpdatedAt string `json:"updatedAt"`
}

type BlobPayload struct {
	ID         string `json:"id"`
	BlobType   string `json:"blobType"`
	CipherText string `json:"cipherText"`
	Nonce      string `json:"nonce"`
	Checksum   string `json:"checksum"`
	Version    int    `json:"version"`
	// ItemCount lets the server enforce tier limits (e.g. "connections")
	// on data it can't see the contents of, since blobs are encrypted
	// client-side. Only meaningful for BlobType "connections".
	ItemCount int `json:"itemCount,omitempty"`
}

// loginRequest / loginResponse are used by the HTTP client
type loginRequest struct {
	Email      string `json:"email"`
	Password   string `json:"password"`
	DeviceName string `json:"deviceName"`
}

type loginResponse struct {
	AccessToken  string      `json:"accessToken"`
	RefreshToken string      `json:"refreshToken"`
	User         userPayload `json:"user"`
}

type userPayload struct {
	ID           string         `json:"id"`
	Email        string         `json:"email"`
	Tier         string         `json:"tier"`
	PublicKey    string         `json:"publicKey"`
	IdentityBlob string         `json:"identityBlob"`
	Limits       map[string]int `json:"limits,omitempty"`
}

type registerRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type refreshRequest struct {
	RefreshToken string `json:"refreshToken"`
}

type refreshResponse struct {
	AccessToken string `json:"accessToken"`
}

type identityRequest struct {
	PublicKey    string `json:"publicKey"`
	IdentityBlob string `json:"identityBlob"`
}

type shareRequest struct {
	BlobID         string `json:"blobId"`
	RecipientEmail string `json:"recipientEmail"`
	Permissions    string `json:"permissions"`
	EncryptedKey   string `json:"encryptedKey"`
}

type shareResponse struct {
	ID             string `json:"id"`
	RecipientEmail string `json:"recipientEmail"`
	BlobID         string `json:"blobId"`
	Permissions    string `json:"permissions"`
	Status         string `json:"status"`
}
