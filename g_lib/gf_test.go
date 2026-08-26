package g_lib

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"reflect"
	"testing"
)

func TestIfTrue(t *testing.T) {

	x := IfTrue(true, 1, 2)
	if x != 1 {
		t.Errorf("If true failed")
	}

	y := IfTrue(false, "y", "n")
	if y != "n" {
		t.Errorf("If true failed")
	}

}

func TestMinMax(t *testing.T) {

	x := Min[int](3, 4)
	if x != 3 {
		t.Errorf("Min failed")
	}

	x = Min[int](4, 3)
	if x != 3 {
		t.Errorf("Min failed")
	}

	x = Max[int](3, 4)
	if x != 4 {
		t.Errorf("Min failed")
	}

	x = Max[int](4, 3)
	if x != 4 {
		t.Errorf("Min failed")
	}

}

func TestMinMaxArray(t *testing.T) {

	x := MinArray[int]([]int{3, 4, 5})
	if x != 3 {
		t.Errorf("Min failed")
	}

	x = MinArray[int]([]int{5, 4, 3, 4, 5})
	if x != 3 {
		t.Errorf("Min failed")
	}

	x = MaxArray[int]([]int{1, 2, 4, 2, 3})
	if x != 4 {
		t.Errorf("Min failed")
	}

	x = MaxArray[int]([]int{4, 1, 2, 4, 2, 3})
	if x != 4 {
		t.Errorf("Min failed")
	}

}

func TestInArray(t *testing.T) {
	found := InArray[int](42, []int{1, 3, 5, 9, 22, 44, 1, 5, 7, 42, 55})
	if !found {
		t.Errorf("Failed to find when should be found in array")
	}

	found = InArray[int](42, []int{1, 3, 5, 9, 22, 44, 1, 5, 7, 43, 55})
	if found {
		t.Errorf("Found in array when not there")
	}
}

func TestLocationInArray(t *testing.T) {
	loc := LocationInArray[int](42, []int{1, 3, 5, 9, 22, 44, 1, 5, 7, 42, 55})
	if loc != 9 {
		t.Errorf("Incorrect Location, found %d expected 9", loc)
	}

	loc = LocationInArray[int](42, []int{1, 3, 5, 9, 22, 44, 1, 5, 7, 43, 55})
	if loc != -1 {
		t.Errorf("Incorrect Location, should not be found")
	}
}

// func KeysForStringMap[T any](aMap map[string]T) (rv []string ) {
func TestKeyForStringMap(t *testing.T) {
	aMap := map[string]int{
		"abc": 1,
		"def": 3,
	}
	k := KeysForStringMap[int](aMap)
	if len(k) != 2 {
		t.Errorf("Incorrect Key Length")
	}
	if !InArray("abc", k) {
		t.Errorf("Incorrect Key")
	}
	if !InArray("def", k) {
		t.Errorf("Incorrect Key")
	}
}

func TestAbs(t *testing.T) {
	x := Abs[int](-4)
	if x != 4 {
		t.Errorf("Abs failed")
	}

	x = Abs[int](4)
	if x != 4 {
		t.Errorf("Abs failed")
	}

	x = Abs[int](0)
	if x != 0 {
		t.Errorf("Abs failed")
	}
}

func TestMapKeys(t *testing.T) {
	ex := make(map[string]int)
	ex["abc"] = 44
	ex["bob"] = 44
	ex["nope"] = 44
	exKey := GetMapKeys(ex)

	if len(exKey) != 3 {
		t.Errorf("Incorrect Length of Slice, should be 3, got %d", len(exKey))
	}
	if !InArray("bob", exKey) {
		t.Errorf("Failed to find 'bob' in %s", exKey)
	}
	if !InArray("abc", exKey) {
		t.Errorf("Failed to find 'abc' in %s", exKey)
	}
	if !InArray("nope", exKey) {
		t.Errorf("Failed to find 'nope' in %s", exKey)
	}
}

/*
func SortSlice[T constraints.Ordered](s []T) {
	sort.Slice(s, func(i, j int) bool {
		return s[i] < s[j]
	})
}
*/

func TestSort(t *testing.T) {
	ss := []string{"c", "a", "d", "z", "r"}
	SortSlice(ss)
	// fmt.Printf("%s\n", ss)
	sorted := []string{"a", "c", "d", "r", "z"}
	if len(ss) != len(sorted) {
		t.Errorf("Incorrect Length of Slice, should be %d, got %d", len(sorted), len(ss))
	}
	for i := range ss {
		if ss[i] != sorted[i] {
			t.Errorf("Incorrect data , should be ->%s<-, got ->%s<-", sorted[i], ss[i])
		}
	}
}

func TestRemoveAt(t *testing.T) {
	a := []string{"a", "b", "c"}
	b := RemoveAt(a, 0)
	expected := []string{"b", "c"}
	if !reflect.DeepEqual(b, expected) {
		t.Errorf("Incorrect data , expected %v got %v", expected, b)
	}
	if !reflect.DeepEqual(a, []string{"a", "b", "c"}) {
		t.Errorf("Incorrect input data , expected %v got %v", []string{"a", "b", "c"}, a)
	}
	// fmt.Printf("Before a=%v\n", a)
	b = RemoveAt(a, 2)
	expected = []string{"a", "b"}
	if !reflect.DeepEqual(b, expected) {
		t.Errorf("Incorrect data , expected %v got %v", expected, b)
	}
	b = RemoveAt(a, 1)
	expected = []string{"a", "c"}
	if !reflect.DeepEqual(b, expected) {
		t.Errorf("Incorrect data , expected %v got %v", expected, b)
	}
}

func TestRemove(t *testing.T) {
	type item struct {
		a int
		b string
	}
	haystack := []item{{1, "a"}, {2, "b"}, {3, "c"}, {2, "b"}}
	got := Remove(haystack, item{2, "b"})
	expected := []item{{1, "a"}, {3, "c"}}
	if !reflect.DeepEqual(got, expected) {
		t.Errorf("Incorrect data, expected %v got %v", expected, got)
	}
}

func TestRemoveComparable(t *testing.T) {
	got := RemoveComparable([]int{1, 2, 3, 2, 1}, 2)
	expected := []int{1, 3, 1}
	if !reflect.DeepEqual(got, expected) {
		t.Errorf("Incorrect data, expected %v got %v", expected, got)
	}

	got = RemoveComparable([]int{1, 2, 3}, 42)
	expected = []int{1, 2, 3}
	if !reflect.DeepEqual(got, expected) {
		t.Errorf("Incorrect data, expected %v got %v", expected, got)
	}
}

func TestRemoveAtDoesNotMutateInput(t *testing.T) {
	a := []string{"a", "b", "c"}
	b := RemoveAt(a, 1)
	if !reflect.DeepEqual(a, []string{"a", "b", "c"}) {
		t.Errorf("RemoveAt mutated its input, got %v", a)
	}
	if !reflect.DeepEqual(b, []string{"a", "c"}) {
		t.Errorf("Incorrect data, expected %v got %v", []string{"a", "c"}, b)
	}
}

func TestRemoveAtOutOfRange(t *testing.T) {
	a := []int{1, 2, 3}
	if got := RemoveAt(a, -1); !reflect.DeepEqual(got, a) {
		t.Errorf("Expected unchanged slice, got %v", got)
	}
	if got := RemoveAt(a, 3); !reflect.DeepEqual(got, a) {
		t.Errorf("Expected unchanged slice, got %v", got)
	}
}

func TestMinMaxArrayEmpty(t *testing.T) {
	if x := MinArray[int](nil); x != 0 {
		t.Errorf("MinArray of empty slice should be 0, got %d", x)
	}
	if x := MaxArray[int](nil); x != 0 {
		t.Errorf("MaxArray of empty slice should be 0, got %d", x)
	}
}

func TestEqualSlice(t *testing.T) {
	if !EqualSlice([]int{1, 2, 3}, []int{1, 2, 3}) {
		t.Errorf("Equal slices reported unequal")
	}
	if EqualSlice([]int{1, 2, 3}, []int{1, 2, 4}) {
		t.Errorf("Unequal slices reported equal")
	}
	if EqualSlice([]int{1, 2, 3}, []int{1, 2}) {
		t.Errorf("Different length slices reported equal")
	}
}

func TestSortedKeysForStringMap(t *testing.T) {
	m := map[string]int{"z": 1, "a": 2, "m": 3}
	got := SortedKeysForStringMap(m)
	expected := []string{"a", "m", "z"}
	if !reflect.DeepEqual(got, expected) {
		t.Errorf("Incorrect data, expected %v got %v", expected, got)
	}
}

func TestUnique(t *testing.T) {
	got := Unique([]string{"a", "b", "a", "c", "b"})
	expected := []string{"a", "b", "c"}
	if !reflect.DeepEqual(got, expected) {
		t.Errorf("Incorrect data, expected %v got %v", expected, got)
	}
	if got := Unique([]int(nil)); len(got) != 0 {
		t.Errorf("Unique of nil should be empty, got %v", got)
	}
}

func TestToBoolMap(t *testing.T) {
	m := ToBoolMap([]string{"a", "b", "a"})
	if len(m) != 2 || !m["a"] || !m["b"] {
		t.Errorf("Incorrect map, got %v", m)
	}
}

func TestPow(t *testing.T) {
	if x := Pow(2, 10); x != 1024 {
		t.Errorf("Pow(2,10) should be 1024, got %d", x)
	}
	if x := Pow(3, 0); x != 1 {
		t.Errorf("Pow(3,0) should be 1, got %d", x)
	}
	if x := Pow(1.5, 2); x != 2.25 {
		t.Errorf("Pow(1.5,2) should be 2.25, got %v", x)
	}
}

func BenchmarkMin(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = Min(i, b.N)
	}
}

func BenchmarkInArray(b *testing.B) {
	haystack := []int{1, 3, 5, 9, 22, 44, 1, 5, 7, 42, 55}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = InArray(42, haystack)
	}
}

func BenchmarkSortSlice(b *testing.B) {
	base := []int{9, 3, 5, 1, 22, 44, 12, 50, 7, 42, 55}
	s := make([]int, len(base))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		copy(s, base)
		SortSlice(s)
	}
}

func BenchmarkRemoveAt(b *testing.B) {
	base := []int{1, 3, 5, 9, 22, 44, 1, 5, 7, 42, 55}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = RemoveAt(base, 5)
	}
}

func BenchmarkUnique(b *testing.B) {
	s := []int{1, 3, 5, 9, 22, 44, 1, 5, 7, 42, 55}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Unique(s)
	}
}

/* vim: set noai ts=4 sw=4: */
