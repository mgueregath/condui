// Package lic implements RDP licensing PDU handling (MS-RDPBCGR section 2.2.1.12).
package lic

import (
	"context"
	"encoding/binary"
	"fmt"
	"log/slog"

	"github.com/bouncyball-git/gopher-rdp/util"
)

// Licensing message types
const (
	LicenseRequest             byte = 0x01
	PlatformChallenge          byte = 0x02
	NewLicense                 byte = 0x03
	UpgradeLicense             byte = 0x04
	LicenseInfo                byte = 0x12
	NewLicenseRequest          byte = 0x13
	PlatformChallengeResponse  byte = 0x15
	ErrorAlert                 byte = 0xFF
)

// Licensing error codes
const (
	ErrInvalidServerCertificate uint32 = 0x00000001
	ErrNoLicense                uint32 = 0x00000002
	ErrInvalidScope             uint32 = 0x00000004
	ErrInvalidMac               uint32 = 0x00000003
	StatusValidClient           uint32 = 0x00000007
)

// Licensing state transition codes
const (
	STTotalAbort    uint32 = 0x00000001
	STNoTransition  uint32 = 0x00000002
	STResetPhaseToStart uint32 = 0x00000003
	STResendLastMessage uint32 = 0x00000004
)

// Preamble flags
const (
	PreambleVersion20 byte = 0x02
	PreambleVersion30 byte = 0x03
)

// Preamble represents the 4-byte licensing preamble that precedes all licensing PDUs.
type Preamble struct {
	MsgType byte
	Flags   byte
	MsgSize uint16
}

// DecodePreamble parses a 4-byte licensing preamble.
// Returns the preamble and the remaining data after it.
func DecodePreamble(log *slog.Logger, data []byte) (Preamble, []byte, error) {
	if len(data) < 4 {
		return Preamble{}, nil, fmt.Errorf("licensing preamble too short: %d bytes, need 4", len(data))
	}

	p := Preamble{
		MsgType: data[0],
		Flags:   data[1],
		MsgSize: binary.LittleEndian.Uint16(data[2:4]),
	}
	log.LogAttrs(context.Background(), slog.LevelDebug, "license preamble", util.Hex2("msgType", p.MsgType), util.Hex2("flags", p.Flags), slog.Int("msgSize", int(p.MsgSize)))
	return p, data[4:], nil
}

// IsValidClientError checks if licensing data represents an ERROR_ALERT with
// STATUS_VALID_CLIENT — the common success case for TLS connections where
// the server indicates no license is needed.
func IsValidClientError(data []byte) bool {
	// Need at least: preamble(4) + dwErrorCode(4) + dwStateTransition(4) = 12 bytes
	if len(data) < 12 {
		return false
	}

	// Check message type is ERROR_ALERT
	if data[0] != ErrorAlert {
		return false
	}

	// dwErrorCode starts at offset 4 (after preamble)
	errorCode := binary.LittleEndian.Uint32(data[4:8])
	return errorCode == StatusValidClient
}

// Blob type constants (MS-RDPELE 2.2.1.12.1.2)
const (
	BBDataBlob        uint16 = 0x0001
	BBRandomBlob      uint16 = 0x0002
	BBCertificateBlob uint16 = 0x0003
	BBErrorBlob       uint16 = 0x0004
	BBEncryptedBlob   uint16 = 0x0009
	BBKeyExchAlgBlob  uint16 = 0x000D
	BBScopeBlob       uint16 = 0x000E
	BBAnyBlob         uint16 = 0xFFFF
)

// Key exchange algorithm
const (
	KeyExchAlgRSA uint32 = 0x00000001
)

// Platform ID constants
const (
	// ClientOSID = WINDOWS(0x04) << 8 shifted into high word
	PlatformIDWindows uint32 = 0x04000000
	// ISV = 0x00010000
	PlatformISV uint32 = 0x00010000
	// Combined typical client platform ID
	ClientPlatformID uint32 = 0x04000401
)

// Blob is a LICENSE_BINARY_BLOB (type(u16) + length(u16) + data).
type Blob struct {
	Type uint16
	Data []byte
}

// DecodeBlob parses a LICENSE_BINARY_BLOB from data.
// Returns the blob and remaining data.
func DecodeBlob(data []byte) (Blob, []byte, error) {
	if len(data) < 4 {
		return Blob{}, nil, fmt.Errorf("license blob too short: %d bytes", len(data))
	}
	b := Blob{
		Type: binary.LittleEndian.Uint16(data[0:2]),
	}
	blobLen := binary.LittleEndian.Uint16(data[2:4])
	if len(data) < 4+int(blobLen) {
		return Blob{}, nil, fmt.Errorf("license blob truncated: need %d, have %d", 4+int(blobLen), len(data))
	}
	if blobLen > 0 {
		b.Data = make([]byte, blobLen)
		copy(b.Data, data[4:4+int(blobLen)])
	}
	return b, data[4+int(blobLen):], nil
}

// EncodeBlob serializes a LICENSE_BINARY_BLOB.
func EncodeBlob(b Blob) []byte {
	buf := make([]byte, 4+len(b.Data))
	binary.LittleEndian.PutUint16(buf[0:2], b.Type)
	binary.LittleEndian.PutUint16(buf[2:4], uint16(len(b.Data)))
	copy(buf[4:], b.Data)
	return buf
}

// EncodePreamble serializes a 4-byte licensing preamble.
func EncodePreamble(msgType, flags byte, msgSize uint16) []byte {
	buf := make([]byte, 4)
	buf[0] = msgType
	buf[1] = flags
	binary.LittleEndian.PutUint16(buf[2:4], msgSize)
	return buf
}

// ProductInfo from SERVER_LICENSE_REQUEST (MS-RDPELE 2.2.2.1.1).
type ProductInfo struct {
	Version    uint32
	CompanyLen uint32
	Company    []byte
	ProductLen uint32
	Product    []byte
}

// LicenseRequestData holds parsed SERVER_LICENSE_REQUEST fields.
type LicenseRequestData struct {
	ServerRandom    []byte // 32 bytes
	ProductInfo     ProductInfo
	KeyExchangeList Blob
	ServerCert      []byte // raw certificate bytes for sec.DecodeServerCertificate
	ScopeList       []Blob
}

// DecodeLicenseRequest parses a SERVER_LICENSE_REQUEST after the preamble.
//
// Layout: ServerRandom(32) + ProductInfo + KeyExchangeList(blob) +
//
//	ServerCertLen(u32) + ServerCert + ScopeCount(u32) + ScopeArray
func DecodeLicenseRequest(data []byte) (*LicenseRequestData, error) {
	if len(data) < 32 {
		return nil, fmt.Errorf("license request too short: %d bytes", len(data))
	}

	lr := &LicenseRequestData{
		ServerRandom: make([]byte, 32),
	}
	copy(lr.ServerRandom, data[0:32])
	off := 32

	// ProductInfo: dwVersion(4) + cbCompanyName(4) + Company + cbProductId(4) + Product
	if off+8 > len(data) {
		return nil, fmt.Errorf("license request truncated at product info")
	}
	lr.ProductInfo.Version = binary.LittleEndian.Uint32(data[off : off+4])
	off += 4
	lr.ProductInfo.CompanyLen = binary.LittleEndian.Uint32(data[off : off+4])
	off += 4
	if off+int(lr.ProductInfo.CompanyLen) > len(data) {
		return nil, fmt.Errorf("license request truncated at company name")
	}
	lr.ProductInfo.Company = make([]byte, lr.ProductInfo.CompanyLen)
	copy(lr.ProductInfo.Company, data[off:off+int(lr.ProductInfo.CompanyLen)])
	off += int(lr.ProductInfo.CompanyLen)

	if off+4 > len(data) {
		return nil, fmt.Errorf("license request truncated at product id length")
	}
	lr.ProductInfo.ProductLen = binary.LittleEndian.Uint32(data[off : off+4])
	off += 4
	if off+int(lr.ProductInfo.ProductLen) > len(data) {
		return nil, fmt.Errorf("license request truncated at product id")
	}
	lr.ProductInfo.Product = make([]byte, lr.ProductInfo.ProductLen)
	copy(lr.ProductInfo.Product, data[off:off+int(lr.ProductInfo.ProductLen)])
	off += int(lr.ProductInfo.ProductLen)

	// KeyExchangeList blob
	var err error
	lr.KeyExchangeList, data, err = DecodeBlob(data[off:])
	if err != nil {
		return nil, fmt.Errorf("license request key exchange list: %w", err)
	}
	rest := data

	// ServerCertificate blob (same blob encoding)
	var certBlob Blob
	certBlob, rest, err = DecodeBlob(rest)
	if err != nil {
		return nil, fmt.Errorf("license request server cert: %w", err)
	}
	lr.ServerCert = certBlob.Data

	// ScopeList: ScopeCount(u32) + ScopeArray[blob...]
	if len(rest) < 4 {
		return nil, fmt.Errorf("license request truncated at scope count")
	}
	scopeCount := binary.LittleEndian.Uint32(rest[0:4])
	rest = rest[4:]

	lr.ScopeList = make([]Blob, 0, scopeCount)
	for i := uint32(0); i < scopeCount; i++ {
		var scope Blob
		scope, rest, err = DecodeBlob(rest)
		if err != nil {
			return nil, fmt.Errorf("license request scope %d: %w", i, err)
		}
		lr.ScopeList = append(lr.ScopeList, scope)
	}

	return lr, nil
}

// EncodeNewLicenseRequest builds a CLIENT_NEW_LICENSE_REQUEST PDU (including preamble).
//
// Layout: Preamble(4) + KeyExchangeAlg(u32) + PlatformId(u32) + ClientRandom(32) +
//
//	EncryptedPreMaster(blob) + ClientUserName(blob) + ClientMachineName(blob)
func EncodeNewLicenseRequest(clientRandom, encryptedPreMaster []byte, username, hostname string) []byte {
	// Null-terminate the strings (UTF-8 / ASCII)
	userBytes := append([]byte(username), 0)
	hostBytes := append([]byte(hostname), 0)

	userBlob := EncodeBlob(Blob{Type: BBDataBlob, Data: userBytes})
	hostBlob := EncodeBlob(Blob{Type: BBDataBlob, Data: hostBytes})
	preMasterBlob := EncodeBlob(Blob{Type: BBRandomBlob, Data: encryptedPreMaster})

	// Body: KeyExchAlg(4) + PlatformId(4) + ClientRandom(32) + 3 blobs
	bodyLen := 4 + 4 + 32 + len(preMasterBlob) + len(userBlob) + len(hostBlob)
	msgSize := 4 + bodyLen // preamble + body

	buf := make([]byte, 0, msgSize)
	buf = append(buf, EncodePreamble(NewLicenseRequest, PreambleVersion30, uint16(msgSize))...)

	var tmp [4]byte
	binary.LittleEndian.PutUint32(tmp[:], KeyExchAlgRSA)
	buf = append(buf, tmp[:]...)
	binary.LittleEndian.PutUint32(tmp[:], ClientPlatformID)
	buf = append(buf, tmp[:]...)
	buf = append(buf, clientRandom...)
	buf = append(buf, preMasterBlob...)
	buf = append(buf, userBlob...)
	buf = append(buf, hostBlob...)

	return buf
}

// PlatformChallengeData holds parsed SERVER_PLATFORM_CHALLENGE fields.
type PlatformChallengeData struct {
	ConnectFlags       uint32
	EncryptedChallenge Blob
	MACData            []byte // 16 bytes
}

// DecodePlatformChallenge parses a SERVER_PLATFORM_CHALLENGE after the preamble.
//
// Layout: ConnectFlags(u32) + EncryptedPlatformChallenge(blob) + MACData(16)
func DecodePlatformChallenge(data []byte) (*PlatformChallengeData, error) {
	if len(data) < 4 {
		return nil, fmt.Errorf("platform challenge too short: %d bytes", len(data))
	}

	pc := &PlatformChallengeData{
		ConnectFlags: binary.LittleEndian.Uint32(data[0:4]),
	}

	var err error
	var rest []byte
	pc.EncryptedChallenge, rest, err = DecodeBlob(data[4:])
	if err != nil {
		return nil, fmt.Errorf("platform challenge blob: %w", err)
	}

	if len(rest) < 16 {
		return nil, fmt.Errorf("platform challenge MAC too short: %d bytes", len(rest))
	}
	pc.MACData = make([]byte, 16)
	copy(pc.MACData, rest[0:16])

	return pc, nil
}

// EncodePlatformChallengeResponse builds a CLIENT_PLATFORM_CHALLENGE_RESPONSE PDU
// (including preamble).
//
// Layout: Preamble(4) + EncryptedChallengeResponse(blob) + EncryptedHWID(blob) + MACData(16)
func EncodePlatformChallengeResponse(encryptedResponse, encryptedHWID, macData []byte) []byte {
	respBlob := EncodeBlob(Blob{Type: BBDataBlob, Data: encryptedResponse})
	hwidBlob := EncodeBlob(Blob{Type: BBDataBlob, Data: encryptedHWID})

	bodyLen := len(respBlob) + len(hwidBlob) + 16
	msgSize := 4 + bodyLen

	buf := make([]byte, 0, msgSize)
	buf = append(buf, EncodePreamble(PlatformChallengeResponse, PreambleVersion30, uint16(msgSize))...)
	buf = append(buf, respBlob...)
	buf = append(buf, hwidBlob...)
	buf = append(buf, macData...)

	return buf
}

// ErrorAlertData holds parsed LICENSING_ERROR_MESSAGE fields.
type ErrorAlertData struct {
	ErrorCode       uint32
	StateTransition uint32
	ErrorInfo       Blob
}

// DecodeErrorAlert parses a LICENSING_ERROR_MESSAGE after the preamble.
//
// Layout: dwErrorCode(u32) + dwStateTransition(u32) + bbErrorInfo(blob)
func DecodeErrorAlert(data []byte) (*ErrorAlertData, error) {
	if len(data) < 8 {
		return nil, fmt.Errorf("error alert too short: %d bytes", len(data))
	}

	ea := &ErrorAlertData{
		ErrorCode:       binary.LittleEndian.Uint32(data[0:4]),
		StateTransition: binary.LittleEndian.Uint32(data[4:8]),
	}

	if len(data) > 8 {
		var err error
		ea.ErrorInfo, _, err = DecodeBlob(data[8:])
		if err != nil {
			return nil, fmt.Errorf("error alert info blob: %w", err)
		}
	}

	return ea, nil
}
