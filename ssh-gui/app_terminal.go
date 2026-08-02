package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"

	"github.com/wailsapp/wails/v3/pkg/application"

	"ssh-gui/backend/osinfo"
	"ssh-gui/backend/sessions"
)

const storedConnectionPasswordPrefix = "__stored_connection_password__:"

type SSHConnectResult struct {
	SessionID string `json:"sessionId"`
	HomePath  string `json:"homePath"`
}

func remoteHomePath(remoteOS osinfo.OSType, username string) string {
	if username == "" {
		return "/"
	}

	switch remoteOS {
	case osinfo.OSDarwin:
		return "/Users/" + username
	case osinfo.OSLinux, osinfo.OSUnix:
		if username == "root" {
			return "/root"
		}
		return "/home/" + username
	default:
		return "/"
	}
}

// buildHostKeyCallback returns an SSH HostKeyCallback that implements TOFU
// verification for the given host:port, prompting the user on first connection.
func (a *App) buildHostKeyCallback(ctx context.Context, host string, port int) ssh.HostKeyCallback {
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
		case <-ctx.Done():
			return fmt.Errorf("connection canceled")
		}
	}
}

// buildSSHConfig builds an ssh.ClientConfig for the given credentials.
// password must already be decrypted.
func (a *App) buildSSHConfig(ctx context.Context, username, authType, privateKeyPath, host string, port int, password string) (*ssh.ClientConfig, error) {
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
		HostKeyCallback: a.buildHostKeyCallback(ctx, host, port),
		Timeout:         15 * time.Second,
	}, nil
}

// dialSSHContext dials addr over TCP honoring ctx for cancellation, then
// performs the SSH handshake. If ctx is canceled while the handshake is in
// flight, the underlying connection is closed to unblock it.
func dialSSHContext(ctx context.Context, addr string, config *ssh.ClientConfig) (*ssh.Client, error) {
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("connection canceled")
		}
		return nil, err
	}
	client, err := sshClientHandshake(ctx, conn, addr, config)
	if err != nil {
		conn.Close()
		return nil, err
	}
	return client, nil
}

// sshClientHandshake performs the SSH handshake over an already-established
// net.Conn, aborting it if ctx is canceled mid-handshake.
func sshClientHandshake(ctx context.Context, conn net.Conn, addr string, config *ssh.ClientConfig) (*ssh.Client, error) {
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			conn.Close()
		case <-done:
		}
	}()

	c, chans, reqs, err := ssh.NewClientConn(conn, addr, config)
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("connection canceled")
		}
		return nil, err
	}
	return ssh.NewClient(c, chans, reqs), nil
}

func strVal(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func (a *App) createSSHClient(ctx context.Context, connectionID string) (*ssh.Client, error) {
	return a.createSSHClientWithJump(ctx, connectionID, "")
}

func (a *App) createSSHClientWithJump(ctx context.Context, connectionID, overrideJumpHostID string) (*ssh.Client, error) {
	connection, err := a.database.GetConnectionByID(connectionID)
	if err != nil {
		return nil, err
	}

	key := a.getMasterKey()
	if key == nil {
		return nil, fmt.Errorf("vault is locked: please unlock the vault before connecting")
	}

	password := ""
	if connection.Password != nil {
		if isSyncEncrypted(*connection.Password) {
			return nil, fmt.Errorf("la contraseña de esta conexión está pendiente de sincronización. Desbloquea el vault con tu contraseña para procesarla")
		}
		password, err = decryptConnectionPassword(*connection.Password, key)
		if err != nil {
			return nil, fmt.Errorf("vault error: %w", err)
		}
	}

	host := connection.Host
	port := connection.Port

	targetConfig, err := a.buildSSHConfig(
		ctx,
		connection.Username,
		connection.AuthType,
		strVal(connection.PrivateKeyPath),
		host,
		port,
		password,
	)

	if err != nil {
		return nil, err
	}

	jumpHostID := strVal(connection.JumpHostID)
	if overrideJumpHostID != "" {
		jumpHostID = overrideJumpHostID
	}

	if jumpHostID != "" {

		jump, err := a.database.GetConnectionByID(jumpHostID)

		if err != nil {
			return nil, fmt.Errorf(
				"jump host not found: %w",
				err,
			)
		}

		jumpPassword := ""

		if jump.Password != nil {

			jumpPassword, err =
				decryptConnectionPassword(
					*jump.Password,
					key,
				)

			if err != nil {
				return nil, fmt.Errorf(
					"vault error (jump host): %w",
					err,
				)
			}
		}

		jumpConfig, err := a.buildSSHConfig(
			ctx,
			jump.Username,
			jump.AuthType,
			strVal(jump.PrivateKeyPath),
			jump.Host,
			jump.Port,
			jumpPassword,
		)

		if err != nil {
			return nil, err
		}

		jumpClient, err := dialSSHContext(
			ctx,
			fmt.Sprintf("%s:%d", jump.Host, jump.Port),
			jumpConfig,
		)

		if err != nil {
			return nil, fmt.Errorf(
				"failed to connect to jump host %s: %w",
				jump.Host,
				err,
			)
		}

		if ctx.Err() != nil {
			jumpClient.Close()
			return nil, fmt.Errorf("connection canceled")
		}

		netConn, err := jumpClient.Dial(
			"tcp",
			fmt.Sprintf(
				"%s:%d",
				host,
				port,
			),
		)

		if err != nil {
			jumpClient.Close()
			return nil, fmt.Errorf(
				"failed to reach %s through jump host: %w",
				host,
				err,
			)
		}

		client, err := sshClientHandshake(ctx, netConn, fmt.Sprintf("%s:%d", host, port), targetConfig)

		if err != nil {
			netConn.Close()
			jumpClient.Close()
			return nil, fmt.Errorf(
				"SSH handshake with %s failed: %w",
				host,
				err,
			)
		}

		go func() {
			client.Conn.Wait()
			jumpClient.Close()
		}()

		return client, nil
	}

	return dialSSHContext(
		ctx,
		fmt.Sprintf("%s:%d", host, port),
		targetConfig,
	)
}

func (a *App) TestConnection(connectionID string) error {
	client, err := a.createSSHClient(context.Background(), connectionID)
	if err != nil {
		return err
	}
	defer client.Close()
	session, err := client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()
	return session.Run("true")
}

// TestConnectionParams tests a connection using raw params without requiring a saved record.
func (a *App) TestConnectionParams(host string, port int, username, authType, password, privateKeyPath, jumpHostID string) error {
	key := a.getMasterKey()
	if key == nil {
		return fmt.Errorf("vault is locked: please unlock the vault before connecting")
	}

	if strings.HasPrefix(password, storedConnectionPasswordPrefix) {
		connectionID := strings.TrimPrefix(password, storedConnectionPasswordPrefix)
		connection, err := a.database.GetConnectionByID(connectionID)
		if err != nil {
			return fmt.Errorf("connection not found: %w", err)
		}
		password = ""
		if connection.Password != nil {
			if isSyncEncrypted(*connection.Password) {
				return fmt.Errorf("la contraseña de esta conexión está pendiente de sincronización. Desbloquea el vault con tu contraseña para procesarla")
			}
			password, err = decryptConnectionPassword(*connection.Password, key)
			if err != nil {
				return fmt.Errorf("vault error: %w", err)
			}
		}
	}

	targetConfig, err := a.buildSSHConfig(context.Background(), username, authType, privateKeyPath, host, port, password)
	if err != nil {
		return err
	}

	var client *ssh.Client

	if jumpHostID != "" {
		jump, err := a.database.GetConnectionByID(jumpHostID)
		if err != nil {
			return fmt.Errorf("jump host not found: %w", err)
		}
		jumpPassword := ""
		if jump.Password != nil {
			jumpPassword, err = decryptConnectionPassword(*jump.Password, key)
			if err != nil {
				return fmt.Errorf("vault error (jump host): %w", err)
			}
		}
		jumpConfig, err := a.buildSSHConfig(context.Background(), jump.Username, jump.AuthType, strVal(jump.PrivateKeyPath), jump.Host, jump.Port, jumpPassword)
		if err != nil {
			return err
		}
		jumpClient, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", jump.Host, jump.Port), jumpConfig)
		if err != nil {
			return fmt.Errorf("failed to connect to jump host %s: %w", jump.Host, err)
		}
		defer jumpClient.Close()
		netConn, err := jumpClient.Dial("tcp", fmt.Sprintf("%s:%d", host, port))
		if err != nil {
			return fmt.Errorf("failed to reach %s through jump host: %w", host, err)
		}
		conn, chans, reqs, err := ssh.NewClientConn(netConn, fmt.Sprintf("%s:%d", host, port), targetConfig)
		if err != nil {
			netConn.Close()
			return fmt.Errorf("SSH handshake with %s failed: %w", host, err)
		}
		client = ssh.NewClient(conn, chans, reqs)
	} else {
		client, err = ssh.Dial("tcp", fmt.Sprintf("%s:%d", host, port), targetConfig)
		if err != nil {
			return err
		}
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()
	return session.Run("true")
}

func (a *App) ConnectSSH(connectionID string) (SSHConnectResult, error) {
	emptyResult := SSHConnectResult{}

	ctx, cancel := context.WithCancel(context.Background())
	a.registerConnectAttempt(connectionID, cancel)
	defer a.clearConnectAttempt(connectionID)
	defer cancel()

	client, err := a.createSSHClient(
		ctx,
		connectionID,
	)

	if err != nil {
		return emptyResult, err
	}

	if ctx.Err() != nil {
		client.Close()
		return emptyResult, fmt.Errorf("connection canceled")
	}

	sftpClient, err := sftp.NewClient(client)

	if err != nil {
		return emptyResult, err
	}

	session, err := client.NewSession()

	if err != nil {
		return emptyResult, err
	}

	modes := ssh.TerminalModes{
		ssh.ECHO: 1,
	}

	err = session.RequestPty("xterm", 40, 120, modes)

	if err != nil {
		return emptyResult, err
	}

	stdin, err := session.StdinPipe()

	if err != nil {
		return emptyResult, err
	}

	stdout, err := session.StdoutPipe()

	if err != nil {
		return emptyResult, err
	}

	stderr, err := session.StderrPipe()

	if err != nil {
		return emptyResult, err
	}

	sessionID := uuid.NewString()
	remoteOS, _ := osinfo.DetectRemoteOS(client)

	a.sessionManager.Add(
		&sessions.SSHSession{
			ID: sessionID,

			Client:  client,
			Session: session,

			SFTP: sftpClient,

			RemoteOS: remoteOS,

			Stdin:  stdin,
			Stdout: stdout,
			Stderr: stderr,

			Connected: true,

			Rows: 40,
			Cols: 120,
		},
	)

	if err := session.Shell(); err != nil {
		return emptyResult, err
	}

	go func() {

		buffer := make([]byte, 4096)

		for {

			n, err := stdout.Read(buffer)

			if err != nil {
				_ = a.closeSessionResources(sessionID)
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

	username := ""
	if connection, err := a.database.GetConnectionByID(connectionID); err == nil {
		username = connection.Username
	}

	return SSHConnectResult{
		SessionID: sessionID,
		HomePath:  remoteHomePath(remoteOS, username),
	}, nil
}

func (a *App) StartLocalTerminal() (SSHConnectResult, error) {
	terminal, err := startLocalTerminal(120, 40)
	if err != nil {
		return SSHConnectResult{}, err
	}

	sessionID := uuid.NewString()
	a.sessionManager.Add(&sessions.SSHSession{
		ID:            sessionID,
		LocalTerminal: terminal,
		Stdin:         terminal,
		Stdout:        terminal,
		Connected:     true,
		IsLocal:       true,
		Rows:          40,
		Cols:          120,
	})

	go func() {
		buffer := make([]byte, 4096)
		for {
			n, err := terminal.Read(buffer)
			if n > 0 {
				application.Get().Event.Emit(
					"terminal-output",
					map[string]any{
						"sessionId": sessionID,
						"data":      string(buffer[:n]),
					},
				)
			}
			if err != nil {
				return
			}
		}
	}()

	go func() {
		_ = terminal.Wait()
		_ = a.closeSessionResources(sessionID)
		application.Get().Event.Emit(
			"session-disconnected",
			map[string]any{
				"sessionId": sessionID,
			},
		)
	}()

	homePath, err := os.UserHomeDir()
	if err != nil || homePath == "" {
		homePath = "/"
	}

	return SSHConnectResult{
		SessionID: sessionID,
		HomePath:  homePath,
	}, nil
}

// ConnectSSHVia connects to connectionID routing through jumpHostID as the jump/bastion host.
func (a *App) ConnectSSHVia(connectionID, jumpHostID string) (SSHConnectResult, error) {
	emptyResult := SSHConnectResult{}

	ctx, cancel := context.WithCancel(context.Background())
	a.registerConnectAttempt(connectionID, cancel)
	defer a.clearConnectAttempt(connectionID)
	defer cancel()

	client, err := a.createSSHClientWithJump(ctx, connectionID, jumpHostID)
	if err != nil {
		return emptyResult, err
	}

	if ctx.Err() != nil {
		client.Close()
		return emptyResult, fmt.Errorf("connection canceled")
	}

	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		return emptyResult, err
	}

	session, err := client.NewSession()
	if err != nil {
		return emptyResult, err
	}

	modes := ssh.TerminalModes{ssh.ECHO: 1}
	if err = session.RequestPty("xterm", 40, 120, modes); err != nil {
		return emptyResult, err
	}

	stdin, err := session.StdinPipe()
	if err != nil {
		return emptyResult, err
	}

	stdout, err := session.StdoutPipe()
	if err != nil {
		return emptyResult, err
	}

	stderr, err := session.StderrPipe()
	if err != nil {
		return emptyResult, err
	}

	sessionID := uuid.NewString()
	remoteOS, _ := osinfo.DetectRemoteOS(client)

	a.sessionManager.Add(&sessions.SSHSession{
		ID:        sessionID,
		Client:    client,
		Session:   session,
		SFTP:      sftpClient,
		RemoteOS:  remoteOS,
		Stdin:     stdin,
		Stdout:    stdout,
		Stderr:    stderr,
		Connected: true,
		Rows:      40,
		Cols:      120,
	})

	if err := session.Shell(); err != nil {
		return emptyResult, err
	}

	go func() {
		buffer := make([]byte, 4096)
		for {
			n, err := stdout.Read(buffer)
			if err != nil {
				_ = a.closeSessionResources(sessionID)
				application.Get().Event.Emit("session-disconnected", map[string]any{"sessionId": sessionID})
				return
			}
			application.Get().Event.Emit("terminal-output", map[string]any{"sessionId": sessionID, "data": string(buffer[:n])})
		}
	}()

	go func() {
		buffer := make([]byte, 4096)
		for {
			n, err := stderr.Read(buffer)
			if err != nil {
				return
			}
			application.Get().Event.Emit("terminal-output", map[string]any{"sessionId": sessionID, "data": string(buffer[:n])})
		}
	}()

	username := ""
	if connection, err := a.database.GetConnectionByID(connectionID); err == nil {
		username = connection.Username
	}

	return SSHConnectResult{
		SessionID: sessionID,
		HomePath:  remoteHomePath(remoteOS, username),
	}, nil
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
	return a.closeSessionResources(
		sessionID,
	)
}

func (a *App) closeSessionResources(
	sessionID string,
) error {

	a.tunnelManager.CloseSession(
		sessionID,
		a.emitLog,
	)

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
		if session.IsLocal && session.LocalTerminal != nil {
			_ = session.LocalTerminal.Resize(cols, rows)
		}
	} else {
		_ = session.Session.WindowChange(
			rows,
			cols,
		)
	}

	session.Rows = rows
	session.Cols = cols
}
