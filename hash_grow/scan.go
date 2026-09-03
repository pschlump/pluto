/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package hash_grow

// Cursor-based incremental Scan (the Redis SCAN contract) for the hash
// table.  Where Walk/All/Values iterate the whole table in one pass, Scan
// returns a batch at a time against an opaque cursor, so iterating a
// huge table never stalls the caller.
//
// Cursor design — generation-stamped, not Redis's reverse-binary:
// Redis's dictScan enumerates buckets in bit-reversed order, which stays
// meaningful across a rehash because a chained bucket i splits cleanly
// into twin buckets i and i+size.  This table is open addressing: on a
// rebuild an element's new slot is NOT constrained to its old twin —
// probe chains move elements backward as well as forward (an element at
// slot 5 whose home is 3 can land at slot 3 in the doubled table).  So
// no bucket-order enumeration survives a resize here.  Instead the
// cursor stamps the table generation (bumped by every growth and by
// Truncate); a Scan whose cursor carries a stale generation restarts
// the bucket walk at 0 against the current table.  Every element present
// for the entire iteration survives every resize, and the post-restart
// walk re-enumerates the whole table, so the full-iteration coverage
// guarantee holds; the restart only re-reports elements, which the
// contract allows.
//
// Encoding: the high 32 bits hold the generation, the low 32 the next
// bucket index — table sizes below 2^32 buckets, and at most 2^32
// resizes distinguishing two cursors (a wrapped generation aliases and
// is then just another foreign cursor: arbitrary results, no harm).

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
//     qualification ties the guarantee to this table's mechanics: a
//     delete's backward-shift chain repair can move a present element
//     backward across the cursor, so under arbitrary insert/delete churn
//     the at-least-once property is best-effort — an element can be
//     missed only through such an internal move, never through a resize
//     (a growth restarts the walk and re-enumerates everything).
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
// Resize-safety: the cursor stamps the table generation; a growth (or a
// Truncate) between calls restarts the walk at bucket 0 — see the file
// comment for why this table cannot use Redis's reverse-binary cursor.
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
		// A stale generation (a resize or Truncate landed since the cursor
		// was issued) restarts the walk at bucket 0 with the current
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
		if tt.originalHash[i] != 0 {
			items = append(items, tt.buckets[i])
		}
	}
	if end >= tt.size {
		return items, 0
	}
	return items, encodeScanCursor(tt.generation, end)
}
