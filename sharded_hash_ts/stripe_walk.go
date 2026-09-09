/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package sharded_hash_ts

// StripeWalk calls `fx` for each element in stripe `i` only, in bucket
// order within the stripe, until all of the stripe's elements have been
// visited or `fx` returns false.  It returns true if the walk ran to
// completion.  The position counter starts at 0 for the stripe (unlike
// Walk, whose counter runs across the whole table).
//
// Only stripe `i`'s read lock is held, and only while that stripe is
// walked — every other stripe stays fully concurrent.  fx must not call
// methods on the same table: a write to stripe `i` deadlocks (Walk's
// no-reentrancy rule, narrowed to the one stripe; use Scan or a snapshot
// iterator when the loop body needs to touch the table).  This is the
// per-shard snapshot primitive: a caller that owns stripe `i` (Ultima's
// shard i) serializes just its own stripe instead of walking the whole
// table.
//
// A nil table visits nothing and returns true (the empty-table
// convention, as in Walk).  It panics if `i` is out of range (a
// programmer error: the stripe count is fixed at construction and
// StripeCount reports it — the SampleStripe convention).
// Complexity is O(n_stripe).
func (tt *ShardedHash[T]) StripeWalk(i int, fx ApplyFunction[T]) (b bool) {
	b = true
	if tt == nil {
		return
	}
	if i < 0 || i >= len(tt.stripes) {
		panic("sharded_hash_ts: StripeWalk called with an out-of-range stripe index")
	}
	s := tt.stripes[i]
	s.lock.RLock()
	defer s.lock.RUnlock()
	pos := 0
	for _, head := range s.tab.heads {
		for n := head; n != nil; n = n.next {
			if !fx(pos, n.data) {
				return false
			}
			pos++
		}
	}
	return
}
