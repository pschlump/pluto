/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package multiset

import (
	"math/rand"
	"reflect"
	"slices"
	"testing"
)

// tableHashes are the hash functions the private tables are exercised with:
// a well-mixed hash, an identity hash, a 4-bucket hash that forces long
// probe runs (and wraparound) for backward-shift deletion, and one that
// returns the reserved empty-slot marker 0 for every value.
var tableHashes = map[string]func(int) uint64{
	"mixed":    func(value int) uint64 { return uint64(value)*0xff51afd7ed558ccd ^ 0x9e3779b97f4a7c15 },
	"identity": func(value int) uint64 { return uint64(value) },
	"clustered": func(value int) uint64 {
		return uint64(value % 4)
	},
	"zero": func(int) uint64 { return 0 },
}

func intEq(a, b int) bool { return a == b }

// checkTable verifies the open-addressing invariants: the length matches
// the occupied slots, no stored hash is 0, the load factor holds, and every
// entry is reachable from its home slot without crossing an empty slot.
func checkTable[P any](t *testing.T, tab *table[int, P]) {
	t.Helper()
	mask := len(tab.slots) - 1
	if len(tab.slots)&mask != 0 || 64-tab.shift != uint(bitsLen(len(tab.slots))) {
		t.Fatalf("bad table geometry %d slots shift %d", len(tab.slots), tab.shift)
	}
	occupied := 0
	for index, s := range tab.slots {
		if s.hash == 0 {
			continue
		}
		occupied++
		for probe := tab.home(s.hash); probe != index; probe = (probe + 1) & mask {
			if tab.slots[probe].hash == 0 {
				t.Fatalf("slot %d unreachable from home %d", index, tab.home(s.hash))
			}
		}
		if found := tab.lookup(s.key); found == nil {
			t.Fatalf("key %d not found", s.key)
		}
	}
	if occupied != tab.length || tab.length*tableLoadDenom > len(tab.slots)*tableLoadNumer {
		t.Fatalf("occupied=%d length=%d slots=%d", occupied, tab.length, len(tab.slots))
	}
}

func bitsLen(n int) int {
	bits := 0
	for n > 1 {
		n >>= 1
		bits++
	}
	return bits
}

func checkLinkedTable(t *testing.T, tab *linkedTable[int, int], order []int) {
	t.Helper()
	mask := len(tab.slots) - 1
	occupied := 0
	for index, s := range tab.slots {
		if s.hash == 0 {
			continue
		}
		occupied++
		for probe := tab.home(s.hash); probe != index; probe = (probe + 1) & mask {
			if tab.slots[probe].hash == 0 {
				t.Fatalf("slot %d unreachable", index)
			}
		}
	}
	if occupied != tab.length || occupied != len(order) {
		t.Fatalf("occupied=%d length=%d model=%d", occupied, tab.length, len(order))
	}
	previous := -1
	walked := []int{}
	for current := tab.head; current >= 0; current = tab.slots[current].next {
		if tab.slots[current].hash == 0 || tab.slots[current].prev != previous {
			t.Fatalf("broken list at slot %d", current)
		}
		walked = append(walked, tab.slots[current].key)
		previous = current
		if len(walked) > occupied {
			t.Fatal("list cycle")
		}
	}
	if tab.tail != previous || !slices.Equal(walked, order) {
		t.Fatalf("list order %v, want %v", walked, order)
	}
}

func TestHashMultisetTableDifferential(t *testing.T) {
	for name, hash := range tableHashes {
		t.Run(name, func(t *testing.T) {
			random := rand.New(rand.NewSource(2026))
			set := NewHashMultisetFunc(intEq, hash)
			model := map[int]int{}
			total := 0
			for step := range 6000 {
				value, count := random.Intn(300), random.Intn(5)
				var got, want int
				switch random.Intn(5) {
				case 0, 1:
					got = set.Add(value, count)
					model[value] += count
					total += count
					want = model[value]
				case 2:
					got = set.Remove(value, count)
					removed := min(count, model[value])
					model[value] -= removed
					total -= removed
					want = model[value]
				case 3:
					got, want = set.SetCount(value, count), model[value]
					total += count - model[value]
					model[value] = count
				case 4:
					got, want = set.Count(value), model[value]
				}
				if got != want {
					t.Fatalf("step %d value %d: got %d want %d", step, value, got, want)
				}
				if model[value] == 0 {
					delete(model, value)
				}
				if step%97 == 0 {
					checkTable(t, set.tab)
					if set.Len() != total || set.DistinctLen() != len(model) {
						t.Fatalf("sizes (%d,%d) want (%d,%d)", set.Len(), set.DistinctLen(), total, len(model))
					}
					seen := map[int]int{}
					for entry := range set.Entries() {
						seen[entry.Value] = entry.Count
					}
					if !reflect.DeepEqual(seen, model) {
						t.Fatalf("step %d entries mismatch", step)
					}
				}
			}
			clone := set.Clone()
			checkTable(t, clone.tab)
			if !clone.Equal(set) {
				t.Fatal("clone differs")
			}
			set.Clear()
			checkTable(t, set.tab)
			if set.Len() != 0 || set.DistinctLen() != 0 || clone.Len() != total {
				t.Fatal("Clear or clone independence failed")
			}
		})
	}
}

func TestLinkedHashMultisetTableDifferential(t *testing.T) {
	for name, hash := range tableHashes {
		t.Run(name, func(t *testing.T) {
			random := rand.New(rand.NewSource(1729))
			set := NewLinkedHashMultisetFunc(intEq, hash)
			model := map[int]int{}
			order := []int{}
			remove := func(value int) {
				order = slices.DeleteFunc(order, func(candidate int) bool { return candidate == value })
			}
			for step := range 6000 {
				value, count := random.Intn(200), random.Intn(5)
				before := model[value]
				var got, want int
				switch random.Intn(4) {
				case 0, 1:
					got = set.Add(value, count)
					model[value] += count
					want = model[value]
				case 2:
					got = set.Remove(value, count)
					model[value] -= min(count, model[value])
					want = model[value]
				case 3:
					got, want = set.SetCount(value, count), before
					model[value] = count
				}
				if got != want {
					t.Fatalf("step %d value %d: got %d want %d", step, value, got, want)
				}
				switch {
				case before == 0 && model[value] > 0:
					order = append(order, value)
				case before > 0 && model[value] == 0:
					remove(value)
					delete(model, value)
				case model[value] == 0:
					delete(model, value)
				}
				if step%89 == 0 {
					checkLinkedTable(t, set.counts, order)
					entries := set.EntriesSlice()
					if len(entries) != len(order) {
						t.Fatalf("step %d: %d entries want %d", step, len(entries), len(order))
					}
					for index, entry := range entries {
						if entry.Value != order[index] || entry.Count != model[entry.Value] {
							t.Fatalf("step %d entry %d = %+v", step, index, entry)
						}
					}
				}
			}
			clone := set.Clone()
			checkLinkedTable(t, clone.counts, order)
			set.Clear()
			checkLinkedTable(t, set.counts, nil)
			if !slices.Equal(slices.Collect(clone.Elements()), order) {
				t.Fatal("clone lost insertion order")
			}
		})
	}
}

func TestLinkedHashMultisetKeepsFirstRepresentative(t *testing.T) {
	type item struct{ key, tag int }
	set := NewLinkedHashMultisetFunc(
		func(a, b item) bool { return a.key == b.key },
		func(a item) uint64 { return uint64(a.key) },
	)
	set.Add(item{1, 10}, 1)
	set.Add(item{2, 20}, 1)
	set.Add(item{1, 11}, 2)
	set.SetCount(item{2, 21}, 5)
	want := []Entry[item]{{Value: item{1, 10}, Count: 3}, {Value: item{2, 20}, Count: 5}}
	if got := set.EntriesSlice(); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v want %+v", got, want)
	}
}

func TestMultisetSteadyStateAllocations(t *testing.T) {
	hash := NewHashMultiset[string]()
	linked := NewLinkedHashMultiset[string]()
	tree := NewTreeMultiset[string]()
	keys := []string{"alpha", "beta", "gamma", "delta"}
	for _, key := range keys {
		hash.Add(key, 10)
		linked.Add(key, 10)
		tree.Add(key, 10)
	}
	for name, run := range map[string]func(){
		"hash": func() {
			hash.Add(keys[1], 2)
			hash.Remove(keys[1], 1)
			hash.SetCount(keys[2], 7)
			_ = hash.Count(keys[3])
		},
		"linked": func() {
			linked.Add(keys[1], 2)
			linked.Remove(keys[1], 1)
			linked.SetCount(keys[2], 7)
			_ = linked.Count(keys[3])
		},
		"tree": func() {
			tree.Add(keys[1], 2)
			tree.Remove(keys[1], 1)
			tree.SetCount(keys[2], 7)
			_ = tree.Count(keys[3])
		},
	} {
		if allocs := testing.AllocsPerRun(200, run); allocs != 0 {
			t.Errorf("%s: %.1f allocations per update of present values, want 0", name, allocs)
		}
	}
}
