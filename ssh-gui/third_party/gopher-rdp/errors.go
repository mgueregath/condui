package rdp

import "errors"

// Client errors
var (
	ErrNoHost            = errors.New("host is required")
	ErrConnectionFailed  = errors.New("connection failed")
	ErrNegotiationFailed = errors.New("protocol negotiation failed")
	ErrTLSFailed         = errors.New("TLS handshake failed")
	ErrAuthFailed        = errors.New("authentication failed")
	ErrMCSConnectFailed    = errors.New("MCS connect failed")
	ErrMCSChannelJoinFailed    = errors.New("MCS channel join failed")
	ErrSecurityExchangeFailed = errors.New("security exchange failed")
	ErrLicensingFailed        = errors.New("licensing failed")
	ErrCapabilitiesFailed     = errors.New("capabilities exchange failed")
	ErrFinalizationFailed     = errors.New("connection finalization failed")
	ErrDisconnected           = errors.New("disconnected by server")
	ErrNotConnected           = errors.New("not connected")
	ErrReconnectFailed        = errors.New("reconnect failed")
	ErrHeartbeatTimeout       = errors.New("heartbeat timeout")
	ErrMonitorPrimaryCount    = errors.New("exactly one primary monitor required")
	ErrMonitorPrimaryPosition = errors.New("primary monitor must be at position (0,0)")
	ErrDriveNameRequired      = errors.New("drive name is required")
	ErrServerRedirect         = errors.New("server redirection")
)
