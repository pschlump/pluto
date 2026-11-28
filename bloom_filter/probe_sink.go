/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package bloom_filter

import (
	"encoding/binary"
	"math"
	"sync"

	"github.com/pschlump/pluto/hashing"
)

// probeSink is the allocation-free replacement for the buffered hashing.Hasher
// that hashing.HashObject would create per call. It encodes every primitive
// exactly as hashing's buffered hasher does (raw bytes, UTF-8 strings without
// a terminator, little-endian integers and IEEE-754 bits), so the bytes fed to
// Murmur3-128 — and therefore every probe position — are unchanged.
//
// The funnel is an opaque function value, so the sink always escapes to the
// heap; pooling it is what removes the per-call allocations. Funnels must not
// retain the sink after they return.
type probeSink struct {
	data []byte
}

// maxPooledSink bounds the buffer a pooled sink may keep, so one huge value
// does not pin a huge buffer in the pool forever.
const maxPooledSink = 64 << 10

var probeSinkPool = sync.Pool{New: func() any { return &probeSink{data: make([]byte, 0, 64)} }}

func getProbeSink() *probeSink {
	sink := probeSinkPool.Get().(*probeSink)
	sink.data = sink.data[:0]
	return sink
}

func putProbeSink(sink *probeSink) {
	if cap(sink.data) > maxPooledSink {
		return
	}
	probeSinkPool.Put(sink)
}

func (s *probeSink) PutByte(value byte) hashing.PrimitiveSink {
	s.data = append(s.data, value)
	return s
}

func (s *probeSink) PutBytes(value []byte) hashing.PrimitiveSink {
	s.data = append(s.data, value...)
	return s
}

func (s *probeSink) PutString(value string) hashing.PrimitiveSink {
	s.data = append(s.data, value...)
	return s
}

func (s *probeSink) PutUint16(value uint16) hashing.PrimitiveSink {
	s.data = binary.LittleEndian.AppendUint16(s.data, value)
	return s
}

func (s *probeSink) PutUint32(value uint32) hashing.PrimitiveSink {
	s.data = binary.LittleEndian.AppendUint32(s.data, value)
	return s
}

func (s *probeSink) PutUint64(value uint64) hashing.PrimitiveSink {
	s.data = binary.LittleEndian.AppendUint64(s.data, value)
	return s
}

func (s *probeSink) PutInt8(value int8) hashing.PrimitiveSink { return s.PutByte(byte(value)) }

func (s *probeSink) PutInt16(value int16) hashing.PrimitiveSink { return s.PutUint16(uint16(value)) }

func (s *probeSink) PutInt32(value int32) hashing.PrimitiveSink { return s.PutUint32(uint32(value)) }

func (s *probeSink) PutInt64(value int64) hashing.PrimitiveSink { return s.PutUint64(uint64(value)) }

func (s *probeSink) PutFloat32(value float32) hashing.PrimitiveSink {
	return s.PutUint32(math.Float32bits(value))
}

func (s *probeSink) PutFloat64(value float64) hashing.PrimitiveSink {
	return s.PutUint64(math.Float64bits(value))
}
