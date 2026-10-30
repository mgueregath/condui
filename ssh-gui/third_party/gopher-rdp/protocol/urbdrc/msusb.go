package urbdrc

import "encoding/binary"

// MSUSBConfig represents a Windows USB configuration descriptor (MS-RDPEUSB 2.2.10).
type MSUSBConfig struct {
	WTotalLength        uint16
	BConfigurationValue uint8
	ConfigurationHandle uint32
	NumInterfaces       uint32
	Interfaces          []*MSUSBInterface
}

// MSUSBInterface represents a Windows USB interface descriptor.
type MSUSBInterface struct {
	Length                 uint16
	NumberOfPipesExpected  uint16
	InterfaceNumber        uint8
	AlternateSetting       uint8
	NumberOfPipes          uint32
	InterfaceHandle        uint32
	BInterfaceClass        uint8
	BInterfaceSubClass     uint8
	BInterfaceProtocol     uint8
	Pipes                  []*MSUSBPipe
}

// MSUSBPipe represents a Windows USB pipe descriptor.
type MSUSBPipe struct {
	MaximumPacketSize   uint16
	MaximumTransferSize uint32
	PipeFlags           uint32
	PipeHandle          uint32
	BEndpointAddress    uint8
	BInterval           uint8
	PipeType            uint8
}

// ReadMSUSBConfig reads a MS USB configuration descriptor from wire format.
// This is the format used in TS_URB_SELECT_CONFIGURATION_RESULT.
func ReadMSUSBConfig(data []byte, numInterfaces uint32) (*MSUSBConfig, int) {
	cfg := &MSUSBConfig{NumInterfaces: numInterfaces}
	off := 0

	cfg.Interfaces = make([]*MSUSBInterface, numInterfaces)
	for i := uint32(0); i < numInterfaces; i++ {
		iface, n := readMSUSBInterface(data[off:])
		if iface == nil {
			return nil, 0
		}
		cfg.Interfaces[i] = iface
		off += n
	}

	// Configuration descriptor header: Length(1) + Type(1) + wTotalLength(2) + skip(1) + bConfigurationValue(1)
	if len(data[off:]) < 6 {
		return nil, 0
	}
	lenByte := data[off]
	typeByte := data[off+1]
	if lenByte != 0x09 || typeByte != 0x02 {
		return nil, 0
	}
	cfg.WTotalLength = binary.LittleEndian.Uint16(data[off+2 : off+4])
	// skip bNumInterfaces at off+4
	cfg.BConfigurationValue = data[off+5]
	off += 6

	return cfg, off
}

// WriteMSUSBConfig writes a MS USB configuration descriptor to wire format.
func WriteMSUSBConfig(cfg *MSUSBConfig) []byte {
	size := 8 // ConfigurationHandle(4) + NumInterfaces(4)
	for _, iface := range cfg.Interfaces {
		size += msusbInterfaceWriteSize(iface)
	}

	buf := make([]byte, size)
	off := 0
	binary.LittleEndian.PutUint32(buf[off:], cfg.ConfigurationHandle)
	off += 4
	binary.LittleEndian.PutUint32(buf[off:], cfg.NumInterfaces)
	off += 4

	for _, iface := range cfg.Interfaces {
		n := writeMSUSBInterface(buf[off:], iface)
		off += n
	}
	return buf[:off]
}

func readMSUSBInterface(data []byte) (*MSUSBInterface, int) {
	if len(data) < 12 {
		return nil, 0
	}
	iface := &MSUSBInterface{}
	iface.Length = binary.LittleEndian.Uint16(data[0:2])
	iface.NumberOfPipesExpected = binary.LittleEndian.Uint16(data[2:4])
	iface.InterfaceNumber = data[4]
	iface.AlternateSetting = data[5]
	// 2 bytes padding at [6:8]
	iface.NumberOfPipes = binary.LittleEndian.Uint32(data[8:12])
	off := 12

	if iface.NumberOfPipes > 0 {
		need := int(iface.NumberOfPipes) * 12
		if len(data[off:]) < need {
			return nil, 0
		}
		iface.Pipes = make([]*MSUSBPipe, iface.NumberOfPipes)
		for i := uint32(0); i < iface.NumberOfPipes; i++ {
			p := &MSUSBPipe{}
			p.MaximumPacketSize = binary.LittleEndian.Uint16(data[off : off+2])
			// 2 bytes reserved at [off+2:off+4]
			p.MaximumTransferSize = binary.LittleEndian.Uint32(data[off+4 : off+8])
			p.PipeFlags = binary.LittleEndian.Uint32(data[off+8 : off+12])
			iface.Pipes[i] = p
			off += 12
		}
	}

	return iface, off
}

func writeMSUSBInterface(buf []byte, iface *MSUSBInterface) int {
	off := 0
	// Length must reflect the RESULT format (16 + pipes×20), not the
	// REQUEST format (12 + pipes×12). The length must be recalculated
	// for the RESULT format before sending (MS-RDPEUSB 2.2.11).
	binary.LittleEndian.PutUint16(buf[off:], uint16(msusbInterfaceWriteSize(iface)))
	off += 2
	buf[off] = iface.InterfaceNumber
	off++
	buf[off] = iface.AlternateSetting
	off++
	buf[off] = iface.BInterfaceClass
	off++
	buf[off] = iface.BInterfaceSubClass
	off++
	buf[off] = iface.BInterfaceProtocol
	off++
	buf[off] = 0 // padding
	off++
	binary.LittleEndian.PutUint32(buf[off:], iface.InterfaceHandle)
	off += 4
	binary.LittleEndian.PutUint32(buf[off:], iface.NumberOfPipes)
	off += 4

	for _, p := range iface.Pipes {
		binary.LittleEndian.PutUint16(buf[off:], p.MaximumPacketSize)
		off += 2
		buf[off] = p.BEndpointAddress
		off++
		buf[off] = p.BInterval
		off++
		binary.LittleEndian.PutUint32(buf[off:], uint32(p.PipeType))
		off += 4
		binary.LittleEndian.PutUint32(buf[off:], p.PipeHandle)
		off += 4
		binary.LittleEndian.PutUint32(buf[off:], p.MaximumTransferSize)
		off += 4
		binary.LittleEndian.PutUint32(buf[off:], p.PipeFlags)
		off += 4
	}
	return off
}

func msusbInterfaceWriteSize(iface *MSUSBInterface) int {
	return 16 + int(iface.NumberOfPipes)*20
}

// WriteMSUSBInterfaceResult writes a single interface for TS_URB_SELECT_INTERFACE_RESULT.
func WriteMSUSBInterfaceResult(iface *MSUSBInterface) []byte {
	size := msusbInterfaceWriteSize(iface)
	buf := make([]byte, size)
	writeMSUSBInterface(buf, iface)
	return buf
}
