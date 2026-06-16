package queries

import (
	"database/sql"
	"time"

	"condui-server/models"
)

type UserStore struct{ db *sql.DB }

func NewUserStore(db *sql.DB) *UserStore { return &UserStore{db} }

func (s *UserStore) Create(id, email, passwordHash string) error {
	_, err := s.db.Exec(
		`INSERT INTO users(id, email, password_hash) VALUES(?, ?, ?)`,
		id, email, passwordHash,
	)
	return err
}

func (s *UserStore) GetByEmail(email string) (*models.User, error) {
	return s.scanUser(s.db.QueryRow(
		`SELECT id, email, password_hash, tier, tier_expires_at,
		        COALESCE(public_key,''), COALESCE(identity_blob,''), created_at
		 FROM users WHERE email = ?`, email,
	))
}

func (s *UserStore) GetByID(id string) (*models.User, error) {
	return s.scanUser(s.db.QueryRow(
		`SELECT id, email, password_hash, tier, tier_expires_at,
		        COALESCE(public_key,''), COALESCE(identity_blob,''), created_at
		 FROM users WHERE id = ?`, id,
	))
}

// ListAll returns all users for admin tooling.
func (s *UserStore) ListAll() ([]*models.User, error) {
	rows, err := s.db.Query(
		`SELECT id, email, password_hash, tier, tier_expires_at,
		        COALESCE(public_key,''), COALESCE(identity_blob,''), created_at
		 FROM users ORDER BY created_at`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []*models.User
	for rows.Next() {
		u, err := s.scanUserRow(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func (s *UserStore) UpdateTier(id, tier string, expiresAt *time.Time) error {
	if expiresAt == nil {
		_, err := s.db.Exec(
			`UPDATE users SET tier = ?, tier_expires_at = NULL WHERE id = ?`, tier, id)
		return err
	}
	_, err := s.db.Exec(
		`UPDATE users SET tier = ?, tier_expires_at = ? WHERE id = ?`,
		tier, expiresAt.UTC().Format(time.RFC3339), id)
	return err
}

func (s *UserStore) UpdateIdentity(id, publicKey, identityBlob string) error {
	_, err := s.db.Exec(
		`UPDATE users SET public_key = ?, identity_blob = ? WHERE id = ?`,
		publicKey, identityBlob, id,
	)
	return err
}

func (s *UserStore) GetPublicKeyByEmail(email string) (string, error) {
	var pk string
	err := s.db.QueryRow(`SELECT COALESCE(public_key,'') FROM users WHERE email = ?`, email).Scan(&pk)
	return pk, err
}

func (s *UserStore) scanUser(row *sql.Row) (*models.User, error) {
	var u models.User
	var tierExpiresAt sql.NullString
	var createdAt string
	err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Tier, &tierExpiresAt,
		&u.PublicKey, &u.IdentityBlob, &createdAt)
	if err != nil {
		return nil, err
	}
	if tierExpiresAt.Valid && tierExpiresAt.String != "" {
		t, _ := time.Parse(time.RFC3339, tierExpiresAt.String)
		u.TierExpiresAt = &t
	}
	u.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	return &u, nil
}

func (s *UserStore) scanUserRow(rows *sql.Rows) (*models.User, error) {
	var u models.User
	var tierExpiresAt sql.NullString
	var createdAt string
	err := rows.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Tier, &tierExpiresAt,
		&u.PublicKey, &u.IdentityBlob, &createdAt)
	if err != nil {
		return nil, err
	}
	if tierExpiresAt.Valid && tierExpiresAt.String != "" {
		t, _ := time.Parse(time.RFC3339, tierExpiresAt.String)
		u.TierExpiresAt = &t
	}
	u.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	return &u, nil
}

// Refresh token operations

type TokenStore struct{ db *sql.DB }

func NewTokenStore(db *sql.DB) *TokenStore { return &TokenStore{db} }

func (s *TokenStore) Create(id, userID, tokenHash, deviceName string, expiresAt time.Time) error {
	_, err := s.db.Exec(
		`INSERT INTO refresh_tokens(id, user_id, token_hash, device_name, expires_at)
		 VALUES(?, ?, ?, ?, ?)`,
		id, userID, tokenHash, deviceName, expiresAt.Format(time.RFC3339),
	)
	return err
}

func (s *TokenStore) GetByHash(hash string) (*models.RefreshToken, error) {
	var t models.RefreshToken
	var expiresAt, createdAt string
	err := s.db.QueryRow(
		`SELECT id, user_id, token_hash, COALESCE(device_name,''), expires_at, created_at
		 FROM refresh_tokens WHERE token_hash = ?`, hash,
	).Scan(&t.ID, &t.UserID, &t.TokenHash, &t.DeviceName, &expiresAt, &createdAt)
	if err != nil {
		return nil, err
	}
	t.ExpiresAt, _ = time.Parse(time.RFC3339, expiresAt)
	t.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	return &t, nil
}

func (s *TokenStore) Delete(hash string) error {
	_, err := s.db.Exec(`DELETE FROM refresh_tokens WHERE token_hash = ?`, hash)
	return err
}

func (s *TokenStore) DeleteExpired() error {
	_, err := s.db.Exec(`DELETE FROM refresh_tokens WHERE expires_at < datetime('now')`)
	return err
}

func (s *TokenStore) CountByUser(userID string) (int, error) {
	var count int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM refresh_tokens WHERE user_id = ? AND expires_at > datetime('now')`,
		userID,
	).Scan(&count)
	return count, err
}
