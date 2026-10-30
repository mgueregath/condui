//go:build !linux && !darwin && !windows

package rdpdr

import "errors"

// NewNativeScardBackend returns an error on unsupported platforms.
func NewNativeScardBackend(_ string) (ScardBackend, error) {
	return nil, errors.New("smartcard redirection not supported on this platform")
}
