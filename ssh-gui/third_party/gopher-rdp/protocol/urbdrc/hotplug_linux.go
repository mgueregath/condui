//go:build linux

package urbdrc

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// sysfsScanner scans /sys/bus/usb/devices/ for matching USB devices.
type sysfsScanner struct{}

func (sysfsScanner) scanDevices(filters []USBDeviceFilter) []usbDeviceIdentity {
	entries, err := os.ReadDir("/sys/bus/usb/devices")
	if err != nil {
		return nil
	}

	var result []usbDeviceIdentity
	for _, e := range entries {
		name := e.Name()
		// Only match root devices (e.g. "1-2", "2-1.3"), skip interfaces and hubs
		if strings.Contains(name, ":") || !strings.Contains(name, "-") {
			continue
		}

		dir := filepath.Join("/sys/bus/usb/devices", name)

		vid, pid, ok := readSysfsVIDPID(dir)
		if !ok {
			continue
		}
		if !matchesAnyFilter(vid, pid, filters) {
			continue
		}

		busNum, devAddr, ok := readSysfsBusAddr(dir)
		if !ok {
			continue
		}

		devClass := readSysfsDeviceClass(dir)

		result = append(result, usbDeviceIdentity{
			BusNum:      busNum,
			DevAddr:     devAddr,
			VID:         vid,
			PID:         pid,
			DeviceClass: devClass,
			SysPath:     name,
		})
	}
	return result
}

func readSysfsVIDPID(dir string) (uint16, uint16, bool) {
	vidData, err := os.ReadFile(filepath.Join(dir, "idVendor"))
	if err != nil {
		return 0, 0, false
	}
	pidData, err := os.ReadFile(filepath.Join(dir, "idProduct"))
	if err != nil {
		return 0, 0, false
	}
	vid, err := strconv.ParseUint(strings.TrimSpace(string(vidData)), 16, 16)
	if err != nil {
		return 0, 0, false
	}
	pid, err := strconv.ParseUint(strings.TrimSpace(string(pidData)), 16, 16)
	if err != nil {
		return 0, 0, false
	}
	return uint16(vid), uint16(pid), true
}

func readSysfsBusAddr(dir string) (uint8, uint8, bool) {
	busData, err := os.ReadFile(filepath.Join(dir, "busnum"))
	if err != nil {
		return 0, 0, false
	}
	devData, err := os.ReadFile(filepath.Join(dir, "devnum"))
	if err != nil {
		return 0, 0, false
	}
	busNum, err := strconv.Atoi(strings.TrimSpace(string(busData)))
	if err != nil {
		return 0, 0, false
	}
	devNum, err := strconv.Atoi(strings.TrimSpace(string(devData)))
	if err != nil {
		return 0, 0, false
	}
	return uint8(busNum), uint8(devNum), true
}

func readSysfsDeviceClass(dir string) uint8 {
	// Read device-level class first.
	data, err := os.ReadFile(filepath.Join(dir, "bDeviceClass"))
	if err != nil {
		return 0
	}
	v, _ := strconv.ParseUint(strings.TrimSpace(string(data)), 16, 8)
	if v != 0 {
		return uint8(v) // device reports its own class (e.g. hub=0x09)
	}
	// bDeviceClass=0 means "per-interface" — most USB devices (HID, mass
	// storage, smartcard, etc.) use this. Read the first interface's class
	// to get the actual class for filtering.
	base := filepath.Base(dir) // e.g. "1-5"
	ifDir := filepath.Join(dir, base+":1.0")
	data, err = os.ReadFile(filepath.Join(ifDir, "bInterfaceClass"))
	if err != nil {
		return 0
	}
	v, _ = strconv.ParseUint(strings.TrimSpace(string(data)), 16, 8)
	return uint8(v)
}

func matchesAnyFilter(vid, pid uint16, filters []USBDeviceFilter) bool {
	for _, f := range filters {
		if (f.VID == 0 || f.VID == vid) && (f.PID == 0 || f.PID == pid) {
			return true
		}
	}
	return false
}

// OpenUSBDevicesAuto scans sysfs for all USB devices, skips excluded device
// classes, and opens the rest. Returns nil, nil when no devices are found
// (not an error — devices may appear later via hotplug).
func OpenUSBDevicesAuto(excludeClasses []uint8) ([]USBDevice, error) {
	scanner := sysfsScanner{}
	found := scanner.scanDevices([]USBDeviceFilter{{VID: 0, PID: 0}})

	var devices []USBDevice
	for _, id := range found {
		if isClassInList(id.DeviceClass, excludeClasses) {
			continue
		}
		dev, err := OpenLinuxUSBDevice(id.BusNum, id.DevAddr)
		if err != nil {
			continue
		}
		devices = append(devices, dev)
	}
	return devices, nil
}

func isClassInList(class uint8, list []uint8) bool {
	for _, c := range list {
		if c == class {
			return true
		}
	}
	return false
}
