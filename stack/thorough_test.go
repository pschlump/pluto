package stack

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Thorough tests: single-element and duplicate edge cases, value-copy
// semantics of Peek, iterator index sequences and live reflection, and a
// fixed-seed randomized property test cross-checked against a slice
// reference model.

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// checkModel verifies the stack's observable state against a reference
// model (a plain slice where the top is the last element).
func checkModel[T comparable](t *testing.T, stk *Stack[T], model []T) {
	t.Helper()
	if got, want := stk.Length(), len(model); got != want {
		t.Fatalf("Length: expected %d, got %d", want, got)
	}
	if got, want := stk.IsEmpty(), len(model) == 0; got != want {
		t.Fatalf("IsEmpty: expected %v, got %v", want, got)
	}
	if len(model) > 0 {
		top, err := stk.Peek()
		if err != nil {
			t.Fatalf("Peek on non-empty stack returned error: %v", err)
		}
		if top != model[len(model)-1] {
			t.Fatalf("Peek: expected %v, got %v", model[len(model)-1], top)
		}
	}
	// All must iterate top to bottom.
	var fwd []T
	for _, v := range stk.All() {
		fwd = append(fwd, v)
	}
	var wantFwd []T
	for _, m := range slices.Backward(model) {
		wantFwd = append(wantFwd, m)
	}
	if !slices.Equal(fwd, wantFwd) {
		t.Fatalf("All: expected %v, got %v", wantFwd, fwd)
	}
	// Backward must iterate bottom to top.
	var bwd []T
	for _, v := range stk.Backward() {
		bwd = append(bwd, v)
	}
	if !slices.Equal(bwd, model) {
		t.Fatalf("Backward: expected %v, got %v", model, bwd)
	}
}

func TestSingleElement(t *testing.T) {
	var stk Stack[string]

	stk.Push("only")
	if stk.IsEmpty() || stk.Length() != 1 {
		t.Errorf("Expected single-element stack, length %d", stk.Length())
	}
	if v, err := stk.Peek(); err != nil || v != "only" {
		t.Errorf("Peek = (%q, %v)", v, err)
	}
	if v, err := stk.Pop(); err != nil || v != "only" {
		t.Errorf("Pop = (%q, %v)", v, err)
	}
	if !stk.IsEmpty() {
		t.Errorf("Expected empty stack after popping the only element.")
	}
	if stk.data != nil {
		t.Errorf("Expected nil backing array after draining.")
	}
	checkModel(t, &stk, nil)
}

func TestDuplicateValues(t *testing.T) {
	var stk Stack[int]
	for _, v := range []int{5, 5, 5, 7} {
		stk.Push(v)
	}
	for i, want := range []int{7, 5, 5, 5} {
		if v, err := stk.Pop(); err != nil || v != want {
			t.Errorf("Pop step %d = (%v, %v), expected %d", i, v, err, want)
		}
	}
}

// TestPeekReturnsValue verifies that Peek returns an independent value:
// mutating it cannot affect the stack.
func TestPeekReturnsValue(t *testing.T) {
	type item struct {
		S string
		N int
	}
	var stk Stack[item]
	stk.Push(item{S: "a", N: 1})
	stk.Push(item{S: "b", N: 2})

	v, err := stk.Peek()
	if err != nil {
		t.Fatalf("Peek: %v", err)
	}
	v.N = 99 // mutate the returned value

	if got, err := stk.Peek(); err != nil || got.N != 2 {
		t.Errorf("Peek after mutation = (%v, %v), expected N=2 unaffected", got, err)
	}
	if stk.Length() != 2 {
		t.Errorf("Expected Peek to leave the length at 2, got %d", stk.Length())
	}
}

func TestLIFOOrder(t *testing.T) {
	var stk Stack[int]
	for i := range 100 {
		stk.Push(i)
	}
	for i := 99; i >= 0; i-- {
		v, err := stk.Pop()
		if err != nil {
			t.Fatalf("Pop(%d): %v", i, err)
		}
		if v != i {
			t.Fatalf("Pop = %d, expected %d", v, i)
		}
	}
	if !stk.IsEmpty() {
		t.Errorf("Expected empty stack after draining.")
	}
}

func TestPushPopInterleaved(t *testing.T) {
	var stk Stack[string]
	var model []string

	ops := []struct {
		push string
	}{
		{push: "a"}, {push: "b"}, {push: "c"},
	}
	for _, op := range ops {
		stk.Push(op.push)
		model = append(model, op.push)
		checkModel(t, &stk, model)
	}

	if v, err := stk.Pop(); err != nil || v != "c" {
		t.Fatalf("Pop = (%q, %v), expected c", v, err)
	}
	model = model[:len(model)-1]
	checkModel(t, &stk, model)

	stk.Push("d")
	model = append(model, "d")
	checkModel(t, &stk, model)
}

func TestBackwardEarlyBreak(t *testing.T) {
	var stk Stack[int]
	for i := range 5 {
		stk.Push(i)
	}

	n := 0
	var first int
	for _, v := range stk.Backward() {
		n++
		first = v
		break
	}
	if n != 1 {
		t.Errorf("Expected early break to yield exactly 1 item, got %d", n)
	}
	if first != 0 {
		t.Errorf("Expected first Backward item to be the bottom (0), got %d", first)
	}

	// Take exactly two items, bottom to top.
	var got []int
	for _, v := range stk.Backward() {
		got = append(got, v)
		if len(got) == 2 {
			break
		}
	}
	if expect := []int{0, 1}; !reflect.DeepEqual(got, expect) {
		t.Errorf("Expected [0 1] from partial Backward, got %v", got)
	}
}

// TestIteratorIndexSequences verifies the index numbering: All numbers
// from 0 at the top, Backward from 0 at the bottom.
func TestIteratorIndexSequences(t *testing.T) {
	var stk Stack[int]
	for i := range 4 {
		stk.Push(i) // 0,1,2,3 pushed; top is 3
	}

	var allIdx, allVal []int
	for i, v := range stk.All() {
		allIdx = append(allIdx, i)
		allVal = append(allVal, v)
	}
	if expect := []int{0, 1, 2, 3}; !reflect.DeepEqual(allIdx, expect) {
		t.Errorf("All indices: expected %v got %v", expect, allIdx)
	}
	if expect := []int{3, 2, 1, 0}; !reflect.DeepEqual(allVal, expect) {
		t.Errorf("All values: expected %v got %v", expect, allVal)
	}

	var bwdIdx, bwdVal []int
	for i, v := range stk.Backward() {
		bwdIdx = append(bwdIdx, i)
		bwdVal = append(bwdVal, v)
	}
	if expect := []int{0, 1, 2, 3}; !reflect.DeepEqual(bwdIdx, expect) {
		t.Errorf("Backward indices: expected %v got %v", expect, bwdIdx)
	}
	if expect := []int{0, 1, 2, 3}; !reflect.DeepEqual(bwdVal, expect) {
		t.Errorf("Backward values: expected %v got %v", expect, bwdVal)
	}
}

// TestIteratorReflectsLiveStack verifies that the iterators reflect
// pushes and pops that happen between iterations.
func TestIteratorReflectsLiveStack(t *testing.T) {
	var stk Stack[int]
	stk.Push(1)
	stk.Push(2)

	var seen []int
	for _, v := range stk.All() {
		seen = append(seen, v)
		break // stop after the top element
	}
	if expect := []int{2}; !reflect.DeepEqual(seen, expect) {
		t.Fatalf("First visit: expected %v got %v", expect, seen)
	}

	// Mutate, then iterate again: the change is visible.
	stk.Push(3)
	seen = nil
	for _, v := range stk.All() {
		seen = append(seen, v)
	}
	if expect := []int{3, 2, 1}; !reflect.DeepEqual(seen, expect) {
		t.Errorf("After push: expected %v got %v", expect, seen)
	}

	if _, err := stk.Pop(); err != nil {
		t.Fatalf("Pop: %v", err)
	}
	seen = nil
	for _, v := range stk.All() {
		seen = append(seen, v)
	}
	if expect := []int{2, 1}; !reflect.DeepEqual(seen, expect) {
		t.Errorf("After pop: expected %v got %v", expect, seen)
	}
}

func TestTruncateThenIterate(t *testing.T) {
	var stk Stack[int]
	for i := range 5 {
		stk.Push(i)
	}
	stk.Truncate()

	n := 0
	for range stk.All() {
		n++
	}
	for range stk.Backward() {
		n++
	}
	if n != 0 {
		t.Errorf("Expected no items from iterators on truncated stack, got %d", n)
	}

	// The stack is reusable.
	stk.Push(9)
	if v, err := stk.Pop(); err != nil || v != 9 {
		t.Errorf("Pop after Truncate = (%v, %v), expected 9", v, err)
	}
}

// TestRandomizedAgainstModel runs thousands of mixed operations against a
// slice reference model with a fixed seed, cross-checking along the way.
func TestRandomizedAgainstModel(t *testing.T) {
	rng := rand.New(rand.NewPCG(20260825, 13))
	const ops = 4000
	const keySpace = 50

	var stk Stack[int]
	var model []int

	for step := range ops {
		v := rng.IntN(keySpace)
		switch rng.IntN(3) {
		case 0, 1: // Push (weighted so the stack grows)
			stk.Push(v)
			model = append(model, v)
		case 2: // Pop
			got, err := stk.Pop()
			if len(model) == 0 {
				if !errors.Is(err, ErrEmptyStack) {
					t.Fatalf("step %d: Pop on empty returned %v", step, err)
				}
			} else {
				if err != nil || got != model[len(model)-1] {
					t.Fatalf("step %d: Pop = (%v, %v), model top %d", step, got, err, model[len(model)-1])
				}
				model = model[:len(model)-1]
			}
		}
		if step%50 == 0 {
			checkModel(t, &stk, model)
		}
	}
	checkModel(t, &stk, model)
}

// -------------------------------------------------------------------------------------------------------
// JSON — encoding/json integration (MarshalJSON/UnmarshalJSON in json.go).
// -------------------------------------------------------------------------------------------------------

// upperString is a string with its own JSON representation, to verify
// that element-level marshalers are honored through the stack.
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
	// Exact array output, top to bottom: pushing 1, 2, 3 leaves 3 on
	// top, so it is element 0.
	var stk Stack[int]
	for _, v := range []int{1, 2, 3} {
		stk.Push(v)
	}
	b, err := json.Marshal(&stk)
	if err != nil {
		t.Fatalf("json.Marshal(&stk): %v", err)
	}
	if string(b) != "[3,2,1]" {
		t.Errorf("Expected [3,2,1], got %s", b)
	}

	// Struct elements use their normal JSON encoding.
	type item struct {
		S string
	}
	var items Stack[item]
	items.Push(item{S: "a"})
	items.Push(item{S: "b"})
	if b, err := json.Marshal(&items); err != nil || string(b) != `[{"S":"b"},{"S":"a"}]` {
		t.Errorf(`Expected [{"S":"b"},{"S":"a"}], got (%s, %v)`, b, err)
	}

	// An empty stack encodes as [].
	var empty Stack[int]
	if b, err := json.Marshal(&empty); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] for an empty stack, got (%s, %v)", b, err)
	}

	// A direct call on a nil stack encodes as []; json.Marshal on a nil
	// *Stack never reaches the method — the json package writes null
	// for nil pointers itself.
	var nilStack *Stack[int]
	if b, err := nilStack.MarshalJSON(); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] from a direct nil-stack call, got (%s, %v)", b, err)
	}
	if b, err := json.Marshal(nilStack); err != nil || string(b) != "null" {
		t.Errorf("Expected null from json.Marshal on a nil stack, got (%s, %v)", b, err)
	}

	// Element-level marshalers are honored.
	var custom Stack[upperString]
	custom.Push("x")
	custom.Push("y")
	if b, err := json.Marshal(&custom); err != nil || string(b) != `["Y","X"]` {
		t.Errorf(`Expected ["Y","X"], got (%s, %v)`, b, err)
	}

	// Encoding errors pass through unchanged.
	var bad Stack[chan int]
	bad.Push(make(chan int))
	if _, err := json.Marshal(&bad); err == nil {
		t.Errorf("Expected an error marshaling a stack of channels.")
	}
}

func TestUnmarshalJSON(t *testing.T) {
	// Decoded order is preserved: element 0 becomes the top.
	var stk Stack[int]
	if err := json.Unmarshal([]byte("[3,1,2]"), &stk); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	checkModel(t, &stk, []int{2, 1, 3})
	if top, err := stk.Peek(); err != nil || top != 3 {
		t.Errorf("Expected top 3, got (%v, %v)", top, err)
	}

	// A round trip rebuilds an equivalent stack.
	var items Stack[string]
	for _, s := range []string{"a", "b", "c"} {
		items.Push(s)
	}
	b, err := json.Marshal(&items)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	var again Stack[string]
	if err := json.Unmarshal(b, &again); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	checkModel(t, &again, []string{"a", "b", "c"})

	// Unmarshaling replaces the contents; it does not push on top.
	if err := json.Unmarshal([]byte("[7]"), &stk); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if got, want := stk.Len(), 1; got != want {
		t.Errorf("Expected replacement, got length %d, want %d", got, want)
	}

	// An empty array and null clear the stack.
	var full Stack[string]
	full.Push("z")
	if err := json.Unmarshal([]byte("[]"), &full); err != nil {
		t.Fatalf("json.Unmarshal([]): %v", err)
	}
	if !full.IsEmpty() {
		t.Errorf("Expected [] to clear the stack.")
	}
	full.Push("z")
	if err := json.Unmarshal([]byte("null"), &full); err != nil {
		t.Fatalf("json.Unmarshal(null): %v", err)
	}
	if !full.IsEmpty() {
		t.Errorf("Expected null to clear the stack.")
	}
	checkModel(t, &full, nil)

	// Element-level unmarshalers are honored.
	var custom Stack[upperString]
	if err := json.Unmarshal([]byte(`["X","Y"]`), &custom); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	var cs []string
	for _, v := range custom.All() {
		cs = append(cs, string(v))
	}
	if fmt.Sprint(cs) != "[X Y]" {
		t.Errorf("Expected [X Y], got %v", cs)
	}

	// Decode errors are returned and leave the stack untouched.
	var keep Stack[string]
	keep.Push("keep")
	for _, badData := range []string{"[1,", `[1]`, `{"S":"a"}`, "7", `["a",3]`} {
		if err := json.Unmarshal([]byte(badData), &keep); err == nil {
			t.Errorf("Expected an error unmarshaling %s.", badData)
		}
		checkModel(t, &keep, []string{"keep"})
	}
}

// TestUnmarshalJSONPanics verifies that UnmarshalJSON follows the Push
// contract: storing elements into a nil stack panics with a message
// naming the method, while [] and null — which store nothing — are
// tolerated everywhere.  Unlike most pluto containers the zero value of
// Stack is fully usable, so unmarshaling into one works.
func TestUnmarshalJSONPanics(t *testing.T) {
	var zero Stack[string]
	for _, data := range []string{"[]", "null", `["a","b"]`} {
		if err := zero.UnmarshalJSON([]byte(data)); err != nil {
			t.Errorf("Expected %s on a zero-value stack to work, got %v", data, err)
		}
	}
	checkModel(t, &zero, []string{"b", "a"}) // element 0 ("a") is the top

	var nilStack *Stack[string]
	for _, data := range []string{"[]", "null"} {
		if err := nilStack.UnmarshalJSON([]byte(data)); err != nil {
			t.Errorf("Expected %s on a nil stack to be tolerated, got %v", data, err)
		}
	}
	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Errorf("Expected UnmarshalJSON with elements to panic on a nil stack.")
				return
			}
			if msg, ok := r.(string); !ok || !strings.Contains(msg, "UnmarshalJSON") || !strings.Contains(msg, "nil stack") {
				t.Errorf("Unexpected panic message: %v", r)
			}
		}()
		_ = nilStack.UnmarshalJSON([]byte(`["a"]`))
	}()
}

// TestJSONStructField marshals and unmarshals a Stack nested in a struct
// through the encoding/json package.
func TestJSONStructField(t *testing.T) {
	type Doc struct {
		Title string         `json:"title"`
		Tags  *Stack[string] `json:"tags"`
	}

	d := Doc{Title: "pluto", Tags: &Stack[string]{}}
	d.Tags.Push("ds")
	d.Tags.Push("go")

	b, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if string(b) != `{"title":"pluto","tags":["go","ds"]}` {
		t.Errorf("Unexpected document: %s", b)
	}

	var out Doc
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	var tags []string
	for _, v := range out.Tags.All() {
		tags = append(tags, v)
	}
	if fmt.Sprint(tags) != "[go ds]" {
		t.Errorf("Expected [go ds], got %v", tags)
	}

	// A nil stack field marshals as null (the json package's own nil
	// pointer rule); null clears a pre-existing stack.  Non-empty data
	// into a nil *Stack field works too: the json package allocates a
	// zero-value Stack, and the zero value is ready to use.
	if b, err := json.Marshal(Doc{Title: "x"}); err != nil || string(b) != `{"title":"x","tags":null}` {
		t.Errorf("Unexpected null document: (%s, %v)", b, err)
	}
	clearDoc := Doc{Title: "x", Tags: &Stack[string]{}}
	clearDoc.Tags.Push("gone")
	if err := json.Unmarshal([]byte(`{"title":"x","tags":null}`), &clearDoc); err != nil {
		t.Fatalf("json.Unmarshal with null tags: %v", err)
	}
	if !clearDoc.Tags.IsEmpty() {
		t.Errorf("Expected null tags to clear the stack.")
	}
	var grown Doc
	if err := json.Unmarshal([]byte(`{"title":"x","tags":["a"]}`), &grown); err != nil {
		t.Fatalf("json.Unmarshal into an uncreated stack field: %v", err)
	}
	if got, err := grown.Tags.Peek(); err != nil || got != "a" {
		t.Errorf("Expected the allocated stack to hold a, got (%q, %v)", got, err)
	}
}

// TestJSONRandomizedModel cross-checks marshaling and unmarshaling
// against a slice reference model at fixed seed.
func TestJSONRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewPCG(20260903, 42))
	const ops = 500

	var stk Stack[int]
	model := []int{} // non-nil, so an emptied model marshals as [] like the stack

	for step := range ops {
		switch rng.IntN(2) {
		case 0:
			v := rng.IntN(100)
			stk.Push(v)
			model = append(model, v)
		case 1:
			if len(model) > 0 {
				v, err := stk.Pop()
				if err != nil || v != model[len(model)-1] {
					t.Fatalf("step %d: Pop = (%v, %v), model top %d", step, v, err, model[len(model)-1])
				}
				model = model[:len(model)-1]
			}
		}

		// Marshal must equal the model marshaled top to bottom.
		topDown := []int{}
		for _, v := range slices.Backward(model) {
			topDown = append(topDown, v)
		}
		got, err := json.Marshal(&stk)
		if err != nil {
			t.Fatalf("step %d: %v", step, err)
		}
		want, _ := json.Marshal(topDown)
		if string(got) != string(want) {
			t.Fatalf("step %d: marshaled %s, model %s", step, got, want)
		}

		// Unmarshaling into a fresh stack must reproduce the model.
		var fresh Stack[int]
		if err := json.Unmarshal(got, &fresh); err != nil {
			t.Fatalf("step %d: %v", step, err)
		}
		checkModel(t, &fresh, model)
	}
}
