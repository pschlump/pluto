package stack_sll_ts

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Thorough tests: reference-model cross-checks, single-element and
// duplicate edge cases, value-copy semantics of Peek, truncate reuse, and
// a fixed-seed randomized property test.

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
func checkModel(t *testing.T, stk *Stack[int], model []int) {
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
	var fwd []int
	for _, v := range stk.All() {
		fwd = append(fwd, v)
	}
	var wantFwd []int
	for _, m := range slices.Backward(model) {
		wantFwd = append(wantFwd, m)
	}
	if !reflect.DeepEqual(fwd, wantFwd) {
		t.Fatalf("All: expected %v, got %v", wantFwd, fwd)
	}
	// Backward must iterate bottom to top.
	var bwd []int
	for _, v := range stk.Backward() {
		bwd = append(bwd, v)
	}
	if !reflect.DeepEqual(bwd, model) {
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
	if stk.head != nil {
		t.Errorf("Expected nil head after draining.")
	}
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

// TestPushPopInterleaved cross-checks against the model between every
// operation.
func TestPushPopInterleaved(t *testing.T) {
	var stk Stack[int]
	var model []int

	for _, v := range []int{1, 2, 3} {
		stk.Push(v)
		model = append(model, v)
		checkModel(t, &stk, model)
	}

	if v, err := stk.Pop(); err != nil || v != 3 {
		t.Fatalf("Pop = (%v, %v), expected 3", v, err)
	}
	model = model[:len(model)-1]
	checkModel(t, &stk, model)

	stk.Push(4)
	model = append(model, 4)
	checkModel(t, &stk, model)
}

// TestTruncateReuse verifies that the stack is fully reusable after a
// truncate.
func TestTruncateReuse(t *testing.T) {
	var stk Stack[int]
	for i := range 10 {
		stk.Push(i)
	}
	stk.Truncate()

	if !stk.IsEmpty() || stk.Length() != 0 {
		t.Errorf("Expected empty stack after Truncate.")
	}
	checkModel(t, &stk, nil)

	// Reusable after the drain.
	stk.Push(7)
	if v, err := stk.Peek(); err != nil || v != 7 {
		t.Errorf("Peek after Truncate = (%v, %v), expected 7", v, err)
	}

	// Truncating an already-empty stack is fine.
	stk.Truncate()
	stk.Truncate()
	if !stk.IsEmpty() {
		t.Errorf("Expected empty stack after double Truncate.")
	}
}

// TestRandomizedAgainstModel runs thousands of mixed operations against a
// slice reference model with a fixed seed, cross-checking along the way.
func TestRandomizedAgainstModel(t *testing.T) {
	rng := rand.New(rand.NewPCG(20260825, 17))
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
	// Exact array output, top to bottom: pushing 1, 2, 3 leaves 3 on top.
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
	// *Stack never reaches the method — the json package writes null for
	// nil pointers itself.
	var nilStk *Stack[int]
	if b, err := nilStk.MarshalJSON(); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] from a direct nil-stack call, got (%s, %v)", b, err)
	}
	if b, err := json.Marshal(nilStk); err != nil || string(b) != "null" {
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
	var got []int
	for _, v := range stk.All() {
		got = append(got, v)
	}
	if fmt.Sprint(got) != "[3 1 2]" {
		t.Errorf("Expected [3 1 2], got %v", got)
	}
	if top, err := stk.Peek(); err != nil || top != 3 {
		t.Errorf("Expected top 3, got (%v, %v)", top, err)
	}
	// Popping drains in LIFO order: 3, 1, 2.
	for _, want := range []int{3, 1, 2} {
		if v, err := stk.Pop(); err != nil || v != want {
			t.Errorf("Pop = (%v, %v), expected %d", v, err, want)
		}
	}

	// A round trip rebuilds an equivalent stack; the zero value is ready
	// to use, so the rebuilt stack needs no constructor.
	var orig Stack[string]
	for _, s := range []string{"a", "b", "c"} {
		orig.Push(s)
	}
	b, err := json.Marshal(&orig)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	var again Stack[string]
	if err := json.Unmarshal(b, &again); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	var vals []string
	for _, v := range again.All() {
		vals = append(vals, v)
	}
	if got, want := fmt.Sprint(vals), "[c b a]"; got != want {
		t.Errorf("Expected %s after round trip, got %s", want, got)
	}
	if top, err := again.Peek(); err != nil || top != "c" {
		t.Errorf("Expected top \"c\" after round trip, got (%q, %v)", top, err)
	}

	// Unmarshaling replaces the contents; it does not push on top.
	if err := json.Unmarshal([]byte("[7]"), &stk); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if got, want := stk.Len(), 1; got != want {
		t.Errorf("Expected replacement, got length %d, want %d", got, want)
	}

	// An empty array and null clear the stack.
	var full Stack[int]
	full.Push(9)
	if err := json.Unmarshal([]byte("[]"), &full); err != nil {
		t.Fatalf("json.Unmarshal([]): %v", err)
	}
	if !full.IsEmpty() {
		t.Errorf("Expected [] to clear the stack.")
	}
	full.Push(9)
	if err := json.Unmarshal([]byte("null"), &full); err != nil {
		t.Fatalf("json.Unmarshal(null): %v", err)
	}
	if !full.IsEmpty() {
		t.Errorf("Expected null to clear the stack.")
	}

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
	var keep Stack[int]
	keep.Push(42)
	for _, badData := range []string{"[1,", `["x"]`, `{"S":"a"}`, "7", `["a",3]`} {
		if err := json.Unmarshal([]byte(badData), &keep); err == nil {
			t.Errorf("Expected an error unmarshaling %s.", badData)
		}
		if got := keep.Len(); got != 1 {
			t.Errorf("Stack changed after the error on %s: length %d", badData, got)
		}
		if top, err := keep.Peek(); err != nil || top != 42 {
			t.Errorf("Stack changed after the error on %s: top (%v, %v)", badData, top, err)
		}
	}
}

// TestUnmarshalJSONPanics verifies that UnmarshalJSON joins the insert
// family: storing elements into a nil stack panics with the standard
// insert-family message, while [] and null — which store nothing — are
// tolerated everywhere.  A zero-value stack is ready to use (there is no
// constructor-set function), so storing into it works.
func TestUnmarshalJSONPanics(t *testing.T) {
	var zero Stack[int]
	for _, data := range []string{"[]", "null", "[1,2]"} {
		if err := zero.UnmarshalJSON([]byte(data)); err != nil {
			t.Errorf("Expected %s on a zero-value stack to be tolerated, got %v", data, err)
		}
	}
	if got := zero.Len(); got != 2 {
		t.Errorf("Expected the zero-value stack to hold 2 elements, got %d", got)
	}

	var nilStk *Stack[int]
	for _, data := range []string{"[]", "null"} {
		if err := nilStk.UnmarshalJSON([]byte(data)); err != nil {
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
		_ = nilStk.UnmarshalJSON([]byte("[1]"))
	}()
}

// TestJSONStructField marshals and unmarshals a Stack nested in a struct
// through the encoding/json package.  Unlike the constructor-based
// packages, a nil *Stack field that the json package allocates itself is
// a ready-to-use zero value, so non-empty data unmarshals without a
// panic.
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

	// Unmarshal into a pre-created stack field.
	var out Doc
	out.Tags = &Stack[string]{}
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
	// pointer rule); null clears a pre-created stack and never allocates.
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

	// Non-empty data into a nil *Stack field: the json package allocates
	// a zero-value stack, which is ready to use — no panic.
	var alloc Doc
	if err := json.Unmarshal([]byte(`{"title":"x","tags":["a"]}`), &alloc); err != nil {
		t.Fatalf("json.Unmarshal into an uncreated stack field: %v", err)
	}
	if top, err := alloc.Tags.Peek(); err != nil || top != "a" {
		t.Errorf("Expected top \"a\" in the allocated stack, got (%q, %v)", top, err)
	}
}

// TestJSONRandomizedModel cross-checks marshaling and unmarshaling
// against a slice reference model at fixed seed.
func TestJSONRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewPCG(20260902, 42))
	const ops = 500

	var stk Stack[int]
	var model []int

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
				if len(model) == 0 {
					model = nil // checkModel compares with DeepEqual: nil, not an empty slice
				}
			}
		}

		// Marshal must equal the model marshaled top-first as a plain
		// slice (the stack's top is the model's last element).
		topFirst := []int{} // non-nil, matches the [] encoding of an empty stack
		for _, m := range slices.Backward(model) {
			topFirst = append(topFirst, m)
		}
		got, err := json.Marshal(&stk)
		if err != nil {
			t.Fatalf("step %d: %v", step, err)
		}
		want, _ := json.Marshal(topFirst)
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
