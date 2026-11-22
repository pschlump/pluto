/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package bimap_ts

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestConcurrentForwardAndInverse(t *testing.T) {
	set := NewHashBiMap[int, string]()
	const writers = 8
	var wait sync.WaitGroup
	for index := 0; index < writers; index++ {
		wait.Add(2)
		go func(key int) {
			defer wait.Done()
			set.ForcePut(key, fmt.Sprintf("value-%d", key))
		}(index)
		go func(key int) {
			defer wait.Done()
			set.Inverse().ForcePut(fmt.Sprintf("inverse-%d", key), key+100)
		}(index)
	}
	wait.Wait()
	if set.Len() != 2*writers || set.Inverse().Len() != 2*writers {
		t.Fatal(set.Len(), set.Inverse().Len())
	}
	for index := 0; index < writers; index++ {
		if value, found := set.GetValue(index); !found || value != fmt.Sprintf("value-%d", index) {
			t.Fatal("forward association missing")
		}
		if key, found := set.GetKey(fmt.Sprintf("inverse-%d", index)); !found || key != index+100 {
			t.Fatal("inverse association missing")
		}
	}
}

func TestInverseIdentityAndDeletion(t *testing.T) {
	set := NewHashBiMap[int, string]()
	if set.Inverse().Inverse() != set {
		t.Fatal("inverse identity failed")
	}
	set.Inverse().ForcePut("a", 1)
	if !set.ContainsKey(1) {
		t.Fatal("inverse mutation not visible")
	}
	if key, found := set.Inverse().DeleteKey("a"); !found || key != 1 || set.Len() != 0 {
		t.Fatal("inverse deletion failed")
	}
}

func TestSnapshotIterationAndClone(t *testing.T) {
	set := NewHashBiMap[int, string]()
	set.ForcePut(1, "a")
	set.ForcePut(2, "b")
	entries := set.Entries()
	clone := set.Clone()
	set.Clear()
	count := 0
	for range entries {
		count++
	}
	if count != 2 || !clone.ContainsKey(2) || clone.Len() != 2 {
		t.Fatal("snapshot or clone failed")
	}
	clone.Inverse().ForcePut("c", 3)
	if set.ContainsKey(3) {
		t.Fatal("clone shares indexes")
	}
}

func TestOppositeEqualityDoesNotDeadlock(t *testing.T) {
	a := NewHashBiMap[int, int]()
	b := NewHashBiMap[int, int]()
	a.ForcePut(1, 2)
	b.ForcePut(2, 1)
	done := make(chan struct{})
	go func() {
		for range 100 {
			_ = a.Equal(b)
			_ = b.Equal(a)
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("opposite equality deadlocked")
	}
}

func TestJSONPreservesInverse(t *testing.T) {
	set := NewHashBiMap[int, string]()
	inverse := set.Inverse()
	if err := json.Unmarshal([]byte(`[{"key":1,"value":"a"}]`), set); err != nil {
		t.Fatal(err)
	}
	if set.Inverse() != inverse {
		t.Fatal("inverse identity changed")
	}
	inverse.ForcePut("b", 2)
	if !set.ContainsKey(2) {
		t.Fatal("inverse detached after JSON")
	}
	encoded, err := json.Marshal(set)
	if err != nil {
		t.Fatal(err)
	}
	decoded := NewHashBiMap[int, string]()
	if err := json.Unmarshal(encoded, decoded); err != nil || !decoded.Equal(set) {
		t.Fatal("JSON round trip failed", err)
	}
}
