// Server Redirection PDU decoding (MS-RDPBCGR 2.2.13.1).
package pdu

import (
	"encoding/binary"
	"fmt"
	"unicode/utf16"
)

// Share Control PDU types for server redirection.
const (
	TypeRedirect         uint16 = 0x0014 // type=4, version=0x0010
	TypeEnhancedRedirect uint16 = 0x001A // type=10, version=0x0010
)

// Redirection flags (MS-RDPBCGR 2.2.13.1).
const (
	LBTargetNetAddress      uint32 = 0x00000001
	LBLoadBalanceInfo       uint32 = 0x00000002
	LBUsername              uint32 = 0x00000004
	LBDomain               uint32 = 0x00000008
	LBPassword              uint32 = 0x00000010
	LBDontStoreUsername     uint32 = 0x00000020
	LBSmartcardLogon        uint32 = 0x00000040
	LBNoRedirect            uint32 = 0x00000080
	LBTargetFQDN            uint32 = 0x00000100
	LBTargetNetBIOS         uint32 = 0x00000200
	LBTargetNetAddresses    uint32 = 0x00000800
	LBClientTSVURL          uint32 = 0x00001000
	LBServerTSVCapable      uint32 = 0x00002000
	LBPasswordIsPKEncrypted uint32 = 0x00004000
	LBRedirectionGUID       uint32 = 0x00008000
	LBTargetCertificate     uint32 = 0x00010000
)

// RedirectInfo holds parsed Server Redirection PDU fields.
type RedirectInfo struct {
	SessionID       uint32 // only set for enhanced redirect
	Flags           uint32
	Server          string // target server (from LBTargetNetAddress or LBTargetFQDN)
	Domain          string
	Username        string
	Password        []byte // opaque redirection password/cookie
	LoadBalanceInfo []byte // load balancer routing token
}

// DecodeRedirectPDU parses a Server Redirection PDU payload.
// enhanced indicates whether this is a Standard (type 4) or Enhanced (type 10) redirect.
// data starts after the Share Control Header.
func DecodeRedirectPDU(data []byte, enhanced bool) (*RedirectInfo, error) {
	off := 0

	// 2 bytes padding
	if off+2 > len(data) {
		return nil, fmt.Errorf("redirect PDU too short for padding")
	}
	off += 2

	ri := &RedirectInfo{}

	if enhanced {
		// Enhanced: redirectIdentifier(u16) + totalLength(u16) + sessionID(u32)
		if off+8 > len(data) {
			return nil, fmt.Errorf("redirect PDU too short for enhanced header")
		}
		off += 2 // redirectIdentifier (0x0400)
		off += 2 // totalLength
		ri.SessionID = binary.LittleEndian.Uint32(data[off : off+4])
		off += 4
	}

	// redirFlags(u32)
	if off+4 > len(data) {
		return nil, fmt.Errorf("redirect PDU too short for flags")
	}
	ri.Flags = binary.LittleEndian.Uint32(data[off : off+4])
	off += 4

	if ri.Flags&LBTargetNetAddress != 0 {
		s, n, err := readRedirectUTF16(data, off)
		if err != nil {
			return nil, fmt.Errorf("TargetNetAddress: %w", err)
		}
		ri.Server = s
		off += n
	}

	if ri.Flags&LBLoadBalanceInfo != 0 {
		b, n, err := readRedirectBlob(data, off)
		if err != nil {
			return nil, fmt.Errorf("LoadBalanceInfo: %w", err)
		}
		ri.LoadBalanceInfo = b
		off += n
	}

	if ri.Flags&LBUsername != 0 {
		s, n, err := readRedirectUTF16(data, off)
		if err != nil {
			return nil, fmt.Errorf("Username: %w", err)
		}
		ri.Username = s
		off += n
	}

	if ri.Flags&LBDomain != 0 {
		s, n, err := readRedirectUTF16(data, off)
		if err != nil {
			return nil, fmt.Errorf("Domain: %w", err)
		}
		ri.Domain = s
		off += n
	}

	if ri.Flags&LBPassword != 0 {
		b, n, err := readRedirectBlob(data, off)
		if err != nil {
			return nil, fmt.Errorf("Password: %w", err)
		}
		ri.Password = b
		off += n
	}

	if ri.Flags&LBTargetFQDN != 0 {
		s, n, err := readRedirectUTF16(data, off)
		if err != nil {
			return nil, fmt.Errorf("TargetFQDN: %w", err)
		}
		// FQDN overrides NetAddress
		ri.Server = s
		off += n
	}

	if ri.Flags&LBNoRedirect != 0 {
		ri.Server = ""
	}

	return ri, nil
}

// readRedirectBlob reads a length(u32)-prefixed opaque blob.
// Returns the data and total bytes consumed (4 + length).
func readRedirectBlob(data []byte, off int) ([]byte, int, error) {
	if off+4 > len(data) {
		return nil, 0, fmt.Errorf("truncated at length")
	}
	blobLen := binary.LittleEndian.Uint32(data[off : off+4])
	off2 := off + 4
	if off2+int(blobLen) > len(data) {
		return nil, 0, fmt.Errorf("truncated at data (need %d, have %d)", blobLen, len(data)-off2)
	}
	b := make([]byte, blobLen)
	copy(b, data[off2:off2+int(blobLen)])
	return b, 4 + int(blobLen), nil
}

// readRedirectUTF16 reads a length(u32)-prefixed null-terminated UTF-16LE string.
// Returns the Go string and total bytes consumed (4 + length).
func readRedirectUTF16(data []byte, off int) (string, int, error) {
	if off+4 > len(data) {
		return "", 0, fmt.Errorf("truncated at length")
	}
	strLen := binary.LittleEndian.Uint32(data[off : off+4])
	off2 := off + 4
	if off2+int(strLen) > len(data) {
		return "", 0, fmt.Errorf("truncated at data (need %d, have %d)", strLen, len(data)-off2)
	}
	s := decodeUTF16(data[off2 : off2+int(strLen)])
	return s, 4 + int(strLen), nil
}

// decodeUTF16 converts a null-terminated UTF-16LE byte slice to a Go string.
func decodeUTF16(b []byte) string {
	if len(b) < 2 {
		return ""
	}
	n := len(b) / 2
	u16s := make([]uint16, n)
	for i := range n {
		u16s[i] = binary.LittleEndian.Uint16(b[i*2 : i*2+2])
	}
	// Strip null terminator
	if len(u16s) > 0 && u16s[len(u16s)-1] == 0 {
		u16s = u16s[:len(u16s)-1]
	}
	return string(utf16.Decode(u16s))
}
