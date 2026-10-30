package nla

import (
	"encoding/asn1"
	"errors"
)

// SPNEGO OIDs
var (
	oidSPNEGO = asn1.ObjectIdentifier{1, 3, 6, 1, 5, 5, 2}
	oidNTLMSSP = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 311, 2, 2, 10}
)

// SPNEGO NegState values
const (
	spnegoAcceptCompleted  = 0
	spnegoAcceptIncomplete = 1
	spnegoReject           = 2
)

// negTokenInit is the initial SPNEGO token containing mechanism type and token.
type negTokenInit struct {
	MechTypes []asn1.ObjectIdentifier `asn1:"explicit,tag:0"`
	MechToken []byte                  `asn1:"explicit,optional,tag:2"`
}

// negTokenResp is the SPNEGO response containing state and response token.
type negTokenResp struct {
	NegState      asn1.Enumerated `asn1:"explicit,optional,tag:0"`
	SupportedMech asn1.ObjectIdentifier `asn1:"explicit,optional,tag:1"`
	ResponseToken []byte          `asn1:"explicit,optional,tag:2"`
}

// encodeNegTokenInit wraps an NTLM Negotiate message in SPNEGO NegTokenInit
// with GSSAPI Application[0] wrapper.
// It returns both the complete SPNEGO token and the raw DER-encoded MechTypeList
// (SEQUENCE OF MechType), which is needed later for mechListMIC computation
// per RFC 4178 Section 5.
func encodeNegTokenInit(ntlmNegotiate []byte) (spnegoToken []byte, mechTypeList []byte) {
	token := negTokenInit{
		MechTypes: []asn1.ObjectIdentifier{oidNTLMSSP},
		MechToken: ntlmNegotiate,
	}

	// Save the DER-encoded MechTypeList (SEQUENCE of OIDs) for mechListMIC.
	// Per RFC 4178 §5: "The input message is the DER encoding of the value
	// of type MechTypeList" — NOT the [0]-tagged wrapper.
	mechTypeList, _ = asn1.Marshal(token.MechTypes)

	// Encode inner NegTokenInit with context tag [0] (CHOICE NegTokenInit)
	innerBytes, _ := asn1.Marshal(token)
	taggedInner := asn1WrapExplicit(0xa0, innerBytes)

	// Build GSSAPI Application[0] wrapper:
	// Application[0] IMPLICIT SEQUENCE { OID, NegTokenInit }
	oidBytes, _ := asn1.Marshal(oidSPNEGO)
	innerSeq := append(oidBytes, taggedInner...)

	spnegoToken = asn1WrapExplicit(0x60, innerSeq)
	return
}

// decodeNegTokenResp parses a SPNEGO NegTokenResp and returns the NTLM challenge token.
func decodeNegTokenResp(data []byte) (negState int, responseToken []byte, err error) {
	// The server sends NegTokenResp wrapped in context tag [1]
	// or directly as a SEQUENCE. Try to unwrap.
	inner := data

	// Check for context-specific tag [1] (NegTokenResp in CHOICE)
	if len(inner) > 2 && inner[0] == 0xa1 {
		var raw asn1.RawValue
		if _, err := asn1.Unmarshal(inner, &raw); err == nil {
			inner = raw.Bytes
		}
	}

	var resp negTokenResp
	if _, err := asn1.Unmarshal(inner, &resp); err != nil {
		return 0, nil, errors.New("spnego: failed to unmarshal NegTokenResp")
	}

	return int(resp.NegState), resp.ResponseToken, nil
}

// encodeNegTokenResp wraps an NTLM Authenticate message in SPNEGO NegTokenResp.
// Client only sends ResponseToken [2] and optional mechListMIC [3],
// no NegState (per RFC 4178).
// mechListMIC is the 16-byte NTLM MAC signature over the MechTypeList bytes
// and MUST be included when the NTLM Authenticate message contains a MIC.
func encodeNegTokenResp(ntlmAuthenticate []byte, mechListMIC []byte) []byte {
	// Manually construct: SEQUENCE { [2] OCTET STRING, [3] OCTET STRING }
	// This avoids encoding NegState (which Go's asn1 would include as zero).
	tokenOctet, _ := asn1.Marshal(asn1.RawValue{
		Class:      asn1.ClassUniversal,
		Tag:        asn1.TagOctetString,
		IsCompound: false,
		Bytes:      ntlmAuthenticate,
	})
	taggedToken := asn1WrapExplicit(0xa2, tokenOctet) // context [2]

	var seqContent []byte
	seqContent = append(seqContent, taggedToken...)

	if len(mechListMIC) > 0 {
		micOctet, _ := asn1.Marshal(asn1.RawValue{
			Class:      asn1.ClassUniversal,
			Tag:        asn1.TagOctetString,
			IsCompound: false,
			Bytes:      mechListMIC,
		})
		taggedMIC := asn1WrapExplicit(0xa3, micOctet) // context [3]
		seqContent = append(seqContent, taggedMIC...)
	}

	seq := asn1WrapExplicit(0x30, seqContent)          // SEQUENCE
	return asn1WrapExplicit(0xa1, seq)                  // context [1] (NegTokenResp CHOICE)
}

// asn1WrapExplicit wraps data in an ASN.1 tag with definite-length encoding.
func asn1WrapExplicit(tag byte, data []byte) []byte {
	lenBytes := asn1EncodeLength(len(data))
	result := make([]byte, 1+len(lenBytes)+len(data))
	result[0] = tag
	copy(result[1:], lenBytes)
	copy(result[1+len(lenBytes):], data)
	return result
}

// asn1EncodeLength produces DER length encoding.
func asn1EncodeLength(length int) []byte {
	if length < 0x80 {
		return []byte{byte(length)}
	}
	if length < 0x100 {
		return []byte{0x81, byte(length)}
	}
	if length < 0x10000 {
		return []byte{0x82, byte(length >> 8), byte(length)}
	}
	return []byte{0x83, byte(length >> 16), byte(length >> 8), byte(length)}
}
