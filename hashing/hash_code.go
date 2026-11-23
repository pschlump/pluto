/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package hashing

import (
	"encoding/hex"
	"fmt"
)

// ByteOrder identifies the interpretation of integer accessors. It does not
// change the stored byte sequence.
type ByteOrder uint8

const (
	LittleEndian ByteOrder = iota
	BigEndian
)

// HashCode is an immutable byte string with explicit integer byte order.
// Constructors and accessors copy data, so a HashCode never aliases caller
// storage. Its zero value is an empty, little-endian code.
type HashCode struct {
	bytes []byte
	order ByteOrder
}

// NewHashCode copies data and associates it with order.
func NewHashCode(data []byte, order ByteOrder) HashCode {
	if order != LittleEndian && order != BigEndian {
		panic("hashing: NewHashCode requires LittleEndian or BigEndian")
	}
	return HashCode{bytes: append([]byte(nil), data...), order: order}
}

func hashFromUint64(value uint64) HashCode {
	data := make([]byte, 8)
	for i := range data {
		data[i] = byte(value >> (8 * i))
	}
	return HashCode{bytes: data, order: LittleEndian}
}

func hashFromUint32(value uint32) HashCode {
	return HashCode{bytes: []byte{byte(value), byte(value >> 8), byte(value >> 16), byte(value >> 24)}, order: LittleEndian}
}

// Len returns the number of bytes.
func (h HashCode) Len() int { return len(h.bytes) }

// Bits returns the number of bits.
func (h HashCode) Bits() int { return len(h.bytes) * 8 }

// Order returns the byte order used by integer accessors.
func (h HashCode) Order() ByteOrder { return h.order }

// Bytes returns a copy of the exact byte sequence.
func (h HashCode) Bytes() []byte { return append([]byte(nil), h.bytes...) }

// AsUint32 returns the low four bytes using the code's byte order. It panics if
// fewer than four bytes are available.
func (h HashCode) AsUint32() uint32 {
	if len(h.bytes) < 4 {
		panic("hashing: HashCode.AsUint32 requires at least 4 bytes")
	}
	if h.order == BigEndian {
		return uint32(h.bytes[0])<<24 | uint32(h.bytes[1])<<16 | uint32(h.bytes[2])<<8 | uint32(h.bytes[3])
	}
	return uint32(h.bytes[0]) | uint32(h.bytes[1])<<8 | uint32(h.bytes[2])<<16 | uint32(h.bytes[3])<<24
}

// AsInt32 returns AsUint32 reinterpreted as a signed integer.
func (h HashCode) AsInt32() int32 { return int32(h.AsUint32()) }

// AsUint64 returns the low eight bytes using the code's byte order. It panics
// if fewer than eight bytes are available.
func (h HashCode) AsUint64() uint64 {
	if len(h.bytes) < 8 {
		panic("hashing: HashCode.AsUint64 requires at least 8 bytes")
	}
	var value uint64
	if h.order == BigEndian {
		for i := range 8 {
			value = value<<8 | uint64(h.bytes[i])
		}
	} else {
		for i := 7; i >= 0; i-- {
			value = value<<8 | uint64(h.bytes[i])
		}
	}
	return value
}

// AsInt64 returns AsUint64 reinterpreted as a signed integer.
func (h HashCode) AsInt64() int64 { return int64(h.AsUint64()) }

// PadToUint64 returns AsUint64, or the available bytes zero-extended according
// to byte order when the code contains fewer than eight bytes.
func (h HashCode) PadToUint64() uint64 {
	if len(h.bytes) >= 8 {
		return h.AsUint64()
	}
	var value uint64
	if h.order == BigEndian {
		for _, b := range h.bytes {
			value = value<<8 | uint64(b)
		}
	} else {
		for i := len(h.bytes) - 1; i >= 0; i-- {
			value = value<<8 | uint64(h.bytes[i])
		}
	}
	return value
}

// WriteBytesTo copies at most maxLength bytes into destination at offset and
// returns the number copied. Invalid ranges return an error.
func (h HashCode) WriteBytesTo(destination []byte, offset, maxLength int) (int, error) {
	if offset < 0 || offset > len(destination) || maxLength < 0 || maxLength > len(destination)-offset {
		return 0, fmt.Errorf("hashing: invalid bounded copy offset=%d length=%d destination=%d", offset, maxLength, len(destination))
	}
	n := min(maxLength, len(h.bytes))
	copy(destination[offset:offset+n], h.bytes[:n])
	return n, nil
}

// Equal compares both bytes and byte order.
func (h HashCode) Equal(other HashCode) bool {
	if h.order != other.order || len(h.bytes) != len(other.bytes) {
		return false
	}
	for i := range h.bytes {
		if h.bytes[i] != other.bytes[i] {
			return false
		}
	}
	return true
}

// String returns lowercase hexadecimal in stored byte order.
func (h HashCode) String() string { return hex.EncodeToString(h.bytes) }

// CombineOrdered combines equal-length codes while preserving sequence order.
// It matches Guava's byte-wise multiply-by-37/xor recurrence.
func CombineOrdered(codes ...HashCode) HashCode {
	if len(codes) == 0 {
		panic("hashing: CombineOrdered requires at least one code")
	}
	out := make([]byte, codes[0].Len())
	for _, code := range codes {
		if code.Len() != len(out) {
			panic("hashing: CombineOrdered requires equal-length codes")
		}
		for i, b := range code.bytes {
			out[i] = out[i]*37 ^ b
		}
	}
	return NewHashCode(out, codes[0].order)
}

// CombineUnordered combines equal-length codes by byte-wise modular addition.
func CombineUnordered(codes ...HashCode) HashCode {
	if len(codes) == 0 {
		panic("hashing: CombineUnordered requires at least one code")
	}
	out := make([]byte, codes[0].Len())
	for _, code := range codes {
		if code.Len() != len(out) {
			panic("hashing: CombineUnordered requires equal-length codes")
		}
		for i, b := range code.bytes {
			out[i] += b
		}
	}
	return NewHashCode(out, codes[0].order)
}

// ConsistentHash selects [0,buckets) with Guava's jump-consistent-hash
// algorithm. Growing the bucket count preserves all assignments except those
// moved to newly added buckets. It panics when buckets is not positive.
func ConsistentHash(code HashCode, buckets int) int {
	if buckets <= 0 {
		panic("hashing: ConsistentHash requires a positive bucket count")
	}
	state := code.PadToUint64()
	candidate := int64(0)
	for {
		state = state*2862933555777941757 + 1
		generated := int64(float64(candidate+1) / (float64(state>>11) / float64(uint64(1)<<53)))
		if generated >= 0 && generated < int64(buckets) {
			candidate = generated
		} else {
			return int(candidate)
		}
	}
}
