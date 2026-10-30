// Package vnc implements the direct RFB password handshake used by the PoC.
package vnc

import (
	"context"
	"crypto/des"
	"encoding/binary"
	"fmt"
	"io"
	"math/bits"
	"net"
	"strconv"
	"time"
)

// Dial authenticates without exposing the server password to the WebView.
// ClientInit is left to the viewer. Vendor greetings negotiate a standard RFB version.
func Dial(ctx context.Context, host string, port int, username, password string) (net.Conn, error) {
	if host == "" || port < 1 || port > 65535 {
		return nil, fmt.Errorf("invalid VNC host or port")
	}
	conn, err := (&net.Dialer{Timeout: 10 * time.Second}).DialContext(ctx, "tcp", net.JoinHostPort(host, strconv.Itoa(port)))
	if err != nil {
		return nil, fmt.Errorf("VNC server is unreachable")
	}
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()
	conn.SetDeadline(time.Now().Add(15 * time.Second))
	if err := authenticate(conn, username, password); err != nil {
		conn.Close()
		return nil, err
	}
	if ctx.Err() != nil {
		conn.Close()
		return nil, fmt.Errorf("connection canceled")
	}
	conn.SetDeadline(time.Time{})
	return conn, nil
}

func authenticate(conn net.Conn, username, password string) error {
	version := make([]byte, 12)
	if _, err := io.ReadFull(conn, version); err != nil {
		return fmt.Errorf("invalid VNC greeting")
	}
	minor := 8
	switch string(version) {
	case "RFB 003.003\n", "RFB 003.006\n": // UltraVNC uses the 3.3 handshake.
		minor = 3
	case "RFB 003.007\n":
		minor = 7
	case "RFB 003.008\n", "RFB 003.889\n", "RFB 004.000\n", "RFB 004.001\n", "RFB 005.000\n":
		// Match noVNC's Apple, Intel AMT and RealVNC version negotiation.
	default:
		return fmt.Errorf("unsupported RFB greeting %q; expected RFB 3.3, 3.7, 3.8 or a compatible vendor version", string(version))
	}
	// Reply with the negotiated standard version, never echo a vendor version.
	if _, err := fmt.Fprintf(conn, "RFB 003.%03d\n", minor); err != nil {
		return fmt.Errorf("VNC handshake interrupted")
	}
	var security byte
	if minor == 3 {
		var selected uint32
		if err := binary.Read(conn, binary.BigEndian, &selected); err != nil {
			return fmt.Errorf("invalid VNC security negotiation")
		}
		if selected != 1 && selected != 2 && selected != 30 {
			return fmt.Errorf("unsupported VNC authentication; use VNC password authentication")
		}
		security = byte(selected)
	} else {
		count := []byte{0}
		if _, err := io.ReadFull(conn, count); err != nil || count[0] == 0 {
			return fmt.Errorf("VNC server rejected security negotiation")
		}
		methods := make([]byte, int(count[0]))
		if _, err := io.ReadFull(conn, methods); err != nil {
			return fmt.Errorf("invalid VNC security methods")
		}
		for _, method := range methods {
			if username != "" {
				if method == 30 {
					security = 30
					break
				}
				continue
			}
			if method == 2 {
				security = 2
				break
			}
			if method == 1 && password == "" {
				security = 1
			}
		}
		if security == 0 {
			if username != "" {
				return fmt.Errorf("server does not offer Apple Remote Desktop authentication; leave username empty for classic VNC")
			}
			for _, method := range methods {
				if method == 30 {
					return fmt.Errorf("macOS username is required for Apple Remote Desktop authentication")
				}
			}
			return fmt.Errorf("unsupported VNC authentication")
		}
		if _, err := conn.Write([]byte{security}); err != nil {
			return fmt.Errorf("VNC handshake interrupted")
		}
	}
	if username != "" && security != 30 {
		return fmt.Errorf("server does not offer Apple Remote Desktop authentication")
	}
	if security == 30 {
		if err := authenticateARD(conn, username, password); err != nil {
			return err
		}
	}
	if security == 1 && password != "" {
		return fmt.Errorf("VNC server does not support password authentication")
	}
	if security == 2 {
		challenge := make([]byte, 16)
		if _, err := io.ReadFull(conn, challenge); err != nil {
			return fmt.Errorf("invalid VNC challenge")
		}
		key := make([]byte, 8)
		copy(key, []byte(password))
		for i := range key {
			key[i] = bits.Reverse8(key[i])
		}
		cipher, err := des.NewCipher(key)
		clear(key)
		if err != nil {
			return fmt.Errorf("VNC authentication unavailable")
		}
		cipher.Encrypt(challenge[:8], challenge[:8])
		cipher.Encrypt(challenge[8:], challenge[8:])
		if _, err := conn.Write(challenge); err != nil {
			return fmt.Errorf("VNC authentication interrupted")
		}
	}
	if security == 2 || security == 30 || minor == 8 {
		var result uint32
		if err := binary.Read(conn, binary.BigEndian, &result); err != nil {
			return fmt.Errorf("VNC authentication interrupted")
		}
		if result != 0 {
			return fmt.Errorf("VNC authentication failed")
		}
	}
	return nil
}

// ViewerHandshake presents a password-free local handshake after the remote
// connection has authenticated. The bridge credential protects this stream.
func ViewerHandshake(conn net.Conn) error {
	conn.SetDeadline(time.Now().Add(15 * time.Second))
	if _, err := conn.Write([]byte("RFB 003.008\n")); err != nil {
		return err
	}
	version := make([]byte, 12)
	if _, err := io.ReadFull(conn, version); err != nil {
		return err
	}
	if string(version) != "RFB 003.008\n" {
		return fmt.Errorf("unsupported viewer version")
	}
	if _, err := conn.Write([]byte{1, 1}); err != nil {
		return err
	}
	method := []byte{0}
	if _, err := io.ReadFull(conn, method); err != nil {
		return err
	}
	if method[0] != 1 {
		return fmt.Errorf("invalid viewer security method")
	}
	if _, err := conn.Write([]byte{0, 0, 0, 0}); err != nil {
		return err
	}
	return conn.SetDeadline(time.Time{})
}

func Test(ctx context.Context, host string, port int, username, password string) error {
	conn, err := Dial(ctx, host, port, username, password)
	if err != nil {
		return err
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()
	conn.SetDeadline(time.Now().Add(10 * time.Second))
	if _, err := conn.Write([]byte{1}); err != nil {
		return fmt.Errorf("VNC initialization failed")
	}
	header := make([]byte, 24)
	if _, err := io.ReadFull(conn, header); err != nil {
		return fmt.Errorf("invalid VNC desktop initialization")
	}
	size := binary.BigEndian.Uint32(header[20:])
	if binary.BigEndian.Uint16(header[:2]) == 0 || binary.BigEndian.Uint16(header[2:4]) == 0 || size > 1024*1024 {
		return fmt.Errorf("invalid VNC desktop")
	}
	if _, err := io.CopyN(io.Discard, conn, int64(size)); err != nil {
		return fmt.Errorf("incomplete VNC desktop initialization")
	}
	return nil
}
