// Package rdpsnd implements the MS-RDPSND (Remote Desktop Protocol: Audio
// Output Virtual Channel Extension) for receiving audio from the server
// over the "rdpsnd" static virtual channel.
//
// PCM, MS-ADPCM, and IMA ADPCM formats are negotiated. ADPCM data is
// decoded to 16-bit PCM before delivery via callback — no actual playback
// is performed.
//
// Protocol reference: MS-RDPSND.
package rdpsnd

import (
	"context"
	"encoding/binary"
	"fmt"
	"log/slog"
	"github.com/bouncyball-git/gopher-rdp/util"
)

// PDU message types (MS-RDPSND 2.2).
const (
	SNDCClose       byte = 0x01
	SNDCWave        byte = 0x02
	SNDCSetVolume   byte = 0x03
	SNDCSetPitch    byte = 0x04
	SNDCWaveConfirm byte = 0x05
	SNDCTraining    byte = 0x06
	SNDCFormats     byte = 0x07
	SNDCCryptKey    byte = 0x08
	SNDCWaveEncrypt byte = 0x09
	SNDCUDPWave     byte = 0x0A
	SNDCUDPWaveComp byte = 0x0B
	SNDCQualityMode byte = 0x0C
	SNDCWave2       byte = 0x0D
)

// Capability flags (dwFlags in SNDC_FORMATS).
const (
	TSSNDCapsAlive  uint16 = 0x0001
	TSSNDCapsVolume uint16 = 0x0002
	TSSNDCapsPitch  uint16 = 0x0004
)

// Quality mode values (MS-RDPEA 2.2.2.3).
const (
	QualityModeDynamic uint16 = 0x0000
	QualityModeMedium  uint16 = 0x0001
	QualityModeHigh    uint16 = 0x0002
)

// Audio format tags.
const (
	WaveFormatPCM       uint16 = 0x0001
	WaveFormatADPCM     uint16 = 0x0002 // MS-ADPCM
	WaveFormatIMAADPCM  uint16 = 0x0011 // IMA ADPCM (DVI)
)

// SNDPROLOG header size: msgType(1) + bPad(1) + bodySize(2).
const sndPrologSize = 4

// formatsHeaderSize is the fixed header in SNDC_FORMATS body:
// dwFlags(4) + dwVolume(4) + dwPitch(4) + wDGramPort(2) +
// wNumberOfFormats(2) + cLastBlockConfirmed(1) + wVersion(2) + bPad(1) = 20.
const formatsHeaderSize = 20

// AudioFormat represents an AUDIO_FORMAT structure (MS-RDPSND 2.2.2.1).
type AudioFormat struct {
	Tag            uint16 // wFormatTag (e.g., WaveFormatPCM)
	Channels       uint16 // nChannels
	SamplesPerSec  uint32 // nSamplesPerSec
	AvgBytesPerSec uint32 // nAvgBytesPerSec
	BlockAlign     uint16 // nBlockAlign
	BitsPerSample  uint16 // wBitsPerSample
	ExtraData      []byte // cbSize + extra format data
}

// audioFormatFixedSize is the size of AUDIO_FORMAT without extra data:
// tag(2) + channels(2) + samplesPerSec(4) + avgBytesPerSec(4) + blockAlign(2) + bitsPerSample(2) + cbSize(2) = 18.
const audioFormatFixedSize = 18

// AudioSample is delivered to the callback when a wave PDU is decoded.
type AudioSample struct {
	Format         AudioFormat // negotiated format for this block
	WaveTimestamp  uint16      // wTimeStamp from server
	AudioTimestamp uint32      // dwAudioTimeStamp (SNDC_WAVE2 only, 0 for SNDC_WAVE)
	Data           []byte      // raw PCM audio data (owned by caller)
}

// FormatFilter controls which server-offered audio formats are accepted.
type FormatFilter struct {
	Stereo  bool   // true = only accept channels >= 2
	MinRate uint32 // 0 = any, e.g. 44100 = reject lower sample rates
	PCMOnly bool   // true = reject ADPCM formats
}

// Handler manages the rdpsnd protocol over the "rdpsnd" static virtual channel.
type Handler struct {
	sendFn func([]byte) error
	log    *slog.Logger
	filter FormatFilter

	// Negotiated state
	serverFormats []AudioFormat // all formats offered by server
	clientFormats []AudioFormat // PCM subset we accepted
	ready         bool          // handshake complete

	// SNDC_WAVE two-part state (MS-RDPSND 2.2.3.3)
	pendingWave  bool
	waveInitBuf  [4]byte // first 4 audio bytes saved from WaveInfo
	waveTS       uint16  // wTimeStamp
	waveFormatNo uint16  // wFormatNo (index into clientFormats)
	waveBlockNo  byte    // cBlockNo

	// Reusable audio buffer — avoids per-packet allocation.
	// Grown as needed, valid only until the next wave PDU.
	audioBuf []byte

	// Reusable ADPCM decode buffers.
	decodeBuf []int16 // decoded PCM samples (int16)
	pcmBuf    []byte  // int16 samples as little-endian bytes

	// Callbacks
	onWaveData func(*AudioSample)
	onClose    func()
}

// NewHandler creates an rdpsnd handler.
// sendFn writes data to the "rdpsnd" static virtual channel.
func NewHandler(sendFn func([]byte) error, log *slog.Logger) *Handler {
	return &Handler{sendFn: sendFn, log: log}
}

// SetSendFn replaces the function used to send PDUs back to the server.
// Used when the transport switches from static channel to DVC.
func (h *Handler) SetSendFn(fn func([]byte) error) {
	h.sendFn = fn
}

// SetFormatFilter configures which server-offered formats are accepted.
// Must be called before the handshake (before ProcessPDU sees SNDC_FORMATS).
func (h *Handler) SetFormatFilter(f FormatFilter) {
	h.filter = f
}

// OnWaveData sets the callback invoked when decoded PCM audio arrives.
func (h *Handler) OnWaveData(fn func(*AudioSample)) {
	h.onWaveData = fn
}

// OnClose sets the callback invoked when the server closes the audio channel.
func (h *Handler) OnClose(fn func()) {
	h.onClose = fn
}

// ProcessPDU dispatches an incoming rdpsnd PDU.
//
// SNDC_WAVE uses a two-part protocol (MS-RDPSND 2.2.3.3 + 2.2.3.4):
// WaveInfo saves the first 4 audio bytes and metadata,
// then the continuation PDU's body has its first 4 junk bytes replaced with
// the saved audio bytes to form the complete wave data.
func (h *Handler) ProcessPDU(data []byte) {
	if h.pendingWave {
		h.handleWaveContinuation(data)
		return
	}

	msgType, body, err := decodePDU(data)
	if err != nil {
		h.log.LogAttrs(context.Background(), slog.LevelError, "PDU decode error", slog.Any("err", err))
		return
	}

	switch msgType {
	case SNDCFormats:
		h.handleServerFormats(body)
	case SNDCTraining:
		h.handleTraining(body)
	case SNDCWave:
		h.handleWaveInfo(body)
	case SNDCWave2:
		h.handleWave2(body)
	case SNDCClose:
		h.handleClose()
	case SNDCQualityMode:
		h.handleQualityMode(body)
	default:
		h.log.LogAttrs(context.Background(), slog.LevelWarn, "unknown PDU type", util.Hex2("type", msgType), slog.Int("bodyLen", len(body)))
	}
}

// encodePDU builds an rdpsnd PDU: SNDPROLOG(4) + body.
func encodePDU(msgType byte, body []byte) []byte {
	buf := make([]byte, sndPrologSize+len(body))
	buf[0] = msgType
	binary.LittleEndian.PutUint16(buf[2:4], uint16(len(body)))
	copy(buf[sndPrologSize:], body)
	return buf
}

// decodePDU parses a SNDPROLOG header and returns the body slice (into input).
func decodePDU(data []byte) (msgType byte, body []byte, err error) {
	if len(data) < sndPrologSize {
		return 0, nil, fmt.Errorf("rdpsnd: PDU too short (%d bytes)", len(data))
	}
	msgType = data[0]
	bodySize := int(binary.LittleEndian.Uint16(data[2:4]))
	end := min(sndPrologSize+bodySize, len(data))
	return msgType, data[sndPrologSize:end], nil
}

// decodeAudioFormat parses one AUDIO_FORMAT at the given offset.
// Returns the format and the number of bytes consumed.
func decodeAudioFormat(data []byte, off int) (AudioFormat, int, error) {
	if off+audioFormatFixedSize > len(data) {
		return AudioFormat{}, 0, fmt.Errorf("rdpsnd: audio format too short at offset %d", off)
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
		return AudioFormat{}, 0, fmt.Errorf("rdpsnd: audio format extra data overflows at offset %d", off)
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

// handleServerFormats processes SNDC_FORMATS from the server.
//
// Body layout (MS-RDPSND 2.2.2.1):
//
//	[0:4]   dwFlags
//	[4:8]   dwVolume
//	[8:12]  dwPitch
//	[12:14] wDGramPort
//	[14:16] wNumberOfFormats
//	[16]    cLastBlockConfirmed
//	[17:19] wVersion
//	[19]    bPad
//	[20:]   AUDIO_FORMAT[wNumberOfFormats]
func (h *Handler) handleServerFormats(body []byte) {
	if len(body) < formatsHeaderSize {
		h.log.LogAttrs(context.Background(), slog.LevelWarn, "server formats body too short", slog.Int("len", len(body)))
		return
	}

	numFormats := binary.LittleEndian.Uint16(body[14:16])
	version := binary.LittleEndian.Uint16(body[17:19])
	h.log.LogAttrs(context.Background(), slog.LevelDebug, "server formats", slog.Int("numFormats", int(numFormats)), slog.Int("version", int(version)))

	// Parse all server formats, accept only PCM.
	// wFormatNo in wave PDUs indexes into the client's format list, so the
	// order we send back matters. We keep clientFormats in the same relative
	// order as the server's list — the server will use our indices.
	h.serverFormats = h.serverFormats[:0]
	h.clientFormats = h.clientFormats[:0]
	off := formatsHeaderSize
	for i := 0; i < int(numFormats); i++ {
		f, consumed, err := decodeAudioFormat(body, off)
		if err != nil {
			h.log.LogAttrs(context.Background(), slog.LevelWarn, "audio format decode error", slog.Any("err", err))
			break
		}
		h.serverFormats = append(h.serverFormats, f)
		accepted := h.acceptFormat(f)
		if accepted {
			h.clientFormats = append(h.clientFormats, f)
		}
		h.log.LogAttrs(context.Background(), slog.LevelDebug, "server format",
			slog.Int("i", i),
			util.Hex4("tag", f.Tag),
			slog.Int("ch", int(f.Channels)),
			slog.Int("rate", int(f.SamplesPerSec)),
			slog.Int("bits", int(f.BitsPerSample)),
			slog.Int("blockAlign", int(f.BlockAlign)),
			slog.Bool("accepted", accepted))
		off += consumed
	}

	// Prefer MS-ADPCM over IMA ADPCM: if any MS-ADPCM format was accepted,
	// drop all IMA ADPCM entries so the server doesn't pick the inferior codec.
	h.clientFormats = preferMSADPCM(h.clientFormats)

	h.log.LogAttrs(context.Background(), slog.LevelDebug, "accepted formats", slog.Int("accepted", len(h.clientFormats)), slog.Int("total", len(h.serverFormats)))

	h.sendClientFormats(version)
	h.ready = true
}

// sendClientFormats sends SNDC_FORMATS back to the server with accepted formats.
// Layout matches MS-RDPSND 2.2.2.2.
func (h *Handler) sendClientFormats(serverVersion uint16) {
	fmtDataSize := 0
	for i := range h.clientFormats {
		fmtDataSize += audioFormatFixedSize + len(h.clientFormats[i].ExtraData)
	}

	body := make([]byte, formatsHeaderSize+fmtDataSize)
	// [0:4] dwFlags — TSSNDCAPS_ALIVE
	binary.LittleEndian.PutUint32(body[0:4], uint32(TSSNDCapsAlive))
	// [4:8] dwVolume — 0 (we don't control volume)
	// [8:12] dwPitch — 0
	// [12:14] wDGramPort — 0
	// [14:16] wNumberOfFormats
	binary.LittleEndian.PutUint16(body[14:16], uint16(len(h.clientFormats)))
	// [16] cLastBlockConfirmed — 0
	// [17:19] wVersion — clamp to min(serverVersion, 8)
	ver := min(serverVersion, 8)
	binary.LittleEndian.PutUint16(body[17:19], ver)
	// [19] bPad — 0

	off := formatsHeaderSize
	for i := range h.clientFormats {
		off += encodeAudioFormat(body, off, &h.clientFormats[i])
	}

	if err := h.sendFn(encodePDU(SNDCFormats, body[:off])); err != nil {
		h.log.LogAttrs(context.Background(), slog.LevelError, "failed to send client formats", slog.Any("err", err))
	}

	// If both sides are version 6+, client MUST send Quality Mode PDU
	// immediately after the format response (MS-RDPEA 2.2.2.3).
	if ver >= 6 {
		h.sendQualityMode(QualityModeHigh)
	}
}

// sendQualityMode sends SNDC_QUALITYMODE to the server.
// Body: wQualityMode(2) + Reserved(2).
func (h *Handler) sendQualityMode(mode uint16) {
	var body [4]byte
	binary.LittleEndian.PutUint16(body[0:2], mode)
	// body[2:4] = Reserved, zero
	if err := h.sendFn(encodePDU(SNDCQualityMode, body[:])); err != nil {
		h.log.LogAttrs(context.Background(), slog.LevelError, "failed to send quality mode", slog.Any("err", err))
	}
}

// handleTraining echoes back the training PDU (zero-alloc).
// Body: wTimeStamp(2) + wPackSize(2).
// Response is identical: SNDC_TRAINING with same wTimeStamp + wPackSize
// (MS-RDPSND 2.2.3.2).
func (h *Handler) handleTraining(body []byte) {
	if len(body) < 4 {
		h.log.LogAttrs(context.Background(), slog.LevelWarn, "training body too short", slog.Int("len", len(body)))
		return
	}
	var buf [8]byte
	buf[0] = SNDCTraining
	binary.LittleEndian.PutUint16(buf[2:4], 4) // bodySize
	copy(buf[4:8], body[0:4])                   // wTimeStamp + wPackSize
	if err := h.sendFn(buf[:]); err != nil {
		h.log.LogAttrs(context.Background(), slog.LevelError, "failed to send training confirm", slog.Any("err", err))
	}
}

// handleWaveInfo processes the first part of SNDC_WAVE.
//
// Body (MS-RDPSND 2.2.3.3):
//
//	[0:2]  wTimeStamp
//	[2:4]  wFormatNo (index into client's format list)
//	[4]    cBlockNo
//	[5:8]  bPad(3)
//	[8:12] first 4 bytes of audio data
//
// The remaining audio arrives in the next ProcessPDU call as a continuation.
func (h *Handler) handleWaveInfo(body []byte) {
	if len(body) < 12 {
		h.log.LogAttrs(context.Background(), slog.LevelWarn, "wave info body too short", slog.Int("len", len(body)))
		return
	}
	h.waveTS = binary.LittleEndian.Uint16(body[0:2])
	h.waveFormatNo = binary.LittleEndian.Uint16(body[2:4])
	h.waveBlockNo = body[4]
	copy(h.waveInitBuf[:], body[8:12])
	h.pendingWave = true
}

// handleWaveContinuation processes the second part of SNDC_WAVE.
//
// The continuation PDU has its own SNDPROLOG header. Its body's first 4 bytes
// are junk/padding — we replace them with the 4 audio bytes saved from WaveInfo
// to form the complete audio frame (MS-RDPSND 2.2.3.4).
func (h *Handler) handleWaveContinuation(data []byte) {
	h.pendingWave = false

	if len(data) < sndPrologSize {
		h.log.LogAttrs(context.Background(), slog.LevelWarn, "wave continuation too short", slog.Int("len", len(data)))
		h.sendWaveConfirm(h.waveTS, h.waveBlockNo)
		return
	}

	body := data[sndPrologSize:]
	n := len(body)
	if cap(h.audioBuf) < n {
		h.audioBuf = make([]byte, n)
	}
	audioData := h.audioBuf[:n]
	copy(audioData, body)
	// Replace first 4 junk bytes with saved waveData
	if n >= 4 {
		copy(audioData[0:4], h.waveInitBuf[:])
	}

	h.deliverAudio(h.waveFormatNo, h.waveTS, 0, audioData)
	h.sendWaveConfirm(h.waveTS, h.waveBlockNo)
}

// handleWave2 processes SNDC_WAVE2 (single-PDU wave, version 6+).
//
// Body (MS-RDPSND 2.2.3.8):
//
//	[0:2]  wTimeStamp
//	[2:4]  wFormatNo
//	[4]    cBlockNo
//	[5:8]  bPad(3)
//	[8:12] dwAudioTimeStamp
//	[12:]  audio data
func (h *Handler) handleWave2(body []byte) {
	if len(body) < 12 {
		h.log.LogAttrs(context.Background(), slog.LevelWarn, "wave2 body too short", slog.Int("len", len(body)))
		return
	}
	ts := binary.LittleEndian.Uint16(body[0:2])
	formatNo := binary.LittleEndian.Uint16(body[2:4])
	blockNo := body[4]
	audioTS := binary.LittleEndian.Uint32(body[8:12])

	var audioData []byte
	if len(body) > 12 {
		n := len(body) - 12
		if cap(h.audioBuf) < n {
			h.audioBuf = make([]byte, n)
		}
		audioData = h.audioBuf[:n]
		copy(audioData, body[12:])
	}

	h.deliverAudio(formatNo, ts, audioTS, audioData)
	h.sendWaveConfirm(ts, blockNo)
}

// deliverAudio invokes the wave data callback with the decoded audio.
// formatNo is an index into the client's format list (sent in SNDC_FORMATS response).
// ADPCM formats are decoded to 16-bit PCM before delivery.
func (h *Handler) deliverAudio(formatNo, waveTS uint16, audioTS uint32, audioData []byte) {
	if h.onWaveData == nil {
		return
	}
	if int(formatNo) >= len(h.clientFormats) {
		h.log.LogAttrs(context.Background(), slog.LevelWarn, "formatNo out of range", slog.Int("formatNo", int(formatNo)), slog.Int("clientFormats", len(h.clientFormats)))
		return
	}

	af := h.clientFormats[formatNo]
	h.log.LogAttrs(context.Background(), slog.LevelDebug, "wave audio",
		slog.Int("formatNo", int(formatNo)),
		util.Hex4("tag", af.Tag),
		slog.Int("ch", int(af.Channels)),
		slog.Int("rate", int(af.SamplesPerSec)),
		slog.Int("dataLen", len(audioData)))

	if af.Tag == WaveFormatIMAADPCM || af.Tag == WaveFormatADPCM {
		audioData, af = h.decodeADPCMBlock(af, audioData)
	}

	h.onWaveData(&AudioSample{
		Format:         af,
		WaveTimestamp:  waveTS,
		AudioTimestamp: audioTS,
		Data:           audioData,
	})
}

// decodeADPCMBlock decodes ADPCM audio data to 16-bit PCM.
// Returns the PCM byte data and an updated AudioFormat with PCM parameters.
func (h *Handler) decodeADPCMBlock(af AudioFormat, audioData []byte) ([]byte, AudioFormat) {
	// Parse samplesPerBlock from ExtraData. For both MS-ADPCM and IMA ADPCM,
	// the first two bytes of cbSize extra data contain wSamplesPerBlock.
	if len(af.ExtraData) < 2 {
		h.log.LogAttrs(context.Background(), slog.LevelWarn, "ADPCM format missing samplesPerBlock in ExtraData")
		return audioData, af
	}
	samplesPerBlock := int(binary.LittleEndian.Uint16(af.ExtraData[0:2]))
	if samplesPerBlock == 0 {
		return audioData, af
	}

	channels := int(af.Channels)
	blockAlign := int(af.BlockAlign)

	switch af.Tag {
	case WaveFormatIMAADPCM:
		h.decodeBuf = decodeIMAADPCM(audioData, channels, samplesPerBlock, blockAlign, h.decodeBuf)
	case WaveFormatADPCM:
		h.decodeBuf = decodeMSADPCM(audioData, channels, samplesPerBlock, blockAlign, h.decodeBuf)
	}

	// Convert int16 samples → little-endian bytes.
	nBytes := len(h.decodeBuf) * 2
	if cap(h.pcmBuf) < nBytes {
		h.pcmBuf = make([]byte, nBytes)
	}
	h.pcmBuf = h.pcmBuf[:nBytes]
	for i, s := range h.decodeBuf {
		binary.LittleEndian.PutUint16(h.pcmBuf[i*2:], uint16(s))
	}

	// Build PCM format descriptor.
	pcmFmt := AudioFormat{
		Tag:           WaveFormatPCM,
		Channels:      af.Channels,
		SamplesPerSec: af.SamplesPerSec,
		BitsPerSample: 16,
		BlockAlign:    af.Channels * 2,
	}
	pcmFmt.AvgBytesPerSec = af.SamplesPerSec * uint32(pcmFmt.BlockAlign)

	return h.pcmBuf, pcmFmt
}

// sendWaveConfirm sends SNDC_WAVECONFIRM (zero-alloc via stack buffer).
//
// Body (MS-RDPSND 2.2.3.5):
//
//	[0:2] wTimeStamp
//	[2]   cConfirmedBlockNo
//	[3]   bPad
func (h *Handler) sendWaveConfirm(timestamp uint16, blockNo byte) {
	var buf [8]byte
	buf[0] = SNDCWaveConfirm
	binary.LittleEndian.PutUint16(buf[2:4], 4) // bodySize
	binary.LittleEndian.PutUint16(buf[4:6], timestamp)
	buf[6] = blockNo
	if err := h.sendFn(buf[:]); err != nil {
		h.log.LogAttrs(context.Background(), slog.LevelError, "failed to send wave confirm", slog.Any("err", err))
	}
}

// handleClose processes SNDC_CLOSE.
func (h *Handler) handleClose() {
	h.log.LogAttrs(context.Background(), slog.LevelInfo, "server closed audio channel")
	h.ready = false
	if h.onClose != nil {
		h.onClose()
	}
}

// handleQualityMode processes SNDC_QUALITYMODE.
// Body: wQualityMode(2) + Reserved(2).
func (h *Handler) handleQualityMode(body []byte) {
	if len(body) < 2 {
		return
	}
	mode := binary.LittleEndian.Uint16(body[0:2])
	h.log.LogAttrs(context.Background(), slog.LevelDebug, "quality mode", slog.Int("mode", int(mode)))
}
