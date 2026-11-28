/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package linked_hash_map

import (
	"encoding/json"
	"math/rand"
	"reflect"
	"slices"
	"testing"
)

// checkIndex verifies the index against the insertion-order list: the list
// length matches Len, every listed key is found at its own node, and for the
// open-addressing table every occupied cell is reachable from its home cell
// without crossing an empty cell (the backward-shift deletion invariant) and
// the occupied-cell count equals Len.
func checkIndex[K comparable, V any](t *testing.T, m *Map[K, V], step int) {
	t.Helper()
	count := 0
	var previous *node[K, V]
	for current := m.head; current != nil; current = current.next {
		if current.prev != previous {
			t.Fatalf("step %d: broken prev link", step)
		}
		if m.find(current.entry.Key) != current {
			t.Fatalf("step %d: key %v not indexed at its node", step, current.entry.Key)
		}
		previous = current
		count++
	}
	if previous != m.tail || count != m.length {
		t.Fatalf("step %d: list count %d, tail ok %v, length %d", step, count, previous == m.tail, m.length)
	}
	if m.native != nil {
		if len(m.slots) != 0 {
			t.Fatalf("step %d: native map allocated table cells", step)
		}
		return
	}
	occupied := 0
	mask := len(m.slots) - 1
	for index, cell := range m.slots {
		if cell.node == nil {
			continue
		}
		occupied++
		if cell.hash != m.hash(cell.node.entry.Key) {
			t.Fatalf("step %d: stale cached hash at cell %d", step, index)
		}
		for probe := int((cell.hash * fibMul) >> m.shift); probe&mask != index; probe++ {
			if m.slots[probe&mask].node == nil {
				t.Fatalf("step %d: cell %d unreachable from its home", step, index)
			}
		}
	}
	if occupied != m.length || occupied*4 > len(m.slots)*3 {
		t.Fatalf("step %d: %d occupied of %d cells, length %d", step, occupied, len(m.slots), m.length)
	}
}

// indexFlavors covers the native built-in-map index and the open-addressing
// table under good, weak, colliding and zero hashes.
func indexFlavors() map[string]func() *Map[int, int] {
	eq := func(a, b int) bool { return a == b }
	return map[string]func() *Map[int, int]{
		"native":   New[int, int],
		"identity": func() *Map[int, int] { return NewFunc[int, int](eq, func(k int) uint64 { return uint64(k) }) },
		"mod7":     func() *Map[int, int] { return NewFunc[int, int](eq, func(k int) uint64 { return uint64(k % 7) }) },
		"zero":     func() *Map[int, int] { return NewFunc[int, int](eq, func(int) uint64 { return 0 }) },
		"high":     func() *Map[int, int] { return NewFunc[int, int](eq, func(k int) uint64 { return uint64(k) << 40 }) },
	}
}

func TestIndexDifferentialModel(t *testing.T) {
	for name, construct := range indexFlavors() {
		t.Run(name, func(t *testing.T) {
			random := rand.New(rand.NewSource(20261127))
			actual := construct()
			model := map[int]int{}
			order := []int{}
			keySpace := 300
			for step := range 20000 {
				key, value := random.Intn(keySpace), random.Intn(1_000_000)
				switch op := random.Intn(100); {
				case op < 40:
					old, found := actual.Put(key, value)
					want, existed := model[key]
					if old != want || found != existed {
						t.Fatalf("step %d Put(%d)=%d/%v want %d/%v", step, key, old, found, want, existed)
					}
					if !existed {
						order = append(order, key)
					}
					model[key] = value
				case op < 70:
					old, found := actual.Delete(key)
					want, existed := model[key]
					if old != want || found != existed {
						t.Fatalf("step %d Delete(%d)=%d/%v want %d/%v", step, key, old, found, want, existed)
					}
					if existed {
						delete(model, key)
						order = slices.DeleteFunc(order, func(candidate int) bool { return candidate == key })
					}
				case op < 95:
					got, found := actual.Get(key)
					want, existed := model[key]
					if got != want || found != existed || actual.ContainsKey(key) != existed {
						t.Fatalf("step %d Get(%d)=%d/%v want %d/%v", step, key, got, found, want, existed)
					}
				case op < 97:
					actual.Clear()
					clear(model)
					order = order[:0]
				case op < 98:
					clone := actual.Clone()
					if (clone.native != nil) != (actual.native != nil) {
						t.Fatalf("step %d: Clone changed the index kind", step)
					}
					checkIndex(t, clone, step)
					clone.Put(-1, -1)
					if actual.ContainsKey(-1) {
						t.Fatalf("step %d: clone shares the index", step)
					}
					actual = clone
					actual.Delete(-1)
				default:
					data, err := json.Marshal(actual)
					if err != nil {
						t.Fatal(err)
					}
					decoded := construct()
					decoded.Put(-2, -2)
					if err = json.Unmarshal(data, decoded); err != nil {
						t.Fatal(err)
					}
					if (decoded.native != nil) != (actual.native != nil) {
						t.Fatalf("step %d: UnmarshalJSON changed the index kind", step)
					}
					actual = decoded
				}
				if step%7 == 0 || step < 200 {
					checkIndex(t, actual, step)
					if actual.Len() != len(model) || !slices.Equal(slices.Collect(actual.Keys()), order) {
						t.Fatalf("step %d: order or length mismatch", step)
					}
				}
			}
			entries := actual.EntriesSlice()
			for index, entry := range entries {
				if entry.Key != order[index] || entry.Value != model[entry.Key] {
					t.Fatalf("final entry %d = %+v", index, entry)
				}
			}
		})
	}
}

func TestIndexGrowthKeepsOrder(t *testing.T) {
	for name, construct := range indexFlavors() {
		t.Run(name, func(t *testing.T) {
			m := construct()
			n := 2000
			if name == "zero" {
				n = 300 // every key collides: quadratic by design
			}
			for key := n - 1; key >= 0; key-- {
				m.Put(key, key*3)
			}
			checkIndex(t, m, 0)
			for key := 0; key < n; key += 2 {
				m.Delete(key)
			}
			checkIndex(t, m, 1)
			want := []int{}
			for key := n - 1; key >= 0; key-- {
				if key%2 == 1 {
					want = append(want, key)
				}
			}
			if got := slices.Collect(m.Keys()); !reflect.DeepEqual(got, want) {
				t.Fatalf("order after growth and deletes: %v", got[:10])
			}
		})
	}
}

func TestSteadyStateAllocations(t *testing.T) {
	for name, construct := range indexFlavors() {
		m := construct()
		for key := range 1000 {
			m.Put(key, key)
		}
		// Grow and shrink once so a later reinsertion reuses a free cell.
		m.Delete(999)
		key := 0
		allocs := testing.AllocsPerRun(1000, func() {
			key = (key + 1) % 999
			m.Put(key, key+1)
			if value, found := m.Get(key); !found || value != key+1 {
				t.Fatal("lost key")
			}
			_ = m.ContainsKey(key + 5000)
		})
		if allocs != 0 {
			t.Errorf("%s: update/get/miss allocated %.1f per run", name, allocs)
		}
		sum := 0
		allocs = testing.AllocsPerRun(100, func() {
			for _, value := range m.All() {
				sum += value
			}
		})
		if allocs != 0 {
			t.Errorf("%s: All iteration allocated %.1f per run", name, allocs)
		}
		// Delete plus reinsertion costs exactly the new node.
		allocs = testing.AllocsPerRun(1000, func() {
			m.Delete(500)
			m.Put(500, 500)
		})
		if allocs != 1 {
			t.Errorf("%s: Delete+Put allocated %.1f per run, want 1", name, allocs)
		}
	}
}
