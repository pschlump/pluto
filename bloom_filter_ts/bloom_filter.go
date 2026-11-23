/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package bloom_filter_ts is the mutex-guarded twin of bloom_filter. Reads and
// writes take an RWMutex; Merge snapshots operands one at a time and never
// holds two filter locks at once.
package bloom_filter_ts

import (
	"io"
	"sync"

	"github.com/pschlump/pluto/bloom_filter"
	"github.com/pschlump/pluto/hashing"
)

var (
	ErrInvalidShape      = bloom_filter.ErrInvalidShape
	ErrIncompatible      = bloom_filter.ErrIncompatible
	ErrInvalidEncoding   = bloom_filter.ErrInvalidEncoding
	ErrUnsupportedFormat = bloom_filter.ErrUnsupportedFormat
)

// BloomFilter is a generic Bloom filter safe for concurrent use.
type BloomFilter[T any] struct {
	inner *bloom_filter.BloomFilter[T]
	lock  sync.RWMutex
}

func New[T any](expectedInsertions int, fpp float64, funnel hashing.Funnel[T]) *BloomFilter[T] {
	return &BloomFilter[T]{inner: bloom_filter.New(expectedInsertions, fpp, funnel)}
}

func NewBloomFilter[T any](expectedInsertions int, fpp float64, funnel hashing.Funnel[T]) *BloomFilter[T] {
	return New(expectedInsertions, fpp, funnel)
}

func NewWithBits[T any](bitSize uint64, hashCount int, funnel hashing.Funnel[T]) *BloomFilter[T] {
	return &BloomFilter[T]{inner: bloom_filter.NewWithBits(bitSize, hashCount, funnel)}
}

func (b *BloomFilter[T]) Put(value T) bool {
	b.lock.Lock()
	defer b.lock.Unlock()
	return b.inner.Put(value)
}

func (b *BloomFilter[T]) MightContain(value T) bool {
	if b == nil {
		return false
	}
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.inner.MightContain(value)
}

func (b *BloomFilter[T]) BitCount() uint64 {
	if b == nil {
		return 0
	}
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.inner.BitCount()
}

func (b *BloomFilter[T]) HashCount() int {
	if b == nil {
		return 0
	}
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.inner.HashCount()
}

func (b *BloomFilter[T]) SetBitCount() uint64 {
	if b == nil {
		return 0
	}
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.inner.SetBitCount()
}

func (b *BloomFilter[T]) Saturation() float64 {
	if b == nil {
		return 0
	}
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.inner.Saturation()
}

func (b *BloomFilter[T]) ExpectedFPP() float64 {
	if b == nil {
		return 0
	}
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.inner.ExpectedFPP()
}

func (b *BloomFilter[T]) Count() uint64 {
	if b == nil {
		return 0
	}
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.inner.Count()
}

func (b *BloomFilter[T]) ApproximateElementCount() uint64 { return b.Count() }

func (b *BloomFilter[T]) Compatible(other *BloomFilter[T]) bool {
	var left, right *bloom_filter.BloomFilter[T]
	if b != nil {
		b.lock.RLock()
		left = b.inner.Copy()
		b.lock.RUnlock()
	}
	if other != nil {
		other.lock.RLock()
		right = other.inner.Copy()
		other.lock.RUnlock()
	}
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return left.Compatible(right)
}

func (b *BloomFilter[T]) Copy() *BloomFilter[T] {
	if b == nil {
		return nil
	}
	b.lock.RLock()
	defer b.lock.RUnlock()
	return &BloomFilter[T]{inner: b.inner.Copy()}
}

func (b *BloomFilter[T]) Clone() *BloomFilter[T] { return b.Copy() }

func (b *BloomFilter[T]) Merge(others ...*BloomFilter[T]) error {
	snapshots := make([]*bloom_filter.BloomFilter[T], len(others))
	self := make([]bool, len(others))
	for i, other := range others {
		if other == b {
			self[i] = true
			continue
		}
		if other == nil {
			return bloom_filter.ErrInvalidShape
		}
		other.lock.RLock()
		snapshots[i] = other.inner.Copy()
		other.lock.RUnlock()
	}
	b.lock.Lock()
	defer b.lock.Unlock()
	for i, snapshot := range snapshots {
		if !self[i] {
			if snapshot == nil {
				return bloom_filter.ErrInvalidShape
			}
			if !b.inner.Compatible(snapshot) {
				return bloom_filter.ErrIncompatible
			}
		}
	}
	for i, snapshot := range snapshots {
		if !self[i] {
			if err := b.inner.Merge(snapshot); err != nil {
				return err
			}
		}
	}
	return nil
}

func (b *BloomFilter[T]) Bytes() []byte {
	if b == nil {
		return nil
	}
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.inner.Bytes()
}

func (b *BloomFilter[T]) WriteTo(w io.Writer) (int64, error) {
	if b == nil {
		return 0, bloom_filter.ErrInvalidShape
	}
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.inner.WriteTo(w)
}

func (b *BloomFilter[T]) WriteGuavaTo(w io.Writer) error {
	if b == nil {
		return bloom_filter.ErrInvalidShape
	}
	b.lock.RLock()
	defer b.lock.RUnlock()
	return b.inner.WriteGuavaTo(w)
}

func ReadGuavaFrom[T any](r io.Reader, funnel hashing.Funnel[T]) (*BloomFilter[T], error) {
	inner, err := bloom_filter.ReadGuavaFrom(r, funnel)
	if err != nil {
		return nil, err
	}
	return &BloomFilter[T]{inner: inner}, nil
}

func FromGuavaBytes[T any](data []byte, funnel hashing.Funnel[T]) (*BloomFilter[T], error) {
	inner, err := bloom_filter.FromGuavaBytes(data, funnel)
	if err != nil {
		return nil, err
	}
	return &BloomFilter[T]{inner: inner}, nil
}

func FromBytes[T any](data []byte, funnel hashing.Funnel[T]) (*BloomFilter[T], error) {
	inner, err := bloom_filter.FromBytes(data, funnel)
	if err != nil {
		return nil, err
	}
	return &BloomFilter[T]{inner: inner}, nil
}

func ReadFrom[T any](r io.Reader, funnel hashing.Funnel[T]) (*BloomFilter[T], error) {
	inner, err := bloom_filter.ReadFrom(r, funnel)
	if err != nil {
		return nil, err
	}
	return &BloomFilter[T]{inner: inner}, nil
}
