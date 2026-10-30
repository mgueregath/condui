//go:build !windows

package rdpdr

import (
	"context"
	"encoding/binary"
	"fmt"
	"log/slog"
	"syscall"
	"unsafe"
)

// termios constants not in the syscall package
const (
	// c_cflag bits
	cmspar  = 0x40000000
	crtscts = 0x80000000

	// Baud rate constants
	b50     = 0x1
	b75     = 0x2
	b110    = 0x3
	b134    = 0x4
	b150    = 0x5
	b200    = 0x6
	b300    = 0x7
	b600    = 0x8
	b1200   = 0x9
	b1800   = 0xA
	b2400   = 0xB
	b4800   = 0xC
	b9600   = 0xD
	b19200  = 0xE
	b38400  = 0xF
	b57600  = 0x1001
	b115200 = 0x1002
	b230400 = 0x1003
	b460800 = 0x1004

	// Modem line bits
	tiocmDTR = 0x002
	tiocmRTS = 0x004
	tiocmCTS = 0x020
	tiocmDSR = 0x100
	tiocmRNG = 0x080
	tiocmDCD = 0x040

	// ioctl numbers
	_TIOCMGET = 0x5415
	_TIOCMBIS = 0x5416
	_TIOCMBIC = 0x5417
	_TIOCSBRK = 0x5427
	_TIOCCBRK = 0x5428
	_TIOCINQ  = 0x541B
	_TIOCOUTQ = 0x5411
	_TCGETS   = 0x5401
	_TCSETS   = 0x5402
	_TCFLSH   = 0x540B
	_TCXONC   = 0x540A

	// tcflow actions
	tcioff = 2
	tcion  = 3

	// tcflush queues
	tciflush  = 0
	tcoflush  = 1
	tcioflush = 2
)

func tcgetattr(fd int, t *syscall.Termios) error {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), _TCGETS, uintptr(unsafe.Pointer(t)))
	if errno != 0 {
		return errno
	}
	return nil
}

func tcsetattr(fd int, t *syscall.Termios) error {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), _TCSETS, uintptr(unsafe.Pointer(t)))
	if errno != 0 {
		return errno
	}
	return nil
}

func ioctlInt(fd int, req uint, val int) error {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), uintptr(req), uintptr(unsafe.Pointer(&val)))
	if errno != 0 {
		return errno
	}
	return nil
}

func ioctlGetInt(fd int, req uint) (int, error) {
	var val int
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), uintptr(req), uintptr(unsafe.Pointer(&val)))
	if errno != 0 {
		return 0, errno
	}
	return val, nil
}

// handleCreate opens the serial device.
func (s *SerialDevice) handleCreate(h *Handler, req *IORequest) {
	var createOut [5]byte

	s.mu.Lock()
	if s.fd != invalidPortHandle {
		// Already open — return existing handle
		binary.LittleEndian.PutUint32(createOut[0:4], s.id)
		createOut[4] = FileOpened
		s.mu.Unlock()
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusSuccess, createOut[:])
		return
	}
	s.mu.Unlock()

	fd, err := syscall.Open(s.path, syscall.O_RDWR|syscall.O_NOCTTY|syscall.O_NONBLOCK, 0)
	if err != nil {
		s.log.LogAttrs(context.Background(), slog.LevelError, "serial open failed",
			slog.String("path", s.path), slog.Any("err", err))
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusUnsuccessful, createOut[:])
		return
	}

	// Clear O_NONBLOCK after open
	flags, err := fcntlGetFlags(fd)
	if err == nil {
		_ = fcntlSetFlags(fd, flags&^syscall.O_NONBLOCK)
	}

	// Reset to 9600/8N1 defaults
	resetTermios(fd)

	s.mu.Lock()
	s.fd = uintptr(fd)
	s.mu.Unlock()

	s.log.LogAttrs(context.Background(), slog.LevelInfo, "serial opened",
		slog.String("path", s.path))

	binary.LittleEndian.PutUint32(createOut[0:4], s.id)
	createOut[4] = FileOpened
	h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusSuccess, createOut[:])
}

func fcntlGetFlags(fd int) (int, error) {
	val, _, errno := syscall.Syscall(syscall.SYS_FCNTL, uintptr(fd), uintptr(syscall.F_GETFL), 0)
	if errno != 0 {
		return 0, errno
	}
	return int(val), nil
}

func fcntlSetFlags(fd int, flags int) error {
	_, _, errno := syscall.Syscall(syscall.SYS_FCNTL, uintptr(fd), uintptr(syscall.F_SETFL), uintptr(flags))
	if errno != 0 {
		return errno
	}
	return nil
}

// resetTermios sets the terminal to 9600/8N1 raw mode.
func resetTermios(fd int) {
	var t syscall.Termios
	if err := tcgetattr(fd, &t); err != nil {
		return
	}
	// Raw mode
	t.Iflag &^= syscall.IGNBRK | syscall.BRKINT | syscall.PARMRK | syscall.ISTRIP |
		syscall.INLCR | syscall.IGNCR | syscall.ICRNL | syscall.IXON
	t.Oflag &^= syscall.OPOST
	t.Lflag &^= syscall.ECHO | syscall.ECHONL | syscall.ICANON | syscall.ISIG | syscall.IEXTEN
	t.Cflag &^= syscall.CSIZE | syscall.PARENB
	t.Cflag |= syscall.CS8 | syscall.CLOCAL | syscall.CREAD
	// 9600 baud
	setTermiosSpeed(&t, b9600)
	// VMIN=1, VTIME=0 — blocking read for at least 1 byte
	t.Cc[syscall.VMIN] = 1
	t.Cc[syscall.VTIME] = 0
	_ = tcsetattr(fd, &t)
}

// handleClose closes the serial device.
func (s *SerialDevice) handleClose(h *Handler, req *IORequest) {
	s.mu.Lock()
	fd := s.fd
	s.fd = invalidPortHandle
	s.mu.Unlock()

	if fd != invalidPortHandle {
		syscall.Close(int(fd))
		s.log.LogAttrs(context.Background(), slog.LevelInfo, "serial closed")
	}

	var pad [5]byte
	h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusSuccess, pad[:])
}

// handleRead reads data from the serial port.
// Runs the blocking syscall.Read in a goroutine to avoid stalling the RDPDR dispatch.
func (s *SerialDevice) handleRead(h *Handler, req *IORequest) {
	if len(req.Payload) < 32 {
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusInvalidParameter, nil)
		return
	}

	length := binary.LittleEndian.Uint32(req.Payload[0:4])
	// offset(8) + padding(20) ignored for serial

	s.mu.Lock()
	fd := s.fd
	s.mu.Unlock()

	if fd == invalidPortHandle {
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusInvalidDeviceRequest, nil)
		return
	}

	devID := req.DeviceID
	compID := req.CompletionID
	ifd := int(fd)
	go func() {
		buf := make([]byte, 4+length)
		n, err := syscall.Read(ifd, buf[4:])
		if err != nil && n <= 0 {
			h.sendIOCompletion(devID, compID, StatusUnsuccessful, nil)
			return
		}
		binary.LittleEndian.PutUint32(buf[0:4], uint32(n))
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
	// offset(8) + padding(20) ignored for serial
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

	n, err := syscall.Write(int(fd), data[:length])
	if err != nil {
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusUnsuccessful, nil)
		return
	}

	var out [5]byte
	binary.LittleEndian.PutUint32(out[0:4], uint32(n))
	h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusSuccess, out[:])
}

// handleDeviceControl dispatches serial IOCTLs.
func (s *SerialDevice) handleDeviceControl(h *Handler, req *IORequest) {
	// MS-RDPEFS 2.2.1.4.5: OutputBufferLength(4) + InputBufferLength(4) +
	// IoControlCode(4) + Padding(20) + InputBuffer(variable)
	if len(req.Payload) < 32 {
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusInvalidParameter, nil)
		return
	}

	outputBufLen := binary.LittleEndian.Uint32(req.Payload[0:4])
	inputBufLen := binary.LittleEndian.Uint32(req.Payload[4:8])
	ioctl := binary.LittleEndian.Uint32(req.Payload[8:12])
	// padding at [12:32]
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

	ifd := int(fd)

	switch ioctl {
	case ioctlSerialSetBaudRate:
		s.ioctlSetBaudRate(h, req, inputBuf, ifd)
	case ioctlSerialGetBaudRate:
		s.ioctlGetBaudRate(h, req, ifd)
	case ioctlSerialSetLineControl:
		s.ioctlSetLineControl(h, req, inputBuf, ifd)
	case ioctlSerialGetLineControl:
		s.ioctlGetLineControl(h, req, ifd)
	case ioctlSerialSetTimeouts:
		s.ioctlSetTimeouts(h, req, inputBuf)
	case ioctlSerialGetTimeouts:
		s.ioctlGetTimeouts(h, req)
	case ioctlSerialSetChars:
		s.ioctlSetChars(h, req, inputBuf, ifd)
	case ioctlSerialGetChars:
		s.ioctlGetChars(h, req, ifd)
	case ioctlSerialSetHandflow:
		s.ioctlSetHandflow(h, req, inputBuf, ifd)
	case ioctlSerialGetHandflow:
		s.ioctlGetHandflow(h, req, ifd)
	case ioctlSerialSetQueueSize:
		// No-op on Linux — kernel manages buffers
		var out [4]byte
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusSuccess, out[:])
	case ioctlSerialSetDTR:
		s.ioctlSetModemBit(h, req, ifd, true, true)
	case ioctlSerialClrDTR:
		s.ioctlSetModemBit(h, req, ifd, true, false)
	case ioctlSerialSetRTS:
		s.ioctlSetModemBit(h, req, ifd, false, true)
	case ioctlSerialClrRTS:
		s.ioctlSetModemBit(h, req, ifd, false, false)
	case ioctlSerialSetBreakOn:
		s.ioctlSetBreak(h, req, ifd, true)
	case ioctlSerialSetBreakOff:
		s.ioctlSetBreak(h, req, ifd, false)
	case ioctlSerialSetXoff:
		s.ioctlFlowControl(h, req, ifd, false)
	case ioctlSerialSetXon:
		s.ioctlFlowControl(h, req, ifd, true)
	case ioctlSerialPurge:
		s.ioctlPurge(h, req, inputBuf, ifd)
	case ioctlSerialGetWaitMask:
		s.ioctlGetWaitMask(h, req)
	case ioctlSerialSetWaitMask:
		s.ioctlSetWaitMask(h, req, inputBuf)
	case ioctlSerialWaitOnMask:
		s.ioctlWaitOnMask(h, req)
	case ioctlSerialGetModemStatus:
		s.ioctlGetModemStatus(h, req, ifd)
	case ioctlSerialGetDTRRTS:
		s.ioctlGetDTRRTS(h, req, ifd)
	case ioctlSerialGetCommStatus:
		s.ioctlGetCommStatus(h, req, ifd)
	case ioctlSerialGetProperties:
		s.ioctlGetProperties(h, req)
	case ioctlSerialResetDevice:
		s.ioctlResetDevice(h, req, ifd)
	case ioctlSerialImmediateChar:
		s.ioctlImmediateChar(h, req, inputBuf, ifd)
	case ioctlSerialConfigSize:
		s.ioctlConfigSize(h, req)
	default:
		s.log.LogAttrs(context.Background(), slog.LevelWarn, "unsupported serial IOCTL",
			slog.String("ioctl", fmt.Sprintf("0x%08X", ioctl)))
		var out [4]byte
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusSuccess, out[:])
	}
}

// baudToSpeed maps Windows baud rates to termios speed constants.
func baudToSpeed(baud uint32) uint32 {
	switch baud {
	case 50:
		return b50
	case 75:
		return b75
	case 110:
		return b110
	case 134:
		return b134
	case 150:
		return b150
	case 200:
		return b200
	case 300:
		return b300
	case 600:
		return b600
	case 1200:
		return b1200
	case 1800:
		return b1800
	case 2400:
		return b2400
	case 4800:
		return b4800
	case 9600:
		return b9600
	case 19200:
		return b19200
	case 38400:
		return b38400
	case 57600:
		return b57600
	case 115200:
		return b115200
	case 230400:
		return b230400
	case 460800:
		return b460800
	default:
		return b9600
	}
}

// speedToBaud maps termios speed constants to Windows baud rates.
func speedToBaud(speed uint32) uint32 {
	switch speed {
	case b50:
		return 50
	case b75:
		return 75
	case b110:
		return 110
	case b134:
		return 134
	case b150:
		return 150
	case b200:
		return 200
	case b300:
		return 300
	case b600:
		return 600
	case b1200:
		return 1200
	case b1800:
		return 1800
	case b2400:
		return 2400
	case b4800:
		return 4800
	case b9600:
		return 9600
	case b19200:
		return 19200
	case b38400:
		return 38400
	case b57600:
		return 57600
	case b115200:
		return 115200
	case b230400:
		return 230400
	case b460800:
		return 460800
	default:
		return 9600
	}
}

func (s *SerialDevice) ioctlSetBaudRate(h *Handler, req *IORequest, input []byte, fd int) {
	if len(input) < 4 {
		var out [4]byte
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusInvalidParameter, out[:])
		return
	}
	baud := binary.LittleEndian.Uint32(input[0:4])
	speed := baudToSpeed(baud)

	var t syscall.Termios
	if err := tcgetattr(fd, &t); err != nil {
		var out [4]byte
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusUnsuccessful, out[:])
		return
	}
	setTermiosSpeed(&t, speed)
	if err := tcsetattr(fd, &t); err != nil {
		var out [4]byte
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusUnsuccessful, out[:])
		return
	}

	s.log.LogAttrs(context.Background(), slog.LevelDebug, "set baud rate", slog.Int("baud", int(baud)))
	var out [4]byte
	h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusSuccess, out[:])
}

func (s *SerialDevice) ioctlGetBaudRate(h *Handler, req *IORequest, fd int) {
	var t syscall.Termios
	if err := tcgetattr(fd, &t); err != nil {
		var out [4]byte
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusUnsuccessful, out[:])
		return
	}
	baud := speedToBaud(getTermiosSpeed(&t))
	var out [4 + 4]byte
	binary.LittleEndian.PutUint32(out[0:4], 4)
	binary.LittleEndian.PutUint32(out[4:8], baud)
	h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusSuccess, out[:])
}

func (s *SerialDevice) ioctlSetLineControl(h *Handler, req *IORequest, input []byte, fd int) {
	if len(input) < 3 {
		var out [4]byte
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusInvalidParameter, out[:])
		return
	}
	stopBits := input[0]
	parity := input[1]
	wordLen := input[2]

	var t syscall.Termios
	if err := tcgetattr(fd, &t); err != nil {
		var out [4]byte
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusUnsuccessful, out[:])
		return
	}

	// Stop bits: 0=1stop, 1=1.5stop, 2=2stop
	t.Cflag &^= syscall.CSTOPB
	if stopBits == 2 {
		t.Cflag |= syscall.CSTOPB
	}

	// Parity: 0=none, 1=odd, 2=even, 3=mark, 4=space
	t.Cflag &^= syscall.PARENB | syscall.PARODD | cmspar
	switch parity {
	case 1: // odd
		t.Cflag |= syscall.PARENB | syscall.PARODD
	case 2: // even
		t.Cflag |= syscall.PARENB
	case 3: // mark
		t.Cflag |= syscall.PARENB | syscall.PARODD | cmspar
	case 4: // space
		t.Cflag |= syscall.PARENB | cmspar
	}

	// Word length
	t.Cflag &^= syscall.CSIZE
	switch wordLen {
	case 5:
		t.Cflag |= syscall.CS5
	case 6:
		t.Cflag |= syscall.CS6
	case 7:
		t.Cflag |= syscall.CS7
	default:
		t.Cflag |= syscall.CS8
	}

	if err := tcsetattr(fd, &t); err != nil {
		var out [4]byte
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusUnsuccessful, out[:])
		return
	}
	var out [4]byte
	h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusSuccess, out[:])
}

func (s *SerialDevice) ioctlGetLineControl(h *Handler, req *IORequest, fd int) {
	var t syscall.Termios
	if err := tcgetattr(fd, &t); err != nil {
		var out [4]byte
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusUnsuccessful, out[:])
		return
	}

	var stopBits, parity, wordLen byte

	// Stop bits
	if t.Cflag&syscall.CSTOPB != 0 {
		stopBits = 2
	}

	// Parity
	if t.Cflag&syscall.PARENB != 0 {
		if t.Cflag&cmspar != 0 {
			if t.Cflag&syscall.PARODD != 0 {
				parity = 3 // mark
			} else {
				parity = 4 // space
			}
		} else if t.Cflag&syscall.PARODD != 0 {
			parity = 1 // odd
		} else {
			parity = 2 // even
		}
	}

	// Word length
	switch t.Cflag & syscall.CSIZE {
	case syscall.CS5:
		wordLen = 5
	case syscall.CS6:
		wordLen = 6
	case syscall.CS7:
		wordLen = 7
	default:
		wordLen = 8
	}

	var out [4 + 3]byte
	binary.LittleEndian.PutUint32(out[0:4], 3)
	out[4] = stopBits
	out[5] = parity
	out[6] = wordLen
	h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusSuccess, out[:])
}

func (s *SerialDevice) ioctlSetChars(h *Handler, req *IORequest, input []byte, fd int) {
	if len(input) < 6 {
		var out [4]byte
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusInvalidParameter, out[:])
		return
	}

	s.mu.Lock()
	copy(s.chars[:], input[:6])
	s.mu.Unlock()

	// Apply XON/XOFF chars to termios
	var t syscall.Termios
	if err := tcgetattr(fd, &t); err == nil {
		t.Cc[syscall.VSTART] = input[4] // XonChar
		t.Cc[syscall.VSTOP] = input[5]  // XoffChar
		_ = tcsetattr(fd, &t)
	}

	var out [4]byte
	h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusSuccess, out[:])
}

func (s *SerialDevice) ioctlGetChars(h *Handler, req *IORequest, fd int) {
	s.mu.Lock()
	c := s.chars
	s.mu.Unlock()

	// Overlay with actual termios values for XON/XOFF
	var t syscall.Termios
	if err := tcgetattr(fd, &t); err == nil {
		c[4] = t.Cc[syscall.VSTART]
		c[5] = t.Cc[syscall.VSTOP]
	}

	var out [4 + 6]byte
	binary.LittleEndian.PutUint32(out[0:4], 6)
	copy(out[4:], c[:])
	h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusSuccess, out[:])
}

func (s *SerialDevice) ioctlSetHandflow(h *Handler, req *IORequest, input []byte, fd int) {
	if len(input) < 16 {
		var out [4]byte
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusInvalidParameter, out[:])
		return
	}

	s.mu.Lock()
	copy(s.handflow[:], input[:16])
	s.mu.Unlock()

	// SERIAL_HANDFLOW: ControlHandShake(4) + FlowReplace(4) + XonLimit(4) + XoffLimit(4)
	controlHS := binary.LittleEndian.Uint32(input[0:4])
	flowReplace := binary.LittleEndian.Uint32(input[4:8])

	var t syscall.Termios
	if err := tcgetattr(fd, &t); err != nil {
		var out [4]byte
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusUnsuccessful, out[:])
		return
	}

	// CTS handshaking → CRTSCTS
	t.Cflag &^= crtscts
	if controlHS&0x08 != 0 { // SERIAL_CTS_HANDSHAKE
		t.Cflag |= crtscts
	}

	// DTR control → HUPCL
	t.Cflag &^= syscall.HUPCL
	if controlHS&0x01 != 0 { // SERIAL_DTR_CONTROL
		t.Cflag |= syscall.HUPCL
	}

	// Software flow control
	t.Iflag &^= syscall.IXON | syscall.IXOFF
	if flowReplace&0x01 != 0 { // SERIAL_AUTO_TRANSMIT (IXON)
		t.Iflag |= syscall.IXON
	}
	if flowReplace&0x02 != 0 { // SERIAL_AUTO_RECEIVE (IXOFF)
		t.Iflag |= syscall.IXOFF
	}

	_ = tcsetattr(fd, &t)
	var out [4]byte
	h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusSuccess, out[:])
}

func (s *SerialDevice) ioctlGetHandflow(h *Handler, req *IORequest, fd int) {
	var t syscall.Termios
	if err := tcgetattr(fd, &t); err != nil {
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

	if t.Cflag&crtscts != 0 {
		controlHS |= 0x08  // SERIAL_CTS_HANDSHAKE
		flowReplace |= 0x40 // SERIAL_RTS_HANDSHAKE
	}
	if t.Cflag&syscall.HUPCL != 0 {
		controlHS |= 0x01 // SERIAL_DTR_CONTROL
	}
	if t.Iflag&syscall.IXON != 0 {
		flowReplace |= 0x01 // SERIAL_AUTO_TRANSMIT
	}
	if t.Iflag&syscall.IXOFF != 0 {
		flowReplace |= 0x02 // SERIAL_AUTO_RECEIVE
	}

	binary.LittleEndian.PutUint32(hf[0:4], controlHS)
	binary.LittleEndian.PutUint32(hf[4:8], flowReplace)
	// XonLimit/XoffLimit from stored values
	s.mu.Lock()
	copy(hf[8:], s.handflow[8:16])
	s.mu.Unlock()

	var out [4 + 16]byte
	binary.LittleEndian.PutUint32(out[0:4], 16)
	copy(out[4:], hf[:])
	h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusSuccess, out[:])
}

func (s *SerialDevice) ioctlSetModemBit(h *Handler, req *IORequest, fd int, isDTR bool, set bool) {
	var bit int
	if isDTR {
		bit = tiocmDTR
	} else {
		bit = tiocmRTS
	}

	var cmd uint
	if set {
		cmd = _TIOCMBIS
	} else {
		cmd = _TIOCMBIC
	}

	_ = ioctlInt(fd, cmd, bit)
	var out [4]byte
	h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusSuccess, out[:])
}

func (s *SerialDevice) ioctlSetBreak(h *Handler, req *IORequest, fd int, on bool) {
	var cmd uint
	if on {
		cmd = _TIOCSBRK
	} else {
		cmd = _TIOCCBRK
	}
	syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), uintptr(cmd), 0)
	var out [4]byte
	h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusSuccess, out[:])
}

func (s *SerialDevice) ioctlFlowControl(h *Handler, req *IORequest, fd int, xon bool) {
	var action int
	if xon {
		action = tcion
	} else {
		action = tcioff
	}
	syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), _TCXONC, uintptr(action))
	var out [4]byte
	h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusSuccess, out[:])
}

func (s *SerialDevice) ioctlPurge(h *Handler, req *IORequest, input []byte, fd int) {
	if len(input) < 4 {
		var out [4]byte
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusInvalidParameter, out[:])
		return
	}
	flags := binary.LittleEndian.Uint32(input[0:4])

	if flags&(serialPurgeRxAbort|serialPurgeRxClear) != 0 {
		syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), _TCFLSH, uintptr(tciflush))
	}
	if flags&(serialPurgeTxAbort|serialPurgeTxClear) != 0 {
		syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), _TCFLSH, uintptr(tcoflush))
	}

	var out [4]byte
	h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusSuccess, out[:])
}

func (s *SerialDevice) ioctlGetModemStatus(h *Handler, req *IORequest, fd int) {
	modem, err := ioctlGetInt(fd, _TIOCMGET)
	if err != nil {
		var out [4]byte
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusUnsuccessful, out[:])
		return
	}

	var msr uint32
	if modem&tiocmCTS != 0 {
		msr |= msrCTSOn
	}
	if modem&tiocmDSR != 0 {
		msr |= msrDSROn
	}
	if modem&tiocmRNG != 0 {
		msr |= msrRingOn
	}
	if modem&tiocmDCD != 0 {
		msr |= msrDCDOn
	}

	var out [4 + 4]byte
	binary.LittleEndian.PutUint32(out[0:4], 4)
	binary.LittleEndian.PutUint32(out[4:8], msr)
	h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusSuccess, out[:])
}

func (s *SerialDevice) ioctlGetDTRRTS(h *Handler, req *IORequest, fd int) {
	modem, err := ioctlGetInt(fd, _TIOCMGET)
	if err != nil {
		var out [4]byte
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusUnsuccessful, out[:])
		return
	}

	var flags uint32
	if modem&tiocmDTR != 0 {
		flags |= 0x01 // SERIAL_DTR_STATE
	}
	if modem&tiocmRTS != 0 {
		flags |= 0x02 // SERIAL_RTS_STATE
	}

	var out [4 + 4]byte
	binary.LittleEndian.PutUint32(out[0:4], 4)
	binary.LittleEndian.PutUint32(out[4:8], flags)
	h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusSuccess, out[:])
}

func (s *SerialDevice) ioctlGetCommStatus(h *Handler, req *IORequest, fd int) {
	// SERIAL_STATUS: 18 bytes
	// Errors(4) + HoldReasons(4) + AmountInInQueue(4) + AmountInOutQueue(4) +
	// EofReceived(1) + WaitForImmediate(1)
	const statusLen = 18
	var out [4 + statusLen]byte
	binary.LittleEndian.PutUint32(out[0:4], statusLen)

	inQueue, _ := ioctlGetInt(fd, _TIOCINQ)
	outQueue, _ := ioctlGetInt(fd, _TIOCOUTQ)
	binary.LittleEndian.PutUint32(out[12:16], uint32(inQueue))
	binary.LittleEndian.PutUint32(out[16:20], uint32(outQueue))

	h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusSuccess, out[:])
}

func (s *SerialDevice) ioctlResetDevice(h *Handler, req *IORequest, fd int) {
	resetTermios(fd)
	s.mu.Lock()
	s.waitMask = 0
	s.timeouts = [20]byte{}
	s.chars = [6]byte{}
	s.handflow = [16]byte{}
	s.mu.Unlock()
	var out [4]byte
	h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusSuccess, out[:])
}

func (s *SerialDevice) ioctlImmediateChar(h *Handler, req *IORequest, input []byte, fd int) {
	if len(input) >= 1 {
		syscall.Write(fd, input[:1])
	}
	var out [4]byte
	h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusSuccess, out[:])
}
