// Smartcard redirection device (MS-RDPESC).
// All smartcard IRPs are IRP_MJ_DEVICE_CONTROL carrying SCARD IOCTLs.
// Call/return structures use NDR type serialization (MS-RPCE section 2.2.6).
// The device dispatches each IOCTL to the ScardBackend interface.

package rdpdr

import (
	"context"
	"encoding/binary"
	"fmt"
	"log/slog"
	"sync"
	"unicode/utf16"
)

// SCARD IOCTL codes (MS-RDPESC section 3.1.4).
// Format: CTL_CODE(FILE_DEVICE_SMARTCARD=0x09, Function, METHOD_BUFFERED, FILE_ANY_ACCESS)
const (
	scardIOCTLEstablishContext uint32 = 0x00090014 // fn 5
	scardIOCTLReleaseContext   uint32 = 0x00090018 // fn 6
	scardIOCTLIsValidContext   uint32 = 0x0009001C // fn 7
	scardIOCTLListReadersA    uint32 = 0x00090028 // fn 10
	scardIOCTLListReadersW    uint32 = 0x0009002C // fn 11
	scardIOCTLGetStatusChangeA uint32 = 0x000900A0 // fn 40
	scardIOCTLGetStatusChangeW uint32 = 0x000900A4 // fn 41
	scardIOCTLCancel           uint32 = 0x000900A8 // fn 42
	scardIOCTLConnectA         uint32 = 0x000900AC // fn 43
	scardIOCTLConnectW         uint32 = 0x000900B0 // fn 44
	scardIOCTLReconnect        uint32 = 0x000900B4 // fn 45
	scardIOCTLDisconnect       uint32 = 0x000900B8 // fn 46
	scardIOCTLBeginTransaction uint32 = 0x000900BC // fn 47
	scardIOCTLEndTransaction   uint32 = 0x000900C0 // fn 48
	scardIOCTLState            uint32 = 0x000900C4 // fn 49
	scardIOCTLStatusA          uint32 = 0x000900C8 // fn 50
	scardIOCTLStatusW          uint32 = 0x000900CC // fn 51
	scardIOCTLTransmit         uint32 = 0x000900D0 // fn 52
	scardIOCTLControl          uint32 = 0x000900D4 // fn 53
	scardIOCTLGetAttrib        uint32 = 0x000900D8 // fn 54
	scardIOCTLAccessStarted    uint32 = 0x000900E0 // fn 56
	scardIOCTLGetTransmitCount uint32 = 0x00090100 // fn 64
	scardIOCTLGetReaderIcon    uint32 = 0x00090104 // fn 65
	scardIOCTLGetDeviceTypeID  uint32 = 0x00090108 // fn 66
)

// --- NDR Type Serialization helpers (MS-RPCE section 2.2.6) ---

const ndrHeaderLen = 16

// ndrCommonTypeHeader is the fixed 8-byte Common Type Header.
var ndrCommonTypeHeader = [8]byte{0x01, 0x10, 0x08, 0x00, 0xCC, 0xCC, 0xCC, 0xCC}

// ndr is a sequential reader for NDR-encoded SCARD IOCTL data.
// Fields are read in two phases: inline (fixed fields, pointer referent IDs)
// then deferred (pointed-to data, in order of pointer encounter).
type ndr struct {
	b   []byte
	off int
	err bool
}

func ndrRead(data []byte) ndr {
	if len(data) < ndrHeaderLen {
		return ndr{err: true}
	}
	return ndr{b: data, off: ndrHeaderLen}
}

func (r *ndr) ok() bool { return !r.err }

func (r *ndr) u32() uint32 {
	if r.err || r.off+4 > len(r.b) {
		r.err = true
		return 0
	}
	v := binary.LittleEndian.Uint32(r.b[r.off:])
	r.off += 4
	return v
}

func (r *ndr) skip(n int) {
	if r.err || r.off+n > len(r.b) {
		r.err = true
		return
	}
	r.off += n
}

func (r *ndr) readBytes(n uint32) []byte {
	nn := int(n)
	if r.err || r.off+nn > len(r.b) {
		r.err = true
		return nil
	}
	b := make([]byte, nn)
	copy(b, r.b[r.off:r.off+nn])
	r.off += nn
	return b
}

func (r *ndr) align4() {
	if r.err {
		return
	}
	if pad := r.off % 4; pad != 0 {
		r.off += 4 - pad
	}
}

// readContextInline reads the inline portion of REDIR_SCARDCONTEXT:
// cbContext(4) + referentID(4). Returns cbContext for deferred reading.
func (r *ndr) readContextInline() uint32 {
	cb := r.u32()
	r.u32() // referent ID (skip)
	return cb
}

// readContextDeferred reads the deferred data of REDIR_SCARDCONTEXT:
// MaximumCount(4) + pbContext(cbContext bytes).
func (r *ndr) readContextDeferred(cb uint32) uint32 {
	if cb == 0 {
		return 0
	}
	r.u32() // MaximumCount
	b := r.readBytes(cb)
	if r.err || len(b) < 4 {
		r.err = true
		return 0
	}
	return binary.LittleEndian.Uint32(b)
}

// readHandleInline reads the inline portion of REDIR_SCARDHANDLE:
// Context{cbContext(4)+refID(4)} + cbHandle(4) + refID(4).
// Returns (cbContext, cbHandle) for deferred reading.
func (r *ndr) readHandleInline() (cbCtx, cbHdl uint32) {
	cbCtx = r.readContextInline()
	cbHdl = r.u32()
	r.u32() // handle referent ID
	return
}

// readHandleDeferred reads deferred data for both context and handle.
func (r *ndr) readHandleDeferred(cbCtx, cbHdl uint32) (ctx, hdl uint32) {
	ctx = r.readContextDeferred(cbCtx)
	if cbHdl == 0 {
		return ctx, 0
	}
	r.u32() // MaximumCount
	b := r.readBytes(cbHdl)
	if r.err || len(b) < 4 {
		r.err = true
		return 0, 0
	}
	return ctx, binary.LittleEndian.Uint32(b)
}

// readWideString reads a deferred NDR conformant varying [string] wchar_t*.
func (r *ndr) readWideString() string {
	r.u32() // MaximumCount
	r.u32() // Offset (always 0)
	actCount := r.u32()
	if r.err || actCount == 0 {
		return ""
	}
	byteLen := int(actCount) * 2
	if r.off+byteLen > len(r.b) {
		r.err = true
		return ""
	}
	// Decode excluding null terminator
	decLen := int(actCount-1) * 2
	s := decodeScardUTF16LE(r.b[r.off : r.off+decLen])
	r.off += byteLen
	r.align4()
	return s
}

// readASCIIString reads a deferred NDR conformant varying [string] char*.
func (r *ndr) readASCIIString() string {
	r.u32() // MaximumCount
	r.u32() // Offset
	actCount := r.u32()
	if r.err || actCount == 0 {
		return ""
	}
	if r.off+int(actCount) > len(r.b) {
		r.err = true
		return ""
	}
	s := string(r.b[r.off : r.off+int(actCount-1)]) // exclude null
	r.off += int(actCount)
	r.align4()
	return s
}

// ndrW builds NDR type serialization responses.
// Inline and deferred data are accumulated separately, then combined in finish().
type ndrW struct {
	inline   []byte
	deferred []byte
	ref      uint32
}

func ndrNewWriter() ndrW {
	return ndrW{ref: 0x00020001}
}

func (w *ndrW) nextRef() uint32 {
	r := w.ref
	w.ref++
	return r
}

func (w *ndrW) u32(v uint32) {
	w.inline = binary.LittleEndian.AppendUint32(w.inline, v)
}

func (w *ndrW) deferU32(v uint32) {
	w.deferred = binary.LittleEndian.AppendUint32(w.deferred, v)
}

// writeContext writes a REDIR_SCARDCONTEXT (inline + deferred).
func (w *ndrW) writeContext(ctx uint32) {
	w.u32(4)           // cbContext
	w.u32(w.nextRef()) // referent ID
	w.deferU32(4)      // MaximumCount
	w.deferU32(ctx)    // data
}

// writeHandle writes a REDIR_SCARDHANDLE (Context + Handle, inline + deferred).
func (w *ndrW) writeHandle(ctx, handle uint32) {
	// Context inline
	w.u32(4)           // cbContext
	w.u32(w.nextRef()) // ctx referent ID
	// Handle inline
	w.u32(4)           // cbHandle
	w.u32(w.nextRef()) // handle referent ID
	// Context deferred
	w.deferU32(4)   // MaximumCount
	w.deferU32(ctx) // data
	// Handle deferred
	w.deferU32(4)      // MaximumCount
	w.deferU32(handle) // data
}

// writeByteArrayPtr writes a [unique][size_is] byte* (inline cBytes+refID, deferred data).
func (w *ndrW) writeByteArrayPtr(data []byte) {
	w.u32(uint32(len(data))) // cBytes
	if len(data) == 0 {
		w.u32(0) // NULL referent
		return
	}
	w.u32(w.nextRef())              // referent ID
	w.deferU32(uint32(len(data)))   // MaximumCount
	w.deferred = append(w.deferred, data...)
	// Pad to 4-byte boundary
	if pad := len(data) % 4; pad != 0 {
		w.deferred = append(w.deferred, make([]byte, 4-pad)...)
	}
}

// finish builds the complete NDR type serialization stream.
func (w *ndrW) finish() []byte {
	body := append(w.inline, w.deferred...)
	// Pad body to 8-byte alignment (ObjectBufferLength must be multiple of 8)
	if pad := len(body) % 8; pad != 0 {
		body = append(body, make([]byte, 8-pad)...)
	}
	out := make([]byte, ndrHeaderLen+len(body))
	copy(out[0:8], ndrCommonTypeHeader[:])
	binary.LittleEndian.PutUint32(out[8:12], uint32(len(body))) // ObjectBufferLength
	// out[12:16] = 0 (filler, already zero)
	copy(out[ndrHeaderLen:], body)
	return out
}

// --- SmartcardDevice ---

// SmartcardDevice represents a redirected smartcard reader.
type SmartcardDevice struct {
	id      uint32
	backend ScardBackend
	log     *slog.Logger
	mu       sync.Mutex
	contexts []uint32      // established contexts (for cancel on close)
	done     chan struct{}  // closed on Close() to unblock goroutines
}

// NewSmartcardDevice creates a new smartcard device.
func NewSmartcardDevice(id uint32, backend ScardBackend, log *slog.Logger) *SmartcardDevice {
	return &SmartcardDevice{
		id:      id,
		backend: backend,
		log:     log.With("device", "SCARD"),
		done:    make(chan struct{}),
	}
}

// Close cancels all outstanding blocking operations by issuing SCardCancel
// on every established context. This unblocks goroutines stuck in
// GetStatusChange or Connect when the RDP session is torn down abruptly.
func (s *SmartcardDevice) Close() {
	select {
	case <-s.done:
		return // already closed
	default:
		close(s.done)
	}

	s.mu.Lock()
	ctxs := make([]uint32, len(s.contexts))
	copy(ctxs, s.contexts)
	s.mu.Unlock()

	for _, ctx := range ctxs {
		s.backend.Cancel(ctx)
	}
}

func (s *SmartcardDevice) ID() uint32   { return s.id }
func (s *SmartcardDevice) Type() uint32 { return DeviceTypeSmartcard }
func (s *SmartcardDevice) Name() string { return "SCARD" }

// HandleIRP dispatches smartcard I/O requests.
func (s *SmartcardDevice) HandleIRP(h *Handler, req *IORequest) {
	if req.MajorFn != IrpDeviceControl {
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusNotSupported, nil)
		return
	}

	// Parse IOCTL header: OutputBufLen(4) + InputBufLen(4) + IoControlCode(4) + padding(20)
	if len(req.Payload) < 32 {
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusInvalidParameter, nil)
		return
	}

	outputBufLen := binary.LittleEndian.Uint32(req.Payload[0:4])
	inputBufLen := binary.LittleEndian.Uint32(req.Payload[4:8])
	ioControlCode := binary.LittleEndian.Uint32(req.Payload[8:12])
	ioctlData := req.Payload[32:]

	s.log.LogAttrs(context.Background(), slog.LevelDebug, "SCARD IOCTL",
		slog.String("code", fmt.Sprintf("0x%08X", ioControlCode)),
		slog.Int("inputLen", int(inputBufLen)),
		slog.Int("outputLen", int(outputBufLen)),
		slog.Int("dataLen", len(ioctlData)))

	switch ioControlCode {
	case scardIOCTLEstablishContext:
		s.handleEstablishContext(h, req, ioctlData)
	case scardIOCTLReleaseContext:
		s.handleReleaseContext(h, req, ioctlData)
	case scardIOCTLIsValidContext:
		s.handleIsValidContext(h, req, ioctlData)
	case scardIOCTLListReadersA:
		s.handleListReaders(h, req, ioctlData, false)
	case scardIOCTLListReadersW:
		s.handleListReaders(h, req, ioctlData, true)
	case scardIOCTLGetStatusChangeA:
		// Copy data — GetStatusChange runs in a goroutine and the caller's
		// buffer may be reused for the next packet before parsing completes.
		dataCopyA := make([]byte, len(ioctlData))
		copy(dataCopyA, ioctlData)
		go s.handleGetStatusChange(h, req, dataCopyA, false)
	case scardIOCTLGetStatusChangeW:
		dataCopyW := make([]byte, len(ioctlData))
		copy(dataCopyW, ioctlData)
		go s.handleGetStatusChange(h, req, dataCopyW, true)
	case scardIOCTLConnectA:
		dataCopyCA := make([]byte, len(ioctlData))
		copy(dataCopyCA, ioctlData)
		go s.handleConnect(h, req, dataCopyCA, false)
	case scardIOCTLConnectW:
		dataCopyCW := make([]byte, len(ioctlData))
		copy(dataCopyCW, ioctlData)
		go s.handleConnect(h, req, dataCopyCW, true)
	case scardIOCTLReconnect:
		s.handleReconnect(h, req, ioctlData)
	case scardIOCTLDisconnect:
		s.handleDisconnect(h, req, ioctlData)
	case scardIOCTLBeginTransaction:
		s.handleBeginTransaction(h, req, ioctlData)
	case scardIOCTLEndTransaction:
		s.handleEndTransaction(h, req, ioctlData)
	case scardIOCTLStatusA, scardIOCTLStatusW:
		s.handleStatus(h, req, ioctlData, ioControlCode == scardIOCTLStatusW)
	case scardIOCTLTransmit:
		s.handleTransmit(h, req, ioctlData)
	case scardIOCTLControl:
		s.handleControl(h, req, ioctlData)
	case scardIOCTLGetAttrib:
		s.handleGetAttrib(h, req, ioctlData)
	case scardIOCTLCancel:
		s.handleCancel(h, req, ioctlData)
	case scardIOCTLAccessStarted:
		s.sendScardReturn(h, req, ScardSuccess)
	default:
		s.log.LogAttrs(context.Background(), slog.LevelWarn, "unsupported SCARD IOCTL",
			slog.String("code", fmt.Sprintf("0x%08X", ioControlCode)))
		s.sendScardReturn(h, req, ScardFInternalError)
	}
}

// sendScardResponse sends an NDR-wrapped SCARD response as an IOCTL completion.
// The ndrData parameter is a complete NDR type serialization stream.
func (s *SmartcardDevice) sendScardResponse(h *Handler, req *IORequest, ndrData []byte) {
	// DR_DEVICE_IOCOMPLETION for DEVICE_CONTROL: OutputBufferLength(4) + OutputBuffer
	out := make([]byte, 4+len(ndrData))
	binary.LittleEndian.PutUint32(out[0:4], uint32(len(ndrData)))
	copy(out[4:], ndrData)
	h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusSuccess, out)
}

// sendScardReturn sends a Long_Return { ReturnCode } response.
func (s *SmartcardDevice) sendScardReturn(h *Handler, req *IORequest, rc uint32) {
	w := ndrNewWriter()
	w.u32(rc)
	s.sendScardResponse(h, req, w.finish())
}

// --- String helpers ---

func decodeScardUTF16LE(b []byte) string {
	if len(b) < 2 {
		return ""
	}
	n := len(b) / 2
	u16 := make([]uint16, n)
	for i := range n {
		u16[i] = binary.LittleEndian.Uint16(b[i*2:])
	}
	return string(utf16.Decode(u16))
}

func encodeScardUTF16LE(s string) []byte {
	runes := utf16.Encode([]rune(s))
	buf := make([]byte, (len(runes)+1)*2) // +1 for null terminator
	for i, r := range runes {
		binary.LittleEndian.PutUint16(buf[i*2:], r)
	}
	return buf
}

// asciiMultiStringToWide converts an ASCII multi-string (null-separated, double-null
// terminated) to UTF-16LE.
func asciiMultiStringToWide(b []byte) []byte {
	w := make([]byte, 0, len(b)*2)
	for _, c := range b {
		w = append(w, c, 0)
	}
	return w
}

// --- IOCTL handlers ---

// EstablishContext_Call: { dwScope }
// EstablishContext_Return: { ReturnCode, REDIR_SCARDCONTEXT }
func (s *SmartcardDevice) handleEstablishContext(h *Handler, req *IORequest, data []byte) {
	r := ndrRead(data)
	scope := r.u32()
	if !r.ok() {
		s.sendScardReturn(h, req, ScardEInvalidParameter)
		return
	}

	s.log.LogAttrs(context.Background(), slog.LevelDebug, "EstablishContext",
		slog.String("scope", fmt.Sprintf("0x%X", scope)),
		slog.Int("dataLen", len(data)))

	ctx, rc := s.backend.EstablishContext(scope)
	s.log.LogAttrs(context.Background(), slog.LevelDebug, "EstablishContext result",
		slog.String("rc", fmt.Sprintf("0x%08X", rc)),
		slog.String("ctx", fmt.Sprintf("0x%X", ctx)))
	if rc != ScardSuccess {
		s.sendScardReturn(h, req, rc)
		return
	}

	// Track context so Close() can cancel outstanding blocking calls.
	s.mu.Lock()
	s.contexts = append(s.contexts, ctx)
	s.mu.Unlock()

	w := ndrNewWriter()
	w.u32(rc)
	w.writeContext(ctx)
	s.sendScardResponse(h, req, w.finish())
}

// Context_Call: { REDIR_SCARDCONTEXT }
// Long_Return: { ReturnCode }
func (s *SmartcardDevice) handleReleaseContext(h *Handler, req *IORequest, data []byte) {
	r := ndrRead(data)
	cbCtx := r.readContextInline()
	ctx := r.readContextDeferred(cbCtx)
	if !r.ok() {
		s.sendScardReturn(h, req, ScardEInvalidParameter)
		return
	}
	rc := s.backend.ReleaseContext(ctx)

	// Remove from tracked contexts.
	s.mu.Lock()
	for i, c := range s.contexts {
		if c == ctx {
			s.contexts[i] = s.contexts[len(s.contexts)-1]
			s.contexts = s.contexts[:len(s.contexts)-1]
			break
		}
	}
	s.mu.Unlock()

	s.sendScardReturn(h, req, rc)
}

func (s *SmartcardDevice) handleIsValidContext(h *Handler, req *IORequest, data []byte) {
	r := ndrRead(data)
	cbCtx := r.readContextInline()
	ctx := r.readContextDeferred(cbCtx)
	if !r.ok() {
		s.sendScardReturn(h, req, ScardEInvalidParameter)
		return
	}
	s.sendScardReturn(h, req, s.backend.IsValidContext(ctx))
}

func (s *SmartcardDevice) handleCancel(h *Handler, req *IORequest, data []byte) {
	r := ndrRead(data)
	cbCtx := r.readContextInline()
	ctx := r.readContextDeferred(cbCtx)
	if !r.ok() {
		s.sendScardReturn(h, req, ScardEInvalidParameter)
		return
	}
	s.sendScardReturn(h, req, s.backend.Cancel(ctx))
}

// ListReaders_Call: { REDIR_SCARDCONTEXT, cBytes, *mszGroups, fmszReadersIsNULL, cchReaders }
// ListReaders_Return: { ReturnCode, cBytes, *msz }
func (s *SmartcardDevice) handleListReaders(h *Handler, req *IORequest, data []byte, wide bool) {
	r := ndrRead(data)
	// Inline
	cbCtx := r.readContextInline()
	cBytes := r.u32()
	groupsRef := r.u32()
	r.u32() // fmszReadersIsNULL
	r.u32() // cchReaders
	// Deferred
	ctx := r.readContextDeferred(cbCtx)
	var groups []byte
	if groupsRef != 0 {
		r.u32() // MaximumCount
		groups = r.readBytes(cBytes)
		r.align4()
	}
	if !r.ok() {
		s.sendScardReturn(h, req, ScardEInvalidParameter)
		return
	}

	readers, rc := s.backend.ListReaders(ctx, groups)
	if rc != ScardSuccess {
		s.sendScardReturn(h, req, rc)
		return
	}

	if wide {
		readers = asciiMultiStringToWide(readers)
	}

	w := ndrNewWriter()
	w.u32(rc)
	w.writeByteArrayPtr(readers)
	s.sendScardResponse(h, req, w.finish())
}

// GetStatusChangeW_Call: { REDIR_SCARDCONTEXT, dwTimeOut, cReaders, *rgReaderStates }
// ReaderStateW: { *szReader, REDIR_SCARDCONTEXT Common, dwCurrentState, dwEventState, cbAtr, rgbAtr[36] }
// GetStatusChange_Return: { ReturnCode, cReaders, *rgReaderStates }
// ReaderState_Return: { dwCurrentState, dwEventState, cbAtr, rgbAtr[36] }
func (s *SmartcardDevice) handleGetStatusChange(h *Handler, req *IORequest, data []byte, wide bool) {
	r := ndrRead(data)

	// Top-level inline
	cbCtx := r.readContextInline()
	timeout := r.u32()
	cReaders := r.u32()
	arrayRef := r.u32() // rgReaderStates referent ID

	// Top-level deferred: context
	ctx := r.readContextDeferred(cbCtx)

	if !r.ok() || cReaders > 11 {
		s.sendScardReturn(h, req, ScardEInvalidParameter)
		return
	}

	states := make([]ScardReaderState, cReaders)
	if arrayRef != 0 && cReaders > 0 {
		r.u32() // array MaximumCount

		// Read all elements' inline/fixed data.
		// ReaderState on the wire: szReader_ptr(4) + dwCurrentState(4) +
		//   dwEventState(4) + cbAtr(4) + rgbAtr(36) = 52 bytes.
		// Note: the hContext field from the IDL is NOT serialized on the wire.
		type elemInfo struct {
			readerRef uint32
			curState  uint32
			evtState  uint32
			cbAtr     uint32
			rgbAtr    [36]byte
		}
		elems := make([]elemInfo, cReaders)
		for i := range cReaders {
			elems[i].readerRef = r.u32() // szReader referent ID
			elems[i].curState = r.u32()
			elems[i].evtState = r.u32()
			elems[i].cbAtr = r.u32()
			b := r.readBytes(36)
			if b != nil {
				copy(elems[i].rgbAtr[:], b)
			}
		}

		// Read all elements' deferred data (reader name strings)
		for i := range cReaders {
			if elems[i].readerRef != 0 {
				if wide {
					states[i].Reader = r.readWideString()
				} else {
					states[i].Reader = r.readASCIIString()
				}
			}
			states[i].CurrentState = elems[i].curState
			states[i].EventState = elems[i].evtState
			if elems[i].cbAtr > 0 && elems[i].cbAtr <= 36 {
				states[i].ATR = make([]byte, elems[i].cbAtr)
				copy(states[i].ATR, elems[i].rgbAtr[:elems[i].cbAtr])
			}
		}
	}

	if !r.ok() {
		s.sendScardReturn(h, req, ScardEInvalidParameter)
		return
	}

	result, rc := s.backend.GetStatusChange(ctx, timeout, states)
	if rc != ScardSuccess {
		s.sendScardReturn(h, req, rc)
		return
	}

	// Build GetStatusChange_Return
	w := ndrNewWriter()
	w.u32(rc)
	w.u32(uint32(len(result))) // cReaders
	if len(result) > 0 {
		w.u32(w.nextRef()) // rgReaderStates referent ID
		// Deferred: array MaximumCount + elements
		w.deferU32(uint32(len(result)))
		for _, rs := range result {
			// ReaderState_Return: dwCurrentState(4) + dwEventState(4) + cbAtr(4) + rgbAtr[36]
			w.deferred = binary.LittleEndian.AppendUint32(w.deferred, rs.CurrentState)
			w.deferred = binary.LittleEndian.AppendUint32(w.deferred, rs.EventState)
			cbAtr := uint32(len(rs.ATR))
			if cbAtr > 36 {
				cbAtr = 36
			}
			w.deferred = binary.LittleEndian.AppendUint32(w.deferred, cbAtr)
			var atrBuf [36]byte
			copy(atrBuf[:], rs.ATR)
			w.deferred = append(w.deferred, atrBuf[:]...)
		}
	} else {
		w.u32(0) // NULL referent
	}
	s.sendScardResponse(h, req, w.finish())
}

// ConnectW_Call: { REDIR_SCARDCONTEXT, *szReader, ShareMode, PreferredProtocols }
// Connect_Return: { ReturnCode, REDIR_SCARDCONTEXT, REDIR_SCARDHANDLE, dwActiveProtocol }
func (s *SmartcardDevice) handleConnect(h *Handler, req *IORequest, data []byte, wide bool) {
	r := ndrRead(data)
	// Inline: szReader refID, then Connect_Common_Call { Context, ShareMode, PreferredProtocols }
	readerRef := r.u32() // szReader referent ID
	cbCtx := r.readContextInline()
	shareMode := r.u32()
	preferredProtocol := r.u32()
	// Deferred (pointer encounter order): szReader string, then context data
	var reader string
	if readerRef != 0 {
		if wide {
			reader = r.readWideString()
		} else {
			reader = r.readASCIIString()
		}
	}
	ctx := r.readContextDeferred(cbCtx)
	if !r.ok() {
		s.sendScardReturn(h, req, ScardEInvalidParameter)
		return
	}

	handle, activeProtocol, rc := s.backend.Connect(ctx, reader, shareMode, preferredProtocol)
	s.log.LogAttrs(context.Background(), slog.LevelDebug, "Connect",
		slog.String("rc", fmt.Sprintf("0x%08X", rc)),
		slog.String("reader", reader),
		slog.String("handle", fmt.Sprintf("0x%X", handle)),
		slog.String("activeProto", fmt.Sprintf("0x%X", activeProtocol)))
	if rc != ScardSuccess {
		s.sendScardReturn(h, req, rc)
		return
	}

	w := ndrNewWriter()
	w.u32(rc)
	w.writeHandle(ctx, handle)
	w.u32(activeProtocol)
	s.sendScardResponse(h, req, w.finish())
}

// Reconnect_Call: { REDIR_SCARDHANDLE, dwShareMode, dwPreferredProtocols, dwInitialization }
// Reconnect_Return: { ReturnCode, dwActiveProtocol }
func (s *SmartcardDevice) handleReconnect(h *Handler, req *IORequest, data []byte) {
	r := ndrRead(data)
	cbCtx, cbHdl := r.readHandleInline()
	shareMode := r.u32()
	preferredProtocol := r.u32()
	disposition := r.u32()
	_, handle := r.readHandleDeferred(cbCtx, cbHdl)
	if !r.ok() {
		s.sendScardReturn(h, req, ScardEInvalidParameter)
		return
	}

	activeProtocol, rc := s.backend.Reconnect(handle, shareMode, preferredProtocol, disposition)
	if rc != ScardSuccess {
		s.sendScardReturn(h, req, rc)
		return
	}

	w := ndrNewWriter()
	w.u32(rc)
	w.u32(activeProtocol)
	s.sendScardResponse(h, req, w.finish())
}

// HCardAndDisposition_Call: { REDIR_SCARDHANDLE, dwDisposition }
func (s *SmartcardDevice) handleDisconnect(h *Handler, req *IORequest, data []byte) {
	r := ndrRead(data)
	cbCtx, cbHdl := r.readHandleInline()
	disposition := r.u32()
	_, handle := r.readHandleDeferred(cbCtx, cbHdl)
	if !r.ok() {
		s.sendScardReturn(h, req, ScardEInvalidParameter)
		return
	}
	s.sendScardReturn(h, req, s.backend.Disconnect(handle, disposition))
}

// HCardAndDisposition_Call: { REDIR_SCARDHANDLE, dwDisposition }
// MS-RDPESC 3.1.4.30: BeginTransaction uses HCardAndDisposition_Call (disposition ignored).
func (s *SmartcardDevice) handleBeginTransaction(h *Handler, req *IORequest, data []byte) {
	r := ndrRead(data)
	cbCtx, cbHdl := r.readHandleInline()
	r.u32() // dwDisposition (ignored for BeginTransaction)
	_, handle := r.readHandleDeferred(cbCtx, cbHdl)
	if !r.ok() {
		s.sendScardReturn(h, req, ScardEInvalidParameter)
		return
	}
	rc := s.backend.BeginTransaction(handle)
	s.log.LogAttrs(context.Background(), slog.LevelDebug, "BeginTransaction",
		slog.String("handle", fmt.Sprintf("0x%X", handle)),
		slog.String("rc", fmt.Sprintf("0x%08X", rc)))
	s.sendScardReturn(h, req, rc)
}

func (s *SmartcardDevice) handleEndTransaction(h *Handler, req *IORequest, data []byte) {
	r := ndrRead(data)
	cbCtx, cbHdl := r.readHandleInline()
	disposition := r.u32()
	_, handle := r.readHandleDeferred(cbCtx, cbHdl)
	if !r.ok() {
		s.sendScardReturn(h, req, ScardEInvalidParameter)
		return
	}
	rc := s.backend.EndTransaction(handle, disposition)
	s.log.LogAttrs(context.Background(), slog.LevelDebug, "EndTransaction",
		slog.String("handle", fmt.Sprintf("0x%X", handle)),
		slog.String("disposition", fmt.Sprintf("0x%X", disposition)),
		slog.String("rc", fmt.Sprintf("0x%08X", rc)))
	s.sendScardReturn(h, req, rc)
}

// Status_Call: { REDIR_SCARDHANDLE, fmszReaderNamesIsNULL, cchReaderLen, cbAtrLen }
// Status_Return: { ReturnCode, cBytes, *mszReaderNames, dwState, dwProtocol, pbAtr[32], cbAtrLen }
func (s *SmartcardDevice) handleStatus(h *Handler, req *IORequest, data []byte, wide bool) {
	r := ndrRead(data)
	cbCtx, cbHdl := r.readHandleInline()
	r.u32() // fmszReaderNamesIsNULL
	r.u32() // cchReaderLen
	r.u32() // cbAtrLen
	_, handle := r.readHandleDeferred(cbCtx, cbHdl)
	if !r.ok() {
		s.sendScardReturn(h, req, ScardEInvalidParameter)
		return
	}

	readerName, state, protocol, atr, rc := s.backend.Status(handle)
	if rc != ScardSuccess {
		s.sendScardReturn(h, req, rc)
		return
	}

	// Encode reader name as multi-string (null-terminated + extra null)
	var nameBytes []byte
	if wide {
		nameBytes = encodeScardUTF16LE(readerName)
		nameBytes = append(nameBytes, 0, 0) // double-null terminator
	} else {
		nameBytes = append([]byte(readerName), 0, 0) // null + extra null
	}

	w := ndrNewWriter()
	w.u32(rc)
	w.writeByteArrayPtr(nameBytes) // cBytes + *mszReaderNames
	w.u32(state)
	w.u32(protocol)
	// pbAtr[32] - fixed 32-byte array
	var atrBuf [32]byte
	copy(atrBuf[:], atr)
	w.inline = append(w.inline, atrBuf[:]...)
	cbAtr := uint32(len(atr))
	if cbAtr > 32 {
		cbAtr = 32
	}
	w.u32(cbAtr)
	s.sendScardResponse(h, req, w.finish())
}

// Transmit_Call: { REDIR_SCARDHANDLE, SCardIO_Request ioSendPci, cbSendLength,
//   *pbSendBuffer, *pioRecvPci, fpbRecvBufferIsNULL, cbRecvLength }
// Transmit_Return: { ReturnCode, *pioRecvPci, cbRecvLength, *pbRecvBuffer }
func (s *SmartcardDevice) handleTransmit(h *Handler, req *IORequest, data []byte) {
	r := ndrRead(data)

	// Inline
	cbCtx, cbHdl := r.readHandleInline()
	// SCardIO_Request inline: dwProtocol(4) + cbExtraBytes(4) + pbExtraBytes refID(4)
	sendPciProtocol := r.u32()
	sendPciExtraLen := r.u32()
	sendPciExtraRef := r.u32()
	cbSendLength := r.u32()
	sendBufRef := r.u32()
	recvPciRef := r.u32()
	r.u32() // fpbRecvBufferIsNULL
	r.u32() // cbRecvLength

	// Deferred (in pointer order)
	_, handle := r.readHandleDeferred(cbCtx, cbHdl)

	// ioSendPci extra bytes
	if sendPciExtraRef != 0 {
		r.u32() // MaximumCount
		r.skip(int(sendPciExtraLen))
		r.align4()
	}

	// pbSendBuffer
	var sendBuf []byte
	if sendBufRef != 0 {
		r.u32() // MaximumCount
		sendBuf = r.readBytes(cbSendLength)
		r.align4()
	}

	// pioRecvPci (skip if present)
	if recvPciRef != 0 {
		r.u32() // dwProtocol
		extraLen := r.u32()
		extraRef := r.u32()
		if extraRef != 0 {
			r.u32() // MaximumCount
			r.skip(int(extraLen))
			r.align4()
		}
	}

	if !r.ok() {
		s.sendScardReturn(h, req, ScardEInvalidParameter)
		return
	}

	// Build sendPCI: just the protocol (8 bytes: dwProtocol + cbPciLength)
	sendPCI := make([]byte, 8)
	binary.LittleEndian.PutUint32(sendPCI[0:4], sendPciProtocol)
	binary.LittleEndian.PutUint32(sendPCI[4:8], 8)

	recvPCI, recvBuf, rc := s.backend.Transmit(handle, sendPCI, sendBuf)
	if rc != ScardSuccess {
		s.log.LogAttrs(context.Background(), slog.LevelWarn, "Transmit failed",
			slog.String("handle", fmt.Sprintf("0x%X", handle)),
			slog.String("rc", fmt.Sprintf("0x%08X", rc)))
		s.sendScardReturn(h, req, rc)
		return
	}

	// Build Transmit_Return
	w := ndrNewWriter()
	w.u32(rc)
	// pioRecvPci: [unique] pointer
	if len(recvPCI) >= 8 {
		w.u32(w.nextRef()) // non-NULL
		// Deferred: SCardIO_Request { dwProtocol, cbExtraBytes, *pbExtraBytes }
		w.deferU32(binary.LittleEndian.Uint32(recvPCI[0:4])) // dwProtocol
		w.deferU32(0)                                          // cbExtraBytes
		w.deferU32(0)                                          // NULL pbExtraBytes
	} else {
		w.u32(0) // NULL pioRecvPci
	}
	// cbRecvLength + *pbRecvBuffer
	w.u32(uint32(len(recvBuf)))
	if len(recvBuf) > 0 {
		w.u32(w.nextRef())
		w.deferU32(uint32(len(recvBuf))) // MaximumCount
		w.deferred = append(w.deferred, recvBuf...)
		if pad := len(recvBuf) % 4; pad != 0 {
			w.deferred = append(w.deferred, make([]byte, 4-pad)...)
		}
	} else {
		w.u32(0) // NULL
	}
	s.sendScardResponse(h, req, w.finish())
}

// Control_Call: { REDIR_SCARDHANDLE, dwControlCode, cbInBufferSize, *pvInBuffer,
//   fpvOutBufferIsNULL, cbOutBufferSize }
// Control_Return: { ReturnCode, cbOutBufferSize, *pvOutBuffer }
func (s *SmartcardDevice) handleControl(h *Handler, req *IORequest, data []byte) {
	r := ndrRead(data)
	cbCtx, cbHdl := r.readHandleInline()
	controlCode := r.u32()
	cbIn := r.u32()
	inBufRef := r.u32()
	r.u32() // fpvOutBufferIsNULL
	r.u32() // cbOutBufferSize
	_, handle := r.readHandleDeferred(cbCtx, cbHdl)
	var inBuf []byte
	if inBufRef != 0 {
		r.u32() // MaximumCount
		inBuf = r.readBytes(cbIn)
		r.align4()
	}
	if !r.ok() {
		s.sendScardReturn(h, req, ScardEInvalidParameter)
		return
	}

	outBuf, rc := s.backend.Control(handle, controlCode, inBuf)
	if rc != ScardSuccess {
		s.sendScardReturn(h, req, rc)
		return
	}

	w := ndrNewWriter()
	w.u32(rc)
	w.writeByteArrayPtr(outBuf)
	s.sendScardResponse(h, req, w.finish())
}

// GetAttrib_Call: { REDIR_SCARDHANDLE, dwAttrId, fpbAttrIsNULL, cbAttrLen }
// GetAttrib_Return: { ReturnCode, cbAttrLen, *pbAttr }
func (s *SmartcardDevice) handleGetAttrib(h *Handler, req *IORequest, data []byte) {
	r := ndrRead(data)
	cbCtx, cbHdl := r.readHandleInline()
	attrID := r.u32()
	r.u32() // fpbAttrIsNULL
	r.u32() // cbAttrLen
	_, handle := r.readHandleDeferred(cbCtx, cbHdl)
	if !r.ok() {
		s.sendScardReturn(h, req, ScardEInvalidParameter)
		return
	}

	attr, rc := s.backend.GetAttrib(handle, attrID)
	if rc != ScardSuccess {
		s.sendScardReturn(h, req, rc)
		return
	}

	w := ndrNewWriter()
	w.u32(rc)
	w.writeByteArrayPtr(attr)
	s.sendScardResponse(h, req, w.finish())
}

// readRedirContext and writeRedirContext are kept for test compatibility
// but now use the NDR-based encoding.
func readRedirContext(data []byte) (ctx uint32, rest []byte, ok bool) {
	if len(data) < 8 {
		return 0, data, false
	}
	cbCtx := binary.LittleEndian.Uint32(data[0:4])
	if cbCtx == 0 {
		return 0, data[8:], true
	}
	if len(data) < 8+int(cbCtx) {
		return 0, data, false
	}
	ctx = binary.LittleEndian.Uint32(data[8 : 8+4])
	return ctx, data[8+cbCtx:], true
}

func writeRedirContext(buf []byte, ctx uint32) []byte {
	b := make([]byte, 12)
	binary.LittleEndian.PutUint32(b[0:4], 4)    // cbContext
	binary.LittleEndian.PutUint32(b[4:8], ctx)   // direct value (for simple encoding)
	return append(buf, b[:8]...)
}

func readRedirHandle(data []byte) (handle uint32, rest []byte, ok bool) {
	if len(data) < 8 {
		return 0, data, false
	}
	cbHandle := binary.LittleEndian.Uint32(data[0:4])
	if cbHandle == 0 {
		return 0, data[8:], true
	}
	if len(data) < 8+int(cbHandle) {
		return 0, data, false
	}
	handle = binary.LittleEndian.Uint32(data[8 : 8+4])
	return handle, data[8+cbHandle:], true
}

func writeRedirHandle(buf []byte, handle uint32) []byte {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint32(b[0:4], 4)      // cbHandle
	binary.LittleEndian.PutUint32(b[4:8], handle)
	return append(buf, b...)
}
