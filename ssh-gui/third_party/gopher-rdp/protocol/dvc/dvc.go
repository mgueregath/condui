// Package dvc implements the Dynamic Virtual Channel (DRDYNVC) protocol
// (MS-RDPEDYC), which multiplexes dynamic channels over the "drdynvc"
// static virtual channel.
package dvc

import (
	"context"
	"encoding/binary"
	"fmt"
	"log/slog"

	"github.com/bouncyball-git/gopher-rdp/util"
)

// DRDYNVC command constants (bits 7-4 of the header byte).
const (
	CmdCreate    byte = 0x01
	CmdDataFirst byte = 0x02
	CmdData      byte = 0x03
	CmdClose     byte = 0x04
	CmdCaps      byte = 0x05
)

// DynChannel represents an open dynamic virtual channel.
type DynChannel struct {
	ID      uint32
	Name    string
	handler func([]byte) // called with complete reassembled data
	onOpen  func()       // called after create response is sent
	onClose func()       // called when the server closes the channel
	// Reassembly state for DataFirst/Data sequences
	reassemBuf    []byte
	reassemTotal  uint32
	rejected      bool // set by Reject() to refuse channel creation
}

// Handler manages DRDYNVC protocol state on the "drdynvc" static channel.
type Handler struct {
	sendFn        func([]byte) error       // sends on the drdynvc static channel
	log           *slog.Logger
	channels      map[uint32]*DynChannel   // open dynamic channels by ID
	version       uint16                   // negotiated caps version
	maxPDUPayload int                      // max bytes per SVC chunk (0 = 1600 default)
	onChannel     func(name string, ch *DynChannel) // callback when server opens a channel
}

// NewHandler creates a DRDYNVC handler.
// sendFn is called to write data to the "drdynvc" static virtual channel.
// maxPDUPayload is the maximum SVC chunk payload size (typically 1600);
// DVC messages larger than this are fragmented using CmdDataFirst/CmdData.
func NewHandler(sendFn func([]byte) error, log *slog.Logger, maxPDUPayload int) *Handler {
	if maxPDUPayload <= 0 {
		maxPDUPayload = 1600
	}
	return &Handler{
		sendFn:        sendFn,
		log:           log,
		channels:      make(map[uint32]*DynChannel),
		maxPDUPayload: maxPDUPayload,
	}
}

// OnChannel sets the callback invoked when the server creates a dynamic channel.
func (h *Handler) OnChannel(fn func(name string, ch *DynChannel)) {
	h.onChannel = fn
}

// SetChannelHandler sets the data handler for a dynamic channel.
func (ch *DynChannel) SetHandler(fn func([]byte)) {
	ch.handler = fn
}

// OnOpen sets a callback that fires after the DVC Create Response is sent.
// Use this to send initial data (e.g. CAPS_ADVERTISE) that the server will
// only accept after acknowledging the channel.
func (ch *DynChannel) OnOpen(fn func()) {
	ch.onOpen = fn
}

// OnClose sets a callback that fires when the server closes this channel.
func (ch *DynChannel) OnClose(fn func()) {
	ch.onClose = fn
}

// Reject marks this channel to be rejected during creation.
// Call this from the OnChannel callback to refuse a channel the client cannot handle.
func (ch *DynChannel) Reject() {
	ch.rejected = true
}

// ProcessPDU handles a complete PDU received on the drdynvc static channel.
func (h *Handler) ProcessPDU(data []byte) {
	if len(data) < 1 {
		return
	}
	hdr := data[0]
	cmd := (hdr >> 4) & 0x0F
	cbId := hdr & 0x03
	switch cmd {
	case CmdCaps:
		h.handleCaps(data)
	case CmdCreate:
		h.handleCreate(data, cbId)
	case CmdData:
		h.handleData(data, cbId)
	case CmdDataFirst:
		h.handleDataFirst(data, cbId)
	case CmdClose:
		h.handleClose(data, cbId)
	default:
		h.log.LogAttrs(context.Background(), slog.LevelWarn, "unknown command", util.Hex2("cmd", cmd))
	}
}

// SendData sends data on a dynamic channel.
// If the DVC PDU exceeds the SVC chunk size, it uses CmdDataFirst/CmdData
// fragmentation so each SVC PDU contains a single complete chunk.
func (h *Handler) SendData(channelID uint32, data []byte) error {
	cbId := cbIdSize(channelID)
	chIDLen := channelIDLen(cbId)
	dataHdrSize := 1 + chIDLen // CmdData header: hdr(1) + channelId

	// Fast path: small enough for a single CmdData PDU.
	if dataHdrSize+len(data) <= h.maxPDUPayload {
		buf := make([]byte, dataHdrSize+len(data))
		buf[0] = (CmdData << 4) | cbId
		putChannelID(buf[1:], cbId, channelID)
		copy(buf[dataHdrSize:], data)
		return h.sendFn(buf)
	}

	// DVC-level fragmentation using CmdDataFirst + CmdData.
	// CmdDataFirst header: hdr(1) + channelId + totalLength (variable).
	sp := cbIdSize(uint32(len(data))) // Sp encoding for totalLength
	spLen := channelIDLen(sp)
	firstHdrSize := 1 + chIDLen + spLen
	firstDataLen := h.maxPDUPayload - firstHdrSize

	buf := make([]byte, h.maxPDUPayload)
	buf[0] = (CmdDataFirst << 4) | cbId | (sp << 2)
	off := 1
	off += putChannelID(buf[off:], cbId, channelID)
	off += putChannelID(buf[off:], sp, uint32(len(data))) // totalLength
	copy(buf[off:], data[:firstDataLen])
	if err := h.sendFn(buf); err != nil {
		return err
	}

	// Subsequent fragments use CmdData.
	pos := firstDataLen
	for pos < len(data) {
		chunkLen := len(data) - pos
		if chunkLen > h.maxPDUPayload-dataHdrSize {
			chunkLen = h.maxPDUPayload - dataHdrSize
		}
		frag := make([]byte, dataHdrSize+chunkLen)
		frag[0] = (CmdData << 4) | cbId
		putChannelID(frag[1:], cbId, channelID)
		copy(frag[dataHdrSize:], data[pos:pos+chunkLen])
		if err := h.sendFn(frag); err != nil {
			return err
		}
		pos += chunkLen
	}
	return nil
}

func (h *Handler) handleCaps(data []byte) {
	// DYNVC_CAPS_VERSION PDU: hdr(1) + pad(1) + version(2) + [prio charge count...]
	if len(data) < 4 {
		return
	}
	serverVersion := binary.LittleEndian.Uint16(data[2:4])
	h.version = serverVersion
	h.log.LogAttrs(context.Background(), slog.LevelDebug, "server caps", slog.Int("version", int(serverVersion)))

	// Respond with client caps (version 1 is always compatible)
	clientVersion := serverVersion
	if clientVersion > 3 {
		clientVersion = 3
	}
	resp := make([]byte, 4)
	resp[0] = (CmdCaps << 4) // Sp=0, cbId=0
	resp[1] = 0              // pad
	binary.LittleEndian.PutUint16(resp[2:4], clientVersion)
	if err := h.sendFn(resp); err != nil {
		h.log.LogAttrs(context.Background(), slog.LevelError, "failed to send caps response", slog.Any("err", err))
	}
}

func (h *Handler) handleCreate(data []byte, cbId byte) {
	off := 1 // skip header byte
	channelID, n := getChannelID(data[off:], cbId)
	off += n
	if off >= len(data) {
		return
	}
	// Channel name is null-terminated ASCII
	nameBytes := data[off:]
	nameEnd := 0
	for nameEnd < len(nameBytes) && nameBytes[nameEnd] != 0 {
		nameEnd++
	}
	name := string(nameBytes[:nameEnd])

	ch := &DynChannel{
		ID:   channelID,
		Name: name,
	}

	// Notify callback — callback may set a handler or call ch.Reject().
	if h.onChannel != nil {
		h.onChannel(name, ch)
	}

	// Build create response
	resp := make([]byte, 1+channelIDLen(cbId)+4)
	resp[0] = (CmdCreate << 4) | cbId
	roff := 1
	roff += putChannelID(resp[roff:], cbId, channelID)

	if ch.rejected {
		// Explicitly rejected by callback
		binary.LittleEndian.PutUint32(resp[roff:], 0xC0000001) // STATUS_UNSUCCESSFUL
		h.log.LogAttrs(context.Background(), slog.LevelInfo, "channel rejected", slog.String("name", name), slog.Int("id", int(channelID)))
	} else {
		// Accept (status=0, already zero)
		h.channels[channelID] = ch
		h.log.LogAttrs(context.Background(), slog.LevelInfo, "channel accepted", slog.String("name", name), slog.Int("id", int(channelID)))
	}

	if err := h.sendFn(resp); err != nil {
		h.log.LogAttrs(context.Background(), slog.LevelError, "failed to send create response", slog.Any("err", err))
	}

	// Fire OnOpen callback after the create response has been sent,
	// so the server has acknowledged the channel before we send data.
	if !ch.rejected && ch.onOpen != nil {
		ch.onOpen()
	}
}

func (h *Handler) handleData(data []byte, cbId byte) {
	off := 1
	channelID, n := getChannelID(data[off:], cbId)
	off += n
	ch := h.channels[channelID]
	if ch == nil {
		h.log.LogAttrs(context.Background(), slog.LevelWarn, "data for unknown channel", slog.Int("id", int(channelID)))
		return
	}
	payload := data[off:]

	// If there's a pending reassembly, append and check if complete
	if ch.reassemTotal > 0 {
		ch.reassemBuf = append(ch.reassemBuf, payload...)
		h.log.LogAttrs(context.Background(), slog.LevelDebug, "data fragment", slog.String("name", ch.Name), slog.Int("id", int(channelID)), slog.Int("chunkLen", len(payload)), slog.Int("reassembled", len(ch.reassemBuf)), slog.Int("total", int(ch.reassemTotal)))
		if uint32(len(ch.reassemBuf)) >= ch.reassemTotal {
			h.log.LogAttrs(context.Background(), slog.LevelDebug, "reassembly complete", slog.String("name", ch.Name), slog.Int("id", int(channelID)), slog.Int("len", int(ch.reassemTotal)))
			if ch.handler != nil {
				ch.handler(ch.reassemBuf[:ch.reassemTotal])
			}
			ch.reassemBuf = ch.reassemBuf[:0]
			ch.reassemTotal = 0
		}
		return
	}

	// Single-fragment data
	h.log.LogAttrs(context.Background(), slog.LevelDebug, "data single", slog.String("name", ch.Name), slog.Int("id", int(channelID)), slog.Int("len", len(payload)))
	if ch.handler != nil {
		ch.handler(payload)
	}
}

func (h *Handler) handleDataFirst(data []byte, cbId byte) {
	off := 1
	channelID, n := getChannelID(data[off:], cbId)
	off += n
	ch := h.channels[channelID]
	if ch == nil {
		h.log.LogAttrs(context.Background(), slog.LevelWarn, "DataFirst for unknown channel", slog.Int("id", int(channelID)))
		return
	}
	// Total data length (same variable-length encoding as cbId but using Sp bits)
	sp := (data[0] >> 2) & 0x03
	totalLen, n := getLength(data[off:], sp)
	off += n

	// Guard against malicious totalLen causing unbounded memory growth.
	const maxDVCReassembly = 64 * 1024 * 1024 // 64 MB
	if totalLen > maxDVCReassembly {
		h.log.LogAttrs(context.Background(), slog.LevelWarn, "DVC DataFirst totalLen exceeds limit, dropping",
			slog.String("name", ch.Name), slog.Int("id", int(channelID)), slog.Int("totalLen", int(totalLen)))
		ch.reassemBuf = ch.reassemBuf[:0]
		ch.reassemTotal = 0
		return
	}

	ch.reassemBuf = append(ch.reassemBuf[:0], data[off:]...)
	h.log.LogAttrs(context.Background(), slog.LevelDebug, "DataFirst", slog.String("name", ch.Name), slog.Int("id", int(channelID)), slog.Int("totalLen", int(totalLen)), slog.Int("firstChunk", len(data[off:])))

	// If the first chunk already contains all the data, dispatch immediately.
	if uint32(len(ch.reassemBuf)) >= totalLen {
		h.log.LogAttrs(context.Background(), slog.LevelDebug, "DataFirst complete (single fragment)", slog.String("name", ch.Name), slog.Int("id", int(channelID)), slog.Int("len", int(totalLen)))
		if ch.handler != nil {
			ch.handler(ch.reassemBuf[:totalLen])
		}
		ch.reassemBuf = ch.reassemBuf[:0]
		ch.reassemTotal = 0
	} else {
		ch.reassemTotal = totalLen
	}
}

// CloseChannel sends a CmdClose PDU for the given channel and removes it
// from the channel map. This is a client-initiated close (device removal);
// the onClose callback is NOT fired (it is reserved for server-initiated closes).
func (h *Handler) CloseChannel(channelID uint32) error {
	ch := h.channels[channelID]
	if ch == nil {
		return nil
	}

	cbId := cbIdSize(channelID)
	chIDLen := channelIDLen(cbId)
	buf := make([]byte, 1+chIDLen)
	buf[0] = (CmdClose << 4) | cbId
	putChannelID(buf[1:], cbId, channelID)

	delete(h.channels, channelID)
	h.log.LogAttrs(context.Background(), slog.LevelInfo, "channel closed (client)",
		slog.Int("id", int(channelID)))

	return h.sendFn(buf)
}

func (h *Handler) handleClose(data []byte, cbId byte) {
	off := 1
	channelID, _ := getChannelID(data[off:], cbId)
	if ch := h.channels[channelID]; ch != nil {
		h.log.LogAttrs(context.Background(), slog.LevelInfo, "channel closed", slog.String("name", ch.Name), slog.Int("id", int(channelID)))
		delete(h.channels, channelID)
		if ch.onClose != nil {
			ch.onClose()
		}
	}
}

// --- Variable-length channel ID encoding ---

// cbIdSize returns the cbId field value (0, 1, or 2) for a channel ID.
func cbIdSize(id uint32) byte {
	if id <= 0xFF {
		return 0
	}
	if id <= 0xFFFF {
		return 1
	}
	return 2
}

// channelIDLen returns the byte length for a cbId field value.
func channelIDLen(cbId byte) int {
	switch cbId {
	case 0:
		return 1
	case 1:
		return 2
	case 2:
		return 4
	}
	return 1
}

// getChannelID reads a variable-length channel ID from data.
func getChannelID(data []byte, cbId byte) (uint32, int) {
	switch cbId {
	case 0:
		if len(data) < 1 {
			return 0, 0
		}
		return uint32(data[0]), 1
	case 1:
		if len(data) < 2 {
			return 0, 0
		}
		return uint32(binary.LittleEndian.Uint16(data[:2])), 2
	case 2:
		if len(data) < 4 {
			return 0, 0
		}
		return binary.LittleEndian.Uint32(data[:4]), 4
	}
	return 0, 0
}

// putChannelID writes a variable-length channel ID. Returns bytes written.
func putChannelID(buf []byte, cbId byte, id uint32) int {
	switch cbId {
	case 0:
		buf[0] = byte(id)
		return 1
	case 1:
		binary.LittleEndian.PutUint16(buf[:2], uint16(id))
		return 2
	case 2:
		binary.LittleEndian.PutUint32(buf[:4], id)
		return 4
	}
	return 0
}

// getLength reads a variable-length value using the Sp encoding bits.
func getLength(data []byte, sp byte) (uint32, int) {
	return getChannelID(data, sp) // same encoding
}

// Version returns the negotiated DRDYNVC protocol version.
func (h *Handler) Version() uint16 {
	return h.version
}

// String returns a description of the handler state.
func (h *Handler) String() string {
	return fmt.Sprintf("DVC(version=%d, channels=%d)", h.version, len(h.channels))
}
