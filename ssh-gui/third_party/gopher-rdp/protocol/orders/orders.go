// Package orders decodes RDP Drawing Orders (MS-RDPEGDI sections 2.2.2)
// and renders glyph-based text into BGRX pixel buffers.
package orders

// Control flags (controlFlags byte, MS-RDPEGDI 2.2.2.2.1.1.2)
const (
	TSStandard         byte = 0x01
	TSSecondary        byte = 0x02
	TSBounds           byte = 0x04
	TSTypeChange       byte = 0x08
	TSDeltaCoordinates byte = 0x10
	TSZeroBoundsDelta  byte = 0x20 // reuse last bounds (don't parse new ones from stream)
	TSZeroFieldByte0   byte = 0x40 // MSB of fieldFlags is zero → read one fewer byte
	TSZeroFieldByte1   byte = 0x80 // two MSBs are zero → read two fewer bytes (combined with bit0)
)

// Primary order types (MS-RDPEGDI 2.2.2.1.1.2)
const (
	OrderDstBlt       byte = 0x00
	OrderPatBlt       byte = 0x01
	OrderScrBlt       byte = 0x02
	OrderLineTo       byte = 0x09
	OrderOpaqueRect   byte = 0x0A
	OrderMemBlt       byte = 0x0D
	OrderMem3Blt      byte = 0x0E
	OrderSaveBitmap byte = 0x0B
	OrderGlyphIndex byte = 0x1B
	OrderFastIndex  byte = 0x13
	OrderPolygonSC       byte = 0x14
	OrderPolygonCB       byte = 0x15
	OrderFastGlyph       byte = 0x18
	OrderPolyline        byte = 0x16
	OrderEllipseSC       byte = 0x19
	OrderEllipseCB       byte = 0x1A
)

// Secondary order types (MS-RDPEGDI 2.2.2.2.1)
const (
	SecondaryBitmapUncompressed byte = 0x00
	SecondaryCacheColorTable    byte = 0x01
	SecondaryCacheBitmap        byte = 0x02
	SecondaryCacheGlyph         byte = 0x03
	SecondaryBitmapUncompV2     byte = 0x04
	SecondaryBitmapCompV2       byte = 0x05
	SecondaryCacheBrush         byte = 0x07
	SecondaryBitmapCompV3       byte = 0x08
)

// orderSupport array indices (MS-RDPBCGR 2.2.7.1.3)
const (
	OrderSupportDstBlt      = 0x00
	OrderSupportPatBlt      = 0x01
	OrderSupportScrBlt      = 0x02
	OrderSupportMemBlt      = 0x03
	OrderSupportMem3Blt     = 0x04
	OrderSupportLineTo      = 0x08
	OrderSupportOpaqueRect  = 0x0A
	OrderSupportSaveBitmap  = 0x0B
	OrderSupportFastIndex  = 0x13
	OrderSupportPolygonSC  = 0x14
	OrderSupportPolygonCB  = 0x15
	OrderSupportPolyline   = 0x16
	OrderSupportFastGlyph  = 0x18
	OrderSupportEllipseSC  = 0x19
	OrderSupportEllipseCB  = 0x1A
	OrderSupportGlyphIndex = 0x1B
)

// Number of field-flag bits per primary order type.
var orderFieldCount = [0x1C]byte{
	OrderDstBlt:          5,
	OrderPatBlt:          12,
	OrderScrBlt:          7,
	OrderLineTo:          10,
	OrderOpaqueRect:      7,
	OrderSaveBitmap:      6,
	OrderMemBlt:          9,
	OrderMem3Blt:         17, // bits 0-16; 3 flag bytes (bit 16 = Unknown field)
	OrderFastIndex: 15,
	OrderPolygonSC:       7,
	OrderPolygonCB:       13,
	OrderPolyline:        7,
	OrderFastGlyph:       15,
	OrderEllipseSC:       7,
	OrderEllipseCB:       13,
	OrderGlyphIndex:      22,
}

// Bounds holds a clipping rectangle for bounded orders.
type Bounds struct {
	Left, Top, Right, Bottom int16
}

// Order is delivered to the callback for each decoded order.
type Order struct {
	// Primary fields
	Type       byte // primary order type (OrderGlyphIndex, etc.)
	FieldFlags uint32
	HasBounds  bool // true when TS_BOUNDS was set — apply state.Bounds as clip rect

	// Secondary fields (valid when IsSecondary is true)
	IsSecondary   bool
	SecondaryType byte
	ExtraFlags    uint16 // secondary order extraFlags (e.g. cacheId for CacheBitmapV2)
	SecData       []byte // sub-slice of input (no copy)
}

// OrderCallback is called for each decoded order.
type OrderCallback func(state *DecoderState, ord *Order)
