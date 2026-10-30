// Package rdpecam implements the MS-RDPECAM webcam redirection protocol.
//
// The protocol uses two DVC channel types:
//   - Enumeration channel ("RDCamera_Device_Enumerator"): version negotiation + device announcements
//   - Per-device channel (deviceId as name): stream negotiation + H.264 sample delivery
//
// This implementation is web-only: the browser captures webcam frames via
// getUserMedia, encodes them as H.264 with WebCodecs VideoEncoder, and sends
// raw NAL units to Go which forwards them as SampleResponse PDUs.
package rdpecam

import (
	"context"
	"encoding/binary"
	"log/slog"
	"sync"
	"unicode/utf16"
)

// Channel name for the camera device enumerator.
const EnumChannelName = "RDCamera_Device_Enumerator"

// Protocol version.
const protoVersion = 0x02

// Message IDs (MS-RDPECAM 2.2).
const (
	msgSuccessResponse        = 0x01
	msgErrorResponse          = 0x02
	msgSelectVersionRequest   = 0x03
	msgSelectVersionResponse  = 0x04
	msgDeviceAddedNotify      = 0x05
	msgDeviceRemovedNotify    = 0x06
	msgActivateDeviceRequest  = 0x07
	msgDeactivateDeviceReq    = 0x08
	msgStreamListRequest      = 0x09
	msgStreamListResponse     = 0x0A
	msgMediaTypeListRequest   = 0x0B
	msgMediaTypeListResponse  = 0x0C
	msgCurrentMediaTypeReq    = 0x0D
	msgCurrentMediaTypeResp   = 0x0E
	msgStartStreamsRequest    = 0x0F
	msgStopStreamsRequest     = 0x10
	msgSampleRequest          = 0x11
	msgSampleResponse         = 0x12
	msgSampleErrorResponse    = 0x13
	msgPropertyListRequest    = 0x14
	msgPropertyListResponse   = 0x15
)

// Media formats.
const (
	FormatH264 = 0x01
)

// Stream source types and categories.
const (
	frameSourceColor   = 0x0001
	streamCategoryCapt = 0x01
)

// Error codes.
const (
	errNone             = 0x00000000
	errUnexpectedError  = 0x00000001
)

// MediaType describes an offered camera media type (26 bytes on wire).
type MediaType struct {
	Format       byte
	Width        uint32
	Height       uint32
	FPSNum       uint32
	FPSDenom     uint32
	PARNum       uint32
	PARDenom     uint32
	Flags        byte // bit0=DecodingRequired
}

func (m *MediaType) marshal(buf []byte) {
	buf[0] = m.Format
	binary.LittleEndian.PutUint32(buf[1:], m.Width)
	binary.LittleEndian.PutUint32(buf[5:], m.Height)
	binary.LittleEndian.PutUint32(buf[9:], m.FPSNum)
	binary.LittleEndian.PutUint32(buf[13:], m.FPSDenom)
	binary.LittleEndian.PutUint32(buf[17:], m.PARNum)
	binary.LittleEndian.PutUint32(buf[21:], m.PARDenom)
	buf[25] = m.Flags
}

func unmarshalMediaType(data []byte) MediaType {
	return MediaType{
		Format:   data[0],
		Width:    binary.LittleEndian.Uint32(data[1:]),
		Height:   binary.LittleEndian.Uint32(data[5:]),
		FPSNum:   binary.LittleEndian.Uint32(data[9:]),
		FPSDenom: binary.LittleEndian.Uint32(data[13:]),
		PARNum:   binary.LittleEndian.Uint32(data[17:]),
		PARDenom: binary.LittleEndian.Uint32(data[21:]),
		Flags:    data[25],
	}
}

// Handler manages the RDPECAM protocol for a single virtual webcam.
// It handles both the enumeration channel and per-device channel.
type Handler struct {
	log    *slog.Logger
	mu     sync.Mutex

	// Enumeration channel
	enumSendFn func([]byte) error
	version    byte

	// Per-device channel
	devSendFns   map[uint32]func([]byte) error // per DVC channel ID
	streamChID   uint32 // DVC channel ID used for streaming (SampleRequest/Response)
	deviceID     string // ASCII device identifier (also used as DVC channel name)
	deviceName   string // human-readable UTF-16 name
	streaming    bool
	currentMedia MediaType

	// Callbacks
	onStartCapture func(width, height, fps int) // notify browser to start capture
	onStopCapture  func()                       // notify browser to stop capture

	// Camera data from browser
	sampleCh  chan []byte // buffered H.264 NAL data from browser
	sampleBuf []byte     // reusable buffer for SampleResponse payload
	done      chan struct{}
	doneOnce  sync.Once
}

// NewHandler creates an RDPECAM handler for a virtual webcam.
// deviceName is the human-readable name shown in Windows (e.g. "gopher-rdp Camera").
func NewHandler(log *slog.Logger, deviceName string) *Handler {
	return &Handler{
		log:        log,
		deviceID:   "gopher-rdp-cam0",
		deviceName: deviceName,
		devSendFns: make(map[uint32]func([]byte) error),
		sampleCh:   make(chan []byte, 8),
		done:       make(chan struct{}),
		currentMedia: MediaType{
			Format:   FormatH264,
			Width:    1280,
			Height:   720,
			FPSNum:   30,
			FPSDenom: 1,
			PARNum:   1,
			PARDenom: 1,
			Flags:    0x01, // DecodingRequired
		},
	}
}

// OnStartCapture sets the callback invoked when the server starts streaming.
func (h *Handler) OnStartCapture(fn func(width, height, fps int)) {
	h.onStartCapture = fn
}

// OnStopCapture sets the callback invoked when the server stops streaming.
func (h *Handler) OnStopCapture(fn func()) {
	h.onStopCapture = fn
}

// DeviceID returns the DVC channel name for the per-device channel.
func (h *Handler) DeviceID() string {
	return h.deviceID
}

// SetEnumSendFn sets the send function for the enumeration channel.
func (h *Handler) SetEnumSendFn(fn func([]byte) error) {
	h.enumSendFn = fn
}

// SetDevSendFn registers a send function for a per-device DVC channel.
// Multiple device channels can be open simultaneously for the same device.
func (h *Handler) SetDevSendFn(chID uint32, fn func([]byte) error) {
	h.mu.Lock()
	h.devSendFns[chID] = fn
	h.mu.Unlock()
}

// RemoveDevSendFn removes the send function for a closed device channel.
func (h *Handler) RemoveDevSendFn(chID uint32) {
	h.mu.Lock()
	delete(h.devSendFns, chID)
	h.mu.Unlock()
}

// SendSample delivers an H.264 NAL unit from the browser for the next SampleResponse.
func (h *Handler) SendSample(nalData []byte) {
	select {
	case h.sampleCh <- nalData:
	default:
		// Drop if buffer full — camera frame drop is acceptable
	}
}

// ProcessEnumPDU handles data on the enumeration channel.
func (h *Handler) ProcessEnumPDU(data []byte) {
	if len(data) < 2 {
		return
	}
	h.version = data[0]
	msgID := data[1]

	h.log.LogAttrs(context.Background(), slog.LevelDebug, "RDPECAM enum PDU",
		slog.Int("version", int(h.version)), slog.Int("msgId", int(msgID)))

	switch msgID {
	case msgSelectVersionResponse:
		h.handleSelectVersionResponse()
	}
}

// ProcessDevPDU handles data on a per-device channel identified by chID.
func (h *Handler) ProcessDevPDU(chID uint32, data []byte) {
	if len(data) < 2 {
		return
	}
	// version := data[0]
	msgID := data[1]

	h.log.LogAttrs(context.Background(), slog.LevelDebug, "RDPECAM dev PDU",
		slog.Int("chId", int(chID)), slog.Int("msgId", int(msgID)), slog.Int("len", len(data)))

	switch msgID {
	case msgActivateDeviceRequest:
		h.sendDevMsg(chID, msgSuccessResponse, nil)
	case msgDeactivateDeviceReq:
		h.handleDeactivate(chID)
	case msgStreamListRequest:
		h.handleStreamListRequest(chID)
	case msgMediaTypeListRequest:
		h.handleMediaTypeListRequest(chID)
	case msgCurrentMediaTypeReq:
		h.handleCurrentMediaTypeRequest(chID)
	case msgStartStreamsRequest:
		h.handleStartStreams(chID, data)
	case msgStopStreamsRequest:
		h.handleStopStreams(chID)
	case msgSampleRequest:
		h.handleSampleRequest(chID, data)
	case msgPropertyListRequest:
		h.sendDevMsg(chID, msgPropertyListResponse, nil)
	}
}

// EnumChannelOpened should be called when the enumeration DVC opens.
// It sends the SelectVersionRequest and device announcement.
func (h *Handler) EnumChannelOpened() {
	h.sendEnumMsg(msgSelectVersionRequest, nil)
}

// DevChannelOpened should be called when the per-device DVC opens.
func (h *Handler) DevChannelOpened(chID uint32) {
	h.log.LogAttrs(context.Background(), slog.LevelInfo, "RDPECAM device channel opened",
		slog.String("deviceId", h.deviceID), slog.Int("chId", int(chID)))
}

// --- Enumeration channel handlers ---

func (h *Handler) handleSelectVersionResponse() {
	h.log.LogAttrs(context.Background(), slog.LevelInfo, "RDPECAM version negotiated",
		slog.Int("version", int(h.version)))

	// Send DeviceAddedNotification: DeviceName(UTF-16LE null-term) + VirtualChannelName(ASCII null-term)
	nameUTF16 := utf16.Encode([]rune(h.deviceName))
	nameBytes := make([]byte, (len(nameUTF16)+1)*2) // +1 for null terminator
	for i, c := range nameUTF16 {
		binary.LittleEndian.PutUint16(nameBytes[i*2:], c)
	}
	// null terminator already zero

	chanName := []byte(h.deviceID)
	chanName = append(chanName, 0) // null-terminate

	payload := make([]byte, len(nameBytes)+len(chanName))
	copy(payload, nameBytes)
	copy(payload[len(nameBytes):], chanName)

	h.sendEnumMsg(msgDeviceAddedNotify, payload)
	h.log.LogAttrs(context.Background(), slog.LevelInfo, "RDPECAM device added",
		slog.String("name", h.deviceName), slog.String("channelName", h.deviceID))
}

// --- Per-device channel handlers ---

func (h *Handler) handleStreamListRequest(chID uint32) {
	// One stream: Color capture, selected, not shared.
	// No count byte — server derives count from payload_length / 5.
	// StreamDescription: FrameSourceTypes(2) + StreamCategory(1) + Selected(1) + CanBeShared(1) = 5 bytes
	var payload [5]byte
	binary.LittleEndian.PutUint16(payload[0:], frameSourceColor)
	payload[2] = streamCategoryCapt
	payload[3] = 1 // Selected
	payload[4] = 0 // CanBeShared

	h.sendDevMsg(chID, msgStreamListResponse, payload[:])
}

func (h *Handler) handleMediaTypeListRequest(chID uint32) {
	var buf [26]byte
	h.currentMedia.marshal(buf[:])
	h.sendDevMsg(chID, msgMediaTypeListResponse, buf[:])
}

func (h *Handler) handleCurrentMediaTypeRequest(chID uint32) {
	var buf [26]byte
	h.currentMedia.marshal(buf[:])
	h.sendDevMsg(chID, msgCurrentMediaTypeResp, buf[:])
}

func (h *Handler) handleStartStreams(chID uint32, data []byte) {
	// Format: [version(1)][msgId(1)][StreamIndex(1)][MediaTypeDescription(26)]
	// No N_Infos byte — count is payload_length / 27.
	if len(data) < 2+1+26 {
		h.sendDevMsg(chID, msgSuccessResponse, nil)
		return
	}
	// streamIdx := data[2]
	mt := unmarshalMediaType(data[3:29])
	h.mu.Lock()
	h.currentMedia = mt
	h.streaming = true
	h.streamChID = chID // remember which channel handles streaming
	h.mu.Unlock()

	fps := 30
	if mt.FPSDenom > 0 {
		fps = int(mt.FPSNum / mt.FPSDenom)
	}
	if fps <= 0 {
		fps = 30
	}

	h.log.LogAttrs(context.Background(), slog.LevelInfo, "RDPECAM start streams",
		slog.Int("width", int(mt.Width)), slog.Int("height", int(mt.Height)),
		slog.Int("fps", fps), slog.Int("format", int(mt.Format)))

	h.sendDevMsg(chID, msgSuccessResponse, nil)

	if h.onStartCapture != nil {
		h.onStartCapture(int(mt.Width), int(mt.Height), fps)
	}
}

func (h *Handler) handleStopStreams(chID uint32) {
	h.mu.Lock()
	h.streaming = false
	h.mu.Unlock()

	// Drain sample channel
	for {
		select {
		case <-h.sampleCh:
		default:
			goto drained
		}
	}
drained:

	h.log.LogAttrs(context.Background(), slog.LevelInfo, "RDPECAM stop streams")
	h.sendDevMsg(chID, msgSuccessResponse, nil)

	if h.onStopCapture != nil {
		h.onStopCapture()
	}
}

func (h *Handler) handleDeactivate(chID uint32) {
	h.handleStopStreams(chID)
}

func (h *Handler) handleSampleRequest(chID uint32, data []byte) {
	streamIdx := byte(0)
	if len(data) >= 3 {
		streamIdx = data[2]
	}

	// Wait for the next camera frame from the browser. The server expects
	// a SampleResponse (not an error) — sending SampleErrorResponse causes
	// the server to stop requesting samples entirely.
	// Run in a goroutine so we don't block the DVC receive loop.
	go func() {
		select {
		case nalData := <-h.sampleCh:
			if len(nalData) == 0 {
				return
			}
			// Reuse buffer: sendDevMsg prepends 2-byte header, so total = 2+1+len(nalData).
			// We build the payload (streamIdx + NAL) and sendDevMsg wraps it.
			need := 1 + len(nalData)
			h.mu.Lock()
			if cap(h.sampleBuf) >= need {
				h.sampleBuf = h.sampleBuf[:need]
			} else {
				h.sampleBuf = make([]byte, need)
			}
			h.sampleBuf[0] = streamIdx
			copy(h.sampleBuf[1:], nalData)
			payload := h.sampleBuf
			h.mu.Unlock()
			h.sendDevMsg(chID, msgSampleResponse, payload)
		case <-h.done:
			return
		}
	}()
}

// Stop halts streaming, unblocks pending goroutines, and cleans up.
func (h *Handler) Stop() {
	h.doneOnce.Do(func() { close(h.done) })

	h.mu.Lock()
	wasStreaming := h.streaming
	h.streaming = false
	clear(h.devSendFns)
	h.mu.Unlock()

	if wasStreaming && h.onStopCapture != nil {
		h.onStopCapture()
	}
}

// --- Send helpers ---

func (h *Handler) sendEnumMsg(msgID byte, payload []byte) {
	buf := make([]byte, 2+len(payload))
	buf[0] = protoVersion
	buf[1] = msgID
	copy(buf[2:], payload)
	if h.enumSendFn != nil {
		if err := h.enumSendFn(buf); err != nil {
			h.log.LogAttrs(context.Background(), slog.LevelError, "RDPECAM enum send error",
				slog.Any("err", err))
		}
	}
}

// sendBuf is a reusable buffer for sendDevMsg. Guarded by mu in handleSampleRequest;
// other callers run on the DVC receive goroutine (single-threaded), so no contention.
var sendBufPool = sync.Pool{New: func() any { return make([]byte, 0, 256) }}

func (h *Handler) sendDevMsg(chID uint32, msgID byte, payload []byte) {
	need := 2 + len(payload)
	buf := sendBufPool.Get().([]byte)
	if cap(buf) >= need {
		buf = buf[:need]
	} else {
		buf = make([]byte, need)
	}
	buf[0] = protoVersion
	buf[1] = msgID
	copy(buf[2:], payload)
	h.log.LogAttrs(context.Background(), slog.LevelDebug, "RDPECAM dev send",
		slog.Int("chId", int(chID)), slog.Int("msgId", int(msgID)), slog.Int("len", need))
	h.mu.Lock()
	fn := h.devSendFns[chID]
	h.mu.Unlock()
	if fn != nil {
		if err := fn(buf); err != nil {
			h.log.LogAttrs(context.Background(), slog.LevelError, "RDPECAM dev send error",
				slog.Any("err", err))
		}
	}
	sendBufPool.Put(buf[:0])
}
