/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package hash_grow_ts

import "testing"

// Read-modify-write benchmarks: UpsertFunc against the Search+Insert pair
// it replaces, in lfu.Touch's shape (the new counter depends on the old
// one).  "Hit" bumps a present entry of a 4096-entry table; "Miss" inserts
// an absent key and deletes it again (the Delete is identical in both
// forms, so the difference is the read-modify-write itself).

func benchBump(old benchEntry, found bool, k string, i int) benchEntry {
	c := uint8(5)
	if found {
		c = old.counter + 1
	}
	return benchEntry{key: k, counter: c, lastMin: uint16(i)}
}

func BenchmarkRMWHitPairSearchInsert(b *testing.B) {
	keys := benchEntryKeys(4096)
	ht := newBenchEntryTab()
	for _, k := range keys {
		ht.Insert(benchEntry{key: k})
	}
	i := 0
	for b.Loop() {
		k := keys[i&4095]
		i++
		old, found := ht.Search(benchEntry{key: k})
		ht.Insert(benchBump(old, found, k, i))
	}
}

func BenchmarkRMWHitUpsertFunc(b *testing.B) {
	keys := benchEntryKeys(4096)
	ht := newBenchEntryTab()
	for _, k := range keys {
		ht.Insert(benchEntry{key: k})
	}
	i := 0
	for b.Loop() {
		k := keys[i&4095]
		i++
		ht.UpsertFunc(benchEntry{key: k}, func(old benchEntry, found bool) benchEntry {
			return benchBump(old, found, k, i)
		})
	}
}

func BenchmarkRMWMissPairSearchInsert(b *testing.B) {
	keys := benchEntryKeys(4096)
	ht := newBenchEntryTab()
	for _, k := range keys[:2048] {
		ht.Insert(benchEntry{key: k})
	}
	i := 0
	for b.Loop() {
		k := keys[2048+i&2047]
		i++
		old, found := ht.Search(benchEntry{key: k})
		ht.Insert(benchBump(old, found, k, i))
		ht.Delete(benchEntry{key: k})
	}
}

func BenchmarkRMWMissUpsertFunc(b *testing.B) {
	keys := benchEntryKeys(4096)
	ht := newBenchEntryTab()
	for _, k := range keys[:2048] {
		ht.Insert(benchEntry{key: k})
	}
	i := 0
	for b.Loop() {
		k := keys[2048+i&2047]
		i++
		ht.UpsertFunc(benchEntry{key: k}, func(old benchEntry, found bool) benchEntry {
			return benchBump(old, found, k, i)
		})
		ht.Delete(benchEntry{key: k})
	}
}
