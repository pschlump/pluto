/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package quicklist

import "iter"

// All returns an iterator over every element, head to tail, yielding
// each element with its absolute index.  It walks the live list:
// mutating the list from inside the loop is at the caller's risk, as
// with the other pluto packages.  Segments stored compressed are
// materialized as the walk reaches them and re-packed when the iterator
// ends (including on an early break).
// Complexity is O(S + n).
func (q *QuickList[T]) All() iter.Seq2[int, T] {
	return func(yield func(int, T) bool) {
		if q == nil {
			return
		}
		defer q.recompress()
		idx := 0
		for s := q.head; s != nil; s = s.next {
			q.materialize(s)
			for k := 0; k < s.n; k++ {
				if !yield(idx, s.data[s.start+k]) {
					return
				}
				idx++
			}
		}
	}
}

// Backward returns an iterator over every element, tail to head,
// yielding each element with its absolute index (counting down from
// Len()-1).  The same live-walk and compression rules as All apply.
// Complexity is O(S + n).
func (q *QuickList[T]) Backward() iter.Seq2[int, T] {
	return func(yield func(int, T) bool) {
		if q == nil {
			return
		}
		defer q.recompress()
		idx := q.length - 1
		for s := q.tail; s != nil; s = s.prev {
			q.materialize(s)
			for k := s.n - 1; k >= 0; k-- {
				if !yield(idx, s.data[s.start+k]) {
					return
				}
				idx--
			}
		}
	}
}

// Range returns an iterator over the inclusive index range [start, stop]
// — Redis LRANGE semantics: negative bounds count back from the tail and
// out-of-range bounds clamp to the list — yielding each element with its
// absolute index.  An empty or inverted range yields nothing.  The walk
// seeks by segments to start, then steps element-by-element; the same
// live-walk and compression rules as All apply.
// Complexity is O(S + m) for m yielded elements.
func (q *QuickList[T]) Range(start, stop int) iter.Seq2[int, T] {
	return func(yield func(int, T) bool) {
		if q == nil {
			return
		}
		lo, hi, ok := q.normRange(start, stop)
		if !ok {
			return
		}
		defer q.recompress()
		s, off := q.locate(lo)
		idx := lo
		for s != nil && idx <= hi {
			for off < s.n && idx <= hi {
				if !yield(idx, s.data[s.start+off]) {
					return
				}
				off++
				idx++
			}
			s = s.next
			if s != nil {
				q.materialize(s)
			}
			off = 0
		}
	}
}
