package models

import "net"

// TunnelInfo is a configured local-port-forwarding tunnel. It is persisted
// as part of its owning Connection (see Connection.Tunnels) and synced
// across devices the same way the rest of the connection's fields are.
// Active reflects only whether a listener is currently running for it in
// this process; it is never persisted or synced and is always false
// immediately after a load from storage.
type TunnelInfo struct {
	ID           string `json:"id"`
	ConnectionID string `json:"connectionId,omitempty"`
	LocalPort    int    `json:"localPort"`
	RemoteHost   string `json:"remoteHost"`
	RemotePort   int    `json:"remotePort"`
	Active       bool   `json:"active"`
}

type ActiveTunnel struct {
	Listener   net.Listener
	LocalPort  int
	RemoteHost string
	RemotePort int
}
