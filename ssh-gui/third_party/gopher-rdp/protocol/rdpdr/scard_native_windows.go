//go:build windows

package rdpdr

// NewNativeScardBackend creates the platform-native smartcard backend.
// On Windows this uses the WinSCard API (winscard.dll).
func NewNativeScardBackend(socketPath string) (ScardBackend, error) {
	return NewWinSCardBackend(socketPath)
}
