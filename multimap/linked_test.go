/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package multimap

import (
	"math/rand"
	"reflect"
	"slices"
	"testing"

	"github.com/pschlump/pluto/multiset"
)

func TestLinkedMultimapOrdering(t *testing.T) {
	list := NewLinkedListMultimap[string, int]()
	list.Put("b", 2)
	list.Put("a", 1)
	list.Put("b", 3)
	list.Put("a", 1)
	if got := list.EntriesSlice(); !reflect.DeepEqual(got, []Association[string, int]{{"b", 2}, {"a", 1}, {"b", 3}, {"a", 1}}) {
		t.Fatal(got)
	}
	list.RemoveAll("b")
	list.Put("b", 4)
	if got := slices.Collect(list.KeySet()); !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Fatal(got)
	}

	set := NewLinkedHashMultimap[string, int]()
	set.PutAll("b", []int{3, 1, 3, 2})
	set.Put("a", 9)
	set.Put("b", 1)
	if got := set.EntriesSlice(); !reflect.DeepEqual(got, []Association[string, int]{{"b", 3}, {"b", 1}, {"b", 2}, {"a", 9}}) {
		t.Fatal(got)
	}
	set.Remove("b", 1)
	set.Put("b", 1)
	if values, _ := set.Get("b"); !reflect.DeepEqual(values, []int{3, 2, 1}) {
		t.Fatal(values)
	}
	if got := set.KeyMultiset().EntriesSlice(); !reflect.DeepEqual(got, []multiset.Entry[string]{{Value: "b", Count: 3}, {Value: "a", Count: 1}}) {
		t.Fatal(got)
	}
}

func TestLinkedMultimapModelAgainstBase(t *testing.T) {
	linked := NewLinkedListMultimap[int, int]()
	base := NewListMultimap[int, int]()
	for step := range 1000 {
		key, value := step%17, step%29
		switch step % 4 {
		case 0:
			linked.Put(key, value)
			base.Put(key, value)
		case 1:
			linked.PutAll(key, []int{value, value + 1})
			base.PutAll(key, []int{value, value + 1})
		case 2:
			linked.Remove(key, value)
			base.Remove(key, value)
		case 3:
			linked.ReplaceValues(key, []int{value, value + 1})
			base.ReplaceValues(key, []int{value, value + 1})
		}
		if linked.Len() != base.Len() || linked.KeyLen() != base.KeyLen() {
			t.Fatalf("step %d size mismatch", step)
		}
		if step%25 == 0 {
			checkLinkedInvariant(t, &linked.core)
		}
		for key := range 17 {
			got, gotFound := linked.Get(key)
			want, wantFound := base.Get(key)
			if gotFound != wantFound || !reflect.DeepEqual(got, want) {
				t.Fatalf("step %d key %d: %v/%v want %v/%v", step, key, got, gotFound, want, wantFound)
			}
		}
	}
}

func TestLinkedHashMultimapGlobalOrderModel(t *testing.T) {
	random := rand.New(rand.NewSource(8128))
	actual := NewLinkedHashMultimap[int, int]()
	model := map[int][]int{}
	global := []Association[int, int]{}
	for step := range 2000 {
		key, value := random.Intn(11), random.Intn(17)
		switch random.Intn(4) {
		case 0:
			added := actual.Put(key, value)
			if !slices.Contains(model[key], value) {
				if !added {
					t.Fatal("new association reported as duplicate")
				}
				model[key] = append(model[key], value)
				global = append(global, Association[int, int]{Key: key, Value: value})
			} else if added {
				t.Fatal("duplicate reported as new")
			}
		case 1:
			values := []int{value, random.Intn(17), value}
			actual.PutAll(key, values)
			for _, item := range values {
				if !slices.Contains(model[key], item) {
					model[key] = append(model[key], item)
					global = append(global, Association[int, int]{Key: key, Value: item})
				}
			}
		case 2:
			removed := actual.Remove(key, value)
			index := slices.Index(model[key], value)
			if removed != (index >= 0) {
				t.Fatal("remove result mismatch")
			}
			if index >= 0 {
				model[key] = append(model[key][:index], model[key][index+1:]...)
				for position, association := range global {
					if association.Key == key && association.Value == value {
						global = append(global[:position], global[position+1:]...)
						break
					}
				}
			}
		case 3:
			actual.RemoveAll(key)
			delete(model, key)
			kept := global[:0]
			for _, association := range global {
				if association.Key != key {
					kept = append(kept, association)
				}
			}
			global = kept
		}
		if len(model[key]) == 0 {
			delete(model, key)
		}
		if step%25 == 0 {
			checkLinkedInvariant(t, &actual.core)
			if !reflect.DeepEqual(actual.EntriesSlice(), global) {
				t.Fatalf("step %d global order mismatch\ngot=%v\nwant=%v", step, actual.EntriesSlice(), global)
			}
			for expectedKey := range 11 {
				got, found := actual.Get(expectedKey)
				want := model[expectedKey]
				if found != (len(want) != 0) || !reflect.DeepEqual(got, want) {
					t.Fatalf("step %d key %d got=%v/%v want=%v", step, expectedKey, got, found, want)
				}
			}
		}
	}
}

func checkLinkedInvariant(t *testing.T, core *linkedCore[int, int]) {
	t.Helper()
	globalCount := 0
	var previous *linkedAssociation[int, int]
	for node := core.head; node != nil; node = node.next {
		if node.prev != previous {
			t.Fatal("broken global previous link")
		}
		if !core.keys.ContainsKey(node.key) {
			t.Fatal("association refers to missing key")
		}
		globalCount++
		previous = node
	}
	if core.tail != previous || globalCount != core.size || core.keys.Len() > core.size {
		t.Fatal("global links or sizes are inconsistent")
	}
	keyTotal := 0
	for key := range core.KeySet() {
		entry, found := core.keys.Get(key)
		if !found || entry.count == 0 {
			t.Fatal("empty key entry")
		}
		count := 0
		var keyPrevious *linkedAssociation[int, int]
		for node := entry.head; node != nil; node = node.keyNext {
			if node.key != key || node.keyPrev != keyPrevious {
				t.Fatal("broken per-key links")
			}
			if core.unique {
				stored, found := entry.values.Get(node.associationValue)
				if !found || stored != node {
					t.Fatal("value index does not reference association")
				}
			}
			count++
			keyPrevious = node
		}
		if entry.tail != keyPrevious || count != entry.count {
			t.Fatal("per-key count mismatch")
		}
		if core.unique && entry.values.Len() != count {
			t.Fatal("value index count mismatch")
		}
		keyTotal += count
	}
	if keyTotal != core.size {
		t.Fatal("per-key and global sizes differ")
	}
}
