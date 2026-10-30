package orders

// GlyphIndexState holds the stateful fields for the GlyphIndex primary order
// (MS-RDPEGDI 2.2.2.2.1.2.5). The server only sends changed fields.
type GlyphIndexState struct {
	CacheID      uint8
	FlAccel      uint8
	UlCharInc    uint8
	FOpRedundant uint8
	BackColor    uint32 // 3-byte RGB packed into low 24 bits
	ForeColor    uint32
	BkLeft       int16
	BkTop        int16
	BkRight      int16
	BkBottom     int16
	OpLeft       int16
	OpTop        int16
	OpRight      int16
	OpBottom     int16
	BrushOrgX    uint8
	BrushOrgY    uint8
	BrushStyle   uint8
	BrushHatch   uint8
	BrushExtra   [7]byte
	X            int16
	Y            int16
	VarBytes     [256]byte // fixed array, no alloc
	VarLen       uint8
}

// FastIndexState holds the stateful fields for the FastIndex primary order
// (MS-RDPEGDI 2.2.2.2.1.2.3).
type FastIndexState struct {
	CacheID   uint8
	FDrawing  uint16 // low byte=ulCharInc, high byte=flAccel
	BackColor uint32
	ForeColor uint32
	BkLeft    int16
	BkTop     int16
	BkRight   int16
	BkBottom  int16
	OpLeft    int16
	OpTop     int16
	OpRight   int16
	OpBottom  int16
	X         int16
	Y         int16
	VarBytes  [256]byte
	VarLen    uint8
}

// DstBltState holds stateful fields for DstBlt (MS-RDPEGDI 2.2.2.2.1.1.2.1).
// 5 fields, 1 flag byte.
type DstBltState struct {
	Left, Top int16
	Width, Height int16
	Rop uint8 // ROP3 operation (high byte)
}

// PatBltState holds stateful fields for PatBlt (MS-RDPEGDI 2.2.2.2.1.1.2.3).
// 12 fields, 2 flag bytes.
type PatBltState struct {
	Left, Top int16
	Width, Height int16
	Rop uint8
	BackColor, ForeColor uint32
	BrushOrgX, BrushOrgY uint8
	BrushStyle, BrushHatch uint8
	BrushExtra [7]byte
}

// ScrBltState holds stateful fields for ScrBlt (MS-RDPEGDI 2.2.2.2.1.1.2.4).
// 7 fields, 1 flag byte.
type ScrBltState struct {
	Left, Top int16
	Width, Height int16
	Rop uint8
	SrcLeft, SrcTop int16
}

// OpaqueRectState holds stateful fields for OpaqueRect (MS-RDPEGDI 2.2.2.2.1.1.2.5).
// 7 fields, 1 flag byte.
type OpaqueRectState struct {
	Left, Top int16
	Width, Height int16
	ColorR, ColorG, ColorB uint8
}

// MemBltState holds stateful fields for MemBlt (MS-RDPEGDI 2.2.2.2.1.1.2.9).
// 9 fields, 2 flag bytes.
type MemBltState struct {
	CacheID uint16
	Left, Top int16
	Width, Height int16
	Rop uint8
	SrcLeft, SrcTop int16
	CacheIndex uint16
}

// LineToState holds stateful fields for LineTo (MS-RDPEGDI 2.2.2.2.1.1.2.1).
// 10 fields, 2 flag bytes.
type LineToState struct {
	BackMode       uint16 // 1=TRANSPARENT, 2=OPAQUE
	StartX, StartY int16
	EndX, EndY     int16
	BackColor      uint32
	Rop2           uint8
	PenStyle       uint8
	PenWidth       uint8
	PenColor       uint32
}

// Mem3BltState holds stateful fields for Mem3Blt (MS-RDPEGDI 2.2.2.2.1.1.2.10).
// 17 fields (bits 0-16), 3 flag bytes.
type Mem3BltState struct {
	CacheID              uint16
	Left, Top            int16
	Width, Height        int16
	Rop                  uint8
	SrcLeft, SrcTop      int16
	BackColor, ForeColor uint32
	BrushOrgX, BrushOrgY uint8
	BrushStyle           uint8
	BrushHatch           uint8
	BrushExtra           [7]byte
	CacheIndex           uint16
	Unknown              uint16 // field 16
}

// PolylineState holds stateful fields for Polyline (MS-RDPEGDI 2.2.2.2.1.1.2.2).
// 7 fields, 1 flag byte.
type PolylineState struct {
	StartX, StartY  int16
	Rop2            uint8
	BrushCacheEntry uint16
	PenColor        uint32
	NumDeltaEntries uint8
	VarBytes        [256]byte
	VarLen          uint8
}

// EllipseSCState holds stateful fields for EllipseSC (MS-RDPEGDI 2.2.2.2.1.1.2.8).
// 7 fields, 1 flag byte.
type EllipseSCState struct {
	Left, Top, Right, Bottom int16
	Rop2                     uint8
	FillMode                 uint8
	Color                    uint32
}

// PolygonSCState holds stateful fields for PolygonSC (MS-RDPEGDI 2.2.2.2.1.1.2.16).
// 7 fields, 1 flag byte.
type PolygonSCState struct {
	X, Y            int16
	Rop2            uint8
	FillMode        uint8
	Color           uint32
	NumDeltaEntries uint8
	VarBytes        [256]byte
	VarLen          uint8
}

// PolygonCBState holds stateful fields for PolygonCB (MS-RDPEGDI 2.2.2.2.1.1.2.17).
// 13 fields, 2 flag bytes.
type PolygonCBState struct {
	X, Y                    int16
	Rop2                    uint8
	FillMode                uint8
	BackColor, ForeColor    uint32
	BrushOrgX, BrushOrgY    uint8
	BrushStyle, BrushHatch  uint8
	BrushExtra              [7]byte
	NumDeltaEntries         uint8
	VarBytes                [256]byte
	VarLen                  uint8
}

// EllipseCBState holds stateful fields for EllipseCB (MS-RDPEGDI 2.2.2.2.1.1.2.19).
// 13 fields, 2 flag bytes.
type EllipseCBState struct {
	Left, Top, Right, Bottom int16
	Rop2                     uint8
	FillMode                 uint8
	BackColor, ForeColor     uint32
	BrushOrgX, BrushOrgY     uint8
	BrushStyle, BrushHatch   uint8
	BrushExtra               [7]byte
}

// SaveBitmapState holds stateful fields for SaveBitmap/DesktopSave
// (MS-RDPEGDI 2.2.2.2.1.1.2.11). 6 fields, 1 flag byte.
type SaveBitmapState struct {
	Offset              uint32
	Left, Top           int16
	Right, Bottom       int16
	Action              uint8 // 0=save, 1=restore
}

// DecoderState holds persistent state across order decoding calls.
// Primary orders are stateful — the server only sends changed fields,
// and the last values are reused for omitted fields.
type DecoderState struct {
	LastOrderType byte
	Bounds        Bounds
	GlyphIndex    GlyphIndexState
	FastIndex     FastIndexState
	FastGlyph     FastIndexState // same field layout as FastIndex
	DstBlt        DstBltState
	PatBlt        PatBltState
	ScrBlt        ScrBltState
	OpaqueRect OpaqueRectState
	MemBlt     MemBltState
	Mem3Blt       Mem3BltState
	LineTo        LineToState
	Polyline      PolylineState
	PolygonSC     PolygonSCState
	PolygonCB     PolygonCBState
	EllipseSC     EllipseSCState
	EllipseCB     EllipseCBState
	SaveBitmap       SaveBitmapState
	DebugBailReason  string // temporary: why DecodeOrders returned early
}
