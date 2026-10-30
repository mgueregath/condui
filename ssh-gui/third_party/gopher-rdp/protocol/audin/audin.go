// Package audin implements the MS-RDPEAI (Remote Desktop Protocol: Audio
// Input Redirection Virtual Channel Extension) for sending microphone
// audio from the client to the server over the "AUDIO_INPUT" dynamic
// virtual channel.
//
// PCM, MS-ADPCM, and IMA ADPCM formats are negotiated. When an ADPCM
// format is selected, callers still provide raw 16-bit PCM via
// SendAudioData — encoding to ADPCM is performed transparently.
//
// Protocol reference: MS-RDPEAI.
package audin

import (
	"context"
	"encoding/binary"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
)

// PDU message types (MS-RDPEAI 2.2).
const (
	MsgVersion      byte = 0x01 // MSG_SNDIN_VERSION
	MsgFormats      byte = 0x02 // MSG_SNDIN_FORMATS
	MsgOpen         byte = 0x03 // MSG_SNDIN_OPEN
	MsgOpenReply    byte = 0x04 // MSG_SNDIN_OPEN_REPLY
	MsgDataIncoming byte = 0x05 // MSG_SNDIN_DATA_INCOMING
	MsgData         byte = 0x06 // MSG_SNDIN_DATA
	MsgFormatChange byte = 0x07 // MSG_SNDIN_FORMATCHANGE
)

// Audio format tags.
const (
	WaveFormatPCM       uint16 = 0x0001
	WaveFormatADPCM     uint16 = 0x0002 // MS-ADPCM
	WaveFormatIMAADPCM  uint16 = 0x0011 // IMA ADPCM (DVI)
)

// audioFormatFixedSize is the size of AUDIO_FORMAT without extra data:
// tag(2) + channels(2) + samplesPerSec(4) + avgBytesPerSec(4) + blockAlign(2) + bitsPerSample(2) + cbSize(2) = 18.
const audioFormatFixedSize = 18

// AudioFormat represents an AUDIO_FORMAT structure (MS-RDPEAI 2.2.1).
type AudioFormat struct {
	Tag            uint16 // wFormatTag (e.g., WaveFormatPCM)
	Channels       uint16 // nChannels
	SamplesPerSec  uint32 // nSamplesPerSec
	AvgBytesPerSec uint32 // nAvgBytesPerSec
	BlockAlign     uint16 // nBlockAlign
	BitsPerSample  uint16 // wBitsPerSample
	ExtraData      []byte // cbSize + extra format data
}

// FormatFilter controls which server-offered audio formats are accepted.
type FormatFilter struct {
	Stereo  bool   // true = only accept channels >= 2
	MinRate uint32 // 0 = any, e.g. 44100 = reject lower sample rates
	PCMOnly bool   // true = reject ADPCM formats
}

// Handler manages the MS-RDPEAI protocol over the "AUDIO_INPUT" DVC.
type Handler struct {
	sendFn func([]byte) error
	log    *slog.Logger
	filter FormatFilter

	// Format state (protected by mu)
	mu            sync.Mutex
	serverFormats []AudioFormat // all formats offered by server
	clientFormats []AudioFormat // PCM subset we accepted
	activeFormat  AudioFormat   // currently active format for recording

	// sendMu serialises sendFn calls so DATA_INCOMING + DATA pairs
	// are never interleaved by concurrent callers (silence fill vs real data).
	sendMu sync.Mutex

	// Recording state
	recording atomic.Bool

	// Reusable send buffer — avoids per-packet allocation in SendAudioData.
	sendBuf []byte

	// ADPCM encoding state.
	encodeBuf   []int16      // reusable buffer for byte→int16 PCM conversion
	adpcmBuf    []byte       // reusable buffer for encoded ADPCM output
	IMAEncState IMAEncState  // IMA ADPCM encoder state carried across blocks
	pcmFormat   AudioFormat  // PCM-equivalent format exposed to callers when ADPCM is active
	pendingSamples []int16   // leftover samples that didn't fill a complete ADPCM block

	// Callbacks
	onOpen  func(AudioFormat)
	onClose func()

}

// NewHandler creates an audin handler.
// sendFn writes data to the "AUDIO_INPUT" dynamic virtual channel.
func NewHandler(sendFn func([]byte) error, log *slog.Logger) *Handler {
	return &Handler{sendFn: sendFn, log: log}
}

// SetSendFn replaces the function used to send PDUs back to the server.
func (h *Handler) SetSendFn(fn func([]byte) error) {
	h.sendFn = fn
}

// SetFormatFilter configures which server-offered formats are accepted.
// Must be called before the handshake (before ProcessPDU sees MSG_SNDIN_FORMATS).
func (h *Handler) SetFormatFilter(f FormatFilter) {
	h.filter = f
}

// OnOpen sets the callback invoked when the server opens audio input
// and recording should begin with the given format.
func (h *Handler) OnOpen(fn func(AudioFormat)) {
	h.onOpen = fn
}

// OnClose sets the callback invoked when recording should stop.
func (h *Handler) OnClose(fn func()) {
	h.onClose = fn
}

// Recording returns true if the server has opened audio input.
func (h *Handler) Recording() bool {
	return h.recording.Load()
}

// ActiveFormat returns the audio format callers should use for PCM data.
// When the server selected ADPCM, this returns the PCM-equivalent format
// (same rate/channels, 16-bit PCM) so callers always provide raw PCM.
// Only meaningful when Recording() returns true.
func (h *Handler) ActiveFormat() AudioFormat {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.pcmFormat
}

// ProcessPDU dispatches an incoming audin PDU.
// MS-RDPEAI PDUs have a 1-byte message type header.
func (h *Handler) ProcessPDU(data []byte) {
	if len(data) < 1 {
		h.log.LogAttrs(context.Background(), slog.LevelError, "PDU too short", slog.Int("len", len(data)))
		return
	}

	msgType := data[0]
	body := data[1:]

	switch msgType {
	case MsgVersion:
		h.handleVersion(body)
	case MsgFormats:
		h.handleFormats(body)
	case MsgOpen:
		h.handleOpen(body)
	case MsgFormatChange:
		h.handleFormatChange(body)
	default:
		h.log.LogAttrs(context.Background(), slog.LevelWarn, "unknown PDU type", slog.Int("type", int(msgType)), slog.Int("bodyLen", len(body)))
	}
}

// handleVersion responds to MSG_SNDIN_VERSION from the server.
// Body: Version(4) — uint32 version number.
// We respond with min(serverVersion, 2). Version 2 adds format change support.
func (h *Handler) handleVersion(body []byte) {
	if len(body) < 4 {
		h.log.LogAttrs(context.Background(), slog.LevelWarn, "version body too short", slog.Int("len", len(body)))
		return
	}
	serverVersion := binary.LittleEndian.Uint32(body[0:4])
	h.log.LogAttrs(context.Background(), slog.LevelDebug, "server version", slog.Int("version", int(serverVersion)))

	// Respond with min(serverVersion, 2)
	clientVersion := serverVersion
	if clientVersion > 2 {
		clientVersion = 2
	}
	var buf [5]byte
	buf[0] = MsgVersion
	binary.LittleEndian.PutUint32(buf[1:5], clientVersion)
	if err := h.sendFn(buf[:]); err != nil {
		h.log.LogAttrs(context.Background(), slog.LevelError, "failed to send version", slog.Any("err", err))
	}
}

// acceptFormat returns true if the given server format passes the filter.
func (h *Handler) acceptFormat(f AudioFormat) bool {
	switch f.Tag {
	case WaveFormatPCM:
		// always acceptable codec
	case WaveFormatADPCM, WaveFormatIMAADPCM:
		if h.filter.PCMOnly {
			return false
		}
	default:
		return false
	}
	if h.filter.Stereo && f.Channels < 2 {
		return false
	}
	if h.filter.MinRate > 0 && f.SamplesPerSec < h.filter.MinRate {
		return false
	}
	return true
}

// preferMSADPCM removes IMA ADPCM entries when MS-ADPCM is available.
// MS-ADPCM produces better quality at comparable bitrates, so we prefer it.
func preferMSADPCM(formats []AudioFormat) []AudioFormat {
	hasMSADPCM := false
	for _, f := range formats {
		if f.Tag == WaveFormatADPCM {
			hasMSADPCM = true
			break
		}
	}
	if !hasMSADPCM {
		return formats
	}
	n := 0
	for _, f := range formats {
		if f.Tag != WaveFormatIMAADPCM {
			formats[n] = f
			n++
		}
	}
	return formats[:n]
}

// handleFormats processes MSG_SNDIN_FORMATS from the server.
//
// Body layout (MS-RDPEAI 2.2.1):
//
//	[0:4]  NumFormats (uint32)
//	[4:8]  cbSizeFormatsPacket (uint32)
//	[8:]   AUDIO_FORMAT[NumFormats]
func (h *Handler) handleFormats(body []byte) {
	if len(body) < 8 {
		h.log.LogAttrs(context.Background(), slog.LevelWarn, "formats body too short", slog.Int("len", len(body)))
		return
	}

	numFormats := binary.LittleEndian.Uint32(body[0:4])
	h.log.LogAttrs(context.Background(), slog.LevelDebug, "server formats", slog.Int("numFormats", int(numFormats)))

	h.mu.Lock()
	defer h.mu.Unlock()

	// Parse all server formats, accept only PCM.
	h.serverFormats = h.serverFormats[:0]
	h.clientFormats = h.clientFormats[:0]
	off := 8
	for i := 0; i < int(numFormats); i++ {
		f, consumed, err := decodeAudioFormat(body, off)
		if err != nil {
			h.log.LogAttrs(context.Background(), slog.LevelWarn, "audio format decode error", slog.Any("err", err))
			break
		}
		h.serverFormats = append(h.serverFormats, f)
		if h.acceptFormat(f) {
			h.clientFormats = append(h.clientFormats, f)
		}
		off += consumed
	}

	// Prefer MS-ADPCM over IMA ADPCM: if any MS-ADPCM format was accepted,
	// drop all IMA ADPCM entries so the server doesn't pick the inferior codec.
	h.clientFormats = preferMSADPCM(h.clientFormats)

	h.log.LogAttrs(context.Background(), slog.LevelDebug, "accepted formats", slog.Int("accepted", len(h.clientFormats)), slog.Int("total", len(h.serverFormats)))

	h.sendClientFormats()
}

// sendClientFormats sends MSG_SNDIN_FORMATS back with PCM-only formats.
//
// Response layout (MS-RDPEAI 2.2.1):
//
//	[0]    MsgType (1)
//	[1:5]  NumFormats (uint32)
//	[5:9]  cbSizeFormatsPacket (uint32)
//	[9:]   AUDIO_FORMAT[NumFormats]
//
// Caller must hold h.mu.
func (h *Handler) sendClientFormats() {
	// Send DATA_INCOMING before formats reply — required by Windows RDP server
	// even though MS-RDPEAI spec associates DATA_INCOMING with audio DATA only.
	var incoming [1]byte
	incoming[0] = MsgDataIncoming
	if err := h.sendFn(incoming[:]); err != nil {
		h.log.LogAttrs(context.Background(), slog.LevelError, "failed to send data incoming before formats", slog.Any("err", err))
		return
	}

	fmtDataSize := 0
	for i := range h.clientFormats {
		fmtDataSize += audioFormatFixedSize + len(h.clientFormats[i].ExtraData)
	}

	totalSize := 1 + 8 + fmtDataSize // header + NumFormats + cbSizeFormatsPacket + formats
	buf := make([]byte, totalSize)
	buf[0] = MsgFormats
	binary.LittleEndian.PutUint32(buf[1:5], uint32(len(h.clientFormats)))
	binary.LittleEndian.PutUint32(buf[5:9], uint32(totalSize)) // cbSizeFormatsPacket = entire PDU size

	off := 9
	for i := range h.clientFormats {
		off += encodeAudioFormat(buf, off, &h.clientFormats[i])
	}

	if err := h.sendFn(buf[:off]); err != nil {
		h.log.LogAttrs(context.Background(), slog.LevelError, "failed to send client formats", slog.Any("err", err))
	}
}

// handleOpen processes MSG_SNDIN_OPEN from the server.
//
// Body layout (MS-RDPEAI 2.2.2):
//
//	[0:4]  FramesPerPacket (uint32)
//	[4:8]  initialFormat (uint32) — index into client's format list
func (h *Handler) handleOpen(body []byte) {
	if len(body) < 8 {
		h.log.LogAttrs(context.Background(), slog.LevelWarn, "open body too short", slog.Int("len", len(body)))
		return
	}

	framesPerPacket := binary.LittleEndian.Uint32(body[0:4])
	formatIdx := binary.LittleEndian.Uint32(body[4:8])

	h.mu.Lock()
	if int(formatIdx) >= len(h.clientFormats) {
		h.mu.Unlock()
		h.log.LogAttrs(context.Background(), slog.LevelWarn, "open format index out of range", slog.Int("idx", int(formatIdx)), slog.Int("formats", len(h.clientFormats)))
		return
	}
	h.activeFormat = h.clientFormats[formatIdx]
	h.IMAEncState = IMAEncState{}
	h.pendingSamples = h.pendingSamples[:0]
	h.computePCMFormat()
	callerFmt := h.pcmFormat
	h.mu.Unlock()

	h.recording.Store(true)

	h.log.LogAttrs(context.Background(), slog.LevelInfo, "audio input opened",
		slog.Int("framesPerPacket", int(framesPerPacket)),
		slog.Int("formatIdx", int(formatIdx)),
		slog.Int("tag", int(h.activeFormat.Tag)),
		slog.Int("rate", int(callerFmt.SamplesPerSec)),
		slog.Int("channels", int(callerFmt.Channels)),
		slog.Int("bits", int(callerFmt.BitsPerSample)),
	)

	// Send OPEN_REPLY with S_OK, then FORMAT_CHANGE confirming the format.
	var reply [5]byte
	reply[0] = MsgOpenReply
	binary.LittleEndian.PutUint32(reply[1:5], 0) // HRESULT S_OK
	if err := h.sendFn(reply[:]); err != nil {
		h.log.LogAttrs(context.Background(), slog.LevelError, "failed to send open reply", slog.Any("err", err))
	}

	// MSG_SNDIN_FORMATCHANGE echoes the initialFormat index back to the server.
	// Required by the Windows RDP server to finalize format selection (version 2).
	var fmtChange [5]byte
	fmtChange[0] = MsgFormatChange
	binary.LittleEndian.PutUint32(fmtChange[1:5], formatIdx)
	if err := h.sendFn(fmtChange[:]); err != nil {
		h.log.LogAttrs(context.Background(), slog.LevelError, "failed to send format change", slog.Any("err", err))
	}

	if h.onOpen != nil {
		h.onOpen(callerFmt)
	}
}

// handleFormatChange processes MSG_SNDIN_FORMATCHANGE from the server.
//
// Body layout (MS-RDPEAI 2.2.5):
//
//	[0:4]  NewFormat (uint32) — index into client's format list
func (h *Handler) handleFormatChange(body []byte) {
	if len(body) < 4 {
		h.log.LogAttrs(context.Background(), slog.LevelWarn, "format change body too short", slog.Int("len", len(body)))
		return
	}

	formatIdx := binary.LittleEndian.Uint32(body[0:4])

	h.mu.Lock()
	if int(formatIdx) >= len(h.clientFormats) {
		h.mu.Unlock()
		h.log.LogAttrs(context.Background(), slog.LevelWarn, "format change index out of range", slog.Int("idx", int(formatIdx)), slog.Int("formats", len(h.clientFormats)))
		return
	}
	h.activeFormat = h.clientFormats[formatIdx]
	h.IMAEncState = IMAEncState{}
	h.pendingSamples = h.pendingSamples[:0]
	h.computePCMFormat()
	callerFmt := h.pcmFormat
	h.mu.Unlock()

	h.log.LogAttrs(context.Background(), slog.LevelInfo, "audio input format changed",
		slog.Int("formatIdx", int(formatIdx)),
		slog.Int("tag", int(h.activeFormat.Tag)),
		slog.Int("rate", int(callerFmt.SamplesPerSec)),
		slog.Int("channels", int(callerFmt.Channels)),
		slog.Int("bits", int(callerFmt.BitsPerSample)),
	)

	if h.onOpen != nil {
		h.onOpen(callerFmt)
	}
}

// SendAudioData sends audio data to the server. Callers always provide raw
// 16-bit PCM bytes. When the negotiated format is ADPCM, the PCM is encoded
// transparently before transmission.
// Sends MSG_SNDIN_DATA_INCOMING followed by MSG_SNDIN_DATA.
// No-op if not currently recording.
func (h *Handler) SendAudioData(pcm []byte) error {
	if !h.recording.Load() {
		return nil
	}
	if len(pcm) == 0 {
		return nil
	}

	h.sendMu.Lock()
	defer h.sendMu.Unlock()

	// Re-check under lock — channel may have closed between check and lock.
	if !h.recording.Load() {
		return nil
	}

	// Determine wire payload: encode to ADPCM if needed, otherwise send PCM.
	payload := pcm

	h.mu.Lock()
	tag := h.activeFormat.Tag
	channels := int(h.activeFormat.Channels)
	blockAlign := int(h.activeFormat.BlockAlign)
	extraData := h.activeFormat.ExtraData
	h.mu.Unlock()

	if tag == WaveFormatIMAADPCM || tag == WaveFormatADPCM {
		// Convert PCM bytes to int16 samples.
		nSamples := len(pcm) / 2
		if cap(h.encodeBuf) < nSamples {
			h.encodeBuf = make([]int16, nSamples)
		}
		samples := h.encodeBuf[:nSamples]
		for i := range nSamples {
			samples[i] = int16(binary.LittleEndian.Uint16(pcm[i*2:]))
		}

		samplesPerBlock := ADPCMSamplesPerBlock(tag, blockAlign, channels, extraData)
		samplesPerFrame := samplesPerBlock * channels // interleaved samples per block

		// Prepend any leftover samples from the previous call so that we
		// only ever encode complete blocks. Partial blocks would be
		// zero-padded, inserting spurious silence into the decoded stream.
		if len(h.pendingSamples) > 0 {
			combined := make([]int16, len(h.pendingSamples)+len(samples))
			copy(combined, h.pendingSamples)
			copy(combined[len(h.pendingSamples):], samples)
			samples = combined
			h.pendingSamples = h.pendingSamples[:0]
		}

		// Encode and send complete blocks only.
		for off := 0; off+samplesPerFrame <= len(samples); off += samplesPerFrame {
			block := samples[off : off+samplesPerFrame]
			if tag == WaveFormatIMAADPCM {
				h.adpcmBuf = EncodeIMAADPCM(block, channels, samplesPerBlock, h.adpcmBuf, &h.IMAEncState)
			} else {
				h.adpcmBuf = encodeMSADPCM(block, channels, samplesPerBlock, blockAlign, h.adpcmBuf)
			}
			// Send one DATA_INCOMING + DATA per block.
			var incoming [1]byte
			incoming[0] = MsgDataIncoming
			if err := h.sendFn(incoming[:]); err != nil {
				return fmt.Errorf("audin: send data incoming: %w", err)
			}
			n := 1 + len(h.adpcmBuf)
			if cap(h.sendBuf) < n {
				h.sendBuf = make([]byte, n)
			}
			buf := h.sendBuf[:n]
			buf[0] = MsgData
			copy(buf[1:], h.adpcmBuf)
			if err := h.sendFn(buf); err != nil {
				return fmt.Errorf("audin: send data: %w", err)
			}
		}

		// Save leftover samples that didn't fill a complete block.
		leftover := len(samples) % samplesPerFrame
		if leftover > 0 {
			tail := samples[len(samples)-leftover:]
			if cap(h.pendingSamples) < leftover {
				h.pendingSamples = make([]int16, leftover)
			}
			h.pendingSamples = h.pendingSamples[:leftover]
			copy(h.pendingSamples, tail)
		}

		return nil
	}

	// PCM: send as single DATA_INCOMING + DATA.
	var incoming [1]byte
	incoming[0] = MsgDataIncoming
	if err := h.sendFn(incoming[:]); err != nil {
		return fmt.Errorf("audin: send data incoming: %w", err)
	}

	n := 1 + len(payload)
	if cap(h.sendBuf) < n {
		h.sendBuf = make([]byte, n)
	}
	buf := h.sendBuf[:n]
	buf[0] = MsgData
	copy(buf[1:], payload)
	if err := h.sendFn(buf); err != nil {
		return fmt.Errorf("audin: send data: %w", err)
	}

	return nil
}

// computePCMFormat sets h.pcmFormat to a PCM-equivalent of h.activeFormat.
// For PCM formats, pcmFormat == activeFormat. For ADPCM, it keeps the same
// rate/channels but uses 16-bit PCM so callers always provide raw PCM.
// Caller must hold h.mu.
func (h *Handler) computePCMFormat() {
	f := h.activeFormat
	if f.Tag == WaveFormatADPCM || f.Tag == WaveFormatIMAADPCM {
		h.pcmFormat = AudioFormat{
			Tag:            WaveFormatPCM,
			Channels:       f.Channels,
			SamplesPerSec:  f.SamplesPerSec,
			BitsPerSample:  16,
			BlockAlign:     f.Channels * 2,
			AvgBytesPerSec: f.SamplesPerSec * uint32(f.Channels) * 2,
		}
	} else {
		h.pcmFormat = f
	}
}

// ADPCMSamplesPerBlock returns the number of samples per block for ADPCM formats.
func ADPCMSamplesPerBlock(tag uint16, blockAlign, channels int, extraData []byte) int {
	if channels < 1 {
		channels = 1
	}
	switch tag {
	case WaveFormatIMAADPCM:
		// From extra data (wSamplesPerBlock) if available.
		if len(extraData) >= 2 {
			return int(binary.LittleEndian.Uint16(extraData[0:2]))
		}
		// Compute: header gives 1 sample per channel, then nibble data.
		headerSize := 4 * channels
		dataBytes := blockAlign - headerSize
		if dataBytes < 0 {
			return 1
		}
		if channels == 1 {
			return 1 + dataBytes*2
		}
		return 1 + (dataBytes/channels/4)*8
	case WaveFormatADPCM:
		// From extra data: [wSamplesPerBlock:u16] [wNumCoef:u16] [coef pairs...]
		if len(extraData) >= 2 {
			return int(binary.LittleEndian.Uint16(extraData[0:2]))
		}
		// Compute: header is 7*channels, remaining bytes hold nibble pairs.
		headerSize := 7 * channels
		dataBytes := blockAlign - headerSize
		if dataBytes < 0 {
			return 2
		}
		return 2 + dataBytes*2/channels
	}
	return 1
}

// Stop stops recording. Called when the DVC is closed.
func (h *Handler) Stop() {
	if h.recording.Swap(false) {
		h.log.LogAttrs(context.Background(), slog.LevelInfo, "audio input stopped")
		if h.onClose != nil {
			h.onClose()
		}
	}
}

// decodeAudioFormat parses one AUDIO_FORMAT at the given offset.
// Returns the format and the number of bytes consumed.
func decodeAudioFormat(data []byte, off int) (AudioFormat, int, error) {
	if off+audioFormatFixedSize > len(data) {
		return AudioFormat{}, 0, fmt.Errorf("audin: audio format too short at offset %d", off)
	}
	f := AudioFormat{
		Tag:            binary.LittleEndian.Uint16(data[off:]),
		Channels:       binary.LittleEndian.Uint16(data[off+2:]),
		SamplesPerSec:  binary.LittleEndian.Uint32(data[off+4:]),
		AvgBytesPerSec: binary.LittleEndian.Uint32(data[off+8:]),
		BlockAlign:     binary.LittleEndian.Uint16(data[off+12:]),
		BitsPerSample:  binary.LittleEndian.Uint16(data[off+14:]),
	}
	cbSize := binary.LittleEndian.Uint16(data[off+16:])
	consumed := audioFormatFixedSize + int(cbSize)
	if off+consumed > len(data) {
		return AudioFormat{}, 0, fmt.Errorf("audin: audio format extra data overflows at offset %d", off)
	}
	if cbSize > 0 {
		f.ExtraData = make([]byte, cbSize)
		copy(f.ExtraData, data[off+audioFormatFixedSize:off+consumed])
	}
	return f, consumed, nil
}

// encodeAudioFormat writes one AUDIO_FORMAT at the given offset in buf.
// Returns the number of bytes written.
func encodeAudioFormat(buf []byte, off int, f *AudioFormat) int {
	binary.LittleEndian.PutUint16(buf[off:], f.Tag)
	binary.LittleEndian.PutUint16(buf[off+2:], f.Channels)
	binary.LittleEndian.PutUint32(buf[off+4:], f.SamplesPerSec)
	binary.LittleEndian.PutUint32(buf[off+8:], f.AvgBytesPerSec)
	binary.LittleEndian.PutUint16(buf[off+12:], f.BlockAlign)
	binary.LittleEndian.PutUint16(buf[off+14:], f.BitsPerSample)
	cbSize := uint16(len(f.ExtraData))
	binary.LittleEndian.PutUint16(buf[off+16:], cbSize)
	copy(buf[off+audioFormatFixedSize:], f.ExtraData)
	return audioFormatFixedSize + int(cbSize)
}
