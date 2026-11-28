/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package intern

import (
	"math/rand"
	"strings"
	"testing"
	"unsafe"
)

// tagged values are equal by key only; tag identifies which equal value was
// stored first, so canonical identity is observable without pointers.
type tagged struct {
	key int
	tag int
}

func taggedFlavors() map[string]func() *Interner[tagged] {
	eq := func(a, b tagged) bool { return a.key == b.key }
	return map[string]func() *Interner[tagged]{
		"identity": func() *Interner[tagged] { return NewFunc(eq, func(v tagged) uint64 { return uint64(v.key) }) },
		"mod5":     func() *Interner[tagged] { return NewFunc(eq, func(v tagged) uint64 { return uint64(v.key % 5) }) },
		"zero":     func() *Interner[tagged] { return NewFunc(eq, func(tagged) uint64 { return 0 }) },
		"zeroOrOne": func() *Interner[tagged] {
			// hash 0 is stored as 1: both must stay distinct classes by equality.
			return NewFunc(eq, func(v tagged) uint64 { return uint64(v.key & 1) })
		},
	}
}

func TestTableDifferentialModel(t *testing.T) {
	for name, construct := range taggedFlavors() {
		t.Run(name, func(t *testing.T) {
			random := rand.New(rand.NewSource(1127))
			interner := construct()
			first := map[int]int{} // key -> tag of the canonical value
			for step := range 20000 {
				value := tagged{key: random.Intn(400), tag: step}
				switch op := random.Intn(100); {
				case op < 60:
					got := interner.Intern(value)
					want, existed := first[value.key]
					if !existed {
						want = value.tag
						first[value.key] = want
					}
					if got.key != value.key || got.tag != want {
						t.Fatalf("step %d Intern(%+v)=%+v want tag %d", step, value, got, want)
					}
				case op < 99:
					_, existed := first[value.key]
					if interner.Contains(value) != existed {
						t.Fatalf("step %d Contains(%d)=%v", step, value.key, !existed)
					}
				default:
					interner.Clear()
					clear(first)
				}
				if interner.Len() != len(first) {
					t.Fatalf("step %d Len=%d want %d", step, interner.Len(), len(first))
				}
				if step%97 == 0 && interner.length*4 > len(interner.slots)*3 {
					t.Fatalf("step %d: load factor above 3/4", step)
				}
			}
		})
	}
}

func TestNativeDifferentialModel(t *testing.T) {
	random := rand.New(rand.NewSource(271))
	interner := New[string]()
	first := map[string]string{}
	for step := range 20000 {
		value := strings.Repeat("k", 1+random.Intn(300))
		switch op := random.Intn(100); {
		case op < 60:
			got := interner.Intern(strings.Clone(value))
			want, existed := first[value]
			if !existed {
				want = got
				first[value] = got
			}
			if got != value || !sameString(got, want) {
				t.Fatalf("step %d: Intern returned a non-canonical string", step)
			}
		case op < 99:
			_, existed := first[value]
			if interner.Contains(value) != existed {
				t.Fatalf("step %d Contains mismatch", step)
			}
		default:
			interner.Clear()
			clear(first)
		}
		if interner.Len() != len(first) {
			t.Fatalf("step %d Len=%d want %d", step, interner.Len(), len(first))
		}
	}
}

// sameString reports whether a and b share one backing array (identity, not
// just equal contents).
func sameString(a, b string) bool {
	return len(a) == len(b) && unsafe.StringData(a) == unsafe.StringData(b)
}

func TestHitAllocations(t *testing.T) {
	native := New[string]()
	table := NewFunc(func(a, b string) bool { return a == b }, func(v string) uint64 { return uint64(len(v)) })
	for _, interner := range []*Interner[string]{native, table} {
		for _, value := range benchmarkValues[:100] {
			interner.Intern(value)
		}
		probe := strings.Clone(benchmarkValues[42])
		allocs := testing.AllocsPerRun(1000, func() {
			if interner.Intern(probe) != probe || !interner.Contains(probe) || interner.Contains("absent") {
				t.Fatal("lost value")
			}
		})
		if allocs != 0 {
			t.Errorf("Intern hit / Contains allocated %.1f per run", allocs)
		}
	}
}
