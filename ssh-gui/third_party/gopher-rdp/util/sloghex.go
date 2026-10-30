// Package sloghex provides zero-allocation slog attributes for hex-formatted
// integers. The formatting is lazy: fmt.Sprintf only runs when the log record
// is actually written by the handler, so disabled log levels incur no cost.
package util

import (
	"encoding/hex"
	"fmt"
	"log/slog"
)

// Hex2 returns a slog.Attr that lazily formats val as "0x%02X".
func Hex2(key string, val uint8) slog.Attr { return slog.Any(key, h2(val)) }

// Hex4 returns a slog.Attr that lazily formats val as "0x%04X".
func Hex4(key string, val uint16) slog.Attr { return slog.Any(key, h4(val)) }

// Hex8 returns a slog.Attr that lazily formats val as "0x%08X".
func Hex8(key string, val uint32) slog.Attr { return slog.Any(key, h8(val)) }

// Bytes returns a slog.Attr that lazily formats val as uppercase hex bytes.
func Bytes(key string, val []byte) slog.Attr { return slog.Any(key, hBytes(val)) }

type h2 uint8
type h4 uint16
type h8 uint32
type hBytes []byte

func (h h2) LogValue() slog.Value     { return slog.StringValue(fmt.Sprintf("0x%02X", uint8(h))) }
func (h h4) LogValue() slog.Value     { return slog.StringValue(fmt.Sprintf("0x%04X", uint16(h))) }
func (h h8) LogValue() slog.Value     { return slog.StringValue(fmt.Sprintf("0x%08X", uint32(h))) }
func (h hBytes) LogValue() slog.Value { return slog.StringValue(hex.EncodeToString(h)) }
