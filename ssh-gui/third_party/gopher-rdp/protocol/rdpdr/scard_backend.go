// Smartcard backend interface for MS-RDPESC (Remote Desktop Protocol:
// Smart Card Virtual Channel Extension).
//
// ScardBackend abstracts the platform-specific smartcard API so the RDPDR
// smartcard device can be tested with a mock and run against pcsclite on
// Linux/macOS.

package rdpdr

// ScardBackend is the interface that smartcard backends must implement.
// Each method corresponds to a PC/SC function. Return codes use Windows
// SCARD error codes (SCARD_S_SUCCESS = 0, SCARD_E_xxx, etc.).
type ScardBackend interface {
	// EstablishContext creates a new resource manager context.
	EstablishContext(scope uint32) (ctx uint32, rc uint32)

	// ReleaseContext releases a previously established context.
	ReleaseContext(ctx uint32) uint32

	// IsValidContext checks whether a context is still valid.
	IsValidContext(ctx uint32) uint32

	// ListReaders returns the list of readers within the given groups.
	// readers is a multi-string (null-separated, double-null terminated).
	ListReaders(ctx uint32, groups []byte) (readers []byte, rc uint32)

	// GetStatusChange blocks until the reader state changes or timeout expires.
	GetStatusChange(ctx uint32, timeout uint32, states []ScardReaderState) ([]ScardReaderState, uint32)

	// Connect establishes a connection to the specified reader.
	Connect(ctx uint32, reader string, shareMode, preferredProtocol uint32) (handle uint32, activeProtocol uint32, rc uint32)

	// Disconnect terminates a connection to the reader.
	Disconnect(handle uint32, disposition uint32) uint32

	// Reconnect re-establishes an existing connection.
	Reconnect(handle uint32, shareMode, preferredProtocol, disposition uint32) (activeProtocol uint32, rc uint32)

	// BeginTransaction starts a transaction.
	BeginTransaction(handle uint32) uint32

	// EndTransaction ends a transaction.
	EndTransaction(handle uint32, disposition uint32) uint32

	// Status returns the current status of the reader.
	Status(handle uint32) (reader string, state, protocol uint32, atr []byte, rc uint32)

	// Transmit sends an APDU to the card.
	Transmit(handle uint32, sendPCI []byte, sendBuf []byte) (recvPCI, recvBuf []byte, rc uint32)

	// Control sends a control command to the reader.
	Control(handle uint32, controlCode uint32, inBuf []byte) (outBuf []byte, rc uint32)

	// GetAttrib retrieves an attribute of the reader.
	GetAttrib(handle uint32, attrID uint32) (attr []byte, rc uint32)

	// Cancel cancels all outstanding operations on the context.
	Cancel(ctx uint32) uint32

	// Close shuts down the backend and releases all resources.
	Close() error
}

// ScardReaderState holds the state of a single reader for GetStatusChange.
type ScardReaderState struct {
	Reader       string
	CurrentState uint32
	EventState   uint32
	ATR          []byte
}

// SCARD return codes (Windows values, MS-RDPESC 2.2.5)
const (
	ScardSuccess              uint32 = 0x00000000
	ScardFInternalError       uint32 = 0x80100001
	ScardECancelled           uint32 = 0x80100002
	ScardEInvalidHandle       uint32 = 0x80100003
	ScardEInvalidParameter    uint32 = 0x80100004
	ScardEInvalidTarget       uint32 = 0x80100005
	ScardENoMemory            uint32 = 0x80100006
	ScardFWaitedTooLong       uint32 = 0x80100007
	ScardEInsufficientBuffer  uint32 = 0x80100008
	ScardEUnknownReader       uint32 = 0x80100009
	ScardETimeout             uint32 = 0x8010000A
	ScardESharingViolation    uint32 = 0x8010000B
	ScardENoSmartcard         uint32 = 0x8010000C
	ScardEUnknownCard         uint32 = 0x8010000D
	ScardECantDispose         uint32 = 0x8010000E
	ScardEProtoMismatch       uint32 = 0x8010000F
	ScardENotReady            uint32 = 0x80100010
	ScardEInvalidValue        uint32 = 0x80100011
	ScardESystemCancelled     uint32 = 0x80100012
	ScardFCommError           uint32 = 0x80100013
	ScardFUnknownError        uint32 = 0x80100014
	ScardEInvalidAtr          uint32 = 0x80100015
	ScardENotTransacted       uint32 = 0x80100016
	ScardEReaderUnavailable   uint32 = 0x80100017
	ScardENoService           uint32 = 0x8010001D
	ScardEServiceStopped      uint32 = 0x8010001E
	ScardENoReadersAvailable  uint32 = 0x8010002E
)

// SCARD reader state flags
const (
	ScardStateUnaware    uint32 = 0x00000000
	ScardStateIgnore     uint32 = 0x00000001
	ScardStateChanged    uint32 = 0x00000002
	ScardStateUnknown    uint32 = 0x00000004
	ScardStateUnavail    uint32 = 0x00000008
	ScardStateEmpty      uint32 = 0x00000010
	ScardStatePresent    uint32 = 0x00000020
	ScardStateAtrMatch   uint32 = 0x00000040
	ScardStateExclusive  uint32 = 0x00000080
	ScardStateInuse      uint32 = 0x00000100
	ScardStateMute       uint32 = 0x00000200
	ScardStateUnpowered  uint32 = 0x00000400
)

// SCARD context scope values
const (
	ScardScopeUser     uint32 = 0x00000000
	ScardScopeTerminal uint32 = 0x00000001
	ScardScopeSystem   uint32 = 0x00000002
)

// SCARD share mode values
const (
	ScardShareExclusive uint32 = 0x00000001
	ScardShareShared    uint32 = 0x00000002
	ScardShareDirect    uint32 = 0x00000003
)

// SCARD protocol values
const (
	ScardProtocolUndefined uint32 = 0x00000000
	ScardProtocolT0        uint32 = 0x00000001
	ScardProtocolT1        uint32 = 0x00000002
	ScardProtocolRaw       uint32 = 0x00010000
)

// SCARD disposition values
const (
	ScardLeaveCard   uint32 = 0x00000000
	ScardResetCard   uint32 = 0x00000001
	ScardUnpowerCard uint32 = 0x00000002
	ScardEjectCard   uint32 = 0x00000003
)
