/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package quicklist_ts implements the generic segmented deque of
// github.com/pschlump/pluto/quicklist safe for concurrent use — the
// identical API guarded by one sync.RWMutex, plus the Lock and Unlock
// pair and the Nl-prefixed (no-lock) methods for compound operations.
//
// Concurrency model — which method takes which lock:
//
//	Len, PeekHead, PeekTail — the read lock (true reads; the head and
//	            tail segments are never stored compressed, so peeks
//	            never mutate).
//	PushHead, PushTail, PopHead, PopTail, At, Set, InsertBefore,
//	InsertAfter, Delete, DeleteRange, Trim — the write lock.  At is a
//	            read in intent, but with compression enabled it
//	            materializes the segment it touches, which mutates; a
//	            read lock would race with itself.
//	All / Backward / Range — the write lock held only while the
//	            snapshot is materialized (one O(n) copy); the returned
//	            iterator then walks the snapshot, so it is safe to
//	            mutate the list (even from inside the loop) and never
//	            observes later modifications.  This differs from the
//	            plain package, whose iterators walk the live segments.
//
// The Lock/Unlock pair takes the real write lock for compound
// operations — the Nl* methods run unlocked while it is held.  The
// canonical compound is the atomic LPOS-style scan: Lock, walk with
// NlAt, Unlock.  Calling a regular method while the lock is held
// deadlocks — use the Nl* forms inside.
//
// A nil *QuickList behaves as an empty list for every operation that
// has a sane answer, exactly as in the plain package; PushHead and
// PushTail on a nil list panic with a message naming the method.
//
// Run the tests with -race.
package quicklist_ts

import (
	"iter"
	"sync"
	"unsafe"

	"github.com/pschlump/pluto/quicklist"
)

// QuickList is a thread-safe segmented deque: a quicklist.QuickList
// guarded by a sync.RWMutex.
//
// The zero value is an empty list with the default fill target, ready
// to use; NewQuickList exists to install options.
type QuickList[T any] struct {
	inner quicklist.QuickList[T]
	lock  sync.RWMutex
}

// NewQuickList returns an empty thread-safe QuickList configured by the
// plain package's options (quicklist.WithSegmentFill and friends).
func NewQuickList[T any](opts ...quicklist.Option[T]) *QuickList[T] {
	q := &QuickList[T]{}
	q.inner = *quicklist.NewQuickList(opts...)
	return q
}

// Len returns the number of elements in the list.
// Complexity is O(1).
func (q *QuickList[T]) Len() int {
	if q == nil {
		return 0
	}
	q.lock.RLock()
	defer q.lock.RUnlock()
	return q.inner.Len()
}

// PushHead inserts v at the head of the list (Redis LPUSH).  It panics
// on a nil list.
// Complexity is amortized O(1).
func (q *QuickList[T]) PushHead(v T) {
	if q == nil {
		panic("quicklist_ts: PushHead called on a nil QuickList")
	}
	q.lock.Lock()
	defer q.lock.Unlock()
	q.inner.PushHead(v)
}

// PushTail inserts v at the tail of the list (Redis RPUSH).  It panics
// on a nil list.
// Complexity is amortized O(1).
func (q *QuickList[T]) PushTail(v T) {
	if q == nil {
		panic("quicklist_ts: PushTail called on a nil QuickList")
	}
	q.lock.Lock()
	defer q.lock.Unlock()
	q.inner.PushTail(v)
}

// PopHead removes and returns the head element (Redis LPOP), or false
// on an empty list.
// Complexity is amortized O(1).
func (q *QuickList[T]) PopHead() (T, bool) {
	if q == nil {
		var rv T
		return rv, false
	}
	q.lock.Lock()
	defer q.lock.Unlock()
	return q.inner.PopHead()
}

// PopTail removes and returns the tail element (Redis RPOP), or false
// on an empty list.
// Complexity is amortized O(1).
func (q *QuickList[T]) PopTail() (T, bool) {
	if q == nil {
		var rv T
		return rv, false
	}
	q.lock.Lock()
	defer q.lock.Unlock()
	return q.inner.PopTail()
}

// PeekHead returns the head element without removing it, or false on an
// empty list.
// Complexity is O(1).
func (q *QuickList[T]) PeekHead() (T, bool) {
	if q == nil {
		var rv T
		return rv, false
	}
	q.lock.RLock()
	defer q.lock.RUnlock()
	return q.inner.PeekHead()
}

// PeekTail returns the tail element without removing it, or false on an
// empty list.
// Complexity is O(1).
func (q *QuickList[T]) PeekTail() (T, bool) {
	if q == nil {
		var rv T
		return rv, false
	}
	q.lock.RLock()
	defer q.lock.RUnlock()
	return q.inner.PeekTail()
}

// At returns the element at index i (negative counts back from the
// tail), or false when i is out of range.  It takes the write lock: a
// hit can materialize a compressed segment.
// Complexity is O(S + fill).
func (q *QuickList[T]) At(i int) (T, bool) {
	if q == nil {
		var rv T
		return rv, false
	}
	q.lock.Lock()
	defer q.lock.Unlock()
	return q.inner.At(i)
}

// Set replaces the element at index i (negative counts back from the
// tail) and reports whether i was in range.
// Complexity is O(S + fill).
func (q *QuickList[T]) Set(i int, v T) bool {
	if q == nil {
		return false
	}
	q.lock.Lock()
	defer q.lock.Unlock()
	return q.inner.Set(i, v)
}

// InsertBefore inserts v ahead of the element at index i (negative
// counts back from the tail) and reports whether i was in range.
// Complexity is O(S + fill).
func (q *QuickList[T]) InsertBefore(i int, v T) bool {
	if q == nil {
		return false
	}
	q.lock.Lock()
	defer q.lock.Unlock()
	return q.inner.InsertBefore(i, v)
}

// InsertAfter inserts v behind the element at index i (negative counts
// back from the tail) and reports whether i was in range.
// Complexity is O(S + fill).
func (q *QuickList[T]) InsertAfter(i int, v T) bool {
	if q == nil {
		return false
	}
	q.lock.Lock()
	defer q.lock.Unlock()
	return q.inner.InsertAfter(i, v)
}

// Delete removes the element at index i (negative counts back from the
// tail) and reports whether i was in range.
// Complexity is O(S + fill).
func (q *QuickList[T]) Delete(i int) bool {
	if q == nil {
		return false
	}
	q.lock.Lock()
	defer q.lock.Unlock()
	return q.inner.Delete(i)
}

// DeleteRange removes the inclusive range [start, stop] — Redis index
// semantics: negative bounds count back from the tail and out-of-range
// bounds clamp — and returns the number of elements removed.
// Complexity is O(S removed + fill).
func (q *QuickList[T]) DeleteRange(start, stop int) int {
	if q == nil {
		return 0
	}
	q.lock.Lock()
	defer q.lock.Unlock()
	return q.inner.DeleteRange(start, stop)
}

// Trim keeps only the inclusive range [start, stop] (Redis LTRIM) and
// drops everything outside it.  An empty or inverted range empties the
// list.
// Complexity is O(S removed + fill).
func (q *QuickList[T]) Trim(start, stop int) {
	if q == nil {
		return
	}
	q.lock.Lock()
	defer q.lock.Unlock()
	q.inner.Trim(start, stop)
}

// snapshot returns every element head to tail, taken under the write
// lock (the walk materializes compressed segments).  A nil list yields
// nil.  The caller must NOT hold the lock.
// Complexity is O(n).
func (q *QuickList[T]) snapshot() []T {
	if q == nil {
		return nil
	}
	q.lock.Lock()
	defer q.lock.Unlock()
	items := make([]T, 0, q.inner.Len())
	for _, v := range q.inner.All() {
		items = append(items, v)
	}
	return items
}

// All returns an iterator over a snapshot of the list, head to tail,
// yielding each element with its index.  The snapshot is taken when All
// is called, so the iterator is safe to use concurrently with any list
// operation — including mutating the list from inside the loop.
// Complexity is O(n) for the snapshot.
func (q *QuickList[T]) All() iter.Seq2[int, T] {
	items := q.snapshot()
	return func(yield func(int, T) bool) {
		for i, v := range items {
			if !yield(i, v) {
				return
			}
		}
	}
}

// Backward returns an iterator over a snapshot of the list, tail to
// head, yielding each element with its absolute index (counting down
// from Len()-1).  The same snapshot rules as All apply.
// Complexity is O(n) for the snapshot.
func (q *QuickList[T]) Backward() iter.Seq2[int, T] {
	items := q.snapshot()
	return func(yield func(int, T) bool) {
		for i := len(items) - 1; i >= 0; i-- {
			if !yield(i, items[i]) {
				return
			}
		}
	}
}

// Range returns an iterator over a snapshot of the inclusive index
// range [start, stop] — Redis LRANGE semantics: negative bounds count
// back from the tail and out-of-range bounds clamp — yielding each
// element with its absolute index.  An empty or inverted range yields
// nothing.  The same snapshot rules as All apply.
// Complexity is O(n) for the snapshot.
func (q *QuickList[T]) Range(start, stop int) iter.Seq2[int, T] {
	items := q.snapshot()
	n := len(items)
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
		start, stop = 0, -1
	}
	return func(yield func(int, T) bool) {
		for i := start; i <= stop; i++ {
			if !yield(i, items[i]) {
				return
			}
		}
	}
}

// MoveHeadToTail moves the head element of src to the tail of dst
// (Redis RPOPLPUSH / LMOVE src dst LEFT RIGHT) and returns it, or false
// when src is empty.  With src == dst it rotates the list by one.  The
// two lists are locked in pointer order, so concurrent
// MoveHeadToTail(a, b) and MoveHeadToTail(b, a) cannot deadlock.
// Complexity is amortized O(1).
func MoveHeadToTail[T any](src, dst *QuickList[T]) (rv T, ok bool) {
	if src == nil || dst == nil {
		return rv, false
	}
	if src == dst {
		src.lock.Lock()
		defer src.lock.Unlock()
		rv, ok = src.inner.PopHead()
		if !ok {
			return rv, false
		}
		src.inner.PushTail(rv)
		return rv, true
	}
	first, second := src, dst
	if uintptr(unsafe.Pointer(dst)) < uintptr(unsafe.Pointer(src)) {
		first, second = dst, src
	}
	first.lock.Lock()
	defer first.lock.Unlock()
	second.lock.Lock()
	defer second.lock.Unlock()
	rv, ok = src.inner.PopHead()
	if !ok {
		return rv, false
	}
	dst.inner.PushTail(rv)
	return rv, true
}

// Lock takes the real write lock, for compound operations — the Nl*
// methods below run unlocked while it is held.  Calling a regular
// method while the lock is held deadlocks — use the Nl* forms inside.
// A nil *QuickList no-ops.
func (q *QuickList[T]) Lock() {
	if q == nil {
		return
	}
	q.lock.Lock()
}

// Unlock releases the write lock taken by Lock.  A nil *QuickList
// no-ops.
func (q *QuickList[T]) Unlock() {
	if q == nil {
		return
	}
	q.lock.Unlock()
}

// NlLen is the no-lock Len — call it only while holding Lock.
// Complexity is O(1).
func (q *QuickList[T]) NlLen() int { return q.inner.Len() }

// NlPushHead is the no-lock PushHead — call it only while holding Lock.
// Complexity is amortized O(1).
func (q *QuickList[T]) NlPushHead(v T) { q.inner.PushHead(v) }

// NlPushTail is the no-lock PushTail — call it only while holding Lock.
// Complexity is amortized O(1).
func (q *QuickList[T]) NlPushTail(v T) { q.inner.PushTail(v) }

// NlPopHead is the no-lock PopHead — call it only while holding Lock.
// Complexity is amortized O(1).
func (q *QuickList[T]) NlPopHead() (T, bool) { return q.inner.PopHead() }

// NlPopTail is the no-lock PopTail — call it only while holding Lock.
// Complexity is amortized O(1).
func (q *QuickList[T]) NlPopTail() (T, bool) { return q.inner.PopTail() }

// NlPeekHead is the no-lock PeekHead — call it only while holding Lock.
// Complexity is O(1).
func (q *QuickList[T]) NlPeekHead() (T, bool) { return q.inner.PeekHead() }

// NlPeekTail is the no-lock PeekTail — call it only while holding Lock.
// Complexity is O(1).
func (q *QuickList[T]) NlPeekTail() (T, bool) { return q.inner.PeekTail() }

// NlAt is the no-lock At — call it only while holding Lock.
// Complexity is O(S + fill).
func (q *QuickList[T]) NlAt(i int) (T, bool) { return q.inner.At(i) }

// NlSet is the no-lock Set — call it only while holding Lock.
// Complexity is O(S + fill).
func (q *QuickList[T]) NlSet(i int, v T) bool { return q.inner.Set(i, v) }

// NlInsertBefore is the no-lock InsertBefore — call it only while
// holding Lock.
// Complexity is O(S + fill).
func (q *QuickList[T]) NlInsertBefore(i int, v T) bool { return q.inner.InsertBefore(i, v) }

// NlInsertAfter is the no-lock InsertAfter — call it only while holding
// Lock.
// Complexity is O(S + fill).
func (q *QuickList[T]) NlInsertAfter(i int, v T) bool { return q.inner.InsertAfter(i, v) }

// NlDelete is the no-lock Delete — call it only while holding Lock.
// Complexity is O(S + fill).
func (q *QuickList[T]) NlDelete(i int) bool { return q.inner.Delete(i) }

// NlDeleteRange is the no-lock DeleteRange — call it only while holding
// Lock.
// Complexity is O(S removed + fill).
func (q *QuickList[T]) NlDeleteRange(start, stop int) int {
	return q.inner.DeleteRange(start, stop)
}

// NlTrim is the no-lock Trim — call it only while holding Lock.
// Complexity is O(S removed + fill).
func (q *QuickList[T]) NlTrim(start, stop int) { q.inner.Trim(start, stop) }
