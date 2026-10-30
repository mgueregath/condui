// Package sec implements RDP security header parsing (MS-RDPBCGR section 2.2.8.1).
package sec

import (
	"encoding/binary"
	"fmt"
)

// Security header flags (SEC_*)
const (
	ExchangePkt      uint16 = 0x0001
	TransportReq     uint16 = 0x0002
	Encrypt          uint16 = 0x0008
	ResetSeqno       uint16 = 0x0010
	IgnoreSeqno      uint16 = 0x0020
	InfoPkt          uint16 = 0x0040
	LicensePkt       uint16 = 0x0080
	LicenseEncryptCS uint16 = 0x0200
	LicenseEncryptSC uint16 = 0x0200
	RedirectionPkt   uint16 = 0x0400
	SecureChecksum   uint16 = 0x0800
	AutodetectReq    uint16 = 0x1000
	AutodetectRsp    uint16 = 0x2000
	Heartbeat        uint16 = 0x4000
	FlagshiValid     uint16 = 0x8000
)

// BasicSecurityHeader represents the 4-byte RDP Basic Security Header.
type BasicSecurityHeader struct {
	Flags   uint16 // SEC_* flags (little-endian)
	FlagsHi uint16 // High flags
}

// DecodeBasicSecurityHeader parses a 4-byte Basic Security Header from data.
// Returns the header and the remaining data after the header.
func DecodeBasicSecurityHeader(data []byte) (BasicSecurityHeader, []byte, error) {
	if len(data) < 4 {
		return BasicSecurityHeader{}, nil, fmt.Errorf("security header too short: %d bytes, need 4", len(data))
	}

	hdr := BasicSecurityHeader{
		Flags:   binary.LittleEndian.Uint16(data[0:2]),
		FlagsHi: binary.LittleEndian.Uint16(data[2:4]),
	}
	return hdr, data[4:], nil
}

// EncodeBasicSecurityHeader serializes a Basic Security Header to 4 bytes.
func EncodeBasicSecurityHeader(hdr BasicSecurityHeader) []byte {
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint16(buf[0:2], hdr.Flags)
	binary.LittleEndian.PutUint16(buf[2:4], hdr.FlagsHi)
	return buf
}
