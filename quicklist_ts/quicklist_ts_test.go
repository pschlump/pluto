package quicklist_ts

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Tests: API parity with the plain package, snapshot-iterator
// semantics (mutate-while-iterating), the Lock/Nl* compound pattern,
// MoveHeadToTail between two lists in both lock orders, and a
// concurrent hammer for -race.

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/pschlump/pluto/quicklist"
)

func build(n int) *QuickList[int] {
	q := NewQuickList(quicklist.WithSegmentFill[int](8))
	for i := 0; i < n; i++ {
		q.PushTail(i)
	}
	return q
}

func TestAPIParity(t *testing.T) {
	q := build(100)
	if q.Len() != 100 {
		t.Fatalf("Len: got %d", q.Len())
	}
	if v, ok := q.At(50); !ok || v != 50 {
		t.Fatalf("At(50): got %d,%v", v, ok)
	}
	if v, ok := q.At(-1); !ok || v != 99 {
		t.Fatalf("At(-1): got %d,%v", v, ok)
	}
	if !q.Set(0, 1000) {
		t.Fatal("Set failed")
	}
	if !q.InsertBefore(1, 999) || !q.InsertAfter(-1, 1001) {
		t.Fatal("inserts failed")
	}
	if !q.Delete(0) || !q.Delete(-1) {
		t.Fatal("deletes failed")
	}
	if n := q.DeleteRange(0, 9); n != 10 {
		t.Fatalf("DeleteRange: got %d", n)
	}
	q.Trim(10, 19)
	if q.Len() != 10 {
		t.Fatalf("after Trim: Len %d", q.Len())
	}
	// List is now [20 21 ... 29].
	if v, _ := q.PeekHead(); v != 20 {
		t.Fatalf("PeekHead: got %d", v)
	}
	if v, _ := q.PeekTail(); v != 29 {
		t.Fatalf("PeekTail: got %d", v)
	}
	if v, ok := q.PopHead(); !ok || v != 20 {
		t.Fatalf("PopHead: got %d,%v", v, ok)
	}
	if v, ok := q.PopTail(); !ok || v != 29 {
		t.Fatalf("PopTail: got %d,%v", v, ok)
	}
}

func TestSnapshotIterators(t *testing.T) {
	q := build(10)
	// Mutating from inside the loop is safe and the iterator keeps
	// walking the snapshot.
	var got []int
	for i, v := range q.All() {
		got = append(got, v)
		if i == 4 {
			q.Delete(0)
			q.PushTail(99)
		}
	}
	if len(got) != 10 {
		t.Fatalf("snapshot All yielded %d elements, want 10", len(got))
	}
	if !slices.Equal(got, []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}) {
		t.Fatalf("snapshot All: got %v", got)
	}
	// Backward indexes count down from Len()-1 of the snapshot.
	var idx []int
	for i := range q.Backward() {
		idx = append(idx, i)
	}
	if !slices.IsSortedFunc(idx, func(a, b int) int { return b - a }) {
		t.Fatalf("Backward indexes not descending: %v", idx)
	}
	// Range over a clamped/negative window.
	var r []int
	for _, v := range q.Range(-3, 100) {
		r = append(r, v)
	}
	if len(r) != 3 {
		t.Fatalf("Range(-3,100): got %v", r)
	}
}

func TestLockNlCompound(t *testing.T) {
	q := build(5)
	q.Lock()
	// Atomic batch: replace every element with its square, atomically.
	for i := 0; i < q.NlLen(); i++ {
		v, _ := q.NlAt(i)
		q.NlSet(i, v*v)
	}
	q.Unlock()
	want := []int{0, 1, 4, 9, 16}
	var got []int
	for _, v := range q.All() {
		got = append(got, v)
	}
	if !slices.Equal(got, want) {
		t.Fatalf("compound: got %v, want %v", got, want)
	}
	// Nl inserts and range ops under the lock: [0,2,4,6,8] + 25 at the
	// tail, drop the first two, then drop the new tail.
	q.Lock()
	q.NlInsertAfter(-1, 25)
	q.NlDeleteRange(0, 1)
	q.NlTrim(0, -2)
	q.Unlock()
	if q.Len() != 3 {
		t.Fatalf("after Nl ops: Len %d", q.Len())
	}
	var rest []int
	for _, v := range q.All() {
		rest = append(rest, v)
	}
	if !slices.Equal(rest, []int{4, 9, 16}) {
		t.Fatalf("after Nl ops: got %v, want [4 9 16]", rest)
	}
}

func TestMoveHeadToTail(t *testing.T) {
	src := NewQuickList[int]()
	dst := NewQuickList[int]()
	src.PushTail(1)
	src.PushTail(2)
	dst.PushTail(9)
	v, ok := MoveHeadToTail(src, dst)
	if !ok || v != 1 {
		t.Fatalf("MoveHeadToTail: got %d,%v", v, ok)
	}
	// Reverse direction locks the pair in the opposite caller order;
	// pointer ordering keeps it deadlock-free.
	v, ok = MoveHeadToTail(dst, src)
	if !ok || v != 9 {
		t.Fatalf("reverse MoveHeadToTail: got %d,%v", v, ok)
	}
	// Self-rotation.
	if _, ok := MoveHeadToTail(src, src); !ok {
		t.Fatal("self-rotation failed")
	}
	if _, ok := MoveHeadToTail(NewQuickList[int](), dst); ok {
		t.Fatal("move from empty returned ok")
	}
}

func TestConcurrentHammer(t *testing.T) {
	q := NewQuickList(quicklist.WithSegmentFill[int](16))
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 2000; i++ {
				switch i % 6 {
				case 0:
					q.PushTail(i)
				case 1:
					q.PushHead(i)
				case 2:
					q.PopHead()
				case 3:
					q.PopTail()
				case 4:
					q.At(i % 50)
				case 5:
					q.Len()
				}
			}
		}(g)
	}
	// Concurrent snapshot iterations.
	for g := 0; g < 2; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				n := 0
				for range q.All() {
					n++
				}
				_ = n
			}
		}()
	}
	wg.Wait()
	// Drained of nothing in particular — just verify consistency: Len
	// agrees with a full walk.
	n := 0
	for range q.All() {
		n++
	}
	if n != q.Len() {
		t.Fatalf("inconsistent after hammer: walk %d, Len %d", n, q.Len())
	}
}

func TestConcurrentMoveHeadToTail(t *testing.T) {
	a := NewQuickList[int]()
	b := NewQuickList[int]()
	for i := 0; i < 100; i++ {
		a.PushTail(i)
		b.PushTail(i)
	}
	var wg sync.WaitGroup
	for g := 0; g < 4; g++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			for i := 0; i < 500; i++ {
				MoveHeadToTail(a, b)
			}
		}()
		go func() {
			defer wg.Done()
			for i := 0; i < 500; i++ {
				MoveHeadToTail(b, a)
			}
		}()
	}
	wg.Wait()
	if got := a.Len() + b.Len(); got != 200 {
		t.Fatalf("elements lost: total %d, want 200", got)
	}
}

func TestNilToleratedAndPanics(t *testing.T) {
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
	if q.Set(0, 1) || q.Delete(0) {
		t.Fatal("nil write returned true")
	}
	q.Trim(0, -1)
	q.Lock()
	q.Unlock()
	for range q.All() {
		t.Fatal("nil All yielded")
	}
	if _, ok := MoveHeadToTail[int](q, q); ok {
		t.Fatal("nil MoveHeadToTail")
	}
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic from nil PushTail")
		}
		if msg := fmt.Sprint(r); msg != "quicklist_ts: PushTail called on a nil QuickList" {
			t.Fatalf("panic message: %q", msg)
		}
	}()
	q.PushTail(1)
}

// -------------------------------------------------------------------------------------------------------
// JSON — encoding/json integration (MarshalJSON/UnmarshalJSON in json.go).
// -------------------------------------------------------------------------------------------------------

// upperString is a string with its own JSON representation, to verify
// that element-level marshalers are honored through the list.
type upperString string

func (u upperString) MarshalJSON() ([]byte, error) {
	return []byte(`"` + strings.ToUpper(string(u)) + `"`), nil
}

func (u *upperString) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	*u = upperString(s)
	return nil
}

func TestMarshalJSON(t *testing.T) {
	// Exact array output, head to tail, spanning several segments.
	q := NewQuickList(quicklist.WithSegmentFill[int](4))
	for _, v := range []int{3, 1, 2, 9, 7, 5} {
		q.PushTail(v)
	}
	b, err := json.Marshal(q)
	if err != nil {
		t.Fatalf("json.Marshal(q): %v", err)
	}
	if string(b) != "[3,1,2,9,7,5]" {
		t.Errorf("Expected [3,1,2,9,7,5], got %s", b)
	}

	// Struct elements use their normal JSON encoding.
	type item struct {
		S string
	}
	items := NewQuickList[item]()
	items.PushTail(item{S: "a"})
	items.PushTail(item{S: "b"})
	if b, err := json.Marshal(items); err != nil || string(b) != `[{"S":"a"},{"S":"b"}]` {
		t.Errorf(`Expected [{"S":"a"},{"S":"b"}], got (%s, %v)`, b, err)
	}

	// An empty list encodes as [].
	if b, err := json.Marshal(NewQuickList[int]()); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] for an empty list, got (%s, %v)", b, err)
	}

	// A zero-value list is a tolerated read: [].
	var zero QuickList[int]
	if b, err := zero.MarshalJSON(); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] for a zero-value list, got (%s, %v)", b, err)
	}

	// A direct call on a nil list encodes as []; json.Marshal on a nil
	// *QuickList never reaches the method — the json package writes null
	// for nil pointers itself.
	var nilList *QuickList[int]
	if b, err := nilList.MarshalJSON(); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] from a direct nil-list call, got (%s, %v)", b, err)
	}
	if b, err := json.Marshal(nilList); err != nil || string(b) != "null" {
		t.Errorf("Expected null from json.Marshal on a nil list, got (%s, %v)", b, err)
	}

	// Element-level marshalers are honored.
	custom := NewQuickList[upperString]()
	custom.PushTail("x")
	custom.PushTail("y")
	if b, err := json.Marshal(custom); err != nil || string(b) != `["X","Y"]` {
		t.Errorf(`Expected ["X","Y"], got (%s, %v)`, b, err)
	}

	// Encoding errors pass through unchanged.
	bad := NewQuickList[chan int]()
	bad.PushTail(make(chan int))
	if _, err := json.Marshal(bad); err == nil {
		t.Errorf("Expected an error marshaling a list of channels.")
	}
}

func TestUnmarshalJSON(t *testing.T) {
	// Decoded order is preserved: element 0 becomes the head.
	q := NewQuickList[int]()
	if err := json.Unmarshal([]byte("[3,1,2]"), q); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	var got []int
	for _, v := range q.All() {
		got = append(got, v)
	}
	if !slices.Equal(got, []int{3, 1, 2}) {
		t.Errorf("Expected [3 1 2], got %v", got)
	}
	if head, ok := q.PeekHead(); !ok || head != 3 {
		t.Errorf("Expected head 3, got (%v, %v)", head, ok)
	}

	// A round trip rebuilds the same contents across segments.
	full := build(50)
	b, err := json.Marshal(full)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	again := NewQuickList(quicklist.WithSegmentFill[int](8))
	if err := json.Unmarshal(b, again); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	var round, want []int
	for _, v := range again.All() {
		round = append(round, v)
	}
	for _, v := range full.All() {
		want = append(want, v)
	}
	if !slices.Equal(round, want) {
		t.Errorf("Round trip changed the contents.")
	}
	if again.Len() != 50 {
		t.Errorf("Round trip Len: got %d", again.Len())
	}

	// The zero value is a ready-to-use list — no constructor functions
	// to keep — so it unmarshals elements directly.
	var zero QuickList[int]
	if err := json.Unmarshal([]byte("[1,2,3]"), &zero); err != nil {
		t.Fatalf("json.Unmarshal into a zero-value list: %v", err)
	}
	got = got[:0]
	for _, v := range zero.All() {
		got = append(got, v)
	}
	if !slices.Equal(got, []int{1, 2, 3}) {
		t.Errorf("Zero-value list contents: got %v", got)
	}

	// An empty array and null clear the list and are tolerated.
	for _, data := range []string{"[]", "null"} {
		if err := json.Unmarshal([]byte(data), full); err != nil {
			t.Fatalf("json.Unmarshal(%s): %v", data, err)
		}
		if full.Len() != 0 {
			t.Errorf("Expected %s to clear the list, Len %d", data, full.Len())
		}
	}

	// Decode errors are returned and leave the list untouched.
	keep := NewQuickList[string]()
	keep.PushTail("keep")
	for _, badData := range []string{"[1,", `[1]`, `{"S":"a"}`, "7", `["a",3]`} {
		if err := json.Unmarshal([]byte(badData), keep); err == nil {
			t.Errorf("Expected an error unmarshaling %s.", badData)
		}
		if keep.Len() != 1 {
			t.Errorf("List changed after the error on %s: Len %d", badData, keep.Len())
		}
		if v, _ := keep.PeekHead(); v != "keep" {
			t.Errorf("List changed after the error on %s: head %q", badData, v)
		}
	}
}

// TestJSONCompressedRoundTrip unmarshals into a list that keeps its
// compression configuration: the rebuilt list reads back transparently.
func TestJSONCompressedRoundTrip(t *testing.T) {
	opts := []quicklist.Option[string]{
		quicklist.WithSegmentFill[string](4),
		quicklist.WithCompression[string](
			quicklist.LZWCodec(), 1,
			quicklist.EncodeStringSegment, quicklist.DecodeStringSegment),
	}
	src := NewQuickList(opts...)
	for i := 0; i < 40; i++ {
		src.PushTail(fmt.Sprintf("item-%d", i))
	}
	b, err := json.Marshal(src)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	dst := NewQuickList(opts...)
	dst.PushTail("old") // replaced by the unmarshal
	if err := json.Unmarshal(b, dst); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	var fromDst, fromSrc []string
	for _, v := range dst.All() {
		fromDst = append(fromDst, v)
	}
	for _, v := range src.All() {
		fromSrc = append(fromSrc, v)
	}
	if !slices.Equal(fromDst, fromSrc) {
		t.Errorf("Compressed round trip mismatch:\n got %v\nwant %v", fromDst, fromSrc)
	}
}

// TestUnmarshalJSONPanics verifies that UnmarshalJSON joins the insert
// family: storing elements into a nil list panics with a message naming
// the method, while [] and null — which store nothing — are tolerated
// everywhere.  A zero-value list needs no constructor, so storing into
// one is fine.
func TestUnmarshalJSONPanics(t *testing.T) {
	var nilList *QuickList[int]
	for _, data := range []string{"[]", "null"} {
		if err := nilList.UnmarshalJSON([]byte(data)); err != nil {
			t.Errorf("Expected %s on a nil list to be tolerated, got %v", data, err)
		}
	}
	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Errorf("Expected UnmarshalJSON with elements to panic on a nil list.")
				return
			}
			if msg, ok := r.(string); !ok || !strings.Contains(msg, "UnmarshalJSON") || !strings.Contains(msg, "nil QuickList") {
				t.Errorf("Unexpected panic message: %v", r)
			}
		}()
		_ = nilList.UnmarshalJSON([]byte("[1]"))
	}()
}
