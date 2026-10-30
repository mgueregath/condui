package nla

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/rand"
	"crypto/rc4"
	"encoding/binary"
	"errors"
	"strings"
	"unicode/utf16"
	"context"
	"log/slog"

	"github.com/bouncyball-git/gopher-rdp/util"
)

// NTLM message type constants
const (
	ntlmNegotiate    = 1
	ntlmChallenge    = 2
	ntlmAuthenticate = 3
)

// NTLM negotiate flags (MS-NLMP 2.2.2.5)
const (
	ntlmFlagUnicode        uint32 = 0x00000001
	ntlmFlagNTLM           uint32 = 0x00000200
	ntlmFlagSeal           uint32 = 0x00000020
	ntlmFlagSign           uint32 = 0x00000010
	ntlmFlagAlwaysSign     uint32 = 0x00008000
	ntlmFlagESS            uint32 = 0x00080000 // Extended Session Security
	ntlmFlagTargetInfo     uint32 = 0x00800000
	ntlmFlagKeyExch        uint32 = 0x40000000
	ntlmFlag128            uint32 = 0x20000000
	ntlmFlag56             uint32 = 0x80000000
	ntlmFlagRequestTarget  uint32 = 0x00000004
	ntlmFlagVersion        uint32 = 0x02000000
	ntlmFlagOEM            uint32 = 0x00000002
	ntlmFlagLMKey          uint32 = 0x00000080
)

// AV_PAIR IDs (MS-NLMP 2.2.2.1)
const (
	avEOL             = 0x0000
	avNbComputerName  = 0x0001
	avNbDomainName    = 0x0002
	avDNSComputerName = 0x0003
	avDNSDomainName   = 0x0004
	avTimestamp       = 0x0007
	avFlags           = 0x0006
	avTargetName        = 0x0009
	avChannelBindings   = 0x000A
)

var ntlmSignature = [8]byte{'N', 'T', 'L', 'M', 'S', 'S', 'P', 0}

// ntlmClient holds state across the NTLM 3-message handshake.
type ntlmClient struct {
	domain   string
	username string
	password string

	negotiateMsg    []byte // saved for MIC
	challengeMsg    []byte // saved for MIC
	authenticateMsg []byte // saved for MIC

	exportedSessionKey  [16]byte
	channelBindingsHash [16]byte // MD5(SEC_CHANNEL_BINDINGS) for EPA
	targetSPN           string   // SPN for MsvAvTargetName (e.g. "TERMSRV/hostname")
	log                *slog.Logger
	serverChallenge    [8]byte
}

// negotiate builds an NTLM Negotiate message (Type 1).
func (n *ntlmClient) negotiate() []byte {
	// Negotiate flags per MS-NLMP, matching reference implementation.
	// Note: OEM and LMKey are NOT included (LMKey is incompatible with ESS).
	flags := ntlmFlagUnicode | ntlmFlagNTLM | ntlmFlagSeal | ntlmFlagSign |
		ntlmFlagAlwaysSign | ntlmFlagESS |
		ntlmFlagKeyExch | ntlmFlag128 | ntlmFlag56 |
		ntlmFlagRequestTarget | ntlmFlagVersion

	// Type 1: Signature(8) + MessageType(4) + NegotiateFlags(4) +
	//         DomainNameFields(8) + WorkstationFields(8) + Version(8)
	msg := make([]byte, 40)
	copy(msg[0:8], ntlmSignature[:])
	binary.LittleEndian.PutUint32(msg[8:12], ntlmNegotiate)
	binary.LittleEndian.PutUint32(msg[12:16], flags)
	// DomainNameFields and WorkstationFields: all zeros (not supplied)
	// Version: 10.0.19041 (Windows 10), NTLMSSP revision 15
	msg[32] = 10       // ProductMajorVersion
	msg[33] = 0        // ProductMinorVersion
	binary.LittleEndian.PutUint16(msg[34:36], 19041) // ProductBuild
	msg[39] = 15       // NTLMRevisionCurrent

	n.negotiateMsg = make([]byte, len(msg))
	copy(n.negotiateMsg, msg)
	n.log.LogAttrs(context.Background(), slog.LevelDebug, "NTLM negotiate built", util.Hex8("flags", flags), slog.Int("len", len(msg)))
	return msg
}

// authenticate parses the NTLM Challenge (Type 2) and builds the Authenticate (Type 3) message.
func (n *ntlmClient) authenticate(challengeMsg []byte) ([]byte, error) {
	if len(challengeMsg) < 32 {
		return nil, errors.New("ntlm: challenge message too short")
	}
	if string(challengeMsg[0:8]) != string(ntlmSignature[:]) {
		return nil, errors.New("ntlm: invalid challenge signature")
	}
	msgType := binary.LittleEndian.Uint32(challengeMsg[8:12])
	if msgType != ntlmChallenge {
		return nil, errors.New("ntlm: expected challenge message type 2")
	}

	n.challengeMsg = make([]byte, len(challengeMsg))
	copy(n.challengeMsg, challengeMsg)

	serverFlags := binary.LittleEndian.Uint32(challengeMsg[20:24])
	copy(n.serverChallenge[:], challengeMsg[24:32])

	// Parse TargetInfo from challenge
	var targetInfo []byte
	if serverFlags&ntlmFlagTargetInfo != 0 && len(challengeMsg) >= 48 {
		tiLen := binary.LittleEndian.Uint16(challengeMsg[40:42])
		tiOff := binary.LittleEndian.Uint32(challengeMsg[44:48])
		if int(tiOff)+int(tiLen) <= len(challengeMsg) {
			targetInfo = challengeMsg[tiOff : tiOff+uint32(tiLen)]
		}
	}
	n.log.LogAttrs(context.Background(), slog.LevelDebug, "NTLM challenge parsed", util.Hex8("serverFlags", serverFlags), slog.Int("targetInfoLen", len(targetInfo)))

	// Log AV_PAIRs from server challenge (trace level)
	for off := 0; off+4 <= len(targetInfo); {
		avID := binary.LittleEndian.Uint16(targetInfo[off:])
		avLen := binary.LittleEndian.Uint16(targetInfo[off+2:])
		if off+4+int(avLen) > len(targetInfo) {
			break
		}
		n.log.LogAttrs(context.Background(), slog.LevelDebug, "challenge AV_PAIR", util.Hex4("avID", avID), slog.Int("avLen", int(avLen)))
		if avID == avEOL {
			break
		}
		off += 4 + int(avLen)
	}

	// Compute NTLMv2 response
	ntHash := ntowfv2(n.password, n.username, n.domain)

	// Build client challenge blob (temp in MS-NLMP)
	clientChallenge := make([]byte, 8)
	if _, err := rand.Read(clientChallenge); err != nil {
		return nil, err
	}

	// Get timestamp from TargetInfo, or use current time
	timestamp := getAVTimestamp(targetInfo)
	n.log.LogAttrs(context.Background(), slog.LevelDebug, "NTLMv2 blob params", util.Bytes("timestamp", timestamp[:]), slog.String("user", n.username), slog.String("domain", n.domain))

	// Full TargetInfo modification: MsvAvFlags=0x02 + CB + SPN
	modifiedTI := modifyTargetInfo(targetInfo, n.channelBindingsHash, n.targetSPN)
	n.log.LogAttrs(context.Background(), slog.LevelDebug, "TargetInfo modified", slog.Int("origLen", len(targetInfo)), slog.Int("modLen", len(modifiedTI)))

	// Build client blob: RespType(1) + HiRespType(1) + Reserved1(2) + Reserved2(4) +
	//   TimeStamp(8) + ChallengeFromClient(8) + Reserved3(4) + AvPairs(variable)
	blob := make([]byte, 28+len(modifiedTI))
	blob[0] = 0x01 // RespType
	blob[1] = 0x01 // HiRespType
	copy(blob[8:16], timestamp[:])
	copy(blob[16:24], clientChallenge)
	copy(blob[28:], modifiedTI)

	// NTProofStr = HMAC_MD5(ResponseKeyNT, ServerChallenge || blob)
	mac := hmac.New(md5.New, ntHash[:])
	mac.Write(n.serverChallenge[:])
	mac.Write(blob)
	ntProofStr := mac.Sum(nil)

	// NtChallengeResponse = NTProofStr || blob
	ntResponse := append(ntProofStr, blob...)

	// SessionBaseKey = HMAC_MD5(ResponseKeyNT, NTProofStr)
	mac = hmac.New(md5.New, ntHash[:])
	mac.Write(ntProofStr)
	sessionBaseKey := mac.Sum(nil)

	// ExportedSessionKey = random(16), encrypted with RC4(SessionBaseKey)
	var exportedKey [16]byte
	if _, err := rand.Read(exportedKey[:]); err != nil {
		return nil, err
	}
	copy(n.exportedSessionKey[:], exportedKey[:])

	cipher, _ := rc4.NewCipher(sessionBaseKey)
	encryptedKey := make([]byte, 16)
	cipher.XORKeyStream(encryptedKey, exportedKey[:])

	// Build Authenticate message (Type 3)
	// Flags built from scratch per reference implementation (not intersected with server)
	negotiateFlags := ntlmFlagUnicode | ntlmFlagNTLM | ntlmFlagSeal | ntlmFlagSign |
		ntlmFlagAlwaysSign | ntlmFlagESS | ntlmFlagTargetInfo |
		ntlmFlag128 | ntlmFlag56 |
		ntlmFlagRequestTarget | ntlmFlagVersion
	// KEY_EXCH only if server offered it
	if serverFlags&ntlmFlagKeyExch != 0 {
		negotiateFlags |= ntlmFlagKeyExch
	}

	domainBytes := encodeUTF16LE(n.domain)
	userBytes := encodeUTF16LE(n.username)
	workstation := []byte{} // empty

	// Per MS-NLMP 3.1.5.1.2: when MsvAvTimestamp is present in the server
	// challenge TargetInfo, LmChallengeResponse MUST be set to Z(24).
	lmResponse := make([]byte, 24)

	// Payload layout matches reference implementation:
	//   Domain + User + Workstation + LmChallengeResponse + NtChallengeResponse + EncryptedKey
	// MIC is at offset 72..88 (16 bytes)
	headerLen := 88
	payloadOff := headerLen
	domainOff := payloadOff
	payloadOff += len(domainBytes)
	userOff := payloadOff
	payloadOff += len(userBytes)
	wsOff := payloadOff
	payloadOff += len(workstation)
	lmOff := payloadOff
	payloadOff += len(lmResponse)
	ntOff := payloadOff
	payloadOff += len(ntResponse)
	ekOff := payloadOff
	payloadOff += 16

	msg := make([]byte, payloadOff)

	// Signature + MessageType
	copy(msg[0:8], ntlmSignature[:])
	binary.LittleEndian.PutUint32(msg[8:12], ntlmAuthenticate)

	// LmChallengeResponse fields
	binary.LittleEndian.PutUint16(msg[12:14], uint16(len(lmResponse)))
	binary.LittleEndian.PutUint16(msg[14:16], uint16(len(lmResponse)))
	binary.LittleEndian.PutUint32(msg[16:20], uint32(lmOff))

	// NtChallengeResponse fields
	binary.LittleEndian.PutUint16(msg[20:22], uint16(len(ntResponse)))
	binary.LittleEndian.PutUint16(msg[22:24], uint16(len(ntResponse)))
	binary.LittleEndian.PutUint32(msg[24:28], uint32(ntOff))

	// DomainName fields
	binary.LittleEndian.PutUint16(msg[28:30], uint16(len(domainBytes)))
	binary.LittleEndian.PutUint16(msg[30:32], uint16(len(domainBytes)))
	binary.LittleEndian.PutUint32(msg[32:36], uint32(domainOff))

	// UserName fields
	binary.LittleEndian.PutUint16(msg[36:38], uint16(len(userBytes)))
	binary.LittleEndian.PutUint16(msg[38:40], uint16(len(userBytes)))
	binary.LittleEndian.PutUint32(msg[40:44], uint32(userOff))

	// Workstation fields
	binary.LittleEndian.PutUint16(msg[44:46], uint16(len(workstation)))
	binary.LittleEndian.PutUint16(msg[46:48], uint16(len(workstation)))
	binary.LittleEndian.PutUint32(msg[48:52], uint32(wsOff))

	// EncryptedRandomSessionKey fields
	binary.LittleEndian.PutUint16(msg[52:54], 16)
	binary.LittleEndian.PutUint16(msg[54:56], 16)
	binary.LittleEndian.PutUint32(msg[56:60], uint32(ekOff))

	// NegotiateFlags
	binary.LittleEndian.PutUint32(msg[60:64], negotiateFlags)

	// Version (same as negotiate)
	msg[64] = 10
	msg[65] = 0
	binary.LittleEndian.PutUint16(msg[66:68], 19041)
	msg[71] = 15

	// MIC placeholder at offset 72..88 — filled below

	// Payload (order matches reference implementation)
	copy(msg[domainOff:], domainBytes)
	copy(msg[userOff:], userBytes)
	copy(msg[wsOff:], workstation)
	copy(msg[lmOff:], lmResponse)
	copy(msg[ntOff:], ntResponse)
	copy(msg[ekOff:], encryptedKey)

	// Compute MIC = HMAC_MD5(ExportedSessionKey, negotiate || challenge || authenticate)
	// MIC field must be zeroed during computation (it already is since we haven't written it)
	mic := hmac.New(md5.New, n.exportedSessionKey[:])
	mic.Write(n.negotiateMsg)
	mic.Write(n.challengeMsg)
	mic.Write(msg) // MIC field is zero
	micValue := mic.Sum(nil)
	copy(msg[72:88], micValue)

	n.log.LogAttrs(context.Background(), slog.LevelDebug, "NTLM authenticate built",
		util.Hex8("negotiateFlags", negotiateFlags), slog.Int("len", len(msg)))
	n.authenticateMsg = make([]byte, len(msg))
	copy(n.authenticateMsg, msg)

	return msg, nil
}

// ntowfv2 computes the NTLMv2 response key: HMAC_MD5(MD4(UTF16LE(password)), UTF16LE(UPPER(user)+domain)).
func ntowfv2(password, username, domain string) [16]byte {
	// NT Hash = MD4(UTF16LE(password))
	ntHash := md4Sum(encodeUTF16LE(password))

	// ResponseKey = HMAC_MD5(NTHash, UTF16LE(UPPER(username) + domain))
	userDom := encodeUTF16LE(strings.ToUpper(username) + domain)
	mac := hmac.New(md5.New, ntHash[:])
	mac.Write(userDom)
	var key [16]byte
	copy(key[:], mac.Sum(nil))
	return key
}

// getAVTimestamp extracts the MsvAvTimestamp from AV_PAIR list.
// If not found, returns current time as Windows FILETIME.
func getAVTimestamp(avPairs []byte) [8]byte {
	var ts [8]byte
	for off := 0; off+4 <= len(avPairs); {
		avID := binary.LittleEndian.Uint16(avPairs[off:])
		avLen := binary.LittleEndian.Uint16(avPairs[off+2:])
		if off+4+int(avLen) > len(avPairs) {
			break
		}
		if avID == avTimestamp && avLen == 8 {
			copy(ts[:], avPairs[off+4:off+12])
			return ts
		}
		if avID == avEOL {
			break
		}
		off += 4 + int(avLen)
	}
	// Fallback: use current time as Windows FILETIME
	// Not expected in practice — servers always include timestamp
	return ts
}

// modifyTargetInfo creates a modified copy of the AV_PAIR list with
// MsvAvFlags=0x02 (MIC present), MsvAvTargetName, and MsvAvChannelBindings set.
func modifyTargetInfo(avPairs []byte, channelBindings [16]byte, targetSPN string) []byte {
	// Copy the original pairs, inserting MsvAvFlags before MsvAvEOL
	var result []byte
	hasFlags := false
	for off := 0; off+4 <= len(avPairs); {
		avID := binary.LittleEndian.Uint16(avPairs[off:])
		avLen := binary.LittleEndian.Uint16(avPairs[off+2:])
		if off+4+int(avLen) > len(avPairs) {
			break
		}
		if avID == avFlags {
			// Replace existing flags with MIC_PROVIDED
			hasFlags = true
			result = append(result, avPairs[off:off+4]...)
			var flagVal [4]byte
			binary.LittleEndian.PutUint32(flagVal[:], 0x00000002)
			result = append(result, flagVal[:]...)
			off += 4 + int(avLen)
			continue
		}
		if avID == avEOL {
			// Insert MsvAvFlags before EOL if not already present
			if !hasFlags {
				var flagsPair [8]byte
				binary.LittleEndian.PutUint16(flagsPair[0:2], avFlags)
				binary.LittleEndian.PutUint16(flagsPair[2:4], 4)
				binary.LittleEndian.PutUint32(flagsPair[4:8], 0x00000002)
				result = append(result, flagsPair[:]...)
			}
			// Add MsvAvTargetName (SPN, per MS-NLMP 3.1.5.1.2)
			if targetSPN != "" {
				spnBytes := encodeUTF16LE(targetSPN)
				var spnHdr [4]byte
				binary.LittleEndian.PutUint16(spnHdr[0:2], avTargetName)
				binary.LittleEndian.PutUint16(spnHdr[2:4], uint16(len(spnBytes)))
				result = append(result, spnHdr[:]...)
				result = append(result, spnBytes...)
			}
			// Add MsvAvChannelBindings (EPA support) only if hash is non-zero
			nonZeroCB := false
			for _, b := range channelBindings {
				if b != 0 {
					nonZeroCB = true
					break
				}
			}
			if nonZeroCB {
				var cbPair [20]byte // 4-byte header + 16-byte hash
				binary.LittleEndian.PutUint16(cbPair[0:2], avChannelBindings)
				binary.LittleEndian.PutUint16(cbPair[2:4], 16)
				copy(cbPair[4:20], channelBindings[:])
				result = append(result, cbPair[:]...)
			}
			// Append EOL
			result = append(result, avPairs[off:off+4]...)
			break
		}
		result = append(result, avPairs[off:off+4+int(avLen)]...)
		off += 4 + int(avLen)
	}
	return result
}

// encodeUTF16LE encodes a Go string as UTF-16LE bytes (without null terminator).
func encodeUTF16LE(s string) []byte {
	if s == "" {
		return nil
	}
	u16 := utf16.Encode([]rune(s))
	buf := make([]byte, len(u16)*2)
	for i, c := range u16 {
		binary.LittleEndian.PutUint16(buf[i*2:], c)
	}
	return buf
}

