package sec

import (
	"crypto/hmac"
	"crypto/md5"
	"encoding/binary"
	"unicode/utf16"
)

// Client Info flags (MS-RDPBCGR 2.2.1.11.1.1).
const (
	InfoMouse             uint32 = 0x00000001
	InfoDisableCtrlAltDel uint32 = 0x00000002
	InfoAutologon         uint32 = 0x00000008
	InfoUnicode           uint32 = 0x00000010
	InfoMaximizeShell     uint32 = 0x00000020
	InfoLogonNotify       uint32 = 0x00000040
	InfoCompression       uint32 = 0x00000080
	InfoEnableWinKey      uint32 = 0x00000100
	InfoCompressionTypeMask uint32 = 0x00001E00
	InfoRemoteConsoleAudio uint32 = 0x00002000 // audio plays on server (audio-mode:1)
	InfoNoAudioPlayback    uint32 = 0x00080000 // no audio playback at all (audio-mode:2)
	InfoAudioCapture      uint32 = 0x00200000 // client supports audio input redirection
)

// ClientInfo holds the fields for the Client Info PDU.
type ClientInfo struct {
	Domain     string
	Username   string
	Password   string
	PerfFlags       uint32 // TS_EXTENDED_INFO_PACKET performanceFlags
	AudioInput      bool   // set INFO_AUDIOCAPTURE flag
	RemoteAudio     bool   // set INFO_REMOTECONSOLEAUDIO flag (audio plays on server)
	NoAudioPlayback bool   // set INFO_NOAUDIOPLAYBACK flag (no audio at all)

	// Auto-reconnect cookie (MS-RDPBCGR 2.2.4.1).
	// When set, ARC_CS_PRIVATE_PACKET is appended to TS_EXTENDED_INFO_PACKET.
	AutoReconnectCookie *AutoReconnectCookie
}

// AutoReconnectCookie holds the server-provided ARC_SC_PRIVATE_PACKET
// data needed to build the client's ARC_CS_PRIVATE_PACKET on reconnect.
type AutoReconnectCookie struct {
	LogonID      uint32
	ServerRandom [16]byte // arcRandomBits from ARC_SC_PRIVATE_PACKET
	ClientRandom [32]byte // client random used during the original connection
}

// Performance flags for TS_EXTENDED_INFO_PACKET (MS-RDPBCGR 2.2.1.11.1.1.1).
const (
	PerfDisableWallpaper       uint32 = 0x00000001
	PerfDisableFullWindowDrag  uint32 = 0x00000002
	PerfDisableMenuAnimations  uint32 = 0x00000004
	PerfDisableTheming         uint32 = 0x00000008
	PerfDisablePlaybackSounds  uint32 = 0x00000010
	PerfDisableCursorShadow    uint32 = 0x00000020
	PerfDisableCursorSettings  uint32 = 0x00000040
	PerfEnableFontSmoothing    uint32 = 0x00000080
	PerfEnableDesktopComposit  uint32 = 0x00000100
)

// EncodeClientInfo serializes a Client Info PDU (TS_INFO_PACKET + TS_EXTENDED_INFO_PACKET).
func EncodeClientInfo(info *ClientInfo) []byte {
	domainUTF16 := encodeUTF16LE(info.Domain)
	userUTF16 := encodeUTF16LE(info.Username)
	passUTF16 := encodeUTF16LE(info.Password)

	// cbDomain, cbUserName, cbPassword are the byte counts of the string
	// data NOT including the mandatory null terminator.
	cbDomain := uint16(len(domainUTF16))
	cbUser := uint16(len(userUTF16))
	cbPass := uint16(len(passUTF16))

	flags := InfoUnicode | InfoMouse | InfoDisableCtrlAltDel | InfoMaximizeShell | InfoEnableWinKey
	if info.Username != "" {
		flags |= InfoLogonNotify
		if info.Password != "" {
			flags |= InfoAutologon
		}
	}
	if info.RemoteAudio {
		flags |= InfoRemoteConsoleAudio
	}
	if info.NoAudioPlayback {
		flags |= InfoNoAudioPlayback
	}
	if info.AudioInput {
		flags |= InfoAudioCapture
	}

	// TS_INFO_PACKET fixed header: codePage(4) + flags(4) + cbDomain(2) + cbUserName(2) +
	//               cbPassword(2) + cbAlternateShell(2) + cbWorkingDir(2) = 18
	headerLen := 18
	// String data: each string + 2 bytes null terminator
	stringsLen := int(cbDomain) + 2 + int(cbUser) + 2 + int(cbPass) + 2 + 2 + 2

	// TS_EXTENDED_INFO_PACKET:
	//   clientAddressFamily(2) + cbClientAddress(2) + clientAddress(variable)
	//   + cbClientDir(2) + clientDir(variable)
	//   + clientTimeZone(172) + clientSessionId(4) + performanceFlags(4)
	//   + cbAutoReconnectCookie(2) [+ ARC_CS_PRIVATE_PACKET(28)]
	clientAddr := encodeUTF16LENull("0.0.0.0")
	clientDir := encodeUTF16LENull("C:\\WINNT\\System32\\mstscax.dll")
	extInfoLen := 2 + 2 + len(clientAddr) + 2 + len(clientDir) + 172 + 4 + 4 + 2
	if info.AutoReconnectCookie != nil {
		extInfoLen += 28 // ARC_CS_PRIVATE_PACKET
	}

	total := headerLen + stringsLen + extInfoLen
	buf := make([]byte, total)
	off := 0

	// codePage
	binary.LittleEndian.PutUint32(buf[off:], 0)
	off += 4

	// flags
	binary.LittleEndian.PutUint32(buf[off:], flags)
	off += 4

	// cb fields
	binary.LittleEndian.PutUint16(buf[off:], cbDomain)
	off += 2
	binary.LittleEndian.PutUint16(buf[off:], cbUser)
	off += 2
	binary.LittleEndian.PutUint16(buf[off:], cbPass)
	off += 2
	binary.LittleEndian.PutUint16(buf[off:], 0) // cbAlternateShell
	off += 2
	binary.LittleEndian.PutUint16(buf[off:], 0) // cbWorkingDir
	off += 2

	// domain + null terminator
	off += copy(buf[off:], domainUTF16)
	off += 2 // null terminator (already zero)

	// username + null terminator
	off += copy(buf[off:], userUTF16)
	off += 2

	// password + null terminator
	off += copy(buf[off:], passUTF16)
	off += 2

	// alternateShell null terminator
	off += 2

	// workingDir null terminator
	off += 2

	// --- TS_EXTENDED_INFO_PACKET ---

	// clientAddressFamily: AF_INET
	binary.LittleEndian.PutUint16(buf[off:], 0x0002)
	off += 2

	// cbClientAddress (byte count including null terminator)
	binary.LittleEndian.PutUint16(buf[off:], uint16(len(clientAddr)))
	off += 2

	// clientAddress
	off += copy(buf[off:], clientAddr)

	// cbClientDir (byte count including null terminator)
	binary.LittleEndian.PutUint16(buf[off:], uint16(len(clientDir)))
	off += 2

	// clientDir
	off += copy(buf[off:], clientDir)

	// clientTimeZone: TS_TIME_ZONE_INFORMATION (172 bytes, all zeros = UTC)
	off += 172

	// clientSessionId
	binary.LittleEndian.PutUint32(buf[off:], 0)
	off += 4

	// performanceFlags
	binary.LittleEndian.PutUint32(buf[off:], info.PerfFlags)
	off += 4

	// cbAutoReconnectCookie + optional ARC_CS_PRIVATE_PACKET
	if arc := info.AutoReconnectCookie; arc != nil {
		binary.LittleEndian.PutUint16(buf[off:], 28) // cbAutoReconnectLen
		off += 2
		// ARC_CS_PRIVATE_PACKET (MS-RDPBCGR 2.2.4.3)
		binary.LittleEndian.PutUint32(buf[off:], 28)           // cbLen
		binary.LittleEndian.PutUint32(buf[off+4:], 1)          // version
		binary.LittleEndian.PutUint32(buf[off+8:], arc.LogonID) // logonId
		// securityVerifier = HMAC-MD5(arcRandomBits, clientRandom)
		mac := hmac.New(md5.New, arc.ServerRandom[:])
		mac.Write(arc.ClientRandom[:])
		copy(buf[off+12:], mac.Sum(nil))
	} else {
		binary.LittleEndian.PutUint16(buf[off:], 0)
	}

	return buf
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

// encodeUTF16LENull encodes a Go string as UTF-16LE bytes with a null terminator.
func encodeUTF16LENull(s string) []byte {
	u16 := utf16.Encode([]rune(s))
	buf := make([]byte, (len(u16)+1)*2)
	for i, c := range u16 {
		binary.LittleEndian.PutUint16(buf[i*2:], c)
	}
	// Last 2 bytes already zero (null terminator)
	return buf
}
