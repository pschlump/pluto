package dll_ts

/*
Copyright (C) Philip Schlump, 2012-2024.

BSD 3 Clause Licensed.
*/

import (
	"testing"
)

func buildList(items ...string) (rv Dll[TestDemo]) {
	for _, s := range items {
		v := TestDemo{S: s}
		rv.AppendAtTail(&v)
	}
	return
}

func TestRangeOverFunc(t *testing.T) {

	Dll1 := buildList("01", "02", "03")

	// All — head to tail with index.
	var got []string
	var idx []int
	for i, v := range Dll1.All() {
		idx = append(idx, i)
		got = append(got, v.S)
	}
	if len(got) != 3 || got[0] != "01" || got[1] != "02" || got[2] != "03" {
		t.Errorf("All: unexpected values %v", got)
	}
	if len(idx) != 3 || idx[0] != 0 || idx[1] != 1 || idx[2] != 2 {
		t.Errorf("All: unexpected indexes %v", idx)
	}

	// Early break.
	count := 0
	for range Dll1.All() {
		count++
		break
	}
	if count != 1 {
		t.Errorf("All with break: expected 1 iteration, got %d", count)
	}

	// Backward — tail to head with index.
	got = nil
	idx = nil
	for i, v := range Dll1.Backward() {
		idx = append(idx, i)
		got = append(got, v.S)
	}
	if len(got) != 3 || got[0] != "03" || got[1] != "02" || got[2] != "01" {
		t.Errorf("Backward: unexpected values %v", got)
	}
	if len(idx) != 3 || idx[0] != 2 || idx[1] != 1 || idx[2] != 0 {
		t.Errorf("Backward: unexpected indexes %v", idx)
	}

	// IterateOver (legacy name for All).
	got = nil
	for _, v := range Dll1.IterateOver() {
		got = append(got, v.S)
	}
	if len(got) != 3 || got[0] != "01" || got[2] != "03" {
		t.Errorf("IterateOver: unexpected values %v", got)
	}

	// IteratePtr.
	got = nil
	for _, v := range Dll1.IteratePtr() {
		got = append(got, v.S)
	}
	if len(got) != 3 || got[0] != "01" || got[2] != "03" {
		t.Errorf("IteratePtr: unexpected values %v", got)
	}

	// Empty list: no iterations.
	var Dll2 Dll[TestDemo]
	for range Dll2.All() {
		t.Errorf("All on empty list iterated")
	}
	for range Dll2.Backward() {
		t.Errorf("Backward on empty list iterated")
	}
}

func TestPeekPopTail(t *testing.T) {

	var Dll1 Dll[TestDemo]

	// Empty list errors.
	if _, err := Dll1.Peek(); err == nil {
		t.Errorf("Expected error on Peek of empty list")
	}
	if _, err := Dll1.PeekTail(); err == nil {
		t.Errorf("Expected error on PeekTail of empty list")
	}
	if _, err := Dll1.PopTail(); err == nil {
		t.Errorf("Expected error on PopTail of empty list")
	}

	Dll1.Enqueue(&TestDemo{S: "01"})
	Dll1.Enqueue(&TestDemo{S: "02"})

	if v, err := Dll1.Peek(); err != nil || v.S != "01" {
		t.Errorf("Peek: expected 01, got %v, %v", v, err)
	}
	if v, err := Dll1.PeekTail(); err != nil || v.S != "02" {
		t.Errorf("PeekTail: expected 02, got %v, %v", v, err)
	}
	if v, err := Dll1.PopTail(); err != nil || v.S != "02" {
		t.Errorf("PopTail: expected 02, got %v, %v", v, err)
	}
	if Dll1.Length() != 1 {
		t.Errorf("Expected length 1, got %d", Dll1.Length())
	}

	// Pop the last element; the tail pointer must be reset so the list
	// can be reused correctly.
	if v, err := Dll1.Pop(); err != nil || v.S != "01" {
		t.Errorf("Pop: expected 01, got %v, %v", v, err)
	}
	Dll1.Enqueue(&TestDemo{S: "03"})
	if v, err := Dll1.PopTail(); err != nil || v.S != "03" {
		t.Errorf("PopTail after reuse: expected 03, got %v, %v", v, err)
	}
	if !Dll1.IsEmpty() {
		t.Errorf("Expected empty list")
	}

	// PopTail the last element; the head pointer must be reset so the list
	// can be reused correctly.
	Dll1.Enqueue(&TestDemo{S: "04"})
	if v, err := Dll1.PopTail(); err != nil || v.S != "04" {
		t.Errorf("PopTail: expected 04, got %v, %v", v, err)
	}
	Dll1.Enqueue(&TestDemo{S: "05"})
	if v, err := Dll1.Pop(); err != nil || v.S != "05" {
		t.Errorf("Pop after reuse: expected 05, got %v, %v", v, err)
	}
}

func TestTrimReuse(t *testing.T) {

	var Dll1 Dll[TestDemo]
	Dll1.AppendAtTail(&TestDemo{S: "aa"})
	Dll1.AppendAtTail(&TestDemo{S: "bb"})
	Dll1.AppendAtTail(&TestDemo{S: "cc"})
	Dll1.AppendAtTail(&TestDemo{S: "dd"})

	// Trim to zero must fully reset the list (head and tail) so that it
	// can be reused without stale nodes leaking back in.
	if err := Dll1.Trim(0); err != nil {
		t.Errorf("Unexpected error from Trim(0): %v", err)
	}
	if Dll1.Length() != 0 {
		t.Errorf("Expected length 0 after Trim(0), got %d", Dll1.Length())
	}
	Dll1.AppendAtTail(&TestDemo{S: "zz"})
	if v, err := Dll1.Pop(); err != nil || v.S != "zz" {
		t.Errorf("Pop after Trim(0)+append: expected zz, got %v, %v", v, err)
	}

	// TrimTail to zero must fully reset the list too.
	Dll1.AppendAtTail(&TestDemo{S: "aa"})
	Dll1.AppendAtTail(&TestDemo{S: "bb"})
	if err := Dll1.TrimTail(0); err != nil {
		t.Errorf("Unexpected error from TrimTail(0): %v", err)
	}
	if Dll1.Length() != 0 {
		t.Errorf("Expected length 0 after TrimTail(0), got %d", Dll1.Length())
	}
	Dll1.AppendAtTail(&TestDemo{S: "zz"})
	if v, err := Dll1.Pop(); err != nil || v.S != "zz" {
		t.Errorf("Pop after TrimTail(0)+append: expected zz, got %v, %v", v, err)
	}

	// Trim on an empty list reports an error.
	var Dll2 Dll[TestDemo]
	if err := Dll2.Trim(1); err == nil {
		t.Errorf("Expected error from Trim on empty list")
	}
	if err := Dll2.TrimTail(1); err == nil {
		t.Errorf("Expected error from TrimTail on empty list")
	}

	// Trim keeps the head, TrimTail keeps the tail.
	Dll3 := buildList("aa", "bb", "cc", "dd")
	if err := Dll3.Trim(2); err != nil {
		t.Errorf("Unexpected error from Trim(2): %v", err)
	}
	if v, _ := Dll3.Peek(); v.S != "aa" {
		t.Errorf("After Trim(2) expected head aa, got %s", v.S)
	}
	if v, _ := Dll3.PeekTail(); v.S != "bb" {
		t.Errorf("After Trim(2) expected tail bb, got %s", v.S)
	}
	Dll4 := buildList("aa", "bb", "cc", "dd")
	if err := Dll4.TrimTail(2); err != nil {
		t.Errorf("Unexpected error from TrimTail(2): %v", err)
	}
	if v, _ := Dll4.Peek(); v.S != "cc" {
		t.Errorf("After TrimTail(2) expected head cc, got %s", v.S)
	}
	if v, _ := Dll4.PeekTail(); v.S != "dd" {
		t.Errorf("After TrimTail(2) expected tail dd, got %s", v.S)
	}
}

func TestDeleteByValue(t *testing.T) {

	Dll1 := buildList("01", "02", "03")

	// Delete removes the first matching element.
	if err := Dll1.Delete(&TestDemo{S: "02"}); err != nil {
		t.Errorf("Unexpected error from Delete: %v", err)
	}
	if Dll1.Length() != 2 {
		t.Errorf("Expected length 2, got %d", Dll1.Length())
	}

	// Delete of a missing element returns an error.
	if err := Dll1.Delete(&TestDemo{S: "99"}); err == nil {
		t.Errorf("Expected error from Delete of missing element")
	}
	if err := Dll1.DeleteSearch(&TestDemo{S: "99"}); err == nil {
		t.Errorf("Expected error from DeleteSearch of missing element")
	}

	// DeleteFound of the single element empties the list.
	Dll2 := buildList("01")
	rv, pos := Dll2.Search(&TestDemo{S: "01"})
	if pos != 0 {
		t.Errorf("Expected pos 0, got %d", pos)
	}
	if err := Dll2.DeleteFound(rv); err != nil {
		t.Errorf("Unexpected error from DeleteFound: %v", err)
	}
	if !Dll2.IsEmpty() {
		t.Errorf("Expected empty list after DeleteFound of only element")
	}

	// DeleteFound of nil returns an error.
	if err := Dll2.DeleteFound(nil); err == nil {
		t.Errorf("Expected error from DeleteFound(nil)")
	}
}

func TestReverseSearchPos(t *testing.T) {

	Dll1 := buildList("01", "02", "03")

	// ReverseSearch returns the position counting from the head.
	_, pos := Dll1.ReverseSearch(&TestDemo{S: "03"})
	if pos != 2 {
		t.Errorf("ReverseSearch: expected pos 2, got %d", pos)
	}
	_, pos = Dll1.ReverseSearch(&TestDemo{S: "01"})
	if pos != 0 {
		t.Errorf("ReverseSearch: expected pos 0, got %d", pos)
	}
	_, pos = Dll1.ReverseSearch(&TestDemo{S: "99"})
	if pos != -1 {
		t.Errorf("ReverseSearch: expected pos -1, got %d", pos)
	}

	// ReverseWalk reports positions counting from the head.
	var idx []int
	Dll1.ReverseWalk(func(pos int, data TestDemo, userData interface{}) bool {
		idx = append(idx, pos)
		return false
	}, nil)
	if len(idx) != 3 || idx[0] != 2 || idx[1] != 1 || idx[2] != 0 {
		t.Errorf("ReverseWalk: unexpected indexes %v", idx)
	}
}

func TestReverseAndConcat(t *testing.T) {

	Dll1 := buildList("01", "02", "03")
	Dll1.Reverse()
	var got []string
	for _, v := range Dll1.All() {
		got = append(got, v.S)
	}
	if len(got) != 3 || got[0] != "03" || got[1] != "02" || got[2] != "01" {
		t.Errorf("Reverse: unexpected values %v", got)
	}

	// ReverseList is the legacy alias for Reverse.
	Dll1.ReverseList()
	got = nil
	for _, v := range Dll1.All() {
		got = append(got, v.S)
	}
	if len(got) != 3 || got[0] != "01" || got[1] != "02" || got[2] != "03" {
		t.Errorf("ReverseList: unexpected values %v", got)
	}

	// Concat appends a copy of another list.
	Dll2 := buildList("04", "05")
	Dll1.Concat(&Dll2)
	if Dll1.Length() != 5 {
		t.Errorf("Expected length 5 after Concat, got %d", Dll1.Length())
	}
	if v, _ := Dll1.PeekTail(); v.S != "05" {
		t.Errorf("Expected tail 05 after Concat, got %s", v.S)
	}
	if Dll2.Length() != 2 {
		t.Errorf("Concat must not modify its argument, got length %d", Dll2.Length())
	}

	// Self-concat duplicates the list onto itself.
	Dll3 := buildList("a", "b")
	Dll3.Concat(&Dll3)
	if Dll3.Length() != 4 {
		t.Errorf("Expected length 4 after self-Concat, got %d", Dll3.Length())
	}
}

func BenchmarkPush(b *testing.B) {
	var Dll1 Dll[TestDemo]
	v := TestDemo{S: "x"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Dll1.Push(&v)
	}
}

func BenchmarkAppendAtTail(b *testing.B) {
	var Dll1 Dll[TestDemo]
	v := TestDemo{S: "x"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Dll1.AppendAtTail(&v)
	}
}

func BenchmarkPop(b *testing.B) {
	var Dll1 Dll[TestDemo]
	v := TestDemo{S: "x"}
	for i := 0; i < b.N; i++ {
		Dll1.AppendAtTail(&v)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Dll1.Pop()
	}
}

func BenchmarkSearch(b *testing.B) {
	var Dll1 Dll[TestDemo]
	last := TestDemo{S: "last"}
	for i := 0; i < 100; i++ {
		v := TestDemo{S: "x"}
		Dll1.AppendAtTail(&v)
	}
	Dll1.AppendAtTail(&last)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Dll1.Search(&last)
	}
}

func BenchmarkAll(b *testing.B) {
	var Dll1 Dll[TestDemo]
	for i := 0; i < 100; i++ {
		v := TestDemo{S: "x"}
		Dll1.AppendAtTail(&v)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for range Dll1.All() {
		}
	}
}
