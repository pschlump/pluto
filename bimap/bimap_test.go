/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package bimap

import (
	"encoding/json"
	"errors"
	"hash/fnv"
	"math/rand"
	"reflect"
	"strings"
	"testing"
)

func TestConflictPermutations(t *testing.T) {
	set := NewHashBiMap[int, string]()
	if err := set.Put(1, "a"); err != nil {
		t.Fatal(err)
	}
	if err := set.Put(2, "a"); !errors.Is(err, ErrValueConflict) {
		t.Fatal("expected value conflict")
	}
	if set.Len() != 1 || set.ContainsKey(2) {
		t.Fatal("conflict mutated bimap")
	}
	if err := set.Put(1, "a"); err != nil || set.Len() != 1 {
		t.Fatal("same association should be a no-op")
	}
	if err := set.Put(1, "b"); err != nil {
		t.Fatal(err)
	}
	if _, found := set.GetKey("a"); found {
		t.Fatal("stale reverse entry after update")
	}
	if err := set.Put(2, "a"); err != nil {
		t.Fatal("released value should be reusable")
	}
	set.ForcePut(3, "b")
	if set.ContainsKey(1) || set.Len() != 2 {
		t.Fatal("ForcePut did not remove conflicting old key")
	}
	if value, _ := set.GetValue(3); value != "b" {
		t.Fatal(value)
	}
	set.ForcePut(3, "b")
	checkInvariant(t, set)
}

func TestInverseMutationAndIdentity(t *testing.T) {
	set := NewHashBiMap[int, string]()
	inverse := set.Inverse()
	if set.Inverse().Inverse() != set || inverse.Inverse() != set {
		t.Fatal("inverse-of-inverse identity failed")
	}
	if err := inverse.Put("x", 9); err != nil {
		t.Fatal(err)
	}
	if value, found := set.GetValue(9); !found || value != "x" {
		t.Fatal("inverse Put not visible forward")
	}
	inverse.ForcePut("x", 10)
	key, _ := set.GetKey("x")
	if set.ContainsKey(9) || key != 10 {
		t.Fatal("inverse ForcePut failed")
	}
	if key, found := set.DeleteValue("x"); !found || key != 10 {
		t.Fatal(key, found)
	}
	if inverse.Len() != 0 || set.Len() != 0 {
		t.Fatal("reverse deletion left stale entries")
	}
}

func TestSeededModel(t *testing.T) {
	random := rand.New(rand.NewSource(42))
	actual := NewHashBiMap[int, int]()
	forward := make(map[int]int)
	reverse := make(map[int]int)
	for step := 0; step < 3000; step++ {
		key, value := random.Intn(40), random.Intn(40)
		switch random.Intn(6) {
		case 0:
			err := actual.Put(key, value)
			oldValue, keyFound := forward[key]
			oldKey, valueFound := reverse[value]
			conflict := valueFound && oldKey != key
			if (err != nil) != conflict {
				t.Fatalf("step %d Put error=%v conflict=%v", step, err, conflict)
			}
			if !conflict {
				if keyFound {
					delete(reverse, oldValue)
				}
				forward[key] = value
				reverse[value] = key
			}
		case 1:
			actual.ForcePut(key, value)
			if oldKey, found := reverse[value]; found && oldKey != key {
				delete(forward, oldKey)
			}
			if oldValue, found := forward[key]; found && oldValue != value {
				delete(reverse, oldValue)
			}
			forward[key] = value
			reverse[value] = key
		case 2:
			got, found := actual.DeleteKey(key)
			want, expected := forward[key]
			if got != want || found != expected {
				t.Fatal("DeleteKey mismatch")
			}
			if expected {
				delete(forward, key)
				delete(reverse, want)
			}
		case 3:
			got, found := actual.DeleteValue(value)
			want, expected := reverse[value]
			if got != want || found != expected {
				t.Fatal("DeleteValue mismatch")
			}
			if expected {
				delete(reverse, value)
				delete(forward, want)
			}
		case 4:
			got, found := actual.GetValue(key)
			want, expected := forward[key]
			if got != want || found != expected {
				t.Fatal("GetValue mismatch")
			}
		case 5:
			got, found := actual.GetKey(value)
			want, expected := reverse[value]
			if got != want || found != expected {
				t.Fatal("GetKey mismatch")
			}
		}
		if step%25 == 0 {
			if actual.Len() != len(forward) || len(reverse) != len(forward) {
				t.Fatal("model size mismatch")
			}
			checkInvariant(t, actual)
		}
	}
}

func checkInvariant[K comparable, V comparable](t *testing.T, set *HashBiMap[K, V]) {
	t.Helper()
	count := 0
	for key, value := range set.Entries() {
		count++
		if reverseKey, found := set.GetKey(value); !found || reverseKey != key {
			t.Fatal("missing reverse association")
		}
	}
	if count != set.Len() || set.Inverse().Len() != set.Len() {
		t.Fatal("direction length mismatch")
	}
}

func TestCloneEqualAndJSON(t *testing.T) {
	set := NewHashBiMap[int, string]()
	set.ForcePut(1, "a")
	set.ForcePut(2, "b")
	clone := set.Clone()
	if !set.Equal(clone) || !clone.Equal(set) {
		t.Fatal("clone not equal")
	}
	clone.ForcePut(2, "c")
	if set.Equal(clone) || set.ContainsValue("c") {
		t.Fatal("clone aliases source")
	}
	encoded, err := json.Marshal(set)
	if err != nil {
		t.Fatal(err)
	}
	decoded := NewHashBiMap[int, string]()
	inverse := decoded.Inverse()
	if err := json.Unmarshal(encoded, decoded); err != nil {
		t.Fatal(err)
	}
	if !decoded.Equal(set) || decoded.Inverse() != inverse {
		t.Fatal("JSON replacement changed inverse identity")
	}
	inverse.ForcePut("d", 4)
	if !decoded.ContainsKey(4) {
		t.Fatal("existing inverse detached by JSON")
	}
	before := decoded.EntriesSlice()
	if err := json.Unmarshal([]byte(`[{"key":7,"value":"a"},{"key":8,"value":"a"}]`), decoded); !errors.Is(err, ErrValueConflict) {
		t.Fatal("expected JSON value conflict")
	}
	if !reflect.DeepEqual(before, decoded.EntriesSlice()) {
		t.Fatal("failed JSON changed bimap")
	}
}

func TestCustomFunctions(t *testing.T) {
	type record struct {
		name string
		data []int
	}
	eq := func(a, b record) bool { return strings.EqualFold(a.name, b.name) }
	hash := func(value record) uint64 {
		hash := fnv.New64a()
		_, _ = hash.Write([]byte(strings.ToLower(value.name)))
		return hash.Sum64()
	}
	set := NewHashBiMapFunc[record, record](eq, hash, eq, hash)
	if err := set.Put(record{name: "A", data: []int{1}}, record{name: "B", data: []int{2}}); err != nil {
		t.Fatal(err)
	}
	if _, found := set.GetValue(record{name: "a"}); !found {
		t.Fatal("custom key equality failed")
	}
	if key, found := set.GetKey(record{name: "b"}); !found || key.data[0] != 1 {
		t.Fatal("custom reverse lookup failed")
	}
}
