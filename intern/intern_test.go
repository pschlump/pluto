/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package intern

import (
	"fmt"
	"strings"
	"sync"
	"testing"
)

type record struct {
	id   int
	name string
}

func TestCanonicalIdentityAndLifetime(t *testing.T) {
	interner := NewFunc(
		func(a, b *record) bool { return a.id == b.id },
		func(value *record) uint64 { return uint64(value.id) },
	)
	first := &record{id: 7, name: "first"}
	if got := interner.Intern(first); got != first {
		t.Fatal("first value was not retained")
	}
	for range 100 {
		if got := interner.Intern(&record{id: 7, name: "duplicate"}); got != first {
			t.Fatal("equal value replaced the canonical representative")
		}
	}
	if interner.Len() != 1 || !interner.Contains(&record{id: 7}) {
		t.Fatal("canonical membership mismatch")
	}
	interner.Clear()
	if !interner.IsEmpty() || interner.Contains(&record{id: 7}) {
		t.Fatal("Clear retained a value")
	}
	replacement := &record{id: 7, name: "replacement"}
	if got := interner.Intern(replacement); got != replacement {
		t.Fatal("new canonical value after Clear was not retained")
	}
}

func TestHashCollisionsAndZeroValue(t *testing.T) {
	var zero Interner[int]
	if zero.Len() != 0 || zero.Contains(1) {
		t.Fatal("zero value is not empty")
	}
	zero.Clear()
	interner := NewFunc(func(a, b int) bool { return a == b }, func(int) uint64 { return 1 })
	for value := range 100 {
		if got := interner.Intern(value); got != value {
			t.Fatal(got)
		}
	}
	for value := range 100 {
		if got := interner.Intern(value); got != value || !interner.Contains(value) {
			t.Fatal(value)
		}
	}
	if interner.Len() != 100 {
		t.Fatal(interner.Len())
	}
}

func TestConcurrentCanonicalization(t *testing.T) {
	interner := NewFunc(
		func(a, b *record) bool { return a.id == b.id },
		func(value *record) uint64 { return uint64(value.id) },
	)
	const workers = 32
	canonical := make([]*record, workers)
	var wg sync.WaitGroup
	for worker := range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 1000 {
				canonical[worker] = interner.Intern(&record{id: 42})
			}
		}()
	}
	wg.Wait()
	for _, value := range canonical {
		if value != canonical[0] {
			t.Fatal("concurrent calls returned different representatives")
		}
	}
	if interner.Len() != 1 {
		t.Fatal(interner.Len())
	}
}

func TestStringDeduplication(t *testing.T) {
	interner := New[string]()
	values := make([]string, 1000)
	for index := range values {
		values[index] = interner.Intern(strings.Clone(fmt.Sprintf("value-%d", index%100)))
	}
	if interner.Len() != 100 {
		t.Fatal(interner.Len())
	}
}

func TestInvalidConstructor(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("nil functions did not panic")
		}
	}()
	NewFunc[int](nil, func(value int) uint64 { return uint64(value) })
}

func ExampleInterner() {
	interner := New[string]()
	canonical := interner.Intern(strings.Clone("pluto"))
	duplicate := interner.Intern(strings.Clone("pluto"))
	fmt.Println(canonical, duplicate, interner.Len())
	// Output: pluto pluto 1
}
