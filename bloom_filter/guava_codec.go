/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package bloom_filter

import (
	"encoding/binary"
	"fmt"
	"io"
	"math/bits"

	"github.com/pschlump/pluto/hashing"
)

// WriteGuavaTo writes Guava BloomFilter.writeTo's Murmur3 strategy encoding:
// strategy byte 1, hash-count byte, big-endian word count, big-endian words.
// It is distinct from Bytes and does not preserve a non-word-aligned bit size.
func (b *BloomFilter[T]) WriteGuavaTo(w io.Writer) error {
	if b == nil || b.bitSize == 0 {
		return fmt.Errorf("%w: zero-value filter", ErrInvalidShape)
	}
	// Encode into one buffer of at most guavaWriteChunk bytes and write it in
	// as few calls as possible: one Write per 8-byte word made the caller's
	// writer (e.g. a bytes.Buffer) grow step by step.
	total := 6 + len(b.words)*8
	buffer := make([]byte, 0, min(total, guavaWriteChunk))
	buffer = append(buffer, 1, byte(b.hashCount))
	buffer = binary.BigEndian.AppendUint32(buffer, uint32(len(b.words)))
	for _, word := range b.words {
		if len(buffer)+8 > cap(buffer) {
			if err := writeAll(w, buffer); err != nil {
				return err
			}
			buffer = buffer[:0]
		}
		buffer = binary.BigEndian.AppendUint64(buffer, word)
	}
	return writeAll(w, buffer)
}

// guavaWriteChunk caps WriteGuavaTo's encode buffer; a multiple of 8 plus the
// 6-byte header so small filters are written in exactly one call.
const guavaWriteChunk = 6 + 4096*8

// ReadGuavaFrom reads Guava's compact Murmur3 Bloom encoding. As in Guava, a
// decoded filter's bit size is rounded up to a whole 64-bit word.
func ReadGuavaFrom[T any](r io.Reader, funnel hashing.Funnel[T]) (*BloomFilter[T], error) {
	var header [6]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return nil, err
	}
	if header[0] != 1 {
		return nil, fmt.Errorf("%w: Guava strategy %d", ErrUnsupportedFormat, header[0])
	}
	if header[1] == 0 {
		return nil, fmt.Errorf("%w: zero hash count", ErrInvalidEncoding)
	}
	words := binary.BigEndian.Uint32(header[2:])
	if words == 0 {
		return nil, fmt.Errorf("%w: zero words", ErrInvalidEncoding)
	}
	data := make([]byte, int(words)*8)
	if _, err := io.ReadFull(r, data); err != nil {
		return nil, err
	}
	filter := NewWithBits(uint64(words)*64, int(header[1]), funnel)
	for i := range filter.words {
		filter.words[i] = binary.BigEndian.Uint64(data[i*8:])
		filter.set += uint64(bits.OnesCount64(filter.words[i]))
	}
	return filter, nil
}

// FromGuavaBytes decodes a complete Guava compact encoding and rejects trailing
// data.
func FromGuavaBytes[T any](data []byte, funnel hashing.Funnel[T]) (*BloomFilter[T], error) {
	if len(data) < 6 {
		return nil, fmt.Errorf("%w: short Guava header", ErrInvalidEncoding)
	}
	words := binary.BigEndian.Uint32(data[2:])
	if len(data) != 6+int(words)*8 {
		return nil, fmt.Errorf("%w: wrong Guava encoding length", ErrInvalidEncoding)
	}
	return ReadGuavaFrom(&sliceReader{data: data}, funnel)
}

type sliceReader struct {
	data []byte
}

func (r *sliceReader) Read(destination []byte) (int, error) {
	if len(r.data) == 0 {
		return 0, io.EOF
	}
	n := copy(destination, r.data)
	r.data = r.data[n:]
	return n, nil
}

func writeAll(w io.Writer, data []byte) error {
	for len(data) > 0 {
		n, err := w.Write(data)
		if err != nil {
			return err
		}
		if n <= 0 {
			return io.ErrShortWrite
		}
		data = data[n:]
	}
	return nil
}
