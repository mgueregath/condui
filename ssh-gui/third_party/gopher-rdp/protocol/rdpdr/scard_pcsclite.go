//go:build linux || darwin

// pcsclite backend for smartcard redirection.
// Communicates with pcscd via its Unix socket protocol.
//
// Protocol: client sends rxHeader{size(4), command(4)} + struct payload.
// Server responds with the same struct (filled in), NO header.
// Some commands (Transmit, Control) exchange extra data buffers after the struct.
// Reader enumeration uses CMD_GET_READERS_STATE which returns READER_STATE[16].

package rdpdr

import (
	"encoding/binary"
	"fmt"
	"net"
	"strings"
	"sync"
)

// pcsclite command codes (pcsclite source: winscard_msg.h)
const (
	pcscCmdEstablishContext             uint32 = 0x01
	pcscCmdReleaseContext               uint32 = 0x02
	pcscCmdConnect                      uint32 = 0x04
	pcscCmdReconnect                    uint32 = 0x05
	pcscCmdDisconnect                   uint32 = 0x06
	pcscCmdBeginTransaction             uint32 = 0x07
	pcscCmdEndTransaction               uint32 = 0x08
	pcscCmdTransmit                     uint32 = 0x09
	pcscCmdControl                      uint32 = 0x0A
	pcscCmdStatus                       uint32 = 0x0B
	pcscCmdCancel                       uint32 = 0x0D
	pcscCmdGetAttrib                    uint32 = 0x0E
	pcscCmdGetVersion                   uint32 = 0x11
	pcscCmdGetReadersState              uint32 = 0x12
	pcscCmdWaitReaderStateChange        uint32 = 0x13
	pcscCmdStopWaitingReaderStateChange uint32 = 0x14
)

// pcsclite protocol version
const pcscProtocolVersionMajor uint32 = 4

// pcsclite struct sizes (all fields including rv and output fields)
const (
	pcscMaxReaderName    = 128
	pcscMaxATRSize       = 33
	pcscMaxBufferSizeExt = 131080 // MAX_BUFFER_SIZE_EXTENDED (protocol v4+)

	// READER_STATE: readerName(128) + eventCounter(4) + readerState(4) +
	//   readerSharing(4) + cardAtr(33) + pad(3) + cardAtrLength(4) + cardProtocol(4) = 184
	pcscReaderStateSize = 184
	pcscMaxReaders      = 16 // PCSCLITE_MAX_READERS_CONTEXTS
)

// Internal pcsclite reader state flags (eventhandler.h) — different from public SCARD_STATE!
const (
	pcscStateUnknown    uint32 = 0x0001
	pcscStateAbsent     uint32 = 0x0002
	pcscStatePresent    uint32 = 0x0004
	pcscStateSwallowed  uint32 = 0x0008
	pcscStatePowered    uint32 = 0x0010
	pcscStateNegotiable uint32 = 0x0020
	pcscStateSpecific   uint32 = 0x0040
)

// pcscContext holds a pcscd socket connection and its mutex.
type pcscContext struct {
	conn net.Conn
	mu   sync.Mutex
}

// PCSCLiteBackend implements ScardBackend by communicating with pcscd.
type PCSCLiteBackend struct {
	socketPath string

	mu         sync.Mutex
	contexts   map[uint32]*pcscContext // pcscd context handle → connection
	cardCtx    map[uint32]uint32      // card handle → context handle
	cardReader map[uint32]string      // card handle → reader name (for Status)
}

// NewPCSCLiteBackend creates a pcsclite backend.
func NewPCSCLiteBackend(socketPath string) (*PCSCLiteBackend, error) {
	if socketPath == "" {
		socketPath = "/var/run/pcscd/pcscd.comm"
	}
	return &PCSCLiteBackend{
		socketPath: socketPath,
		contexts:   make(map[uint32]*pcscContext),
		cardCtx:    make(map[uint32]uint32),
		cardReader: make(map[uint32]string),
	}, nil
}

func (p *PCSCLiteBackend) dial() (net.Conn, error) {
	return net.Dial("unix", p.socketPath)
}

// sendRecv sends a command with fixed-size struct payload, then reads the response
// back into the same buffer. The pcsclite protocol: client sends rxHeader(8) + struct,
// server responds with just the struct (same size, no header).
func sendRecv(conn net.Conn, cmd uint32, buf []byte) error {
	// Send header: size(4) + command(4)
	var hdr [8]byte
	binary.LittleEndian.PutUint32(hdr[0:4], uint32(len(buf)))
	binary.LittleEndian.PutUint32(hdr[4:8], cmd)
	if _, err := conn.Write(hdr[:]); err != nil {
		return fmt.Errorf("pcsclite write header: %w", err)
	}
	if len(buf) > 0 {
		if _, err := conn.Write(buf); err != nil {
			return fmt.Errorf("pcsclite write payload: %w", err)
		}
	}
	// Read response: struct only (same size), no header
	if len(buf) > 0 {
		if _, err := readFull(conn, buf); err != nil {
			return fmt.Errorf("pcsclite read response: %w", err)
		}
	}
	return nil
}

// sendCommandOnly sends header + payload without reading a response.
func sendCommandOnly(conn net.Conn, cmd uint32, buf []byte) error {
	var hdr [8]byte
	binary.LittleEndian.PutUint32(hdr[0:4], uint32(len(buf)))
	binary.LittleEndian.PutUint32(hdr[4:8], cmd)
	if _, err := conn.Write(hdr[:]); err != nil {
		return err
	}
	if len(buf) > 0 {
		if _, err := conn.Write(buf); err != nil {
			return err
		}
	}
	return nil
}

func readFull(conn net.Conn, buf []byte) (int, error) {
	total := 0
	for total < len(buf) {
		n, err := conn.Read(buf[total:])
		total += n
		if err != nil {
			return total, err
		}
	}
	return total, nil
}

func (p *PCSCLiteBackend) getCtx(ctx uint32) (*pcscContext, bool) {
	p.mu.Lock()
	pc, ok := p.contexts[ctx]
	p.mu.Unlock()
	return pc, ok
}

func (p *PCSCLiteBackend) getCtxForCard(handle uint32) (*pcscContext, bool) {
	p.mu.Lock()
	ctx, ok := p.cardCtx[handle]
	if !ok {
		p.mu.Unlock()
		return nil, false
	}
	pc, ok := p.contexts[ctx]
	p.mu.Unlock()
	return pc, ok
}

// negotiateVersion sends CMD_GET_VERSION and checks compatibility.
func negotiateVersion(conn net.Conn) error {
	// version_struct: major(4) + minor(4) + rv(4) = 12 bytes
	var buf [12]byte
	binary.LittleEndian.PutUint32(buf[0:4], pcscProtocolVersionMajor)
	binary.LittleEndian.PutUint32(buf[4:8], 0) // minor
	// buf[8:12] = rv = 0
	if err := sendRecv(conn, pcscCmdGetVersion, buf[:]); err != nil {
		return err
	}
	// rv is informational — pcscd may return non-zero (e.g. SCARD_E_SERVICE_STOPPED)
	// even when the protocol works fine. Only check that the server speaks v4+.
	major := binary.LittleEndian.Uint32(buf[0:4])
	if major < pcscProtocolVersionMajor {
		return fmt.Errorf("pcsclite protocol version %d too old (need %d+)", major, pcscProtocolVersionMajor)
	}
	return nil
}

// getReadersState sends CMD_GET_READERS_STATE and returns the READER_STATE array.
func getReadersState(conn net.Conn) ([]byte, error) {
	// CMD_GET_READERS_STATE: send header with size=0, no payload.
	// Response: READER_STATE[16] = 16 * 184 = 2944 bytes, no header.
	if err := sendCommandOnly(conn, pcscCmdGetReadersState, nil); err != nil {
		return nil, err
	}
	resp := make([]byte, pcscReaderStateSize*pcscMaxReaders)
	if _, err := readFull(conn, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// parseReaderState extracts fields from a single 184-byte READER_STATE.
type readerStateInfo struct {
	Name         string
	EventCounter uint32
	State        uint32 // internal pcsclite state flags
	Sharing      int32
	ATR          []byte
	Protocol     uint32
}

func parseReaderState(data []byte) readerStateInfo {
	var rs readerStateInfo
	if len(data) < pcscReaderStateSize {
		return rs
	}
	rs.Name = cString(data[0:pcscMaxReaderName])
	rs.EventCounter = binary.LittleEndian.Uint32(data[128:132])
	rs.State = binary.LittleEndian.Uint32(data[132:136])
	rs.Sharing = int32(binary.LittleEndian.Uint32(data[136:140]))
	atrLen := binary.LittleEndian.Uint32(data[176:180])
	if atrLen > 0 && atrLen <= pcscMaxATRSize {
		rs.ATR = make([]byte, atrLen)
		copy(rs.ATR, data[140:140+atrLen])
	}
	rs.Protocol = binary.LittleEndian.Uint32(data[180:184])
	return rs
}

// SCardStatus dwState values (MS-RDPESC 2.2.3.9) — sequential, NOT bitmasks.
const (
	scardUnknown    uint32 = 0
	scardAbsent     uint32 = 1
	scardPresent    uint32 = 2
	scardSwallowed  uint32 = 3
	scardPowered    uint32 = 4
	scardNegotiable uint32 = 5
	scardSpecific   uint32 = 6
)

// mapInternalToCardState converts pcsclite internal state to SCardStatus dwState.
// These are sequential 0-6 values, different from SCARD_STATE_* bitmask flags.
func mapInternalToCardState(rs readerStateInfo) uint32 {
	switch {
	case rs.State&pcscStateSpecific != 0:
		return scardSpecific
	case rs.State&pcscStateNegotiable != 0:
		return scardNegotiable
	case rs.State&pcscStatePowered != 0:
		return scardPowered
	case rs.State&pcscStateSwallowed != 0:
		return scardSwallowed
	case rs.State&pcscStatePresent != 0:
		return scardPresent
	case rs.State&pcscStateAbsent != 0:
		return scardAbsent
	default:
		return scardUnknown
	}
}

func (p *PCSCLiteBackend) EstablishContext(scope uint32) (uint32, uint32) {
	conn, err := p.dial()
	if err != nil {
		return 0, ScardENoService
	}

	if err := negotiateVersion(conn); err != nil {
		conn.Close()
		return 0, ScardENoService
	}

	// establish_struct: scope(4) + hContext(4) + rv(4) = 12 bytes
	var buf [12]byte
	binary.LittleEndian.PutUint32(buf[0:4], scope)
	// buf[4:8] = hContext = 0 (output)
	// buf[8:12] = rv = 0
	if err := sendRecv(conn, pcscCmdEstablishContext, buf[:]); err != nil {
		conn.Close()
		return 0, ScardFCommError
	}

	hContext := binary.LittleEndian.Uint32(buf[4:8])
	rv := binary.LittleEndian.Uint32(buf[8:12])
	if rv != ScardSuccess {
		conn.Close()
		return 0, rv
	}

	p.mu.Lock()
	p.contexts[hContext] = &pcscContext{conn: conn}
	p.mu.Unlock()

	return hContext, ScardSuccess
}

func (p *PCSCLiteBackend) ReleaseContext(ctx uint32) uint32 {
	p.mu.Lock()
	pc, ok := p.contexts[ctx]
	delete(p.contexts, ctx)
	// Clean up any card handles for this context
	for h, c := range p.cardCtx {
		if c == ctx {
			delete(p.cardCtx, h)
			delete(p.cardReader, h)
		}
	}
	p.mu.Unlock()

	if !ok {
		return ScardEInvalidHandle
	}

	pc.mu.Lock()
	defer pc.mu.Unlock()

	// release_struct: hContext(4) + rv(4) = 8 bytes
	var buf [8]byte
	binary.LittleEndian.PutUint32(buf[0:4], ctx)
	err := sendRecv(pc.conn, pcscCmdReleaseContext, buf[:])
	pc.conn.Close()
	if err != nil {
		return ScardFCommError
	}
	return binary.LittleEndian.Uint32(buf[4:8])
}

func (p *PCSCLiteBackend) IsValidContext(ctx uint32) uint32 {
	_, ok := p.getCtx(ctx)
	if !ok {
		return ScardEInvalidHandle
	}
	return ScardSuccess
}

func (p *PCSCLiteBackend) ListReaders(ctx uint32, groups []byte) ([]byte, uint32) {
	pc, ok := p.getCtx(ctx)
	if !ok {
		return nil, ScardEInvalidHandle
	}

	pc.mu.Lock()
	defer pc.mu.Unlock()

	// Use CMD_GET_READERS_STATE to enumerate readers
	stateData, err := getReadersState(pc.conn)
	if err != nil {
		return nil, ScardFCommError
	}

	// Build multi-string from reader names (null-separated, double-null terminated)
	var names []byte
	found := false
	for i := range pcscMaxReaders {
		rs := parseReaderState(stateData[i*pcscReaderStateSize : (i+1)*pcscReaderStateSize])
		if rs.Name == "" {
			continue
		}
		found = true
		names = append(names, rs.Name...)
		names = append(names, 0) // null separator
	}
	if !found {
		return nil, ScardENoReadersAvailable
	}
	names = append(names, 0) // double-null terminator
	return names, ScardSuccess
}

func (p *PCSCLiteBackend) GetStatusChange(ctx uint32, timeout uint32, states []ScardReaderState) ([]ScardReaderState, uint32) {
	// Verify context exists but do NOT lock it — GetStatusChange can block
	// for a long time and must not prevent other operations (ListReaders,
	// BeginTransaction, Transmit, etc.) on the same context.
	// CMD_WAIT_READER_STATE_CHANGE and CMD_GET_READERS_STATE are global
	// operations that don't require a context, so we use a dedicated
	// temporary connection.
	_, ok := p.getCtx(ctx)
	if !ok {
		return nil, ScardEInvalidHandle
	}

	conn, err := p.dial()
	if err != nil {
		return nil, ScardENoService
	}
	defer conn.Close()

	if err := negotiateVersion(conn); err != nil {
		return nil, ScardENoService
	}

	// pcsclite protocol for GetStatusChange:
	// 1. Send CMD_WAIT_READER_STATE_CHANGE header (size=0, no payload) to
	//    register for events and get a snapshot of reader states.
	// 2. Read READER_STATE[16] = 2944 bytes (sent immediately by pcscd).
	// 3. Compare states. If changed: send CMD_STOP_WAITING to unregister,
	//    read the 8-byte wait_reader_state_change ack, and return.
	// 4. If not changed: block reading 8-byte event notification, then
	//    re-read states via CMD_GET_READERS_STATE.

	if err := sendCommandOnly(conn, pcscCmdWaitReaderStateChange, nil); err != nil {
		return nil, ScardFCommError
	}

	stateData := make([]byte, pcscReaderStateSize*pcscMaxReaders)
	if _, err := readFull(conn, stateData); err != nil {
		return nil, ScardFCommError
	}

	readerMap := buildReaderMap(stateData)
	result := make([]ScardReaderState, len(states))
	changed := compareReaderStates(states, readerMap, result)

	if changed || timeout == 0 {
		// Unregister from event notifications
		if err := unregisterFromEvents(conn); err != nil {
			return nil, ScardFCommError
		}
		if changed {
			return result, ScardSuccess
		}
		return result, ScardETimeout
	}

	// Block until event notification (8 bytes: timeout + rv)
	var waitBuf [8]byte
	if _, err := readFull(conn, waitBuf[:]); err != nil {
		return nil, ScardFCommError
	}
	rv := binary.LittleEndian.Uint32(waitBuf[4:8])
	if rv != ScardSuccess {
		return nil, rv
	}

	// Re-read states after change notification
	stateData, err = getReadersState(conn)
	if err != nil {
		return nil, ScardFCommError
	}

	readerMap = buildReaderMap(stateData)
	compareReaderStates(states, readerMap, result)
	return result, ScardSuccess
}

// buildReaderMap parses the 2944-byte READER_STATE array into a name→info map.
func buildReaderMap(stateData []byte) map[string]readerStateInfo {
	m := make(map[string]readerStateInfo)
	for i := range pcscMaxReaders {
		rs := parseReaderState(stateData[i*pcscReaderStateSize : (i+1)*pcscReaderStateSize])
		if rs.Name != "" {
			m[rs.Name] = rs
		}
	}
	return m
}

// compareReaderStates fills result from states+readerMap, returns true if any changed.
// Logic mirrors pcsclite's SCardGetStatusChange (winscard_clnt.c):
//   1. Event counter (upper 16 bits of dwCurrentState, only when non-zero)
//   2. State flag transitions (EMPTY↔PRESENT, MUTE, EXCLUSIVE↔INUSE)
//   3. SCARD_STATE_UNAWARE (dwCurrentState == 0) always returns CHANGED
func compareReaderStates(states []ScardReaderState, readerMap map[string]readerStateInfo, result []ScardReaderState) bool {
	changed := false
	for i, st := range states {
		result[i].Reader = st.Reader

		if strings.HasPrefix(st.Reader, "\\\\?PnP?\\") {
			// PnP notification pseudo-reader: upper 16 bits = reader count.
			readerCount := uint32(len(readerMap))
			curCount := st.CurrentState >> 16
			result[i].CurrentState = st.CurrentState
			result[i].EventState = readerCount << 16
			if readerCount != curCount && curCount != 0 {
				result[i].EventState |= ScardStateChanged
				changed = true
			}
			continue
		}

		rs, exists := readerMap[st.Reader]
		if !exists {
			result[i].CurrentState = st.CurrentState
			if st.CurrentState&ScardStateUnknown == 0 {
				result[i].EventState = ScardStateUnknown | ScardStateChanged
				changed = true
			} else {
				result[i].EventState = ScardStateUnknown
			}
			continue
		}

		// Build EventState from pcscd internal state (matching pcsclite logic)
		var evtState uint32

		// Event counter in upper 16 bits
		evtState |= uint32(rs.EventCounter) << 16

		// Check event counter (only when client has one — upper bits non-zero)
		breakFlag := false
		if st.CurrentState&0xFFFF0000 != 0 {
			curCounter := (st.CurrentState >> 16) & 0xFFFF
			if rs.EventCounter != curCounter {
				evtState |= ScardStateChanged
				breakFlag = true
			}
		}

		// Reader was UNKNOWN but now exists — it "came back"
		if st.CurrentState&ScardStateUnknown != 0 {
			evtState |= ScardStateChanged
			evtState &^= ScardStateUnknown
			breakFlag = true
		}

		// Card presence/absence
		if rs.State&pcscStateAbsent != 0 {
			evtState |= ScardStateEmpty
			if st.CurrentState&ScardStatePresent != 0 {
				evtState |= ScardStateChanged
				breakFlag = true
			}
		} else if rs.State&pcscStatePresent != 0 {
			evtState |= ScardStatePresent
			if st.CurrentState&ScardStateEmpty != 0 {
				evtState |= ScardStateChanged
				breakFlag = true
			}
			// Mute (swallowed)
			if rs.State&pcscStateSwallowed != 0 {
				evtState |= ScardStateMute
				if st.CurrentState&ScardStateMute == 0 {
					evtState |= ScardStateChanged
					breakFlag = true
				}
			} else if st.CurrentState&ScardStateMute != 0 {
				evtState |= ScardStateChanged
				breakFlag = true
			}
			// ATR
			result[i].ATR = rs.ATR
		}

		// Sharing modes (matching pcsclite exactly)
		if rs.Sharing == -1 { // EXCLUSIVE
			evtState |= ScardStateExclusive
			evtState &^= ScardStateInuse
			if st.CurrentState&ScardStateInuse != 0 {
				evtState |= ScardStateChanged
				breakFlag = true
			}
		} else if rs.Sharing >= 1 { // SHARED (card must be present)
			if rs.State&pcscStatePresent != 0 {
				evtState |= ScardStateInuse
				evtState &^= ScardStateExclusive
				if st.CurrentState&ScardStateExclusive != 0 {
					evtState |= ScardStateChanged
					breakFlag = true
				}
			}
		} else { // NO_CONTEXT (sharing == 0)
			evtState &^= ScardStateInuse
			evtState &^= ScardStateExclusive
			if st.CurrentState&ScardStateInuse != 0 {
				evtState |= ScardStateChanged
				breakFlag = true
			} else if st.CurrentState&ScardStateExclusive != 0 {
				evtState |= ScardStateChanged
				breakFlag = true
			}
		}

		// SCARD_STATE_UNAWARE: always return immediately with CHANGED
		if st.CurrentState == ScardStateUnaware {
			evtState |= ScardStateChanged
			breakFlag = true
		}

		result[i].CurrentState = st.CurrentState
		result[i].EventState = evtState
		if breakFlag {
			changed = true
		}
	}
	return changed
}

// unregisterFromEvents sends CMD_STOP_WAITING and reads the 8-byte ack.
func unregisterFromEvents(conn net.Conn) error {
	if err := sendCommandOnly(conn, pcscCmdStopWaitingReaderStateChange, nil); err != nil {
		return err
	}
	var buf [8]byte
	_, err := readFull(conn, buf[:])
	return err
}

func (p *PCSCLiteBackend) Connect(ctx uint32, reader string, shareMode, preferredProtocol uint32) (uint32, uint32, uint32) {
	pc, ok := p.getCtx(ctx)
	if !ok {
		return 0, 0, ScardEInvalidHandle
	}

	pc.mu.Lock()
	defer pc.mu.Unlock()

	// connect_struct: hContext(4) + szReader(128) + dwShareMode(4) +
	//   dwPreferredProtocols(4) + hCard(4) + dwActiveProtocol(4) + rv(4) = 152 bytes
	var buf [152]byte
	binary.LittleEndian.PutUint32(buf[0:4], ctx)
	copy(buf[4:4+pcscMaxReaderName], reader)
	binary.LittleEndian.PutUint32(buf[132:136], shareMode)
	binary.LittleEndian.PutUint32(buf[136:140], preferredProtocol)
	// buf[140:144] = hCard = 0 (output)
	// buf[144:148] = dwActiveProtocol = 0 (output)
	// buf[148:152] = rv = 0

	if err := sendRecv(pc.conn, pcscCmdConnect, buf[:]); err != nil {
		return 0, 0, ScardFCommError
	}

	hCard := binary.LittleEndian.Uint32(buf[140:144])
	activeProtocol := binary.LittleEndian.Uint32(buf[144:148])
	rv := binary.LittleEndian.Uint32(buf[148:152])

	if rv == ScardSuccess {
		p.mu.Lock()
		p.cardCtx[hCard] = ctx
		p.cardReader[hCard] = reader
		p.mu.Unlock()
	}

	return hCard, activeProtocol, rv
}

func (p *PCSCLiteBackend) Disconnect(handle uint32, disposition uint32) uint32 {
	pc, ok := p.getCtxForCard(handle)
	if !ok {
		return ScardEInvalidHandle
	}

	pc.mu.Lock()
	defer pc.mu.Unlock()

	// disconnect_struct: hCard(4) + dwDisposition(4) + rv(4) = 12 bytes
	var buf [12]byte
	binary.LittleEndian.PutUint32(buf[0:4], handle)
	binary.LittleEndian.PutUint32(buf[4:8], disposition)
	if err := sendRecv(pc.conn, pcscCmdDisconnect, buf[:]); err != nil {
		return ScardFCommError
	}

	rv := binary.LittleEndian.Uint32(buf[8:12])
	if rv == ScardSuccess {
		p.mu.Lock()
		delete(p.cardCtx, handle)
		delete(p.cardReader, handle)
		p.mu.Unlock()
	}
	return rv
}

func (p *PCSCLiteBackend) Reconnect(handle uint32, shareMode, preferredProtocol, disposition uint32) (uint32, uint32) {
	pc, ok := p.getCtxForCard(handle)
	if !ok {
		return 0, ScardEInvalidHandle
	}

	pc.mu.Lock()
	defer pc.mu.Unlock()

	// reconnect_struct: hCard(4) + dwShareMode(4) + dwPreferredProtocols(4) +
	//   dwInitialization(4) + dwActiveProtocol(4) + rv(4) = 24 bytes
	var buf [24]byte
	binary.LittleEndian.PutUint32(buf[0:4], handle)
	binary.LittleEndian.PutUint32(buf[4:8], shareMode)
	binary.LittleEndian.PutUint32(buf[8:12], preferredProtocol)
	binary.LittleEndian.PutUint32(buf[12:16], disposition)
	// buf[16:20] = dwActiveProtocol = 0 (output)
	// buf[20:24] = rv = 0

	if err := sendRecv(pc.conn, pcscCmdReconnect, buf[:]); err != nil {
		return 0, ScardFCommError
	}

	activeProtocol := binary.LittleEndian.Uint32(buf[16:20])
	rv := binary.LittleEndian.Uint32(buf[20:24])
	return activeProtocol, rv
}

func (p *PCSCLiteBackend) BeginTransaction(handle uint32) uint32 {
	pc, ok := p.getCtxForCard(handle)
	if !ok {
		return ScardEInvalidHandle
	}

	pc.mu.Lock()
	defer pc.mu.Unlock()

	// begin_struct: hCard(4) + rv(4) = 8 bytes
	var buf [8]byte
	binary.LittleEndian.PutUint32(buf[0:4], handle)
	if err := sendRecv(pc.conn, pcscCmdBeginTransaction, buf[:]); err != nil {
		return ScardFCommError
	}
	return binary.LittleEndian.Uint32(buf[4:8])
}

func (p *PCSCLiteBackend) EndTransaction(handle uint32, disposition uint32) uint32 {
	pc, ok := p.getCtxForCard(handle)
	if !ok {
		return ScardEInvalidHandle
	}

	pc.mu.Lock()
	defer pc.mu.Unlock()

	// end_struct: hCard(4) + dwDisposition(4) + rv(4) = 12 bytes
	var buf [12]byte
	binary.LittleEndian.PutUint32(buf[0:4], handle)
	binary.LittleEndian.PutUint32(buf[4:8], disposition)
	if err := sendRecv(pc.conn, pcscCmdEndTransaction, buf[:]); err != nil {
		return ScardFCommError
	}
	return binary.LittleEndian.Uint32(buf[8:12])
}

func (p *PCSCLiteBackend) Status(handle uint32) (string, uint32, uint32, []byte, uint32) {
	pc, ok := p.getCtxForCard(handle)
	if !ok {
		return "", 0, 0, nil, ScardEInvalidHandle
	}

	pc.mu.Lock()
	defer pc.mu.Unlock()

	// status_struct: hCard(4) + rv(4) = 8 bytes
	var buf [8]byte
	binary.LittleEndian.PutUint32(buf[0:4], handle)
	if err := sendRecv(pc.conn, pcscCmdStatus, buf[:]); err != nil {
		return "", 0, 0, nil, ScardFCommError
	}
	rv := binary.LittleEndian.Uint32(buf[4:8])
	if rv != ScardSuccess {
		return "", 0, 0, nil, rv
	}

	// Get reader info from CMD_GET_READERS_STATE
	stateData, err := getReadersState(pc.conn)
	if err != nil {
		return "", 0, 0, nil, ScardFCommError
	}

	// Look up reader name from our tracking
	p.mu.Lock()
	readerName := p.cardReader[handle]
	p.mu.Unlock()

	// Find the reader in the state array
	for i := range pcscMaxReaders {
		rs := parseReaderState(stateData[i*pcscReaderStateSize : (i+1)*pcscReaderStateSize])
		if rs.Name == readerName {
			dwState := mapInternalToCardState(rs)
			return rs.Name, dwState, rs.Protocol, rs.ATR, ScardSuccess
		}
	}

	// Reader not found in states, return what we know
	return readerName, 0, 0, nil, ScardSuccess
}

func (p *PCSCLiteBackend) Transmit(handle uint32, sendPCI, sendBuf []byte) ([]byte, []byte, uint32) {
	pc, ok := p.getCtxForCard(handle)
	if !ok {
		return nil, nil, ScardEInvalidHandle
	}

	pc.mu.Lock()
	defer pc.mu.Unlock()

	// Parse sendPCI: dwProtocol(4) + cbPciLength(4)
	var sendProtocol uint32
	if len(sendPCI) >= 4 {
		sendProtocol = binary.LittleEndian.Uint32(sendPCI[0:4])
	}

	// transmit_struct: hCard(4) + ioSendPciProtocol(4) + ioSendPciLength(4) +
	//   cbSendLength(4) + ioRecvPciProtocol(4) + ioRecvPciLength(4) +
	//   pcbRecvLength(4) + rv(4) = 32 bytes
	var buf [32]byte
	binary.LittleEndian.PutUint32(buf[0:4], handle)
	binary.LittleEndian.PutUint32(buf[4:8], sendProtocol)
	binary.LittleEndian.PutUint32(buf[8:12], 8) // ioSendPciLength (standard PCI = 8)
	binary.LittleEndian.PutUint32(buf[12:16], uint32(len(sendBuf)))
	// buf[16:20] = ioRecvPciProtocol = 0
	// buf[20:24] = ioRecvPciLength = 0
	binary.LittleEndian.PutUint32(buf[24:28], pcscMaxBufferSizeExt) // pcbRecvLength (max)
	// buf[28:32] = rv = 0

	// 1. Send header + transmit_struct
	if err := sendCommandOnly(pc.conn, pcscCmdTransmit, buf[:]); err != nil {
		return nil, nil, ScardFCommError
	}

	// 2. Send APDU data (raw, no header)
	if len(sendBuf) > 0 {
		if _, err := pc.conn.Write(sendBuf); err != nil {
			return nil, nil, ScardFCommError
		}
	}

	// 3. Read transmit_struct response
	if _, err := readFull(pc.conn, buf[:]); err != nil {
		return nil, nil, ScardFCommError
	}

	rv := binary.LittleEndian.Uint32(buf[28:32])
	if rv != ScardSuccess {
		return nil, nil, rv
	}

	recvProtocol := binary.LittleEndian.Uint32(buf[16:20])
	recvLen := binary.LittleEndian.Uint32(buf[24:28])

	// 4. Read response data
	var recvBuf []byte
	if recvLen > 0 {
		recvBuf = make([]byte, recvLen)
		if _, err := readFull(pc.conn, recvBuf); err != nil {
			return nil, nil, ScardFCommError
		}
	}

	// Build recvPCI
	recvPCIBuf := make([]byte, 8)
	binary.LittleEndian.PutUint32(recvPCIBuf[0:4], recvProtocol)
	binary.LittleEndian.PutUint32(recvPCIBuf[4:8], 8)

	return recvPCIBuf, recvBuf, ScardSuccess
}

func (p *PCSCLiteBackend) Control(handle uint32, controlCode uint32, inBuf []byte) ([]byte, uint32) {
	pc, ok := p.getCtxForCard(handle)
	if !ok {
		return nil, ScardEInvalidHandle
	}

	pc.mu.Lock()
	defer pc.mu.Unlock()

	// control_struct: hCard(4) + dwControlCode(4) + cbSendLength(4) +
	//   cbRecvLength(4) + dwBytesReturned(4) + rv(4) = 24 bytes
	var buf [24]byte
	binary.LittleEndian.PutUint32(buf[0:4], handle)
	binary.LittleEndian.PutUint32(buf[4:8], controlCode)
	binary.LittleEndian.PutUint32(buf[8:12], uint32(len(inBuf)))
	binary.LittleEndian.PutUint32(buf[12:16], pcscMaxBufferSizeExt) // cbRecvLength (max)
	// buf[16:20] = dwBytesReturned = 0 (output)
	// buf[20:24] = rv = 0

	// 1. Send header + control_struct
	if err := sendCommandOnly(pc.conn, pcscCmdControl, buf[:]); err != nil {
		return nil, ScardFCommError
	}

	// 2. Send input data (raw, no header)
	if len(inBuf) > 0 {
		if _, err := pc.conn.Write(inBuf); err != nil {
			return nil, ScardFCommError
		}
	}

	// 3. Read control_struct response
	if _, err := readFull(pc.conn, buf[:]); err != nil {
		return nil, ScardFCommError
	}

	bytesReturned := binary.LittleEndian.Uint32(buf[16:20])
	rv := binary.LittleEndian.Uint32(buf[20:24])
	if rv != ScardSuccess {
		return nil, rv
	}

	// 4. Read output data
	var outBuf []byte
	if bytesReturned > 0 {
		outBuf = make([]byte, bytesReturned)
		if _, err := readFull(pc.conn, outBuf); err != nil {
			return nil, ScardFCommError
		}
	}

	return outBuf, ScardSuccess
}

func (p *PCSCLiteBackend) GetAttrib(handle uint32, attrID uint32) ([]byte, uint32) {
	pc, ok := p.getCtxForCard(handle)
	if !ok {
		return nil, ScardEInvalidHandle
	}

	pc.mu.Lock()
	defer pc.mu.Unlock()

	// getset_struct: hCard(4) + dwAttrId(4) + pbAttr(MAX_BUFFER_SIZE_EXTENDED) +
	//   cbAttrLen(4) + rv(4)
	const structSize = 4 + 4 + pcscMaxBufferSizeExt + 4 + 4 // 131096
	buf := make([]byte, structSize)
	binary.LittleEndian.PutUint32(buf[0:4], handle)
	binary.LittleEndian.PutUint32(buf[4:8], attrID)
	// Request max attribute size
	binary.LittleEndian.PutUint32(buf[8+pcscMaxBufferSizeExt:8+pcscMaxBufferSizeExt+4], pcscMaxBufferSizeExt) // cbAttrLen
	// rv = 0

	if err := sendRecv(pc.conn, pcscCmdGetAttrib, buf); err != nil {
		return nil, ScardFCommError
	}

	cbAttrLen := binary.LittleEndian.Uint32(buf[8+pcscMaxBufferSizeExt : 8+pcscMaxBufferSizeExt+4])
	rv := binary.LittleEndian.Uint32(buf[8+pcscMaxBufferSizeExt+4 : 8+pcscMaxBufferSizeExt+8])
	if rv != ScardSuccess {
		return nil, rv
	}

	if cbAttrLen > pcscMaxBufferSizeExt {
		return nil, ScardFCommError
	}

	attr := make([]byte, cbAttrLen)
	copy(attr, buf[8:8+cbAttrLen])
	return attr, ScardSuccess
}

func (p *PCSCLiteBackend) Cancel(ctx uint32) uint32 {
	// Verify context exists (but don't lock it — it's blocked by GetStatusChange).
	_, ok := p.getCtx(ctx)
	if !ok {
		return ScardEInvalidHandle
	}

	// pcsclite SCardCancel: open a NEW connection, send SCARD_CANCEL with
	// hContext, read the response, close the connection. The server finds
	// the blocked context's connection and sends SCARD_E_CANCELLED on it,
	// waking up the blocked GetStatusChange.
	conn, err := p.dial()
	if err != nil {
		return ScardENoService
	}
	defer conn.Close()

	if err := negotiateVersion(conn); err != nil {
		return ScardENoService
	}

	// cancel_struct: hContext(4) + rv(4) = 8 bytes
	var buf [8]byte
	binary.LittleEndian.PutUint32(buf[0:4], ctx)
	if err := sendRecv(conn, pcscCmdCancel, buf[:]); err != nil {
		return ScardFCommError
	}
	return binary.LittleEndian.Uint32(buf[4:8])
}

func (p *PCSCLiteBackend) Close() error {
	p.mu.Lock()
	contexts := p.contexts
	p.contexts = make(map[uint32]*pcscContext)
	p.cardCtx = make(map[uint32]uint32)
	p.cardReader = make(map[uint32]string)
	p.mu.Unlock()

	for _, pc := range contexts {
		pc.conn.Close()
	}
	return nil
}

// cString extracts a null-terminated string from a byte slice.
func cString(b []byte) string {
	for i, c := range b {
		if c == 0 {
			return string(b[:i])
		}
	}
	return string(b)
}
