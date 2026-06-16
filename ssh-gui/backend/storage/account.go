package storage

type AccountRow struct {
	ServerURL    string
	UserID       string
	Email        string
	Tier         string
	AccessToken  string
	RefreshToken string
	PublicKey    string
	IdentityBlob string
}

func (d *Database) GetAccount() (*AccountRow, error) {
	row := d.DB.QueryRow(
		`SELECT server_url, user_id, email, tier, access_token, refresh_token, public_key, identity_blob
		 FROM account WHERE id = 1`,
	)
	var a AccountRow
	err := row.Scan(
		&a.ServerURL, &a.UserID, &a.Email, &a.Tier,
		&a.AccessToken, &a.RefreshToken, &a.PublicKey, &a.IdentityBlob,
	)
	if err != nil {
		return nil, nil // no account saved
	}
	return &a, nil
}

func (d *Database) SaveAccount(a *AccountRow) error {
	_, err := d.DB.Exec(
		`INSERT INTO account(id, server_url, user_id, email, tier, access_token, refresh_token, public_key, identity_blob)
		 VALUES(1, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
		   server_url=excluded.server_url,
		   user_id=excluded.user_id,
		   email=excluded.email,
		   tier=excluded.tier,
		   access_token=excluded.access_token,
		   refresh_token=excluded.refresh_token,
		   public_key=excluded.public_key,
		   identity_blob=excluded.identity_blob`,
		a.ServerURL, a.UserID, a.Email, a.Tier,
		a.AccessToken, a.RefreshToken, a.PublicKey, a.IdentityBlob,
	)
	return err
}

func (d *Database) ClearAccount() error {
	_, err := d.DB.Exec(`DELETE FROM account WHERE id = 1`)
	return err
}
