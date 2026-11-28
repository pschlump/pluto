/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package multimap

import (
	"math/rand"
	"reflect"
	"slices"
	"sort"
	"testing"
)

func intEqual(a, b int) bool { return a == b }

// checkKeyTable verifies the open-addressing invariants of a table: the
// length matches the occupied slots, the load factor holds, and every entry
// is reachable from its home slot without crossing an empty slot.
func checkKeyTable[K comparable, P any](t *testing.T, tab *table[K, P]) {
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
				t.Fatalf("slot %d unreachable from home %d", index, tab.home(s.hash))
			}
		}
		if tab.lookup(s.key) == nil {
			t.Fatalf("key %v not found", s.key)
		}
	}
	if occupied != tab.length || tab.length*tableLoadDenom > len(tab.slots)*tableLoadNumer {
		t.Fatalf("occupied=%d length=%d slots=%d", occupied, tab.length, len(tab.slots))
	}
}

// TestMultimapClusteredHashModels drives all three hash-keyed variants with
// hashes that collapse keys and values into a few home slots (long probe
// runs and backward-shift deletion), against a map model, covering PutAll's
// in-place paths with duplicate-bearing batches and ReplaceValues.
func TestMultimapClusteredHashModels(t *testing.T) {
	keyHash := func(key int) uint64 { return uint64(key % 3) }
	valueHash := func(value int) uint64 { return uint64(value % 2) }
	for _, variant := range []string{"list", "set", "sorted"} {
		t.Run(variant, func(t *testing.T) {
			var actual intMultimap
			switch variant {
			case "list":
				actual = NewListMultimapFunc(intEqual, keyHash, intEqual)
			case "set":
				actual = NewSetMultimapFunc(intEqual, keyHash, intEqual, valueHash)
			default:
				actual = NewSortedSetMultimapFunc(intEqual, keyHash, func(a, b int) int { return a - b })
			}
			unique := variant != "list"
			random := rand.New(rand.NewSource(31337))
			model := map[int][]int{}
			add := func(key, value int) bool {
				if unique && slices.Contains(model[key], value) {
					return false
				}
				model[key] = append(model[key], value)
				return true
			}
			for step := range 5000 {
				key, value := random.Intn(40), random.Intn(25)
				switch random.Intn(6) {
				case 0:
					if actual.Put(key, value) != add(key, value) {
						t.Fatalf("step %d Put result mismatch", step)
					}
				case 1:
					batch := make([]int, random.Intn(6))
					for index := range batch {
						batch[index] = random.Intn(25)
					}
					want := 0
					for _, item := range batch {
						if add(key, item) {
							want++
						}
					}
					if got := actual.PutAll(key, batch); got != want {
						t.Fatalf("step %d PutAll(%d, %v)=%d want %d", step, key, batch, got, want)
					}
				case 2:
					index := slices.Index(model[key], value)
					if actual.Remove(key, value) != (index >= 0) {
						t.Fatalf("step %d Remove result mismatch", step)
					}
					if index >= 0 {
						model[key] = slices.Delete(model[key], index, index+1)
					}
				case 3:
					if got := actual.RemoveAll(key); len(got) != len(model[key]) {
						t.Fatalf("step %d RemoveAll returned %d values", step, len(got))
					}
					delete(model, key)
				case 4:
					batch := []int{value, random.Intn(25), value}
					actual.ReplaceValues(key, batch)
					delete(model, key)
					for _, item := range batch {
						add(key, item)
					}
				case 5:
					if actual.ContainsEntry(key, value) != slices.Contains(model[key], value) {
						t.Fatalf("step %d ContainsEntry mismatch", step)
					}
				}
				if len(model[key]) == 0 {
					delete(model, key)
				}
				if step%50 == 0 {
					checkClusteredModel(t, actual, model, variant)
				}
			}
			checkClusteredModel(t, actual, model, variant)
		})
	}
}

func checkClusteredModel(t *testing.T, actual intMultimap, model map[int][]int, variant string) {
	t.Helper()
	checkInvariant(t, actual)
	total := 0
	for key := range 40 {
		got, found := actual.Get(key)
		want := slices.Clone(model[key])
		total += len(want)
		switch variant {
		case "sorted":
			sort.Ints(want)
		case "set":
			sort.Ints(got)
			sort.Ints(want)
		}
		if found != (len(want) > 0) || (found && !slices.Equal(got, want)) {
			t.Fatalf("key %d got %v/%v want %v", key, got, found, want)
		}
	}
	if actual.Len() != total || actual.KeyLen() != len(model) {
		t.Fatalf("sizes (%d,%d) want (%d,%d)", actual.Len(), actual.KeyLen(), total, len(model))
	}
	switch m := actual.(type) {
	case *ListMultimap[int, int]:
		checkKeyTable(t, m.core.tab)
	case *SortedSetMultimap[int, int]:
		checkKeyTable(t, m.core.tab)
	case *SetMultimap[int, int]:
		checkKeyTable(t, m.core.tab)
		for _, values := range m.core.tab.all() {
			checkKeyTable(t, values)
		}
	}
}

type tagged struct{ value, tag int }

func taggedEq(a, b tagged) bool         { return a.value == b.value }
func taggedHash(a tagged) uint64        { return uint64(a.value) }
func taggedCompare(a, b tagged) int     { return a.value - b.value }
func keyTagged(name string, tag int) kt { return kt{name, tag} }

type kt struct {
	name string
	tag  int
}

// TestMultimapRepresentatives pins which equal key and value representatives
// are stored, as the hash_grow-backed implementation did: an adding write
// stores the latest key; a set Put of a present value replaces the stored
// value; a set PutAll that adds nothing changes nothing, otherwise the last
// equal batch value wins; a sorted PutAll keeps existing values and the
// first equal batch value; Remove keeps the stored key.
func TestMultimapRepresentatives(t *testing.T) {
	keyEq := func(a, b kt) bool { return a.name == b.name }
	keyHash := func(a kt) uint64 { return uint64(len(a.name)) }
	set := NewSetMultimapFunc(keyEq, keyHash, taggedEq, taggedHash)
	set.Put(keyTagged("k", 1), tagged{1, 1})
	if set.Put(keyTagged("k", 2), tagged{1, 2}) {
		t.Fatal("duplicate Put reported as added")
	}
	if got := slices.Collect(set.KeySet()); got[0].tag != 1 {
		t.Fatalf("non-adding Put replaced the key: %v", got)
	}
	if values, _ := set.Get(keyTagged("k", 0)); values[0].tag != 2 {
		t.Fatalf("duplicate Put kept the old value representative: %v", values)
	}
	if set.PutAll(keyTagged("k", 3), []tagged{{1, 3}}) != 0 {
		t.Fatal("PutAll of a present value added")
	}
	if values, _ := set.Get(keyTagged("k", 0)); values[0].tag != 2 || slices.Collect(set.KeySet())[0].tag != 1 {
		t.Fatalf("a PutAll that added nothing changed representatives: %v", values)
	}
	if set.PutAll(keyTagged("k", 4), []tagged{{1, 4}, {2, 4}, {2, 5}}) != 1 {
		t.Fatal("PutAll count")
	}
	values, _ := set.Get(keyTagged("k", 0))
	sort.Slice(values, func(i, j int) bool { return values[i].value < values[j].value })
	if !reflect.DeepEqual(values, []tagged{{1, 4}, {2, 5}}) || slices.Collect(set.KeySet())[0].tag != 4 {
		t.Fatalf("adding PutAll representatives: %v %v", values, slices.Collect(set.KeySet()))
	}
	set.Remove(keyTagged("k", 6), tagged{2, 0})
	if slices.Collect(set.KeySet())[0].tag != 4 {
		t.Fatal("Remove replaced the stored key")
	}

	sorted := NewSortedSetMultimapFunc(keyEq, keyHash, taggedCompare)
	sorted.PutAll(keyTagged("s", 1), []tagged{{5, 1}, {3, 1}})
	sorted.PutAll(keyTagged("s", 2), []tagged{{3, 2}, {4, 2}, {4, 3}, {1, 2}})
	got, _ := sorted.Get(keyTagged("s", 0))
	if !reflect.DeepEqual(got, []tagged{{1, 2}, {3, 1}, {4, 2}, {5, 1}}) || slices.Collect(sorted.KeySet())[0].tag != 2 {
		t.Fatalf("sorted PutAll representatives: %v", got)
	}
	if !sorted.Put(keyTagged("s", 3), tagged{9, 3}) || slices.Collect(sorted.KeySet())[0].tag != 3 {
		t.Fatal("an adding Put did not store the latest key representative")
	}

	list := NewListMultimapFunc(keyEq, keyHash, taggedEq)
	batch := []tagged{{1, 1}, {2, 1}}
	list.PutAll(keyTagged("l", 1), batch)
	batch[0].tag = 99
	if got, _ := list.Get(keyTagged("l", 0)); got[0].tag != 1 {
		t.Fatal("list PutAll aliased the caller's batch")
	}
}

// rebuildSortedBatch is the pre-merge reference: copy existing and batch,
// stable-sort, and keep the first of every equal run.
func rebuildSortedBatch(values, batch []tagged) []tagged {
	result := append(slices.Clone(values), batch...)
	sort.SliceStable(result, func(i, j int) bool { return result[i].value < result[j].value })
	write := 0
	for read := range result {
		if write > 0 && result[write-1].value == result[read].value {
			continue
		}
		result[write] = result[read]
		write++
	}
	return result[:write]
}

func TestMergeSortedBatchMatchesRebuild(t *testing.T) {
	random := rand.New(rand.NewSource(99))
	for trial := range 3000 {
		existing := rebuildSortedBatch(nil, randomTagged(random, random.Intn(12), 1))
		batch := randomTagged(random, random.Intn(10), 2)
		want := rebuildSortedBatch(existing, batch)
		got, added := mergeSortedBatch(slices.Clone(existing), batch, taggedCompare)
		if !reflect.DeepEqual(got, want) || added != len(want)-len(existing) {
			t.Fatalf("trial %d: existing %v batch %v\ngot  %v (%d)\nwant %v", trial, existing, batch, got, added, want)
		}
	}
}

func randomTagged(random *rand.Rand, n, tag int) []tagged {
	values := make([]tagged, n)
	for index := range values {
		values[index] = tagged{random.Intn(16), tag*100 + index}
	}
	return values
}

func TestMergeSortedBatchPanicLeavesValuesUnchanged(t *testing.T) {
	values := make([]int, 3, 8)
	copy(values, []int{1, 3, 5})
	backing := values[:8]
	compare := func(a, b int) int {
		if a == 99 || b == 99 {
			panic("comparator")
		}
		return a - b
	}
	func() {
		defer func() { _ = recover() }()
		mergeSortedBatch(values, []int{2, 4, 99}, compare)
	}()
	if !slices.Equal(backing, []int{1, 3, 5, 0, 0, 0, 0, 0}) {
		t.Fatalf("panicking comparator mutated the collection: %v", backing)
	}
}

func TestMultimapSteadyStateAllocations(t *testing.T) {
	set := NewSetMultimap[string, string]()
	present := []string{"a", "c"} // allocated once: a PutAll batch escapes
	sorted := NewSortedSetMultimap[string, string]()
	linked := NewLinkedHashMultimap[string, string]()
	for _, value := range []string{"a", "b", "c", "d"} {
		set.Put("key", value)
		sorted.Put("key", value)
		linked.Put("key", value)
	}
	for name, run := range map[string]func(){
		"set": func() {
			set.Remove("key", "b")
			set.Put("key", "b")
			_ = set.Put("key", "c")
			_ = set.ContainsEntry("key", "d")
			_ = set.PutAll("key", present)
		},
		"sorted": func() {
			sorted.Remove("key", "b")
			sorted.Put("key", "b")
			_ = sorted.ContainsEntry("key", "d")
		},
		"linked": func() {
			_ = linked.Put("key", "c")
			_ = linked.ContainsEntry("key", "d")
			_ = linked.ContainsKey("key")
		},
	} {
		if allocs := testing.AllocsPerRun(200, run); allocs != 0 {
			t.Errorf("%s: %.1f allocations per steady-state operation, want 0", name, allocs)
		}
	}
}
