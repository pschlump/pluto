/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package binary_search_st

import (
	"fmt"
	"testing"

	"github.com/pschlump/pluto/skip_list"
)

// prefilled builds a table with n random two-letter keys (and a matching
// skip list, the capability twin, for the comparison benchmarks).
func prefilled(n int) (*BinarySearchST[string, int], *skip_list.SkipList[string], []string) {
	rng := newRand()
	st := NewBinarySearchST[string, int]()
	sl := skip_list.NewSkipList[string]()
	keys := make([]string, 0, n)
	for len(keys) < n {
		k := string(rune('a'+rng.Intn(26))) + string(rune('a'+rng.Intn(26))) + fmt.Sprintf("%04d", rng.Intn(10000))
		if !st.Contains(k) {
			st.Put(k, len(keys))
			sl.Insert(k)
			keys = append(keys, k)
		}
	}
	return st, sl, keys
}

// BenchmarkGetHit is the array ST's home ground: a binary search over
// contiguous memory, no pointers followed.
func BenchmarkGetHit(b *testing.B) {
	st, _, keys := prefilled(1000)
	b.ResetTimer()
	for i := range b.N {
		if _, found := st.Get(keys[i%len(keys)]); !found {
			b.Fatal("GetHit missed")
		}
	}
}

func BenchmarkGetMiss(b *testing.B) {
	st, _, _ := prefilled(1000)
	b.ResetTimer()
	for range b.N {
		if _, found := st.Get("zzzz-miss"); found {
			b.Fatal("GetMiss hit")
		}
	}
}

// BenchmarkPutReplace measures the pure search+store path (no shift).
func BenchmarkPutReplace(b *testing.B) {
	st, _, keys := prefilled(1000)
	b.ResetTimer()
	for i := range b.N {
		st.Put(keys[i%len(keys)], i)
	}
}

// BenchmarkPutDeleteRandom is the honest write cost: one insert shift
// plus one delete shift per iteration at a steady 1000 entries.
func BenchmarkPutDeleteRandom(b *testing.B) {
	st, _, _ := prefilled(1000)
	rng := newRand()
	b.ResetTimer()
	for i := range b.N {
		k := string(rune('A'+rng.Intn(26))) + fmt.Sprintf("%06d", i)
		st.Put(k, i)
		st.Delete(k)
	}
}

func BenchmarkFloor(b *testing.B) {
	st, _, keys := prefilled(1000)
	rng := newRand()
	b.ResetTimer()
	for range b.N {
		if _, _, found := st.Floor(keys[rng.Intn(len(keys))]); !found {
			b.Fatal("Floor missed")
		}
	}
}

// BenchmarkSearchSkipListHit is the same probe against skip_list, for
// the benchmark-reality note: O(log n) both, but the tree of pointers
// pays for every hop.
func BenchmarkSearchSkipListHit(b *testing.B) {
	_, sl, keys := prefilled(1000)
	b.ResetTimer()
	for i := range b.N {
		if _, found := sl.Search(keys[i%len(keys)]); !found {
			b.Fatal("SearchSkipListHit missed")
		}
	}
}
