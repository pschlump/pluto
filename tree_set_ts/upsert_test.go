/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package tree_set_ts

import (
	"math/rand"
	"sync"
	"testing"

	"github.com/pschlump/pluto/tree_set"
)

// TestTwinUpsertRemoveParity runs the same operations on the twin and the
// plain set and requires identical results.
func TestTwinUpsertRemoveParity(t *testing.T) {
	random := rand.New(rand.NewSource(42))
	twin, plain := NewTreeSet[int](), tree_set.NewTreeSet[int]()
	for step := range 4000 {
		value := random.Intn(300)
		if random.Intn(2) == 0 {
			a, ar := twin.Upsert(value)
			b, br := plain.Upsert(value)
			if a != b || ar != br {
				t.Fatalf("step %d Upsert(%d): twin %v,%v plain %v,%v", step, value, a, ar, b, br)
			}
		} else {
			a, af := twin.Remove(value)
			b, bf := plain.Remove(value)
			if a != b || af != bf {
				t.Fatalf("step %d Remove(%d): twin %v,%v plain %v,%v", step, value, a, af, b, bf)
			}
		}
	}
	if twin.Len() != plain.Len() {
		t.Fatalf("Len twin %d plain %d", twin.Len(), plain.Len())
	}
}

func TestTwinUpsertRemoveContract(t *testing.T) {
	var nilSet *TreeSet[int]
	expectPanic(t, "nil Upsert", func() { nilSet.Upsert(1) })
	expectPanic(t, "zero-value Upsert", func() { (&TreeSet[int]{}).Upsert(1) })
	if _, found := nilSet.Remove(1); found {
		t.Fatal("nil Remove reported found")
	}
	if _, found := nilSet.NlRemove(1); found {
		t.Fatal("nil NlRemove reported found")
	}
	set := NewTreeSet[int]()
	set.Lock()
	if _, replaced := set.NlUpsert(5); replaced {
		t.Fatal("NlUpsert of a new value reported replaced")
	}
	if removed, found := set.NlRemove(5); !found || removed != 5 {
		t.Fatalf("NlRemove = %v,%v", removed, found)
	}
	set.Unlock()
}

// TestTwinUpsertRemoveConcurrent: for each value, exactly one of the racing
// Upserts sees replaced == false, and Remove/Upsert never corrupt the set.
func TestTwinUpsertRemoveConcurrent(t *testing.T) {
	set := NewTreeSet[int]()
	var mu sync.Mutex
	fresh := map[int]int{}
	var wait sync.WaitGroup
	for range 8 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			for value := range 300 {
				if _, replaced := set.Upsert(value); !replaced {
					mu.Lock()
					fresh[value]++
					mu.Unlock()
				}
			}
		}()
	}
	wait.Wait()
	for value := range 300 {
		if fresh[value] != 1 {
			t.Fatalf("value %d inserted fresh %d times", value, fresh[value])
		}
	}
	for value := range 300 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			if _, found := set.Remove(value); !found {
				panic("concurrent Remove missed a present value")
			}
		}()
	}
	wait.Wait()
	if set.Len() != 0 {
		t.Fatalf("Len after removing all = %d", set.Len())
	}
}
