/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package bitmap

import (
	"encoding/json"
	"fmt"
)

// Bitmap is a bitmap buffer as a named type so it can carry JSON
// methods: a Bitmap is exactly the []byte the pure functions operate
// on (a plain conversion either way, no constructor, no stored state),
// and SetBit/GetBit and the rest of the package apply to it unchanged.
// The zero value — a nil Bitmap — is a usable, fully cleared bitmap.
type Bitmap []byte

// MarshalJSON implements json.Marshaler so a Bitmap can be used
// directly with the encoding/json package.  The bitmap is encoded as a
// JSON array of the offsets of its set bits in ascending order — the
// natural bit order, bit 0 (the MSB of byte 0) first — so the one-byte
// buffer {0x81} encodes as [0,7].
//
// The positions are plain integers, so there is no element-level
// marshaling to honor — only the bitmap-to-array shape is pluto's.
//
// A bitmap with no set bits encodes as []; in particular a nil Bitmap
// (a zero value) encodes as [], the "nil reads as empty" contract.
// Note that json.Marshal on a nil *Bitmap never reaches this method —
// the json package writes null for nil pointers itself.
// Complexity is O(n) in the length of the buffer.
func (b Bitmap) MarshalJSON() ([]byte, error) {
	positions := make([]uint64, 0)
	for i, by := range b {
		if by == 0 {
			continue
		}
		base := uint64(i) << 3
		for j := uint(0); j < 8; j++ {
			if by>>(7-j)&1 == 1 {
				positions = append(positions, base+uint64(j))
			}
		}
	}
	return json.Marshal(positions)
}

// UnmarshalJSON implements json.Unmarshaler so a Bitmap can be used
// directly with the encoding/json package.  data must be a JSON array
// of bit offsets (or null), as produced by MarshalJSON; the decoded
// offsets replace the current contents — the buffer is cleared and
// then each listed bit is set through SetBit, so the result is exactly
// long enough to hold the highest offset and a duplicate offset is
// harmless.  A zero-value Bitmap needs no constructor: it is usable
// before and after unmarshaling.
//
// The JSON is decoded and every offset validated before anything is
// mutated, so a decode error (malformed JSON, a non-array document, a
// negative or non-integer offset) or an offset past MaxBitOffset —
// reported as an ErrBitOffset-wrapping error, the SetBit range rule —
// is returned with the bitmap untouched.
//
// Unmarshaling stores bits, so data that would set a bit through a nil
// *Bitmap panics with the standard message.  An empty array or null
// clears the bitmap and is tolerated everywhere — it stores nothing.
// Complexity is O(m) in the highest offset.
func (b *Bitmap) UnmarshalJSON(data []byte) error {
	var positions []uint64
	if err := json.Unmarshal(data, &positions); err != nil {
		return err
	}

	// The write contract only fires when a bit would actually be set.
	if len(positions) > 0 && b == nil {
		panic("bitmap: UnmarshalJSON called on a nil bitmap")
	}
	if b == nil {
		return nil // null or []: nothing to store, nothing to clear
	}

	// Validate every offset before touching the bitmap.
	for _, p := range positions {
		if p > MaxBitOffset {
			return fmt.Errorf("%w: UnmarshalJSON offset %d", ErrBitOffset, p)
		}
	}

	// Clear, then set: build the replacement on a fresh buffer and swap
	// it in only after every bit is in place.
	var buf []byte
	for _, p := range positions {
		nb, _, err := SetBit(buf, p, 1)
		if err != nil {
			return err
		}
		buf = nb
	}
	*b = Bitmap(buf)
	return nil
}
