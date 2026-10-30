// Package ber implements ASN.1 BER (Basic Encoding Rules) encoding and decoding
// for the subset used by the MCS protocol in RDP.
//
// BER is a tag-length-value encoding:
//
//	+-------+--------+---------+
//	|  Tag  | Length |  Value  |
//	+-------+--------+---------+
//
// Tags identify the type (INTEGER, OCTET STRING, SEQUENCE, APPLICATION, etc.).
// Length can be short form (1 byte, 0-127) or long form (multi-byte, 128+).
package ber

import (
	"encoding/binary"
	"fmt"
	"io"
)

// Tag classes (bits 7-6 of the tag byte)
const (
	ClassUniversal   = 0x00
	ClassApplication = 0x40
	ClassContext      = 0x80
	ClassPrivate     = 0xC0
)

// Constructed flag (bit 5 of the tag byte)
const (
	Primitive   = 0x00
	Constructed = 0x20
)

// Universal tag numbers
const (
	TagBoolean     = 0x01
	TagInteger     = 0x02
	TagBitString   = 0x03
	TagOctetString = 0x04
	TagNull        = 0x05
	TagOID         = 0x06
	TagEnumerated  = 0x0A
	TagSequence    = 0x10
	TagSet         = 0x11
)

// writeByte writes a single byte to w without allocating a slice.
func writeByte(w io.Writer, b byte) error {
	if bw, ok := w.(io.ByteWriter); ok {
		return bw.WriteByte(b)
	}
	var buf [1]byte
	buf[0] = b
	_, err := w.Write(buf[:])
	return err
}

// WriteTag writes a BER tag to w.
// For tag numbers 0-30, a single byte is written.
// For tag numbers >= 31, the long form (multi-byte) is used.
func WriteTag(w io.Writer, class, constructed byte, tag int) error {
	if tag < 31 {
		return writeByte(w, class|constructed|byte(tag))
	}
	// Long form: first byte has tag bits set to 11111
	if err := writeByte(w, class|constructed|0x1F); err != nil {
		return err
	}
	return writeTagNumber(w, tag)
}

// writeTagNumber encodes tag numbers >= 31 using base-128 with continuation bits.
func writeTagNumber(w io.Writer, tag int) error {
	if tag < 0x80 {
		return writeByte(w, byte(tag))
	}
	// Collect base-128 digits into a stack array (max 5 bytes for 32-bit tags)
	var digits [5]byte
	n := 0
	for tag > 0 {
		digits[n] = byte(tag & 0x7F)
		tag >>= 7
		n++
	}
	// Write in reverse order, setting high bit on all but last
	for i := n - 1; i >= 0; i-- {
		b := digits[i]
		if i > 0 {
			b |= 0x80 // continuation bit
		}
		if err := writeByte(w, b); err != nil {
			return err
		}
	}
	return nil
}

// WriteLength writes a BER length to w.
// Short form (1 byte) for lengths 0-127.
// Long form (multi-byte) for lengths >= 128.
func WriteLength(w io.Writer, length int) error {
	if length < 0x80 {
		return writeByte(w, byte(length))
	}
	// Long form: encode into stack buffer
	var buf [5]byte // 1 length-of-length + max 4 length bytes
	var tmp [4]byte
	binary.BigEndian.PutUint32(tmp[:], uint32(length))
	// Find first non-zero byte
	start := 0
	for start < 3 && tmp[start] == 0 {
		start++
	}
	numBytes := 4 - start
	buf[0] = 0x80 | byte(numBytes)
	copy(buf[1:], tmp[start:])
	_, err := w.Write(buf[:1+numBytes])
	return err
}

// WriteInteger writes a BER INTEGER (tag + length + value).
func WriteInteger(w io.Writer, value int) error {
	if err := WriteTag(w, ClassUniversal, Primitive, TagInteger); err != nil {
		return err
	}
	data := encodeIntegerValue(value)
	if err := WriteLength(w, len(data)); err != nil {
		return err
	}
	_, err := w.Write(data)
	return err
}

// encodeIntegerValue encodes an integer as a minimal big-endian signed byte sequence.
// Returns a slice of a stack-allocated array.
func encodeIntegerValue(value int) []byte {
	if value == 0 {
		return []byte{0}
	}

	negative := value < 0
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], uint64(value))

	// Find first significant byte
	start := 0
	if !negative {
		for start < 7 && buf[start] == 0 {
			start++
		}
		// If high bit is set, prepend a zero byte for positive numbers
		if buf[start]&0x80 != 0 {
			if start > 0 {
				start--
			} else {
				// All 8 bytes significant with high bit set — prepend zero.
				var out [9]byte
				copy(out[1:], buf[:])
				return out[:]
			}
		}
	} else {
		for start < 7 && buf[start] == 0xFF {
			start++
		}
		// If high bit is clear, we need the 0xFF prefix for negative numbers
		if buf[start]&0x80 == 0 {
			if start > 0 {
				start--
			} else {
				// All 8 bytes significant with high bit clear — prepend 0xFF.
				var out [9]byte
				out[0] = 0xFF
				copy(out[1:], buf[:])
				return out[:]
			}
		}
	}
	return buf[start:]
}

// WriteOctetString writes a BER OCTET STRING (tag + length + raw bytes).
func WriteOctetString(w io.Writer, data []byte) error {
	if err := WriteTag(w, ClassUniversal, Primitive, TagOctetString); err != nil {
		return err
	}
	if err := WriteLength(w, len(data)); err != nil {
		return err
	}
	_, err := w.Write(data)
	return err
}

// WriteBoolean writes a BER BOOLEAN.
func WriteBoolean(w io.Writer, value bool) error {
	if err := WriteTag(w, ClassUniversal, Primitive, TagBoolean); err != nil {
		return err
	}
	if err := WriteLength(w, 1); err != nil {
		return err
	}
	if value {
		return writeByte(w, 0xFF)
	}
	return writeByte(w, 0x00)
}

// WriteEnumerated writes a BER ENUMERATED (same encoding as INTEGER, different tag).
func WriteEnumerated(w io.Writer, value int) error {
	if err := WriteTag(w, ClassUniversal, Primitive, TagEnumerated); err != nil {
		return err
	}
	data := encodeIntegerValue(value)
	if err := WriteLength(w, len(data)); err != nil {
		return err
	}
	_, err := w.Write(data)
	return err
}

// DomainParameters represents the MCS DomainParameters SEQUENCE.
type DomainParameters struct {
	MaxChannelIDs   int
	MaxUserIDs      int
	MaxTokenIDs     int
	NumPriorities   int
	MinThroughput   int
	MaxHeight       int
	MaxMCSPDUSize   int
	ProtocolVersion int
}

// WriteDomainParameters writes an MCS DomainParameters as a BER SEQUENCE.
func WriteDomainParameters(w io.Writer, p DomainParameters) error {
	// Encode all 8 integers into a stack buffer.
	// Max per integer TLV: 1 tag + 1 length + 8 value = 10 bytes, x8 = 80 bytes max
	var buf [80]byte
	n := 0
	for _, v := range [8]int{
		p.MaxChannelIDs, p.MaxUserIDs, p.MaxTokenIDs,
		p.NumPriorities, p.MinThroughput, p.MaxHeight,
		p.MaxMCSPDUSize, p.ProtocolVersion,
	} {
		n += putIntegerTLV(buf[n:], v)
	}
	content := buf[:n]

	// Write SEQUENCE tag + length + content
	if err := WriteTag(w, ClassUniversal, Constructed, TagSequence); err != nil {
		return err
	}
	if err := WriteLength(w, len(content)); err != nil {
		return err
	}
	_, err := w.Write(content)
	return err
}

// putIntegerTLV writes a complete BER INTEGER TLV into dst and returns bytes written.
func putIntegerTLV(dst []byte, value int) int {
	data := encodeIntegerValue(value)
	dst[0] = ClassUniversal | Primitive | TagInteger // tag
	dst[1] = byte(len(data))                         // short-form length (always < 128)
	copy(dst[2:], data)
	return 2 + len(data)
}

// ReadTag reads a BER tag from r.
// Returns the class, constructed flag, and tag number.
func ReadTag(r io.Reader) (class byte, constructed bool, tag int, err error) {
	var first [1]byte
	if _, err = io.ReadFull(r, first[:]); err != nil {
		return 0, false, 0, fmt.Errorf("reading tag: %w", err)
	}

	class = first[0] & 0xC0
	constructed = (first[0] & 0x20) != 0
	tag = int(first[0] & 0x1F)

	if tag == 0x1F {
		// Long form tag
		tag, err = readTagNumber(r)
		if err != nil {
			return 0, false, 0, err
		}
	}

	return class, constructed, tag, nil
}

// readTagNumber reads a multi-byte tag number (base-128).
func readTagNumber(r io.Reader) (int, error) {
	tag := 0
	var b [1]byte
	for {
		if _, err := io.ReadFull(r, b[:]); err != nil {
			return 0, fmt.Errorf("reading tag number: %w", err)
		}
		tag = (tag << 7) | int(b[0]&0x7F)
		if b[0]&0x80 == 0 {
			break
		}
	}
	return tag, nil
}

// ReadLength reads a BER length from r.
func ReadLength(r io.Reader) (int, error) {
	var first [1]byte
	if _, err := io.ReadFull(r, first[:]); err != nil {
		return 0, fmt.Errorf("reading length: %w", err)
	}

	if first[0] < 0x80 {
		return int(first[0]), nil
	}

	// Long form
	numBytes := int(first[0] & 0x7F)
	if numBytes == 0 || numBytes > 4 {
		return 0, fmt.Errorf("unsupported length encoding: %d bytes", numBytes)
	}

	var buf [4]byte
	if _, err := io.ReadFull(r, buf[:numBytes]); err != nil {
		return 0, fmt.Errorf("reading length bytes: %w", err)
	}

	length := 0
	for i := 0; i < numBytes; i++ {
		length = (length << 8) | int(buf[i])
	}
	return length, nil
}

// ReadInteger reads a BER INTEGER from r (tag + length + value).
func ReadInteger(r io.Reader) (int, error) {
	class, _, tag, err := ReadTag(r)
	if err != nil {
		return 0, err
	}
	if class != ClassUniversal || tag != TagInteger {
		return 0, fmt.Errorf("expected INTEGER tag, got class=0x%02X tag=%d", class, tag)
	}

	length, err := ReadLength(r)
	if err != nil {
		return 0, err
	}

	var buf [8]byte
	if _, err := io.ReadFull(r, buf[:length]); err != nil {
		return 0, fmt.Errorf("reading integer value: %w", err)
	}

	return decodeIntegerValue(buf[:length]), nil
}

// decodeIntegerValue decodes a big-endian signed integer from BER bytes.
func decodeIntegerValue(data []byte) int {
	if len(data) == 0 {
		return 0
	}
	// Sign-extend the first byte
	result := int(int8(data[0]))
	for _, b := range data[1:] {
		result = (result << 8) | int(b)
	}
	return result
}

// ReadEnumerated reads a BER ENUMERATED from r.
func ReadEnumerated(r io.Reader) (int, error) {
	class, _, tag, err := ReadTag(r)
	if err != nil {
		return 0, err
	}
	if class != ClassUniversal || tag != TagEnumerated {
		return 0, fmt.Errorf("expected ENUMERATED tag, got class=0x%02X tag=%d", class, tag)
	}

	length, err := ReadLength(r)
	if err != nil {
		return 0, err
	}

	var buf [8]byte
	if _, err := io.ReadFull(r, buf[:length]); err != nil {
		return 0, fmt.Errorf("reading enumerated value: %w", err)
	}

	return decodeIntegerValue(buf[:length]), nil
}

// ReadDomainParameters reads an MCS DomainParameters SEQUENCE from r.
func ReadDomainParameters(r io.Reader) (DomainParameters, error) {
	class, constructed, tag, err := ReadTag(r)
	if err != nil {
		return DomainParameters{}, err
	}
	if class != ClassUniversal || !constructed || tag != TagSequence {
		return DomainParameters{}, fmt.Errorf("expected SEQUENCE, got class=0x%02X constructed=%v tag=%d", class, constructed, tag)
	}

	_, err = ReadLength(r) // consume length; we just read fields sequentially
	if err != nil {
		return DomainParameters{}, err
	}

	var p DomainParameters
	fields := [8]*int{
		&p.MaxChannelIDs, &p.MaxUserIDs, &p.MaxTokenIDs,
		&p.NumPriorities, &p.MinThroughput, &p.MaxHeight,
		&p.MaxMCSPDUSize, &p.ProtocolVersion,
	}
	for _, f := range fields {
		v, err := ReadInteger(r)
		if err != nil {
			return DomainParameters{}, fmt.Errorf("reading domain parameter: %w", err)
		}
		*f = v
	}
	return p, nil
}

// LengthSize returns the number of bytes needed to encode a BER length value.
func LengthSize(length int) int {
	if length < 0x80 {
		return 1
	}
	if length < 0x100 {
		return 2
	}
	if length < 0x10000 {
		return 3
	}
	if length < 0x1000000 {
		return 4
	}
	return 5
}

// TagSize returns the number of bytes needed to encode a BER tag.
func TagSize(tag int) int {
	if tag < 31 {
		return 1
	}
	size := 1 // first byte with 0x1F
	for tag > 0 {
		size++
		tag >>= 7
	}
	return size
}
