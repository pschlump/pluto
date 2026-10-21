/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package multiway_merge merges any number of already-sorted slices
// into one sorted slice — Sedgwick's Multiway.java (algs4 §2.4): the
// head of every input stream sits in an indexed priority queue, the
// minimum head is emitted, and that stream's next element takes its
// place.  With k non-empty streams and n total elements the merge
// costs O(n log₂ k) compares — for k far below n this beats the full
// O(n log₂ n) sort of the concatenation, which is why the pattern
// backs external sorting (merging sorted runs) and log-structured
// storage.
//
// There is no merge object, no constructor and no stored state:
// MergeK orders naturally ordered element types (all integers, floats
// and strings — cmp.Ordered) by the built-in < and > operators, and
// MergeKFunc orders any type by a caller supplied comparison function
// — no boxing, no dynamic dispatch, no type assertions.
//
// The merge is STABLE in the merge_sort sense: elements that compare
// equal keep their relative order from the input, with the streams
// ordered by their position in the call — all equals from streams[0]
// come before those from streams[1], and within one stream the input
// order is kept.  MergeK(a, b) is exactly merge_sort's two-run merge
// generalized to k runs.  Stability rides a private value+stream
// wrapper compared lexicographically (value, then stream id) inside
// the queue: a bare value cannot tell the comparator which stream it
// is from, and the underlying queue does not break ties by index.
//
// Elements are stored and returned by value — the result is a freshly
// allocated []T of copies, and the input slices are never modified.
//
// The package panics in exactly one situation, a programmer error
// with no sane fallback: MergeKFunc with a nil comparison function.
// Nil streams, empty streams and a call with no streams at all are
// tolerated — they contribute nothing, and merging no elements
// returns an empty, non-nil slice.  Each input must already be sorted
// by the same order; the result of merging unsorted input is
// unspecified (but deterministic for a given input).
//
// The functions hold no package state and are safe for concurrent use
// on disjoint slices; no _ts twin exists or is needed (the merge_sort
// and suffix_array precedent).
package multiway_merge

import (
	"cmp"

	"github.com/pschlump/pluto/index_pq"
)

// head is one queue entry: the value at a stream's cursor together
// with the stream's index.  The id exists only to break ties — equal
// values from different streams must come out in stream order for the
// merge to be stable.  Ids are unique, so the (value, id)
// lexicographic comparison is a total order.
type head[T any] struct {
	v  T   // the value at the stream's cursor — what cmpFn orders
	id int // which stream it came from
}

// MergeK merges any number of sorted slices into one sorted slice,
// ordering elements with the built-in < and > operators of any
// naturally ordered type (all integers, floats and strings —
// cmp.Ordered).
//
// The merge is stable: of elements that compare equal, those from
// earlier streams come first, and within one stream the input order
// is kept.  Nil and empty streams contribute nothing; a call with no
// streams returns an empty, non-nil slice.  The inputs are not
// modified.
// Complexity is O(n log₂ k) compares for n total elements over k
// non-empty streams, with O(n + k) workspace.
func MergeK[T cmp.Ordered](streams ...[]T) []T {
	return MergeKFunc(cmp.Compare[T], streams...)
}

// MergeKFunc merges any number of sorted slices into one sorted
// slice, ordering elements with the caller supplied comparison
// function fx — for example a struct ordered by one of its fields.  A
// reversed comparison returns a descending merge.
//
// The merge is stable exactly like MergeK: of elements that compare
// equal, those from earlier streams come first, and within one stream
// the input order is kept.  Nil and empty streams contribute nothing.
// The inputs are not modified.
// It panics if fx is nil.
// Complexity is O(n log₂ k) compares for n total elements over k
// non-empty streams, with O(n + k) workspace.
func MergeKFunc[T any](fx func(a, b T) int, streams ...[]T) []T {
	if fx == nil {
		panic("multiway_merge: MergeKFunc called with a nil comparison function")
	}
	n := 0
	for _, s := range streams {
		n += len(s)
	}
	if n == 0 {
		return []T{}
	}
	out := make([]T, 0, n)

	// One queue entry per non-empty stream: index = stream id, value =
	// the stream's head.  cursors[id] is the next position of that
	// stream to read.
	q := index_pq.NewIndexPQFunc(len(streams), func(a, b head[T]) int {
		if c := fx(a.v, b.v); c != 0 {
			return c
		}
		return a.id - b.id
	})
	cursors := make([]int, len(streams))
	for id, s := range streams {
		if len(s) > 0 {
			q.Insert(id, head[T]{v: s[0], id: id})
			cursors[id] = 1
		}
	}

	// Multiway.merge: emit the minimum head, then either replace it
	// with the next element of the same stream — the decrease-key,
	// one sift where the algs4 client pays a delete plus an insert —
	// or delete the stream when it runs out.
	for {
		id, h, found := q.Peek()
		if !found {
			return out
		}
		out = append(out, h.v)
		s := streams[id]
		if cursors[id] < len(s) {
			q.Change(id, head[T]{v: s[cursors[id]], id: id})
			cursors[id]++
		} else {
			q.Delete(id)
		}
	}
}
