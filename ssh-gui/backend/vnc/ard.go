package vnc

import (
	"bytes"
	"crypto/aes"
	"crypto/md5" // Required by Apple's legacy ARD authentication protocol.
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	"math/big"
	"net"
)

// authenticateARD implements security type 30, following noVNC's ARD handshake.
// Credentials stay in Go and are encrypted before being sent to the server.
func authenticateARD(conn net.Conn, username, password string) error {
	if username == "" {
		return fmt.Errorf("macOS username is required for Apple Remote Desktop authentication")
	}
	if len([]byte(username)) > 63 || len([]byte(password)) > 63 {
		return fmt.Errorf("Apple Remote Desktop credentials must be at most 63 UTF-8 bytes each")
	}
	header := make([]byte, 4)
	if _, err := io.ReadFull(conn, header); err != nil {
		return fmt.Errorf("invalid Apple Remote Desktop handshake")
	}
	length := int(binary.BigEndian.Uint16(header[2:]))
	if length < 64 || length > 512 {
		return fmt.Errorf("unsupported Apple Remote Desktop key size")
	}
	parameters := make([]byte, 2*length)
	if _, err := io.ReadFull(conn, parameters); err != nil {
		return fmt.Errorf("incomplete Apple Remote Desktop key exchange")
	}
	prime := new(big.Int).SetBytes(parameters[:length])
	serverPublic := new(big.Int).SetBytes(parameters[length:])
	generator := new(big.Int).SetUint64(uint64(binary.BigEndian.Uint16(header[:2])))
	one := big.NewInt(1)
	upper := new(big.Int).Sub(prime, one)
	if prime.BitLen() < 512 || prime.Bit(0) != 1 || generator.Cmp(one) <= 0 || generator.Cmp(upper) >= 0 || serverPublic.Cmp(one) <= 0 || serverPublic.Cmp(upper) >= 0 {
		return fmt.Errorf("invalid Apple Remote Desktop key parameters")
	}
	private, err := rand.Int(rand.Reader, new(big.Int).Sub(prime, big.NewInt(3)))
	if err != nil {
		return fmt.Errorf("Apple Remote Desktop key generation failed")
	}
	private.Add(private, big.NewInt(2))
	public := new(big.Int).Exp(generator, private, prime).FillBytes(make([]byte, length))
	shared := new(big.Int).Exp(serverPublic, private, prime).FillBytes(make([]byte, length))
	private.SetInt64(0)
	key := md5.Sum(shared)
	clear(shared)
	cipher, err := aes.NewCipher(key[:])
	clear(key[:])
	if err != nil {
		return fmt.Errorf("Apple Remote Desktop encryption unavailable")
	}
	credentials := make([]byte, 128)
	if _, err := rand.Read(credentials); err != nil {
		return fmt.Errorf("Apple Remote Desktop credential encryption failed")
	}
	copy(credentials[:64], []byte(username))
	credentials[len([]byte(username))] = 0
	copy(credentials[64:], []byte(password))
	credentials[64+len([]byte(password))] = 0
	// ARD specifies AES-128 ECB without padding for its fixed 128-byte payload.
	for offset := 0; offset < len(credentials); offset += aes.BlockSize {
		cipher.Encrypt(credentials[offset:offset+aes.BlockSize], credentials[offset:offset+aes.BlockSize])
	}
	payload := append(credentials, public...)
	_, err = io.Copy(conn, bytes.NewReader(payload))
	clear(payload)
	if err != nil {
		return fmt.Errorf("Apple Remote Desktop authentication interrupted")
	}
	return nil
}
