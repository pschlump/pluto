/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package tree_set

import (
	"math/rand"
	"testing"
)

// TestTreeSetUpsertRemoveModel drives Upsert/Remove/Insert/Delete against a
// map. Items carry satellite data (N) the comparator ignores, so the test can
// tell which equal element is returned and which is stored.
func TestTreeSetUpsertRemoveModel(t *testing.T) {
	random := rand.New(rand.NewSource(42))
	set := NewTreeSetFunc(cmpTestItem)
	model := map[string]int{}
	for step := range 6000 {
		key := string(rune('a' + random.Intn(26)))
		item := testItem{Key: key, N: step}
		switch random.Intn(4) {
		case 0:
			old, replaced := set.Upsert(item)
			want, present := model[key]
			if replaced != present || (replaced && old.N != want) {
				t.Fatalf("step %d Upsert(%s) = %v,%v; model %v,%v", step, key, old, replaced, want, present)
			}
			model[key] = step
		case 1:
			removed, found := set.Remove(testItem{Key: key})
			want, present := model[key]
			if found != present || (found && removed.N != want) {
				t.Fatalf("step %d Remove(%s) = %v,%v; model %v,%v", step, key, removed, found, want, present)
			}
			delete(model, key)
		case 2:
			set.Insert(item)
			model[key] = step
		case 3:
			set.Delete(testItem{Key: key})
			delete(model, key)
		}
		if set.Len() != len(model) {
			t.Fatalf("step %d Len %d, model %d", step, set.Len(), len(model))
		}
	}
	for key, n := range model {
		if got, found := set.Search(testItem{Key: key}); !found || got.N != n {
			t.Fatalf("final Search(%s) = %v,%v want N=%d", key, got, found, n)
		}
	}
}

func TestTreeSetUpsertRemoveNilAndZero(t *testing.T) {
	var nilSet *TreeSet[int]
	expectInsertPanic(t, "nil Upsert", "Upsert", func() { nilSet.Upsert(1) })
	expectInsertPanic(t, "zero-value Upsert", "Upsert", func() { (&TreeSet[int]{}).Upsert(1) })
	if _, found := nilSet.Remove(1); found {
		t.Fatal("nil Remove reported found")
	}
	if _, found := (&TreeSet[int]{}).Remove(1); found {
		t.Fatal("zero-value Remove reported found")
	}
}
