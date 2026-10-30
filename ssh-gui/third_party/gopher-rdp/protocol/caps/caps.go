// Package caps implements RDP capability set encoding and decoding
// (MS-RDPBCGR sections 2.2.1.13, 2.2.7).
package caps

import (
	"context"
	"encoding/binary"
	"log/slog"

	"github.com/bouncyball-git/gopher-rdp/util"
)

// Capability set type constants
const (
	TypeGeneral              uint16 = 0x0001
	TypeBitmap               uint16 = 0x0002
	TypeOrder                uint16 = 0x0003
	TypeControl              uint16 = 0x0005
	TypeActivation           uint16 = 0x0007
	TypePointer              uint16 = 0x0008
	TypeShare                uint16 = 0x0009
	TypeColorCache           uint16 = 0x000A
	TypeSound                uint16 = 0x000C
	TypeInput                uint16 = 0x000D
	TypeFont                 uint16 = 0x000E
	TypeBrush                uint16 = 0x000F
	TypeGlyphCache           uint16 = 0x0010
	TypeOffscreenBitmapCache uint16 = 0x0012
	TypeBitmapCacheV2        uint16 = 0x0013
	TypeVirtualChannel       uint16 = 0x0014
	TypeCompDesk             uint16 = 0x0019
	TypeMultifragUpdate      uint16 = 0x001A
	TypeLargePointer         uint16 = 0x001B
	TypeSurfaceCommands      uint16 = 0x001C
	TypeBitmapCodecs         uint16 = 0x001D
	TypeFrameAcknowledge     uint16 = 0x001E
)

// Capability set sizes (header + payload).
const (
	generalSize              = 24  // 4 + 20
	bitmapSize               = 28  // 4 + 24
	orderSize                = 88  // 4 + 84
	bitmapCacheV2Size        = 40  // 4 + 36
	pointerSize              = 10  // 4 + 6
	inputSize                = 88  // 4 + 84
	brushSize                = 8   // 4 + 4
	glyphCacheSize           = 52  // 4 + 48
	offscreenBitmapCacheSize = 12  // 4 + 8
	virtualChannelSize       = 8   // 4 + 4
	soundSize                = 8   // 4 + 4
	controlSize              = 12  // 4 + 8
	activationSize           = 12  // 4 + 8
	shareSize                = 8   // 4 + 4
	colorCacheSize           = 8   // 4 + 4
	fontSize                 = 8   // 4 + 4
	multifragUpdateSize      = 8   // 4 + 4
	largePointerSize         = 6   // 4 + 2
	compDeskSize             = 6   // 4 + 2 CompDeskSupportLevel
	surfaceCommandsSize      = 12  // 4 + 4 cmdFlags + 4 reserved
	bitmapCodecsSize         = 118 // 4 hdr + 1 count + NSCodec(22) + RemoteFX(68) + RemoteFX Image(23)
	frameAcknowledgeSize     = 8   // 4 + 4 maxUnacknowledgedFrameCount

	// Base caps: always sent (15 core caps including sound)
	capsCountBase = 15
	baseCapsSize  = generalSize + bitmapSize + orderSize + bitmapCacheV2Size +
		pointerSize + inputSize + brushSize + glyphCacheSize +
		virtualChannelSize + soundSize +
		controlSize + activationSize + shareSize + colorCacheSize +
		fontSize

	// Individual conditional cap sizes are used directly in BuildConfirmCapabilities.
)

// CapabilitySet is a raw capability set with its type and payload.
type CapabilitySet struct {
	Type    uint16
	Payload []byte // data after the 4-byte header
}

// putCapHeader writes a 4-byte capability set header at buf[off:].
func putCapHeader(buf []byte, off int, capType, totalLen uint16) {
	binary.LittleEndian.PutUint16(buf[off:], capType)
	binary.LittleEndian.PutUint16(buf[off+2:], totalLen)
}

// --- put functions: write directly into buf at offset, zero bytes assumed ---

func putGeneral(buf []byte, off int) {
	putCapHeader(buf, off, TypeGeneral, generalSize)
	binary.LittleEndian.PutUint16(buf[off+4:], 1)       // osMajorType = OSMAJORTYPE_WINDOWS
	binary.LittleEndian.PutUint16(buf[off+6:], 3)       // osMinorType = OSMINORTYPE_WINDOWSNT
	binary.LittleEndian.PutUint16(buf[off+8:], 0x0200)  // protocolVersion
	binary.LittleEndian.PutUint16(buf[off+14:], 0x040D) // extraFlags: FASTPATH|LONG_CRED|AUTORECONNECT|NO_BITMAP_COMPRESSION_HDR
	// refreshRectSupport=0, suppressOutputSupport=0 (MS-RDPBCGR 2.2.7.1.1)
}

func putBitmap(buf []byte, off int, width, height, depth uint16) {
	putCapHeader(buf, off, TypeBitmap, bitmapSize)
	binary.LittleEndian.PutUint16(buf[off+4:], depth)   // preferredBitsPerPixel
	binary.LittleEndian.PutUint16(buf[off+6:], 1)       // receive1BitPerPixel
	binary.LittleEndian.PutUint16(buf[off+8:], 1)       // receive4BitsPerPixel
	binary.LittleEndian.PutUint16(buf[off+10:], 1)      // receive8BitsPerPixel
	binary.LittleEndian.PutUint16(buf[off+12:], width)  // desktopWidth
	binary.LittleEndian.PutUint16(buf[off+14:], height) // desktopHeight
	binary.LittleEndian.PutUint16(buf[off+18:], 1)      // desktopResizeFlag
	binary.LittleEndian.PutUint16(buf[off+20:], 1)      // bitmapCompressionFlag
	binary.LittleEndian.PutUint16(buf[off+24:], 1)      // multipleRectangleSupport
	// drawingFlags=0 (MS-RDPBCGR 2.2.7.1.2)
}

func putOrder(buf []byte, off int) {
	putCapHeader(buf, off, TypeOrder, orderSize)
	binary.LittleEndian.PutUint16(buf[off+24:], 1)       // desktopSaveXGranularity
	binary.LittleEndian.PutUint16(buf[off+26:], 20)      // desktopSaveYGranularity
	binary.LittleEndian.PutUint16(buf[off+30:], 1)       // maximumOrderLevel
	binary.LittleEndian.PutUint16(buf[off+34:], 0x002A)  // orderFlags: NEGOTIATE(0x02)|ZEROBOUNDSDELTA(0x08)|COLORINDEX(0x20)
	// orderSupport[32] starts at offset 36 from cap start
	// MS-RDPBCGR 2.2.7.1.3 — orderSupport indices
	buf[off+36+0x00] = 1 // DstBlt
	buf[off+36+0x01] = 1 // PatBlt
	buf[off+36+0x02] = 1 // ScrBlt
	buf[off+36+0x03] = 1 // MemBlt
	buf[off+36+0x04] = 1 // Mem3Blt
	buf[off+36+0x08] = 1 // LineTo
	buf[off+36+0x0A] = 1 // OpaqueRect
	buf[off+36+0x0B] = 1 // SaveBitmap
	// MultiOpaqueRect (0x12) intentionally NOT advertised — coded delta list
	// decoding is unreliable; server falls back to individual OpaqueRect orders.
	buf[off+36+0x13] = 1 // FastIndex
	buf[off+36+0x16] = 1 // Polyline
	buf[off+36+0x18] = 1 // FastGlyph
	buf[off+36+0x14] = 1 // PolygonSC
	buf[off+36+0x15] = 1 // PolygonCB
	buf[off+36+0x19] = 1 // EllipseSC
	buf[off+36+0x1A] = 1 // EllipseCB
	buf[off+36+0x1B] = 1 // GlyphIndex
	// off+68: textFlags(2) = 0
	// off+70: orderSupportExFlags(2) = 0
	// off+72: pad4octetsB(4) = 0
	binary.LittleEndian.PutUint32(buf[off+76:], 230400) // desktopSaveSize
	// off+80: pad2octetsC(2) = 0
	// off+82: pad2octetsD(2) = 0
	binary.LittleEndian.PutUint16(buf[off+84:], 1252) // textANSICodePage
}

func putBitmapCacheV2(buf []byte, off int) {
	putCapHeader(buf, off, TypeBitmapCacheV2, bitmapCacheV2Size)
	// cacheFlags(2) = 0 (no persist, MS-RDPBCGR 2.2.7.1.4.2)
	// pad2(1) = 0
	buf[off+7] = 3 // numCellCaches
	// Cache 0: 0x78 entries
	binary.LittleEndian.PutUint32(buf[off+8:], 0x78)
	// Cache 1: 0x78 entries
	binary.LittleEndian.PutUint32(buf[off+12:], 0x78)
	// Cache 2: 0x150 entries
	binary.LittleEndian.PutUint32(buf[off+16:], 0x150)
	// Caches 3-4: unused (0, already zeroed)
}

func putPointer(buf []byte, off int) {
	putCapHeader(buf, off, TypePointer, pointerSize)
	binary.LittleEndian.PutUint16(buf[off+4:], 1)  // colorPointerFlag
	binary.LittleEndian.PutUint16(buf[off+6:], 20) // colorPointerCacheSize
	binary.LittleEndian.PutUint16(buf[off+8:], 20) // pointerCacheSize
}

func putInput(buf []byte, off int) {
	putCapHeader(buf, off, TypeInput, inputSize)
	binary.LittleEndian.PutUint16(buf[off+4:], 0x0001)    // inputFlags = INPUT_FLAG_SCANCODES
	binary.LittleEndian.PutUint32(buf[off+8:], 0x00000409) // keyboardLayout
	binary.LittleEndian.PutUint32(buf[off+12:], 4)         // keyboardType
	binary.LittleEndian.PutUint32(buf[off+20:], 12)        // keyboardFunctionKey
}

func putBrush(buf []byte, off int) {
	putCapHeader(buf, off, TypeBrush, brushSize)
	binary.LittleEndian.PutUint32(buf[off+4:], 1) // BRUSH_COLOR_8x8
}

func putGlyphCache(buf []byte, off int) {
	putCapHeader(buf, off, TypeGlyphCache, glyphCacheSize)
	// 10 glyph caches: each is 4 bytes (numEntries u16 + maxCellSize u16)
	// Caches 0-1: 254 entries, 4 bytes max
	// Caches 2-3: 254 entries, 8 bytes max
	// Cache 4:    254 entries, 16 bytes max
	// Cache 5:    254 entries, 32 bytes max
	// Cache 6:    254 entries, 64 bytes max
	// Cache 7:    254 entries, 128 bytes max
	// Cache 8:    254 entries, 256 bytes max
	// Cache 9:     64 entries, 2048 bytes max
	type cacheEntry struct{ entries, cellSize uint16 }
	caches := [10]cacheEntry{
		{254, 4}, {254, 4}, {254, 8}, {254, 8},
		{254, 16}, {254, 32}, {254, 64}, {254, 128},
		{254, 256}, {64, 2048},
	}
	p := off + 4 // after header
	for _, c := range caches {
		binary.LittleEndian.PutUint16(buf[p:], c.entries)
		binary.LittleEndian.PutUint16(buf[p+2:], c.cellSize)
		p += 4
	}
	// Fragment cache: 256 entries, 256 bytes max
	binary.LittleEndian.PutUint16(buf[p:], 256)
	binary.LittleEndian.PutUint16(buf[p+2:], 256)
	p += 4
	// GlyphSupportLevel: 0x0002 = GLYPH_SUPPORT_FULL (Rev 1)
	binary.LittleEndian.PutUint16(buf[p:], 0x0002)
}

func putOffscreenBitmapCache(buf []byte, off int) {
	putCapHeader(buf, off, TypeOffscreenBitmapCache, offscreenBitmapCacheSize)
	binary.LittleEndian.PutUint32(buf[off+4:], 0)    // offscreenSupportLevel = FALSE (not implemented)
	binary.LittleEndian.PutUint16(buf[off+8:], 7680) // offscreenCacheSize (KB)
	binary.LittleEndian.PutUint16(buf[off+10:], 2000) // offscreenCacheEntries
}

func putVirtualChannel(buf []byte, off int) {
	putCapHeader(buf, off, TypeVirtualChannel, virtualChannelSize)
	binary.LittleEndian.PutUint32(buf[off+4:], 1) // flags = VCCAPS_COMPR_CS_8K
}

func putSound(buf []byte, off int, beeps bool) {
	putCapHeader(buf, off, TypeSound, soundSize)
	if beeps {
		binary.LittleEndian.PutUint16(buf[off+4:], 1) // soundFlags = SOUND_BEEPS_FLAG
	}
}

func putControl(buf []byte, off int) {
	putCapHeader(buf, off, TypeControl, controlSize)
	// controlFlags(2)=0, remoteDetachFlag(2)=0, controlInterest(2)=2, detachInterest(2)=2
	binary.LittleEndian.PutUint16(buf[off+8:], 2)  // controlInterest = CONTROLPRIORITY_NEVER
	binary.LittleEndian.PutUint16(buf[off+10:], 2) // detachInterest = CONTROLPRIORITY_NEVER
}

func putActivation(buf []byte, off int) {
	putCapHeader(buf, off, TypeActivation, activationSize)
	// helpKeyFlag(2)=0, helpKeyIndexFlag(2)=0, helpExtendedKeyFlag(2)=0, windowManagerKeyFlag(2)=0
}

func putShare(buf []byte, off int) {
	putCapHeader(buf, off, TypeShare, shareSize)
	// nodeId(2)=0, pad2octets(2)=0
}

func putColorCache(buf []byte, off int) {
	putCapHeader(buf, off, TypeColorCache, colorCacheSize)
	binary.LittleEndian.PutUint16(buf[off+4:], 6) // colorTableCacheSize
}

func putFont(buf []byte, off int) {
	putCapHeader(buf, off, TypeFont, fontSize)
	binary.LittleEndian.PutUint16(buf[off+4:], 0x0001) // fontSupportFlags = FONTSUPPORT_FONTLIST
}

func putMultifragUpdate(buf []byte, off int) {
	putCapHeader(buf, off, TypeMultifragUpdate, multifragUpdateSize)
	binary.LittleEndian.PutUint32(buf[off+4:], 0x300000) // MaxRequestSize = 3 MB
}

func putLargePointer(buf []byte, off int) {
	putCapHeader(buf, off, TypeLargePointer, largePointerSize)
	binary.LittleEndian.PutUint16(buf[off+4:], 0x0001) // LARGE_POINTER_FLAG_96x96
}

func putCompDesk(buf []byte, off int) {
	putCapHeader(buf, off, TypeCompDesk, compDeskSize)
	binary.LittleEndian.PutUint16(buf[off+4:], 0x0001) // COMPDESK_SUPPORTED
}

func putSurfaceCommands(buf []byte, off int) {
	putCapHeader(buf, off, TypeSurfaceCommands, surfaceCommandsSize)
	// cmdFlags: SETSURFACEBITS(0x02) | FRAMEMARKER(0x10) | STREAMSURFACEBITS(0x40)
	binary.LittleEndian.PutUint32(buf[off+4:], 0x00000052)
	// reserved(4) = 0 (already zero)
}

func putBitmapCodecs(buf []byte, off int) {
	putCapHeader(buf, off, TypeBitmapCodecs, bitmapCodecsSize)
	p := off + 4
	buf[p] = 3 // bitmapCodecCount
	p++

	// Codec 1: NSCodec (codecID=0x01)
	copy(buf[p:], []byte{0xB9, 0x1B, 0x8D, 0xCA, 0x0F, 0x00, 0x4F, 0x15, 0x58, 0x9F, 0xAE, 0x2D, 0x1A, 0x87, 0xE2, 0xD6})
	p += 16
	buf[p] = 0x01 // codecID
	p++
	binary.LittleEndian.PutUint16(buf[p:], 3) // codecPropertiesLength
	p += 2
	buf[p] = 1   // fAllowDynamicFidelity
	buf[p+1] = 1 // fAllowSubsampling
	buf[p+2] = 3 // colorLossLevel
	p += 3

	// Codec 2: RemoteFX (codecID=0x03) — TS_RFX_CLNT_CAPS_CONTAINER (49 bytes)
	copy(buf[p:], []byte{0x12, 0x2F, 0x77, 0x76, 0x72, 0xBD, 0x63, 0x44, 0xAF, 0xB3, 0xB7, 0x3C, 0x9C, 0x6F, 0x78, 0x86})
	p += 16
	buf[p] = 0x03 // codecID = RDP_CODEC_ID_REMOTEFX
	p++
	binary.LittleEndian.PutUint16(buf[p:], 49) // codecPropertiesLength
	p += 2
	// TS_RFX_CLNT_CAPS_CONTAINER
	binary.LittleEndian.PutUint32(buf[p:], 49)   // length
	binary.LittleEndian.PutUint32(buf[p+4:], 1)   // captureFlags = CARDP_CAPS_CAPTURE_NON_CAC
	binary.LittleEndian.PutUint32(buf[p+8:], 37)  // capsLength
	binary.LittleEndian.PutUint16(buf[p+12:], 0xCBC0) // TS_RFX_CAPS blockType = CBY_CAPS
	binary.LittleEndian.PutUint32(buf[p+14:], 8)      // TS_RFX_CAPS blockLen
	binary.LittleEndian.PutUint16(buf[p+18:], 1)      // numCapsets
	binary.LittleEndian.PutUint16(buf[p+20:], 0xCBC1) // TS_RFX_CAPSET blockType = CBY_CAPSET
	binary.LittleEndian.PutUint32(buf[p+22:], 29)     // TS_RFX_CAPSET blockLen
	buf[p+26] = 0x01                                    // codecId = 0x01
	binary.LittleEndian.PutUint16(buf[p+27:], 0xCFC0) // capsetType = CLY_CAPSET
	binary.LittleEndian.PutUint16(buf[p+29:], 2)      // numIcaps
	binary.LittleEndian.PutUint16(buf[p+31:], 8)      // icapLen
	// ICAP 1: RLGR1
	binary.LittleEndian.PutUint16(buf[p+33:], 0x0100) // version = CLW_VERSION_1_0
	binary.LittleEndian.PutUint16(buf[p+35:], 0x0040) // tileSize = CT_TILE_64x64
	buf[p+37] = 0x00                                    // flags (video mode)
	buf[p+38] = 0x01                                    // colConvBits = CLW_COL_CONV_ICT
	buf[p+39] = 0x01                                    // transformBits = CLW_XFORM_DWT_53_A
	buf[p+40] = 0x01                                    // entropyBits = CLW_ENTROPY_RLGR1
	// ICAP 2: RLGR3
	binary.LittleEndian.PutUint16(buf[p+41:], 0x0100) // version
	binary.LittleEndian.PutUint16(buf[p+43:], 0x0040) // tileSize
	buf[p+45] = 0x00                                    // flags
	buf[p+46] = 0x01                                    // colConvBits
	buf[p+47] = 0x01                                    // transformBits
	buf[p+48] = 0x04                                    // entropyBits = CLW_ENTROPY_RLGR3
	p += 49

	// Codec 3: RemoteFX Image (codecID=0x00)
	copy(buf[p:], []byte{0xA6, 0x51, 0x43, 0x9C, 0x35, 0x35, 0xAE, 0x42, 0x91, 0x0C, 0xCD, 0xFC, 0xE5, 0x76, 0x0B, 0x58})
	p += 16
	buf[p] = 0x00 // codecID (image codec)
	p++
	binary.LittleEndian.PutUint16(buf[p:], 4) // codecPropertiesLength
	// 4 bytes of zero properties (already zeroed)
}

func putFrameAcknowledge(buf []byte, off int) {
	putCapHeader(buf, off, TypeFrameAcknowledge, frameAcknowledgeSize)
	binary.LittleEndian.PutUint32(buf[off+4:], 32) // maxUnacknowledgedFrameCount
}

// --- Public encoders: 1 allocation each, for standalone use ---

// EncodeGeneral returns the General capability set (type 0x0001).
func EncodeGeneral() []byte {
	buf := make([]byte, generalSize)
	putGeneral(buf, 0)
	return buf
}

// EncodeBitmap returns the Bitmap capability set (type 0x0002).
func EncodeBitmap(width, height, depth uint16) []byte {
	buf := make([]byte, bitmapSize)
	putBitmap(buf, 0, width, height, depth)
	return buf
}

// EncodeOrder returns the Order capability set (type 0x0003).
func EncodeOrder() []byte {
	buf := make([]byte, orderSize)
	putOrder(buf, 0)
	return buf
}

// EncodeBitmapCacheV2 returns the Bitmap Cache v2 capability set (type 0x0013).
func EncodeBitmapCacheV2() []byte {
	buf := make([]byte, bitmapCacheV2Size)
	putBitmapCacheV2(buf, 0)
	return buf
}

// EncodePointer returns the Pointer capability set (type 0x0008).
func EncodePointer() []byte {
	buf := make([]byte, pointerSize)
	putPointer(buf, 0)
	return buf
}

// EncodeInput returns the Input capability set (type 0x000D).
func EncodeInput() []byte {
	buf := make([]byte, inputSize)
	putInput(buf, 0)
	return buf
}

// EncodeBrush returns the Brush capability set (type 0x000F).
func EncodeBrush() []byte {
	buf := make([]byte, brushSize)
	putBrush(buf, 0)
	return buf
}

// EncodeGlyphCache returns the Glyph Cache capability set (type 0x0010).
func EncodeGlyphCache() []byte {
	buf := make([]byte, glyphCacheSize)
	putGlyphCache(buf, 0)
	return buf
}

// EncodeOffscreenBitmapCache returns the Offscreen Bitmap Cache capability set (type 0x001E).
func EncodeOffscreenBitmapCache() []byte {
	buf := make([]byte, offscreenBitmapCacheSize)
	putOffscreenBitmapCache(buf, 0)
	return buf
}

// EncodeVirtualChannel returns the Virtual Channel capability set (type 0x0014).
func EncodeVirtualChannel() []byte {
	buf := make([]byte, virtualChannelSize)
	putVirtualChannel(buf, 0)
	return buf
}

// EncodeSound returns the Sound capability set (type 0x000C).
func EncodeSound() []byte {
	buf := make([]byte, soundSize)
	putSound(buf, 0, true)
	return buf
}

// EncodeBitmapCodecs returns the Bitmap Codecs capability set (type 0x001D).
func EncodeBitmapCodecs() []byte {
	buf := make([]byte, bitmapCodecsSize)
	putBitmapCodecs(buf, 0)
	return buf
}

// serverHasCap returns true if the server advertised the given capability type.
func serverHasCap(serverCaps uint32, capType uint16) bool {
	return capType < 32 && serverCaps&(1<<capType) != 0
}

// BuildConfirmCapabilities builds the combined capability set data for a
// Confirm Active PDU. Single allocation for all capability sets.
// serverCaps is a bitfield of cap types the server advertised in Demand Active;
// conditional caps (MultifragUpdate, LargePointer, CompDesk, OffscreenBitmapCache,
// and GFX caps) are only included when the server also advertises them.
func BuildConfirmCapabilities(width, height, depth uint16, gfx bool, serverCaps uint32, soundBeeps bool) ([]byte, uint16) {
	size := baseCapsSize
	count := uint16(capsCountBase)

	// Conditional caps: echo only if server advertised them
	hasMultifrag := serverHasCap(serverCaps, TypeMultifragUpdate)
	hasLargePtr := serverHasCap(serverCaps, TypeLargePointer)
	hasCompDesk := serverHasCap(serverCaps, TypeCompDesk)
	hasOffscreen := serverHasCap(serverCaps, TypeOffscreenBitmapCache)

	if hasMultifrag {
		size += multifragUpdateSize
		count++
	}
	if hasLargePtr {
		size += largePointerSize
		count++
	}
	if hasCompDesk {
		size += compDeskSize
		count++
	}
	if hasOffscreen {
		size += offscreenBitmapCacheSize
		count++
	}

	// GFX caps: only if gfx=true AND server advertised each one
	hasSurfCmd := gfx && serverHasCap(serverCaps, TypeSurfaceCommands)
	hasCodecs := gfx && serverHasCap(serverCaps, TypeBitmapCodecs)
	hasFrameAck := gfx && serverHasCap(serverCaps, TypeFrameAcknowledge)
	if hasSurfCmd {
		size += surfaceCommandsSize
		count++
	}
	if hasCodecs {
		size += bitmapCodecsSize
		count++
	}
	if hasFrameAck {
		size += frameAcknowledgeSize
		count++
	}

	buf := make([]byte, size)
	off := 0

	// 15 base caps (always sent)
	putGeneral(buf, off)
	off += generalSize
	putBitmap(buf, off, width, height, depth)
	off += bitmapSize
	putOrder(buf, off)
	off += orderSize
	putBitmapCacheV2(buf, off)
	off += bitmapCacheV2Size
	putPointer(buf, off)
	off += pointerSize
	putInput(buf, off)
	off += inputSize
	putBrush(buf, off)
	off += brushSize
	putGlyphCache(buf, off)
	off += glyphCacheSize
	putVirtualChannel(buf, off)
	off += virtualChannelSize
	putSound(buf, off, soundBeeps)
	off += soundSize
	putControl(buf, off)
	off += controlSize
	putActivation(buf, off)
	off += activationSize
	putShare(buf, off)
	off += shareSize
	putColorCache(buf, off)
	off += colorCacheSize
	putFont(buf, off)
	off += fontSize

	// Conditional caps
	if hasMultifrag {
		putMultifragUpdate(buf, off)
		off += multifragUpdateSize
	}
	if hasLargePtr {
		putLargePointer(buf, off)
		off += largePointerSize
	}
	if hasCompDesk {
		putCompDesk(buf, off)
		off += compDeskSize
	}
	if hasOffscreen {
		putOffscreenBitmapCache(buf, off)
		off += offscreenBitmapCacheSize
	}

	// GFX caps
	if hasSurfCmd {
		putSurfaceCommands(buf, off)
		off += surfaceCommandsSize
	}
	if hasCodecs {
		putBitmapCodecs(buf, off)
		off += bitmapCodecsSize
	}
	if hasFrameAck {
		putFrameAcknowledge(buf, off)
	}
	return buf, count
}

// DecodeCapabilitySets parses raw capability set data into individual sets.
func DecodeCapabilitySets(log *slog.Logger, data []byte, count uint16) ([]CapabilitySet, error) {
	sets := make([]CapabilitySet, 0, count)
	off := 0
	for i := uint16(0); i < count; i++ {
		if off+4 > len(data) {
			return sets, nil // allow partial parse
		}
		capType := binary.LittleEndian.Uint16(data[off : off+2])
		capLen := int(binary.LittleEndian.Uint16(data[off+2 : off+4]))
		if capLen < 4 || off+capLen > len(data) {
			return sets, nil // allow partial parse
		}
		sets = append(sets, CapabilitySet{
			Type:    capType,
			Payload: data[off+4 : off+capLen], // sub-slice, no copy
		})
		log.LogAttrs(context.Background(), slog.LevelDebug, "capability set", util.Hex4("type", capType), slog.Int("len", capLen))
		off += capLen
	}
	return sets, nil
}
