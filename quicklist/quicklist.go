/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package quicklist implements a generic segmented deque — the Go
// equivalent of a Redis quicklist.  The list is a doubly-linked list of
// segments; each segment is a packed []T holding up to a configurable
// number of entries (the fill target, default 128) instead of one
// heap-allocated node per element.  Compared with dll/dqueue this cuts
// the per-element pointer overhead to nearly zero: a 1M-element list of
// small strings costs within ~1.5x of a plain packed slice, several
// times less than a linked list.
//
//	             head ──────────────────────────────► tail
//	┌──────────┐     ┌──────────┐     ┌──────────┐
//	│ []T seg  │ ◄─► │ []T seg  │ ◄─► │ []T seg  │
//	└──────────┘     └──────────┘     └──────────┘
//
// Complexity (n = elements, S = number of segments, fill = 128 default):
//
//	PushHead / PushTail — Insert at either end.				amortized O(1)
//	PopHead / PopTail — Remove and return an end element.			amortized O(1)
//	PeekHead / PeekTail — Return an end element without removing it.	O(1)
//	Len — Number of elements.						O(1)
//	At / Set — Index access (negative index counts from the tail).	O(S + fill)
//	InsertBefore / InsertAfter / Delete — Positional edits.		O(S + fill)
//	DeleteRange / Trim — Inclusive-range removal, Redis LTRIM semantics.	O(S removed + fill)
//	Range / All / Backward — Range-over-func iterators.			O(S + m) for m yielded
//	MoveHeadToTail — RPOPLPUSH/LMOVE rotation between two lists.	amortized O(1)
//
// Index operations are seek-based: they walk whole segments (O(S)) to
// locate the one holding the index, then operate inside it at O(fill) —
// they never hop element-by-element through the list.
//
// Segment behavior: a segment splits in half when an insertion pushes it
// past the fill target, and two adjacent segments merge when their
// combined count falls to half the fill target, so repeated insert/delete
// at one position cannot degenerate into many tiny segments.  No segment
// is ever empty while linked into the list.
//
// Optional compression (Redis list-compress-depth semantics):
// WithCompression installs a byte-level Codec plus segment
// encoders/decoders; segments deeper than depth from BOTH ends are then
// stored compressed and decompressed transparently on access.  The first
// and last depth segments are always kept plain.  Re-establishing the
// compression windows after an operation costs O(S), so compression
// trades CPU for memory on every operation — leave it off (the default)
// for hot lists.
//
// The element type needs no constraints at all: there is no ordering and
// no equality to supply.  The zero value of QuickList is an empty list
// with the default fill target, ready to use; NewQuickList exists to
// install options.
//
// A nil *QuickList behaves as an empty list for every read operation —
// At, PeekHead, PeekTail, Len, the iterators report empty/not-found —
// and for the no-op-capable writes Delete, DeleteRange and Trim (all
// false/0).  PushHead, PushTail and MoveHeadToTail with a nil receiver
// panic with a message naming the method: a nil list cannot store an
// element.  These are the package's only panics.
//
// This implementation is NOT thread safe.  A mutex-guarded version with
// the exact same interface lives alongside it (see quicklist_ts).
package quicklist

import (
	"math"
	"unsafe"
)

// DefaultSegmentFill is the target number of entries per segment when
// WithSegmentFill is not used — the Redis list-max-listpack-size default.
const DefaultSegmentFill = 128

// segment is one packed node of the list.  The live elements are
// data[start : start+n]; the slack before start lets PushHead/PopHead run
// in O(1) and the slack after start+n lets PushTail/PopTail run in O(1).
// When packed is non-nil the segment is stored compressed: data is nil
// and n alone records the element count.
type segment[T any] struct {
	prev, next *segment[T]
	data       []T
	start      int
	n          int
	packed     []byte
}

// QuickList is a generic segmented deque: a doubly-linked list of packed
// []T segments.
//
// The zero value of QuickList is an empty list with the default fill
// target, ready to use.
type QuickList[T any] struct {
	head, tail *segment[T]
	length     int

	fill     int // target entries per segment: 0 = default, -1 = no count cap
	byteCap  int // payload-bytes cap per segment: 0 = no byte cap
	elemSize int // unsafe.Sizeof of one T, set by WithSegmentBytes

	codec Codec                 // nil = no compression
	depth int                   // plain segments kept at each end
	enc   func([]T) []byte      // segment -> bytes, for the codec
	dec   func([]byte, int) []T // bytes -> segment of n elements
}

// Option configures a QuickList at construction.  Options apply in
// order; WithSegmentBytes after WithSegmentFill keeps both caps (a
// segment fills at whichever it hits first), while WithSegmentBytes
// alone replaces the count cap with the byte cap.
type Option[T any] func(*QuickList[T])

// WithSegmentFill sets the target number of entries per segment (the
// split/merge threshold base).  The default is DefaultSegmentFill (128).
func WithSegmentFill[T any](n int) Option[T] {
	return func(q *QuickList[T]) {
		if n < 1 {
			n = 1
		}
		q.fill = n
	}
}

// WithSegmentBytes sets a payload-bytes cap per segment, the size-based
// alternative to WithSegmentFill (the Redis ~8 KiB listpack cap).  The
// byte size of an element is unsafe.Sizeof(T) — for a string that is the
// 16-byte header, not the pointed-at bytes.  Used on its own it replaces
// the count cap; combined with WithSegmentFill both caps apply.
func WithSegmentBytes[T any](n int) Option[T] {
	return func(q *QuickList[T]) {
		if n < 1 {
			n = 1
		}
		q.byteCap = n
		var zero T
		q.elemSize = int(unsafe.Sizeof(zero))
		if q.fill == 0 {
			q.fill = -1 // byte cap alone: no count cap
		}
	}
}

// WithCompression enables Redis list-compress-depth semantics: segments
// deeper than depth from both ends of the list are stored compressed
// with codec and decompressed transparently on access; the first and
// last depth segments are always plain.  depth <= 0 disables compression
// (the Redis depth-0 convention).
//
// Because T is unconstrained the caller must supply the segment wire
// format: enc serializes a segment's elements and dec restores exactly
// n of them.  EncodeStringSegment/DecodeStringSegment cover string
// lists; LZWCodec adapts pluto/lzw as the Codec.
func WithCompression[T any](codec Codec, depth int, enc func([]T) []byte, dec func([]byte, int) []T) Option[T] {
	return func(q *QuickList[T]) {
		q.codec = codec
		q.depth = depth
		q.enc = enc
		q.dec = dec
	}
}

// NewQuickList returns an empty QuickList configured by opts.  Without
// options it behaves exactly like the zero value.
func NewQuickList[T any](opts ...Option[T]) *QuickList[T] {
	q := &QuickList[T]{}
	for _, o := range opts {
		o(q)
	}
	return q
}

// fillTarget returns the entry-count cap per segment.
func (q *QuickList[T]) fillTarget() int {
	switch {
	case q.fill < 0:
		return math.MaxInt
	case q.fill == 0:
		return DefaultSegmentFill
	default:
		return q.fill
	}
}

// segCap returns the effective capacity of one segment in elements: the
// count cap, tightened by the byte cap when one is set (at least 1, so a
// single oversized element still fits its own segment).
func (q *QuickList[T]) segCap() int {
	f := q.fillTarget()
	if q.byteCap > 0 && q.elemSize > 0 {
		b := q.byteCap / q.elemSize
		if b < 1 {
			b = 1
		}
		if b < f {
			f = b
		}
	}
	return f
}

// segFull reports whether s has reached its capacity.
func (q *QuickList[T]) segFull(s *segment[T]) bool {
	return s.n >= q.segCap()
}

// mergeThreshold is the combined element count at which two adjacent
// segments merge — half the capacity, so a merge cannot immediately be
// undone by the next split.
func (q *QuickList[T]) mergeThreshold() int {
	return q.segCap() / 2
}

// materialize decompresses s in place if it is stored compressed.  The
// restored elements are copied into a fresh segCap-sized backing array
// so the segment has the same push slack as a never-compressed one.
func (q *QuickList[T]) materialize(s *segment[T]) {
	if s.packed == nil {
		return
	}
	items := q.dec(q.codec.Decompress(s.packed), s.n)
	c := q.segCap()
	if c < len(items) {
		c = len(items)
	}
	s.data = make([]T, c)
	copy(s.data, items)
	s.start = 0
	s.packed = nil
}

// compressSeg stores s compressed; it is a no-op on an already-packed
// segment.  Callers never pass an empty segment (none exist in the list).
func (q *QuickList[T]) compressSeg(s *segment[T]) {
	if s.packed != nil {
		return
	}
	s.packed = q.codec.Compress(q.enc(s.data[s.start : s.start+s.n]))
	s.data = nil
	s.start = 0
}

// recompress restores the compression windows after an operation: the
// first and last depth segments are materialized, every segment deeper
// than depth from both ends is packed.  It is a no-op when compression
// is disabled.  Complexity is O(S) — the price of the memory saving.
func (q *QuickList[T]) recompress() {
	if q.codec == nil || q.depth <= 0 {
		return
	}
	m := 0
	for s := q.head; s != nil; s = s.next {
		m++
	}
	i := 0
	for s := q.head; s != nil; s = s.next {
		if i < q.depth || i >= m-q.depth {
			q.materialize(s)
		} else {
			q.compressSeg(s)
		}
		i++
	}
}

// unlink splices s out of the list.  s must be empty (or its elements
// already accounted for); the caller adjusts length.
func (q *QuickList[T]) unlink(s *segment[T]) {
	if s.prev != nil {
		s.prev.next = s.next
	} else {
		q.head = s.next
	}
	if s.next != nil {
		s.next.prev = s.prev
	} else {
		q.tail = s.prev
	}
	s.prev, s.next = nil, nil
}

// mergeSegments folds b into a (b must be a.next) and unlinks b.
func (q *QuickList[T]) mergeSegments(a, b *segment[T]) {
	q.materialize(a)
	q.materialize(b)
	cap_ := q.segCap()
	if a.n+b.n > cap_ {
		cap_ = a.n + b.n
	}
	data := make([]T, cap_)
	copy(data, a.data[a.start:a.start+a.n])
	copy(data[a.n:], b.data[b.start:b.start+b.n])
	a.data = data
	a.start = 0
	a.n += b.n
	a.next = b.next
	if b.next != nil {
		b.next.prev = a
	} else {
		q.tail = a
	}
	b.prev, b.next = nil, nil
}

// maybeMerge merges s with its neighbors while any adjacent pair's
// combined count is at or below the merge threshold, preferring the
// previous segment so the merged segment keeps the lower position in
// list order.  The cascade matters after range removals, where one
// merge can drop the combined segment below the threshold against its
// other neighbor too.
func (q *QuickList[T]) maybeMerge(s *segment[T]) {
	for {
		if s.prev != nil && s.prev.n+s.n <= q.mergeThreshold() {
			p := s.prev // mergeSegments unlinks s and clears s.prev
			q.mergeSegments(p, s)
			s = p
			continue
		}
		if s.next != nil && s.n+s.next.n <= q.mergeThreshold() {
			q.mergeSegments(s, s.next)
			continue
		}
		return
	}
}

// splitAt moves elements [mid, s.n) of s into a fresh segment linked
// after s.  Requires 0 < mid < s.n so neither half is empty.
func (q *QuickList[T]) splitAt(s *segment[T], mid int) {
	q.materialize(s)
	ns := &segment[T]{data: make([]T, q.segCap()), n: s.n - mid}
	copy(ns.data, s.data[s.start+mid:s.start+s.n])
	clear(s.data[s.start+mid : s.start+s.n])
	s.n = mid
	ns.prev = s
	ns.next = s.next
	if s.next != nil {
		s.next.prev = ns
	} else {
		q.tail = ns
	}
	s.next = ns
}

// norm converts a Redis-style index (negative counts back from the tail)
// to an absolute index, reporting whether it is in range.
func (q *QuickList[T]) norm(i int) (int, bool) {
	if i < 0 {
		i += q.length
	}
	if i < 0 || i >= q.length {
		return 0, false
	}
	return i, true
}

// normRange converts Redis-style inclusive range bounds to absolute
// clamped bounds, reporting whether the range is non-empty.
func (q *QuickList[T]) normRange(start, stop int) (int, int, bool) {
	n := q.length
	if start < 0 {
		start += n
	}
	if stop < 0 {
		stop += n
	}
	if start < 0 {
		start = 0
	}
	if stop >= n {
		stop = n - 1
	}
	if start > stop || start >= n || stop < 0 {
		return 0, 0, false
	}
	return start, stop, true
}

// locate returns the materialized segment holding absolute index i
// (0 <= i < length) and the element's offset within the segment.  It
// walks whole segments from whichever end is closer — O(S), never
// element-by-element.
func (q *QuickList[T]) locate(i int) (*segment[T], int) {
	if i < q.length/2 {
		s := q.head
		for i >= s.n {
			i -= s.n
			s = s.next
		}
		q.materialize(s)
		return s, i
	}
	s := q.tail
	j := q.length - 1 - i
	for j >= s.n {
		j -= s.n
		s = s.prev
	}
	q.materialize(s)
	return s, s.n - 1 - j
}

// Len returns the number of elements in the list.
// Complexity is O(1).
func (q *QuickList[T]) Len() int {
	if q == nil {
		return 0
	}
	return q.length
}

// PushHead inserts v at the head of the list (Redis LPUSH).  It panics
// on a nil list — one of the package's few panics.
// Complexity is amortized O(1).
func (q *QuickList[T]) PushHead(v T) {
	if q == nil {
		panic("quicklist: PushHead called on a nil QuickList")
	}
	s := q.head
	if s == nil || q.segFull(s) {
		// New head segment, placed at the back of its backing array so
		// the next PushHead calls are plain start-- steps.
		c := q.segCap()
		ns := &segment[T]{data: make([]T, c), start: c - 1, n: 1}
		ns.data[c-1] = v
		ns.next = q.head
		if q.head != nil {
			q.head.prev = ns
		} else {
			q.tail = ns
		}
		q.head = ns
	} else {
		q.materialize(s)
		if s.start > 0 {
			s.start--
			s.data[s.start] = v
			s.n++
		} else {
			copy(s.data[1:s.n+1], s.data[:s.n])
			s.data[0] = v
			s.n++
		}
	}
	q.length++
	q.recompress()
}

// PushTail inserts v at the tail of the list (Redis RPUSH).  It panics
// on a nil list — one of the package's few panics.
// Complexity is amortized O(1).
func (q *QuickList[T]) PushTail(v T) {
	if q == nil {
		panic("quicklist: PushTail called on a nil QuickList")
	}
	s := q.tail
	if s == nil || q.segFull(s) {
		ns := &segment[T]{data: make([]T, q.segCap()), n: 1}
		ns.data[0] = v
		ns.prev = q.tail
		if q.tail != nil {
			q.tail.next = ns
		} else {
			q.head = ns
		}
		q.tail = ns
	} else {
		q.materialize(s)
		if s.start+s.n < len(s.data) {
			s.data[s.start+s.n] = v
			s.n++
		} else {
			copy(s.data, s.data[s.start:s.start+s.n])
			s.start = 0
			s.data[s.n] = v
			s.n++
		}
	}
	q.length++
	q.recompress()
}

// PopHead removes and returns the head element (Redis LPOP), or false
// on an empty list.
// Complexity is amortized O(1).
func (q *QuickList[T]) PopHead() (rv T, ok bool) {
	if q == nil || q.length == 0 {
		return rv, false
	}
	s := q.head
	q.materialize(s)
	rv = s.data[s.start]
	var zero T
	s.data[s.start] = zero
	s.start++
	s.n--
	q.length--
	if s.n == 0 {
		nxt := s.next
		q.unlink(s)
		if nxt != nil {
			q.maybeMerge(nxt)
		}
	} else {
		q.maybeMerge(s)
	}
	q.recompress()
	return rv, true
}

// PopTail removes and returns the tail element (Redis RPOP), or false
// on an empty list.
// Complexity is amortized O(1).
func (q *QuickList[T]) PopTail() (rv T, ok bool) {
	if q == nil || q.length == 0 {
		return rv, false
	}
	s := q.tail
	q.materialize(s)
	rv = s.data[s.start+s.n-1]
	var zero T
	s.data[s.start+s.n-1] = zero
	s.n--
	q.length--
	if s.n == 0 {
		prv := s.prev
		q.unlink(s)
		if prv != nil {
			q.maybeMerge(prv)
		}
	} else {
		q.maybeMerge(s)
	}
	q.recompress()
	return rv, true
}

// PeekHead returns the head element without removing it, or false on an
// empty list.
// Complexity is O(1).
func (q *QuickList[T]) PeekHead() (rv T, ok bool) {
	if q == nil || q.length == 0 {
		return rv, false
	}
	s := q.head
	q.materialize(s)
	return s.data[s.start], true
}

// PeekTail returns the tail element without removing it, or false on an
// empty list.
// Complexity is O(1).
func (q *QuickList[T]) PeekTail() (rv T, ok bool) {
	if q == nil || q.length == 0 {
		return rv, false
	}
	s := q.tail
	q.materialize(s)
	return s.data[s.start+s.n-1], true
}

// At returns the element at index i (negative counts back from the
// tail), or false when i is out of range.
// Complexity is O(S + fill).
func (q *QuickList[T]) At(i int) (rv T, ok bool) {
	if q == nil {
		return rv, false
	}
	i, ok = q.norm(i)
	if !ok {
		return rv, false
	}
	s, off := q.locate(i)
	rv = s.data[s.start+off]
	q.recompress()
	return rv, true
}

// Set replaces the element at index i (negative counts back from the
// tail) and reports whether i was in range.
// Complexity is O(S + fill).
func (q *QuickList[T]) Set(i int, v T) bool {
	if q == nil {
		return false
	}
	i, ok := q.norm(i)
	if !ok {
		return false
	}
	s, off := q.locate(i)
	s.data[s.start+off] = v
	q.recompress()
	return true
}

// insertAt inserts v into the materialized segment s at offset off
// (0 <= off <= s.n), growing the backing array when needed and splitting
// the segment in half when the insertion pushes it past capacity.
func (q *QuickList[T]) insertAt(s *segment[T], off int, v T) {
	if s.start+s.n >= len(s.data) {
		if s.start > 0 {
			copy(s.data, s.data[s.start:s.start+s.n])
			s.start = 0
		} else {
			data := make([]T, len(s.data)+q.segCap())
			copy(data, s.data[:s.n])
			s.data = data
		}
	}
	copy(s.data[s.start+off+1:s.start+s.n+1], s.data[s.start+off:s.start+s.n])
	s.data[s.start+off] = v
	s.n++
	if s.n > q.segCap() {
		q.splitAt(s, s.n/2)
	}
}

// InsertBefore inserts v ahead of the element at index i (negative
// counts back from the tail) and reports whether i was in range.  i == 0
// lands at the head, in the head segment.
// Complexity is O(S + fill).
func (q *QuickList[T]) InsertBefore(i int, v T) bool {
	if q == nil {
		return false
	}
	i, ok := q.norm(i)
	if !ok {
		return false
	}
	if i == 0 {
		q.PushHead(v)
		return true
	}
	s, off := q.locate(i)
	q.insertAt(s, off, v)
	q.length++
	q.recompress()
	return true
}

// InsertAfter inserts v behind the element at index i (negative counts
// back from the tail) and reports whether i was in range.  i == Len()-1
// lands at the tail, in the tail segment.
// Complexity is O(S + fill).
func (q *QuickList[T]) InsertAfter(i int, v T) bool {
	if q == nil {
		return false
	}
	i, ok := q.norm(i)
	if !ok {
		return false
	}
	if i == q.length-1 {
		q.PushTail(v)
		return true
	}
	s, off := q.locate(i)
	q.insertAt(s, off+1, v)
	q.length++
	q.recompress()
	return true
}

// Delete removes the element at index i (negative counts back from the
// tail) and reports whether i was in range.
// Complexity is O(S + fill).
func (q *QuickList[T]) Delete(i int) bool {
	if q == nil {
		return false
	}
	i, ok := q.norm(i)
	if !ok {
		return false
	}
	s, off := q.locate(i)
	copy(s.data[s.start+off:s.start+s.n-1], s.data[s.start+off+1:s.start+s.n])
	var zero T
	s.data[s.start+s.n-1] = zero
	s.n--
	q.length--
	if s.n == 0 {
		prv, nxt := s.prev, s.next
		q.unlink(s)
		if prv != nil {
			q.maybeMerge(prv)
		} else if nxt != nil {
			q.maybeMerge(nxt)
		}
	} else {
		q.maybeMerge(s)
	}
	q.recompress()
	return true
}

// deleteRangeNorm removes the inclusive absolute range [lo, hi] (already
// normalized by normRange) and returns the removal count.  It does not
// recompress — the exported callers do.
func (q *QuickList[T]) deleteRangeNorm(lo, hi int) int {
	count := hi - lo + 1
	s, off := q.locate(lo)
	remaining := count
	for remaining > 0 {
		take := s.n - off
		if take > remaining {
			take = remaining
		}
		if off == 0 && take == s.n {
			// Whole segment inside the range: unlink it outright.
			nxt := s.next
			q.unlink(s)
			s = nxt
		} else {
			copy(s.data[s.start+off:s.start+s.n-take], s.data[s.start+off+take:s.start+s.n])
			clear(s.data[s.start+s.n-take : s.start+s.n])
			s.n -= take
			if take < remaining {
				// Removed through the end of s; continue in the next.
				s = s.next
				off = 0
			}
		}
		remaining -= take
	}
	q.length -= count
	// Rebalance the two boundary segments, if any survive.
	if s != nil {
		q.maybeMerge(s)
		if s.prev != nil {
			q.maybeMerge(s.prev)
		}
	} else if q.tail != nil {
		q.maybeMerge(q.tail)
	}
	return count
}

// DeleteRange removes the inclusive range [start, stop] — Redis index
// semantics: negative bounds count back from the tail and out-of-range
// bounds clamp to the list — and returns the number of elements removed.
// Complexity is O(S removed + fill).
func (q *QuickList[T]) DeleteRange(start, stop int) int {
	if q == nil {
		return 0
	}
	lo, hi, ok := q.normRange(start, stop)
	if !ok {
		return 0
	}
	n := q.deleteRangeNorm(lo, hi)
	q.recompress()
	return n
}

// Trim keeps only the inclusive range [start, stop] (Redis LTRIM:
// negative bounds count back from the tail, out-of-range bounds clamp)
// and drops everything outside it.  An empty or inverted range empties
// the list.
// Complexity is O(S removed + fill).
func (q *QuickList[T]) Trim(start, stop int) {
	if q == nil {
		return
	}
	lo, hi, ok := q.normRange(start, stop)
	if !ok {
		q.head, q.tail, q.length = nil, nil, 0
		return
	}
	if hi < q.length-1 {
		q.deleteRangeNorm(hi+1, q.length-1)
	}
	if lo > 0 {
		q.deleteRangeNorm(0, lo-1)
	}
	q.recompress()
}

// MoveHeadToTail moves the head element of src to the tail of dst
// (Redis RPOPLPUSH / LMOVE src dst LEFT RIGHT) and returns it, or false
// when src is empty.  With src == dst it rotates the list by one.
// Complexity is amortized O(1).
func MoveHeadToTail[T any](src, dst *QuickList[T]) (rv T, ok bool) {
	if src == nil || dst == nil {
		return rv, false
	}
	rv, ok = src.PopHead()
	if !ok {
		return rv, false
	}
	dst.PushTail(rv)
	return rv, true
}
