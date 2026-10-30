// Package urbdrc implements MS-RDPEUSB (Remote Desktop Protocol: USB Devices
// Virtual Channel Extension) for redirecting USB devices from client to server
// over the "URBDRC" dynamic virtual channel.
//
// The protocol operates at the USB Request Block (URB) level: the server sends
// raw USB requests and the client forwards them to the physical device via
// usbdevfs (Linux) or equivalent OS interfaces.
//
// Protocol reference: MS-RDPEUSB sections 2.2 and 3.2.
package urbdrc

import (
	"context"
	"encoding/binary"
	"fmt"
	"log/slog"
	"sync"
	"time"
	"unicode/utf16"
)

// ChannelName is the DVC channel name used by the server (MS-RDPEUSB 2.1).
const ChannelName = "URBDRC"

// Stream IDs (bits 31-30 of InterfaceId).
const (
	streamIDNone  uint32 = 0x00000000
	streamIDProxy uint32 = 0x40000000 // 1 << 30
	streamIDStub  uint32 = 0x80000000 // 2 << 30
)

const (
	streamIDMask    uint32 = 0xC0000000
	interfaceIDMask uint32 = 0x3FFFFFFF
)

// Interface IDs (bits 29-0 of InterfaceId).
const (
	capabilitiesNegotiator    uint32 = 0x00000000
	clientDeviceSink          uint32 = 0x00000001
	serverChannelNotification uint32 = 0x00000002
	clientChannelNotification uint32 = 0x00000003
	baseUSBDeviceNum          uint32 = 0x00000005
)

// Function IDs on the capabilities/channel negotiation interfaces.
const (
	rimCallRelease               uint32 = 0x00000001
	rimExchangeCapabilityRequest uint32 = 0x00000100
	channelCreatedFn             uint32 = 0x00000100
	addVirtualChannelFn          uint32 = 0x00000100
	addDeviceFn                  uint32 = 0x00000101
)

// Function IDs on per-device interfaces.
const (
	cancelRequestFn           uint32 = 0x00000100
	registerRequestCallbackFn uint32 = 0x00000101
	ioControlFn               uint32 = 0x00000102
	internalIOControlFn       uint32 = 0x00000103
	queryDeviceTextFn         uint32 = 0x00000104
	transferInRequestFn       uint32 = 0x00000105
	transferOutRequestFn      uint32 = 0x00000106
	retractDeviceFn           uint32 = 0x00000107
)

// Completion function IDs (client → server responses).
const (
	ioControlCompletionFn uint32 = 0x00000100
	urbCompletionFn       uint32 = 0x00000101
	urbCompletionNoDataFn uint32 = 0x00000102
)

// Device text types (MS-RDPEUSB 2.2.6.5).
const (
	deviceTextDescription         uint32 = 0
	deviceTextLocationInformation uint32 = 1
)

// Capability version.
const rimCapabilityVersion01 uint32 = 0x00000001

// IOCTL codes (MS-RDPEUSB 2.2.12).
const (
	ioctlInternalUSBSubmitURB              uint32 = 0x00220003
	ioctlInternalUSBResetPort              uint32 = 0x00220007
	ioctlInternalUSBGetPortStatus          uint32 = 0x00220013
	ioctlInternalUSBCyclePort              uint32 = 0x0022001F
	ioctlInternalUSBSubmitIdleNotification uint32 = 0x00220027
	ioctlTSUSBGDQueryBusTime               uint32 = 0x00224000
)

// Retract reasons (MS-RDPEUSB 2.2.9).
const usbRetractReasonBlockedByPolicy uint32 = 0x00000001

// Handler state.
const (
	stateInitChannelIn  = 0 // waiting for first control channel
	stateInitChannelOut = 1 // announcing devices on per-device channels
)

// USBDeviceFilter specifies VID/PID criteria for hotplug device matching.
type USBDeviceFilter struct {
	VID uint16
	PID uint16
}

// usbDeviceIdentity holds the identity of a USB device discovered via sysfs scan.
type usbDeviceIdentity struct {
	BusNum      uint8
	DevAddr     uint8
	VID         uint16
	PID         uint16
	DeviceClass uint8  // bDeviceClass from sysfs
	SysPath     string // sysfs device name, e.g. "1-4"
}

// DefaultExcludeClasses lists device classes excluded from auto-add by default.
// Classes that have dedicated RDP channels, would break the session, or
// block the URBDRC server's serial device setup pipeline.
// Explicit VID:PID filters bypass this list.
var DefaultExcludeClasses = []uint8{
	0x01, // Audio — use -audio-out/-audio-in (RDPSND/AUDIN)
	0x03, // HID — keyboard/mouse handled by RDP input
	0x09, // Hub — cannot be redirected
	0x0b, // Smart Card — use -smartcard (RDPESC), avoids blocking URBDRC pipeline
}

// deviceScanner scans for USB devices matching the given filters.
type deviceScanner interface {
	scanDevices(filters []USBDeviceFilter) []usbDeviceIdentity
}

// Handler manages the MS-RDPEUSB protocol over URBDRC dynamic virtual channels.
//
// The server creates multiple DVC channels named "URBDRC": the first is the
// control channel (caps exchange + device announcement), and subsequent channels
// are per-device I/O channels.
type Handler struct {
	dvcSend func(channelID uint32, data []byte) error
	log     *slog.Logger

	mu             sync.Mutex
	state          int
	controlChID    uint32                  // DVC channel ID of the control channel
	devices        []*deviceState
	devByUsbID     map[uint32]*deviceState // by UsbDevice ID
	devByChannel   map[uint32]*deviceState // by DVC channel ID
	knownDevices   map[string]*deviceState // by sysfs path (for hotplug diffing)
	pendingDevices []*deviceState          // FIFO queue for channel assignment
	nextUsbDevice  uint32                  // next UsbDevice ID to assign

	// Semaphore to bound concurrent USB transfer goroutines.
	transferSem chan struct{}

	// Hotplug
	filters        []USBDeviceFilter
	excludeClasses []uint8 // device classes to skip for wildcard filters
	dvcClose       func(channelID uint32)
	ctx            context.Context
	monitorCancel  context.CancelFunc
	scanner       deviceScanner
	openDevice    func(uint8, uint8) (USBDevice, error)
}

// deviceState tracks per-device protocol state.
type deviceState struct {
	dev           USBDevice
	usbDeviceID   uint32 // unique ID for the RDP protocol
	channelID     uint32 // DVC channel ID assigned by server
	reqCompletion uint32 // RequestCompletion interface ID from server
	channelOpen   bool
}

// NewHandler creates a new URBDRC handler.
// dvcSend sends data on the specified DVC channel (wraps dvc.SendData).
func NewHandler(dvcSend func(channelID uint32, data []byte) error, log *slog.Logger) *Handler {
	return &Handler{
		dvcSend:       dvcSend,
		log:           log,
		devByUsbID:    make(map[uint32]*deviceState),
		devByChannel:  make(map[uint32]*deviceState),
		knownDevices:  make(map[string]*deviceState),
		nextUsbDevice: baseUSBDeviceNum,
		transferSem:   make(chan struct{}, 64), // bound concurrent USB transfer goroutines
	}
}

// AddDevice registers a USB device for redirection. Must be called before
// the connection is established (before ProcessPDU is called).
func (h *Handler) AddDevice(dev USBDevice) {
	h.mu.Lock()
	defer h.mu.Unlock()

	ds := &deviceState{
		dev:         dev,
		usbDeviceID: h.nextUsbDevice,
	}
	h.nextUsbDevice++
	h.devices = append(h.devices, ds)
	h.devByUsbID[ds.usbDeviceID] = ds
	h.knownDevices[dev.Path()] = ds
}

// ProcessPDU handles an incoming URBDRC PDU on the given DVC channel.
func (h *Handler) ProcessPDU(channelID uint32, data []byte) {
	if len(data) < 12 {
		h.log.LogAttrs(context.Background(), slog.LevelError, "PDU too short",
			slog.Int("len", len(data)))
		return
	}

	interfaceID := binary.LittleEndian.Uint32(data[0:4])
	messageID := binary.LittleEndian.Uint32(data[4:8])
	functionID := binary.LittleEndian.Uint32(data[8:12])
	body := data[12:]

	h.log.LogAttrs(context.Background(), slog.LevelDebug, "URBDRC PDU",
		slog.String("interface", fmt.Sprintf("%08x", interfaceID)),
		slog.String("message", fmt.Sprintf("%08x", messageID)),
		slog.String("function", fmt.Sprintf("%08x", functionID)),
		slog.Int("bodyLen", len(body)),
		slog.Int("channel", int(channelID)))

	switch interfaceID {
	case streamIDNone | capabilitiesNegotiator:
		h.handleCapabilityRequest(channelID, messageID, functionID, body)
	case streamIDProxy | serverChannelNotification:
		h.handleChannelNotification(channelID, messageID, functionID, body)
	default:
		h.handleDeviceData(channelID, interfaceID, messageID, functionID, body)
	}
}

// OnChannelOpen is called when a new DVC channel named "URBDRC" is opened.
func (h *Handler) OnChannelOpen(channelID uint32) {
	h.log.LogAttrs(context.Background(), slog.LevelInfo, "URBDRC channel opened",
		slog.Int("channel", int(channelID)))
}

// OnChannelClose is called when a DVC channel named "URBDRC" is closed.
func (h *Handler) OnChannelClose(channelID uint32) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if ds, ok := h.devByChannel[channelID]; ok {
		h.log.LogAttrs(context.Background(), slog.LevelInfo, "device channel closed",
			slog.Int("usbDevice", int(ds.usbDeviceID)))
		ds.channelOpen = false
	}
}

// Close stops the hotplug monitor and releases all USB devices.
func (h *Handler) Close() {
	h.StopMonitor()

	h.mu.Lock()
	defer h.mu.Unlock()

	for _, ds := range h.devices {
		ds.dev.Close()
	}
}

// --- Hotplug support ---

// SetDVCClose sets the callback used to close a DVC channel when a device
// is removed. The callback should send a CmdClose PDU (e.g. dvc.CloseChannel).
func (h *Handler) SetDVCClose(fn func(channelID uint32)) {
	h.dvcClose = fn
}

// SetHotplugFilters sets the VID/PID filters for hotplug monitoring.
// Also sets the default sysfs scanner and exclude classes if none were set.
func (h *Handler) SetHotplugFilters(filters []USBDeviceFilter) {
	h.filters = filters
	if h.scanner == nil {
		h.scanner = sysfsScanner{}
	}
	if h.excludeClasses == nil {
		h.excludeClasses = DefaultExcludeClasses
	}
}

// SetExcludeClasses overrides the default device classes excluded from
// auto-add. Pass nil to clear all exclusions.
func (h *Handler) SetExcludeClasses(classes []uint8) {
	h.excludeClasses = classes
}

// SetContext sets the parent context for the hotplug monitor goroutine.
func (h *Handler) SetContext(ctx context.Context) {
	h.ctx = ctx
}

// startMonitorLocked starts the hotplug monitor goroutine if filters are
// configured. Must be called with h.mu held.
//
// Before starting the poll loop, it snapshots all currently-present devices
// into knownDevices (without opening them). This way the first poll only
// picks up devices plugged in AFTER the session started.
func (h *Handler) startMonitorLocked() {
	if len(h.filters) == 0 || h.scanner == nil {
		return
	}
	if h.monitorCancel != nil {
		return // already running
	}

	// Seed knownDevices with devices already present so the monitor
	// only reacts to newly inserted devices.
	existing := h.scanner.scanDevices(h.filters)
	for _, id := range existing {
		if _, ok := h.knownDevices[id.SysPath]; !ok {
			// Mark as known but don't open — these were plugged in
			// before the session, so they shouldn't be redirected.
			h.knownDevices[id.SysPath] = nil
		}
	}

	ctx := h.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	mctx, cancel := context.WithCancel(ctx)
	h.monitorCancel = cancel
	go h.monitorLoop(mctx)
	h.log.LogAttrs(context.Background(), slog.LevelInfo, "hotplug monitor started",
		slog.Int("filters", len(h.filters)),
		slog.Int("existing", len(existing)))
}

// StopMonitor stops the hotplug monitor goroutine.
func (h *Handler) StopMonitor() {
	h.mu.Lock()
	cancel := h.monitorCancel
	h.monitorCancel = nil
	h.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// monitorLoop polls for USB device changes at regular intervals.
func (h *Handler) monitorLoop(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			h.pollDevices()
		}
	}
}

// pollDevices scans for USB devices and diffs against known state.
// Inserts are announced via ADD_VIRTUAL_CHANNEL; removals close the channel.
// Device opens happen OUTSIDE the lock to avoid blocking the handler when
// the kernel driver detach takes time (e.g. mounted mass storage).
func (h *Handler) pollDevices() {
	// Scan outside lock (read-only sysfs I/O, no shared state).
	found := h.scanner.scanDevices(h.filters)

	h.mu.Lock()

	// Build set of found sysfs paths.
	foundSet := make(map[string]usbDeviceIdentity, len(found))
	for _, id := range found {
		foundSet[id.SysPath] = id
	}

	// Removals: in knownDevices but not in scan.
	for path, ds := range h.knownDevices {
		if _, ok := foundSet[path]; !ok {
			h.removeDeviceLocked(ds, path)
		}
	}

	// Collect new devices to add (don't open yet).
	var toAdd []usbDeviceIdentity
	for _, id := range found {
		if _, ok := h.knownDevices[id.SysPath]; !ok {
			if h.isOnlyWildcardMatch(id) && h.isClassExcluded(id.DeviceClass) {
				continue
			}
			toAdd = append(toAdd, id)
		}
	}

	h.mu.Unlock()

	// Open devices outside lock — may block for mass storage (kernel
	// driver detach waits for pending I/O to drain).
	for _, id := range toAdd {
		open := h.openDevice
		if open == nil {
			open = OpenUSBDeviceByAddr
		}
		dev, err := open(id.BusNum, id.DevAddr)
		if err != nil {
			h.log.LogAttrs(context.Background(), slog.LevelError, "failed to open hotplug device",
				slog.String("path", id.SysPath), slog.Any("err", err))
			continue
		}

		h.mu.Lock()
		// Re-check: device may have been added by a concurrent path
		// or removed between unlock and re-lock.
		if _, ok := h.knownDevices[id.SysPath]; ok {
			h.mu.Unlock()
			dev.Close()
			continue
		}
		h.registerDeviceLocked(id, dev)
		h.mu.Unlock()
	}
}

// isOnlyWildcardMatch returns true if the device only matched via wildcard
// filters (VID=0, PID=0). Explicit VID:PID filters bypass class exclusion,
// Explicit VID:PID filters bypass class exclusion.
func (h *Handler) isOnlyWildcardMatch(id usbDeviceIdentity) bool {
	for _, f := range h.filters {
		if f.VID == 0 && f.PID == 0 {
			continue // wildcard, skip
		}
		if (f.VID == 0 || f.VID == id.VID) && (f.PID == 0 || f.PID == id.PID) {
			return false // matched by an explicit filter
		}
	}
	return true
}

// isClassExcluded returns true if the device class is in the exclusion list.
func (h *Handler) isClassExcluded(class uint8) bool {
	for _, c := range h.excludeClasses {
		if c == class {
			return true
		}
	}
	return false
}

// registerDeviceLocked adds an already-opened device to all tracking maps
// and sends ADD_VIRTUAL_CHANNEL. Must be called with h.mu held.
func (h *Handler) registerDeviceLocked(id usbDeviceIdentity, dev USBDevice) {
	ds := &deviceState{
		dev:         dev,
		usbDeviceID: h.nextUsbDevice,
	}
	h.nextUsbDevice++
	h.devices = append(h.devices, ds)
	h.devByUsbID[ds.usbDeviceID] = ds
	h.knownDevices[id.SysPath] = ds

	h.pendingDevices = append(h.pendingDevices, ds)
	h.sendAddVirtualChannel(ds)

	desc := dev.Descriptor()
	h.log.LogAttrs(context.Background(), slog.LevelInfo, "hotplug device added",
		slog.String("path", id.SysPath),
		slog.String("vid", fmt.Sprintf("%04x", desc.IDVendor)),
		slog.String("pid", fmt.Sprintf("%04x", desc.IDProduct)))
}

// removeDeviceLocked closes a removed device, cleans up all maps, and
// closes the DVC channel if open. Must be called with h.mu held.
// ds may be nil for devices that were seeded as "already present" at
// monitor start (known but never opened).
func (h *Handler) removeDeviceLocked(ds *deviceState, sysPath string) {
	delete(h.knownDevices, sysPath)

	if ds == nil {
		// Seeded device that was never opened — just remove from known set.
		return
	}

	h.log.LogAttrs(context.Background(), slog.LevelInfo, "hotplug device removed",
		slog.String("path", sysPath),
		slog.Int("usbDevice", int(ds.usbDeviceID)))

	ds.dev.Close()

	delete(h.devByUsbID, ds.usbDeviceID)
	if ds.channelOpen {
		delete(h.devByChannel, ds.channelID)
		if h.dvcClose != nil {
			h.dvcClose(ds.channelID)
		}
	}

	// Remove from devices slice.
	for i, d := range h.devices {
		if d == ds {
			h.devices = append(h.devices[:i], h.devices[i+1:]...)
			break
		}
	}

	// Remove from pendingDevices if queued.
	for i, d := range h.pendingDevices {
		if d == ds {
			h.pendingDevices = append(h.pendingDevices[:i], h.pendingDevices[i+1:]...)
			break
		}
	}
}

// handleCapabilityRequest responds to RIM_EXCHANGE_CAPABILITY_REQUEST.
func (h *Handler) handleCapabilityRequest(channelID, messageID, functionID uint32, body []byte) {
	switch functionID {
	case rimExchangeCapabilityRequest:
		if len(body) < 4 {
			h.log.LogAttrs(context.Background(), slog.LevelError, "capability request too short")
			return
		}
		version := binary.LittleEndian.Uint32(body[0:4])
		if version > rimCapabilityVersion01 {
			version = rimCapabilityVersion01
		}
		h.log.LogAttrs(context.Background(), slog.LevelDebug, "RIM_EXCHANGE_CAPABILITY_REQUEST",
			slog.Int("version", int(version)))
		h.sendCapabilityResponse(channelID, messageID, version)
	case rimCallRelease:
		// Ignore
	}
}

// sendCapabilityResponse sends RIM_EXCHANGE_CAPABILITY_RESPONSE.
func (h *Handler) sendCapabilityResponse(channelID, messageID, version uint32) {
	interfaceID := streamIDNone | capabilitiesNegotiator
	buf := make([]byte, 16)
	binary.LittleEndian.PutUint32(buf[0:4], interfaceID)
	binary.LittleEndian.PutUint32(buf[4:8], messageID)
	binary.LittleEndian.PutUint32(buf[8:12], version) // FunctionId = version
	binary.LittleEndian.PutUint32(buf[12:16], 0)      // HRESULT = S_OK

	if err := h.dvcSend(channelID, buf); err != nil {
		h.log.LogAttrs(context.Background(), slog.LevelError, "failed to send capability response",
			slog.Any("err", err))
	}
}

// handleChannelNotification handles CHANNEL_CREATED and RIMCALL_RELEASE on
// the SERVER_CHANNEL_NOTIFICATION interface.
func (h *Handler) handleChannelNotification(channelID, messageID, functionID uint32, body []byte) {
	switch functionID {
	case channelCreatedFn:
		h.handleChannelCreated(channelID, messageID, body)
	case rimCallRelease:
		h.handleRIMCallRelease(channelID)
	}
}

// handleChannelCreated responds to CHANNEL_CREATED notification.
func (h *Handler) handleChannelCreated(channelID, messageID uint32, body []byte) {
	if len(body) < 12 {
		h.log.LogAttrs(context.Background(), slog.LevelError, "CHANNEL_CREATED too short")
		return
	}

	majorVersion := binary.LittleEndian.Uint32(body[0:4])
	minorVersion := binary.LittleEndian.Uint32(body[4:8])
	capabilities := binary.LittleEndian.Uint32(body[8:12])

	if majorVersion != 1 || minorVersion != 0 {
		h.log.LogAttrs(context.Background(), slog.LevelWarn, "unexpected URBDRC version, forcing 1.0",
			slog.Int("major", int(majorVersion)), slog.Int("minor", int(minorVersion)))
		majorVersion = 1
		minorVersion = 0
	}
	if capabilities != 0 {
		capabilities = 0
	}

	h.log.LogAttrs(context.Background(), slog.LevelDebug, "CHANNEL_CREATED",
		slog.Int("major", int(majorVersion)),
		slog.Int("minor", int(minorVersion)))

	// Respond with CHANNEL_CREATED on CLIENT_CHANNEL_NOTIFICATION
	interfaceID := streamIDProxy | clientChannelNotification
	buf := make([]byte, 24)
	binary.LittleEndian.PutUint32(buf[0:4], interfaceID)
	binary.LittleEndian.PutUint32(buf[4:8], messageID)
	binary.LittleEndian.PutUint32(buf[8:12], channelCreatedFn)
	binary.LittleEndian.PutUint32(buf[12:16], majorVersion)
	binary.LittleEndian.PutUint32(buf[16:20], minorVersion)
	binary.LittleEndian.PutUint32(buf[20:24], capabilities)

	if err := h.dvcSend(channelID, buf); err != nil {
		h.log.LogAttrs(context.Background(), slog.LevelError, "failed to send CHANNEL_CREATED response",
			slog.Any("err", err))
	}
}

// handleRIMCallRelease handles the state machine transitions after
// RIMCALL_RELEASE is received.
func (h *Handler) handleRIMCallRelease(channelID uint32) {
	h.mu.Lock()
	defer h.mu.Unlock()

	switch h.state {
	case stateInitChannelIn:
		// First RIMCALL_RELEASE on the control channel:
		// save control channel ID and announce all devices.
		h.controlChID = channelID
		h.state = stateInitChannelOut
		h.log.LogAttrs(context.Background(), slog.LevelInfo, "control channel established, announcing devices",
			slog.Int("count", len(h.devices)))
		h.announceDevicesLocked()

		h.startMonitorLocked()

	case stateInitChannelOut:
		// RIMCALL_RELEASE on a new per-device channel:
		// pop next device from pending queue and send ADD_DEVICE.
		if len(h.pendingDevices) == 0 {
			h.log.LogAttrs(context.Background(), slog.LevelWarn, "no more devices to announce")
			return
		}
		ds := h.pendingDevices[0]
		h.pendingDevices = h.pendingDevices[1:]
		ds.channelID = channelID
		ds.channelOpen = true
		h.devByChannel[channelID] = ds
		h.log.LogAttrs(context.Background(), slog.LevelInfo, "device channel assigned",
			slog.Int("usbDevice", int(ds.usbDeviceID)),
			slog.Int("channel", int(channelID)))
		h.sendAddDevice(channelID, ds)
	}
}

// announceDevicesLocked populates pendingDevices and sends ADD_VIRTUAL_CHANNEL
// for each device on the control channel. Must be called with h.mu held.
func (h *Handler) announceDevicesLocked() {
	for _, ds := range h.devices {
		h.pendingDevices = append(h.pendingDevices, ds)
		h.sendAddVirtualChannel(ds)
	}
}

// sendAddVirtualChannel sends ADD_VIRTUAL_CHANNEL for a single device on
// the control channel. Must be called with h.mu held.
func (h *Handler) sendAddVirtualChannel(ds *deviceState) {
	interfaceID := streamIDProxy | clientDeviceSink
	buf := make([]byte, 12)
	binary.LittleEndian.PutUint32(buf[0:4], interfaceID)
	binary.LittleEndian.PutUint32(buf[4:8], ds.usbDeviceID) // MessageId = UsbDevice
	binary.LittleEndian.PutUint32(buf[8:12], addVirtualChannelFn)

	if err := h.dvcSend(h.controlChID, buf); err != nil {
		h.log.LogAttrs(context.Background(), slog.LevelError, "failed to send ADD_VIRTUAL_CHANNEL",
			slog.Any("err", err))
	}
}

// sendAddDevice sends the ADD_DEVICE PDU on a per-device channel.
func (h *Handler) sendAddDevice(channelID uint32, ds *deviceState) {
	desc := ds.dev.Descriptor()
	path := ds.dev.Path()

	// Generate instance ID from device path.
	instanceID := generateInstanceID(path)
	// Generate container ID from VID/PID/path.
	containerID := generateContainerID(desc.IDVendor, desc.IDProduct, path)

	// Hardware IDs: "USB\VID_xxxx&PID_yyyy&REV_zzzz", "USB\VID_xxxx&PID_yyyy"
	hwID0 := fmt.Sprintf("USB\\VID_%04X&PID_%04X&REV_%04X", desc.IDVendor, desc.IDProduct, desc.BCDDevice)
	hwID1 := fmt.Sprintf("USB\\VID_%04X&PID_%04X", desc.IDVendor, desc.IDProduct)

	// Compatibility IDs based on device class
	var compatIDs []string
	if ds.dev.IsComposite() {
		compatIDs = []string{
			"USB\\DevClass_00&SubClass_00&Prot_00",
			"USB\\DevClass_00&SubClass_00",
			"USB\\DevClass_00",
			"USB\\COMPOSITE",
		}
	} else {
		compatIDs = []string{
			fmt.Sprintf("USB\\Class_%02X&SubClass_%02X&Prot_%02X", desc.BDeviceClass, desc.BDeviceSubClass, desc.BDeviceProtocol),
			fmt.Sprintf("USB\\Class_%02X&SubClass_%02X", desc.BDeviceClass, desc.BDeviceSubClass),
			fmt.Sprintf("USB\\Class_%02X", desc.BDeviceClass),
		}
	}

	// Build the PDU
	interfaceID := streamIDProxy | clientDeviceSink

	// Pre-calculate size
	size := 12 // header
	size += 8  // NumUsbDevice + UsbDevice

	instanceUTF16 := utf16Encode(instanceID)
	size += 4 + (len(instanceUTF16)+1)*2 // cchInstanceId + chars + null

	hwIDs := []string{hwID0, hwID1}
	hwUTF16 := make([][]uint16, len(hwIDs))
	hwTotalChars := 0
	for i, s := range hwIDs {
		hwUTF16[i] = utf16Encode(s)
		hwTotalChars += len(hwUTF16[i]) + 1 // +1 for null terminator per string
	}
	hwTotalChars++ // multi-SZ final null
	size += 4 + hwTotalChars*2

	compatUTF16 := make([][]uint16, len(compatIDs))
	compatTotalChars := 0
	for i, s := range compatIDs {
		compatUTF16[i] = utf16Encode(s)
		compatTotalChars += len(compatUTF16[i]) + 1
	}
	compatTotalChars++ // multi-SZ final null
	size += 4 + compatTotalChars*2

	containerUTF16 := utf16Encode(containerID)
	size += 4 + (len(containerUTF16)+1)*2 // cchContainerId + chars + null

	size += 28 // USB_DEVICE_CAPABILITIES

	buf := make([]byte, size)
	off := 0

	// Shared header
	binary.LittleEndian.PutUint32(buf[off:], interfaceID)
	off += 4
	binary.LittleEndian.PutUint32(buf[off:], 0) // MessageId
	off += 4
	binary.LittleEndian.PutUint32(buf[off:], addDeviceFn)
	off += 4

	// NumUsbDevice + UsbDevice
	binary.LittleEndian.PutUint32(buf[off:], 1)
	off += 4
	binary.LittleEndian.PutUint32(buf[off:], ds.usbDeviceID)
	off += 4

	// Instance ID
	off += writeStringBlock(buf[off:], instanceUTF16, false)

	// Hardware IDs (multi-SZ)
	off += writeMultiStringBlock(buf[off:], hwUTF16)

	// Compatibility IDs (multi-SZ)
	off += writeMultiStringBlock(buf[off:], compatUTF16)

	// Container ID
	off += writeStringBlock(buf[off:], containerUTF16, false)

	// USB_DEVICE_CAPABILITIES (28 bytes)
	binary.LittleEndian.PutUint32(buf[off:], 0x1c) // CbSize = 28
	off += 4
	binary.LittleEndian.PutUint32(buf[off:], 2) // UsbBusInterfaceVersion
	off += 4
	binary.LittleEndian.PutUint32(buf[off:], 0x600) // USBDI_Version
	off += 4
	binary.LittleEndian.PutUint32(buf[off:], uint32(desc.BCDUSB)) // Supported_USB_Version
	off += 4
	binary.LittleEndian.PutUint32(buf[off:], 0) // HcdCapabilities (must be 0)
	off += 4
	highSpeed := uint32(0)
	if desc.BCDUSB >= 0x200 {
		highSpeed = 1
	}
	binary.LittleEndian.PutUint32(buf[off:], highSpeed) // DeviceIsHighSpeed
	off += 4
	binary.LittleEndian.PutUint32(buf[off:], 0x50) // NoAckIsochWriteJitterBufferSizeInMs
	off += 4

	if err := h.dvcSend(channelID, buf[:off]); err != nil {
		h.log.LogAttrs(context.Background(), slog.LevelError, "failed to send ADD_DEVICE",
			slog.Any("err", err))
	}
}

// handleDeviceData dispatches device-specific messages (transfers, IOCTL, etc).
func (h *Handler) handleDeviceData(channelID, interfaceID, messageID, functionID uint32, body []byte) {
	h.mu.Lock()
	ds, ok := h.devByChannel[channelID]
	h.mu.Unlock()

	if !ok {
		h.log.LogAttrs(context.Background(), slog.LevelWarn, "PDU for unknown channel",
			slog.Int("channel", int(channelID)),
			slog.String("interface", fmt.Sprintf("%08x", interfaceID)))
		return
	}

	switch functionID {
	case registerRequestCallbackFn:
		h.handleRegisterRequestCallback(ds, body)
	case cancelRequestFn:
		h.handleCancelRequest(ds, body)
	case queryDeviceTextFn:
		h.handleQueryDeviceText(channelID, ds, messageID, body)
	case ioControlFn:
		h.handleIOControl(channelID, ds, messageID, body)
	case internalIOControlFn:
		h.handleInternalIOControl(channelID, ds, messageID, body)
	case transferInRequestFn:
		h.handleTransferRequest(channelID, ds, messageID, body, true)
	case transferOutRequestFn:
		h.handleTransferRequest(channelID, ds, messageID, body, false)
	case retractDeviceFn:
		h.handleRetractDevice(ds, body)
	default:
		h.log.LogAttrs(context.Background(), slog.LevelWarn, "unknown device function",
			slog.String("function", fmt.Sprintf("%08x", functionID)))
	}
}

// handleRegisterRequestCallback stores the RequestCompletion interface ID.
func (h *Handler) handleRegisterRequestCallback(ds *deviceState, body []byte) {
	if len(body) < 4 {
		return
	}
	numReqCompletion := binary.LittleEndian.Uint32(body[0:4])
	if numReqCompletion < 1 || len(body) < 8 {
		return
	}
	ds.reqCompletion = binary.LittleEndian.Uint32(body[4:8])
	h.log.LogAttrs(context.Background(), slog.LevelDebug, "REGISTER_REQUEST_CALLBACK",
		slog.String("reqCompletion", fmt.Sprintf("%08x", ds.reqCompletion)))
}

// handleCancelRequest cancels a pending transfer.
func (h *Handler) handleCancelRequest(ds *deviceState, body []byte) {
	if len(body) < 4 {
		return
	}
	cancelID := binary.LittleEndian.Uint32(body[0:4])
	h.log.LogAttrs(context.Background(), slog.LevelDebug, "CANCEL_REQUEST",
		slog.String("cancelId", fmt.Sprintf("%08x", cancelID)))
	ds.dev.CancelTransfer(cancelID)
}

// handleRetractDevice handles device retraction (blocked by policy).
func (h *Handler) handleRetractDevice(ds *deviceState, body []byte) {
	if len(body) < 4 {
		return
	}
	reason := binary.LittleEndian.Uint32(body[0:4])
	h.log.LogAttrs(context.Background(), slog.LevelWarn, "RETRACT_DEVICE",
		slog.Int("reason", int(reason)))
}

// handleQueryDeviceText responds with device description or location.
func (h *Handler) handleQueryDeviceText(channelID uint32, ds *deviceState, messageID uint32, body []byte) {
	if len(body) < 8 {
		return
	}
	textType := binary.LittleEndian.Uint32(body[0:4])
	// localeID := binary.LittleEndian.Uint32(body[4:8])

	desc := ds.dev.Descriptor()
	var text string
	switch textType {
	case deviceTextDescription:
		text = ds.dev.DeviceText()
		if text == "" {
			text = fmt.Sprintf("USB Device (VID_%04X&PID_%04X)", desc.IDVendor, desc.IDProduct)
		}
	case deviceTextLocationInformation:
		bus, addr := ds.dev.BusAddr()
		text = fmt.Sprintf("Port_#%04d.Hub_#%04d", addr, bus)
	default:
		text = ""
	}

	// Encode text as null-terminated UTF-16LE
	textUTF16 := utf16Encode(text)
	textUTF16 = append(textUTF16, 0) // null terminator
	charLen := uint32(len(textUTF16))
	byteLen := charLen * 2

	interfaceID := streamIDStub | ds.usbDeviceID
	buf := make([]byte, 12+byteLen+4)
	binary.LittleEndian.PutUint32(buf[0:4], interfaceID)
	binary.LittleEndian.PutUint32(buf[4:8], messageID)
	binary.LittleEndian.PutUint32(buf[8:12], charLen) // FunctionId = charLen (character count)
	off := 12
	for _, ch := range textUTF16 {
		binary.LittleEndian.PutUint16(buf[off:], ch)
		off += 2
	}
	binary.LittleEndian.PutUint32(buf[off:], 0) // HRESULT = S_OK

	if err := h.dvcSend(channelID, buf); err != nil {
		h.log.LogAttrs(context.Background(), slog.LevelError, "failed to send QUERY_DEVICE_TEXT response",
			slog.Any("err", err))
	}
}

// handleIOControl handles IO_CONTROL requests.
func (h *Handler) handleIOControl(channelID uint32, ds *deviceState, messageID uint32, body []byte) {
	if len(body) < 8 {
		return
	}
	ioControlCode := binary.LittleEndian.Uint32(body[0:4])
	inputBufSize := binary.LittleEndian.Uint32(body[4:8])

	off := 8 + int(inputBufSize)
	if len(body) < off+8 {
		return
	}
	outputBufSize := binary.LittleEndian.Uint32(body[off : off+4])
	requestID := binary.LittleEndian.Uint32(body[off+4 : off+8])

	h.log.LogAttrs(context.Background(), slog.LevelDebug, "IO_CONTROL",
		slog.String("code", fmt.Sprintf("%08x", ioControlCode)),
		slog.Int("requestId", int(requestID)))

	// Build IOCONTROL_COMPLETION
	respOutSize := outputBufSize + 4
	interfaceID := streamIDProxy | ds.reqCompletion

	buf := make([]byte, 12+16+respOutSize)
	binary.LittleEndian.PutUint32(buf[0:4], interfaceID)
	binary.LittleEndian.PutUint32(buf[4:8], messageID)
	binary.LittleEndian.PutUint32(buf[8:12], ioControlCompletionFn)
	binary.LittleEndian.PutUint32(buf[12:16], requestID)
	binary.LittleEndian.PutUint32(buf[16:20], 0) // HResult = S_OK
	binary.LittleEndian.PutUint32(buf[20:24], respOutSize)
	binary.LittleEndian.PutUint32(buf[24:28], respOutSize)
	outOff := 28

	switch ioControlCode {
	case ioctlInternalUSBGetPortStatus:
		desc := ds.dev.Descriptor()
		var portStatus uint32
		switch {
		case desc.BCDUSB >= 0x200:
			portStatus = 0x503
		case desc.BCDUSB >= 0x110:
			portStatus = 0x103
		default:
			portStatus = 0x303
		}
		binary.LittleEndian.PutUint32(buf[outOff:], portStatus)
	case ioctlInternalUSBResetPort:
		// Just succeed
	case ioctlInternalUSBCyclePort:
		// Just succeed
	case ioctlInternalUSBSubmitIdleNotification:
		// Just succeed
	default:
		h.log.LogAttrs(context.Background(), slog.LevelWarn, "unknown IO_CONTROL code",
			slog.String("code", fmt.Sprintf("%08x", ioControlCode)))
	}

	if err := h.dvcSend(channelID, buf); err != nil {
		h.log.LogAttrs(context.Background(), slog.LevelError, "failed to send IOCONTROL_COMPLETION",
			slog.Any("err", err))
	}
}

// handleInternalIOControl handles INTERNAL_IO_CONTROL (bus time query).
func (h *Handler) handleInternalIOControl(channelID uint32, ds *deviceState, messageID uint32, body []byte) {
	if len(body) < 8 {
		return
	}
	ioControlCode := binary.LittleEndian.Uint32(body[0:4])
	inputBufSize := binary.LittleEndian.Uint32(body[4:8])

	off := 8 + int(inputBufSize)
	if len(body) < off+8 {
		return
	}
	// outputBufSize := binary.LittleEndian.Uint32(body[off : off+4])
	requestID := binary.LittleEndian.Uint32(body[off+4 : off+8])

	if ioControlCode != ioctlTSUSBGDQueryBusTime {
		h.log.LogAttrs(context.Background(), slog.LevelWarn, "unexpected INTERNAL_IO_CONTROL code",
			slog.String("code", fmt.Sprintf("%08x", ioControlCode)))
		return
	}

	// Return tick count as frame number (MS-RDPEUSB 2.2.10.5)
	frames := uint32(time.Now().UnixMilli())
	outputSize := uint32(4)
	interfaceID := streamIDProxy | ds.reqCompletion

	buf := make([]byte, 12+16+4)
	binary.LittleEndian.PutUint32(buf[0:4], interfaceID)
	binary.LittleEndian.PutUint32(buf[4:8], messageID)
	binary.LittleEndian.PutUint32(buf[8:12], ioControlCompletionFn)
	binary.LittleEndian.PutUint32(buf[12:16], requestID)
	binary.LittleEndian.PutUint32(buf[16:20], 0)          // HResult = S_OK
	binary.LittleEndian.PutUint32(buf[20:24], outputSize)  // Information
	binary.LittleEndian.PutUint32(buf[24:28], outputSize)  // OutputBufferSize
	binary.LittleEndian.PutUint32(buf[28:32], frames)

	if err := h.dvcSend(channelID, buf); err != nil {
		h.log.LogAttrs(context.Background(), slog.LevelError, "failed to send INTERNAL_IO_CONTROL response",
			slog.Any("err", err))
	}
}

// --- Helpers ---

// utf16Encode encodes a Go string as a slice of UTF-16 code units.
func utf16Encode(s string) []uint16 {
	return utf16.Encode([]rune(s))
}

// writeStringBlock writes a length-prefixed UTF-16LE string to buf.
// Returns bytes written.
func writeStringBlock(buf []byte, utf16Chars []uint16, multiSZ bool) int {
	charLen := len(utf16Chars) + 1 // +1 for null terminator
	if multiSZ {
		charLen++ // final multi-SZ null
	}
	off := 0
	binary.LittleEndian.PutUint32(buf[off:], uint32(charLen))
	off += 4
	for _, ch := range utf16Chars {
		binary.LittleEndian.PutUint16(buf[off:], ch)
		off += 2
	}
	binary.LittleEndian.PutUint16(buf[off:], 0) // null terminator
	off += 2
	if multiSZ {
		binary.LittleEndian.PutUint16(buf[off:], 0) // multi-SZ final null
		off += 2
	}
	return off
}

// writeMultiStringBlock writes a multi-SZ UTF-16LE string block.
func writeMultiStringBlock(buf []byte, strings [][]uint16) int {
	totalChars := 0
	for _, s := range strings {
		totalChars += len(s) + 1 // +1 for per-string null
	}
	totalChars++ // multi-SZ final null

	off := 0
	binary.LittleEndian.PutUint32(buf[off:], uint32(totalChars))
	off += 4
	for _, s := range strings {
		for _, ch := range s {
			binary.LittleEndian.PutUint16(buf[off:], ch)
			off += 2
		}
		binary.LittleEndian.PutUint16(buf[off:], 0) // null terminator
		off += 2
	}
	binary.LittleEndian.PutUint16(buf[off:], 0) // multi-SZ final null
	off += 2
	return off
}

// generateInstanceID generates a GUID-like instance ID from device path.
func generateInstanceID(path string) string {
	var id [16]byte
	p := "\\" + path
	copy(id[:], p)
	return fmt.Sprintf("%02x%02x%02x%02x-%02x%02x-%02x%02x-%02x%02x-%02x%02x%02x%02x%02x%02x",
		id[0], id[1], id[2], id[3], id[4], id[5], id[6], id[7],
		id[8], id[9], id[10], id[11], id[12], id[13], id[14], id[15])
}

// generateContainerID generates a GUID-like container ID from VID/PID/path.
func generateContainerID(vid, pid uint16, path string) string {
	var id [16]byte
	s := fmt.Sprintf("%04X%04X", vid, pid)
	if len(path) > 8 {
		s += path[len(path)-8:]
	} else {
		s += path
	}
	copy(id[:], s)
	return fmt.Sprintf("{%02x%02x%02x%02x-%02x%02x-%02x%02x-%02x%02x-%02x%02x%02x%02x%02x%02x}",
		id[0], id[1], id[2], id[3], id[4], id[5], id[6], id[7],
		id[8], id[9], id[10], id[11], id[12], id[13], id[14], id[15])
}
