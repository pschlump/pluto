package quicklist

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Unit tests: end operations, positional operations with negative
// indexes, range semantics, rotation, compression transparency, the
// depth-window invariant, nil tolerance and the nil-write panics.

import (
	"fmt"
	"slices"
	"testing"
)

// collect drains an iterator into a slice.
func collect[T any](seq func(func(int, T) bool)) []T {
	var out []T
	for _, v := range seq {
		out = append(out, v)
	}
	return out
}

// build pushes 0..n-1 at the tail.
func build(n int, opts ...Option[int]) *QuickList[int] {
	q := NewQuickList(opts...)
	for i := 0; i < n; i++ {
		q.PushTail(i)
	}
	return q
}

func TestEndOperations(t *testing.T) {
	q := NewQuickList(WithSegmentFill[int](4))
	if _, ok := q.PopHead(); ok {
		t.Fatal("PopHead on empty list returned ok")
	}
	if _, ok := q.PopTail(); ok {
		t.Fatal("PopTail on empty list returned ok")
	}
	if _, ok := q.PeekHead(); ok {
		t.Fatal("PeekHead on empty list returned ok")
	}
	if _, ok := q.PeekTail(); ok {
		t.Fatal("PeekTail on empty list returned ok")
	}
	q.PushHead(2)
	q.PushHead(1)
	q.PushTail(3)
	q.PushTail(4)
	if got := collect(q.All()); !slices.Equal(got, []int{1, 2, 3, 4}) {
		t.Fatalf("contents: got %v", got)
	}
	if v, _ := q.PeekHead(); v != 1 {
		t.Fatalf("PeekHead: got %d", v)
	}
	if v, _ := q.PeekTail(); v != 4 {
		t.Fatalf("PeekTail: got %d", v)
	}
	if v, ok := q.PopHead(); !ok || v != 1 {
		t.Fatalf("PopHead: got %d,%v", v, ok)
	}
	if v, ok := q.PopTail(); !ok || v != 4 {
		t.Fatalf("PopTail: got %d,%v", v, ok)
	}
	if q.Len() != 2 {
		t.Fatalf("Len: got %d", q.Len())
	}
	// Drain to empty; the list must collapse to no segments.
	q.PopHead()
	q.PopHead()
	if q.Len() != 0 || q.head != nil || q.tail != nil {
		t.Fatal("drained list still holds segments")
	}
	// Reusable after draining.
	q.PushTail(9)
	if v, _ := q.PeekTail(); v != 9 {
		t.Fatalf("PushTail after drain: got %d", v)
	}
}

func TestZeroValueUsable(t *testing.T) {
	var q QuickList[int]
	for i := 0; i < 300; i++ {
		q.PushTail(i)
	}
	if q.Len() != 300 {
		t.Fatalf("Len: got %d", q.Len())
	}
	if v, ok := q.At(299); !ok || v != 299 {
		t.Fatalf("At: got %d,%v", v, ok)
	}
}

func TestAtSetNegativeIndexes(t *testing.T) {
	q := build(10)
	for i := 0; i < 10; i++ {
		if v, ok := q.At(i); !ok || v != i {
			t.Fatalf("At(%d): got %d,%v", i, v, ok)
		}
		if v, ok := q.At(i - 10); !ok || v != i {
			t.Fatalf("At(%d): got %d,%v", i-10, v, ok)
		}
	}
	if _, ok := q.At(10); ok {
		t.Fatal("At(10) out of range returned ok")
	}
	if _, ok := q.At(-11); ok {
		t.Fatal("At(-11) out of range returned ok")
	}
	if !q.Set(-1, 99) {
		t.Fatal("Set(-1) failed")
	}
	if v, _ := q.At(9); v != 99 {
		t.Fatalf("after Set(-1,99) At(9): got %d", v)
	}
	if q.Set(10, 0) || q.Set(-11, 0) {
		t.Fatal("Set out of range returned true")
	}
}

func TestInsertBoundaries(t *testing.T) {
	q := NewQuickList(WithSegmentFill[int](4))
	q.PushTail(1)
	q.PushTail(2)
	if !q.InsertBefore(0, 0) {
		t.Fatal("InsertBefore(0) failed")
	}
	if !q.InsertAfter(-1, 3) {
		t.Fatal("InsertAfter(-1) failed")
	}
	if got := collect(q.All()); !slices.Equal(got, []int{0, 1, 2, 3}) {
		t.Fatalf("contents: got %v", got)
	}
	if q.InsertBefore(4, 9) || q.InsertAfter(4, 9) || q.InsertBefore(-5, 9) {
		t.Fatal("insert out of range returned true")
	}
	// No phantom empty segments after the boundary inserts.
	for s := q.head; s != nil; s = s.next {
		if s.n == 0 {
			t.Fatal("empty segment after boundary inserts")
		}
	}
}

func TestInsertDeleteMiddle(t *testing.T) {
	q := build(20, WithSegmentFill[int](4))
	if !q.InsertBefore(10, 100) {
		t.Fatal("InsertBefore failed")
	}
	if !q.InsertAfter(10, 200) {
		t.Fatal("InsertAfter failed")
	}
	// After InsertBefore(10, 100) index 10 holds 100, so InsertAfter(10,
	// 200) lands between 100 and 10.
	want := []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 100, 200, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}
	if got := collect(q.All()); !slices.Equal(got, want) {
		t.Fatalf("contents:\n got %v\nwant %v", got, want)
	}
	if !q.Delete(10) {
		t.Fatal("first Delete failed")
	}
	if !q.Delete(10) {
		t.Fatal("second Delete failed")
	}
	if got := collect(q.All()); !slices.Equal(got, slices.Concat(want[:10], want[12:])) {
		t.Fatalf("after deletes: got %v", got)
	}
	if q.Delete(20) || q.Delete(-21) {
		t.Fatal("Delete out of range returned true")
	}
}

func TestDeleteRange(t *testing.T) {
	q := build(10)
	if n := q.DeleteRange(2, 4); n != 3 {
		t.Fatalf("DeleteRange count: got %d", n)
	}
	if got := collect(q.All()); !slices.Equal(got, []int{0, 1, 5, 6, 7, 8, 9}) {
		t.Fatalf("contents: got %v", got)
	}
	// Negative and clamped bounds.
	if n := q.DeleteRange(-2, 100); n != 2 {
		t.Fatalf("DeleteRange count: got %d", n)
	}
	if got := collect(q.All()); !slices.Equal(got, []int{0, 1, 5, 6, 7}) {
		t.Fatalf("contents: got %v", got)
	}
	// Inverted and out-of-range ranges remove nothing.
	if n := q.DeleteRange(3, 1); n != 0 {
		t.Fatalf("inverted range: got %d", n)
	}
	if n := q.DeleteRange(10, 20); n != 0 {
		t.Fatalf("out-of-range range: got %d", n)
	}
	// Whole list.
	if n := q.DeleteRange(0, -1); n != 5 || q.Len() != 0 {
		t.Fatalf("DeleteRange all: got %d len %d", n, q.Len())
	}
}

func TestTrim(t *testing.T) {
	q := build(10)
	q.Trim(2, 7)
	if got := collect(q.All()); !slices.Equal(got, []int{2, 3, 4, 5, 6, 7}) {
		t.Fatalf("Trim(2,7): got %v", got)
	}
	q.Trim(-3, -1)
	if got := collect(q.All()); !slices.Equal(got, []int{5, 6, 7}) {
		t.Fatalf("Trim(-3,-1): got %v", got)
	}
	q.Trim(5, 2) // inverted: empties the list
	if q.Len() != 0 {
		t.Fatalf("inverted Trim left %d elements", q.Len())
	}
}

func TestRangeIterators(t *testing.T) {
	q := build(10)
	if got := collect(q.Range(2, 5)); !slices.Equal(got, []int{2, 3, 4, 5}) {
		t.Fatalf("Range(2,5): got %v", got)
	}
	if got := collect(q.Range(-3, -1)); !slices.Equal(got, []int{7, 8, 9}) {
		t.Fatalf("Range(-3,-1): got %v", got)
	}
	if got := collect(q.Range(0, 100)); len(got) != 10 {
		t.Fatalf("clamped Range: got %v", got)
	}
	if got := collect(q.Range(5, 2)); len(got) != 0 {
		t.Fatalf("inverted Range: got %v", got)
	}
	// Backward yields descending absolute indexes.
	var idx []int
	for i, v := range q.Backward() {
		idx = append(idx, i)
		if v != i {
			t.Fatalf("Backward: index %d value %d", i, v)
		}
	}
	if !slices.Equal(idx, []int{9, 8, 7, 6, 5, 4, 3, 2, 1, 0}) {
		t.Fatalf("Backward indexes: got %v", idx)
	}
	// Early break stops the walk.
	n := 0
	for range q.All() {
		n++
		if n == 3 {
			break
		}
	}
	if n != 3 {
		t.Fatalf("early break: got %d", n)
	}
}

func TestMoveHeadToTail(t *testing.T) {
	src := build(3)
	dst := build(2)
	v, ok := MoveHeadToTail(src, dst)
	if !ok || v != 0 {
		t.Fatalf("MoveHeadToTail: got %d,%v", v, ok)
	}
	if got := collect(src.All()); !slices.Equal(got, []int{1, 2}) {
		t.Fatalf("src: got %v", got)
	}
	if got := collect(dst.All()); !slices.Equal(got, []int{0, 1, 0}) {
		t.Fatalf("dst: got %v", got)
	}
	// src == dst rotates the list by one.
	if _, ok := MoveHeadToTail(src, src); !ok {
		t.Fatal("self-rotation failed")
	}
	if got := collect(src.All()); !slices.Equal(got, []int{2, 1}) {
		t.Fatalf("rotated src: got %v", got)
	}
	empty := NewQuickList[int]()
	if _, ok := MoveHeadToTail(empty, dst); ok {
		t.Fatal("MoveHeadToTail from empty list returned ok")
	}
}

// strBuild pushes s000..s0(n-1) at the tail of a string list.
func strBuild(n int, opts ...Option[string]) *QuickList[string] {
	q := NewQuickList(opts...)
	for i := 0; i < n; i++ {
		q.PushTail(fmt.Sprintf("s%03d", i))
	}
	return q
}

// checkDepthWindow verifies the Redis list-compress-depth invariant: the
// first and last depth segments are plain, everything deeper is packed.
func checkDepthWindow[T any](t *testing.T, q *QuickList[T]) {
	t.Helper()
	if q.codec == nil || q.depth <= 0 {
		return
	}
	m := 0
	for s := q.head; s != nil; s = s.next {
		m++
	}
	i := 0
	for s := q.head; s != nil; s = s.next {
		protected := i < q.depth || i >= m-q.depth
		if protected && s.packed != nil {
			t.Fatalf("segment %d of %d inside depth window is packed", i, m)
		}
		if !protected && s.packed == nil {
			t.Fatalf("segment %d of %d outside depth window is plain", i, m)
		}
		i++
	}
}

func TestCompressionTransparent(t *testing.T) {
	opts := []Option[string]{
		WithSegmentFill[string](8),
		WithCompression[string](LZWCodec(), 1, EncodeStringSegment, DecodeStringSegment),
	}
	q := strBuild(64, opts...)
	checkDepthWindow(t, q)
	// Full content check through the compressed interior.
	var want []string
	for i := 0; i < 64; i++ {
		want = append(want, fmt.Sprintf("s%03d", i))
	}
	if got := collect(q.All()); !slices.Equal(got, want) {
		t.Fatalf("contents mismatch through compression")
	}
	checkDepthWindow(t, q)
	// Random-access reads and writes hit packed segments transparently.
	if v, ok := q.At(32); !ok || v != "s032" {
		t.Fatalf("At(32): got %q,%v", v, ok)
	}
	checkDepthWindow(t, q)
	if !q.Set(32, "middle") {
		t.Fatal("Set failed")
	}
	if v, _ := q.At(-32); v != "middle" {
		t.Fatalf("At(-32) after Set: got %q", v)
	}
	checkDepthWindow(t, q)
	// Mutations across packed segments.
	if !q.InsertBefore(32, "before") || !q.Delete(0) {
		t.Fatal("insert/delete across packed segments failed")
	}
	if n := q.DeleteRange(30, 34); n != 5 {
		t.Fatalf("DeleteRange across packed segments: got %d", n)
	}
	q.Trim(0, 19)
	if q.Len() != 20 {
		t.Fatalf("Trim: len %d", q.Len())
	}
	checkDepthWindow(t, q)
	// Pops drain through the packed region correctly.
	for q.Len() > 0 {
		q.PopTail()
		checkDepthWindow(t, q)
	}
	if q.head != nil {
		t.Fatal("drained compressed list still holds segments")
	}
}

func TestCompressionDepthZeroDisables(t *testing.T) {
	q := strBuild(32,
		WithSegmentFill[string](8),
		WithCompression[string](LZWCodec(), 0, EncodeStringSegment, DecodeStringSegment))
	for s := q.head; s != nil; s = s.next {
		if s.packed != nil {
			t.Fatal("depth 0 packed a segment")
		}
	}
}

func TestSegmentBytesCap(t *testing.T) {
	// string header is 16 bytes; a 64-byte cap packs 4 per segment.
	q := strBuild(20, WithSegmentBytes[string](64))
	m := 0
	for s := q.head; s != nil; s = s.next {
		if s.n > 4 {
			t.Fatalf("segment holds %d elements over the 64-byte cap", s.n)
		}
		m++
	}
	if m < 5 {
		t.Fatalf("expected at least 5 segments, got %d", m)
	}
	if q.Len() != 20 {
		t.Fatalf("Len: got %d", q.Len())
	}
}

func TestNilTolerated(t *testing.T) {
	var q *QuickList[int]
	if q.Len() != 0 {
		t.Fatal("nil Len")
	}
	if _, ok := q.At(0); ok {
		t.Fatal("nil At")
	}
	if _, ok := q.PopHead(); ok {
		t.Fatal("nil PopHead")
	}
	if _, ok := q.PeekTail(); ok {
		t.Fatal("nil PeekTail")
	}
	if q.Set(0, 1) || q.Delete(0) || q.InsertBefore(0, 1) || q.InsertAfter(0, 1) {
		t.Fatal("nil positional write returned true")
	}
	if n := q.DeleteRange(0, -1); n != 0 {
		t.Fatal("nil DeleteRange")
	}
	q.Trim(0, -1) // must not panic
	if got := collect(q.All()); len(got) != 0 {
		t.Fatal("nil All")
	}
	if _, ok := MoveHeadToTail[int](q, q); ok {
		t.Fatal("nil MoveHeadToTail")
	}
}

func TestNilWritePanics(t *testing.T) {
	for name, fx := range map[string]func(){
		"PushHead": func() { var q *QuickList[int]; q.PushHead(1) },
		"PushTail": func() { var q *QuickList[int]; q.PushTail(1) },
	} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				r := recover()
				if r == nil {
					t.Fatal("expected panic")
				}
				msg := fmt.Sprint(r)
				if !slices.Contains([]string{msg}, "quicklist: "+name+" called on a nil QuickList") {
					t.Fatalf("panic message: %q", msg)
				}
			}()
			fx()
		})
	}
}
