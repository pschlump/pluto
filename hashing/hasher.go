/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package hashing

import (
	"encoding/binary"
	"math"
)

type bufferedHasher struct {
	function hashFunction
	data     []byte
}

func (h *bufferedHasher) PutByte(value byte) PrimitiveSink {
	h.data = append(h.data, value)
	return h
}
func (h *bufferedHasher) PutBytes(value []byte) PrimitiveSink {
	h.data = append(h.data, value...)
	return h
}
func (h *bufferedHasher) PutString(value string) PrimitiveSink {
	h.data = append(h.data, value...)
	return h
}
func (h *bufferedHasher) PutUint16(value uint16) PrimitiveSink {
	var data [2]byte
	binary.LittleEndian.PutUint16(data[:], value)
	return h.PutBytes(data[:])
}
func (h *bufferedHasher) PutUint32(value uint32) PrimitiveSink {
	var data [4]byte
	binary.LittleEndian.PutUint32(data[:], value)
	return h.PutBytes(data[:])
}
func (h *bufferedHasher) PutUint64(value uint64) PrimitiveSink {
	var data [8]byte
	binary.LittleEndian.PutUint64(data[:], value)
	return h.PutBytes(data[:])
}
func (h *bufferedHasher) PutInt8(value int8) PrimitiveSink { return h.PutByte(byte(value)) }
func (h *bufferedHasher) PutInt16(value int16) PrimitiveSink {
	return h.PutUint16(uint16(value))
}
func (h *bufferedHasher) PutInt32(value int32) PrimitiveSink {
	return h.PutUint32(uint32(value))
}
func (h *bufferedHasher) PutInt64(value int64) PrimitiveSink {
	return h.PutUint64(uint64(value))
}
func (h *bufferedHasher) PutFloat32(value float32) PrimitiveSink {
	return h.PutUint32(math.Float32bits(value))
}
func (h *bufferedHasher) PutFloat64(value float64) PrimitiveSink {
	return h.PutUint64(math.Float64bits(value))
}
func (h *bufferedHasher) Hash() HashCode { return h.function.hash(h.data) }
