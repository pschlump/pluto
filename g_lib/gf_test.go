package g_lib

/*
Copyright (C) Philip Schlump, 2012-2021.

BSD 3 Clause Licensed.
*/

import (
	"reflect"
	"testing"
)

func TestIfTrue(t *testing.T) {

	x := IfTrue(true, 1, 2)
	if x != 1 {
		t.Errorf("If true falied")
	}

	y := IfTrue(false, "y", "n")
	if y != "n" {
		t.Errorf("If true falied")
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
		t.Errorf("Failed to find when shoudl be found in arary")
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
	for i := 0; i < len(ss); i++ {
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
}

func TestRemoveComparable(t *testing.T) {
}

/* vim: set noai ts=4 sw=4: */
