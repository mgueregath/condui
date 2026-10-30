// Package rdpdr implements the MS-RDPEFS (Remote Desktop Protocol: File System
// Virtual Channel Extension) for redirecting local disk drives to an RDP server.
//
// The handler manages the RDPDR static virtual channel initialization sequence
// and dispatches I/O requests to registered disk devices.
package rdpdr

import (
	"context"
	"encoding/binary"
	"fmt"
	"log/slog"
	"sync"
	"unicode/utf16"
)

// Shared header Component values (MS-RDPEFS 2.2.1.1)
const (
	ComponentRDPDR uint16 = 0x4472 // "Dr" — device redirection
	ComponentPRTYP uint16 = 0x5052 // "PR" — printing
)

// Shared header PacketID values (MS-RDPEFS 2.2.1.1)
const (
	PakIDServerAnnounce     uint16 = 0x496E // Server Announce Request
	PakIDClientIDConfirm    uint16 = 0x4343 // Client Announce Reply / Server Client ID Confirm (same ID)
	PakIDClientNameReq      uint16 = 0x434E // Client Name Request
	PakIDServerCoreCap      uint16 = 0x5350 // Server Core Capability Request
	PakIDClientCoreCap      uint16 = 0x4350 // Client Core Capability Response
	PakIDDeviceListAnnounce uint16 = 0x4441 // Client Device List Announce
	PakIDDeviceReply        uint16 = 0x6472 // Server Device Announce Response
	PakIDDeviceIORequest    uint16 = 0x4952 // Server Device I/O Request
	PakIDDeviceIOCompletion uint16 = 0x4943 // Client Device I/O Completion
	PakIDDeviceListRemove   uint16 = 0x444D // Client Device List Remove
	PakIDUserLoggedOn       uint16 = 0x554C // Server User Logged On

	// MS-RDPEPC 2.2.2.3 — Printer Component PacketID
	PakIDPrnCacheData uint16 = 0x5043 // Server Printer Cache Data
)

// Device types (MS-RDPEFS 2.2.1.3)
const (
	DeviceTypeSerial    uint32 = 0x00000001
	DeviceTypeParallel  uint32 = 0x00000002
	DeviceTypePrinter   uint32 = 0x00000004
	DeviceTypeDisk      uint32 = 0x00000008
	DeviceTypeSmartcard uint32 = 0x00000020
)

// invalidPortHandle is the sentinel value for a closed port handle.
// On Unix this equals -1 (as uintptr); on Windows it equals INVALID_HANDLE_VALUE.
const invalidPortHandle = ^uintptr(0)

// Device is the interface that all redirected devices must implement.
type Device interface {
	ID() uint32
	Type() uint32
	Name() string
	HandleIRP(h *Handler, req *IORequest)
}

// Capability types (MS-RDPEFS 2.2.2.1)
const (
	CapGeneralType uint16 = 0x0001
	CapPrinterType uint16 = 0x0002
	CapPortType    uint16 = 0x0003
	CapDriveType   uint16 = 0x0004
	CapSmartCard   uint16 = 0x0005
)

// General capability version (MS-RDPEFS 2.2.2.7.1)
const (
	GeneralCapVersion1 uint32 = 0x00000001
	GeneralCapVersion2 uint32 = 0x00000002
)

// I/O code 1 flags (MS-RDPEFS 2.2.2.7.1)
const (
	IOCode1FirstRDPDR  uint32 = 0x00000001
	IOCode1ReadWrite   uint32 = 0x00000002
	IOCode1DeviceCtl   uint32 = 0x00000004
	IOCode1QueryVolume uint32 = 0x00000008
	IOCode1QueryInfo   uint32 = 0x00000010
	IOCode1SetInfo     uint32 = 0x00000020
	IOCode1QueryDir    uint32 = 0x00000040
	IOCode1NotifyDir   uint32 = 0x00000080
	IOCode1LockCtl     uint32 = 0x00000100
)

// Extra flags 1 (MS-RDPEFS 2.2.2.7.1)
const (
	ExtraFlags1AllowUsers uint32 = 0x00000001
)

// IRP major function codes (MS-RDPEFS 2.2.1.4)
const (
	IrpCreate        uint32 = 0x00000000 // IRP_MJ_CREATE
	IrpClose         uint32 = 0x00000002 // IRP_MJ_CLOSE
	IrpRead          uint32 = 0x00000003 // IRP_MJ_READ
	IrpWrite         uint32 = 0x00000004 // IRP_MJ_WRITE
	IrpQueryInfo     uint32 = 0x00000005 // IRP_MJ_QUERY_INFORMATION
	IrpSetInfo       uint32 = 0x00000006 // IRP_MJ_SET_INFORMATION
	IrpQueryVolume   uint32 = 0x0000000A // IRP_MJ_QUERY_VOLUME_INFORMATION
	IrpSetVolume     uint32 = 0x0000000B // IRP_MJ_SET_VOLUME_INFORMATION
	IrpDirControl    uint32 = 0x0000000C // IRP_MJ_DIRECTORY_CONTROL
	IrpDeviceControl uint32 = 0x0000000E // IRP_MJ_DEVICE_CONTROL
	IrpLockControl   uint32 = 0x00000011 // IRP_MJ_LOCK_CONTROL
)

// IRP minor function codes
const (
	IrpMnQueryDir  uint32 = 0x00000001
	IrpMnNotifyDir uint32 = 0x00000002
)

// NTSTATUS codes (subset used by MS-RDPEFS)
const (
	StatusSuccess           uint32 = 0x00000000
	StatusNoMoreFiles       uint32 = 0x80000006
	StatusAccessDenied      uint32 = 0xC0000022
	StatusObjectNameInvalid uint32 = 0xC0000033
	StatusObjectNameNotFound uint32 = 0xC0000034
	StatusObjectNameCollision uint32 = 0xC0000035
	StatusNoSuchFile        uint32 = 0xC000000F
	StatusNotADirectory     uint32 = 0xC0000103
	StatusFileIsADirectory  uint32 = 0xC00000BA
	StatusNotSupported      uint32 = 0xC00000BB
	StatusDirectoryNotEmpty uint32 = 0xC0000101
	StatusInvalidParameter     uint32 = 0xC000000D
	StatusInvalidDeviceRequest uint32 = 0xC0000010
	StatusNotAReparsePoint     uint32 = 0xC0000275
	StatusObjectPathNotFound uint32 = 0xC000003A
	StatusDiskFull          uint32 = 0xC000007F
	StatusEndOfFile         uint32 = 0xC0000011
	StatusUnsuccessful         uint32 = 0xC0000001
	StatusTooManyOpenedFiles   uint32 = 0xC000011F
)

// Create disposition values (MS-SMB2 2.2.13)
const (
	FileSupersede  uint32 = 0x00000000
	FileOpen       uint32 = 0x00000001
	FileCreate     uint32 = 0x00000002
	FileOpenIf     uint32 = 0x00000003
	FileOverwrite  uint32 = 0x00000004
	FileOverwriteIf uint32 = 0x00000005
)

// Create options (MS-SMB2 2.2.13)
const (
	FileDirectoryFile    uint32 = 0x00000001
	FileNonDirectoryFile uint32 = 0x00000040
	FileDeleteOnClose    uint32 = 0x00001000
)

// File information classes (MS-FSCC)
const (
	FileDirectoryInformation      uint32 = 1
	FileFullDirectoryInformation  uint32 = 2
	FileBothDirectoryInformation  uint32 = 3
	FileBasicInformation          uint32 = 4
	FileStandardInformation       uint32 = 5
	FileRenameInformation         uint32 = 10
	FileNamesInformation          uint32 = 12
	FileDispositionInformation    uint32 = 13
	FileEndOfFileInformation      uint32 = 20
	FileAttributeTagInformation   uint32 = 35
)

// Volume information classes (MS-FSCC)
const (
	FileFsVolumeInformation     uint32 = 1
	FileFsSizeInformation       uint32 = 3
	FileFsAttributeInformation  uint32 = 5
	FileFsFullSizeInformation   uint32 = 7
	FileFsDeviceInformation     uint32 = 4
)

// File attributes (MS-FSCC 2.6)
const (
	FileAttrReadOnly  uint32 = 0x00000001
	FileAttrHidden    uint32 = 0x00000002
	FileAttrDirectory uint32 = 0x00000010
	FileAttrNormal    uint32 = 0x00000080
	FileAttrArchive   uint32 = 0x00000020
)

// Create information values returned in IO completion
const (
	FileSuperseded uint8 = 0x00
	FileOpened     uint8 = 0x01
	FileCreated    uint8 = 0x02
	FileOverwritten uint8 = 0x03
)

// IORequest holds the parsed fields of a DR_DEVICE_IOREQUEST.
type IORequest struct {
	DeviceID     uint32
	FileID       uint32
	CompletionID uint32
	MajorFn      uint32
	MinorFn      uint32
	Payload      []byte // remaining bytes after the 20-byte header
}

// Handler manages the RDPDR static virtual channel.
type Handler struct {
	sendFn       func([]byte) error
	log          *slog.Logger
	clientID     uint32
	versionMajor uint16
	versionMinor uint16
	computerName string
	userLoggedOn     bool   // true after Server User Logged On received
	gotServerCap     bool   // true after Server Core Capability Request received
	gotIDConfirm     bool   // true after Server Client ID Confirm received
	serverCapGenVer  uint32 // server's general capability version (1 or 2)
	serverIOCode1    uint32 // server's ioCode1 from general capability
	mu           sync.Mutex
	devices      map[uint32]Device
}

// NewHandler creates an RDPDR handler.
// sendFn writes data to the "rdpdr" static virtual channel.
func NewHandler(sendFn func([]byte) error, log *slog.Logger, computerName string) *Handler {
	return &Handler{
		sendFn:       sendFn,
		log:          log,
		computerName: computerName,
		devices:      make(map[uint32]Device),
	}
}

// AddDrive registers a disk device for redirection.
func (h *Handler) AddDrive(deviceID uint32, name, localPath string, readOnly bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.devices[deviceID] = NewDiskDevice(deviceID, name, localPath, readOnly, h.log)
}

// AddSerial registers a serial port device for redirection.
func (h *Handler) AddSerial(deviceID uint32, name, path string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.devices[deviceID] = NewSerialDevice(deviceID, name, path, h.log)
}

// AddParallel registers a parallel port device for redirection.
func (h *Handler) AddParallel(deviceID uint32, name, path string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.devices[deviceID] = NewParallelDevice(deviceID, name, path, h.log)
}

// AddPrinter registers a printer device for redirection.
func (h *Handler) AddPrinter(deviceID uint32, name, driverName, outputDir, ippURL string, isDefault bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.devices[deviceID] = NewPrinterDevice(deviceID, name, driverName, outputDir, ippURL, isDefault, h.log)
}

// AddSmartcard registers a smartcard device for redirection.
func (h *Handler) AddSmartcard(deviceID uint32, backend ScardBackend) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.devices[deviceID] = NewSmartcardDevice(deviceID, backend, h.log)
}

// Close releases resources held by RDPDR devices. In particular, it cancels
// outstanding blocking smartcard operations so their goroutines can exit.
func (h *Handler) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, dev := range h.devices {
		if sc, ok := dev.(*SmartcardDevice); ok {
			sc.Close()
		}
	}
}

// ProcessPDU dispatches an incoming RDPDR PDU based on the shared header.
func (h *Handler) ProcessPDU(data []byte) {
	if len(data) < 4 {
		h.log.LogAttrs(context.Background(), slog.LevelError, "PDU too short", slog.Int("len", len(data)))
		return
	}

	component := binary.LittleEndian.Uint16(data[0:2])
	packetID := binary.LittleEndian.Uint16(data[2:4])
	body := data[4:]

	switch component {
	case ComponentRDPDR:
		switch packetID {
		case PakIDServerAnnounce:
			h.handleServerAnnounce(body)
		case PakIDServerCoreCap:
			h.handleServerCoreCap(body)
		case PakIDClientIDConfirm:
			h.handleClientIDConfirm(body)
		case PakIDDeviceReply:
			h.handleDeviceReply(body)
		case PakIDDeviceIORequest:
			h.handleIORequest(body)
		case PakIDUserLoggedOn:
			h.handleUserLoggedOn()
		default:
			h.log.LogAttrs(context.Background(), slog.LevelWarn, "unhandled packet ID",
				slog.Int("packetID", int(packetID)))
		}
	case ComponentPRTYP:
		h.handlePrinterComponent(packetID, body)
	default:
		h.log.LogAttrs(context.Background(), slog.LevelWarn, "unexpected component",
			slog.Int("component", int(component)))
	}
}

// handleServerAnnounce processes Server Announce Request and sends Client Announce Reply + Client Name.
func (h *Handler) handleServerAnnounce(body []byte) {
	if len(body) < 8 {
		h.log.LogAttrs(context.Background(), slog.LevelError, "Server Announce too short")
		return
	}

	h.versionMajor = binary.LittleEndian.Uint16(body[0:2])
	h.versionMinor = binary.LittleEndian.Uint16(body[2:4])
	h.clientID = binary.LittleEndian.Uint32(body[4:8])

	h.log.LogAttrs(context.Background(), slog.LevelDebug, "Server Announce",
		slog.Int("versionMajor", int(h.versionMajor)),
		slog.Int("versionMinor", int(h.versionMinor)),
		slog.Int("clientID", int(h.clientID)))

	// Send Client Announce Reply (version 1.13)
	reply := make([]byte, 12)
	binary.LittleEndian.PutUint16(reply[0:2], ComponentRDPDR)
	binary.LittleEndian.PutUint16(reply[2:4], PakIDClientIDConfirm)
	binary.LittleEndian.PutUint16(reply[4:6], 1)  // versionMajor
	binary.LittleEndian.PutUint16(reply[6:8], 0x000C) // versionMinor = RDP6X (max supported)
	binary.LittleEndian.PutUint32(reply[8:12], h.clientID)

	if err := h.sendFn(reply); err != nil {
		h.log.LogAttrs(context.Background(), slog.LevelError, "send announce reply failed",
			slog.Any("err", err))
		return
	}

	// Send Client Name Request (Unicode)
	h.sendClientName()
}

// sendClientName sends the DR_CORE_CLIENT_NAME_REQ PDU.
func (h *Handler) sendClientName() {
	nameUTF16 := encodeUTF16LE(h.computerName)
	// Add null terminator
	nameUTF16 = append(nameUTF16, 0, 0)

	// SharedHeader(4) + UnicodeFlag(4) + CodePage(4) + ComputerNameLen(4) + ComputerName(variable)
	pdu := make([]byte, 16+len(nameUTF16))
	binary.LittleEndian.PutUint16(pdu[0:2], ComponentRDPDR)
	binary.LittleEndian.PutUint16(pdu[2:4], PakIDClientNameReq)
	binary.LittleEndian.PutUint32(pdu[4:8], 1) // unicodeFlag = 1
	// codePage = 0 (offset 8, already zero)
	binary.LittleEndian.PutUint32(pdu[12:16], uint32(len(nameUTF16)))
	copy(pdu[16:], nameUTF16)

	if err := h.sendFn(pdu); err != nil {
		h.log.LogAttrs(context.Background(), slog.LevelError, "send client name failed",
			slog.Any("err", err))
	}
}

// handleServerCoreCap processes Server Core Capability Request.
// Waits for both Server Caps and Server ID Confirm before sending the client response.
func (h *Handler) handleServerCoreCap(body []byte) {
	if len(body) < 4 {
		h.log.LogAttrs(context.Background(), slog.LevelError, "Server Core Caps too short")
		return
	}

	numCaps := binary.LittleEndian.Uint16(body[0:2])
	// padding at body[2:4]

	// Parse individual capability sets to extract versions
	off := 4
	for i := 0; i < int(numCaps) && off+4 <= len(body); i++ {
		capType := binary.LittleEndian.Uint16(body[off : off+2])
		capLen := binary.LittleEndian.Uint16(body[off+2 : off+4])
		if capLen < 4 || off+int(capLen) > len(body) {
			break
		}
		var capVer uint32
		if capLen >= 8 {
			capVer = binary.LittleEndian.Uint32(body[off+4 : off+8])
		}
		if capType == CapGeneralType {
			h.serverCapGenVer = capVer
			// ioCode1 is at offset 20 within the general capability set
			if capLen >= 24 {
				h.serverIOCode1 = binary.LittleEndian.Uint32(body[off+20 : off+24])
			}
		}
		h.log.LogAttrs(context.Background(), slog.LevelDebug, "server cap",
			slog.Int("type", int(capType)),
			slog.Int("len", int(capLen)),
			slog.Int("version", int(capVer)))
		off += int(capLen)
	}

	h.log.LogAttrs(context.Background(), slog.LevelDebug, "Server Core Caps",
		slog.Int("numCaps", int(numCaps)),
		slog.Int("generalVersion", int(h.serverCapGenVer)))

	h.gotServerCap = true
	h.trySendCapAndDeviceList()
}

// sendCoreCapResponse sends DR_CORE_CAPABILITY_RSP with General, Drive, and
// optionally Port capability sets.
func (h *Handler) sendCoreCapResponse() {
	// General capability set: header(4) + version(4) + osType(4) + osVersion(4) +
	// protocolMajor(2) + protocolMinor(2) + ioCode1(4) + ioCode2(4) +
	// extendedPDU(4) + extraFlags1(4) + extraFlags2(4) + specialTypeDeviceCap(4) = 44 bytes
	const generalCapLen = 44
	const capSetLen = 8 // type(2) + length(2) + version(4)

	// Check which device types are registered
	var hasPort, hasPrinter, hasSmartcard bool
	var smartcardCount uint32
	for _, dev := range h.devices {
		switch dev.Type() {
		case DeviceTypeSerial, DeviceTypeParallel:
			hasPort = true
		case DeviceTypePrinter:
			hasPrinter = true
		case DeviceTypeSmartcard:
			hasSmartcard = true
			smartcardCount++
		}
	}

	numCaps := uint16(2) // General + Drive (always)
	capsSize := generalCapLen + capSetLen
	if hasPort {
		numCaps++
		capsSize += capSetLen
	}
	if hasPrinter {
		numCaps++
		capsSize += capSetLen
	}
	if hasSmartcard {
		numCaps++
		capsSize += capSetLen
	}
	pduLen := 4 + 4 + capsSize // header(4) + numCaps(2)+pad(2) + caps
	pdu := make([]byte, pduLen)

	binary.LittleEndian.PutUint16(pdu[0:2], ComponentRDPDR)
	binary.LittleEndian.PutUint16(pdu[2:4], PakIDClientCoreCap)
	binary.LittleEndian.PutUint16(pdu[4:6], numCaps)
	// pdu[6:8] = padding

	off := 8

	// General capability set
	binary.LittleEndian.PutUint16(pdu[off:off+2], CapGeneralType)
	binary.LittleEndian.PutUint16(pdu[off+2:off+4], generalCapLen)
	binary.LittleEndian.PutUint32(pdu[off+4:off+8], GeneralCapVersion2)
	// osType = 0, osVersion = 0 (off+8..off+16)
	binary.LittleEndian.PutUint16(pdu[off+16:off+18], 1) // protocolMajor = 1
	binary.LittleEndian.PutUint16(pdu[off+18:off+20], 0x000C) // protocolMinor = RDP6X
	ioCode1 := h.serverIOCode1 & 0x0000FFFF // negotiate: AND of client and server ioCode1
	if ioCode1 == 0 {
		ioCode1 = 0x0000FFFF // fallback if server didn't advertise
	}
	binary.LittleEndian.PutUint32(pdu[off+20:off+24], ioCode1)
	// ioCode2 = 0 (off+24..off+28)
	binary.LittleEndian.PutUint32(pdu[off+28:off+32], 0x00000007) // extendedPDU: DEVICE_REMOVE | DISPLAY_NAME | USER_LOGGEDON
	binary.LittleEndian.PutUint32(pdu[off+32:off+36], 0x00000001) // extraFlags1: ENABLE_ASYNCIO
	// extraFlags2 = 0 (off+36..off+40)
	// specialTypeDeviceCap: count of smartcard devices (MS-RDPEFS 2.2.2.7.1)
	binary.LittleEndian.PutUint32(pdu[off+40:off+44], smartcardCount)
	off += generalCapLen

	// Drive capability set
	binary.LittleEndian.PutUint16(pdu[off:off+2], CapDriveType)
	binary.LittleEndian.PutUint16(pdu[off+2:off+4], capSetLen)
	binary.LittleEndian.PutUint32(pdu[off+4:off+8], GeneralCapVersion2)
	off += capSetLen

	// Port capability set (serial/parallel)
	if hasPort {
		binary.LittleEndian.PutUint16(pdu[off:off+2], CapPortType)
		binary.LittleEndian.PutUint16(pdu[off+2:off+4], capSetLen)
		binary.LittleEndian.PutUint32(pdu[off+4:off+8], GeneralCapVersion1)
		off += capSetLen
	}

	// Printer capability set
	if hasPrinter {
		binary.LittleEndian.PutUint16(pdu[off:off+2], CapPrinterType)
		binary.LittleEndian.PutUint16(pdu[off+2:off+4], capSetLen)
		binary.LittleEndian.PutUint32(pdu[off+4:off+8], GeneralCapVersion1)
		off += capSetLen
	}

	// Smartcard capability set
	if hasSmartcard {
		binary.LittleEndian.PutUint16(pdu[off:off+2], CapSmartCard)
		binary.LittleEndian.PutUint16(pdu[off+2:off+4], capSetLen)
		binary.LittleEndian.PutUint32(pdu[off+4:off+8], GeneralCapVersion1)
	}

	if err := h.sendFn(pdu); err != nil {
		h.log.LogAttrs(context.Background(), slog.LevelError, "send core cap response failed",
			slog.Any("err", err))
	}
}

// handleClientIDConfirm processes Server Client ID Confirm.
// Waits for both Server Caps and Server ID Confirm before sending the client response.
func (h *Handler) handleClientIDConfirm(body []byte) {
	if len(body) < 8 {
		h.log.LogAttrs(context.Background(), slog.LevelError, "Client ID Confirm too short")
		return
	}

	h.versionMajor = binary.LittleEndian.Uint16(body[0:2])
	h.versionMinor = binary.LittleEndian.Uint16(body[2:4])
	h.clientID = binary.LittleEndian.Uint32(body[4:8])

	h.log.LogAttrs(context.Background(), slog.LevelDebug, "Client ID Confirm",
		slog.Int("clientID", int(h.clientID)))

	h.gotIDConfirm = true
	h.trySendCapAndDeviceList()
}

// trySendCapAndDeviceList sends the capability response and device list once
// both Server Core Capability Request and Server Client ID Confirm have arrived.
// Disk drives are NOT announced until User Logged On (MS-RDPEFS 3.2.5.1.4).
func (h *Handler) trySendCapAndDeviceList() {
	if !h.gotServerCap || !h.gotIDConfirm {
		return
	}
	h.sendCoreCapResponse()
	// Send empty device list pre-logon. All devices (disk, serial, parallel)
	// are announced after Server User Logged On (MS-RDPEFS 3.2.5.1.4).
	h.sendDeviceListFiltered(false)
}

// handleUserLoggedOn processes Server User Logged On and announces disk devices.
func (h *Handler) handleUserLoggedOn() {
	h.log.LogAttrs(context.Background(), slog.LevelDebug, "user logged on, announcing drives")
	h.userLoggedOn = true
	h.sendDeviceListFiltered(true)
}

// sendDeviceListFiltered sends DR_CORE_DEVICELIST_ANNOUNCE_REQ.
// sendDeviceListFiltered sends DR_CORE_DEVICELIST_ANNOUNCE_REQ.
// When includeDevices is false, sends an empty list (pre-logon).
// When true, sends all registered devices (post User Logged On).
// Non-smartcard devices are announced after user logon (MS-RDPEFS 3.2.5.1.4);
// only smartcards go pre-logon.
func (h *Handler) sendDeviceListFiltered(includeDevices bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	type devEntry struct {
		devType uint32
		id      uint32
		dosName [8]byte
		devData []byte
	}

	var entries []devEntry
	var prnIdx int
	for _, dev := range h.devices {
		// Pre-logon: only announce smartcard devices.
		// Post-logon: announce all non-smartcard devices (smartcard already announced).
		if !includeDevices {
			if dev.Type() != DeviceTypeSmartcard {
				continue
			}
		} else {
			if dev.Type() == DeviceTypeSmartcard {
				continue
			}
		}
		var dosName [8]byte
		name := dev.Name()
		if dev.Type() == DeviceTypePrinter {
			// MS-RDPEPC: PreferredDosName for printers is "PRNn" (port name).
			s := fmt.Sprintf("PRN%d", prnIdx)
			copy(dosName[:], s)
			prnIdx++
		} else {
			// PreferredDosName: max 7 ASCII chars + null, replace non-ASCII with '_'
			for i := 0; i < 7 && i < len(name); i++ {
				c := name[i]
				if c > 0x7F {
					c = '_'
				}
				dosName[i] = c
			}
		}
		var devData []byte
		switch dev.Type() {
		case DeviceTypeDisk:
			// DeviceData: full drive name as null-terminated ASCII (MS-RDPEFS
			// 2.2.1.3), matching serial/parallel and the MS reference client.
			// NOT UTF-16: Windows reads this field as the ASCII friendly name,
			// so a UTF-16 value displays only the first character ("test" -> "t").
			devData = append([]byte(name), 0)
		case DeviceTypeSerial, DeviceTypeParallel:
			// DeviceData: null-terminated ASCII name (MS-RDPESP 2.2.2.1)
			devData = append([]byte(name), 0)
		case DeviceTypePrinter:
			devData = dev.(*PrinterDevice).DeviceData()
		case DeviceTypeSmartcard:
			// DeviceDataLength = 0 for smartcard
		}
		entries = append(entries, devEntry{devType: dev.Type(), id: dev.ID(), dosName: dosName, devData: devData})
	}

	// header(4) + deviceCount(4) + per-device(20 + devDataLen)
	numDevices := uint32(len(entries))
	totalLen := 8
	for _, e := range entries {
		totalLen += 20 + len(e.devData)
	}
	pdu := make([]byte, totalLen)
	binary.LittleEndian.PutUint16(pdu[0:2], ComponentRDPDR)
	binary.LittleEndian.PutUint16(pdu[2:4], PakIDDeviceListAnnounce)
	binary.LittleEndian.PutUint32(pdu[4:8], numDevices)

	off := 8
	for _, e := range entries {
		binary.LittleEndian.PutUint32(pdu[off:off+4], e.devType)
		binary.LittleEndian.PutUint32(pdu[off+4:off+8], e.id)
		copy(pdu[off+8:off+16], e.dosName[:])
		binary.LittleEndian.PutUint32(pdu[off+16:off+20], uint32(len(e.devData)))
		copy(pdu[off+20:], e.devData)
		off += 20 + len(e.devData)
	}

	if numDevices > 0 {
		h.log.LogAttrs(context.Background(), slog.LevelDebug, "device list PDU",
			slog.String("hex", fmt.Sprintf("%x", pdu)))
	}

	if err := h.sendFn(pdu); err != nil {
		h.log.LogAttrs(context.Background(), slog.LevelError, "send device list failed",
			slog.Any("err", err))
	}

	h.log.LogAttrs(context.Background(), slog.LevelInfo, "device list announced",
		slog.Int("devices", int(numDevices)))
}

// handleDeviceReply processes Server Device Announce Response.
func (h *Handler) handleDeviceReply(body []byte) {
	if len(body) < 8 {
		return
	}
	deviceID := binary.LittleEndian.Uint32(body[0:4])
	resultCode := binary.LittleEndian.Uint32(body[4:8])

	if resultCode == StatusSuccess {
		h.log.LogAttrs(context.Background(), slog.LevelInfo, "device accepted",
			slog.Int("deviceID", int(deviceID)))
	} else {
		h.log.LogAttrs(context.Background(), slog.LevelWarn, "device rejected",
			slog.Int("deviceID", int(deviceID)),
			slog.String("status", fmt.Sprintf("0x%08X", resultCode)))
	}
}

// handleIORequest parses DR_DEVICE_IOREQUEST and dispatches to the appropriate device.
func (h *Handler) handleIORequest(body []byte) {
	if len(body) < 20 {
		h.log.LogAttrs(context.Background(), slog.LevelError, "IO request too short")
		return
	}

	req := &IORequest{
		DeviceID:     binary.LittleEndian.Uint32(body[0:4]),
		FileID:       binary.LittleEndian.Uint32(body[4:8]),
		CompletionID: binary.LittleEndian.Uint32(body[8:12]),
		MajorFn:      binary.LittleEndian.Uint32(body[12:16]),
		MinorFn:      binary.LittleEndian.Uint32(body[16:20]),
		Payload:      body[20:],
	}

	h.log.LogAttrs(context.Background(), slog.LevelDebug, "IO request",
		slog.Int("deviceID", int(req.DeviceID)),
		slog.Int("fileID", int(req.FileID)),
		slog.Int("completionID", int(req.CompletionID)),
		slog.String("majorFn", irpMajorName(req.MajorFn)),
		slog.Int("minorFn", int(req.MinorFn)),
		slog.Int("payloadLen", len(req.Payload)))

	h.mu.Lock()
	dev, ok := h.devices[req.DeviceID]
	h.mu.Unlock()

	if !ok {
		h.log.LogAttrs(context.Background(), slog.LevelWarn, "IO request for unknown device",
			slog.Int("deviceID", int(req.DeviceID)))
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusNoSuchFile, nil)
		return
	}

	dev.HandleIRP(h, req)
}

// sendIOCompletion sends a DR_DEVICE_IOCOMPLETION PDU.
// Safe to call from multiple goroutines (serial read, smartcard operations).
func (h *Handler) sendIOCompletion(deviceID, completionID, ioStatus uint32, outputData []byte) {
	h.log.LogAttrs(context.Background(), slog.LevelDebug, "IO completion",
		slog.Int("deviceID", int(deviceID)),
		slog.Int("completionID", int(completionID)),
		slog.String("ioStatus", fmt.Sprintf("0x%08X", ioStatus)),
		slog.Int("outputLen", len(outputData)))
	const hdr = 16 // shared header(4) + deviceID(4) + completionID(4) + ioStatus(4)
	need := hdr + len(outputData)
	// Allocate per-call — this function is called from multiple goroutines
	// (serial read, smartcard GetStatusChange/Connect) so a shared buffer races.
	pdu := make([]byte, need)
	binary.LittleEndian.PutUint16(pdu[0:2], ComponentRDPDR)
	binary.LittleEndian.PutUint16(pdu[2:4], PakIDDeviceIOCompletion)
	binary.LittleEndian.PutUint32(pdu[4:8], deviceID)
	binary.LittleEndian.PutUint32(pdu[8:12], completionID)
	binary.LittleEndian.PutUint32(pdu[12:16], ioStatus)
	if len(outputData) > 0 {
		copy(pdu[hdr:], outputData)
	}
	if err := h.sendFn(pdu); err != nil {
		h.log.LogAttrs(context.Background(), slog.LevelError, "send IO completion failed",
			slog.Any("err", err))
	}
}

// irpMajorName returns a human-readable name for an IRP major function code.
func irpMajorName(fn uint32) string {
	switch fn {
	case IrpCreate:
		return "CREATE"
	case IrpClose:
		return "CLOSE"
	case IrpRead:
		return "READ"
	case IrpWrite:
		return "WRITE"
	case IrpDeviceControl:
		return "DEVICE_CONTROL"
	case IrpQueryVolume:
		return "QUERY_VOLUME"
	case IrpSetVolume:
		return "SET_VOLUME"
	case IrpQueryInfo:
		return "QUERY_INFO"
	case IrpSetInfo:
		return "SET_INFO"
	case IrpDirControl:
		return "DIRECTORY_CONTROL"
	case IrpLockControl:
		return "LOCK_CONTROL"
	default:
		return fmt.Sprintf("0x%X", fn)
	}
}

// encodeUTF16LE encodes a Go string as UTF-16LE bytes (no BOM, no null terminator).
func encodeUTF16LE(s string) []byte {
	runes := utf16.Encode([]rune(s))
	buf := make([]byte, len(runes)*2)
	for i, r := range runes {
		binary.LittleEndian.PutUint16(buf[i*2:], r)
	}
	return buf
}

// decodeUTF16LE decodes UTF-16LE bytes to a Go string, stripping any null terminator.
func decodeUTF16LE(b []byte) string {
	if len(b) < 2 {
		return ""
	}
	n := len(b) / 2
	u16 := make([]uint16, n)
	for i := 0; i < n; i++ {
		u16[i] = binary.LittleEndian.Uint16(b[i*2:])
	}
	// Strip trailing null
	for len(u16) > 0 && u16[len(u16)-1] == 0 {
		u16 = u16[:len(u16)-1]
	}
	return string(utf16.Decode(u16))
}

// Printer cache event IDs (MS-RDPEPC 2.2.2.3)
const (
	rdpdrAddPrinterEvent    uint32 = 0x00000001
	rdpdrUpdatePrinterEvent uint32 = 0x00000002
	rdpdrDeletePrinterEvent uint32 = 0x00000003
	rdpdrRenamePrinterEvent uint32 = 0x00000004
)

// handlePrinterComponent processes MS-RDPEPC printer-specific PDUs.
func (h *Handler) handlePrinterComponent(packetID uint16, body []byte) {
	if packetID != PakIDPrnCacheData {
		h.log.LogAttrs(context.Background(), slog.LevelWarn, "unknown printer packet ID",
			slog.String("packetID", fmt.Sprintf("0x%04X", packetID)))
		return
	}
	if len(body) < 4 {
		return
	}
	eventID := binary.LittleEndian.Uint32(body[0:4])
	body = body[4:]

	switch eventID {
	case rdpdrAddPrinterEvent:
		h.handlePrnCacheAdd(body)
	case rdpdrUpdatePrinterEvent:
		h.handlePrnCacheUpdate(body)
	case rdpdrDeletePrinterEvent, rdpdrRenamePrinterEvent:
		h.log.LogAttrs(context.Background(), slog.LevelInfo, "printer cache event (ignored)",
			slog.String("event", fmt.Sprintf("0x%08X", eventID)))
	default:
		h.log.LogAttrs(context.Background(), slog.LevelWarn, "unknown printer cache event",
			slog.String("eventID", fmt.Sprintf("0x%08X", eventID)))
	}
}

// handlePrnCacheAdd processes RDPDR_ADD_PRINTER_EVENT — server sends full
// printer configuration for the client to cache and include in future announces.
func (h *Handler) handlePrnCacheAdd(body []byte) {
	// PortDosName(8) + PnPNameLen(4) + DriverNameLen(4) + PrintNameLen(4) + CacheFieldsLen(4)
	if len(body) < 24 {
		return
	}
	portDosName := string(body[0:8])
	pnpNameLen := binary.LittleEndian.Uint32(body[8:12])
	driverNameLen := binary.LittleEndian.Uint32(body[12:16])
	printNameLen := binary.LittleEndian.Uint32(body[16:20])
	cacheFieldsLen := binary.LittleEndian.Uint32(body[20:24])
	body = body[24:]

	needed := int(pnpNameLen + driverNameLen + printNameLen + cacheFieldsLen)
	if len(body) < needed {
		return
	}

	off := pnpNameLen + driverNameLen
	printerName := ""
	if printNameLen > 0 {
		printerName = decodeUTF16LE(body[off : off+printNameLen])
	}

	off += printNameLen
	cacheData := body[off : off+cacheFieldsLen]

	h.log.LogAttrs(context.Background(), slog.LevelInfo, "printer cache add",
		slog.String("port", portDosName), slog.String("printer", printerName),
		slog.Int("cacheBytes", int(cacheFieldsLen)))

	// Store cached config on the matching PrinterDevice for future reconnects.
	h.mu.Lock()
	for _, dev := range h.devices {
		if dev.Type() == DeviceTypePrinter {
			pd := dev.(*PrinterDevice)
			if pd.name == printerName || portDosName != "" {
				cached := make([]byte, len(cacheData))
				copy(cached, cacheData)
				pd.mu.Lock()
				pd.cachedConfig = cached
				pd.mu.Unlock()
				break
			}
		}
	}
	h.mu.Unlock()
}

// handlePrnCacheUpdate processes RDPDR_UPDATE_PRINTER_EVENT.
func (h *Handler) handlePrnCacheUpdate(body []byte) {
	if len(body) < 8 {
		return
	}
	printNameLen := binary.LittleEndian.Uint32(body[0:4])
	configDataLen := binary.LittleEndian.Uint32(body[4:8])
	body = body[8:]

	needed := int(printNameLen + configDataLen)
	if len(body) < needed {
		return
	}

	printerName := ""
	if printNameLen > 0 {
		printerName = decodeUTF16LE(body[:printNameLen])
	}
	cacheData := body[printNameLen : printNameLen+configDataLen]

	h.log.LogAttrs(context.Background(), slog.LevelInfo, "printer cache update",
		slog.String("printer", printerName), slog.Int("cacheBytes", int(configDataLen)))

	h.mu.Lock()
	for _, dev := range h.devices {
		if dev.Type() == DeviceTypePrinter {
			pd := dev.(*PrinterDevice)
			if pd.name == printerName {
				cached := make([]byte, len(cacheData))
				copy(cached, cacheData)
				pd.mu.Lock()
				pd.cachedConfig = cached
				pd.mu.Unlock()
				break
			}
		}
	}
	h.mu.Unlock()
}

// EncodeSharedHeader writes a 4-byte RDPDR shared header.
func EncodeSharedHeader(component, packetID uint16) []byte {
	var buf [4]byte
	binary.LittleEndian.PutUint16(buf[0:2], component)
	binary.LittleEndian.PutUint16(buf[2:4], packetID)
	return buf[:]
}
