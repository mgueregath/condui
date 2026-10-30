//go:build windows

// WinSCard backend for smartcard redirection on Windows.
// Calls winscard.dll directly via syscall.
//
// Windows SCARDCONTEXT and SCARDHANDLE are ULONG_PTR (8 bytes on 64-bit),
// but the ScardBackend interface uses uint32 (matching the NDR wire format).
// We maintain bidirectional maps (uintptr↔uint32) so no truncation occurs.

package rdpdr

import (
	"sync"
	"sync/atomic"
	"syscall"
	"unsafe"
)

var (
	winscard                    = syscall.NewLazyDLL("winscard.dll")
	procEstablishContext        = winscard.NewProc("SCardEstablishContext")
	procReleaseContext          = winscard.NewProc("SCardReleaseContext")
	procIsValidContext          = winscard.NewProc("SCardIsValidContext")
	procListReadersA            = winscard.NewProc("SCardListReadersA")
	procGetStatusChangeW        = winscard.NewProc("SCardGetStatusChangeW")
	procConnectW                = winscard.NewProc("SCardConnectW")
	procDisconnect              = winscard.NewProc("SCardDisconnect")
	procReconnect               = winscard.NewProc("SCardReconnect")
	procBeginTransaction        = winscard.NewProc("SCardBeginTransaction")
	procEndTransaction          = winscard.NewProc("SCardEndTransaction")
	procStatusW                 = winscard.NewProc("SCardStatusW")
	procTransmit                = winscard.NewProc("SCardTransmit")
	procControl                 = winscard.NewProc("SCardControl")
	procGetAttrib               = winscard.NewProc("SCardGetAttrib")
	procCancel                  = winscard.NewProc("SCardCancel")
)

const maxATRSize = 36

// scardIORequest matches the Windows SCARD_IO_REQUEST structure.
type scardIORequest struct {
	Protocol  uint32
	PciLength uint32
}

// scardReaderStateW matches the Windows SCARD_READERSTATEW structure.
type scardReaderStateW struct {
	Reader       *uint16
	UserData     uintptr
	CurrentState uint32
	EventState   uint32
	AtrLen       uint32
	Atr          [maxATRSize]byte
}

// handleMap maps synthetic uint32 IDs to real Windows ULONG_PTR handles
// and back. This avoids truncating 64-bit handles to 32 bits.
type handleMap struct {
	mu      sync.RWMutex
	nextID  atomic.Uint32
	toNat   map[uint32]uintptr  // synthetic → native
	fromNat map[uintptr]uint32  // native → synthetic
}

func newHandleMap() *handleMap {
	m := &handleMap{
		toNat:   make(map[uint32]uintptr),
		fromNat: make(map[uintptr]uint32),
	}
	m.nextID.Store(1)
	return m
}

func (m *handleMap) add(native uintptr) uint32 {
	m.mu.Lock()
	defer m.mu.Unlock()
	if id, ok := m.fromNat[native]; ok {
		return id
	}
	id := m.nextID.Add(1) - 1
	m.toNat[id] = native
	m.fromNat[native] = id
	return id
}

func (m *handleMap) get(id uint32) (uintptr, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	n, ok := m.toNat[id]
	return n, ok
}

func (m *handleMap) remove(id uint32) uintptr {
	m.mu.Lock()
	defer m.mu.Unlock()
	n, ok := m.toNat[id]
	if ok {
		delete(m.toNat, id)
		delete(m.fromNat, n)
	}
	return n
}

func (m *handleMap) removeAll() []uintptr {
	m.mu.Lock()
	defer m.mu.Unlock()
	all := make([]uintptr, 0, len(m.toNat))
	for _, n := range m.toNat {
		all = append(all, n)
	}
	m.toNat = make(map[uint32]uintptr)
	m.fromNat = make(map[uintptr]uint32)
	return all
}

// WinSCardBackend implements ScardBackend using the Windows WinSCard API.
type WinSCardBackend struct {
	contexts *handleMap // SCARDCONTEXT mapping
	handles  *handleMap // SCARDHANDLE mapping
}

// NewWinSCardBackend creates a WinSCard backend.
// The socketPath parameter is ignored on Windows.
func NewWinSCardBackend(_ string) (*WinSCardBackend, error) {
	if err := winscard.Load(); err != nil {
		return nil, err
	}
	return &WinSCardBackend{
		contexts: newHandleMap(),
		handles:  newHandleMap(),
	}, nil
}

func (w *WinSCardBackend) EstablishContext(scope uint32) (uint32, uint32) {
	var ctx uintptr
	r, _, _ := procEstablishContext.Call(
		uintptr(scope),
		0, // reserved
		0, // reserved
		uintptr(unsafe.Pointer(&ctx)),
	)
	rc := uint32(r)
	if rc != ScardSuccess {
		return 0, rc
	}
	return w.contexts.add(ctx), ScardSuccess
}

func (w *WinSCardBackend) ReleaseContext(ctx uint32) uint32 {
	native := w.contexts.remove(ctx)
	r, _, _ := procReleaseContext.Call(native)
	return uint32(r)
}

func (w *WinSCardBackend) IsValidContext(ctx uint32) uint32 {
	native, ok := w.contexts.get(ctx)
	if !ok {
		return ScardEInvalidHandle
	}
	r, _, _ := procIsValidContext.Call(native)
	return uint32(r)
}

func (w *WinSCardBackend) ListReaders(ctx uint32, groups []byte) ([]byte, uint32) {
	native, ok := w.contexts.get(ctx)
	if !ok {
		return nil, ScardEInvalidHandle
	}

	var groupsPtr unsafe.Pointer
	if len(groups) > 0 {
		groupsPtr = unsafe.Pointer(&groups[0])
	}

	// Use SCardListReadersA — returns ASCII multi-string matching pcsclite contract.
	var cchReaders uint32
	r, _, _ := procListReadersA.Call(
		native,
		uintptr(groupsPtr),
		0, // null pointer → get size
		uintptr(unsafe.Pointer(&cchReaders)),
	)
	rc := uint32(r)
	if rc != ScardSuccess {
		return nil, rc
	}

	buf := make([]byte, cchReaders)
	r, _, _ = procListReadersA.Call(
		native,
		uintptr(groupsPtr),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&cchReaders)),
	)
	rc = uint32(r)
	if rc != ScardSuccess {
		return nil, rc
	}
	return buf[:cchReaders], ScardSuccess
}

func (w *WinSCardBackend) GetStatusChange(ctx uint32, timeout uint32, states []ScardReaderState) ([]ScardReaderState, uint32) {
	if len(states) == 0 {
		return states, ScardSuccess
	}

	native, ok := w.contexts.get(ctx)
	if !ok {
		return nil, ScardEInvalidHandle
	}

	// Convert to Windows structures
	winStates := make([]scardReaderStateW, len(states))
	readerPtrs := make([]*uint16, len(states)) // prevent GC
	for i, s := range states {
		ptr, _ := syscall.UTF16PtrFromString(s.Reader)
		readerPtrs[i] = ptr
		winStates[i].Reader = ptr
		winStates[i].CurrentState = s.CurrentState
		winStates[i].EventState = s.EventState
		if len(s.ATR) > 0 {
			n := copy(winStates[i].Atr[:], s.ATR)
			winStates[i].AtrLen = uint32(n)
		}
	}

	r, _, _ := procGetStatusChangeW.Call(
		native,
		uintptr(timeout),
		uintptr(unsafe.Pointer(&winStates[0])),
		uintptr(len(winStates)),
	)
	rc := uint32(r)
	if rc != ScardSuccess {
		return nil, rc
	}

	// Convert back
	result := make([]ScardReaderState, len(states))
	for i := range winStates {
		result[i].Reader = states[i].Reader
		result[i].CurrentState = winStates[i].CurrentState
		result[i].EventState = winStates[i].EventState
		if winStates[i].AtrLen > 0 {
			result[i].ATR = make([]byte, winStates[i].AtrLen)
			copy(result[i].ATR, winStates[i].Atr[:winStates[i].AtrLen])
		}
	}
	return result, ScardSuccess
}

func (w *WinSCardBackend) Connect(ctx uint32, reader string, shareMode, preferredProtocol uint32) (uint32, uint32, uint32) {
	nativeCtx, ok := w.contexts.get(ctx)
	if !ok {
		return 0, 0, ScardEInvalidHandle
	}

	readerPtr, _ := syscall.UTF16PtrFromString(reader)
	var handle uintptr
	var activeProtocol uint32

	r, _, _ := procConnectW.Call(
		nativeCtx,
		uintptr(unsafe.Pointer(readerPtr)),
		uintptr(shareMode),
		uintptr(preferredProtocol),
		uintptr(unsafe.Pointer(&handle)),
		uintptr(unsafe.Pointer(&activeProtocol)),
	)
	rc := uint32(r)
	if rc != ScardSuccess {
		return 0, 0, rc
	}
	return w.handles.add(handle), activeProtocol, ScardSuccess
}

func (w *WinSCardBackend) Disconnect(handle uint32, disposition uint32) uint32 {
	native := w.handles.remove(handle)
	r, _, _ := procDisconnect.Call(native, uintptr(disposition))
	return uint32(r)
}

func (w *WinSCardBackend) Reconnect(handle uint32, shareMode, preferredProtocol, disposition uint32) (uint32, uint32) {
	native, ok := w.handles.get(handle)
	if !ok {
		return 0, ScardEInvalidHandle
	}

	var activeProtocol uint32
	r, _, _ := procReconnect.Call(
		native,
		uintptr(shareMode),
		uintptr(preferredProtocol),
		uintptr(disposition),
		uintptr(unsafe.Pointer(&activeProtocol)),
	)
	rc := uint32(r)
	if rc != ScardSuccess {
		return 0, rc
	}
	return activeProtocol, ScardSuccess
}

func (w *WinSCardBackend) BeginTransaction(handle uint32) uint32 {
	native, ok := w.handles.get(handle)
	if !ok {
		return ScardEInvalidHandle
	}
	r, _, _ := procBeginTransaction.Call(native)
	return uint32(r)
}

func (w *WinSCardBackend) EndTransaction(handle uint32, disposition uint32) uint32 {
	native, ok := w.handles.get(handle)
	if !ok {
		return ScardEInvalidHandle
	}
	r, _, _ := procEndTransaction.Call(native, uintptr(disposition))
	return uint32(r)
}

func (w *WinSCardBackend) Status(handle uint32) (string, uint32, uint32, []byte, uint32) {
	native, ok := w.handles.get(handle)
	if !ok {
		return "", 0, 0, nil, ScardEInvalidHandle
	}

	// First call: get required buffer sizes
	var cchReaderLen uint32
	var state, protocol uint32
	var atr [maxATRSize]byte
	atrLen := uint32(maxATRSize)

	r, _, _ := procStatusW.Call(
		native,
		0, // null → get size
		uintptr(unsafe.Pointer(&cchReaderLen)),
		uintptr(unsafe.Pointer(&state)),
		uintptr(unsafe.Pointer(&protocol)),
		uintptr(unsafe.Pointer(&atr[0])),
		uintptr(unsafe.Pointer(&atrLen)),
	)
	rc := uint32(r)
	if rc != ScardSuccess && rc != ScardEInsufficientBuffer {
		return "", 0, 0, nil, rc
	}

	// Allocate reader name buffer and call again
	if cchReaderLen == 0 {
		cchReaderLen = 256
	}
	readerBuf := make([]uint16, cchReaderLen)
	atrLen = maxATRSize

	r, _, _ = procStatusW.Call(
		native,
		uintptr(unsafe.Pointer(&readerBuf[0])),
		uintptr(unsafe.Pointer(&cchReaderLen)),
		uintptr(unsafe.Pointer(&state)),
		uintptr(unsafe.Pointer(&protocol)),
		uintptr(unsafe.Pointer(&atr[0])),
		uintptr(unsafe.Pointer(&atrLen)),
	)
	rc = uint32(r)
	if rc != ScardSuccess {
		return "", 0, 0, nil, rc
	}

	reader := syscall.UTF16ToString(readerBuf[:cchReaderLen])
	var atrSlice []byte
	if atrLen > 0 {
		atrSlice = make([]byte, atrLen)
		copy(atrSlice, atr[:atrLen])
	}
	return reader, state, protocol, atrSlice, ScardSuccess
}

func (w *WinSCardBackend) Transmit(handle uint32, sendPCI, sendBuf []byte) ([]byte, []byte, uint32) {
	native, ok := w.handles.get(handle)
	if !ok {
		return nil, nil, ScardEInvalidHandle
	}

	// Parse send PCI: protocol(4) + length(4)
	var ioSend scardIORequest
	if len(sendPCI) >= 8 {
		ioSend.Protocol = uint32(sendPCI[0]) | uint32(sendPCI[1])<<8 | uint32(sendPCI[2])<<16 | uint32(sendPCI[3])<<24
		ioSend.PciLength = uint32(unsafe.Sizeof(ioSend))
	} else {
		// Default to T1
		ioSend.Protocol = ScardProtocolT1
		ioSend.PciLength = uint32(unsafe.Sizeof(ioSend))
	}

	var ioRecv scardIORequest
	recvBuf := make([]byte, 65536)
	recvLen := uint32(len(recvBuf))

	r, _, _ := procTransmit.Call(
		native,
		uintptr(unsafe.Pointer(&ioSend)),
		uintptr(unsafe.Pointer(&sendBuf[0])),
		uintptr(len(sendBuf)),
		uintptr(unsafe.Pointer(&ioRecv)),
		uintptr(unsafe.Pointer(&recvBuf[0])),
		uintptr(unsafe.Pointer(&recvLen)),
	)
	rc := uint32(r)
	if rc != ScardSuccess {
		return nil, nil, rc
	}

	// Encode recv PCI
	recvPCI := make([]byte, 8)
	recvPCI[0] = byte(ioRecv.Protocol)
	recvPCI[1] = byte(ioRecv.Protocol >> 8)
	recvPCI[2] = byte(ioRecv.Protocol >> 16)
	recvPCI[3] = byte(ioRecv.Protocol >> 24)
	recvPCI[4] = byte(ioRecv.PciLength)
	recvPCI[5] = byte(ioRecv.PciLength >> 8)
	recvPCI[6] = byte(ioRecv.PciLength >> 16)
	recvPCI[7] = byte(ioRecv.PciLength >> 24)

	return recvPCI, recvBuf[:recvLen], ScardSuccess
}

func (w *WinSCardBackend) Control(handle uint32, controlCode uint32, inBuf []byte) ([]byte, uint32) {
	native, ok := w.handles.get(handle)
	if !ok {
		return nil, ScardEInvalidHandle
	}

	outBuf := make([]byte, 65536)
	var bytesReturned uint32

	var inPtr unsafe.Pointer
	if len(inBuf) > 0 {
		inPtr = unsafe.Pointer(&inBuf[0])
	}

	r, _, _ := procControl.Call(
		native,
		uintptr(controlCode),
		uintptr(inPtr),
		uintptr(len(inBuf)),
		uintptr(unsafe.Pointer(&outBuf[0])),
		uintptr(len(outBuf)),
		uintptr(unsafe.Pointer(&bytesReturned)),
	)
	rc := uint32(r)
	if rc != ScardSuccess {
		return nil, rc
	}
	return outBuf[:bytesReturned], ScardSuccess
}

func (w *WinSCardBackend) GetAttrib(handle uint32, attrID uint32) ([]byte, uint32) {
	native, ok := w.handles.get(handle)
	if !ok {
		return nil, ScardEInvalidHandle
	}

	// First call: get size
	var attrLen uint32
	r, _, _ := procGetAttrib.Call(
		native,
		uintptr(attrID),
		0,
		uintptr(unsafe.Pointer(&attrLen)),
	)
	rc := uint32(r)
	if rc != ScardSuccess && rc != ScardEInsufficientBuffer {
		return nil, rc
	}
	if attrLen == 0 {
		return nil, ScardSuccess
	}

	buf := make([]byte, attrLen)
	r, _, _ = procGetAttrib.Call(
		native,
		uintptr(attrID),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&attrLen)),
	)
	rc = uint32(r)
	if rc != ScardSuccess {
		return nil, rc
	}
	return buf[:attrLen], ScardSuccess
}

func (w *WinSCardBackend) Cancel(ctx uint32) uint32 {
	native, ok := w.contexts.get(ctx)
	if !ok {
		return ScardEInvalidHandle
	}
	r, _, _ := procCancel.Call(native)
	return uint32(r)
}

func (w *WinSCardBackend) Close() error {
	// Release all tracked contexts
	for _, native := range w.contexts.removeAll() {
		procReleaseContext.Call(native)
	}
	w.handles.removeAll()
	return nil
}
