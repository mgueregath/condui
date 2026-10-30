//go:build linux || darwin

package rdpdr

// NewNativeScardBackend creates the platform-native smartcard backend.
// On Linux/macOS this uses the pcsclite daemon via Unix socket.
func NewNativeScardBackend(socketPath string) (ScardBackend, error) {
	return NewPCSCLiteBackend(socketPath)
}
