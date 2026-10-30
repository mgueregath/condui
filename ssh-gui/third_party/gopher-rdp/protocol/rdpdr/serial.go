// Serial port redirection device (MS-RDPESP).
// IRP dispatch and IOCTL code definitions are platform-independent;
// actual device I/O lives in serial_unix.go / serial_windows.go.

package rdpdr

import (
	"encoding/binary"
	"log/slog"
	"sync"
)

// Serial IOCTL codes (MS-RDPESP 2.2.2, derived from ntddser.h)
const (
	ioctlSerialSetBaudRate     uint32 = 0x001B0004
	ioctlSerialGetBaudRate     uint32 = 0x001B0050
	ioctlSerialSetLineControl  uint32 = 0x001B000C
	ioctlSerialGetLineControl  uint32 = 0x001B0054
	ioctlSerialSetTimeouts     uint32 = 0x001B001C
	ioctlSerialGetTimeouts     uint32 = 0x001B0020
	ioctlSerialSetChars        uint32 = 0x001B005C
	ioctlSerialGetChars        uint32 = 0x001B0058
	ioctlSerialSetHandflow     uint32 = 0x001B0064
	ioctlSerialGetHandflow     uint32 = 0x001B0060
	ioctlSerialSetQueueSize    uint32 = 0x001B0008
	ioctlSerialSetDTR          uint32 = 0x001B0024
	ioctlSerialClrDTR          uint32 = 0x001B0028
	ioctlSerialSetRTS          uint32 = 0x001B0030
	ioctlSerialClrRTS          uint32 = 0x001B0034
	ioctlSerialSetBreakOn      uint32 = 0x001B0010
	ioctlSerialSetBreakOff     uint32 = 0x001B0014
	ioctlSerialSetXoff         uint32 = 0x001B0038
	ioctlSerialSetXon          uint32 = 0x001B003C
	ioctlSerialPurge           uint32 = 0x001B004C
	ioctlSerialGetWaitMask     uint32 = 0x001B0040
	ioctlSerialSetWaitMask     uint32 = 0x001B0044
	ioctlSerialWaitOnMask      uint32 = 0x001B0048
	ioctlSerialGetModemStatus  uint32 = 0x001B0068
	ioctlSerialGetDTRRTS       uint32 = 0x001B0078
	ioctlSerialGetCommStatus   uint32 = 0x001B006C
	ioctlSerialGetProperties   uint32 = 0x001B0074
	ioctlSerialResetDevice     uint32 = 0x001B002C
	ioctlSerialImmediateChar   uint32 = 0x001B0018
	ioctlSerialConfigSize      uint32 = 0x001B0080
)

// Windows modem status register bits (MS-RDPESP)
const (
	msrCTSOn  = 0x10
	msrDSROn  = 0x20
	msrRingOn = 0x40
	msrDCDOn  = 0x80
)

// Windows SERIAL_PURGE flags
const (
	serialPurgeTxAbort = 0x00000001
	serialPurgeRxAbort = 0x00000002
	serialPurgeTxClear = 0x00000004
	serialPurgeRxClear = 0x00000008
)

// SerialDevice represents a redirected serial port.
type SerialDevice struct {
	id   uint32
	name string // e.g. "COM3"
	path string // e.g. "/dev/ttyUSB0" or "COM3"
	log  *slog.Logger
	mu   sync.Mutex
	fd   uintptr // platform handle, invalidPortHandle when closed

	// Cached state for GET IOCTLs
	waitMask uint32
	timeouts [20]byte  // SERIAL_TIMEOUTS (5 x uint32)
	chars    [6]byte   // SERIAL_CHARS
	handflow [16]byte  // SERIAL_HANDFLOW
	dtrRts   uint32    // cached DTR/RTS state for Windows GetDTRRTS
}

// NewSerialDevice creates a new serial port device.
func NewSerialDevice(id uint32, name, path string, log *slog.Logger) *SerialDevice {
	return &SerialDevice{
		id:   id,
		name: name,
		path: path,
		log:  log.With("device", name),
		fd:   invalidPortHandle,
	}
}

// ID returns the device ID.
func (s *SerialDevice) ID() uint32 { return s.id }

// Type returns DeviceTypeSerial.
func (s *SerialDevice) Type() uint32 { return DeviceTypeSerial }

// Name returns the device display name.
func (s *SerialDevice) Name() string { return s.name }

// HandleIRP dispatches an I/O request to the appropriate handler.
func (s *SerialDevice) HandleIRP(h *Handler, req *IORequest) {
	switch req.MajorFn {
	case IrpCreate:
		s.handleCreate(h, req)
	case IrpClose:
		s.handleClose(h, req)
	case IrpRead:
		s.handleRead(h, req)
	case IrpWrite:
		s.handleWrite(h, req)
	case IrpDeviceControl:
		s.handleDeviceControl(h, req)
	default:
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusNotSupported, nil)
	}
}

// Shared IOCTL handlers — no platform-specific code.

// ioctlSetTimeouts stores the serial timeouts internally.
func (s *SerialDevice) ioctlSetTimeouts(h *Handler, req *IORequest, input []byte) {
	if len(input) >= 20 {
		s.mu.Lock()
		copy(s.timeouts[:], input[:20])
		s.mu.Unlock()
	}
	var out [4]byte
	h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusSuccess, out[:])
}

// ioctlGetTimeouts returns the stored serial timeouts.
func (s *SerialDevice) ioctlGetTimeouts(h *Handler, req *IORequest) {
	s.mu.Lock()
	t := s.timeouts
	s.mu.Unlock()
	var out [4 + 20]byte
	binary.LittleEndian.PutUint32(out[0:4], 20)
	copy(out[4:], t[:])
	h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusSuccess, out[:])
}

// ioctlGetProperties returns fixed SERIAL_COMMPROP (MS-RDPESP).
func (s *SerialDevice) ioctlGetProperties(h *Handler, req *IORequest) {
	const propsLen = 64
	var out [4 + propsLen]byte
	binary.LittleEndian.PutUint32(out[0:4], propsLen)
	// PacketLength(2)
	binary.LittleEndian.PutUint16(out[4:6], propsLen)
	// PacketVersion(2)
	binary.LittleEndian.PutUint16(out[6:8], 2)
	// ServiceMask(4) = SERIAL_SP_SERIALCOMM
	binary.LittleEndian.PutUint32(out[8:12], 0x00000001)
	// MaxTxQueue(4)
	binary.LittleEndian.PutUint32(out[16:20], 0)
	// MaxRxQueue(4)
	binary.LittleEndian.PutUint32(out[20:24], 0)
	// MaxBaud(4) = SERIAL_BAUD_115200
	binary.LittleEndian.PutUint32(out[24:28], 0x00020000)
	// ProvSubType(4) = SERIAL_SP_RS232
	binary.LittleEndian.PutUint32(out[28:32], 0x00000001)
	// ProvCapabilities(4) = DTR/RTS/RLSD/PARITY/XONXOFF/SETTABLE_*
	binary.LittleEndian.PutUint32(out[32:36], 0x000001FF)
	// SettableParams(4)
	binary.LittleEndian.PutUint32(out[36:40], 0x0000007F)
	// SettableBaud(4) = common baud rates
	binary.LittleEndian.PutUint32(out[40:44], 0x0007FFFF)
	// SettableData(4) = 5,6,7,8 bit
	binary.LittleEndian.PutUint16(out[44:46], 0x000F)
	// SettableStopParity(2)
	binary.LittleEndian.PutUint16(out[46:48], 0x1F07)
	// CurrentTxQueue(4)
	binary.LittleEndian.PutUint32(out[48:52], 0)
	// CurrentRxQueue(4)
	binary.LittleEndian.PutUint32(out[52:56], 0)
	h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusSuccess, out[:])
}

// ioctlGetWaitMask returns the stored wait mask.
func (s *SerialDevice) ioctlGetWaitMask(h *Handler, req *IORequest) {
	s.mu.Lock()
	mask := s.waitMask
	s.mu.Unlock()
	var out [4 + 4]byte // OutputBufferLength(4) + WaitMask(4)
	binary.LittleEndian.PutUint32(out[0:4], 4)
	binary.LittleEndian.PutUint32(out[4:8], mask)
	h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusSuccess, out[:])
}

// ioctlSetWaitMask stores the wait mask.
func (s *SerialDevice) ioctlSetWaitMask(h *Handler, req *IORequest, input []byte) {
	if len(input) >= 4 {
		s.mu.Lock()
		s.waitMask = binary.LittleEndian.Uint32(input[0:4])
		s.mu.Unlock()
	}
	var out [4]byte
	h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusSuccess, out[:])
}

// ioctlWaitOnMask returns 0 (no events pending).
func (s *SerialDevice) ioctlWaitOnMask(h *Handler, req *IORequest) {
	var out [4 + 4]byte
	binary.LittleEndian.PutUint32(out[0:4], 4)
	h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusSuccess, out[:])
}

// ioctlConfigSize returns 0 (no driver-specific config).
func (s *SerialDevice) ioctlConfigSize(h *Handler, req *IORequest) {
	var out [4 + 4]byte
	binary.LittleEndian.PutUint32(out[0:4], 4)
	h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusSuccess, out[:])
}
