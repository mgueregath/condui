//go:build !linux

package urbdrc

// sysfsScanner is a no-op on non-Linux platforms.
type sysfsScanner struct{}

func (sysfsScanner) scanDevices([]USBDeviceFilter) []usbDeviceIdentity {
	return nil
}
