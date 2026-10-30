//go:build !linux

package urbdrc

import "errors"

// OpenUSBDeviceByAddr is not available on this platform.
func OpenUSBDeviceByAddr(busNum, devAddr uint8) (USBDevice, error) {
	return nil, errors.New("USB device access not supported on this platform")
}

// OpenUSBDevicesByVIDPID is not available on this platform.
func OpenUSBDevicesByVIDPID(vid, pid uint16) ([]USBDevice, error) {
	return nil, errors.New("USB device enumeration not supported on this platform")
}

// OpenLinuxUSBDevice is not available on non-Linux platforms.
func OpenLinuxUSBDevice(busNum, devAddr uint8) (*LinuxUSBDevice, error) {
	return nil, errors.New("USB device access not supported on this platform")
}

// EnumerateUSBDevices is not available on non-Linux platforms.
func EnumerateUSBDevices(vid, pid uint16) ([]*LinuxUSBDevice, error) {
	return nil, errors.New("USB device enumeration not supported on this platform")
}

// OpenUSBDevicesAuto is not available on non-Linux platforms.
func OpenUSBDevicesAuto(excludeClasses []uint8) ([]USBDevice, error) {
	return nil, errors.New("USB device enumeration not supported on this platform")
}

// LinuxUSBDevice stub for non-Linux builds.
type LinuxUSBDevice struct{}

func (d *LinuxUSBDevice) Descriptor() DeviceDescriptor { return DeviceDescriptor{} }
func (d *LinuxUSBDevice) Path() string                 { return "" }
func (d *LinuxUSBDevice) BusAddr() (uint8, uint8)      { return 0, 0 }
func (d *LinuxUSBDevice) IsComposite() bool             { return false }
func (d *LinuxUSBDevice) DeviceText() string            { return "" }
func (d *LinuxUSBDevice) DetachKernelDriver() error     { return nil }
func (d *LinuxUSBDevice) SelectConfiguration(uint8) error { return nil }
func (d *LinuxUSBDevice) SelectInterface(uint8, uint8) error { return nil }
func (d *LinuxUSBDevice) ControlTransfer(uint8, uint8, uint16, uint16, []byte, uint32) (int, uint32) {
	return -1, usbdStatusNotSupported
}
func (d *LinuxUSBDevice) BulkOrInterruptTransfer(uint8, []byte, uint32) (int, uint32) {
	return -1, usbdStatusNotSupported
}
func (d *LinuxUSBDevice) IsochTransfer(uint8, uint32, uint32, []IsochPacket, []byte, uint32) ([]IsochPacketResult, []byte, uint32) {
	return nil, nil, usbdStatusNotSupported
}
func (d *LinuxUSBDevice) ClearHalt(uint8) error         { return nil }
func (d *LinuxUSBDevice) CancelTransfer(uint32)          {}
func (d *LinuxUSBDevice) GetActiveConfig() *MSUSBConfig  { return nil }
func (d *LinuxUSBDevice) CompleteConfig(cfg *MSUSBConfig) *MSUSBConfig { return cfg }
func (d *LinuxUSBDevice) Close()                         {}
