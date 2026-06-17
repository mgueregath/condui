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

// buildHostKeyCallback returns an SSH HostKeyCallback that implements TOFU
// verification for the given host:port, prompting the user on first connection.
func (a *App) buildHostKeyCallback(host string, port int) ssh.HostKeyCallback {
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		fingerprint := ssh.FingerprintSHA256(key)

		stored, err := a.database.GetKnownHost(host, port)
		if err != nil {
			return err
		}
		if stored == fingerprint {
			return nil
		}
		if stored != "" && stored != fingerprint {
			return fmt.Errorf(
				"HOST KEY CHANGED for %s:%d\nExpected: %s\nGot: %s\n\nConnection rejected to prevent potential MITM attack.",
				host, port, stored, fingerprint,
			)
		}

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
			return a.database.UpsertKnownHost(host, port, fingerprint)
		case <-time.After(90 * time.Second):
			return fmt.Errorf("host key verification timed out")
		}
	}
}

// buildSSHConfig builds an ssh.ClientConfig for the given credentials.
// password must already be decrypted.
func (a *App) buildSSHConfig(username, authType, privateKeyPath, host string, port int, password string) (*ssh.ClientConfig, error) {
	var authMethods []ssh.AuthMethod
	if authType == "private_key" && privateKeyPath != "" {
		signer, err := loadPrivateKey(privateKeyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load private key: %w", err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	} else {
		authMethods = append(authMethods, ssh.Password(password))
	}
	return &ssh.ClientConfig{
		User:            username,
		Auth:            authMethods,
		HostKeyCallback: a.buildHostKeyCallback(host, port),
		Timeout:         15 * time.Second,
	}, nil
}

func strVal(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func (a *App) ConnectSSH(connectionID string) (string, error) {
	connection, err := a.database.GetConnectionByID(connectionID)
	if err != nil {
		return "", err
	}

	key := a.getMasterKey()
	if key == nil {
		return "", fmt.Errorf("vault is locked: please unlock the vault before connecting")
	}

	// Decrypt target password
	password := ""
	if connection.Password != nil {
		password, err = decryptConnectionPassword(*connection.Password, key)
		if err != nil {
			return "", fmt.Errorf("vault error: %w", err)
		}
	}

	host := connection.Host
	port := connection.Port

	targetConfig, err := a.buildSSHConfig(
		connection.Username, connection.AuthType, strVal(connection.PrivateKeyPath),
		host, port, password,
	)
	if err != nil {
		return "", err
	}

	var client *ssh.Client

	if connection.JumpHostID != nil && *connection.JumpHostID != "" {
		// ── Jump host (bastion) mode ──────────────────────────────────────
		jump, err := a.database.GetConnectionByID(*connection.JumpHostID)
		if err != nil {
			return "", fmt.Errorf("jump host not found: %w", err)
		}

		jumpPassword := ""
		if jump.Password != nil {
			jumpPassword, err = decryptConnectionPassword(*jump.Password, key)
			if err != nil {
				return "", fmt.Errorf("vault error (jump host): %w", err)
			}
		}

		jumpConfig, err := a.buildSSHConfig(
			jump.Username, jump.AuthType, strVal(jump.PrivateKeyPath),
			jump.Host, jump.Port, jumpPassword,
		)
		if err != nil {
			return "", err
		}

		jumpClient, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", jump.Host, jump.Port), jumpConfig)
		if err != nil {
			return "", fmt.Errorf("failed to connect to jump host %s: %w", jump.Host, err)
		}

		// Dial the target through the jump host TCP tunnel
		netConn, err := jumpClient.Dial("tcp", fmt.Sprintf("%s:%d", host, port))
		if err != nil {
			jumpClient.Close()
			return "", fmt.Errorf("failed to reach %s through jump host: %w", host, err)
		}

		conn, chans, reqs, err := ssh.NewClientConn(netConn, fmt.Sprintf("%s:%d", host, port), targetConfig)
		if err != nil {
			netConn.Close()
			jumpClient.Close()
			return "", fmt.Errorf("SSH handshake with %s failed: %w", host, err)
		}
		client = ssh.NewClient(conn, chans, reqs)

		// Close jump client when the main connection closes
		go func() {
			client.Conn.Wait()
			jumpClient.Close()
		}()
	} else {
		// ── Direct connection ─────────────────────────────────────────────
		client, err = ssh.Dial("tcp", fmt.Sprintf("%s:%d", host, port), targetConfig)
		if err != nil {
			return "", err
		}
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
