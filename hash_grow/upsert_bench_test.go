/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package hash_grow

import (
	"fmt"
	"hash/maphash"
	"testing"
)

// Benchmarks for the single-probe compound operations against the
// two-call sequences they replace, plus user-pattern benchmarks that
// mimic how the in-repo callers drive the table today (they are the
// workloads the slot-layout experiment was judged on — see
// docs/packages/hash_grow.md).
//
//	replace    — Search for the old element, then Insert a new one that
//	             does not depend on it (bimap put, hash_set re-insert);
//	             always a hit after warm-up.
//	graph      — generic_graph's Func-config indexedSet: add is
//	             Search-then-Insert-on-miss, remove is Search-then-Delete;
//	             int nodes, caller hash, entries carry an id.
//	bimap      — DeleteKey: Search the forward entry, then Delete it.
//
// Each pair is measured "Pair" (today's two calls) vs the new method.

var benchSinkInt int
var benchSinkBool bool

type benchEntry struct {
	key     string
	counter uint8
	lastMin uint16
}

func newBenchEntryTab() *HashTab[benchEntry] {
	seed := maphash.MakeSeed()
	return NewHashTabFunc(
		func(a, b benchEntry) bool { return a.key == b.key },
		func(e benchEntry) uint64 { return maphash.String(seed, e.key) },
		16, 0,
	)
}

func benchEntryKeys(n int) []string {
	keys := make([]string, n)
	for i := range keys {
		keys[i] = fmt.Sprintf("key:%07d", i*7919)
	}
	return keys
}

// Replace shape: every operation replaces a present entry and wants the
// old one back.

func BenchmarkReplacePairSearchInsert(b *testing.B) {
	keys := benchEntryKeys(4096)
	ht := newBenchEntryTab()
	for _, k := range keys {
		ht.Insert(benchEntry{key: k})
	}
	i := 0
	for b.Loop() {
		k := keys[i&4095]
		i++
		if e, ok := ht.Search(benchEntry{key: k}); ok {
			benchSinkInt += int(e.counter)
		}
		ht.Insert(benchEntry{key: k, counter: uint8(i), lastMin: uint16(i)})
	}
}

func BenchmarkReplaceUpsert(b *testing.B) {
	keys := benchEntryKeys(4096)
	ht := newBenchEntryTab()
	for _, k := range keys {
		ht.Insert(benchEntry{key: k})
	}
	i := 0
	for b.Loop() {
		k := keys[i&4095]
		i++
		old, _ := ht.Upsert(benchEntry{key: k, counter: uint8(i), lastMin: uint16(i)})
		benchSinkInt += int(old.counter)
	}
}

// generic_graph Func-config indexedSet shape.

type benchIndexed struct {
	value  int
	id     int
	active bool
}

func newBenchIndexedTab() *HashTab[benchIndexed] {
	return NewHashTabFunc(
		func(a, b benchIndexed) bool { return a.value == b.value },
		func(v benchIndexed) uint64 { return uint64(v.value) * 0x9E3779B97F4A7C15 },
		16, 0,
	)
}

const benchGraphNodes = 20_000

// add every node twice (the second pass is all hits), then remove them all.
func BenchmarkGraphPairAddRemove(b *testing.B) {
	for b.Loop() {
		ht := newBenchIndexedTab()
		id := 0
		for pass := 0; pass < 2; pass++ {
			for v := range benchGraphNodes {
				if _, found := ht.Search(benchIndexed{value: v}); found {
					continue
				}
				ht.Insert(benchIndexed{value: v, id: id, active: true})
				id++
			}
		}
		for v := range benchGraphNodes {
			if e, found := ht.Search(benchIndexed{value: v}); found {
				benchSinkInt += e.id
				ht.Delete(benchIndexed{value: v})
			}
		}
	}
}

func BenchmarkGraphGetOrInsertRemove(b *testing.B) {
	for b.Loop() {
		ht := newBenchIndexedTab()
		id := 0
		for pass := 0; pass < 2; pass++ {
			for v := range benchGraphNodes {
				if _, found := ht.GetOrInsert(benchIndexed{value: v, id: id, active: true}); !found {
					id++
				}
			}
		}
		for v := range benchGraphNodes {
			if e, found := ht.Remove(benchIndexed{value: v}); found {
				benchSinkInt += e.id
			}
		}
	}
}

// bimap DeleteKey shape: remove-and-return over a refilled table.

func BenchmarkRemovePairSearchDelete(b *testing.B) {
	keys := benchEntryKeys(4096)
	ht := newBenchEntryTab()
	i := 0
	for b.Loop() {
		if ht.IsEmpty() {
			for _, k := range keys {
				ht.Insert(benchEntry{key: k})
			}
		}
		k := keys[i&4095]
		i++
		if e, ok := ht.Search(benchEntry{key: k}); ok {
			benchSinkInt += int(e.counter)
			ht.Delete(benchEntry{key: k})
		}
	}
}

func BenchmarkRemove(b *testing.B) {
	keys := benchEntryKeys(4096)
	ht := newBenchEntryTab()
	i := 0
	for b.Loop() {
		if ht.IsEmpty() {
			for _, k := range keys {
				ht.Insert(benchEntry{key: k})
			}
		}
		k := keys[i&4095]
		i++
		if e, ok := ht.Remove(benchEntry{key: k}); ok {
			benchSinkInt += int(e.counter)
		}
	}
}

// User-pattern benchmarks over the existing API only (no new methods):
// what the in-repo callers cost today, used to judge layout changes.

// BenchmarkUserGraphFuncIndex mimics generic_graph's Func-config node
// index: grow from empty with Search+Insert, look every node up, then
// Search+Delete half of them.
func BenchmarkUserGraphFuncIndex(b *testing.B) {
	for b.Loop() {
		ht := newBenchIndexedTab()
		for v := range benchGraphNodes {
			if _, found := ht.Search(benchIndexed{value: v}); !found {
				ht.Insert(benchIndexed{value: v, id: v, active: true})
			}
		}
		for v := range benchGraphNodes {
			e, _ := ht.Search(benchIndexed{value: v})
			benchSinkInt += e.id
		}
		for v := 0; v < benchGraphNodes; v += 2 {
			if _, found := ht.Search(benchIndexed{value: v}); found {
				ht.Delete(benchIndexed{value: v})
			}
		}
	}
}

// BenchmarkUserMissHeavy probes a 100k-element string-keyed table with
// a 50/50 hit/miss mix — misses walk the probe run to its first empty
// slot, the case wider slots were seen to hurt.
func BenchmarkUserMissHeavy(b *testing.B) {
	const n = 100_000
	keys := benchEntryKeys(2 * n)
	ht := newBenchEntryTab()
	for i := 0; i < len(keys); i += 2 {
		ht.Insert(benchEntry{key: keys[i]})
	}
	i := 0
	for b.Loop() {
		_, ok := ht.Search(benchEntry{key: keys[i%len(keys)]})
		benchSinkBool = ok
		i++
	}
}
