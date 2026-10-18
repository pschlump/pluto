/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package tree_set

import (
	"fmt"
	"math/rand"
	"strconv"
	"testing"

	"github.com/pschlump/pluto/hash_set"
)

// The one honest comparative benchmark the plan asks for: the sorted
// TreeSet against Go's native map[T]bool set-of-membership idiom, at
// 1,000 and 100,000 string elements.  The workloads are the set's
// staples — insert everything from a shuffled input, look up a hit/miss
// mix, delete half the elements — and the two contenders do identical
// work on identical data.
//
// The constants are what they are, and that is the point of recording
// them: an O(log₂ n) comparison-ordered set loses to a hash table on
// raw lookup speed, and it should — TreeSet is bought for its ORDER:
// ascending Slice/String, Floor/Ceil, Range and rank riders that a hash
// set cannot offer at any price.  The hash_set arm (the phase-6 facade
// over hash_grow, the same workloads on the same keys) completes the
// plan §8 three-way comparison: engine vs engine vs the runtime map.
//
// Fixed seed: benchmark inputs must be reproducible.

func benchKeys(n int) []string {
	keys := make([]string, n)
	for i := range keys {
		keys[i] = "key-" + strconv.Itoa(i)
	}
	rng := rand.New(rand.NewSource(42))
	rng.Shuffle(len(keys), func(i, j int) { keys[i], keys[j] = keys[j], keys[i] })
	return keys
}

func benchmarkTreeSetInsert(b *testing.B, n int) {
	keys := benchKeys(n)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		set := NewTreeSet[string]()
		set.InsertSlice(keys)
	}
}

func benchmarkMapInsert(b *testing.B, n int) {
	keys := benchKeys(n)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m := make(map[string]bool, n)
		for _, k := range keys {
			m[k] = true
		}
	}
}

func benchmarkTreeSetLookup(b *testing.B, n int) {
	keys := benchKeys(n)
	set := NewTreeSet[string]()
	set.InsertSlice(keys)
	// Half the probes hit, half miss by a distinct prefix.
	probes := make([]string, 0, n)
	for i, k := range keys {
		if i%2 == 0 {
			probes = append(probes, k)
		} else {
			probes = append(probes, "miss-"+k)
		}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hits := 0
		for _, k := range probes {
			if set.Contains(k) {
				hits++
			}
		}
		if hits != n/2 {
			b.Fatalf("hits = %d, want %d", hits, n/2)
		}
	}
}

func benchmarkMapLookup(b *testing.B, n int) {
	keys := benchKeys(n)
	m := make(map[string]bool, n)
	for _, k := range keys {
		m[k] = true
	}
	probes := make([]string, 0, n)
	for i, k := range keys {
		if i%2 == 0 {
			probes = append(probes, k)
		} else {
			probes = append(probes, "miss-"+k)
		}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hits := 0
		for _, k := range probes {
			if m[k] {
				hits++
			}
		}
		if hits != n/2 {
			b.Fatalf("hits = %d, want %d", hits, n/2)
		}
	}
}

func benchmarkTreeSetDelete(b *testing.B, n int) {
	keys := benchKeys(n)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		set := NewTreeSet[string]()
		set.InsertSlice(keys)
		b.StartTimer()
		for _, k := range keys[:n/2] {
			set.Delete(k)
		}
		if set.Len() != n-n/2 {
			b.Fatalf("Len = %d, want %d", set.Len(), n-n/2)
		}
	}
}

func benchmarkMapDelete(b *testing.B, n int) {
	keys := benchKeys(n)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		m := make(map[string]bool, n)
		for _, k := range keys {
			m[k] = true
		}
		b.StartTimer()
		for _, k := range keys[:n/2] {
			delete(m, k)
		}
		if len(m) != n-n/2 {
			b.Fatalf("len = %d, want %d", len(m), n-n/2)
		}
	}
}

// The hash_set arms — the identical workloads on the phase-6 facade
// over hash_grow's auto-growing open-addressed table, so the three-way
// table (tree engine vs hash engine vs runtime map) reads off one run.

func benchmarkHashSetInsert(b *testing.B, n int) {
	keys := benchKeys(n)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		set := hash_set.NewHashSet[string]()
		set.InsertSlice(keys)
	}
}

func benchmarkHashSetLookup(b *testing.B, n int) {
	keys := benchKeys(n)
	set := hash_set.NewHashSet[string]()
	set.InsertSlice(keys)
	probes := make([]string, 0, n)
	for i, k := range keys {
		if i%2 == 0 {
			probes = append(probes, k)
		} else {
			probes = append(probes, "miss-"+k)
		}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hits := 0
		for _, k := range probes {
			if set.Contains(k) {
				hits++
			}
		}
		if hits != n/2 {
			b.Fatalf("hits = %d, want %d", hits, n/2)
		}
	}
}

func benchmarkHashSetDelete(b *testing.B, n int) {
	keys := benchKeys(n)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		set := hash_set.NewHashSet[string]()
		set.InsertSlice(keys)
		b.StartTimer()
		for _, k := range keys[:n/2] {
			set.Delete(k)
		}
		if set.Len() != n-n/2 {
			b.Fatalf("Len = %d, want %d", set.Len(), n-n/2)
		}
	}
}

func BenchmarkTreeSet_vs_Map(b *testing.B) {
	for _, n := range []int{1_000, 100_000} {
		b.Run(fmt.Sprintf("n=%d/TreeSet/insert", n), func(b *testing.B) { benchmarkTreeSetInsert(b, n) })
		b.Run(fmt.Sprintf("n=%d/HashSet/insert", n), func(b *testing.B) { benchmarkHashSetInsert(b, n) })
		b.Run(fmt.Sprintf("n=%d/map/insert", n), func(b *testing.B) { benchmarkMapInsert(b, n) })
		b.Run(fmt.Sprintf("n=%d/TreeSet/lookup", n), func(b *testing.B) { benchmarkTreeSetLookup(b, n) })
		b.Run(fmt.Sprintf("n=%d/HashSet/lookup", n), func(b *testing.B) { benchmarkHashSetLookup(b, n) })
		b.Run(fmt.Sprintf("n=%d/map/lookup", n), func(b *testing.B) { benchmarkMapLookup(b, n) })
		b.Run(fmt.Sprintf("n=%d/TreeSet/delete", n), func(b *testing.B) { benchmarkTreeSetDelete(b, n) })
		b.Run(fmt.Sprintf("n=%d/HashSet/delete", n), func(b *testing.B) { benchmarkHashSetDelete(b, n) })
		b.Run(fmt.Sprintf("n=%d/map/delete", n), func(b *testing.B) { benchmarkMapDelete(b, n) })
	}
}
