package sec

import "encoding/binary"

// EncodeSecurityExchange builds a Security Exchange PDU.
//
// Format:
//
//	SecHeader(4 bytes: flags=SEC_EXCHANGE_PKT, flagsHi=0) +
//	length(u32 LE) + encryptedClientRandom
func EncodeSecurityExchange(encryptedClientRandom []byte) []byte {
	totalLen := 4 + 4 + len(encryptedClientRandom) // secHeader + length + data
	buf := make([]byte, totalLen)

	// Security header: flags = SEC_EXCHANGE_PKT
	binary.LittleEndian.PutUint16(buf[0:2], ExchangePkt)
	binary.LittleEndian.PutUint16(buf[2:4], 0) // flagsHi

	// Length of encrypted client random
	binary.LittleEndian.PutUint32(buf[4:8], uint32(len(encryptedClientRandom)))

	// Encrypted client random
	copy(buf[8:], encryptedClientRandom)

	return buf
}
