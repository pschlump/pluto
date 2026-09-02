/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Bulk removal over a span of elements, backed by the span counters carried
// on the forward pointers (see skip_list_ts.go).  The public methods take
// the write lock; the Nl* variants are the lock-free bodies for callers
// that hold Lock() themselves and need an atomic compound operation (for
// example rank-then-delete).

package skip_list_ts

// spliceOut unlinks the run of k consecutive level-0 nodes that ends at
// `last`, given update[i] = the predecessor of the run's first node at each
// level (a findPath result).  On a level whose forward pointer points into
// the run the predecessor is re-linked to the first survivor past the run
// and its span absorbs the spans of every run node on that level — minus all
// k removed nodes; on every other level the run lies inside one span, which
// merely loses the k skipped nodes.  The caller must hold the write lock.
func (tt *SkipList[T]) spliceOut(update []*SkipListNode[T], last *SkipListNode[T], k int) {
	for i := 0; i < tt.level; i++ {
		fwd := update[i].forward[i]
		if fwd == nil || tt.cmp(fwd.data, last.data) > 0 {
			// The run sits strictly inside this level's span.
			update[i].span[i] -= k
			continue
		}
		// Walk this level's chain across the run: the last run node on this
		// level is the one whose level-i successor is past the run.
		sum := 0
		x := fwd
		for {
			sum += x.span[i]
			if x.forward[i] == nil || tt.cmp(x.forward[i].data, last.data) > 0 {
				break
			}
			x = x.forward[i]
		}
		update[i].span[i] += sum - k
		update[i].forward[i] = x.forward[i]
	}
	// Drop levels that are no longer in use.
	for tt.level > 0 && tt.head.forward[tt.level-1] == nil {
		tt.level--
	}
	tt.length -= k
}

// NlDeleteRange is the lock-free body of DeleteRange; the caller must hold
// the write lock.  It removes every element x with lo <= x <= hi and
// returns how many were removed.  A range with lo > hi contains nothing and
// 0 is returned.  As with Search, `lo` and `hi` only need the fields that
// the list's comparison function reads.
// Complexity is O(log₂ n + m) where m is the number of elements removed.
func (tt *SkipList[T]) NlDeleteRange(lo, hi T) int {
	if tt == nil || tt.isEmpty() || tt.cmp(lo, hi) > 0 {
		return 0
	}
	tt.ensureHead()

	// update[i] is the last node < lo at each level, so update[0].forward[0]
	// is the first element of the range (or a node beyond it).
	update := tt.findPath(lo)
	first := update[0].forward[0]
	if first == nil || tt.cmp(first.data, hi) > 0 {
		return 0
	}

	// Walk the level-0 chain to the last element of the range.
	last := first
	k := 1
	for last.forward[0] != nil && tt.cmp(last.forward[0].data, hi) <= 0 {
		last = last.forward[0]
		k++
	}

	tt.spliceOut(update, last, k)
	return k
}

// NlDeleteByRank is the lock-free body of DeleteByRank; the caller must
// hold the write lock.  It removes the elements at 0-based ranks [start,
// stop] inclusive and returns how many were removed.  start > stop, a
// negative start or a start beyond the last element remove nothing; stop is
// clamped to Len()-1.
// Complexity is O(log₂ n + m) where m is the number of elements removed.
func (tt *SkipList[T]) NlDeleteByRank(start, stop int) int {
	if tt == nil || tt.isEmpty() || start < 0 || start > stop || start >= tt.length {
		return 0
	}
	if stop >= tt.length {
		stop = tt.length - 1
	}

	// Descend by spans to the node just before rank start; update[i] is its
	// predecessor at each level.  Advancing is safe while the hop would not
	// land beyond 0-based rank start-1.
	update := make([]*SkipListNode[T], maxLevel)
	for i := range update {
		update[i] = tt.head
	}
	traversed := 0
	cur := tt.head
	for i := tt.level - 1; i >= 0; i-- {
		for cur.forward[i] != nil && traversed+cur.span[i] <= start {
			traversed += cur.span[i]
			cur = cur.forward[i]
		}
		update[i] = cur
	}

	// cur.forward[0] is the element at rank start; walk to rank stop.
	first := cur.forward[0]
	if first == nil {
		return 0 // unreachable for consistent spans
	}
	k := stop - start + 1
	last := first
	for range k - 1 {
		last = last.forward[0]
	}

	tt.spliceOut(update, last, k)
	return k
}

// DeleteRange removes every element x with lo <= x <= hi and returns how
// many were removed.  A range with lo > hi contains nothing and 0 is
// returned.  As with Search, `lo` and `hi` only need the fields that the
// list's comparison function reads.
// Complexity is O(log₂ n + m) where m is the number of elements removed.
func (tt *SkipList[T]) DeleteRange(lo, hi T) int {
	if tt == nil {
		return 0
	}
	tt.lock.Lock()
	defer tt.lock.Unlock()
	return tt.NlDeleteRange(lo, hi)
}

// DeleteByRank removes the elements at 0-based ranks [start, stop]
// inclusive and returns how many were removed.  start > stop, a negative
// start or a start beyond the last element remove nothing; stop is clamped
// to Len()-1.
// Complexity is O(log₂ n + m) where m is the number of elements removed.
func (tt *SkipList[T]) DeleteByRank(start, stop int) int {
	if tt == nil {
		return 0
	}
	tt.lock.Lock()
	defer tt.lock.Unlock()
	return tt.NlDeleteByRank(start, stop)
}
