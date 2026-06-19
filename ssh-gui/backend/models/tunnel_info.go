package models

import "net"

type TunnelInfo struct {
	ID         string `json:"id"`
	LocalPort  int    `json:"localPort"`
	RemoteHost string `json:"remoteHost"`
	RemotePort int    `json:"remotePort"`
	Active     bool   `json:"active"`
}

type ActiveTunnel struct {
    Listener   net.Listener
    LocalPort  int
    RemoteHost string
    RemotePort int
}