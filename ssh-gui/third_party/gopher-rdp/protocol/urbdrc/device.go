package urbdrc

// DeviceDescriptor holds the standard USB device descriptor fields.
type DeviceDescriptor struct {
	BLength            uint8
	BDescriptorType    uint8
	BCDUSB             uint16
	BDeviceClass       uint8
	BDeviceSubClass    uint8
	BDeviceProtocol    uint8
	BMaxPacketSize0    uint8
	IDVendor           uint16
	IDProduct          uint16
	BCDDevice          uint16
	IManufacturer      uint8
	IProduct           uint8
	ISerialNumber      uint8
	BNumConfigurations uint8
}

// USBDevice is the interface that platform-specific USB backends must implement.
type USBDevice interface {
	// Descriptor returns the cached USB device descriptor.
	Descriptor() DeviceDescriptor

	// Path returns the device path string (e.g. "1-4" for bus 1, port 4).
	Path() string

	// BusAddr returns the USB bus number and device address.
	BusAddr() (bus uint8, addr uint8)

	// IsComposite returns true if the device has multiple interfaces
	// with bDeviceClass == 0 (per-interface).
	IsComposite() bool

	// DeviceText returns a human-readable device description string.
	// Returns empty string if unavailable.
	DeviceText() string

	// DetachKernelDriver detaches any kernel drivers from all interfaces.
	DetachKernelDriver() error

	// SelectConfiguration selects the given USB configuration.
	// Pass 0 to unconfigure.
	SelectConfiguration(bConfigurationValue uint8) error

	// SelectInterface selects an alternate setting for the given interface.
	SelectInterface(interfaceNumber, alternateSetting uint8) error

	// ControlTransfer performs a synchronous USB control transfer.
	// Returns the actual bytes transferred and a USBD status code.
	ControlTransfer(bmRequestType, bRequest uint8, wValue, wIndex uint16,
		data []byte, timeout uint32) (int, uint32)

	// BulkOrInterruptTransfer performs a synchronous bulk or interrupt transfer.
	// endpointAddr includes the direction bit (bit 7: 1=IN, 0=OUT).
	// Returns actual bytes transferred and a USBD status code.
	BulkOrInterruptTransfer(endpointAddr uint8, data []byte, timeout uint32) (int, uint32)

	// IsochTransfer performs an isochronous transfer.
	// packets contains per-packet offset/length descriptors.
	// Returns per-packet results, actual data, and USBD status code.
	IsochTransfer(endpointAddr uint8, transferFlags uint32, startFrame uint32,
		packets []IsochPacket, data []byte, timeout uint32) ([]IsochPacketResult, []byte, uint32)

	// ClearHalt clears a halt/stall condition on the given endpoint.
	ClearHalt(endpointAddr uint8) error

	// CancelTransfer cancels a pending async transfer by request ID.
	CancelTransfer(requestID uint32)

	// GetActiveConfig returns the current configuration descriptor in
	// the MS USB format for the RDP protocol.
	GetActiveConfig() *MSUSBConfig

	// CompleteConfig fills in the backend-specific pipe handles and
	// interface handles for a configuration received from the server.
	CompleteConfig(cfg *MSUSBConfig) *MSUSBConfig

	// Close releases the device.
	Close()
}

// IsochPacket describes a single packet in an isochronous transfer request.
type IsochPacket struct {
	Offset uint32
	Length uint32
	Status uint32
}

// IsochPacketResult describes the result of a single isochronous packet.
type IsochPacketResult struct {
	Offset uint32
	Length uint32
	Status uint32
}
