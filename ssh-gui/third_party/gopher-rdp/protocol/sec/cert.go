package sec

import (
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/binary"
	"fmt"
	"math/big"
)

// RSAPublicKey holds the server's RSA public key from a proprietary certificate.
type RSAPublicKey struct {
	BitLen  uint32
	PubExp  uint32
	Modulus []byte // little-endian, keylen bytes (includes 8-byte zero padding)
}

// ServerCertificate holds the extracted RSA public key from either a
// proprietary (version 1) or X.509 (version 2) server certificate.
type ServerCertificate struct {
	PublicKey RSAPublicKey
}

// ServerSecurityBlob holds the parsed server random and certificate from
// ServerSecurityData.RawData.
type ServerSecurityBlob struct {
	ServerRandom []byte // 32 bytes
	Certificate  ServerCertificate
}

// Certificate version and proprietary certificate constants.
const (
	certVersionMask    = 0x7FFFFFFF
	certVersionProp    = 1
	certVersionX509    = 2
	rsaKeyMagic        = 0x31415352 // "RSA1" little-endian
	sigAlgRSA          = 0x00000001
	keyAlgRSA          = 0x00000001
	publicKeyBlobType  = 0x0006
	signatureBlobType  = 0x0008
)

// DecodeServerSecurityBlob parses the RawData from ServerSecurityData into
// a ServerSecurityBlob containing the server random and proprietary certificate.
//
// RawData layout:
//   ServerRandomLen(u32) + ServerCertLen(u32) + ServerRandom(32) + ServerCertificate(variable)
func DecodeServerSecurityBlob(data []byte) (*ServerSecurityBlob, error) {
	if len(data) < 8 {
		return nil, fmt.Errorf("server security blob too short: %d bytes", len(data))
	}

	serverRandomLen := uint64(binary.LittleEndian.Uint32(data[0:4]))
	serverCertLen := uint64(binary.LittleEndian.Uint32(data[4:8]))

	if uint64(len(data)) < 8+serverRandomLen+serverCertLen {
		return nil, fmt.Errorf("server security blob truncated: need %d, have %d",
			8+serverRandomLen+serverCertLen, len(data))
	}

	blob := &ServerSecurityBlob{
		ServerRandom: make([]byte, serverRandomLen),
	}
	copy(blob.ServerRandom, data[8:8+serverRandomLen])

	certData := data[8+serverRandomLen : 8+serverRandomLen+serverCertLen]
	cert, err := DecodeServerCertificate(certData)
	if err != nil {
		return nil, err
	}
	blob.Certificate = *cert

	return blob, nil
}

// DecodeServerCertificate dispatches to proprietary (v1) or X.509 (v2) decoder
// based on dwVersion.
func DecodeServerCertificate(data []byte) (*ServerCertificate, error) {
	if len(data) < 4 {
		return nil, fmt.Errorf("server certificate too short: %d bytes", len(data))
	}

	dwVersion := binary.LittleEndian.Uint32(data[0:4])
	switch dwVersion & certVersionMask {
	case certVersionProp:
		return decodeProprietaryCert(data)
	case certVersionX509:
		return decodeX509CertChain(data)
	default:
		return nil, fmt.Errorf("unsupported certificate version %d", dwVersion&certVersionMask)
	}
}

// decodeProprietaryCert parses a proprietary certificate (version 1).
//
// Layout:
//
//	dwVersion(u32) + dwSigAlgId(u32) + dwKeyAlgId(u32) +
//	wPublicKeyBlobType(u16) + wPublicKeyBlobLen(u16) + PublicKeyBlob +
//	wSignatureBlobType(u16) + wSignatureBlobLen(u16) + SignatureBlob
func decodeProprietaryCert(data []byte) (*ServerCertificate, error) {
	if len(data) < 12 {
		return nil, fmt.Errorf("proprietary cert too short: %d bytes", len(data))
	}

	// Skip dwVersion(4) + dwSigAlgId(4) + dwKeyAlgId(4)
	off := 12

	// Public key blob
	if off+4 > len(data) {
		return nil, fmt.Errorf("proprietary cert truncated at public key blob header")
	}
	// wPublicKeyBlobType (u16) + wPublicKeyBlobLen (u16)
	pkBlobLen := int(binary.LittleEndian.Uint16(data[off+2 : off+4]))
	off += 4

	if off+pkBlobLen > len(data) {
		return nil, fmt.Errorf("proprietary cert truncated at public key blob data")
	}

	pubKey, err := decodeRSAPublicKeyBlob(data[off : off+pkBlobLen])
	if err != nil {
		return nil, err
	}

	// We skip signature validation (common practice for RDP clients).
	return &ServerCertificate{PublicKey: *pubKey}, nil
}

// decodeX509CertChain parses an X.509 certificate chain (version 2).
// The RSA public key is extracted from the last certificate in the chain
// (the terminal server certificate).
//
// Layout:
//
//	dwVersion(u32) + NumCertBlobs(u32) +
//	CertBlobArray[ cbCert(u32) + abCert(cbCert bytes) ... ]
func decodeX509CertChain(data []byte) (*ServerCertificate, error) {
	if len(data) < 8 {
		return nil, fmt.Errorf("X.509 cert chain too short: %d bytes", len(data))
	}

	numCerts := binary.LittleEndian.Uint32(data[4:8])
	if numCerts == 0 {
		return nil, fmt.Errorf("X.509 cert chain has no certificates")
	}

	// Walk through cert blobs, only parse the last one.
	off := 8
	var lastCertDER []byte
	for i := uint32(0); i < numCerts; i++ {
		if off+4 > len(data) {
			return nil, fmt.Errorf("X.509 cert chain truncated at blob %d header", i)
		}
		cbCert := int(binary.LittleEndian.Uint32(data[off : off+4]))
		off += 4

		if off+cbCert > len(data) {
			return nil, fmt.Errorf("X.509 cert chain truncated at blob %d data (need %d, have %d)",
				i, cbCert, len(data)-off)
		}

		if i == numCerts-1 {
			lastCertDER = data[off : off+cbCert]
		}
		off += cbCert
	}

	// Parse the terminal server certificate. Try standard parsing first;
	// fall back to raw ASN.1 public key extraction if the certificate has
	// non-conforming extensions (e.g. authority key identifier marked critical).
	rsaPub, err := parseX509RSAPublicKey(lastCertDER)
	if err != nil {
		return nil, err
	}

	// Convert from crypto/rsa (big-endian big.Int) to RDP wire format
	// (little-endian modulus with 8-byte zero padding).
	modBytes := rsaPub.N.Bytes() // big-endian
	modLen := len(modBytes)

	// Reverse to little-endian + append 8 zero bytes.
	modulus := make([]byte, modLen+8)
	for i := 0; i < modLen; i++ {
		modulus[i] = modBytes[modLen-1-i]
	}

	return &ServerCertificate{
		PublicKey: RSAPublicKey{
			BitLen:  uint32(rsaPub.N.BitLen()),
			PubExp:  uint32(rsaPub.E),
			Modulus: modulus,
		},
	}, nil
}

// decodeRSAPublicKeyBlob parses the RSA public key blob.
//
// Layout:
//   magic(u32) + keylen(u32) + bitlen(u32) + datalen(u32) + pubExp(u32) + modulus(keylen bytes)
func decodeRSAPublicKeyBlob(data []byte) (*RSAPublicKey, error) {
	if len(data) < 20 {
		return nil, fmt.Errorf("RSA public key blob too short: %d bytes", len(data))
	}

	magic := binary.LittleEndian.Uint32(data[0:4])
	if magic != rsaKeyMagic {
		return nil, fmt.Errorf("bad RSA public key magic: 0x%08X (expected 0x%08X)", magic, rsaKeyMagic)
	}

	keyLen := binary.LittleEndian.Uint32(data[4:8])
	bitLen := binary.LittleEndian.Uint32(data[8:12])
	// datalen at data[12:16] — not needed
	pubExp := binary.LittleEndian.Uint32(data[16:20])

	if uint32(len(data)) < 20+keyLen {
		return nil, fmt.Errorf("RSA public key blob truncated: need %d modulus bytes, have %d",
			keyLen, len(data)-20)
	}

	modulus := make([]byte, keyLen)
	copy(modulus, data[20:20+keyLen])

	return &RSAPublicKey{
		BitLen:  bitLen,
		PubExp:  pubExp,
		Modulus: modulus,
	}, nil
}

// parseX509RSAPublicKey extracts the RSA public key from a DER-encoded X.509
// certificate. Tries the standard library first; on failure, falls back to raw
// ASN.1 SubjectPublicKeyInfo extraction (handles certs with non-conforming
// extensions that Go's strict parser rejects).
func parseX509RSAPublicKey(der []byte) (*rsa.PublicKey, error) {
	cert, parseErr := x509.ParseCertificate(der)
	if parseErr == nil {
		rsaPub, ok := cert.PublicKey.(*rsa.PublicKey)
		if !ok {
			return nil, fmt.Errorf("X.509 cert has non-RSA public key (%T)", cert.PublicKey)
		}
		return rsaPub, nil
	}

	// Fallback: extract SubjectPublicKeyInfo directly from raw ASN.1.
	// Certificate ::= SEQUENCE { tbsCertificate TBSCertificate, ... }
	// TBSCertificate ::= SEQUENCE { version [0], serialNumber, signature,
	//                                issuer, validity, subject,
	//                                subjectPublicKeyInfo, ... }
	return extractRSAPublicKeyASN1(der, parseErr)
}

// extractRSAPublicKeyASN1 extracts an RSA public key from a DER certificate
// by walking the raw ASN.1 structure. origErr is the original x509.ParseCertificate
// error, included in returned errors for context.
func extractRSAPublicKeyASN1(der []byte, origErr error) (*rsa.PublicKey, error) {
	var outer asn1.RawValue
	rest, err := asn1.Unmarshal(der, &outer)
	if err != nil {
		return nil, fmt.Errorf("X.509 fallback: outer SEQUENCE: %v (original: %w)", err, origErr)
	}
	_ = rest

	// TBSCertificate is the first element of the Certificate SEQUENCE.
	var tbsRaw asn1.RawValue
	rest, err = asn1.Unmarshal(outer.Bytes, &tbsRaw)
	if err != nil {
		return nil, fmt.Errorf("X.509 fallback: TBSCertificate: %v (original: %w)", err, origErr)
	}

	// Walk 6 fields to reach subjectPublicKeyInfo (index 6).
	rest = tbsRaw.Bytes
	for i := 0; i < 6; i++ {
		var field asn1.RawValue
		rest, err = asn1.Unmarshal(rest, &field)
		if err != nil {
			return nil, fmt.Errorf("X.509 fallback: field %d: %v (original: %w)", i, err, origErr)
		}
	}

	// Parse SubjectPublicKeyInfo at current position.
	var pubKeyInfo struct {
		Algorithm pkix.AlgorithmIdentifier
		PublicKey asn1.BitString
	}
	if _, err = asn1.Unmarshal(rest, &pubKeyInfo); err != nil {
		return nil, fmt.Errorf("X.509 fallback: SubjectPublicKeyInfo: %v (original: %w)", err, origErr)
	}

	// Parse the RSA public key from the bit string payload.
	var rsaKey struct {
		N *big.Int
		E int
	}
	if _, err = asn1.Unmarshal(pubKeyInfo.PublicKey.Bytes, &rsaKey); err != nil {
		return nil, fmt.Errorf("X.509 fallback: RSA key: %v (original: %w)", err, origErr)
	}

	return &rsa.PublicKey{N: rsaKey.N, E: rsaKey.E}, nil
}
