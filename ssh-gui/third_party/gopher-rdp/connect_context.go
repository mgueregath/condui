package rdp

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/bouncyball-git/gopher-rdp/protocol/orders"
	"github.com/bouncyball-git/gopher-rdp/protocol/tpkt"
	"github.com/bouncyball-git/gopher-rdp/protocol/x224"
)

// ConnectContext uses verified TLS/NLA by default, or Standard RDP Security
// when explicitly selected. Cancellation covers the entire initial handshake.
// Security modes never fall back automatically.
func (c *Client) ConnectContext(parent context.Context) (result error) {
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()
	c.closeMu.Lock()
	if c.closed || c.connectCancel != nil {
		c.closeMu.Unlock()
		return fmt.Errorf("RDP client already used")
	}
	c.connectCancel = cancel
	c.closeMu.Unlock()
	defer func() {
		if result != nil {
			_ = c.Close()
		}
	}()
	c.setState(StateConnecting)
	conn, err := (&net.Dialer{Timeout: 10 * time.Second}).DialContext(ctx, "tcp", net.JoinHostPort(c.opts.Host, strconv.Itoa(c.opts.Port)))
	if err != nil {
		return fmt.Errorf("%w: %w", ErrConnectionFailed, err)
	}
	c.closeMu.Lock()
	if c.closed || ctx.Err() != nil {
		c.closeMu.Unlock()
		conn.Close()
		return fmt.Errorf("RDP connection canceled")
	}
	c.conn = conn
	c.tpktConn = tpkt.NewConn(conn, c.logTpkt)
	c.closeMu.Unlock()
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	c.setState(StateX224Negotiation)
	requested := uint32(x224.ProtocolHybrid)
	if c.opts.DisableTLS {
		requested = x224.ProtocolRDP
	}
	if err = c.negotiateX224(requested); err != nil {
		return err
	}
	if c.selectedProtocol != requested {
		return fmt.Errorf("%w: server did not accept the configured security mode", ErrNegotiationFailed)
	}
	if !c.opts.DisableTLS {
		c.setState(StateTLSHandshake)
		config := &tls.Config{ServerName: c.opts.Host, MinVersion: tls.VersionTLS12}
		if c.opts.TLSConfig != nil {
			config = c.opts.TLSConfig.Clone()
		}
		if config.ServerName == "" {
			config.ServerName = c.opts.Host
		}
		if config.MinVersion < tls.VersionTLS12 {
			config.MinVersion = tls.VersionTLS12
		}
		// Certificate bypass requires the explicit per-connection option.
		if config.InsecureSkipVerify && config.VerifyConnection == nil && !c.opts.AllowUntrusted {
			return fmt.Errorf("RDP TLS certificate verification is required")
		}
		// Windows CredSSP interoperability, matching the existing upstream path.
		config.MaxVersion = tls.VersionTLS12
		secured := tls.Client(conn, config)
		if err = secured.HandshakeContext(ctx); err != nil {
			return fmt.Errorf("%w: %w", ErrTLSFailed, err)
		}
		c.closeMu.Lock()
		if c.closed || ctx.Err() != nil {
			c.closeMu.Unlock()
			return fmt.Errorf("RDP connection canceled")
		}
		c.tlsConn = secured
		c.tpktConn.SetTCPConn(secured)
		c.closeMu.Unlock()
		c.setState(StateCredSSPAuth)
		if err = c.credsspAuth(); err != nil {
			return err
		}
	}
	c.setState(StateMCSConnect)
	for _, step := range []func() error{c.mcsConnect, c.mcsErectDomain, c.mcsAttachUser, c.mcsJoinChannels} {
		if err = step(); err != nil {
			return err
		}
	}
	c.setState(StateSecurityExchange)
	if err = c.securityExchange(); err != nil {
		return err
	}
	if err = c.sendClientInfo(); err != nil {
		return err
	}
	c.setState(StateLicensing)
	if err = c.handleLicensing(); err != nil {
		return err
	}
	c.setState(StateCapabilities)
	if err = c.handleCapabilitiesExchange(); err != nil {
		return err
	}
	c.setState(StateFinalization)
	if err = c.handleFinalization(); err != nil {
		return err
	}
	if ctx.Err() != nil || !stop() {
		return fmt.Errorf("RDP connection canceled")
	}
	if err = conn.SetDeadline(time.Time{}); err != nil {
		return err
	}
	c.framebuf = NewFramebuffer(int(c.opts.Width), int(c.opts.Height))
	c.bitmapCache.Init([orders.NumBitmapCaches]int{600, 600, 2048, 4096, 2048})
	c.desktopSaveBuf = new(orders.DesktopSaveCache)
	c.closeMu.Lock()
	if c.closed {
		c.closeMu.Unlock()
		return fmt.Errorf("RDP connection canceled")
	}
	c.receiveLoopDone = make(chan struct{})
	c.setState(StateConnected)
	c.closeMu.Unlock()
	go c.receiveLoop()
	if c.opts.HeartbeatTimeout > 0 {
		c.lastReceived.Store(time.Now().UnixNano())
		go c.keepAliveLoop()
		go c.heartbeatLoop()
	}
	return nil
}
