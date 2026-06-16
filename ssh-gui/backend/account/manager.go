package account

import (
	"fmt"
	"runtime"
	"sync"
	"time"
)

// State holds the in-memory account credentials.
type State struct {
	ServerURL    string
	AccessToken  string
	RefreshToken string
	UserID       string
	Email        string
	Tier         string
	PublicKey    string
	IdentityBlob string // encrypted private key blob
}

type Manager struct {
	mu    sync.RWMutex
	state State
}

func NewManager() *Manager {
	return &Manager{}
}

// Load injects state restored from persistent storage.
func (m *Manager) Load(s State) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.state = s
}

// GetState returns a copy of current state (for persistence).
func (m *Manager) GetState() State {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state
}

func (m *Manager) GetStatus(lastSync string) AccountStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return AccountStatus{
		LoggedIn:  m.state.AccessToken != "",
		Email:     m.state.Email,
		Tier:      m.state.Tier,
		ServerURL: m.state.ServerURL,
		LastSync:  lastSync,
	}
}

func (m *Manager) IsLoggedIn() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state.AccessToken != ""
}

func (m *Manager) Register(serverURL, email, password string) error {
	client := newAPIClient(serverURL, "")
	return client.register(email, password)
}

// Login authenticates and stores credentials in memory. Returns updated State for persistence.
func (m *Manager) Login(serverURL, email, password string) (State, error) {
	client := newAPIClient(serverURL, "")
	resp, err := client.login(email, password, deviceName())
	if err != nil {
		return State{}, err
	}

	s := State{
		ServerURL:    serverURL,
		AccessToken:  resp.AccessToken,
		RefreshToken: resp.RefreshToken,
		UserID:       resp.User.ID,
		Email:        resp.User.Email,
		Tier:         resp.User.Tier,
		PublicKey:    resp.User.PublicKey,
	}

	m.mu.Lock()
	m.state = s
	m.mu.Unlock()
	return s, nil
}

// Logout clears in-memory state. Returns server URL and tokens for server-side logout.
func (m *Manager) Logout() (serverURL, accessToken, refreshToken string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	serverURL = m.state.ServerURL
	accessToken = m.state.AccessToken
	refreshToken = m.state.RefreshToken
	m.state = State{}
	return
}

// RefreshAccessToken gets a new access token and re-fetches /me so the
// in-memory tier always reflects the current server value (including expiry).
// Returns updated State for persistence.
func (m *Manager) RefreshAccessToken() (State, error) {
	m.mu.RLock()
	s := m.state
	m.mu.RUnlock()

	if s.RefreshToken == "" {
		return State{}, fmt.Errorf("not logged in")
	}

	client := newAPIClient(s.ServerURL, "")
	newToken, err := client.refreshToken(s.RefreshToken)
	if err != nil {
		return State{}, err
	}

	// Fetch current user info (tier may have changed or expired)
	meClient := newAPIClient(s.ServerURL, newToken)
	me, err := meClient.getMe()
	if err != nil {
		// Non-fatal: keep the old tier, at least the token is fresh
		m.mu.Lock()
		m.state.AccessToken = newToken
		updated := m.state
		m.mu.Unlock()
		return updated, nil
	}

	m.mu.Lock()
	m.state.AccessToken = newToken
	m.state.Tier = me.Tier
	updated := m.state
	m.mu.Unlock()
	return updated, nil
}

// SetupIdentity generates an X25519 keypair, encrypts privkey with masterKey,
// and uploads to server. Returns updated State for persistence.
func (m *Manager) SetupIdentity(masterKey []byte) (State, error) {
	m.mu.RLock()
	s := m.state
	m.mu.RUnlock()

	if s.PublicKey != "" {
		return s, nil // already set up
	}

	pubB64, privB64, err := GenerateX25519Keypair()
	if err != nil {
		return State{}, err
	}

	encryptedPriv, err := EncryptField(masterKey, privB64)
	if err != nil {
		return State{}, err
	}

	client := newAPIClient(s.ServerURL, s.AccessToken)
	if err := client.updateIdentity(pubB64, encryptedPriv); err != nil {
		return State{}, err
	}

	m.mu.Lock()
	m.state.PublicKey = pubB64
	m.state.IdentityBlob = encryptedPriv
	updated := m.state
	m.mu.Unlock()
	return updated, nil
}

// SyncConnections encrypts and uploads connections JSON to the server.
func (m *Manager) SyncConnections(connectionsJSON []byte, masterKey []byte) error {
	m.mu.RLock()
	s := m.state
	m.mu.RUnlock()

	if s.AccessToken == "" {
		return fmt.Errorf("not logged in")
	}

	cipherB64, nonceB64, checksumB64, err := EncryptBlob(masterKey, connectionsJSON)
	if err != nil {
		return err
	}

	blobID := "connections-" + s.UserID
	client := newAPIClient(s.ServerURL, s.AccessToken)
	return client.upsertBlob(BlobPayload{
		ID:         blobID,
		BlobType:   "connections",
		CipherText: cipherB64,
		Nonce:      nonceB64,
		Checksum:   checksumB64,
	})
}

// FetchConnections downloads and decrypts the connections blob.
func (m *Manager) FetchConnections(masterKey []byte) ([]byte, error) {
	m.mu.RLock()
	s := m.state
	m.mu.RUnlock()

	blobID := "connections-" + s.UserID
	client := newAPIClient(s.ServerURL, s.AccessToken)
	blob, err := client.getBlob(blobID)
	if err != nil {
		return nil, err
	}
	return DecryptBlob(masterKey, blob.CipherText, blob.Nonce)
}

// ShareConnection encrypts a connection and creates a share invite.
func (m *Manager) ShareConnection(connectionJSON []byte, recipientEmail string, readOnly bool, masterKey []byte) error {
	m.mu.RLock()
	s := m.state
	m.mu.RUnlock()

	client := newAPIClient(s.ServerURL, s.AccessToken)

	recipientPubKey, err := client.getRecipientPublicKey(recipientEmail)
	if err != nil {
		return fmt.Errorf("recipient not found or has no public key: %w", err)
	}

	cipherB64, nonceB64, checksumB64, err := EncryptBlob(masterKey, connectionJSON)
	if err != nil {
		return err
	}

	blobID := fmt.Sprintf("shared-%s-%d", s.UserID, time.Now().UnixNano())
	if err := client.upsertBlob(BlobPayload{
		ID:       blobID,
		BlobType: "shared_connection",
		CipherText: cipherB64,
		Nonce:      nonceB64,
		Checksum:   checksumB64,
	}); err != nil {
		return err
	}

	encryptedKeyForRecipient, err := WrapKey(recipientPubKey, masterKey)
	if err != nil {
		return fmt.Errorf("failed to wrap key for recipient: %w", err)
	}

	permissions := "write"
	if readOnly {
		permissions = "read"
	}

	_, err = client.createShare(shareRequest{
		BlobID:         blobID,
		RecipientEmail: recipientEmail,
		Permissions:    permissions,
		EncryptedKey:   encryptedKeyForRecipient,
	})
	return err
}

// AcceptShare accepts an incoming share, decrypts the connection data.
func (m *Manager) AcceptShare(shareID, encryptedKey string, blobID string, masterKey []byte) ([]byte, error) {
	m.mu.RLock()
	s := m.state
	m.mu.RUnlock()

	// Decrypt private key from identity blob
	privKeyB64, err := DecryptField(masterKey, s.IdentityBlob)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt identity key: %w", err)
	}

	// Unwrap the symmetric key
	symmetricKey, err := UnwrapKey(privKeyB64, encryptedKey)
	if err != nil {
		return nil, fmt.Errorf("failed to unwrap shared key: %w", err)
	}

	// Download and decrypt the blob
	client := newAPIClient(s.ServerURL, s.AccessToken)
	blob, err := client.getBlob(blobID)
	if err != nil {
		return nil, err
	}

	plaintext, err := DecryptBlob(symmetricKey, blob.CipherText, blob.Nonce)
	if err != nil {
		return nil, err
	}

	_ = client.acceptShare(shareID)
	return plaintext, nil
}

func (m *Manager) GetIncomingShares() ([]ShareInfo, error) {
	m.mu.RLock()
	s := m.state
	m.mu.RUnlock()

	if s.AccessToken == "" {
		return nil, nil
	}
	client := newAPIClient(s.ServerURL, s.AccessToken)
	return client.getReceivedShares()
}

// GetPublicKey returns the user's X25519 public key.
func (m *Manager) GetPublicKey() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state.PublicKey
}

// DoServerLogout performs a best-effort server-side token invalidation.
func (m *Manager) DoServerLogout(serverURL, accessToken, refreshToken string) {
	if accessToken == "" {
		return
	}
	client := newAPIClient(serverURL, accessToken)
	_ = client.logout(refreshToken)
}

func deviceName() string {
	return fmt.Sprintf("Condui on %s", runtime.GOOS)
}
