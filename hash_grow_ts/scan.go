/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package hash_grow_ts

// Cursor-based incremental Scan (the Redis SCAN contract) — the
// thread-safe twin of hash_grow's Scan; see hash_grow/scan.go for the
// full contract and why the cursor is generation-stamped rather than
// Redis's reverse-binary (open addressing gives no clean twin-bucket
// split on a rebuild).
//
// Concurrency: no lock is held across Scan calls — each call takes the
// read lock, copies out its batch, and releases it, so a concurrent
// growth lands strictly between batches and merely restarts the walk
// (duplicates allowed by the contract; nothing is lost, nothing
// deadlocks).

const (
	// scanPosMask extracts the bucket index from the low 32 cursor bits.
	scanPosMask = 0xFFFFFFFF
	// defaultScanCount is the batch size when count <= 0 is passed.
	defaultScanCount = 10
)

// encodeScanCursor packs a table generation and the next bucket index
// into an opaque cursor.
func encodeScanCursor(generation uint32, pos int) uint64 {
	return (uint64(generation) << 32) | uint64(uint32(pos))
}

// decodeScanCursor unpacks a cursor produced by encodeScanCursor.
func decodeScanCursor(cursor uint64) (generation uint32, pos int) {
	return uint32(cursor >> 32), int(cursor & scanPosMask)
}

// Scan returns up to `count` elements starting at `cursor`, and the next
// cursor; next == 0 means the iteration is complete.  Start a full
// iteration with cursor == 0.  This is the Redis SCAN contract:
//
//   - A full iteration (0 → … → 0) returns every element that was present
//     in the table for the ENTIRE iteration, at least once.  One
//     qualification ties the guarantee to this table's mechanics: a delete's backward-shift chain repair can move a present element backward across the cursor,
//     so under arbitrary insert/delete churn the at-least-once property is
//     best-effort — an element can be missed only through such an internal
//     move, never through a resize (a growth restarts the walk
//     and re-enumerates everything).
//   - Elements added or deleted mid-iteration may or may not be returned;
//     duplicates are allowed.
//   - `count` is a hint, not a limit: a batch holds the occupied slots of
//     the next `count` bucket positions and may be smaller (even empty);
//     count <= 0 selects a default of 10.
//   - Cursor values are opaque; only 0 is meaningful (start/done).
//   - A cursor not produced by a prior Scan of this table may return
//     arbitrary results, but never corrupts the table (an out-of-range
//     position simply ends the iteration).
//
// Resize-safety: the cursor stamps the table generation (bumped by every
// growth and by Truncate); a stale generation restarts the walk at
// bucket 0 against the current table — re-reporting is allowed, losing
// elements is not.
//
// A nil table and an empty table scan as (nil, 0).
// Complexity is O(count) per call under the read lock; a full iteration
// of a table that does not resize costs O(size) across the calls.
func (tt *HashTab[T]) Scan(cursor uint64, count int) (items []T, next uint64) {
	if tt == nil {
		return nil, 0
	}
	tt.lock.RLock()
	defer tt.lock.RUnlock()
	if tt.length == 0 {
		return nil, 0
	}
	if count <= 0 {
		count = defaultScanCount
	}
	pos := 0
	if cursor != 0 {
		gen, p := decodeScanCursor(cursor)
		if gen == tt.generation {
			pos = p
		}
	}
	if pos < 0 || pos >= tt.size {
		return nil, 0 // foreign or past-the-end cursor: done, no harm done
	}
	end := pos + count
	if end > tt.size {
		end = tt.size
	}
	for i := pos; i < end; i++ {
		if tt.originalHash[i] != 0 {
			items = append(items, tt.buckets[i])
		}
	}
	if end >= tt.size {
		return items, 0
	}
	return items, encodeScanCursor(tt.generation, end)
}
