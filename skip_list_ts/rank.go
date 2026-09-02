/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Positional and neighbor queries backed by the span counters carried on the
// forward pointers (see skip_list_ts.go).  The public methods take the read
// lock; the Nl* variants are the lock-free bodies for callers that hold
// Lock() or RLock() themselves and need an atomic compound operation (for
// example rank-then-delete).

package skip_list_ts

// Lock takes the write lock.  It is the real lock, exposed so that callers
// can build atomic compound operations with the Nl* methods; a nil list's
// Lock is a no-op.  Calling a public method while the lock is held
// deadlocks.
func (tt *SkipList[T]) Lock() {
	if tt == nil {
		return
	}
	tt.lock.Lock()
}

// Unlock releases the write lock taken by Lock.  A nil list's Unlock is a
// no-op.
func (tt *SkipList[T]) Unlock() {
	if tt == nil {
		return
	}
	tt.lock.Unlock()
}

// NlLen returns the number of elements in the list without taking the lock;
// the caller must already hold the read or write lock.
// Complexity is O(1).
func (tt *SkipList[T]) NlLen() int {
	if tt == nil {
		return 0
	}
	return tt.length
}

// NlIsEmpty reports whether the list is empty without taking the lock; the
// caller must already hold the read or write lock.
// Complexity is O(1).
func (tt *SkipList[T]) NlIsEmpty() bool {
	return tt == nil || tt.length == 0
}

// NlRank is the lock-free body of Rank; the caller must hold the read or
// write lock.
// Complexity is O(log₂ n) expected.
func (tt *SkipList[T]) NlRank(key T) (rank int, found bool) {
	if tt == nil || tt.isEmpty() {
		return
	}
	cur := tt.head
	for i := tt.level - 1; i >= 0; i-- {
		for cur.forward[i] != nil && tt.cmp(cur.forward[i].data, key) < 0 {
			rank += cur.span[i]
			cur = cur.forward[i]
		}
	}
	cur = cur.forward[0]
	if cur != nil && tt.cmp(cur.data, key) == 0 {
		return rank, true // rank = elements before cur = its 0-based position
	}
	return 0, false
}

// NlAtIndex is the lock-free body of AtIndex; the caller must hold the read
// or write lock.
// Complexity is O(log₂ n) expected.
func (tt *SkipList[T]) NlAtIndex(i int) (rv T, found bool) {
	if tt == nil || i < 0 || i >= tt.length {
		return
	}
	// Walk by spans towards the 1-based rank i+1: advancing is safe while
	// the hop would not overshoot it.
	target := i + 1
	traversed := 0
	cur := tt.head
	for i := tt.level - 1; i >= 0; i-- {
		for cur.forward[i] != nil && traversed+cur.span[i] <= target {
			traversed += cur.span[i]
			cur = cur.forward[i]
		}
	}
	if traversed == target {
		return cur.data, true
	}
	return // unreachable for a list with consistent spans
}

// NlCeil is the lock-free body of Ceil; the caller must hold the read or
// write lock.
// Complexity is O(log₂ n) expected.
func (tt *SkipList[T]) NlCeil(key T) (rv T, found bool) {
	if tt == nil || tt.isEmpty() {
		return
	}
	// Land on the last node strictly less than key; its level-0 successor is
	// the first candidate >= key.
	cur := tt.head
	for i := tt.level - 1; i >= 0; i-- {
		for cur.forward[i] != nil && tt.cmp(cur.forward[i].data, key) < 0 {
			cur = cur.forward[i]
		}
	}
	if next := cur.forward[0]; next != nil {
		return next.data, true
	}
	return
}

// NlFloor is the lock-free body of Floor; the caller must hold the read or
// write lock.
// Complexity is O(log₂ n) expected.
func (tt *SkipList[T]) NlFloor(key T) (rv T, found bool) {
	if tt == nil || tt.isEmpty() {
		return
	}
	// Land on the last node less than or equal to key; unlike Ceil the
	// answer is the node itself, which may be the head sentinel.
	cur := tt.head
	for i := tt.level - 1; i >= 0; i-- {
		for cur.forward[i] != nil && tt.cmp(cur.forward[i].data, key) <= 0 {
			cur = cur.forward[i]
		}
	}
	if cur != tt.head {
		return cur.data, true
	}
	return
}

// nlCountLess returns the number of elements that compare strictly less than
// key, or less than or equal when orEqual is set.  The caller must hold the
// read or write lock.
// Complexity is O(log₂ n) expected.
func (tt *SkipList[T]) nlCountLess(key T, orEqual bool) int {
	count := 0
	cur := tt.head
	for i := tt.level - 1; i >= 0; i-- {
		for cur.forward[i] != nil {
			c := tt.cmp(cur.forward[i].data, key)
			if c > 0 || (c == 0 && !orEqual) {
				break
			}
			count += cur.span[i]
			cur = cur.forward[i]
		}
	}
	return count
}

// NlCountRange is the lock-free body of CountRange; the caller must hold
// the read or write lock.
// Complexity is O(log₂ n) expected.
func (tt *SkipList[T]) NlCountRange(lo, hi T) int {
	if tt == nil || tt.isEmpty() || tt.cmp(lo, hi) > 0 {
		return 0
	}
	return tt.nlCountLess(hi, true) - tt.nlCountLess(lo, false)
}

// Rank returns the 0-based position of key in ascending order — the number
// of elements that sort before it.  found is false when no element compares
// equal to key; the rank of a missing key is not defined and 0 is returned
// with it.  As with Search, `key` only needs the fields that the list's
// comparison function reads.
// Complexity is O(log₂ n) expected.
func (tt *SkipList[T]) Rank(key T) (rank int, found bool) {
	if tt == nil {
		return
	}
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	return tt.NlRank(key)
}

// AtIndex returns the element at 0-based position i in ascending order —
// AtIndex(0) is FindMin, AtIndex(Len()-1) is FindMax.  found is false when
// i is out of range [0, Len()).
// Complexity is O(log₂ n) expected.
func (tt *SkipList[T]) AtIndex(i int) (rv T, found bool) {
	if tt == nil {
		return
	}
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	return tt.NlAtIndex(i)
}

// Ceil returns the smallest element that compares >= key, or found=false
// when key is greater than every element.  As with Search, `key` only needs
// the fields that the comparison function reads.
// Complexity is O(log₂ n) expected.
func (tt *SkipList[T]) Ceil(key T) (rv T, found bool) {
	if tt == nil {
		return
	}
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	return tt.NlCeil(key)
}

// Floor returns the largest element that compares <= key, or found=false
// when key is less than every element.  As with Search, `key` only needs
// the fields that the comparison function reads.
// Complexity is O(log₂ n) expected.
func (tt *SkipList[T]) Floor(key T) (rv T, found bool) {
	if tt == nil {
		return
	}
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	return tt.NlFloor(key)
}

// CountRange returns the number of elements x with lo <= x <= hi.  A range
// with lo > hi contains nothing and returns 0.
// Complexity is O(log₂ n) expected.
func (tt *SkipList[T]) CountRange(lo, hi T) int {
	if tt == nil {
		return 0
	}
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	return tt.NlCountRange(lo, hi)
}
