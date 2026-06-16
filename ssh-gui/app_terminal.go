package main

import (
	"fmt"
	"net"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"

	"github.com/wailsapp/wails/v3/pkg/application"

	"ssh-gui/backend/sessions"
)

func (a *App) ConnectSSH(
	connectionID string,
) (string, error) {

	connection, err := a.database.GetConnectionByID(connectionID)

	if err != nil {
		return "", err
	}

	// Decrypt password if vault is unlocked
	password := ""
	if connection.Password != nil {
		key := a.getMasterKey()
		if key != nil {
			decrypted, err := decryptConnectionPassword(*connection.Password, key)
			if err != nil {
				return "", fmt.Errorf("vault error: %w", err)
			}
			password = decrypted
		} else {
			// Vault is locked — cannot connect
			return "", fmt.Errorf("vault is locked: please unlock the vault before connecting")
		}
	}

	host := connection.Host
	port := connection.Port

	hostKeyCallback := func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		fingerprint := ssh.FingerprintSHA256(key)

		stored, err := a.database.GetKnownHost(host, port)
		if err != nil {
			return err
		}

		if stored == fingerprint {
			// Known and matches — allow
			return nil
		}

		if stored != "" && stored != fingerprint {
			// Fingerprint changed — MITM warning
			return fmt.Errorf(
				"HOST KEY CHANGED for %s:%d\nExpected: %s\nGot: %s\n\nConnection rejected to prevent potential MITM attack.",
				host, port, stored, fingerprint,
			)
		}

		// First time seeing this host — ask user
		channelKey := fmt.Sprintf("%s:%d", host, port)
		ch := make(chan bool, 1)
		a.hostKeyChannels.Store(channelKey, ch)
		defer a.hostKeyChannels.Delete(channelKey)

		application.Get().Event.Emit("host-key-verify", map[string]any{
			"hostname":    host,
			"port":        port,
			"fingerprint": fingerprint,
			"channelKey":  channelKey,
		})

		select {
		case approved := <-ch:
			if !approved {
				return fmt.Errorf("host key rejected by user")
			}
			// Store approved fingerprint
			return a.database.UpsertKnownHost(host, port, fingerprint)
		case <-time.After(90 * time.Second):
			return fmt.Errorf("host key verification timed out")
		}
	}

	var authMethods []ssh.AuthMethod

	if connection.AuthType == "private_key" && connection.PrivateKeyPath != nil {
		signer, err := loadPrivateKey(*connection.PrivateKeyPath)
		if err != nil {
			return "", fmt.Errorf("failed to load private key: %w", err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	} else {
		authMethods = append(authMethods, ssh.Password(password))
	}

	config := &ssh.ClientConfig{
		User:            connection.Username,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCallback,
		Timeout:         15 * time.Second,
	}

	client, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", host, port), config)

	if err != nil {
		return "", err
	}

	sftpClient, err := sftp.NewClient(client)

	if err != nil {
		return "", err
	}

	session, err := client.NewSession()

	if err != nil {
		return "", err
	}

	modes := ssh.TerminalModes{
		ssh.ECHO: 1,
	}

	err = session.RequestPty("xterm", 40, 120, modes)

	if err != nil {
		return "", err
	}

	stdin, err := session.StdinPipe()

	if err != nil {
		return "", err
	}

	stdout, err := session.StdoutPipe()

	if err != nil {
		return "", err
	}

	stderr, err := session.StderrPipe()

	if err != nil {
		return "", err
	}

	sessionID := uuid.NewString()

	a.sessionManager.Add(
		&sessions.SSHSession{
			ID: sessionID,

			Client:  client,
			Session: session,

			SFTP: sftpClient,

			Stdin:  stdin,
			Stdout: stdout,
			Stderr: stderr,

			Connected: true,

			Rows: 40,
			Cols: 120,
		},
	)

	if err := session.Shell(); err != nil {
		return "", err
	}

	go func() {

		buffer := make([]byte, 4096)

		for {

			n, err := stdout.Read(buffer)

			if err != nil {
				application.Get().Event.Emit(
					"session-disconnected",
					map[string]any{
						"sessionId": sessionID,
					},
				)
				return
			}

			application.Get().Event.Emit(
				"terminal-output",
				map[string]any{
					"sessionId": sessionID,
					"data":      string(buffer[:n]),
				},
			)
		}
	}()

	go func() {

		buffer := make([]byte, 4096)

		for {

			n, err := stderr.Read(buffer)

			if err != nil {
				return
			}

			application.Get().Event.Emit(
				"terminal-output",
				map[string]any{
					"sessionId": sessionID,
					"data":      string(buffer[:n]),
				},
			)
		}
	}()

	return sessionID, nil
}

// ApproveHostKey is called by the frontend in response to a host-key-verify event.
func (a *App) ApproveHostKey(channelKey string, approved bool) {
	a.approveHostKeyChannel(channelKey, approved)
}

func (a *App) SendInput(
	sessionID string,
	data string,
) {

	session, ok :=
		a.sessionManager.Get(sessionID)

	if !ok {
		return
	}

	_, _ = session.Stdin.Write(
		[]byte(data),
	)
}

func (a *App) ListSessions() []string {

	list :=
		a.sessionManager.List()

	result := []string{}

	for _, s := range list {
		result = append(result, s.ID)
	}

	return result
}

func (a *App) CloseSession(
	sessionID string,
) error {

	return a.sessionManager.Remove(
		sessionID,
	)
}

func (a *App) ResizeTerminal(
	sessionID string,
	rows int,
	cols int,
) {

	session, ok :=
		a.sessionManager.Get(sessionID)

	if !ok {
		return
	}

	if session.Session == nil {
		return
	}

	_ = session.Session.WindowChange(
		rows,
		cols,
	)

	session.Rows = rows
	session.Cols = cols
}
