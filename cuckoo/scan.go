/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package cuckoo

// Cursor-based incremental Scan (the Redis SCAN contract) for the cuckoo
// table.  Where Walk/All/Values iterate the whole table in one pass, Scan
// returns a batch at a time against an opaque cursor, so iterating a
// huge table never stalls the caller.
//
// Cursor design — generation-stamped, not Redis's reverse-binary:
// Redis's dictScan enumerates buckets in bit-reversed order, which stays
// meaningful across a rehash because a chained bucket i splits cleanly
// into twin buckets i and i+size.  A cuckoo rebuild gives no such twin
// property: each element re-places into ANY of its four candidate
// positions, so an element can move to an arbitrary slot in either
// direction on a grow AND on a shrink.  No slot-order enumeration
// survives a rebuild here.  Instead the cursor stamps the table
// generation (bumped by every successful rebuild and by Truncate); a
// Scan whose cursor carries a stale generation restarts the slot walk
// at 0 against the current table.  Every element present for the entire
// iteration survives every rebuild, and the post-restart walk
// re-enumerates the whole table, so the full-iteration coverage
// guarantee holds; the restart only re-reports elements, which the
// contract allows.
//
// Encoding: the high 32 bits hold the generation, the low 32 the next
// slot index — table sizes below 2^32 slots, and at most 2^32 rebuilds
// distinguishing two cursors (a wrapped generation aliases and is then
// just another foreign cursor: arbitrary results, no harm).

const (
	// scanPosMask extracts the slot index from the low 32 cursor bits.
	scanPosMask = 0xFFFFFFFF
	// defaultScanCount is the batch size when count <= 0 is passed.
	defaultScanCount = 10
)

// encodeScanCursor packs a table generation and the next slot index
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
//     qualification ties the guarantee to this table's mechanics: an insert's displacement chain can re-place a present element into a lower candidate slot, carrying it backward across the cursor,
//     so under arbitrary insert/delete churn the at-least-once property is
//     best-effort — an element can be missed only through such an internal
//     move, never through a resize (a rebuild restarts the walk
//     and re-enumerates everything).
//   - Elements added or deleted mid-iteration may or may not be returned;
//     duplicates are allowed.
//   - `count` is a hint, not a limit: a batch holds the occupied slots of
//     the next `count` slot positions and may be smaller (even empty);
//     count <= 0 selects a default of 10.
//   - Cursor values are opaque; only 0 is meaningful (start/done).
//   - A cursor not produced by a prior Scan of this table may return
//     arbitrary results, but never corrupts the table (an out-of-range
//     position simply ends the iteration).
//
// Resize-safety: the cursor stamps the table generation; a rebuild —
// grow OR shrink — or a Truncate between calls restarts the walk at
// slot 0; see the file comment for why this table cannot use Redis's
// reverse-binary cursor.
//
// A nil table and an empty table scan as (nil, 0).
// Complexity is O(count) per call; a full iteration of a table that does
// not resize costs O(size) across the calls.
func (tt *HashTab[T]) Scan(cursor uint64, count int) (items []T, next uint64) {
	if tt == nil || tt.length == 0 {
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
		// A stale generation (a rebuild or Truncate landed since the cursor
		// was issued) restarts the walk at slot 0 with the current
		// generation — re-reporting is allowed, losing elements is not.
	}
	if pos < 0 || pos >= tt.size {
		return nil, 0 // foreign or past-the-end cursor: done, no harm done
	}
	end := pos + count
	if end > tt.size {
		end = tt.size
	}
	for i := pos; i < end; i++ {
		if tt.slots[i].used {
			items = append(items, tt.slots[i].data)
		}
	}
	if end >= tt.size {
		return items, 0
	}
	return items, encodeScanCursor(tt.generation, end)
}
