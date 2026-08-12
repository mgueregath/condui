package models

type Connection struct {
	ID string `json:"id"`

	FolderID *string `json:"folderId,omitempty"`

	Name string `json:"name"`

	Host string `json:"host"`

	Port int `json:"port"`

	Username string `json:"username"`

	AuthType string `json:"authType"`

	Password *string `json:"password,omitempty"`

	PrivateKeyPath *string `json:"privateKeyPath,omitempty"`

	// Passphrase decrypts PrivateKeyPath when the key itself is encrypted.
	// Stored and synced with the same encryption treatment as Password.
	Passphrase *string `json:"passphrase,omitempty"`

	Color *string `json:"color,omitempty"`

	// JumpHostID: if set, the connection is established by tunneling through
	// the referenced connection (bastion / jump host pattern).
	JumpHostID *string `json:"jumpHostId,omitempty"`

	// PasswordPending is true when the password was synced from another device
	// and is awaiting decryption (vault not yet unlocked with the correct password).
	PasswordPending bool `json:"passwordPending,omitempty"`

	// Tunnels are the local-port-forwarding tunnels configured for this
	// connection. Persisted and synced alongside it. Deliberately no
	// `omitempty`: nil (key absent/null) means "leave stored tunnels
	// untouched" — used by callers like UpdateConnection that don't manage
	// tunnels — while a present-but-empty slice means "this connection has
	// zero tunnels", which storage must apply as a real deletion. omitempty
	// would make those indistinguishable on the wire.
	Tunnels []TunnelInfo `json:"tunnels"`
}
