package queries

import (
	"database/sql"
	"time"

	"condui-server/models"
)

type BlobStore struct{ db *sql.DB }

func NewBlobStore(db *sql.DB) *BlobStore { return &BlobStore{db} }

func (s *BlobStore) List(userID string) ([]models.BlobMeta, error) {
	rows, err := s.db.Query(
		`SELECT id, blob_type, version, checksum, updated_at FROM blobs WHERE user_id = ? ORDER BY updated_at DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []models.BlobMeta
	for rows.Next() {
		var m models.BlobMeta
		var updatedAt string
		if err := rows.Scan(&m.ID, &m.BlobType, &m.Version, &m.Checksum, &updatedAt); err != nil {
			return nil, err
		}
		m.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
		result = append(result, m)
	}
	return result, rows.Err()
}

func (s *BlobStore) GetMeta(id, userID string) (*models.BlobMeta, error) {
	var m models.BlobMeta
	var updatedAt string
	err := s.db.QueryRow(
		`SELECT id, blob_type, version, checksum, updated_at
		 FROM blobs WHERE id = ? AND user_id = ?`, id, userID,
	).Scan(&m.ID, &m.BlobType, &m.Version, &m.Checksum, &updatedAt)
	if err != nil {
		return nil, err
	}
	m.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
	return &m, nil
}

func (s *BlobStore) Get(id, userID string) (*models.Blob, error) {
	var b models.Blob
	var updatedAt string
	err := s.db.QueryRow(
		`SELECT id, user_id, blob_type, version, cipher_text, nonce, checksum, updated_at
		 FROM blobs WHERE id = ? AND user_id = ?`, id, userID,
	).Scan(&b.ID, &b.UserID, &b.BlobType, &b.Version, &b.CipherText, &b.Nonce, &b.Checksum, &updatedAt)
	if err != nil {
		return nil, err
	}
	b.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
	return &b, nil
}

func (s *BlobStore) GetForShare(id string) (*models.Blob, error) {
	var b models.Blob
	var updatedAt string
	err := s.db.QueryRow(
		`SELECT id, user_id, blob_type, version, cipher_text, nonce, checksum, updated_at
		 FROM blobs WHERE id = ?`, id,
	).Scan(&b.ID, &b.UserID, &b.BlobType, &b.Version, &b.CipherText, &b.Nonce, &b.Checksum, &updatedAt)
	if err != nil {
		return nil, err
	}
	b.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
	return &b, nil
}

func (s *BlobStore) Create(b *models.Blob) error {
	_, err := s.db.Exec(
		`INSERT INTO blobs(id, user_id, blob_type, version, cipher_text, nonce, checksum, updated_at)
		 VALUES(?, ?, ?, 1, ?, ?, ?, datetime('now'))`,
		b.ID, b.UserID, b.BlobType, b.CipherText, b.Nonce, b.Checksum,
	)
	return err
}

func (s *BlobStore) Update(b *models.Blob) error {
	result, err := s.db.Exec(
		`UPDATE blobs SET cipher_text=?, nonce=?, checksum=?, version=version+1, updated_at=datetime('now')
		 WHERE id=? AND user_id=?`,
		b.CipherText, b.Nonce, b.Checksum, b.ID, b.UserID,
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

func (s *BlobStore) Delete(id, userID string) error {
	_, err := s.db.Exec(`DELETE FROM blobs WHERE id=? AND user_id=?`, id, userID)
	return err
}

func (s *BlobStore) CountByUser(userID string) (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM blobs WHERE user_id=?`, userID).Scan(&count)
	return count, err
}

func (s *BlobStore) GetTierLimit(tier, resource string) (int, error) {
	var limit int
	err := s.db.QueryRow(
		`SELECT max_count FROM tier_limits WHERE tier=? AND resource=?`, tier, resource,
	).Scan(&limit)
	return limit, err
}
