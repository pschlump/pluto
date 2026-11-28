/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package hash_grow

import "fmt"

// This file holds the single-probe compound operations: Upsert,
// GetOrInsert and Remove.  Each replaces a Search followed by an Insert or
// a Delete — two hash computations and two probe walks — with one hash and
// one probe walk.  The hash-0 remap, the growth rule and the stored-hash
// prefilter are exactly Insert's and Delete's.

// writeContract applies the store panic contract for the single-probe
// writers (Upsert, GetOrInsert).  `method` names the public method.
func (tt *HashTab[T]) writeContract(method string) {
	if tt == nil {
		panic(fmt.Sprintf("hash_grow: %s called on a nil table (a nil table cannot store an element; create the table with NewHashTab or NewHashTabFunc)", method))
	}
	if tt.eq == nil || tt.hash == nil {
		panic(fmt.Sprintf("hash_grow: %s called on a table with no equality/hash functions (create the table with NewHashTab or NewHashTabFunc)", method))
	}
}

// probe walks the probe chain of `item` (raw hash rh) once.  It returns
// the slot holding the stored element equal to item with found == true,
// or, when no equal element is stored, the first empty slot of the chain —
// where an insert belongs, since backward-shift deletion leaves no
// tombstones — with found == false.  It returns -1, false only for a
// completely full table with no equal element (reachable only when a
// saturation of 1.0 or more deferred growth).
func (tt *HashTab[T]) probe(rh uint64, item T) (slot int, found bool) {
	h := tt.home(rh)
	for probes := 0; probes < tt.size; probes++ {
		stored := tt.originalHash[h]
		if stored == 0 {
			return h, false
		}
		if stored == rh && tt.eq(tt.buckets[h], item) { // hash check first: equal elements hash equal
			return h, true
		}
		h = tt.nextIndex(h)
	}
	return -1, false
}

// placeNew stores item (raw hash rh, known to be absent) in the empty slot
// `slot` found by probe — or, when slot is -1 (a completely full table),
// grows first and re-probes — then applies the usual load-factor growth.
func (tt *HashTab[T]) placeNew(slot int, rh uint64, item T) {
	if slot < 0 {
		// Growth halves the load factor; the item is still absent, so the
		// re-probe ends at an empty slot.
		tt.grow()
		slot, _ = tt.probe(rh, item)
	}
	tt.buckets[slot] = item
	tt.originalHash[slot] = rh
	tt.length++
	if float64(tt.length)/float64(tt.size) > tt.saturationThreshold {
		tt.grow()
	}
}

// Upsert inserts item, or replaces the stored element equal to it.  When
// an element was replaced it is returned as old with replaced == true;
// otherwise old is the zero value of T and replaced is false.  It is the
// one-probe form of Search followed by Insert: the element is hashed once
// and its probe chain is walked once (plus any growth, exactly as Insert
// grows).  The replaced flag is the negation of Insert's result.
//
// Upsert panics on a nil table or a zero-value table (no equality/hash
// functions), like Insert; the message names the constructors.
// Complexity is O(1) average, O(n) worst case; growth is amortized O(1).
func (tt *HashTab[T]) Upsert(item T) (old T, replaced bool) {
	tt.writeContract("Upsert")
	rh := tt.hashOf(item)
	slot, found := tt.probe(rh, item)
	if found {
		old = tt.buckets[slot]
		tt.buckets[slot] = item
		return old, true
	}
	tt.placeNew(slot, rh, item)
	return old, false
}

// GetOrInsert returns the stored element equal to item with found ==
// true, leaving the table unchanged; when no equal element is stored it
// inserts item and returns item with found == false.  It is the one-probe
// form of Search followed by an Insert on a miss: the element is hashed
// once and its probe chain is walked once (plus any growth).  Unlike
// Upsert and Insert, a present element is never replaced.
//
// GetOrInsert panics on a nil table or a zero-value table (no
// equality/hash functions), like Insert, even when the element would have
// been found; the message names the constructors.
// Complexity is O(1) average, O(n) worst case; growth is amortized O(1).
func (tt *HashTab[T]) GetOrInsert(item T) (existing T, found bool) {
	tt.writeContract("GetOrInsert")
	rh := tt.hashOf(item)
	slot, found := tt.probe(rh, item)
	if found {
		return tt.buckets[slot], true
	}
	tt.placeNew(slot, rh, item)
	return item, false
}

// Remove deletes the stored element equal to `find` and returns it with
// found == true; when no equal element is stored it returns the zero value
// of T and false.  It is the one-probe form of Search followed by Delete:
// one hash, one probe walk, then the same backward-shift repair Delete
// performs.  A nil table and the zero value report not-found.
// Complexity is O(1) average, O(n) worst case.
func (tt *HashTab[T]) Remove(find T) (removed T, found bool) {
	if tt == nil || tt.nlIsEmpty() || tt.eq == nil {
		return removed, false
	}
	rh := tt.hashOf(find)
	h := tt.home(rh)
	for probes := 0; probes < tt.size; probes++ {
		stored := tt.originalHash[h]
		if stored == 0 {
			return removed, false
		}
		if stored == rh && tt.eq(tt.buckets[h], find) {
			removed = tt.buckets[h]
			tt.deleteAt(h)
			return removed, true
		}
		h = tt.nextIndex(h)
	}
	return removed, false // full cycle: not present
}
