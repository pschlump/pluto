/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package sharded_hash_ts

import "math/bits"

// defaultScanCount is the number of elements Scan collects per call when
// count <= 0 — the same default hint as Redis SCAN's COUNT.
const defaultScanCount = 10

// Scan returns up to `count` elements plus the cursor to continue with (0
// means the iteration is complete).  A full iteration — from cursor 0 until
// Scan returns cursor 0 — has the Redis SCAN guarantee:
//
// Every element present in the table for the entire duration of the scan is
// returned at least once.  Elements added or removed while the scan is in
// progress may or may not be returned.  Elements are never returned twice
// within one stripe unless the stripe's table doubled mid-scan; an element
// that survives a doubling may legitimately be returned again.
//
// The cursor packs the stripe index in its high bits and a reverse-binary
// bucket cursor for that stripe in its low slotBits bits (see the constants
// at the top of sharded_hash.go).  Stripes are visited in index order and an
// element never migrates stripes, so each stripe's scan is independent.
//
// Within a stripe the bucket cursor counts in reverse-binary order (0, 2, 1,
// 3, 0, 4, 2, 6, 1, 5, 3, 7, ... for sizes 2, 4) — the iteration order of
// Redis's dictScan (note/redis/src/dict.c).  Reverse-binary is what makes
// the cursor rehash-safe: when a stripe's table doubles, every old bucket v
// splits into exactly the two buckets v and v+oldSize (decided by one hash
// bit — see stripeTab.grow), and the reverse-binary successors of a cursor
// at the larger size are precisely the not-yet-visited buckets together with
// the upper halves of the already-visited ones.  A mid-scan doubling
// therefore never strands an unvisited bucket: already-scanned buckets may
// be re-emitted (harmless duplicates), never-visited ones are all still
// coming.  Insert and delete never move an existing node to another bucket,
// so nothing but a doubling can re-place an element mid-scan.
//
// Per call, Scan holds at most one stripe's read lock and never holds any
// lock across a call boundary.  Whole buckets are emitted (count is a hint,
// as in Redis): a call may return somewhat more than count elements, never
// fewer than one bucket's worth while work remains.  A Truncate concurrent
// with a scan simply empties stripes; the cursor stays valid and the
// guarantee above covers only what remains.
//
// A cursor value that is not 0 and not one Scan previously returned (e.g. a
// corrupted value naming a stripe past the end) is treated as 0 — the scan
// restarts — rather than panicking or skipping.
// Complexity is O(count) amortized per call; a full iteration is O(n).
func (tt *ShardedHash[T]) Scan(cursor uint64, count int) (items []T, next uint64) {
	if tt == nil || len(tt.stripes) == 0 {
		return nil, 0
	}
	if count <= 0 {
		count = defaultScanCount
	}
	prealloc := count
	if prealloc > 4096 { // never pre-allocate gigabytes for a huge count hint
		prealloc = 4096
	}
	items = make([]T, 0, prealloc)

	stripeIdx := int(cursor >> slotBits)
	v := cursor & slotMask
	if stripeIdx >= len(tt.stripes) {
		stripeIdx, v = 0, 0 // an invalid cursor restarts the scan
	}

	for stripeIdx < len(tt.stripes) && len(items) < count {
		s := tt.stripes[stripeIdx]
		s.lock.RLock()
		wrapped := false
		for {
			heads := s.tab.heads
			m := uint64(len(heads) - 1)
			for n := heads[v&m]; n != nil; n = n.next { // emit the whole bucket
				items = append(items, n.data)
			}
			v = revBinNext(v, m) // next bucket in reverse-binary order
			if v == 0 {          // wrapped: this stripe is exhausted
				wrapped = true
				break
			}
			if len(items) >= count {
				break
			}
		}
		s.lock.RUnlock()

		if wrapped {
			stripeIdx++ // continue collecting into the next stripe in this call
			v = 0
		}
	}

	if stripeIdx >= len(tt.stripes) {
		return items, 0 // every stripe exhausted: the iteration is complete
	}
	return items, uint64(stripeIdx)<<slotBits | v
}

// revBinNext advances the reverse-binary bucket cursor v over a table with
// bucket mask m, the increment Redis's dictScan uses: fill every bit above
// the mask with ones, reverse the word, add one, reverse back.  The result
// is the next smaller value in the "bits reversed" ordering and is always
// <= m; it returns to 0 after visiting every bucket exactly once (0 -> 2 ->
// 1 -> 3 -> 0 for m = 3, the size-4 order).
func revBinNext(v, m uint64) uint64 {
	v |= ^m
	v = bits.Reverse64(v)
	v++
	return bits.Reverse64(v) & m
}
