/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package multimap

import (
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func TestValuesAndMapViewsWriteThrough(t *testing.T) {
	m := NewListMultimap[string, int]()
	view := m.GetView("a")
	if !view.IsEmpty() {
		t.Fatal("new view is not empty")
	}
	view.Add(1)
	view.AddAll([]int{2, 3})
	if got, found := m.Get("a"); !found || !reflect.DeepEqual(got, []int{1, 2, 3}) {
		t.Fatal(got, found)
	}
	m.Put("a", 4)
	if view.Len() != 4 || !view.Contains(4) {
		t.Fatal("parent mutation not visible")
	}
	mapView := m.AsMapView()
	mapView.Put("b", 9)
	if !m.ContainsEntry("b", 9) || mapView.KeyLen() != 2 {
		t.Fatal("map view write failed")
	}
	view.Remove(2)
	if got, _ := m.Get("a"); !reflect.DeepEqual(got, []int{1, 3, 4}) {
		t.Fatal(got)
	}
	view.Clear()
	if m.ContainsKey("a") {
		t.Fatal("empty key retained")
	}
	view.Add(5)
	if got, _ := m.Get("a"); !reflect.DeepEqual(got, []int{5}) {
		t.Fatal("view stopped tracking re-created key", got)
	}
}

func TestViewsAcrossVariants(t *testing.T) {
	set := NewSetMultimap[int, int]()
	view := set.GetView(1)
	if !view.Add(2) || view.Add(2) {
		t.Fatal("set view duplicates changed")
	}
	sorted := NewSortedSetMultimap[int, int]()
	sortedView := sorted.GetView(1)
	sortedView.AddAll([]int{3, 1, 2})
	if got := slices.Collect(sortedView.All()); !reflect.DeepEqual(got, []int{1, 2, 3}) {
		t.Fatal(got)
	}
	linked := NewLinkedHashMultimap[int, int]()
	linkedView := linked.AsMapView()
	linkedView.Put(2, 3)
	linkedView.Put(1, 4)
	if got := slices.Collect(linkedView.KeySet()); !reflect.DeepEqual(got, []int{2, 1}) {
		t.Fatal(got)
	}
}

func ExampleValuesView() {
	m := NewListMultimap[string, int]()
	values := m.GetView("numbers")
	values.AddAll([]int{1, 2, 3})
	m.Put("numbers", 4)
	fmt.Println(slices.Collect(values.All()))
	// Output: [1 2 3 4]
}
