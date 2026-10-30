package mcs

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"

	"github.com/bouncyball-git/gopher-rdp/protocol/ber"
)

// compile-time check: ensure bytes.Buffer satisfies io.ByteWriter for BER perf
var _ io.ByteWriter = (*bytes.Buffer)(nil)

// MCS PDU tag constants (Application class, Constructed)
const (
	// BER Application tags for MCS Connect PDUs
	TagConnectInitial  = 101 // Application 101, Constructed
	TagConnectResponse = 102 // Application 102, Constructed

	// PER-encoded MCS domain PDU types (first byte)
	DomainMCSPDUErectDomainRequest = 0x04
	DomainMCSPDUAttachUserRequest  = 0x28
	DomainMCSPDUAttachUserConfirm  = 0x2C // top 6 bits: 001011, bottom 2 bits vary
	DomainMCSPDUChannelJoinRequest = 0x38
	DomainMCSPDUChannelJoinConfirm = 0x3C // top 6 bits: 001111, bottom 2 bits vary
	DomainMCSPDUDisconnectProviderUltimatum = 0x20 // top 6 bits: 001000, bottom 2 bits = reason[2:1]
	DomainMCSPDUSendDataRequest             = 0x64
	DomainMCSPDUSendDataIndication          = 0x68
)

// GCC Conference Create constants
var (
	// T.124 GCC Conference Create Request PER header
	// Object identifier: 0.0.20.124.0.1 (ITU-T T.124)
	gccConferenceCreateRequestHeader = []byte{
		0x00, 0x05, 0x00, 0x14, 0x7C, 0x00, 0x01,
		// ConnectData::connectPDU length (2 bytes PER, filled later)
		// ConferenceCreateRequest::conferenceName = 1 (numeric string "1")
		// userData key = h221NonStandard "Duca"
	}

	// "Duca" key for client-to-server GCC user data
	gccH221ClientKey = []byte{0x44, 0x75, 0x63, 0x61}
	// "McDn" key for server-to-client GCC user data
	gccH221ServerKey = []byte{0x4D, 0x63, 0x44, 0x6E}
)

// ConnectInitial represents an MCS Connect Initial PDU.
type ConnectInitial struct {
	CallingDomainSelector []byte
	CalledDomainSelector  []byte
	UpwardFlag            bool
	TargetParameters      ber.DomainParameters
	MinimumParameters     ber.DomainParameters
	MaximumParameters     ber.DomainParameters
	UserData              []byte // GCC Conference Create Request (encoded)
}

// Encode serializes the MCS Connect Initial as BER with Application 101 tag.
func (ci *ConnectInitial) Encode() []byte {
	// Encode the content first to get the length
	var content bytes.Buffer
	ber.WriteOctetString(&content, ci.CallingDomainSelector)
	ber.WriteOctetString(&content, ci.CalledDomainSelector)
	ber.WriteBoolean(&content, ci.UpwardFlag)
	ber.WriteDomainParameters(&content, ci.TargetParameters)
	ber.WriteDomainParameters(&content, ci.MinimumParameters)
	ber.WriteDomainParameters(&content, ci.MaximumParameters)
	ber.WriteOctetString(&content, ci.UserData)

	// Wrap with Application 101 Constructed tag
	var result bytes.Buffer
	ber.WriteTag(&result, ber.ClassApplication, ber.Constructed, TagConnectInitial)
	ber.WriteLength(&result, content.Len())
	result.Write(content.Bytes())

	return result.Bytes()
}

// ConnectResponse represents an MCS Connect Response PDU (parsed from server).
type ConnectResponse struct {
	Result           int
	CalledConnectID  int
	DomainParameters ber.DomainParameters
	UserData         []byte // GCC Conference Create Response data
}

// DecodeConnectResponse parses an MCS Connect Response from BER-encoded data.
func DecodeConnectResponse(log *slog.Logger, data []byte) (*ConnectResponse, error) {
	r := bytes.NewReader(data)

	// Read Application 102 tag
	class, constructed, tag, err := ber.ReadTag(r)
	if err != nil {
		return nil, fmt.Errorf("reading connect response tag: %w", err)
	}
	if class != ber.ClassApplication || !constructed || tag != TagConnectResponse {
		return nil, fmt.Errorf("expected Application 102 Constructed, got class=0x%02X constructed=%v tag=%d",
			class, constructed, tag)
	}

	// Read outer length
	if _, err := ber.ReadLength(r); err != nil {
		return nil, fmt.Errorf("reading connect response length: %w", err)
	}

	cr := &ConnectResponse{}

	// Result (ENUMERATED)
	cr.Result, err = ber.ReadEnumerated(r)
	if err != nil {
		return nil, fmt.Errorf("reading result: %w", err)
	}

	// Called Connect ID (INTEGER)
	cr.CalledConnectID, err = ber.ReadInteger(r)
	if err != nil {
		return nil, fmt.Errorf("reading called connect id: %w", err)
	}

	// Domain Parameters (SEQUENCE)
	cr.DomainParameters, err = ber.ReadDomainParameters(r)
	if err != nil {
		return nil, fmt.Errorf("reading domain parameters: %w", err)
	}

	// User Data (OCTET STRING)
	uClass, _, uTag, err := ber.ReadTag(r)
	if err != nil {
		return nil, fmt.Errorf("reading user data tag: %w", err)
	}
	if uClass != ber.ClassUniversal || uTag != ber.TagOctetString {
		return nil, fmt.Errorf("expected OCTET STRING for user data, got class=0x%02X tag=%d", uClass, uTag)
	}
	udLen, err := ber.ReadLength(r)
	if err != nil {
		return nil, fmt.Errorf("reading user data length: %w", err)
	}
	cr.UserData = make([]byte, udLen)
	if _, err := io.ReadFull(r, cr.UserData); err != nil {
		return nil, fmt.Errorf("reading user data: %w", err)
	}

	log.LogAttrs(context.Background(), slog.LevelDebug, "MCS Connect Response", slog.Int("result", cr.Result), slog.Int("calledConnectID", cr.CalledConnectID), slog.Int("userDataLen", len(cr.UserData)))
	return cr, nil
}

// EncodeGCCConferenceCreateRequest builds the GCC Conference Create Request
// with client data blocks embedded as PER-encoded user data.
func EncodeGCCConferenceCreateRequest(clientData []byte) []byte {
	var buf bytes.Buffer

	// T.124 GCC Conference Create Request PER-encoded header
	// Object identifier: 0.0.20.124.0.1
	buf.Write([]byte{
		0x00, 0x05, 0x00, 0x14, 0x7C, 0x00, 0x01,
	})

	// ConnectData::connectPDU length (PER length determinant)
	// This is the length of the rest of the data starting from conferenceName
	// ConferenceCreateRequest encoding:
	//   - conferenceName (numeric string "1")
	//   - lockedConference = false, listedConference = false
	//   - conductibleConference = false, terminationMethod = automatic
	//   - userData present flag
	//   - UserData: key = h221NonStandard "Duca", value = client data
	innerLen := 14 + len(clientData) // 14 bytes of GCC inner headers + client data
	writePerLength(&buf, innerLen)

	// ConferenceCreateRequest
	buf.Write([]byte{
		0x00, 0x08, // ConferenceCreateRequest choice + extension bits
		0x00, 0x10, // conferenceName numeric string "1"
	})

	// userData: h221NonStandard key "Duca" + length + data
	buf.Write([]byte{
		0x00, 0x01, 0xC0, 0x00, // userData set count=1, key choice h221NonStandard
	})
	buf.Write(gccH221ClientKey) // "Duca"

	// User data value length (PER length determinant)
	writePerLength(&buf, len(clientData))

	buf.Write(clientData)

	return buf.Bytes()
}

// DecodeGCCConferenceCreateResponse extracts server user data from a
// GCC Conference Create Response.
func DecodeGCCConferenceCreateResponse(data []byte) ([]byte, error) {
	// The GCC Conference Create Response has a PER-encoded header.
	// We need to find the "McDn" key and extract the user data after it.

	// Minimum size check
	if len(data) < 9 {
		return nil, fmt.Errorf("GCC response too short: %d bytes", len(data))
	}

	// Skip the PER header. Structure:
	// - ConnectGCCPDU choice (1 byte = 0x00 for ConferenceCreateResponse)
	// ... actual layout varies. Find "McDn" marker.
	idx := bytes.Index(data, gccH221ServerKey)
	if idx < 0 {
		return nil, fmt.Errorf("McDn key not found in GCC response")
	}

	// After "McDn" (4 bytes), there's a PER length determinant for the user data value
	offset := idx + 4
	if offset >= len(data) {
		return nil, fmt.Errorf("GCC response truncated after McDn key")
	}

	// Read PER length determinant
	udLen, n := readPerLength(data[offset:])
	offset += n

	if offset+udLen > len(data) {
		// Some servers don't encode length correctly; use remaining data
		udLen = len(data) - offset
	}

	return data[offset : offset+udLen], nil
}

// readPerLength reads a PER length determinant from data.
// Returns the length value and number of bytes consumed.
func readPerLength(data []byte) (int, int) {
	if len(data) == 0 {
		return 0, 0
	}
	if data[0]&0x80 == 0 {
		return int(data[0]), 1
	}
	if len(data) < 2 {
		return 0, 1
	}
	return int(data[0]&0x7F)<<8 | int(data[1]), 2
}

// writePerLength writes a PER length determinant.
func writePerLength(w *bytes.Buffer, length int) {
	if length < 0x80 {
		w.WriteByte(byte(length))
	} else {
		w.WriteByte(byte(0x80 | (length >> 8)))
		w.WriteByte(byte(length & 0xFF))
	}
}

// EncodeErectDomainRequest builds an MCS Erect Domain Request PDU (PER-encoded).
// subHeight=0, subInterval=0
func EncodeErectDomainRequest() []byte {
	return []byte{
		DomainMCSPDUErectDomainRequest, // PDU type
		0x01, 0x00, // subHeight = 0 (PER: length 1, value 0)
		0x01, 0x00, // subInterval = 0 (PER: length 1, value 0)
	}
}

// EncodeAttachUserRequest builds an MCS Attach User Request PDU (PER-encoded).
func EncodeAttachUserRequest() []byte {
	return []byte{DomainMCSPDUAttachUserRequest}
}

// DecodeAttachUserConfirm parses an MCS Attach User Confirm PDU.
// Returns the assigned user channel ID.
func DecodeAttachUserConfirm(log *slog.Logger, data []byte) (uint16, error) {
	if len(data) < 4 {
		return 0, fmt.Errorf("attach user confirm too short: %d bytes", len(data))
	}

	// First byte: top 6 bits = 001011 (0x2C >> 2 = 0x0B)
	// Bottom 2 bits: bit 1 = result present, bit 0 = user ID present
	if data[0]>>2 != DomainMCSPDUAttachUserConfirm>>2 {
		return 0, fmt.Errorf("expected Attach User Confirm (0x2C), got 0x%02X", data[0])
	}

	// Result: enumerated (1 byte), 0 = rt-successful
	result := data[1]
	if result != 0 {
		return 0, fmt.Errorf("attach user confirm failed: result=%d", result)
	}

	// User channel ID: 16-bit big-endian, offset by 1001
	userID := binary.BigEndian.Uint16(data[2:4])

	log.LogAttrs(context.Background(), slog.LevelDebug, "MCS Attach User Confirm", slog.Int("userChannelID", int(userID+1001)))
	return userID + 1001, nil
}

// EncodeChannelJoinRequest builds an MCS Channel Join Request PDU (PER-encoded).
func EncodeChannelJoinRequest(userID, channelID uint16) []byte {
	buf := make([]byte, 5)
	buf[0] = DomainMCSPDUChannelJoinRequest
	binary.BigEndian.PutUint16(buf[1:3], userID-1001)
	binary.BigEndian.PutUint16(buf[3:5], channelID)
	return buf
}

// DecodeChannelJoinConfirm parses an MCS Channel Join Confirm PDU.
// Returns the confirmed channel ID.
func DecodeChannelJoinConfirm(log *slog.Logger, data []byte) (uint16, error) {
	if len(data) < 8 {
		return 0, fmt.Errorf("channel join confirm too short: %d bytes", len(data))
	}

	// First byte: top 6 bits identify the PDU type
	if data[0]>>2 != DomainMCSPDUChannelJoinConfirm>>2 {
		return 0, fmt.Errorf("expected Channel Join Confirm (0x3C), got 0x%02X", data[0])
	}

	// Result (1 byte): 0 = success
	result := data[1]
	if result != 0 {
		return 0, fmt.Errorf("channel join confirm failed: result=%d", result)
	}

	// Initiator (2 bytes) + requested (2 bytes) + channelID (2 bytes)
	channelID := binary.BigEndian.Uint16(data[6:8])

	log.LogAttrs(context.Background(), slog.LevelDebug, "MCS Channel Join Confirm", slog.Int("channelID", int(channelID)))
	return channelID, nil
}

// EncodeSendDataRequest builds an MCS Send Data Request PDU header (PER-encoded).
func EncodeSendDataRequest(userID, channelID uint16, data []byte) []byte {
	// Header: type(1) + initiator(2) + channel(2) + priority(1) + length(1-3) = 7-9 bytes
	hdrSize := 6 // fixed part
	dataLen := len(data)
	if dataLen < 0x80 {
		hdrSize += 1
	} else if dataLen < 0x4000 {
		hdrSize += 2
	} else {
		hdrSize += 3
	}

	buf := make([]byte, hdrSize+dataLen)
	buf[0] = DomainMCSPDUSendDataRequest
	binary.BigEndian.PutUint16(buf[1:3], userID-1001)
	binary.BigEndian.PutUint16(buf[3:5], channelID)
	buf[5] = 0x70 // dataPriority=high, segmentation=begin|end

	off := 6
	if dataLen < 0x80 {
		buf[off] = byte(dataLen)
		off++
	} else if dataLen < 0x4000 {
		buf[off] = byte(0x80 | (dataLen >> 8))
		buf[off+1] = byte(dataLen & 0xFF)
		off += 2
	} else {
		buf[off] = 0x80
		binary.BigEndian.PutUint16(buf[off+1:], uint16(dataLen))
		off += 3
	}

	copy(buf[off:], data)
	return buf
}

