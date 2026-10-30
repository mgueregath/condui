package main

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	rdp "github.com/bouncyball-git/gopher-rdp"
	"github.com/google/uuid"
	"github.com/wailsapp/wails/v3/pkg/application"
	"ssh-gui/backend/models"
)

var errRDPCertificateMismatch = errors.New("RDP certificate fingerprint mismatch")

type RDPConnectResult struct {
	SessionID string `json:"sessionId"`
}

func (a *App) rdpClient(connection models.Connection, stored *models.Connection) (*rdp.Client, error) {
	username, password, err := a.resolveRDP(connection, stored)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(username) == "" {
		return nil, fmt.Errorf("RDP username is required")
	}
	options := rdp.DefaultOptions()
	options.Host = connection.Host
	options.Port = connection.RDP.Port
	options.Username = username
	options.Password = password
	options.Domain = connection.RDP.Domain
	options.DisableTLS = connection.RDP.DisableTLS
	options.AllowUntrusted = connection.RDP.AllowUntrusted && !connection.RDP.DisableTLS
	if options.Domain == "" {
		if parts := strings.SplitN(username, `\`, 2); len(parts) == 2 {
			options.Domain = parts[0]
			options.Username = parts[1]
		}
	}
	options.Width, options.Height, err = connection.RDP.DesktopSize()
	if err != nil {
		return nil, err
	}
	options.Depth = 32
	options.AutoReconnect = false
	options.Clipboard = false
	options.GFX = false
	options.NoAVC = true
	options.TLSConfig = &tls.Config{ServerName: connection.Host, MinVersion: tls.VersionTLS12, InsecureSkipVerify: options.AllowUntrusted}
	fingerprint := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(connection.RDP.CertificateFingerprint), ":", ""))
	if !connection.RDP.DisableTLS && !connection.RDP.AllowUntrusted && fingerprint != "" {
		expected, err := hex.DecodeString(fingerprint)
		if err != nil || len(expected) != sha256.Size {
			return nil, fmt.Errorf("invalid RDP SHA-256 certificate fingerprint")
		}
		// This explicit pin replaces CA verification, not certificate identity checks.
		options.TLSConfig.InsecureSkipVerify = true
		options.TLSConfig.VerifyConnection = func(state tls.ConnectionState) error {
			if len(state.PeerCertificates) == 0 {
				return fmt.Errorf("RDP certificate missing")
			}
			actual := sha256.Sum256(state.PeerCertificates[0].Raw)
			if subtle.ConstantTimeCompare(expected, actual[:]) != 1 {
				return errRDPCertificateMismatch
			}
			return nil
		}
	}
	return rdp.NewClient(options)
}
func rdpConnectionError(err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("RDP connection canceled or timed out")
	}
	if errors.Is(err, rdp.ErrTLSFailed) {
		var authority x509.UnknownAuthorityError
		var hostname x509.HostnameError
		var certificate x509.CertificateInvalidError
		switch {
		case errors.Is(err, errRDPCertificateMismatch):
			return fmt.Errorf("RDP certificate differs from the configured SHA-256 fingerprint; verify the server certificate: %w", err)
		case errors.As(err, &authority):
			return fmt.Errorf("RDP certificate issuer is not trusted; trust its CA or enter the verified server SHA-256 fingerprint in the RDP settings: %w", err)
		case errors.As(err, &hostname):
			return fmt.Errorf("RDP certificate does not match Host/IP; use the certificate hostname or configure the verified server SHA-256 fingerprint: %w", err)
		case errors.As(err, &certificate):
			return fmt.Errorf("RDP certificate is invalid; check its validity and server configuration: %w", err)
		default:
			return fmt.Errorf("RDP TLS negotiation failed: %w", err)
		}
	}
	if errors.Is(err, rdp.ErrAuthFailed) {
		return fmt.Errorf("RDP authentication failed; check user, domain and password")
	}
	if errors.Is(err, rdp.ErrConnectionFailed) {
		return fmt.Errorf("RDP server is unreachable")
	}
	if errors.Is(err, rdp.ErrNegotiationFailed) {
		return fmt.Errorf("RDP server rejected the configured security mode (TLS/NLA or Standard RDP without TLS): %w", err)
	}
	return fmt.Errorf("RDP session could not be established: %w", err)
}
func (a *App) TestRDPConfig(connection models.Connection) error {
	var stored *models.Connection
	if connection.ID != "" {
		var err error
		stored, err = a.database.GetConnectionByID(connection.ID)
		if err != nil {
			return err
		}
	}
	if stored != nil && stored.ShareRevoked {
		return fmt.Errorf("shared connection access was revoked")
	}
	client, err := a.rdpClient(connection, stored)
	if err != nil {
		return err
	}
	defer client.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err = client.ConnectContext(ctx); err != nil {
		return rdpConnectionError(err)
	}
	return nil
}
func (a *App) ConnectRDP(connectionID string) (RDPConnectResult, error) {
	empty := RDPConnectResult{}
	connection, err := a.database.GetConnectionByID(connectionID)
	if err != nil {
		return empty, err
	}
	client, err := a.rdpClient(*connection, connection)
	if err != nil {
		return empty, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	id := uuid.NewString()
	session := newRDPSession(id, client, ctx, cancel)
	a.rdpSessions.Store(id, session)
	a.registerConnectAttempt(connectionID+":rdp", func() { cancel(); _ = a.CloseRDP(id) })
	defer a.clearConnectAttempt(connectionID + ":rdp")
	client.OnBitmap(func(*rdp.BitmapUpdate) {})
	// Upstream only opens paint batches when OnBeginPaint is registered.
	// Without it OnEndPaint never fires and no desktop frames are captured.
	client.OnBeginPaint(func() {})
	client.OnEndPaint(session.capture)
	client.OnPointer(session.captureCursor)
	client.OnDisconnect(func(error) { go a.CloseRDP(id) })
	go session.stream()
	if err = client.ConnectContext(ctx); err != nil {
		_ = a.CloseRDP(id)
		return empty, rdpConnectionError(err)
	}
	if ctx.Err() != nil {
		_ = a.CloseRDP(id)
		return empty, fmt.Errorf("RDP connection canceled")
	}
	return RDPConnectResult{SessionID: id}, nil
}
func (a *App) CloseRDP(sessionID string) error {
	value, ok := a.rdpSessions.LoadAndDelete(sessionID)
	if !ok {
		return nil
	}
	session := value.(*rdpSession)
	session.cancel()
	err := session.client.Close()
	application.Get().Event.Emit("session-disconnected", map[string]any{"sessionId": sessionID})
	return err
}
func (a *App) GetRDPFrame(sessionID string) (RDPFrame, error) {
	value, ok := a.rdpSessions.Load(sessionID)
	if !ok {
		return RDPFrame{}, fmt.Errorf("RDP session unavailable")
	}
	session := value.(*rdpSession)
	session.frameMu.RLock()
	defer session.frameMu.RUnlock()
	return session.frame, nil
}
