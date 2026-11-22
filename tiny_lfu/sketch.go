/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package tiny_lfu

import (
	"hash/maphash"
	"math/bits"
)

const (
	resetMask     = uint64(0x7777777777777777)
	oneMask       = uint64(0x1111111111111111)
	minSketchSize = 256
)

// FrequencySketch is a four-bit Count-Min sketch with periodic aging. It
// estimates recent access frequencies in the range 0 through 15. Hash
// collisions can overestimate a frequency, but never underestimate one before
// aging.
//
// The zero value is uninitialized: Frequency returns 0 and Increment does
// nothing until EnsureCapacity is called. FrequencySketch is not safe for
// concurrent use.
type FrequencySketch[K comparable] struct {
	table      []uint64
	blockShift int
	sampleSize int
	size       int
	hash       func(K) uint64
}

// NewFrequencySketch creates a sketch sized for a cache of maximumSize
// entries. It panics if maximumSize is negative.
func NewFrequencySketch[K comparable](maximumSize int) *FrequencySketch[K] {
	s := &FrequencySketch[K]{}
	s.EnsureCapacity(maximumSize)
	return s
}

// NewFrequencySketchWithHash creates a sketch using hash for key hashing. It
// is useful when keys already have a stable hash or when deterministic hash
// vectors are required. It panics if maximumSize is negative or hash is nil.
func NewFrequencySketchWithHash[K comparable](maximumSize int, hash func(K) uint64) *FrequencySketch[K] {
	if hash == nil {
		panic("tiny_lfu: NewFrequencySketchWithHash called with a nil hash function")
	}
	s := &FrequencySketch[K]{hash: hash}
	s.EnsureCapacity(maximumSize)
	return s
}

// EnsureCapacity sizes the sketch for maximumSize cache entries. Growing the
// table forgets all counts. Retaining an oversized table preserves counts and
// adjusts the aging period to the new maximum. It panics if maximumSize is
// negative.
func (s *FrequencySketch[K]) EnsureCapacity(maximumSize int) {
	if maximumSize < 0 {
		panic("tiny_lfu: FrequencySketch.EnsureCapacity called with a negative maximum size")
	}
	if s.hash == nil {
		seed := maphash.MakeSeed()
		s.hash = func(key K) uint64 { return maphash.Comparable(seed, key) }
	}

	maximum := max(maximumSize, minSketchSize)
	var sampleSize int
	if maximum > int(^uint(0)>>1)/10 {
		sampleSize = int(^uint(0) >> 1)
	} else {
		sampleSize = 10 * maximum
	}
	if len(s.table) >= maximum {
		if sampleSize != s.sampleSize {
			s.sampleSize = sampleSize
			s.size = min(s.size, sampleSize-1)
		}
		return
	}

	s.table = make([]uint64, ceilingPowerOfTwo(maximum))
	s.blockShift = bits.LeadingZeros32(uint32((len(s.table) >> 3) - 1))
	s.sampleSize = sampleSize
	s.size = 0
}

// Frequency returns the estimated recent access frequency of key, capped at
// 15. An uninitialized sketch returns 0.
func (s *FrequencySketch[K]) Frequency(key K) int {
	if s == nil || len(s.table) == 0 {
		return 0
	}

	frequency := 15
	blockHash := spread(uint32(s.hash(key)))
	counterHash := rehash(blockHash)
	block := int(blockHash>>s.blockShift) << 3
	for i := range 4 {
		h := counterHash >> (i << 3)
		index := (h >> 1) & 15
		offset := h & 1
		slot := block + int(offset) + (i << 1)
		count := int((s.table[slot] >> (index << 2)) & 0xf)
		frequency = min(frequency, count)
	}
	return frequency
}

// Increment records one access of key. After the configured sample size is
// observed, all counters are halved so old popularity gradually fades.
func (s *FrequencySketch[K]) Increment(key K) {
	if s == nil || len(s.table) == 0 {
		return
	}

	blockHash := spread(uint32(s.hash(key)))
	counterHash := rehash(blockHash)
	block := int(blockHash>>s.blockShift) << 3
	added := false
	for i := range 4 {
		h := counterHash >> (i << 3)
		index := int((h >> 1) & 15)
		slot := block + int(h&1) + (i << 1)
		added = s.incrementAt(slot, index) || added
	}
	if added {
		s.size++
		if s.size == s.sampleSize {
			s.reset()
		}
	}
}

func (s *FrequencySketch[K]) incrementAt(slot, index int) bool {
	offset := index << 2
	mask := uint64(0xf) << offset
	if s.table[slot]&mask == mask {
		return false
	}
	s.table[slot] += uint64(1) << offset
	return true
}

func (s *FrequencySketch[K]) reset() {
	count := 0
	for i, slot := range s.table {
		count += bits.OnesCount64(slot & oneMask)
		s.table[i] = (slot >> 1) & resetMask
	}
	s.size = max(0, (s.size-count/4)/2)
}

func spread(x uint32) uint32 {
	x ^= x >> 17
	x *= 0xed5ad4bb
	x ^= x >> 11
	x *= 0xac4c1b51
	x ^= x >> 15
	return x
}

func rehash(x uint32) uint32 {
	x *= 0x31848bab
	x ^= x >> 14
	return x
}

func ceilingPowerOfTwo(x int) int {
	if x <= 1 {
		return 1
	}
	return 1 << bits.Len(uint(x-1))
}
