package queries

import (
	"database/sql"
	"time"

	"condui-server/models"
)

type ShareStore struct{ db *sql.DB }

func NewShareStore(db *sql.DB) *ShareStore { return &ShareStore{db} }

func (s *ShareStore) Create(share *models.ShareInvite) error {
	_, err := s.db.Exec(
		`INSERT INTO share_invites(id, owner_id, recipient_email, blob_id, encrypted_key, permissions, status)
		 VALUES(?, ?, ?, ?, ?, ?, 'pending')`,
		share.ID, share.OwnerID, share.RecipientEmail, share.BlobID, share.EncryptedKey, share.Permissions,
	)
	return err
}

func (s *ShareStore) GetSentByOwner(ownerID string) ([]models.ShareInvite, error) {
	rows, err := s.db.Query(
		`SELECT id, owner_id, recipient_email, blob_id, COALESCE(encrypted_key,''), permissions, status, created_at
		 FROM share_invites WHERE owner_id = ? ORDER BY created_at DESC`, ownerID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return s.scanShares(rows)
}

func (s *ShareStore) GetReceivedByEmail(email string) ([]models.ShareInvite, error) {
	rows, err := s.db.Query(
		`SELECT si.id, si.owner_id, u.email as owner_email, si.blob_id,
		        COALESCE(si.encrypted_key,''), si.permissions, si.status, si.created_at
		 FROM share_invites si
		 JOIN users u ON u.id = si.owner_id
		 WHERE si.recipient_email = ? AND si.status != 'revoked'
		 ORDER BY si.created_at DESC`, email,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []models.ShareInvite
	for rows.Next() {
		var si models.ShareInvite
		var createdAt string
		if err := rows.Scan(&si.ID, &si.OwnerID, &si.OwnerEmail, &si.BlobID,
			&si.EncryptedKey, &si.Permissions, &si.Status, &createdAt); err != nil {
			return nil, err
		}
		si.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		result = append(result, si)
	}
	return result, rows.Err()
}

func (s *ShareStore) Accept(id, recipientEmail string) error {
	result, err := s.db.Exec(
		`UPDATE share_invites SET status='accepted' WHERE id=? AND recipient_email=? AND status='pending'`,
		id, recipientEmail,
	)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *ShareStore) Delete(id, actorID, actorEmail string) error {
	// Allow owner to revoke or recipient to decline
	result, err := s.db.Exec(
		`UPDATE share_invites SET status='revoked'
		 WHERE id=? AND (owner_id=? OR recipient_email=?)`,
		id, actorID, actorEmail,
	)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *ShareStore) scanShares(rows *sql.Rows) ([]models.ShareInvite, error) {
	var result []models.ShareInvite
	for rows.Next() {
		var si models.ShareInvite
		var createdAt string
		if err := rows.Scan(&si.ID, &si.OwnerID, &si.RecipientEmail,
			&si.BlobID, &si.EncryptedKey, &si.Permissions, &si.Status, &createdAt); err != nil {
			return nil, err
		}
		si.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		result = append(result, si)
	}
	return result, rows.Err()
}
