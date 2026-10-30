//go:build linux

package urbdrc

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

// Linux usbdevfs ioctl numbers.
const (
	usbdevfsControl        = 0xC0185500 // USBDEVFS_CONTROL
	usbdevfsBulk           = 0xC0185502 // USBDEVFS_BULK
	usbdevfsSetInterface   = 0x80085504 // USBDEVFS_SETINTERFACE
	usbdevfsSetConfig      = 0x80045505 // USBDEVFS_SETCONFIGURATION
	usbdevfsClaimInterface = 0x8004550F // USBDEVFS_CLAIMINTERFACE
	usbdevfsRelease        = 0x80045510 // USBDEVFS_RELEASEINTERFACE
	usbdevfsDisconnect     = 0x80045516 // USBDEVFS_DISCONNECT
	usbdevfsConnect        = 0x80045517 // USBDEVFS_CONNECT
	usbdevfsClearHalt      = 0x80045515 // USBDEVFS_CLEAR_HALT
	usbdevfsDisconnectClaim = 0x8108551B // USBDEVFS_DISCONNECT_CLAIM
)

// USBDEVFS_DISCONNECT_CLAIM flags.
const usbdevfsDisconnectClaimExceptDriver = 0x02 // disconnect any driver except the named one

// usbdevfsDisconnectClaimReq matches struct usbdevfs_disconnect_claim.
type usbdevfsDisconnectClaimReq struct {
	Interface uint32
	Flags     uint32
	Driver    [256]byte
}

// usbdevfsCtrlTransfer matches struct usbdevfs_ctrltransfer.
type usbdevfsCtrlTransfer struct {
	BRequestType uint8
	BRequest     uint8
	WValue       uint16
	WIndex       uint16
	WLength      uint16
	Timeout      uint32
	Data         uintptr
}

// usbdevfsBulkTransfer matches struct usbdevfs_bulktransfer.
type usbdevfsBulkTransfer struct {
	Ep      uint32
	Len     uint32
	Timeout uint32
	Data    uintptr
}

// usbdevfsSetInterfaceReq matches struct usbdevfs_setinterface.
type usbdevfsSetInterfaceReq struct {
	Interface uint32
	AltSetting uint32
}

// LinuxUSBDevice implements USBDevice using Linux usbdevfs.
type LinuxUSBDevice struct {
	fd       int
	busNum   uint8
	devAddr  uint8
	devPath  string // e.g. "1-4"
	desc     DeviceDescriptor
	rawDesc  [18]byte // raw device descriptor bytes
	composite bool

	// Cached active configuration
	configDesc []byte
	numIfaces  int
	claimed    []int // claimed interface numbers
}

// OpenLinuxUSBDevice opens a USB device by bus number and device address.
func OpenLinuxUSBDevice(busNum, devAddr uint8) (*LinuxUSBDevice, error) {
	devNode := fmt.Sprintf("/dev/bus/usb/%03d/%03d", busNum, devAddr)
	fd, err := syscall.Open(devNode, syscall.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", devNode, err)
	}

	dev := &LinuxUSBDevice{
		fd:      fd,
		busNum:  busNum,
		devAddr: devAddr,
	}

	// Read device descriptor (first 18 bytes of device node)
	n, err := syscall.Pread(fd, dev.rawDesc[:], 0)
	if err != nil || n < 18 {
		syscall.Close(fd)
		return nil, fmt.Errorf("read device descriptor: %w", err)
	}
	dev.parseDescriptor()
	dev.devPath = dev.readSysfsPath()
	dev.detectComposite()
	dev.readConfigDescriptor()

	// Detach kernel drivers and claim all interfaces. For devices using the
	// UAS driver (USB 3.0 storage), detaching causes a device reset and
	// re-enumeration with a new address. We detect this and reopen.
	if err := dev.detachAndClaim(); err != nil {
		syscall.Close(dev.fd)
		return nil, fmt.Errorf("claim interfaces: %w", err)
	}

	return dev, nil
}

// detachAndClaim detaches kernel drivers, claims interfaces, and reopens
// the device if it re-enumerated (common for USB 3.0 UAS devices whose
// driver disconnect triggers a port reset).
func (d *LinuxUSBDevice) detachAndClaim() error {
	_ = d.DetachKernelDriver()
	if err := d.claimAllInterfaces(); err != nil {
		return err
	}

	// Check if the device re-enumerated (UAS driver disconnect causes
	// an async port reset on USB 3.0). Poll for up to 2 seconds.
	for range 10 {
		time.Sleep(200 * time.Millisecond)
		var tmp [18]byte
		if _, err := syscall.Pread(d.fd, tmp[:], 0); err != nil {
			// fd is stale — device re-enumerated. Reopen.
			return d.reopenAfterReset()
		}
	}
	return nil
}

// reopenAfterReset waits for a re-enumerated device to reappear in sysfs
// with a new address, reopens the device node, and claims interfaces using
// plain CLAIMINTERFACE (no DISCONNECT_CLAIM) to avoid triggering another
// reset cycle.
func (d *LinuxUSBDevice) reopenAfterReset() error {
	d.releaseAllInterfaces()
	syscall.Close(d.fd)
	d.fd = -1

	// Wait for device to reappear with a new devnum (up to 5 seconds).
	var newAddr uint8
	for range 50 {
		time.Sleep(100 * time.Millisecond)
		a := d.readSysfsDevnum()
		if a != 0 && a != d.devAddr {
			newAddr = a
			break
		}
	}
	if newAddr == 0 {
		return fmt.Errorf("device %s did not reappear after port reset", d.devPath)
	}

	// Wait a bit for udev to create the device node.
	time.Sleep(200 * time.Millisecond)

	newNode := fmt.Sprintf("/dev/bus/usb/%03d/%03d", d.busNum, newAddr)
	fd, err := syscall.Open(newNode, syscall.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("reopen %s: %w", newNode, err)
	}
	d.fd = fd
	d.devAddr = newAddr

	n, err := syscall.Pread(fd, d.rawDesc[:], 0)
	if err != nil || n < 18 {
		return fmt.Errorf("re-read descriptor from %s: %w", newNode, err)
	}
	d.parseDescriptor()
	d.readConfigDescriptor()

	// Claim interfaces with plain CLAIMINTERFACE first — this avoids
	// calling any driver's disconnect handler (which would trigger
	// another reset for UAS). Only fall back to DISCONNECT_CLAIM if
	// a driver already bound.
	d.claimed = d.claimed[:0]
	for i := range d.numIfaces {
		ifnum := uint32(i)
		_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(d.fd),
			uintptr(usbdevfsClaimInterface), uintptr(unsafe.Pointer(&ifnum)))
		if errno == 0 {
			d.claimed = append(d.claimed, i)
			continue
		}
		// Driver already bound — use DISCONNECT_CLAIM as last resort.
		var dc usbdevfsDisconnectClaimReq
		dc.Interface = ifnum
		dc.Flags = usbdevfsDisconnectClaimExceptDriver
		copy(dc.Driver[:], "usbfs")
		_, _, errno = syscall.Syscall(syscall.SYS_IOCTL, uintptr(d.fd),
			uintptr(usbdevfsDisconnectClaim), uintptr(unsafe.Pointer(&dc)))
		if errno != 0 {
			return fmt.Errorf("claim interface %d after reopen: %w", i, errno)
		}
		d.claimed = append(d.claimed, i)
	}
	return nil
}

func (d *LinuxUSBDevice) readSysfsDevnum() uint8 {
	path := fmt.Sprintf("/sys/bus/usb/devices/%s/devnum", d.devPath)
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	v, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0
	}
	return uint8(v)
}

func (d *LinuxUSBDevice) parseDescriptor() {
	b := d.rawDesc[:]
	d.desc = DeviceDescriptor{
		BLength:            b[0],
		BDescriptorType:    b[1],
		BCDUSB:             le16(b[2:4]),
		BDeviceClass:       b[4],
		BDeviceSubClass:    b[5],
		BDeviceProtocol:    b[6],
		BMaxPacketSize0:    b[7],
		IDVendor:           le16(b[8:10]),
		IDProduct:          le16(b[10:12]),
		BCDDevice:          le16(b[12:14]),
		IManufacturer:      b[14],
		IProduct:           b[15],
		ISerialNumber:      b[16],
		BNumConfigurations: b[17],
	}
}

func (d *LinuxUSBDevice) readSysfsPath() string {
	// Read from /sys/bus/usb/devices/ to find the path "bus-port"
	pattern := fmt.Sprintf("/sys/bus/usb/devices/%d-*", d.busNum)
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		return fmt.Sprintf("%d-%d", d.busNum, d.devAddr)
	}

	for _, m := range matches {
		devnumFile := filepath.Join(m, "devnum")
		data, err := os.ReadFile(devnumFile)
		if err != nil {
			continue
		}
		num, err := strconv.Atoi(strings.TrimSpace(string(data)))
		if err != nil {
			continue
		}
		if uint8(num) == d.devAddr {
			return filepath.Base(m)
		}
	}
	return fmt.Sprintf("%d-%d", d.busNum, d.devAddr)
}

func (d *LinuxUSBDevice) detectComposite() {
	// A device is composite if it has class 0 (per-interface) and multiple interfaces.
	// Or class 0xEF/0x02/0x01 (IAD).
	if d.desc.BDeviceClass == 0xEF && d.desc.BDeviceSubClass == 0x02 && d.desc.BDeviceProtocol == 0x01 {
		d.composite = true
		return
	}
	if d.desc.BDeviceClass == 0 && d.desc.BNumConfigurations == 1 {
		// Will check numIfaces after reading config descriptor
		d.composite = true // tentative, refined in readConfigDescriptor
	}
}

func (d *LinuxUSBDevice) readConfigDescriptor() {
	// Read the full descriptor from the device node (it follows the device descriptor)
	buf := make([]byte, 4096)
	n, err := syscall.Pread(d.fd, buf, 0)
	if err != nil || n < 18 {
		return
	}

	// Skip device descriptor (first 18 bytes)
	off := 18
	if off >= n {
		return
	}

	// The config descriptor follows
	if off+4 > n {
		return
	}
	if buf[off+1] != 0x02 { // bDescriptorType != CONFIGURATION
		return
	}
	wTotalLength := int(le16(buf[off+2 : off+4]))
	if off+wTotalLength > n {
		wTotalLength = n - off
	}
	d.configDesc = make([]byte, wTotalLength)
	copy(d.configDesc, buf[off:off+wTotalLength])

	// Count interfaces
	if wTotalLength >= 5 {
		d.numIfaces = int(buf[off+4]) // bNumInterfaces
	}

	// Refine composite detection
	if d.desc.BDeviceClass == 0 && d.numIfaces > 1 {
		d.composite = true
	} else if d.desc.BDeviceClass == 0 && d.numIfaces <= 1 {
		d.composite = false
	}

	// Always override device class from first interface when bDeviceClass is 0
	// (per-interface class). This ensures CompatibilityIDs report the actual
	// interface class (e.g. 0x0B for CCID) rather than 0x00, which Windows
	// needs to find the correct driver. Matches reference implementation behavior
	// at libusb_udevice.c:1829-1832.
	if d.numIfaces > 0 {
		d.overrideClassFromFirstInterface()
	}
}

func (d *LinuxUSBDevice) overrideClassFromFirstInterface() {
	// Walk config descriptor to find first interface descriptor
	off := 0
	for off+2 <= len(d.configDesc) {
		bLength := int(d.configDesc[off])
		if bLength < 2 {
			break
		}
		bDescType := d.configDesc[off+1]
		if bDescType == 0x04 && bLength >= 9 { // INTERFACE descriptor
			d.desc.BDeviceClass = d.configDesc[off+5]
			d.desc.BDeviceSubClass = d.configDesc[off+6]
			d.desc.BDeviceProtocol = d.configDesc[off+7]
			return
		}
		off += bLength
	}
}

func (d *LinuxUSBDevice) Descriptor() DeviceDescriptor {
	return d.desc
}

func (d *LinuxUSBDevice) Path() string {
	return d.devPath
}

func (d *LinuxUSBDevice) BusAddr() (uint8, uint8) {
	return d.busNum, d.devAddr
}

func (d *LinuxUSBDevice) IsComposite() bool {
	return d.composite
}

func (d *LinuxUSBDevice) DeviceText() string {
	// Try to read from sysfs product file
	sysfsDir := fmt.Sprintf("/sys/bus/usb/devices/%s", d.devPath)
	data, err := os.ReadFile(filepath.Join(sysfsDir, "product"))
	if err == nil {
		return strings.TrimSpace(string(data))
	}
	return ""
}

func (d *LinuxUSBDevice) DetachKernelDriver() error {
	for i := 0; i < d.numIfaces; i++ {
		ifnum := uint32(i)
		// Try to disconnect kernel driver; ignore errors (may not be attached)
		_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(d.fd),
			uintptr(usbdevfsDisconnect), uintptr(unsafe.Pointer(&ifnum)))
		if errno != 0 && errno != syscall.ENODATA {
			// ENODATA means no driver was attached, which is fine
		}
	}
	return nil
}

func (d *LinuxUSBDevice) claimAllInterfaces() error {
	d.claimed = d.claimed[:0]
	for i := 0; i < d.numIfaces; i++ {
		ifnum := uint32(i)
		// Use USBDEVFS_DISCONNECT_CLAIM to atomically detach any kernel
		// driver and claim the interface. This is critical for devices
		// whose kernel driver (e.g. usb-storage) cannot be detached via
		// the separate USBDEVFS_DISCONNECT ioctl when the device is in
		// use. Matches libusb's detach_kernel_driver_and_claim() approach.
		var dc usbdevfsDisconnectClaimReq
		dc.Interface = ifnum
		dc.Flags = usbdevfsDisconnectClaimExceptDriver
		copy(dc.Driver[:], "usbfs")
		_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(d.fd),
			uintptr(usbdevfsDisconnectClaim), uintptr(unsafe.Pointer(&dc)))
		if errno != 0 {
			// Fall back to simple claim for kernels without DISCONNECT_CLAIM.
			_, _, errno = syscall.Syscall(syscall.SYS_IOCTL, uintptr(d.fd),
				uintptr(usbdevfsClaimInterface), uintptr(unsafe.Pointer(&ifnum)))
			if errno != 0 {
				return fmt.Errorf("claim interface %d: %w", i, errno)
			}
		}
		d.claimed = append(d.claimed, i)
	}
	return nil
}

func (d *LinuxUSBDevice) releaseAllInterfaces() {
	for _, i := range d.claimed {
		ifnum := uint32(i)
		syscall.Syscall(syscall.SYS_IOCTL, uintptr(d.fd),
			uintptr(usbdevfsRelease), uintptr(unsafe.Pointer(&ifnum)))
	}
	d.claimed = d.claimed[:0]
}

func (d *LinuxUSBDevice) SelectConfiguration(bConfigurationValue uint8) error {
	// If the requested config is already active, skip SETCONFIGURATION.
	// The ioctl causes a port reset on USB 3.0 devices, which re-enumerates
	// the device with a new address and invalidates our file descriptor.
	// Skip the ioctl if the requested config is already active.
	activeConfig := uint8(0)
	if len(d.configDesc) >= 6 {
		activeConfig = d.configDesc[5] // bConfigurationValue
	}
	if bConfigurationValue != 0 && bConfigurationValue == activeConfig {
		// Already at the right config — just re-claim interfaces.
		return nil
	}

	d.releaseAllInterfaces()

	val := int32(bConfigurationValue)
	if bConfigurationValue == 0 {
		val = -1 // unconfigure
	}
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(d.fd),
		uintptr(usbdevfsSetConfig), uintptr(unsafe.Pointer(&val)))
	if errno != 0 {
		return fmt.Errorf("set configuration %d: %w", bConfigurationValue, errno)
	}

	// Re-read config descriptor
	d.readConfigDescriptor()
	_ = d.DetachKernelDriver()
	return d.claimAllInterfaces()
}

func (d *LinuxUSBDevice) SelectInterface(interfaceNumber, alternateSetting uint8) error {
	req := usbdevfsSetInterfaceReq{
		Interface:  uint32(interfaceNumber),
		AltSetting: uint32(alternateSetting),
	}
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(d.fd),
		uintptr(usbdevfsSetInterface), uintptr(unsafe.Pointer(&req)))
	if errno != 0 {
		return fmt.Errorf("set interface %d/%d: %w", interfaceNumber, alternateSetting, errno)
	}
	return nil
}

func (d *LinuxUSBDevice) ControlTransfer(bmRequestType, bRequest uint8, wValue, wIndex uint16, data []byte, timeout uint32) (int, uint32) {
	ctrl := usbdevfsCtrlTransfer{
		BRequestType: bmRequestType,
		BRequest:     bRequest,
		WValue:       wValue,
		WIndex:       wIndex,
		WLength:      uint16(len(data)),
		Timeout:      timeout,
	}
	if len(data) > 0 {
		ctrl.Data = uintptr(unsafe.Pointer(&data[0]))
	}

	r, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(d.fd),
		uintptr(usbdevfsControl), uintptr(unsafe.Pointer(&ctrl)))
	if errno != 0 {
		return -1, errnoToUSBDStatus(errno)
	}
	return int(r), usbdStatusSuccess
}

func (d *LinuxUSBDevice) BulkOrInterruptTransfer(endpointAddr uint8, data []byte, timeout uint32) (int, uint32) {
	bulk := usbdevfsBulkTransfer{
		Ep:      uint32(endpointAddr),
		Len:     uint32(len(data)),
		Timeout: timeout,
	}
	if len(data) > 0 {
		bulk.Data = uintptr(unsafe.Pointer(&data[0]))
	}

	r, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(d.fd),
		uintptr(usbdevfsBulk), uintptr(unsafe.Pointer(&bulk)))
	if errno != 0 {
		return -1, errnoToUSBDStatus(errno)
	}
	return int(r), usbdStatusSuccess
}

func (d *LinuxUSBDevice) IsochTransfer(endpointAddr uint8, transferFlags uint32, startFrame uint32,
	packets []IsochPacket, data []byte, timeout uint32) ([]IsochPacketResult, []byte, uint32) {
	// Isochronous transfers via usbdevfs require USBDEVFS_SUBMITURB + REAPURB.
	// For now, return not-supported status.
	results := make([]IsochPacketResult, len(packets))
	for i := range results {
		results[i].Status = usbdStatusNotSupported
	}
	return results, nil, usbdStatusNotSupported
}

func (d *LinuxUSBDevice) ClearHalt(endpointAddr uint8) error {
	ep := uint32(endpointAddr)
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(d.fd),
		uintptr(usbdevfsClearHalt), uintptr(unsafe.Pointer(&ep)))
	if errno != 0 {
		return fmt.Errorf("clear halt endpoint %02x: %w", endpointAddr, errno)
	}
	return nil
}

func (d *LinuxUSBDevice) CancelTransfer(requestID uint32) {
	// usbdevfs cancel requires USBDEVFS_DISCARDURB with pointer to submitted URB.
	// For synchronous-only operation, this is a no-op.
}

func (d *LinuxUSBDevice) GetActiveConfig() *MSUSBConfig {
	if len(d.configDesc) < 9 {
		return nil
	}
	return d.buildMSUSBConfig()
}

func (d *LinuxUSBDevice) CompleteConfig(cfg *MSUSBConfig) *MSUSBConfig {
	active := d.buildMSUSBConfig()
	if active == nil {
		return cfg
	}

	cfg.ConfigurationHandle = active.ConfigurationHandle

	// Fill in pipe handles from active config
	for _, mi := range cfg.Interfaces {
		for _, ai := range active.Interfaces {
			if ai.InterfaceNumber == mi.InterfaceNumber {
				mi.InterfaceHandle = ai.InterfaceHandle
				mi.BInterfaceClass = ai.BInterfaceClass
				mi.BInterfaceSubClass = ai.BInterfaceSubClass
				mi.BInterfaceProtocol = ai.BInterfaceProtocol
				// Copy pipe info from active config
				for j, p := range ai.Pipes {
					if j < len(mi.Pipes) {
						mi.Pipes[j].MaximumPacketSize = p.MaximumPacketSize
						mi.Pipes[j].PipeHandle = p.PipeHandle
						mi.Pipes[j].BEndpointAddress = p.BEndpointAddress
						mi.Pipes[j].BInterval = p.BInterval
						mi.Pipes[j].PipeType = p.PipeType
					}
				}
				break
			}
		}
	}

	return cfg
}

func (d *LinuxUSBDevice) Close() {
	d.releaseAllInterfaces()
	syscall.Close(d.fd)
}

// buildMSUSBConfig parses the raw config descriptor into MSUSBConfig format.
func (d *LinuxUSBDevice) buildMSUSBConfig() *MSUSBConfig {
	if len(d.configDesc) < 9 {
		return nil
	}

	bConfigValue := d.configDesc[5]
	numIfaces := int(d.configDesc[4])

	cfg := &MSUSBConfig{
		WTotalLength:        le16(d.configDesc[2:4]),
		BConfigurationValue: bConfigValue,
		ConfigurationHandle: uint32(bConfigValue) | uint32(d.devAddr)<<16 | uint32(d.busNum)<<24,
		NumInterfaces:       uint32(numIfaces),
	}

	// Parse interface and endpoint descriptors
	ifaces := make(map[uint8]*MSUSBInterface)
	var currentIface *MSUSBInterface

	off := 0
	for off+2 <= len(d.configDesc) {
		bLength := int(d.configDesc[off])
		if bLength < 2 || off+bLength > len(d.configDesc) {
			break
		}
		bDescType := d.configDesc[off+1]

		switch bDescType {
		case 0x04: // INTERFACE descriptor
			if bLength < 9 {
				break
			}
			ifNum := d.configDesc[off+2]
			altSetting := d.configDesc[off+3]
			numEndpoints := d.configDesc[off+4]
			ifClass := d.configDesc[off+5]
			ifSubClass := d.configDesc[off+6]
			ifProtocol := d.configDesc[off+7]

			iface := &MSUSBInterface{
				InterfaceNumber:       ifNum,
				AlternateSetting:      altSetting,
				NumberOfPipesExpected: uint16(numEndpoints),
				NumberOfPipes:         uint32(numEndpoints),
				BInterfaceClass:       ifClass,
				BInterfaceSubClass:    ifSubClass,
				BInterfaceProtocol:    ifProtocol,
				InterfaceHandle:       uint32(ifNum) | uint32(altSetting)<<8 | uint32(d.devAddr)<<16 | uint32(d.busNum)<<24,
			}
			// Only use alt setting 0 (default). The raw config descriptor
			// contains all alt settings; higher ones (e.g. UAS alt 1) would
			// overwrite the BOT alt 0 that the server actually uses.
			if altSetting == 0 {
				ifaces[ifNum] = iface
			}
			currentIface = iface

		case 0x05: // ENDPOINT descriptor
			if bLength < 7 || currentIface == nil {
				break
			}
			epAddr := d.configDesc[off+2]
			bmAttributes := d.configDesc[off+3]
			wMaxPacketSize := le16(d.configDesc[off+4 : off+6])
			bInterval := d.configDesc[off+6]

			// Calculate actual max packet size for high-bandwidth endpoints
			maxPktBase := wMaxPacketSize & 0x07FF
			mult := uint16(1) + ((wMaxPacketSize >> 11) & 3)
			pipeType := bmAttributes & 0x03
			if pipeType == 0x01 || pipeType == 0x03 { // isochronous or interrupt
				maxPktBase *= mult
			}

			pipe := &MSUSBPipe{
				MaximumPacketSize:   maxPktBase,
				MaximumTransferSize: 0x00400000, // 4MB default
				PipeHandle:          uint32(epAddr) | uint32(d.devAddr)<<16 | uint32(d.busNum)<<24,
				BEndpointAddress:    epAddr,
				BInterval:           bInterval,
				PipeType:            pipeType,
			}
			currentIface.Pipes = append(currentIface.Pipes, pipe)
		}

		off += bLength
	}

	// Build ordered interface list
	cfg.Interfaces = make([]*MSUSBInterface, 0, numIfaces)
	for i := 0; i < numIfaces; i++ {
		if iface, ok := ifaces[uint8(i)]; ok {
			iface.Length = uint16(msusbInterfaceWriteSize(iface))
			cfg.Interfaces = append(cfg.Interfaces, iface)
		}
	}
	cfg.NumInterfaces = uint32(len(cfg.Interfaces))

	return cfg
}

func le16(b []byte) uint16 {
	return uint16(b[0]) | uint16(b[1])<<8
}

func errnoToUSBDStatus(errno syscall.Errno) uint32 {
	switch errno {
	case syscall.EPIPE:
		return usbdStatusStallPID
	case syscall.ETIMEDOUT:
		return 0xC0006000 // USBD_STATUS_TIMEOUT
	case syscall.ENODEV, syscall.ENXIO:
		return 0xC0007000 // USBD_STATUS_DEVICE_GONE
	default:
		return usbdStatusRequestFailed
	}
}

// OpenUSBDeviceByAddr opens a USB device by bus number and device address.
func OpenUSBDeviceByAddr(busNum, devAddr uint8) (USBDevice, error) {
	return OpenLinuxUSBDevice(busNum, devAddr)
}

// OpenUSBDevicesByVIDPID returns all USB devices matching the given VID/PID filter.
func OpenUSBDevicesByVIDPID(vid, pid uint16) ([]USBDevice, error) {
	devs, err := EnumerateUSBDevices(vid, pid)
	if err != nil {
		return nil, err
	}
	result := make([]USBDevice, len(devs))
	for i, d := range devs {
		result[i] = d
	}
	return result, nil
}

// EnumerateUSBDevices returns all USB devices matching the given VID/PID filter.
// Pass vid=0 and pid=0 to match all devices.
func EnumerateUSBDevices(vid, pid uint16) ([]*LinuxUSBDevice, error) {
	entries, err := os.ReadDir("/sys/bus/usb/devices")
	if err != nil {
		return nil, err
	}

	var devices []*LinuxUSBDevice
	for _, e := range entries {
		name := e.Name()
		// Only match root devices (e.g. "1-2", "2-1.3"), skip ports and endpoints
		if strings.Contains(name, ":") {
			continue
		}
		// Must contain a hyphen (bus-port format)
		if !strings.Contains(name, "-") {
			continue
		}

		dir := filepath.Join("/sys/bus/usb/devices", name)

		busNumStr, err := os.ReadFile(filepath.Join(dir, "busnum"))
		if err != nil {
			continue
		}
		devNumStr, err := os.ReadFile(filepath.Join(dir, "devnum"))
		if err != nil {
			continue
		}

		busNum, err := strconv.Atoi(strings.TrimSpace(string(busNumStr)))
		if err != nil {
			continue
		}
		devNum, err := strconv.Atoi(strings.TrimSpace(string(devNumStr)))
		if err != nil {
			continue
		}

		// Filter by VID/PID if specified
		if vid != 0 || pid != 0 {
			vidStr, err := os.ReadFile(filepath.Join(dir, "idVendor"))
			if err != nil {
				continue
			}
			pidStr, err := os.ReadFile(filepath.Join(dir, "idProduct"))
			if err != nil {
				continue
			}
			devVID, _ := strconv.ParseUint(strings.TrimSpace(string(vidStr)), 16, 16)
			devPID, _ := strconv.ParseUint(strings.TrimSpace(string(pidStr)), 16, 16)

			if vid != 0 && uint16(devVID) != vid {
				continue
			}
			if pid != 0 && uint16(devPID) != pid {
				continue
			}
		}

		dev, err := OpenLinuxUSBDevice(uint8(busNum), uint8(devNum))
		if err != nil {
			continue
		}
		devices = append(devices, dev)
	}

	if len(devices) == 0 {
		return nil, errors.New("no matching USB devices found")
	}
	return devices, nil
}
