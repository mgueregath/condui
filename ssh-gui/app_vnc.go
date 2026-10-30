package main

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"
	"github.com/wailsapp/wails/v3/pkg/application"
	"ssh-gui/backend/models"
	"ssh-gui/backend/vnc"
)

type VNCConnectResult struct {
	SessionID string `json:"sessionId"`
	URL       string `json:"url"`
	Token     string `json:"token"`
}

type vncSession struct {
	remote net.Conn
	server *http.Server
	cancel context.CancelFunc
	once   sync.Once
	claim  sync.Once
	timer  *time.Timer
}

// TestVNCConfig resolves draft values and stored secret references in Go.
func (a *App) TestVNCConfig(connection models.Connection) error {
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
	username, password, err := a.resolveVNC(connection, stored)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	return vnc.Test(ctx, connection.Host, connection.VNC.Port, username, password)
}

func validVNCOrigin(origin string) bool {
	u, err := url.Parse(origin)
	if err != nil || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" {
		return false
	}
	host := u.Hostname()
	return (host == "localhost" && u.Scheme == "wails") || (host == "wails.localhost" && (u.Scheme == "http" || u.Scheme == "https" || u.Scheme == "wails")) ||
		((host == "localhost" || host == "127.0.0.1") && (u.Scheme == "http" || u.Scheme == "https"))
}

func (a *App) ConnectVNC(connectionID, origin string) (VNCConnectResult, error) {
	empty := VNCConnectResult{}
	if !validVNCOrigin(origin) {
		return empty, fmt.Errorf("invalid VNC viewer origin")
	}
	connection, err := a.database.GetConnectionByID(connectionID)
	if err != nil {
		return empty, err
	}
	username, password, err := a.resolveVNC(*connection, connection)
	if err != nil {
		return empty, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.registerConnectAttempt(connectionID+":vnc", cancel)
	defer a.clearConnectAttempt(connectionID + ":vnc")
	success := false
	defer func() {
		if !success {
			cancel()
		}
	}()
	remote, err := vnc.Dial(ctx, connection.Host, connection.VNC.Port, username, password)
	if err != nil {
		return empty, err
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		remote.Close()
		return empty, fmt.Errorf("unable to open local VNC bridge")
	}
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		listener.Close()
		remote.Close()
		return empty, fmt.Errorf("unable to secure local VNC bridge")
	}
	token := hex.EncodeToString(tokenBytes)
	id := uuid.NewString()
	session := &vncSession{remote: remote, cancel: cancel}
	session.server = &http.Server{ReadHeaderTimeout: 5 * time.Second}
	session.server.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" || r.Header.Get("Origin") != origin || r.Host != listener.Addr().String() {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		ws, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: []string{origin}})
		if err != nil {
			return
		}
		defer ws.CloseNow()
		ws.SetReadLimit(1024 * 1024)
		authCtx, authCancel := context.WithTimeout(ctx, 5*time.Second)
		kind, data, err := ws.Read(authCtx)
		authCancel()
		if err != nil || kind != websocket.MessageText || subtle.ConstantTimeCompare(data, []byte(token)) != 1 {
			return
		}
		claimed := false
		session.claim.Do(func() { claimed = true })
		if !claimed {
			return
		}
		session.timer.Stop()
		defer a.CloseVNC(id)
		stream := websocket.NetConn(ctx, ws, websocket.MessageBinary)
		if err := vnc.ViewerHandshake(stream); err != nil {
			return
		}
		done := make(chan struct{})
		go func() { io.Copy(remote, stream); remote.Close(); close(done) }()
		io.Copy(stream, remote)
		cancel()
		remote.Close()
		ws.CloseNow()
		<-done
	})
	// Expire sessions whose viewer never attaches.
	session.timer = time.AfterFunc(30*time.Second, func() { _ = a.CloseVNC(id) })
	a.vncSessions.Store(id, session)
	if ctx.Err() != nil {
		a.CloseVNC(id)
		listener.Close()
		return empty, fmt.Errorf("connection canceled")
	}
	go func() { _ = session.server.Serve(listener) }()
	success = true
	return VNCConnectResult{SessionID: id, URL: "ws://" + listener.Addr().String() + "/", Token: token}, nil
}

func (a *App) CloseVNC(sessionID string) error {
	value, ok := a.vncSessions.LoadAndDelete(sessionID)
	if !ok {
		return nil
	}
	session := value.(*vncSession)
	session.once.Do(func() {
		if session.timer != nil {
			session.timer.Stop()
		}
		session.cancel()
		session.remote.Close()
		session.server.Close()
		application.Get().Event.Emit("session-disconnected", map[string]any{"sessionId": sessionID})
	})
	return nil
}
