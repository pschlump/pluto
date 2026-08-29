package g_lib

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"math"
	"math/rand"
	"reflect"
	"sort"
	"testing"
)

// ---------------------------------------------------------------------------
// Min / Max — edge cases
// ---------------------------------------------------------------------------

func TestMinMaxEdgeCases(t *testing.T) {
	if x := Min(0, 0); x != 0 {
		t.Errorf("Min(0,0) should be 0, got %d", x)
	}
	if x := Max(0, 0); x != 0 {
		t.Errorf("Max(0,0) should be 0, got %d", x)
	}
	if x := Min(-5, -3); x != -5 {
		t.Errorf("Min(-5,-3) should be -5, got %d", x)
	}
	if x := Max(-5, -3); x != -3 {
		t.Errorf("Max(-5,-3) should be -3, got %d", x)
	}
	if x := Min("b", "a"); x != "a" {
		t.Errorf("Min of strings should be \"a\", got %q", x)
	}
	if x := Max("b", "a"); x != "b" {
		t.Errorf("Max of strings should be \"b\", got %q", x)
	}
	if x := Min(2.5, 2.4); x != 2.4 {
		t.Errorf("Min of floats should be 2.4, got %v", x)
	}
	if x := Max(2.5, 2.4); x != 2.5 {
		t.Errorf("Max of floats should be 2.5, got %v", x)
	}
	// User-defined ordered type (constraint uses ~T semantics via cmp.Ordered).
	type celsius float64
	if x := Min(celsius(-40), celsius(10)); x != celsius(-40) {
		t.Errorf("Min of named type should be -40, got %v", x)
	}
}

// ---------------------------------------------------------------------------
// MinArray / MaxArray — single element, negatives, duplicates
// ---------------------------------------------------------------------------

func TestMinMaxArrayMore(t *testing.T) {
	if x := MinArray([]int{7}); x != 7 {
		t.Errorf("MinArray of single element should be 7, got %d", x)
	}
	if x := MaxArray([]int{7}); x != 7 {
		t.Errorf("MaxArray of single element should be 7, got %d", x)
	}
	if x := MinArray([]int{-1, -9, -4}); x != -9 {
		t.Errorf("MinArray of negatives should be -9, got %d", x)
	}
	if x := MaxArray([]int{-1, -9, -4}); x != -1 {
		t.Errorf("MaxArray of negatives should be -1, got %d", x)
	}
	if x := MinArray([]string{"pear", "apple", "apple", "zebra"}); x != "apple" {
		t.Errorf("MinArray of strings should be \"apple\", got %q", x)
	}
	if x := MaxArray([]string{"pear", "apple", "zebra", "zebra"}); x != "zebra" {
		t.Errorf("MaxArray of strings should be \"zebra\", got %q", x)
	}
	if x := MinArray([]float64(nil)); x != 0 {
		t.Errorf("MinArray of empty float slice should be 0, got %v", x)
	}
	if x := MaxArray([]float64(nil)); x != 0 {
		t.Errorf("MaxArray of empty float slice should be 0, got %v", x)
	}
}

// ---------------------------------------------------------------------------
// IfTrue — with composite types
// ---------------------------------------------------------------------------

func TestIfTrueComposite(t *testing.T) {
	type pair struct{ a, b int }
	if got := IfTrue(true, pair{1, 2}, pair{3, 4}); got != (pair{1, 2}) {
		t.Errorf("IfTrue with struct failed, got %v", got)
	}
	if got := IfTrue(false, []int{1}, []int{2, 3}); !reflect.DeepEqual(got, []int{2, 3}) {
		t.Errorf("IfTrue with slice failed, got %v", got)
	}
	var p1, p2 *int
	n1, n2 := 1, 2
	p1, p2 = &n1, &n2
	if got := IfTrue(true, p1, p2); got != p1 {
		t.Errorf("IfTrue with pointer failed, got %v", got)
	}
}

// ---------------------------------------------------------------------------
// InArray / LocationInArray — boundaries, duplicates, empty
// ---------------------------------------------------------------------------

func TestInArrayEdgeCases(t *testing.T) {
	if InArray(1, []int(nil)) {
		t.Errorf("InArray of nil slice should be false")
	}
	if InArray(1, []int{}) {
		t.Errorf("InArray of empty slice should be false")
	}
	if !InArray("only", []string{"only"}) {
		t.Errorf("InArray of single-element slice should find the element")
	}
}

func TestLocationInArrayEdgeCases(t *testing.T) {
	if loc := LocationInArray(1, []int(nil)); loc != -1 {
		t.Errorf("LocationInArray of nil slice should be -1, got %d", loc)
	}
	// First element.
	if loc := LocationInArray(9, []int{9, 1, 2}); loc != 0 {
		t.Errorf("LocationInArray should be 0, got %d", loc)
	}
	// Last element.
	if loc := LocationInArray(9, []int{1, 2, 9}); loc != 2 {
		t.Errorf("LocationInArray should be 2, got %d", loc)
	}
	// Duplicates: must return the FIRST occurrence.
	if loc := LocationInArray(5, []int{5, 1, 5, 5}); loc != 0 {
		t.Errorf("LocationInArray with duplicates should return first index 0, got %d", loc)
	}
}

// ---------------------------------------------------------------------------
// KeysForStringMap / GetMapKeys / SortedKeysForStringMap — empty & nil maps
// ---------------------------------------------------------------------------

func TestKeysForStringMapEmpty(t *testing.T) {
	if k := KeysForStringMap(map[string]int{}); len(k) != 0 {
		t.Errorf("Keys of empty map should be empty, got %v", k)
	}
	if k := KeysForStringMap[int](nil); len(k) != 0 {
		t.Errorf("Keys of nil map should be empty, got %v", k)
	}
	// Single entry.
	k := KeysForStringMap(map[string]string{"k": "v"})
	if len(k) != 1 || k[0] != "k" {
		t.Errorf("Keys of single-entry map should be [k], got %v", k)
	}
}

func TestGetMapKeysEmptyAndNonString(t *testing.T) {
	if k := GetMapKeys(map[int]string{}); len(k) != 0 {
		t.Errorf("Keys of empty map should be empty, got %v", k)
	}
	if k := GetMapKeys[int, string](nil); len(k) != 0 {
		t.Errorf("Keys of nil map should be empty, got %v", k)
	}
	k := GetMapKeys(map[int]string{3: "c", 1: "a", 2: "b"})
	sort.Ints(k)
	if !reflect.DeepEqual(k, []int{1, 2, 3}) {
		t.Errorf("GetMapKeys with int keys failed, got %v", k)
	}
}

func TestSortedKeysForStringMapMore(t *testing.T) {
	if k := SortedKeysForStringMap(map[string]int{}); len(k) != 0 {
		t.Errorf("Sorted keys of empty map should be empty, got %v", k)
	}
	if k := SortedKeysForStringMap[int](nil); len(k) != 0 {
		t.Errorf("Sorted keys of nil map should be empty, got %v", k)
	}
	// Already-sorted input stays sorted; duplicate-free.
	k := SortedKeysForStringMap(map[string]bool{"a": true, "b": true, "c": true})
	if !reflect.DeepEqual(k, []string{"a", "b", "c"}) {
		t.Errorf("Sorted keys failed, got %v", k)
	}
}

// ---------------------------------------------------------------------------
// Abs — floats, typed integers, documented overflow
// ---------------------------------------------------------------------------

func TestAbsMore(t *testing.T) {
	if x := Abs(-4.5); x != 4.5 {
		t.Errorf("Abs(-4.5) should be 4.5, got %v", x)
	}
	if x := Abs(4.5); x != 4.5 {
		t.Errorf("Abs(4.5) should be 4.5, got %v", x)
	}
	if x := Abs(int8(-12)); x != int8(12) {
		t.Errorf("Abs(int8(-12)) should be 12, got %v", x)
	}
	if x := Abs(int64(-1) << 40); x != int64(1)<<40 {
		t.Errorf("Abs of large negative int64 failed, got %v", x)
	}
	// Documented behavior: Abs(math.MinInt64) overflows and stays negative.
	if x := Abs(int64(math.MinInt64)); x >= 0 {
		t.Errorf("Abs(math.MinInt64) is documented to stay negative, got %v", x)
	}
}

// ---------------------------------------------------------------------------
// SortSlice — empty, single, sorted, reversed, duplicates
// ---------------------------------------------------------------------------

func TestSortSliceMore(t *testing.T) {
	empty := []int{}
	SortSlice(empty)
	if len(empty) != 0 {
		t.Errorf("SortSlice of empty slice should stay empty, got %v", empty)
	}
	single := []int{42}
	SortSlice(single)
	if !reflect.DeepEqual(single, []int{42}) {
		t.Errorf("SortSlice of single element failed, got %v", single)
	}
	sorted := []int{1, 2, 3}
	SortSlice(sorted)
	if !reflect.DeepEqual(sorted, []int{1, 2, 3}) {
		t.Errorf("SortSlice of sorted slice failed, got %v", sorted)
	}
	rev := []int{5, 4, 3, 2, 1}
	SortSlice(rev)
	if !reflect.DeepEqual(rev, []int{1, 2, 3, 4, 5}) {
		t.Errorf("SortSlice of reversed slice failed, got %v", rev)
	}
	dup := []int{3, 1, 3, 1, 2}
	SortSlice(dup)
	if !reflect.DeepEqual(dup, []int{1, 1, 2, 3, 3}) {
		t.Errorf("SortSlice with duplicates failed, got %v", dup)
	}
}

// ---------------------------------------------------------------------------
// EqualSlice — nil vs empty, strings
// ---------------------------------------------------------------------------

func TestEqualSliceMore(t *testing.T) {
	if !EqualSlice([]int(nil), []int(nil)) {
		t.Errorf("Two nil slices should be equal")
	}
	// nil and empty are both length 0 — EqualSlice treats them as equal.
	if !EqualSlice([]int(nil), []int{}) {
		t.Errorf("nil and empty slices should be equal by length")
	}
	if !EqualSlice([]string{"a", "b"}, []string{"a", "b"}) {
		t.Errorf("Equal string slices reported unequal")
	}
	if EqualSlice([]string{"a", "b"}, []string{"b", "a"}) {
		t.Errorf("Reordered slices reported equal")
	}
	if EqualSlice([]int(nil), []int{1}) {
		t.Errorf("nil and non-empty slice reported equal")
	}
}

// ---------------------------------------------------------------------------
// RemoveAt — pos == len, single-element, two-element boundaries
// ---------------------------------------------------------------------------

func TestRemoveAtMore(t *testing.T) {
	a := []int{1, 2, 3}
	if got := RemoveAt(a, len(a)); !reflect.DeepEqual(got, a) {
		t.Errorf("RemoveAt at pos==len should return slice unchanged, got %v", got)
	}
	one := []int{9}
	if got := RemoveAt(one, 0); len(got) != 0 {
		t.Errorf("RemoveAt on single-element slice should be empty, got %v", got)
	}
	// Removing from the result must not alias back into the input's tail.
	b := RemoveAt(a, 0)
	if !reflect.DeepEqual(a, []int{1, 2, 3}) {
		t.Errorf("RemoveAt mutated its input, got %v", a)
	}
	if !reflect.DeepEqual(b, []int{2, 3}) {
		t.Errorf("RemoveAt(a,0) should be [2 3], got %v", b)
	}
}

// ---------------------------------------------------------------------------
// Remove — non-comparable element types, empty, no-match
// ---------------------------------------------------------------------------

func TestRemoveMore(t *testing.T) {
	// Elements that are themselves slices only work with reflect.DeepEqual.
	haystack := [][]int{{1, 2}, {3}, {1, 2}}
	got := Remove(haystack, []int{1, 2})
	if !reflect.DeepEqual(got, [][]int{{3}}) {
		t.Errorf("Remove with slice elements failed, got %v", got)
	}
	// No match returns an equal copy of the input.
	got2 := Remove([]int{1, 2, 3}, 42)
	if !reflect.DeepEqual(got2, []int{1, 2, 3}) {
		t.Errorf("Remove with no match should keep all items, got %v", got2)
	}
	// Empty input.
	if got := Remove([]int{}, 1); len(got) != 0 {
		t.Errorf("Remove of empty slice should be empty, got %v", got)
	}
	// Remove everything.
	if got := Remove([]int{7, 7, 7}, 7); len(got) != 0 {
		t.Errorf("Remove of all-matching slice should be empty, got %v", got)
	}
}

func TestRemoveComparableMore(t *testing.T) {
	if got := RemoveComparable([]string{}, "x"); len(got) != 0 {
		t.Errorf("RemoveComparable of empty slice should be empty, got %v", got)
	}
	if got := RemoveComparable([]string{"x", "x"}, "x"); len(got) != 0 {
		t.Errorf("RemoveComparable of all-matching slice should be empty, got %v", got)
	}
}

// ---------------------------------------------------------------------------
// Unique — all duplicates, no duplicates, order preservation
// ---------------------------------------------------------------------------

func TestUniqueMore(t *testing.T) {
	if got := Unique([]int{5, 5, 5, 5}); !reflect.DeepEqual(got, []int{5}) {
		t.Errorf("Unique of all-same slice should be [5], got %v", got)
	}
	if got := Unique([]int{1, 2, 3}); !reflect.DeepEqual(got, []int{1, 2, 3}) {
		t.Errorf("Unique of distinct slice should be unchanged, got %v", got)
	}
	// First-occurrence order, not sorted order.
	if got := Unique([]int{3, 1, 3, 2, 1}); !reflect.DeepEqual(got, []int{3, 1, 2}) {
		t.Errorf("Unique should preserve first-occurrence order, got %v", got)
	}
}

// ---------------------------------------------------------------------------
// ToBoolMap — nil input, duplicates collapse
// ---------------------------------------------------------------------------

func TestToBoolMapMore(t *testing.T) {
	m := ToBoolMap([]int(nil))
	if m == nil || len(m) != 0 {
		t.Errorf("ToBoolMap of nil slice should be an empty non-nil map, got %v", m)
	}
	m = ToBoolMap([]int{1, 1, 1})
	if len(m) != 1 || !m[1] {
		t.Errorf("ToBoolMap with duplicates should collapse, got %v", m)
	}
}

// ---------------------------------------------------------------------------
// Pow — negative exponent (documented), exponent 1, base 0, floats
// ---------------------------------------------------------------------------

func TestPowMore(t *testing.T) {
	// Documented: negative exponent returns T(1) for integer types.
	if x := Pow(5, -3); x != 1 {
		t.Errorf("Pow(5,-3) should be 1, got %d", x)
	}
	if x := Pow(7, 1); x != 7 {
		t.Errorf("Pow(7,1) should be 7, got %d", x)
	}
	if x := Pow(0, 5); x != 0 {
		t.Errorf("Pow(0,5) should be 0, got %d", x)
	}
	if x := Pow(0, 0); x != 1 {
		t.Errorf("Pow(0,0) should be 1, got %d", x)
	}
	if x := Pow(-2, 3); x != -8 {
		t.Errorf("Pow(-2,3) should be -8, got %d", x)
	}
	if x := Pow(-2, 4); x != 16 {
		t.Errorf("Pow(-2,4) should be 16, got %d", x)
	}
	if x := Pow(uint8(2), 8); x != uint8(0) { // 2^8 overflows uint8 to 0
		t.Errorf("Pow(uint8(2),8) should overflow to 0, got %d", x)
	}
	if x := Pow(0.5, 3); x != 0.125 {
		t.Errorf("Pow(0.5,3) should be 0.125, got %v", x)
	}
}

// ---------------------------------------------------------------------------
// Ptr — copy semantics
// ---------------------------------------------------------------------------

func TestPtr(t *testing.T) {
	v := 42
	p := Ptr(v)
	if p == nil {
		t.Fatalf("Ptr returned nil")
	}
	if *p != 42 {
		t.Errorf("*Ptr(42) should be 42, got %d", *p)
	}
	// The pointer is to a copy made at call time: mutating the original
	// afterwards must not change it.
	v = 7
	if *p != 42 {
		t.Errorf("*Ptr should be independent of later changes to v, got %d", *p)
	}
	// And mutating through the pointer must not write back to v.
	*p = 99
	if v != 7 {
		t.Errorf("writing through Ptr(v) must not change v, got %d", v)
	}
}

func TestPtrComposite(t *testing.T) {
	type item struct {
		a int
		b string
	}
	it := item{1, "x"}
	p := Ptr(it)
	it.a = 2 // mutate the original
	if *p != (item{1, "x"}) {
		t.Errorf("Ptr of struct should point at a copy, got %v", *p)
	}
	p.b = "y"
	if it.b != "x" {
		t.Errorf("writing through Ptr must not change the original, got %q", it.b)
	}

	s := []int{1, 2}
	sp := Ptr(s)
	if len(*sp) != 2 || (*sp)[0] != 1 {
		t.Errorf("Ptr of slice failed, got %v", *sp)
	}
	// Each call yields a distinct pointer.
	p1, p2 := Ptr(s), Ptr(s)
	if p1 == p2 {
		t.Errorf("Two Ptr calls returned the same pointer")
	}
}

func TestPtrZeroValue(t *testing.T) {
	var zero int
	p := Ptr(zero)
	if *p != 0 {
		t.Errorf("Ptr of zero value should be 0, got %d", *p)
	}
	type big struct{ a, b, c [16]byte }
	seed := big{a: [16]byte{1}, b: [16]byte{2}, c: [16]byte{3}}
	bp := Ptr(seed)
	if *bp != seed {
		t.Errorf("Ptr of struct failed, got %v want %v", *bp, seed)
	}
}

// ---------------------------------------------------------------------------
// Numeric constraints — instantiation across the type sets
// ---------------------------------------------------------------------------

// Named (defined) types: accepted by the ~T constraints, not by Number.
type (
	myInt     int
	myInt8    int8
	myUint    uint
	myFloat32 float32
)

// accept* are tiny generic probes: a type is in the constraint's type
// set if (and only if) the instantiation compiles.
func acceptNumeric[T Numeric](T)         {}
func acceptSigned[T SignedInteger](T)    {}
func acceptSignedNum[T SignedNumeric](T) {}
func acceptUnsigned[T Unsigned](T)       {}
func acceptNumber[T Number](T)           {}

// TestConstraintTypeSets exercises every member of each numeric
// constraint's type set (and the ~T acceptance of defined types).  A
// type outside a set fails at compile time, which is the test — note
// that Pow's Number constraint has no ~T, so defined types are rejected
// there (Number's doc says so).
func TestConstraintTypeSets(t *testing.T) {
	v := 1
	acceptNumeric(v)
	acceptSigned(v)
	acceptSignedNum(v)
	acceptNumber(v)
	acceptNumeric(int8(v))
	acceptSigned(int8(v))
	acceptSignedNum(int8(v))
	acceptNumber(int8(v))
	acceptNumeric(int16(v))
	acceptSigned(int16(v))
	acceptSignedNum(int16(v))
	acceptNumber(int16(v))
	acceptNumeric(int32(v))
	acceptSigned(int32(v))
	acceptSignedNum(int32(v))
	acceptNumber(int32(v))
	acceptNumeric(int64(v))
	acceptSigned(int64(v))
	acceptSignedNum(int64(v))
	acceptNumber(int64(v))
	acceptNumeric(uint(v))
	acceptUnsigned(uint(v))
	acceptNumber(uint(v))
	acceptNumeric(uint8(v))
	acceptUnsigned(uint8(v))
	acceptNumber(uint8(v))
	acceptNumeric(uint16(v))
	acceptUnsigned(uint16(v))
	acceptNumber(uint16(v))
	acceptNumeric(uint32(v))
	acceptUnsigned(uint32(v))
	acceptNumber(uint32(v))
	acceptNumeric(uint64(v))
	acceptUnsigned(uint64(v))
	acceptNumber(uint64(v))
	acceptNumeric(float32(v))
	acceptSignedNum(float32(v))
	acceptNumber(float32(v))
	acceptNumeric(float64(v))
	acceptSignedNum(float64(v))
	acceptNumber(float64(v))

	// uintptr is a member of Unsigned only.
	acceptUnsigned(uintptr(v))

	// Defined types pass the ~T constraints…
	acceptNumeric(myInt(v))
	acceptSigned(myInt(v))
	acceptSignedNum(myInt(v))
	acceptNumeric(myInt8(v))
	acceptSigned(myInt8(v))
	acceptSignedNum(myInt8(v))
	acceptNumeric(myUint(v))
	acceptUnsigned(myUint(v))
	acceptNumeric(myFloat32(v))
	acceptSignedNum(myFloat32(v))

	// …and the functions parameterized with ~T constraints take them.
	if got := Abs(myInt(-3)); got != 3 {
		t.Errorf("Abs(myInt(-3)) should be 3, got %d", got)
	}
	if got := Min(myInt(2), myInt(1)); got != 1 {
		t.Errorf("Min of myInt failed, got %d", got)
	}
	if got := MinArray([]myFloat32{2.5, 1.5}); got != 1.5 {
		t.Errorf("MinArray of myFloat32 failed, got %v", got)
	}
}

// ---------------------------------------------------------------------------
// Randomized property test — fixed seed, cross-checked against reference
// models (sorted slice, map, naive loops) over hundreds of mixed operations.
// ---------------------------------------------------------------------------

func TestPropertyRandomizedFixedSeed(t *testing.T) {
	rng := rand.New(rand.NewSource(42))

	refMin := func(a []int) int {
		m := a[0]
		for _, v := range a {
			if v < m {
				m = v
			}
		}
		return m
	}
	refMax := func(a []int) int {
		m := a[0]
		for _, v := range a {
			if v > m {
				m = v
			}
		}
		return m
	}

	for iter := range 500 {
		n := rng.Intn(50)
		a := make([]int, n)
		for i := range a {
			a[i] = rng.Intn(40) - 20 // duplicates are likely
		}

		// MinArray / MaxArray agree with a naive reference scan.
		if n > 0 {
			if got, want := MinArray(a), refMin(a); got != want {
				t.Fatalf("iter %d: MinArray(%v) = %d, want %d", iter, a, got, want)
			}
			if got, want := MaxArray(a), refMax(a); got != want {
				t.Fatalf("iter %d: MaxArray(%v) = %d, want %d", iter, a, got, want)
			}
		} else {
			if got := MinArray(a); got != 0 {
				t.Fatalf("iter %d: MinArray of empty = %d, want 0", iter, got)
			}
			if got := MaxArray(a); got != 0 {
				t.Fatalf("iter %d: MaxArray of empty = %d, want 0", iter, got)
			}
		}

		// Min/Max on random pairs agree with each other and with MinArray.
		if n >= 2 {
			lo, hi := Min(a[0], a[1]), Max(a[0], a[1])
			if lo > hi {
				t.Fatalf("iter %d: Min > Max for pair (%d, %d)", iter, a[0], a[1])
			}
			if got := MinArray(a[:2]); got != lo {
				t.Fatalf("iter %d: MinArray of pair = %d, want %d", iter, got, lo)
			}
		}

		// SortSlice produces a non-decreasing permutation of the input.
		orig := append([]int(nil), a...)
		SortSlice(a)
		if !sort.IntsAreSorted(a) {
			t.Fatalf("iter %d: SortSlice result not sorted: %v", iter, a)
		}
		// A permutation preserves the set of elements.
		if !reflect.DeepEqual(ToBoolMap(a), ToBoolMap(orig)) {
			t.Fatalf("iter %d: SortSlice changed the element set: %v from %v", iter, a, orig)
		}
		for _, v := range a {
			if !InArray(v, orig) {
				t.Fatalf("iter %d: SortSlice introduced %d not in input %v", iter, v, orig)
			}
		}
		if len(a) != len(orig) {
			t.Fatalf("iter %d: SortSlice changed length from %d to %d", iter, len(orig), len(a))
		}

		// InArray / LocationInArray agree with a map-based membership model.
		model := make(map[int]bool, n)
		for _, v := range orig {
			model[v] = true
		}
		probe := rng.Intn(44) - 22
		if got, want := InArray(probe, orig), model[probe]; got != want {
			t.Fatalf("iter %d: InArray(%d) = %v, want %v", iter, probe, got, want)
		}
		loc := LocationInArray(probe, orig)
		if loc == -1 {
			if model[probe] {
				t.Fatalf("iter %d: LocationInArray(%d) = -1 but element present", iter, probe)
			}
		} else {
			if orig[loc] != probe {
				t.Fatalf("iter %d: LocationInArray(%d) = %d but a[%d] = %d", iter, probe, loc, loc, orig[loc])
			}
			for i := range loc {
				if orig[i] == probe {
					t.Fatalf("iter %d: LocationInArray(%d) = %d is not the first occurrence", iter, probe, loc)
				}
			}
		}

		// RemoveAt at a valid position matches manual splicing and does not
		// mutate its input.
		if n > 0 {
			pos := rng.Intn(n)
			before := append([]int(nil), orig...)
			got := RemoveAt(orig, pos)
			want := append(append([]int(nil), orig[:pos]...), orig[pos+1:]...)
			if !EqualSlice(got, want) {
				t.Fatalf("iter %d: RemoveAt(pos=%d) = %v, want %v", iter, pos, got, want)
			}
			if !EqualSlice(orig, before) {
				t.Fatalf("iter %d: RemoveAt mutated input: was %v now %v", iter, before, orig)
			}
		}

		// RemoveComparable removes exactly the matching elements.
		got := RemoveComparable(orig, probe)
		for _, v := range got {
			if v == probe {
				t.Fatalf("iter %d: RemoveComparable left %d in %v", iter, probe, got)
			}
		}
		wantLen := 0
		for _, v := range orig {
			if v != probe {
				wantLen++
			}
		}
		if len(got) != wantLen {
			t.Fatalf("iter %d: RemoveComparable len = %d, want %d", iter, len(got), wantLen)
		}

		// Unique preserves first-occurrence order and produces a set.
		u := Unique(orig)
		seen := make(map[int]bool, len(u))
		for _, v := range u {
			if seen[v] {
				t.Fatalf("iter %d: Unique returned duplicate %d in %v", iter, v, u)
			}
			seen[v] = true
		}
		if !reflect.DeepEqual(ToBoolMap(u), ToBoolMap(orig)) {
			t.Fatalf("iter %d: Unique lost elements: %v from %v", iter, u, orig)
		}
		// Every element of u appears in orig at the same relative order.
		idx := 0
		for _, v := range orig {
			if idx < len(u) && v == u[idx] {
				idx++
			}
		}
		if idx != len(u) {
			t.Fatalf("iter %d: Unique order %v is not a subsequence of %v", iter, u, orig)
		}

		// EqualSlice reflexivity and mismatch detection.
		if !EqualSlice(orig, append([]int(nil), orig...)) {
			t.Fatalf("iter %d: EqualSlice reflexivity failed", iter)
		}
		if n > 0 {
			diff := append([]int(nil), orig...)
			diff[rng.Intn(n)] += 1000
			if EqualSlice(orig, diff) {
				t.Fatalf("iter %d: EqualSlice missed a difference", iter)
			}
		}
	}
}

// Pow cross-check against math.Pow for float64, and against repeated
// multiplication for ints, with a fixed seed.
func TestPowPropertyFixedSeed(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	for iter := range 300 {
		base := rng.Intn(13) - 6
		exp := rng.Intn(10) // 0..9 keeps results small
		want := 1
		for range exp {
			want *= base
		}
		if got := Pow(base, exp); got != want {
			t.Fatalf("iter %d: Pow(%d, %d) = %d, want %d", iter, base, exp, got, want)
		}
	}
	for iter := range 300 {
		base := rng.Float64()*8 - 4
		exp := rng.Intn(8)
		got := Pow(base, exp)
		want := math.Pow(base, float64(exp))
		if math.Abs(got-want) > 1e-9*math.Max(1, math.Abs(want)) {
			t.Fatalf("iter %d: Pow(%v, %d) = %v, want %v", iter, base, exp, got, want)
		}
	}
}
