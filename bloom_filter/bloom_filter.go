/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package bloom_filter implements a generic Bloom filter using a typed hashing
// funnel and Guava's Murmur3-128 double-hashing strategy. It is separate from
// package bloom, whose probes and serialized bytes remain frozen.
package bloom_filter

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"math/bits"

	"github.com/pschlump/pluto/hashing"
)

const (
	strategyVersion = 1
	wireHeaderSize  = 16
	maxHashCount    = 255
)

var wireMagic = [4]byte{'P', 'B', 'F', strategyVersion}

var (
	ErrInvalidShape      = errors.New("bloom_filter: invalid shape")
	ErrIncompatible      = errors.New("bloom_filter: incompatible filters")
	ErrInvalidEncoding   = errors.New("bloom_filter: invalid encoding")
	ErrUnsupportedFormat = errors.New("bloom_filter: unsupported stream format")
)

// BloomFilter is a fixed-shape probabilistic set. The zero value is readable
// but cannot be written. A funnel must encode values identically in filters
// that will be merged; Go functions cannot be compared for semantic identity.
type BloomFilter[T any] struct {
	funnel    hashing.Funnel[T]
	words     []uint64
	bitSize   uint64
	hashCount int
	set       uint64
}

// New returns a filter sized for expectedInsertions distinct values and fpp.
// Both parameters must be positive, fpp must be less than one, and funnel must
// be non-nil. Invalid arguments panic.
func New[T any](expectedInsertions int, fpp float64, funnel hashing.Funnel[T]) *BloomFilter[T] {
	if expectedInsertions <= 0 {
		panic("bloom_filter: New requires positive expected insertions")
	}
	if !(fpp > 0 && fpp < 1) {
		panic("bloom_filter: New requires 0 < fpp < 1")
	}
	bitSizeFloat := float64(expectedInsertions) * -math.Log(fpp) / (math.Ln2 * math.Ln2)
	if math.IsInf(bitSizeFloat, 0) || bitSizeFloat > float64(^uint64(0)-64) {
		panic("bloom_filter: requested shape is too large")
	}
	bitSize := uint64(bitSizeFloat)
	if bitSize == 0 {
		bitSize = 1
	}
	hashCount := int(math.Round(float64(bitSize) / float64(expectedInsertions) * math.Ln2))
	if hashCount < 1 {
		hashCount = 1
	}
	if hashCount > maxHashCount {
		hashCount = maxHashCount
	}
	return NewWithBits(bitSize, hashCount, funnel)
}

// NewBloomFilter is an explicit-name alias for New.
func NewBloomFilter[T any](expectedInsertions int, fpp float64, funnel hashing.Funnel[T]) *BloomFilter[T] {
	return New(expectedInsertions, fpp, funnel)
}

// NewWithBits returns an explicitly shaped filter. It panics for a nil funnel,
// zero bits, or a hash count outside [1,255].
func NewWithBits[T any](bitSize uint64, hashCount int, funnel hashing.Funnel[T]) *BloomFilter[T] {
	if bitSize == 0 || bitSize > ^uint64(0)-63 || hashCount < 1 || hashCount > maxHashCount {
		panic("bloom_filter: NewWithBits received an invalid shape")
	}
	if funnel == nil {
		panic("bloom_filter: NewWithBits requires a non-nil funnel")
	}
	words := (bitSize + 63) / 64
	if uint64(int(words)) != words {
		panic("bloom_filter: requested shape exceeds addressable memory")
	}
	return &BloomFilter[T]{funnel: funnel, words: make([]uint64, int(words)), bitSize: words * 64, hashCount: hashCount}
}

func (b *BloomFilter[T]) positions(value T) []uint64 {
	code := hashing.HashObject(value, b.funnel, hashing.Murmur3_128())
	bytes := code.Bytes()
	hash1 := binary.LittleEndian.Uint64(bytes)
	hash2 := binary.LittleEndian.Uint64(bytes[8:])
	positions := make([]uint64, b.hashCount)
	combined := hash1
	for i := range positions {
		positions[i] = (combined & math.MaxInt64) % b.bitSize
		combined += hash2
	}
	return positions
}

// Put records value and reports whether any probe bit changed.
func (b *BloomFilter[T]) Put(value T) bool {
	if b == nil || b.bitSize == 0 {
		panic("bloom_filter: Put on a zero-value filter; use New or NewWithBits")
	}
	changed := false
	for _, position := range b.positions(value) {
		word, mask := position>>6, uint64(1)<<(position&63)
		if b.words[word]&mask == 0 {
			b.words[word] |= mask
			b.set++
			changed = true
		}
	}
	return changed
}

// MightContain reports whether all probe bits are set. Added elements never
// produce false negatives; absent elements may produce false positives.
func (b *BloomFilter[T]) MightContain(value T) bool {
	if b == nil || b.bitSize == 0 {
		return false
	}
	for _, position := range b.positions(value) {
		if b.words[position>>6]&(uint64(1)<<(position&63)) == 0 {
			return false
		}
	}
	return true
}

// BitCount returns the number of addressable bits.
func (b *BloomFilter[T]) BitCount() uint64 {
	if b == nil {
		return 0
	}
	return b.bitSize
}

// HashCount returns the number of probes per value.
func (b *BloomFilter[T]) HashCount() int {
	if b == nil {
		return 0
	}
	return b.hashCount
}

// SetBitCount returns the current number of set bits.
func (b *BloomFilter[T]) SetBitCount() uint64 {
	if b == nil {
		return 0
	}
	return b.set
}

// Saturation returns the fraction of bits set.
func (b *BloomFilter[T]) Saturation() float64 {
	if b == nil || b.bitSize == 0 {
		return 0
	}
	return float64(b.set) / float64(b.bitSize)
}

// ExpectedFPP returns Saturation()^HashCount(), which is 1 at full saturation.
func (b *BloomFilter[T]) ExpectedFPP() float64 {
	if b == nil || b.bitSize == 0 {
		return 0
	}
	return math.Pow(b.Saturation(), float64(b.hashCount))
}

// Count estimates distinct inserted values using the fill-ratio estimator.
// Full saturation carries no count information and returns bitSize.
func (b *BloomFilter[T]) Count() uint64 {
	if b == nil || b.bitSize == 0 || b.set == 0 {
		return 0
	}
	if b.set == b.bitSize {
		return b.bitSize
	}
	estimate := -float64(b.bitSize) / float64(b.hashCount) * math.Log1p(-float64(b.set)/float64(b.bitSize))
	return uint64(math.Round(estimate))
}

// ApproximateElementCount is an alias for Count.
func (b *BloomFilter[T]) ApproximateElementCount() uint64 { return b.Count() }

// Compatible reports identical shape and strategy. Funnel semantic equality
// cannot be checked in Go and remains the caller's responsibility.
func (b *BloomFilter[T]) Compatible(other *BloomFilter[T]) bool {
	return b.BitCount() == other.BitCount() && b.HashCount() == other.HashCount()
}

// Copy returns an independent copy. The immutable funnel is shared.
func (b *BloomFilter[T]) Copy() *BloomFilter[T] {
	if b == nil || b.bitSize == 0 {
		return nil
	}
	copyFilter := &BloomFilter[T]{funnel: b.funnel, words: append([]uint64(nil), b.words...), bitSize: b.bitSize, hashCount: b.hashCount, set: b.set}
	return copyFilter
}

// Clone is an alias for Copy.
func (b *BloomFilter[T]) Clone() *BloomFilter[T] { return b.Copy() }

// Merge ORs compatible filters into b. Validation occurs before mutation.
func (b *BloomFilter[T]) Merge(others ...*BloomFilter[T]) error {
	if b == nil || b.bitSize == 0 {
		return fmt.Errorf("%w: zero-value destination", ErrInvalidShape)
	}
	for _, other := range others {
		if other == nil || other.bitSize == 0 {
			return fmt.Errorf("%w: zero-value operand", ErrInvalidShape)
		}
		if !b.Compatible(other) {
			return ErrIncompatible
		}
	}
	for _, other := range others {
		if other == b {
			continue
		}
		for i, word := range other.words {
			combined := b.words[i] | word
			b.set += uint64(bits.OnesCount64(combined) - bits.OnesCount64(b.words[i]))
			b.words[i] = combined
		}
	}
	return nil
}

// Bytes returns the versioned Pluto stream encoding: magic PBF and version 1,
// little-endian uint64 bit size, uint32 hash count, then little-endian words.
func (b *BloomFilter[T]) Bytes() []byte {
	if b == nil || b.bitSize == 0 {
		return nil
	}
	out := make([]byte, wireHeaderSize+len(b.words)*8)
	copy(out, wireMagic[:])
	binary.LittleEndian.PutUint64(out[4:], b.bitSize)
	binary.LittleEndian.PutUint32(out[12:], uint32(b.hashCount))
	for i, word := range b.words {
		binary.LittleEndian.PutUint64(out[wireHeaderSize+i*8:], word)
	}
	return out
}

// WriteTo writes Bytes to w, including short-write checking.
func (b *BloomFilter[T]) WriteTo(w io.Writer) (int64, error) {
	data := b.Bytes()
	if data == nil {
		return 0, fmt.Errorf("%w: zero-value filter", ErrInvalidShape)
	}
	total := int64(len(data))
	for len(data) > 0 {
		n, err := w.Write(data)
		if err != nil {
			return total - int64(len(data)), err
		}
		if n <= 0 {
			return total - int64(len(data)), io.ErrShortWrite
		}
		data = data[n:]
	}
	return total, nil
}

// FromBytes decodes a versioned stream encoding and associates funnel.
func FromBytes[T any](data []byte, funnel hashing.Funnel[T]) (*BloomFilter[T], error) {
	if len(data) < wireHeaderSize {
		return nil, fmt.Errorf("%w: short header", ErrInvalidEncoding)
	}
	if string(data[:3]) != "PBF" || data[3] != strategyVersion {
		return nil, ErrUnsupportedFormat
	}
	bitSize := binary.LittleEndian.Uint64(data[4:])
	hashCount := binary.LittleEndian.Uint64(data[12:]) & math.MaxUint32
	if bitSize == 0 || hashCount < 1 || hashCount > maxHashCount {
		return nil, fmt.Errorf("%w: bit size %d, hash count %d", ErrInvalidShape, bitSize, hashCount)
	}
	words := (bitSize + 63) / 64
	if words > uint64((len(data)-wireHeaderSize)/8) || len(data) != wireHeaderSize+int(words)*8 {
		return nil, fmt.Errorf("%w: wrong length", ErrInvalidEncoding)
	}
	filter := NewWithBits(bitSize, int(hashCount), funnel)
	for i := range filter.words {
		filter.words[i] = binary.LittleEndian.Uint64(data[wireHeaderSize+i*8:])
		filter.set += uint64(bits.OnesCount64(filter.words[i]))
	}
	if remainder := bitSize & 63; remainder != 0 && filter.words[len(filter.words)-1]>>remainder != 0 {
		return nil, fmt.Errorf("%w: set padding bit", ErrInvalidEncoding)
	}
	return filter, nil
}

// ReadFrom reads exactly one encoded filter from r. Trailing data belongs to
// the caller's next stream value.
func ReadFrom[T any](r io.Reader, funnel hashing.Funnel[T]) (*BloomFilter[T], error) {
	header := make([]byte, wireHeaderSize)
	if _, err := io.ReadFull(r, header); err != nil {
		return nil, err
	}
	if string(header[:3]) != "PBF" || header[3] != strategyVersion {
		return nil, ErrUnsupportedFormat
	}
	bitSize := binary.LittleEndian.Uint64(header[4:])
	if bitSize == 0 || bitSize > ^uint64(0)-63 {
		return nil, ErrInvalidShape
	}
	data := make([]byte, wireHeaderSize+int((bitSize+63)/64)*8)
	copy(data, header)
	if _, err := io.ReadFull(r, data[wireHeaderSize:]); err != nil {
		return nil, err
	}
	return FromBytes(data, funnel)
}
