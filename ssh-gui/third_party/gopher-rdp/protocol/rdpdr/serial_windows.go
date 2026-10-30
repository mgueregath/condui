//go:build windows

package rdpdr

import (
	"context"
	"encoding/binary"
	"fmt"
	"log/slog"
	"strings"
	"syscall"
	"unsafe"
)

var (
	kernel32                   = syscall.NewLazyDLL("kernel32.dll")
	procSetCommState           = kernel32.NewProc("SetCommState")
	procGetCommState           = kernel32.NewProc("GetCommState")
	procSetCommTimeouts        = kernel32.NewProc("SetCommTimeouts")
	procSetupComm              = kernel32.NewProc("SetupComm")
	procEscapeCommFunction     = kernel32.NewProc("EscapeCommFunction")
	procPurgeComm              = kernel32.NewProc("PurgeComm")
	procGetCommModemStatus     = kernel32.NewProc("GetCommModemStatus")
	procClearCommError         = kernel32.NewProc("ClearCommError")
	procTransmitCommChar       = kernel32.NewProc("TransmitCommChar")
)

// dcb represents the Win32 DCB structure for serial port configuration.
type dcb struct {
	DCBlength  uint32
	BaudRate   uint32
	Flags      uint32
	wReserved  uint16
	XonLim     uint16
	XoffLim    uint16
	ByteSize   byte
	Parity     byte
	StopBits   byte
	XonChar    byte
	XoffChar   byte
	ErrorChar  byte
	EofChar    byte
	EvtChar    byte
	wReserved1 uint16
}

// DCB flag bit positions
const (
	dcbBinary           = 1 << 0
	dcbParity           = 1 << 1
	dcbOutxCtsFlow      = 1 << 2
	dcbOutxDsrFlow      = 1 << 3
	dcbDtrControlShift  = 4  // bits 4-5
	dcbDtrControlMask   = 0x3 << dcbDtrControlShift
	dcbDsrSensitivity   = 1 << 6
	dcbTXContinueOnXoff = 1 << 7
	dcbOutX             = 1 << 8
	dcbInX              = 1 << 9
	dcbErrorChar        = 1 << 10
	dcbNull             = 1 << 11
	dcbRtsControlShift  = 12 // bits 12-13
	dcbRtsControlMask   = 0x3 << dcbRtsControlShift
	dcbAbortOnError     = 1 << 14
)

// DTR/RTS control values
const (
	dtrControlDisable  = 0x00
	dtrControlEnable   = 0x01
	dtrControlHandshake = 0x02
	rtsControlDisable  = 0x00
	rtsControlEnable   = 0x01
	rtsControlHandshake = 0x02
	rtsControlToggle   = 0x03
)

// EscapeCommFunction constants
const (
	escSetXoff  = 1
	escSetXon   = 2
	escSetRts   = 3
	escClrRts   = 4
	escSetDtr   = 5
	escClrDtr   = 6
	escSetBreak = 8
	escClrBreak = 9
)

// commTimeouts represents Win32 COMMTIMEOUTS.
type commTimeouts struct {
	ReadIntervalTimeout         uint32
	ReadTotalTimeoutMultiplier  uint32
	ReadTotalTimeoutConstant    uint32
	WriteTotalTimeoutMultiplier uint32
	WriteTotalTimeoutConstant   uint32
}

// comstat represents Win32 COMSTAT.
type comstat struct {
	Flags    uint32
	CbInQue  uint32
	CbOutQue uint32
}

func getCommState(h syscall.Handle, d *dcb) error {
	r, _, e := procGetCommState.Call(uintptr(h), uintptr(unsafe.Pointer(d)))
	if r == 0 {
		return e
	}
	return nil
}

func setCommState(h syscall.Handle, d *dcb) error {
	r, _, e := procSetCommState.Call(uintptr(h), uintptr(unsafe.Pointer(d)))
	if r == 0 {
		return e
	}
	return nil
}

func setCommTimeoutsWin(h syscall.Handle, t *commTimeouts) error {
	r, _, e := procSetCommTimeouts.Call(uintptr(h), uintptr(unsafe.Pointer(t)))
	if r == 0 {
		return e
	}
	return nil
}

func setupComm(h syscall.Handle, inQueue, outQueue uint32) error {
	r, _, e := procSetupComm.Call(uintptr(h), uintptr(inQueue), uintptr(outQueue))
	if r == 0 {
		return e
	}
	return nil
}

func escapeCommFunction(h syscall.Handle, fn uint32) error {
	r, _, e := procEscapeCommFunction.Call(uintptr(h), uintptr(fn))
	if r == 0 {
		return e
	}
	return nil
}

func purgeComm(h syscall.Handle, flags uint32) error {
	r, _, e := procPurgeComm.Call(uintptr(h), uintptr(flags))
	if r == 0 {
		return e
	}
	return nil
}

func getCommModemStatus(h syscall.Handle) (uint32, error) {
	var status uint32
	r, _, e := procGetCommModemStatus.Call(uintptr(h), uintptr(unsafe.Pointer(&status)))
	if r == 0 {
		return 0, e
	}
	return status, nil
}

func clearCommError(h syscall.Handle) (uint32, comstat, error) {
	var errors uint32
	var cs comstat
	r, _, e := procClearCommError.Call(uintptr(h), uintptr(unsafe.Pointer(&errors)), uintptr(unsafe.Pointer(&cs)))
	if r == 0 {
		return 0, cs, e
	}
	return errors, cs, nil
}

func transmitCommChar(h syscall.Handle, ch byte) error {
	r, _, e := procTransmitCommChar.Call(uintptr(h), uintptr(ch))
	if r == 0 {
		return e
	}
	return nil
}

// handleCreate opens the serial device.
func (s *SerialDevice) handleCreate(h *Handler, req *IORequest) {
	var createOut [5]byte

	s.mu.Lock()
	if s.fd != invalidPortHandle {
		binary.LittleEndian.PutUint32(createOut[0:4], s.id)
		createOut[4] = FileOpened
		s.mu.Unlock()
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusSuccess, createOut[:])
		return
	}
	s.mu.Unlock()

	// Prepend \\.\  for device path if not already present
	path := s.path
	if !strings.HasPrefix(path, `\\.\`) {
		path = `\\.\` + path
	}

	pathp, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		s.log.LogAttrs(context.Background(), slog.LevelError, "serial path encode failed",
			slog.String("path", s.path), slog.Any("err", err))
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusUnsuccessful, createOut[:])
		return
	}

	handle, err := syscall.CreateFile(pathp,
		syscall.GENERIC_READ|syscall.GENERIC_WRITE,
		0, nil, syscall.OPEN_EXISTING, 0, 0)
	if err != nil {
		s.log.LogAttrs(context.Background(), slog.LevelError, "serial open failed",
			slog.String("path", s.path), slog.Any("err", err))
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusUnsuccessful, createOut[:])
		return
	}

	// Set 9600/8N1 defaults
	var d dcb
	d.DCBlength = uint32(unsafe.Sizeof(d))
	if err := getCommState(handle, &d); err == nil {
		d.BaudRate = 9600
		d.ByteSize = 8
		d.Parity = 0  // NOPARITY
		d.StopBits = 0 // ONESTOPBIT
		d.Flags = dcbBinary | (dtrControlEnable << dcbDtrControlShift) | (rtsControlEnable << dcbRtsControlShift)
		_ = setCommState(handle, &d)
	}

	// Set reasonable timeouts
	ct := commTimeouts{
		ReadIntervalTimeout:        0xFFFFFFFF, // MAXDWORD — return immediately with available data
		ReadTotalTimeoutMultiplier: 0,
		ReadTotalTimeoutConstant:   0,
	}
	_ = setCommTimeoutsWin(handle, &ct)

	s.mu.Lock()
	s.fd = uintptr(handle)
	s.dtrRts = 0x03 // DTR and RTS enabled by default
	s.mu.Unlock()

	s.log.LogAttrs(context.Background(), slog.LevelInfo, "serial opened",
		slog.String("path", s.path))

	binary.LittleEndian.PutUint32(createOut[0:4], s.id)
	createOut[4] = FileOpened
	h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusSuccess, createOut[:])
}

// handleClose closes the serial device.
func (s *SerialDevice) handleClose(h *Handler, req *IORequest) {
	s.mu.Lock()
	fd := s.fd
	s.fd = invalidPortHandle
	s.mu.Unlock()

	if fd != invalidPortHandle {
		syscall.CloseHandle(syscall.Handle(fd))
		s.log.LogAttrs(context.Background(), slog.LevelInfo, "serial closed")
	}

	var pad [5]byte
	h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusSuccess, pad[:])
}

// handleRead reads data from the serial port.
// Runs in a goroutine to avoid stalling the RDPDR dispatch.
func (s *SerialDevice) handleRead(h *Handler, req *IORequest) {
	if len(req.Payload) < 32 {
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusInvalidParameter, nil)
		return
	}

	length := binary.LittleEndian.Uint32(req.Payload[0:4])

	s.mu.Lock()
	fd := s.fd
	s.mu.Unlock()

	if fd == invalidPortHandle {
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusInvalidDeviceRequest, nil)
		return
	}

	devID := req.DeviceID
	compID := req.CompletionID
	handle := syscall.Handle(fd)
	go func() {
		buf := make([]byte, 4+length)
		var n uint32
		err := syscall.ReadFile(handle, buf[4:], &n, nil)
		if err != nil && n == 0 {
			h.sendIOCompletion(devID, compID, StatusUnsuccessful, nil)
			return
		}
		binary.LittleEndian.PutUint32(buf[0:4], n)
		h.sendIOCompletion(devID, compID, StatusSuccess, buf[:4+n])
	}()
}

// handleWrite writes data to the serial port.
func (s *SerialDevice) handleWrite(h *Handler, req *IORequest) {
	if len(req.Payload) < 32 {
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusInvalidParameter, nil)
		return
	}

	length := binary.LittleEndian.Uint32(req.Payload[0:4])
	data := req.Payload[32:]
	if uint32(len(data)) < length {
		length = uint32(len(data))
	}

	s.mu.Lock()
	fd := s.fd
	s.mu.Unlock()

	if fd == invalidPortHandle {
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusInvalidDeviceRequest, nil)
		return
	}

	var n uint32
	err := syscall.WriteFile(syscall.Handle(fd), data[:length], &n, nil)
	if err != nil {
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusUnsuccessful, nil)
		return
	}

	var out [5]byte
	binary.LittleEndian.PutUint32(out[0:4], n)
	h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusSuccess, out[:])
}

// handleDeviceControl dispatches serial IOCTLs.
func (s *SerialDevice) handleDeviceControl(h *Handler, req *IORequest) {
	if len(req.Payload) < 32 {
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusInvalidParameter, nil)
		return
	}

	outputBufLen := binary.LittleEndian.Uint32(req.Payload[0:4])
	inputBufLen := binary.LittleEndian.Uint32(req.Payload[4:8])
	ioctl := binary.LittleEndian.Uint32(req.Payload[8:12])
	var inputBuf []byte
	if inputBufLen > 0 && uint32(len(req.Payload)) >= 32+inputBufLen {
		inputBuf = req.Payload[32 : 32+inputBufLen]
	}

	s.log.LogAttrs(context.Background(), slog.LevelDebug, "serial IOCTL",
		slog.String("ioctl", fmt.Sprintf("0x%08X", ioctl)),
		slog.Int("inputLen", int(inputBufLen)),
		slog.Int("outputLen", int(outputBufLen)))

	s.mu.Lock()
	fd := s.fd
	s.mu.Unlock()

	if fd == invalidPortHandle {
		var out [4]byte
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusInvalidDeviceRequest, out[:])
		return
	}

	handle := syscall.Handle(fd)

	switch ioctl {
	case ioctlSerialSetBaudRate:
		s.ioctlSetBaudRate(h, req, inputBuf, handle)
	case ioctlSerialGetBaudRate:
		s.ioctlGetBaudRate(h, req, handle)
	case ioctlSerialSetLineControl:
		s.ioctlSetLineControl(h, req, inputBuf, handle)
	case ioctlSerialGetLineControl:
		s.ioctlGetLineControl(h, req, handle)
	case ioctlSerialSetTimeouts:
		s.ioctlSetTimeouts(h, req, inputBuf)
	case ioctlSerialGetTimeouts:
		s.ioctlGetTimeouts(h, req)
	case ioctlSerialSetChars:
		s.ioctlSetChars(h, req, inputBuf, handle)
	case ioctlSerialGetChars:
		s.ioctlGetChars(h, req, handle)
	case ioctlSerialSetHandflow:
		s.ioctlSetHandflow(h, req, inputBuf, handle)
	case ioctlSerialGetHandflow:
		s.ioctlGetHandflow(h, req, handle)
	case ioctlSerialSetQueueSize:
		s.ioctlSetQueueSize(h, req, inputBuf, handle)
	case ioctlSerialSetDTR:
		s.ioctlEscapeComm(h, req, handle, escSetDtr)
		s.mu.Lock()
		s.dtrRts |= 0x01
		s.mu.Unlock()
	case ioctlSerialClrDTR:
		s.ioctlEscapeComm(h, req, handle, escClrDtr)
		s.mu.Lock()
		s.dtrRts &^= 0x01
		s.mu.Unlock()
	case ioctlSerialSetRTS:
		s.ioctlEscapeComm(h, req, handle, escSetRts)
		s.mu.Lock()
		s.dtrRts |= 0x02
		s.mu.Unlock()
	case ioctlSerialClrRTS:
		s.ioctlEscapeComm(h, req, handle, escClrRts)
		s.mu.Lock()
		s.dtrRts &^= 0x02
		s.mu.Unlock()
	case ioctlSerialSetBreakOn:
		s.ioctlEscapeComm(h, req, handle, escSetBreak)
	case ioctlSerialSetBreakOff:
		s.ioctlEscapeComm(h, req, handle, escClrBreak)
	case ioctlSerialSetXoff:
		s.ioctlEscapeComm(h, req, handle, escSetXoff)
	case ioctlSerialSetXon:
		s.ioctlEscapeComm(h, req, handle, escSetXon)
	case ioctlSerialPurge:
		s.ioctlPurge(h, req, inputBuf, handle)
	case ioctlSerialGetWaitMask:
		s.ioctlGetWaitMask(h, req)
	case ioctlSerialSetWaitMask:
		s.ioctlSetWaitMask(h, req, inputBuf)
	case ioctlSerialWaitOnMask:
		s.ioctlWaitOnMask(h, req)
	case ioctlSerialGetModemStatus:
		s.ioctlGetModemStatus(h, req, handle)
	case ioctlSerialGetDTRRTS:
		s.ioctlGetDTRRTS(h, req)
	case ioctlSerialGetCommStatus:
		s.ioctlGetCommStatus(h, req, handle)
	case ioctlSerialGetProperties:
		s.ioctlGetProperties(h, req)
	case ioctlSerialResetDevice:
		s.ioctlResetDevice(h, req, handle)
	case ioctlSerialImmediateChar:
		s.ioctlImmediateChar(h, req, inputBuf, handle)
	case ioctlSerialConfigSize:
		s.ioctlConfigSize(h, req)
	default:
		s.log.LogAttrs(context.Background(), slog.LevelWarn, "unsupported serial IOCTL",
			slog.String("ioctl", fmt.Sprintf("0x%08X", ioctl)))
		var out [4]byte
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusSuccess, out[:])
	}
}

func (s *SerialDevice) ioctlSetBaudRate(h *Handler, req *IORequest, input []byte, handle syscall.Handle) {
	if len(input) < 4 {
		var out [4]byte
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusInvalidParameter, out[:])
		return
	}
	baud := binary.LittleEndian.Uint32(input[0:4])

	var d dcb
	d.DCBlength = uint32(unsafe.Sizeof(d))
	if err := getCommState(handle, &d); err != nil {
		var out [4]byte
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusUnsuccessful, out[:])
		return
	}
	d.BaudRate = baud
	if err := setCommState(handle, &d); err != nil {
		var out [4]byte
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusUnsuccessful, out[:])
		return
	}

	s.log.LogAttrs(context.Background(), slog.LevelDebug, "set baud rate", slog.Int("baud", int(baud)))
	var out [4]byte
	h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusSuccess, out[:])
}

func (s *SerialDevice) ioctlGetBaudRate(h *Handler, req *IORequest, handle syscall.Handle) {
	var d dcb
	d.DCBlength = uint32(unsafe.Sizeof(d))
	if err := getCommState(handle, &d); err != nil {
		var out [4]byte
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusUnsuccessful, out[:])
		return
	}
	var out [4 + 4]byte
	binary.LittleEndian.PutUint32(out[0:4], 4)
	binary.LittleEndian.PutUint32(out[4:8], d.BaudRate)
	h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusSuccess, out[:])
}

func (s *SerialDevice) ioctlSetLineControl(h *Handler, req *IORequest, input []byte, handle syscall.Handle) {
	if len(input) < 3 {
		var out [4]byte
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusInvalidParameter, out[:])
		return
	}
	stopBits := input[0]
	parity := input[1]
	wordLen := input[2]

	var d dcb
	d.DCBlength = uint32(unsafe.Sizeof(d))
	if err := getCommState(handle, &d); err != nil {
		var out [4]byte
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusUnsuccessful, out[:])
		return
	}

	d.StopBits = stopBits
	d.Parity = parity
	d.ByteSize = wordLen
	if parity != 0 {
		d.Flags |= dcbParity
	} else {
		d.Flags &^= dcbParity
	}

	if err := setCommState(handle, &d); err != nil {
		var out [4]byte
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusUnsuccessful, out[:])
		return
	}
	var out [4]byte
	h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusSuccess, out[:])
}

func (s *SerialDevice) ioctlGetLineControl(h *Handler, req *IORequest, handle syscall.Handle) {
	var d dcb
	d.DCBlength = uint32(unsafe.Sizeof(d))
	if err := getCommState(handle, &d); err != nil {
		var out [4]byte
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusUnsuccessful, out[:])
		return
	}
	var out [4 + 3]byte
	binary.LittleEndian.PutUint32(out[0:4], 3)
	out[4] = d.StopBits
	out[5] = d.Parity
	out[6] = d.ByteSize
	h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusSuccess, out[:])
}

func (s *SerialDevice) ioctlSetChars(h *Handler, req *IORequest, input []byte, handle syscall.Handle) {
	if len(input) < 6 {
		var out [4]byte
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusInvalidParameter, out[:])
		return
	}

	s.mu.Lock()
	copy(s.chars[:], input[:6])
	s.mu.Unlock()

	var d dcb
	d.DCBlength = uint32(unsafe.Sizeof(d))
	if err := getCommState(handle, &d); err == nil {
		d.XonChar = input[4]
		d.XoffChar = input[5]
		d.ErrorChar = input[2]
		d.EofChar = input[0]
		d.EvtChar = input[1]
		_ = setCommState(handle, &d)
	}

	var out [4]byte
	h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusSuccess, out[:])
}

func (s *SerialDevice) ioctlGetChars(h *Handler, req *IORequest, handle syscall.Handle) {
	s.mu.Lock()
	c := s.chars
	s.mu.Unlock()

	var d dcb
	d.DCBlength = uint32(unsafe.Sizeof(d))
	if err := getCommState(handle, &d); err == nil {
		c[0] = d.EofChar
		c[1] = d.EvtChar
		c[2] = d.ErrorChar
		c[3] = 0 // BreakChar — not in DCB, use cached
		c[4] = d.XonChar
		c[5] = d.XoffChar
	}

	var out [4 + 6]byte
	binary.LittleEndian.PutUint32(out[0:4], 6)
	copy(out[4:], c[:])
	h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusSuccess, out[:])
}

func (s *SerialDevice) ioctlSetHandflow(h *Handler, req *IORequest, input []byte, handle syscall.Handle) {
	if len(input) < 16 {
		var out [4]byte
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusInvalidParameter, out[:])
		return
	}

	s.mu.Lock()
	copy(s.handflow[:], input[:16])
	s.mu.Unlock()

	controlHS := binary.LittleEndian.Uint32(input[0:4])
	flowReplace := binary.LittleEndian.Uint32(input[4:8])

	var d dcb
	d.DCBlength = uint32(unsafe.Sizeof(d))
	if err := getCommState(handle, &d); err != nil {
		var out [4]byte
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusUnsuccessful, out[:])
		return
	}

	// Clear flow control bits
	d.Flags &^= dcbOutxCtsFlow | dcbOutxDsrFlow | dcbDtrControlMask | dcbRtsControlMask | dcbOutX | dcbInX

	// CTS handshaking
	if controlHS&0x08 != 0 {
		d.Flags |= dcbOutxCtsFlow
	}
	// DSR handshaking
	if controlHS&0x10 != 0 {
		d.Flags |= dcbOutxDsrFlow
	}
	// DTR control
	dtrCtl := uint32(dtrControlEnable)
	if controlHS&0x01 != 0 {
		dtrCtl = dtrControlHandshake
	}
	d.Flags |= dtrCtl << dcbDtrControlShift
	// RTS control
	rtsCtl := uint32(rtsControlEnable)
	if flowReplace&0x40 != 0 {
		rtsCtl = rtsControlHandshake
	}
	d.Flags |= rtsCtl << dcbRtsControlShift
	// Software flow control
	if flowReplace&0x01 != 0 {
		d.Flags |= dcbOutX
	}
	if flowReplace&0x02 != 0 {
		d.Flags |= dcbInX
	}

	d.XonLim = binary.LittleEndian.Uint16(input[8:10])
	d.XoffLim = binary.LittleEndian.Uint16(input[12:14])

	_ = setCommState(handle, &d)
	var out [4]byte
	h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusSuccess, out[:])
}

func (s *SerialDevice) ioctlGetHandflow(h *Handler, req *IORequest, handle syscall.Handle) {
	var d dcb
	d.DCBlength = uint32(unsafe.Sizeof(d))
	if err := getCommState(handle, &d); err != nil {
		s.mu.Lock()
		hf := s.handflow
		s.mu.Unlock()
		var out [4 + 16]byte
		binary.LittleEndian.PutUint32(out[0:4], 16)
		copy(out[4:], hf[:])
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusSuccess, out[:])
		return
	}

	var hf [16]byte
	var controlHS, flowReplace uint32

	if d.Flags&dcbOutxCtsFlow != 0 {
		controlHS |= 0x08
	}
	if d.Flags&dcbOutxDsrFlow != 0 {
		controlHS |= 0x10
	}
	dtrCtl := (d.Flags & dcbDtrControlMask) >> dcbDtrControlShift
	if dtrCtl == dtrControlHandshake {
		controlHS |= 0x01
	}
	rtsCtl := (d.Flags & dcbRtsControlMask) >> dcbRtsControlShift
	if rtsCtl == rtsControlHandshake {
		flowReplace |= 0x40
	}
	if d.Flags&dcbOutX != 0 {
		flowReplace |= 0x01
	}
	if d.Flags&dcbInX != 0 {
		flowReplace |= 0x02
	}

	binary.LittleEndian.PutUint32(hf[0:4], controlHS)
	binary.LittleEndian.PutUint32(hf[4:8], flowReplace)
	binary.LittleEndian.PutUint16(hf[8:10], d.XonLim)
	binary.LittleEndian.PutUint16(hf[12:14], d.XoffLim)

	var out [4 + 16]byte
	binary.LittleEndian.PutUint32(out[0:4], 16)
	copy(out[4:], hf[:])
	h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusSuccess, out[:])
}

func (s *SerialDevice) ioctlEscapeComm(h *Handler, req *IORequest, handle syscall.Handle, fn uint32) {
	_ = escapeCommFunction(handle, fn)
	var out [4]byte
	h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusSuccess, out[:])
}

func (s *SerialDevice) ioctlPurge(h *Handler, req *IORequest, input []byte, handle syscall.Handle) {
	if len(input) < 4 {
		var out [4]byte
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusInvalidParameter, out[:])
		return
	}
	flags := binary.LittleEndian.Uint32(input[0:4])
	_ = purgeComm(handle, flags)
	var out [4]byte
	h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusSuccess, out[:])
}

func (s *SerialDevice) ioctlGetModemStatus(h *Handler, req *IORequest, handle syscall.Handle) {
	msr, err := getCommModemStatus(handle)
	if err != nil {
		var out [4]byte
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusUnsuccessful, out[:])
		return
	}
	var out [4 + 4]byte
	binary.LittleEndian.PutUint32(out[0:4], 4)
	binary.LittleEndian.PutUint32(out[4:8], msr)
	h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusSuccess, out[:])
}

func (s *SerialDevice) ioctlGetDTRRTS(h *Handler, req *IORequest) {
	s.mu.Lock()
	flags := s.dtrRts
	s.mu.Unlock()
	var out [4 + 4]byte
	binary.LittleEndian.PutUint32(out[0:4], 4)
	binary.LittleEndian.PutUint32(out[4:8], flags)
	h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusSuccess, out[:])
}

func (s *SerialDevice) ioctlGetCommStatus(h *Handler, req *IORequest, handle syscall.Handle) {
	const statusLen = 18
	var out [4 + statusLen]byte
	binary.LittleEndian.PutUint32(out[0:4], statusLen)

	errors, cs, err := clearCommError(handle)
	if err == nil {
		binary.LittleEndian.PutUint32(out[4:8], errors)
		binary.LittleEndian.PutUint32(out[12:16], cs.CbInQue)
		binary.LittleEndian.PutUint32(out[16:20], cs.CbOutQue)
	}

	h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusSuccess, out[:])
}

func (s *SerialDevice) ioctlResetDevice(h *Handler, req *IORequest, handle syscall.Handle) {
	// Reset to 9600/8N1
	var d dcb
	d.DCBlength = uint32(unsafe.Sizeof(d))
	if err := getCommState(handle, &d); err == nil {
		d.BaudRate = 9600
		d.ByteSize = 8
		d.Parity = 0
		d.StopBits = 0
		d.Flags = dcbBinary | (dtrControlEnable << dcbDtrControlShift) | (rtsControlEnable << dcbRtsControlShift)
		_ = setCommState(handle, &d)
	}

	s.mu.Lock()
	s.waitMask = 0
	s.timeouts = [20]byte{}
	s.chars = [6]byte{}
	s.handflow = [16]byte{}
	s.dtrRts = 0x03
	s.mu.Unlock()
	var out [4]byte
	h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusSuccess, out[:])
}

func (s *SerialDevice) ioctlImmediateChar(h *Handler, req *IORequest, input []byte, handle syscall.Handle) {
	if len(input) >= 1 {
		_ = transmitCommChar(handle, input[0])
	}
	var out [4]byte
	h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusSuccess, out[:])
}

func (s *SerialDevice) ioctlSetQueueSize(h *Handler, req *IORequest, input []byte, handle syscall.Handle) {
	if len(input) >= 8 {
		inSize := binary.LittleEndian.Uint32(input[0:4])
		outSize := binary.LittleEndian.Uint32(input[4:8])
		_ = setupComm(handle, inSize, outSize)
	}
	var out [4]byte
	h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusSuccess, out[:])
}
