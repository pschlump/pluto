package bag

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Thorough tests: reference-model cross-checks, link-integrity checks,
// duplicate edge cases, live-walk semantics of the iterators, truncate
// reuse, JSON round trips, and a fixed-seed randomized property test.

import (
	"encoding/json"
	"math/rand/v2"
	"slices"
	"testing"
)

// checkModel verifies the bag's observable state against a reference
// model (a plain slice in insertion order).
func checkModel[T comparable](t *testing.T, b *Bag[T], model []T) {
	t.Helper()
	if got, want := b.Length(), len(model); got != want {
		t.Fatalf("Length: expected %d, got %d", want, got)
	}
	if got, want := b.IsEmpty(), len(model) == 0; got != want {
		t.Fatalf("IsEmpty: expected %v, got %v", want, got)
	}
	// All must iterate in insertion order.  slices.Equal (not
	// reflect.DeepEqual) so that a nil result and an emptied model
	// compare equal.
	var fwd []T
	for _, v := range b.All() {
		fwd = append(fwd, v)
	}
	if !slices.Equal(fwd, model) {
		t.Fatalf("All: expected %v, got %v", model, fwd)
	}
	// Backward must iterate reverse-insertion order with indexes
	// counting down from Length()-1.
	var bwd []T
	var bwdIdx []int
	for i, v := range b.Backward() {
		bwd = append(bwd, v)
		bwdIdx = append(bwdIdx, i)
	}
	var wantBwd []T
	for _, m := range slices.Backward(model) {
		wantBwd = append(wantBwd, m)
	}
	if !slices.Equal(bwd, wantBwd) {
		t.Fatalf("Backward: expected %v, got %v", wantBwd, bwd)
	}
	for k, i := range bwdIdx {
		if i != len(model)-1-k {
			t.Fatalf("Backward index %d = %d, expected %d", k, i, len(model)-1-k)
		}
	}
}

// checkLinks verifies the structural invariants of the singly linked
// list: the forward walk from the head counts exactly Length nodes and
// ends at the tail, and the tail's next pointer is nil.  Call it after
// structural changes.
func checkLinks[T any](t *testing.T, b *Bag[T]) {
	t.Helper()

	n := 0
	var last *bagElement[T]
	for p := b.head; p != nil; p = p.next {
		last = p
		n++
	}
	if last != b.tail {
		t.Fatalf("forward walk ended at %v, tail is %v", last, b.tail)
	}
	if b.tail != nil && b.tail.next != nil {
		t.Fatalf("tail.next = %v, expected nil", b.tail.next)
	}
	if n != b.length {
		t.Fatalf("forward walk counted %d nodes, length is %d", n, b.length)
	}
	if (b.head == nil) != (b.length == 0) {
		t.Fatalf("head = %v with length %d: an empty bag must have a nil head", b.head, b.length)
	}
}

// TestSingleElement covers the single-element edge case: adding one
// element and truncating must leave an empty bag with nil head and
// tail.
func TestSingleElement(t *testing.T) {
	var b Bag[int]
	b.Add(42)
	checkLinks(t, &b)
	checkModel(t, &b, []int{42})

	b.Truncate()
	checkLinks(t, &b)
	checkModel(t, &b, nil)
	if b.head != nil || b.tail != nil {
		t.Fatalf("Truncate left head=%v tail=%v, expected nil/nil", b.head, b.tail)
	}
}

// TestDuplicateValues verifies the multiset contract: duplicates are
// kept, each Add is one element, and iteration order follows the order
// the duplicates were added.
func TestDuplicateValues(t *testing.T) {
	var b Bag[string]
	for _, s := range []string{"a", "b", "a", "c", "a", "b"} {
		b.Add(s)
	}
	checkLinks(t, &b)
	checkModel(t, &b, []string{"a", "b", "a", "c", "a", "b"})
	if b.Length() != 6 {
		t.Fatalf("Expected 6 elements counting duplicates, got %d", b.Length())
	}
}

// TestManyAddsThenTruncate drives a longer grow to exercise the tail
// append path and then releases everything at once.
func TestManyAddsThenTruncate(t *testing.T) {
	var b Bag[int]
	const n = 1000
	for i := range n {
		b.Add(i)
	}
	checkLinks(t, &b)
	if b.Length() != n {
		t.Fatalf("Expected %d elements, got %d", n, b.Length())
	}
	b.Truncate()
	checkLinks(t, &b)
	checkModel(t, &b, nil)
}

// TestTruncateReuse verifies that a bag emptied with Truncate keeps
// working for a fresh round of adds.
func TestTruncateReuse(t *testing.T) {
	var b Bag[int]
	for round := range 5 {
		for i := range 10 {
			b.Add(round*10 + i)
		}
		want := make([]int, 10)
		for i := range want {
			want[i] = round*10 + i
		}
		checkModel(t, &b, want)
		b.Truncate()
		checkModel(t, &b, nil)
	}
}

// TestIteratorReflectsLiveBag checks the plain package's contract that
// the iterators walk the live list (inverted from the ts twin's
// snapshot): an Add after taking the iterator is visible mid-walk.
func TestIteratorReflectsLiveBag(t *testing.T) {
	var b Bag[int]
	b.Add(1)
	b.Add(2)

	var seen []int
	for _, v := range b.All() {
		seen = append(seen, v)
		if len(seen) == 1 {
			b.Add(3) // visible to the walk that is still in progress
		}
	}
	if !slices.Equal(seen, []int{1, 2, 3}) {
		t.Fatalf("live walk: expected [1 2 3], got %v", seen)
	}
}

// TestIteratorEarlyExit verifies that breaking out of the iteration
// stops the walk.
func TestIteratorEarlyExit(t *testing.T) {
	var b Bag[int]
	for i := range 10 {
		b.Add(i)
	}
	count := 0
	for range b.All() {
		count++
		break
	}
	if count != 1 {
		t.Fatalf("Early exit: expected 1 yield, got %d", count)
	}
	count = 0
	for range b.Backward() {
		count++
		if count == 3 {
			break
		}
	}
	if count != 3 {
		t.Fatalf("Backward early exit: expected 3 yields, got %d", count)
	}
}

// TestRandomizedAgainstModel runs interleaved Add/Truncate/iterate
// rounds against a slice model, verifying every observable after each
// step.  Fixed PCG seed for reproducibility.
func TestRandomizedAgainstModel(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 7))

	var b Bag[int]
	var model []int
	for step := range 4000 {
		switch rng.IntN(8) {
		case 0:
			b.Truncate()
			model = nil
		default:
			v := rng.IntN(100) // duplicates are common
			b.Add(v)
			model = append(model, v)
		}
		if step%97 == 0 {
			checkLinks(t, &b)
		}
	}
	checkLinks(t, &b)
	checkModel(t, &b, model)
}

// TestMarshalJSON verifies the JSON array encoding, insertion order,
// and the empty/nil conventions.
func TestMarshalJSON(t *testing.T) {
	var b Bag[int]
	blob, err := json.Marshal(&b)
	if err != nil {
		t.Fatalf("Marshal empty bag: %v", err)
	}
	if string(blob) != "[]" {
		t.Fatalf("Marshal empty bag: expected [], got %s", blob)
	}

	b.Add(1)
	b.Add(2)
	b.Add(3)
	blob, err = json.Marshal(&b)
	if err != nil {
		t.Fatalf("Marshal bag: %v", err)
	}
	if string(blob) != "[1,2,3]" {
		t.Fatalf("Marshal bag: expected [1,2,3], got %s", blob)
	}

	var nilBag *Bag[int]
	blob, err = json.Marshal(nilBag) // the json package writes null itself
	if err != nil {
		t.Fatalf("Marshal nil bag: %v", err)
	}
	if string(blob) != "null" {
		t.Fatalf("Marshal nil bag: expected null, got %s", blob)
	}
	blob, err = nilBag.MarshalJSON() // a direct call uses the read contract
	if err != nil {
		t.Fatalf("direct MarshalJSON on nil bag: %v", err)
	}
	if string(blob) != "[]" {
		t.Fatalf("direct MarshalJSON on nil bag: expected [], got %s", blob)
	}
}

// TestUnmarshalJSON verifies decoding into a bag: contents replaced,
// order preserved, prior contents dropped, errors leaving the bag
// untouched, and the nil/empty conventions.
func TestUnmarshalJSON(t *testing.T) {
	var b Bag[int]
	b.Add(99)

	if err := json.Unmarshal([]byte(`[3,1,2,1]`), &b); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	checkModel(t, &b, []int{3, 1, 2, 1})

	// A decode error must leave the bag untouched.
	if err := json.Unmarshal([]byte(`[4,"x"]`), &b); err == nil {
		t.Fatalf("Expected a decode error for mixed types")
	}
	checkModel(t, &b, []int{3, 1, 2, 1})

	if err := json.Unmarshal([]byte(`"nope"`), &b); err == nil {
		t.Fatalf("Expected a decode error for a non-array document")
	}
	checkModel(t, &b, []int{3, 1, 2, 1})

	// null and [] clear the bag.
	if err := json.Unmarshal([]byte(`null`), &b); err != nil {
		t.Fatalf("Unmarshal null: %v", err)
	}
	checkModel(t, &b, nil)
	b.Add(7)
	if err := json.Unmarshal([]byte(`[]`), &b); err != nil {
		t.Fatalf("Unmarshal []: %v", err)
	}
	checkModel(t, &b, nil)

	// The zero value round-trips through the json package.
	var b2 Bag[string]
	if err := json.Unmarshal([]byte(`["a","b"]`), &b2); err != nil {
		t.Fatalf("Unmarshal into zero value: %v", err)
	}
	blob, err := json.Marshal(&b2)
	if err != nil {
		t.Fatalf("Marshal round trip: %v", err)
	}
	if string(blob) != `["a","b"]` {
		t.Fatalf("Round trip: expected [\"a\",\"b\"], got %s", blob)
	}
}

// TestJSONStructField verifies the json.Unmarshal behavior for a nil
// bag in a struct field: the json package allocates a zero-value bag —
// and unlike dll (whose inserts panic on a zero value for want of an
// equality function), a bag's zero value is fully usable, so the
// allocation is enough and non-empty data decodes into it.
func TestJSONStructField(t *testing.T) {
	type doc struct {
		Items *Bag[int] `json:"items"`
	}

	// Non-nil field: normal round trip.
	d1 := doc{Items: &Bag[int]{}}
	if err := json.Unmarshal([]byte(`{"items":[5,6]}`), &d1); err != nil {
		t.Fatalf("Unmarshal into non-nil field: %v", err)
	}
	checkModel(t, d1.Items, []int{5, 6})

	// Non-empty data into a nil field: the json package allocates the
	// zero-value bag and the decode succeeds.
	d2 := doc{}
	if err := json.Unmarshal([]byte(`{"items":[5,6]}`), &d2); err != nil {
		t.Fatalf("Unmarshal into nil field: %v", err)
	}
	if d2.Items == nil {
		t.Fatalf("Expected the json package to allocate the nil field bag")
	}
	checkModel(t, d2.Items, []int{5, 6})
}

// TestJSONElementTypeWithTags verifies that a T with its own JSON
// behavior is honored end to end.
func TestJSONElementTypeWithTags(t *testing.T) {
	type item struct {
		Name string `json:"name"`
	}
	var b Bag[item]
	b.Add(item{Name: "x"})
	blob, err := json.Marshal(&b)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if string(blob) != `[{"name":"x"}]` {
		t.Fatalf("Marshal tagged element: got %s", blob)
	}
	var b2 Bag[item]
	if err := json.Unmarshal(blob, &b2); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if b2.Length() != 1 {
		t.Fatalf("Expected 1 element after round trip, got %d", b2.Length())
	}
	for _, got := range b2.All() {
		if got != (item{Name: "x"}) {
			t.Fatalf("Round trip changed the element: %+v", got)
		}
	}
}

// TestValueSemantics verifies that elements are stored by value: after
// Add the bag holds its own copy, and mutating the caller's variable
// does not change the bag.
func TestValueSemantics(t *testing.T) {
	type big struct{ a, b, c int }
	var b Bag[big]
	v := big{1, 2, 3}
	b.Add(v)
	v.a = 99
	for _, got := range b.All() {
		if got != (big{1, 2, 3}) {
			t.Fatalf("Element aliases the caller's variable: %+v", got)
		}
	}
}
