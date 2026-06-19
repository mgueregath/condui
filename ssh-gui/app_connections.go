package main

import (
	"ssh-gui/backend/models"
)

func (a *App) CreateFolder(
	name string,
) error {

	_, err :=
		a.database.CreateFolder(
			name,
		)

	if err != nil {
		return err
	}

	a.triggerBackgroundSync()
	return nil
}

func (a *App) GetFolders() (
	[]models.Folder,
	error,
) {

	return a.database.GetFolders()

}

// GetConnections returns all connections with passwords redacted.
// The frontend never receives plaintext passwords.
func (a *App) GetConnections() (
	[]models.Connection,
	error,
) {

	connections, err := a.database.GetConnections()
	if err != nil {
		return nil, err
	}

	// Redact passwords; flag connections whose password is pending cross-device decryption.
	for i := range connections {
		if connections[i].Password != nil && isSyncEncrypted(*connections[i].Password) {
			connections[i].PasswordPending = true
		}
		connections[i].Password = nil
	}

	return connections, nil
}

// CreateConnection encrypts the password before persisting it.
func (a *App) CreateConnection(
	connection models.Connection,
) error {

	if connection.Password != nil && *connection.Password != "" {
		key := a.getMasterKey()
		if key == nil {
			// If vault is not set up yet, store plaintext (migration path)
			// Once vault is set up, all new passwords will be encrypted
		} else {
			enc, err := encryptConnectionPassword(*connection.Password, key)
			if err != nil {
				return err
			}
			connection.Password = &enc
		}
	}

	err := a.database.CreateConnection(&connection)
	if err != nil {
		return err
	}

	// Background sync if logged in
	a.triggerBackgroundSync()
	return nil
}

// UpdateConnection encrypts the password before updating.
func (a *App) UpdateConnection(
	connection models.Connection,
) error {

	// If password is nil (redacted from frontend), preserve the stored password
	if connection.Password == nil {
		existing, err := a.database.GetConnectionByID(connection.ID)
		if err != nil {
			return err
		}
		connection.Password = existing.Password
	} else if *connection.Password != "" {
		key := a.getMasterKey()
		if key != nil {
			// Only encrypt if the value is not already encrypted (no ":" = plaintext)
			isEncrypted := false
			for _, c := range *connection.Password {
				if c == ':' {
					isEncrypted = true
					break
				}
			}
			if !isEncrypted {
				enc, err := encryptConnectionPassword(*connection.Password, key)
				if err != nil {
					return err
				}
				connection.Password = &enc
			}
		}
	}

	err := a.database.UpdateConnection(&connection)
	if err != nil {
		return err
	}

	a.triggerBackgroundSync()
	return nil
}

func (a *App) DeleteConnection(
	id string,
) error {

	err := a.database.DeleteConnection(id)
	if err != nil {
		return err
	}

	a.triggerBackgroundSync()
	return nil
}

func (a *App) UpdateFolder(
	id string,
	name string,
) error {

	err := a.database.UpdateFolder(
		id,
		name,
	)
	if err != nil {
		return err
	}

	a.triggerBackgroundSync()
	return nil
}

func (a *App) DeleteFolder(
	id string,
) error {

	err := a.database.DeleteFolder(
		id,
	)
	if err != nil {
		return err
	}

	a.triggerBackgroundSync()
	return nil
}

// GetConnectionByID returns a connection with password redacted (use ConnectSSH to connect).
func (a *App) GetConnectionByID(
	id string,
) (*models.Connection, error) {

	conn, err := a.database.GetConnectionByID(id)
	if err != nil {
		return nil, err
	}

	// Redact password from frontend response
	conn.Password = nil
	return conn, nil
}

// triggerBackgroundSync triggers an async sync if the user is logged in and vault is unlocked.
func (a *App) triggerBackgroundSync() {
	if !a.accountManager.IsLoggedIn() {
		return
	}
	key := a.getMasterKey()
	if key == nil {
		return
	}
	go func() {
		_ = a.SyncNow()
	}()
}
