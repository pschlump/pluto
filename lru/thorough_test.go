/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package lru

import (
	"math/rand"
	"reflect"
	"testing"
)

// model is the reference implementation: a map for the contents and a
// recency slice (most-recently-used first) for the order.
type model struct {
	contents map[int]int
	recency  []int // MRU first
	capacity int
}

func newModel(capacity int) *model {
	return &model{contents: make(map[int]int), recency: []int{}, capacity: capacity}
}

func (m *model) touch(i int) {
	for j, k := range m.recency {
		if k == i {
			m.recency = append(m.recency[:j], m.recency[j+1:]...)
			break
		}
	}
	m.recency = append([]int{i}, m.recency...)
}

func (m *model) put(k, v int) {
	if _, ok := m.contents[k]; ok {
		m.contents[k] = v
		m.touch(k)
		return
	}
	for len(m.recency) >= m.capacity {
		victim := m.recency[len(m.recency)-1]
		m.recency = m.recency[:len(m.recency)-1]
		delete(m.contents, victim)
	}
	m.contents[k] = v
	m.recency = append([]int{k}, m.recency...)
}

func (m *model) get(k int) (int, bool) {
	v, ok := m.contents[k]
	if ok {
		m.touch(k)
	}
	return v, ok
}

func (m *model) delete(k int) bool {
	if _, ok := m.contents[k]; !ok {
		return false
	}
	delete(m.contents, k)
	for j, key := range m.recency {
		if key == k {
			m.recency = append(m.recency[:j], m.recency[j+1:]...)
			break
		}
	}
	return true
}

// Randomized model test: mixed Get/Put/Delete against the reference
// model, verifying the contents AND the full recency order after every
// step.  The seed is fixed so the run is deterministic.
func TestLruRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewSource(42)) // fixed seed: deterministic run

	const capacity = 7
	const keySpace = 40 // larger than capacity, so evictions are constant
	cache := NewLru[int, int](capacity)
	m := newModel(capacity)

	verify := func(step int) {
		if cache.Len() != len(m.contents) {
			t.Fatalf("step %d: Len()=%d, model has %d entries", step, cache.Len(), len(m.contents))
		}
		order := []int{}
		for k, v := range cache.All() {
			order = append(order, k)
			if mv, ok := m.contents[k]; !ok || mv != v {
				t.Fatalf("step %d: cache has %d=%d, model has %d,%v", step, k, v, mv, ok)
			}
		}
		if !reflect.DeepEqual(order, m.recency) {
			t.Fatalf("step %d: recency order %v, model %v", step, order, m.recency)
		}
	}

	for step := 0; step < 10000; step++ {
		k := rng.Intn(keySpace)
		switch rng.Intn(10) {
		case 0, 1, 2, 3: // Put, including updates
			v := rng.Intn(1000)
			cache.Put(k, v)
			m.put(k, v)
		case 4, 5, 6, 7, 8: // Get
			cv, cok := cache.Get(k)
			mv, mok := m.get(k)
			if cok != mok || (cok && cv != mv) {
				t.Fatalf("step %d: Get(%d) = (%d, %v), model (%d, %v)", step, k, cv, cok, mv, mok)
			}
		default: // Delete
			if cok, mok := cache.Delete(k), m.delete(k); cok != mok {
				t.Fatalf("step %d: Delete(%d) = %v, model %v", step, k, cok, mok)
			}
		}
		verify(step)
	}
}

// The veto callback must never let eviction drop a protected entry,
// even under a random sequence: protected keys always survive.
func TestVetoRandomized(t *testing.T) {
	rng := rand.New(rand.NewSource(42)) // fixed seed: deterministic run

	protected := map[int]bool{3: true, 17: true, 28: true}
	cache := NewLruFunc(5, func(k int, _ int) bool { return !protected[k] })
	putProtected := map[int]bool{} // protected keys inserted so far
	for step := 0; step < 2000; step++ {
		k := rng.Intn(40)
		if rng.Intn(2) == 0 {
			cache.Put(k, step)
			if protected[k] {
				putProtected[k] = true
			}
		} else {
			cache.Get(k)
		}
		// A protected key that was put must still be present: the veto
		// must never let eviction drop it (this test never deletes).
		for pk := range putProtected {
			if _, ok := cache.Peek(pk); !ok {
				t.Fatalf("step %d: protected key %d was evicted", step, pk)
			}
		}
	}
}
