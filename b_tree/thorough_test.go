package b_tree

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Thorough tests: the B-tree structural invariant checker (called after
// every structural change), sequential fill/drain stress across orders,
// Dump, and a fixed-seed randomized property test cross-checked against a
// sorted-slice reference model.

import (
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"sort"
	"strings"
	"testing"
)

// checkBTreeInvariants verifies the structural invariants of a B-tree:
//
//   - every non-root node holds between ceil(order/2)-1 and order-1 keys
//     (the root holds between 1 and order-1),
//   - keys are strictly ascending within every node,
//   - an internal node with k keys has exactly k+1 children, and every
//     subtree respects the key ranges of its separating keys,
//   - all leaves are at the same depth,
//   - the total number of keys equals tree.Length().
//
// It returns the number of keys in the tree.
func checkBTreeInvariants[T any](t *testing.T, tree *BTree[T]) int {
	t.Helper()
	if tree.root == nil {
		if tree.length != 0 {
			t.Fatalf("empty tree has length %d, expected 0", tree.length)
		}
		return 0
	}
	minKeys := tree.minKeys()
	leafDepth := -1

	var walk func(n *BTreeNode[T], depth int, lo, hi *T) int
	walk = func(n *BTreeNode[T], depth int, lo, hi *T) int {
		if len(n.keys) > tree.order-1 {
			t.Fatalf("node %v at depth %d has %d keys, maximum is %d", n.keys, depth, len(n.keys), tree.order-1)
		}
		if n == tree.root {
			if len(n.keys) == 0 {
				t.Fatalf("root has 0 keys")
			}
		} else if len(n.keys) < minKeys {
			t.Fatalf("node %v at depth %d has %d keys, minimum for a non-root is %d", n.keys, depth, len(n.keys), minKeys)
		}
		for i, k := range n.keys {
			if i > 0 && tree.cmp(n.keys[i-1], k) >= 0 {
				t.Fatalf("keys not strictly ascending in node %v", n.keys)
			}
			if lo != nil && tree.cmp(*lo, k) >= 0 {
				t.Fatalf("key %v not greater than the left separator %v", k, *lo)
			}
			if hi != nil && tree.cmp(k, *hi) >= 0 {
				t.Fatalf("key %v not less than the right separator %v", k, *hi)
			}
		}
		if n.children == nil {
			if leafDepth == -1 {
				leafDepth = depth
			} else if depth != leafDepth {
				t.Fatalf("leaf at depth %d, another leaf is at depth %d", depth, leafDepth)
			}
			return len(n.keys)
		}
		if len(n.children) != len(n.keys)+1 {
			t.Fatalf("internal node %v has %d keys and %d children", n.keys, len(n.keys), len(n.children))
		}
		total := len(n.keys)
		for i, c := range n.children {
			clo, chi := lo, hi
			if i > 0 {
				clo = &n.keys[i-1]
			}
			if i < len(n.keys) {
				chi = &n.keys[i]
			}
			total += walk(c, depth+1, clo, chi)
		}
		return total
	}

	count := walk(tree.root, 0, nil, nil)
	if count != tree.length {
		t.Fatalf("node key count %d does not match Length %d", count, tree.length)
	}
	return count
}

// expectPanic runs fx and fails the test unless it panics.
func expectPanic(t *testing.T, name string, fx func()) {
	t.Helper()
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Expected %s to panic, it did not.", name)
		}
	}()
	fx()
}

// failingWriter always returns an error from Write, to exercise the error
// branch in Dump.
type failingWriter struct{}

func (failingWriter) Write(p []byte) (int, error) {
	return 0, fmt.Errorf("write failed")
}

func TestTreeDump(t *testing.T) {
	tree := newTestTree(4)

	// Dump on an empty tree must not panic and must produce no output.
	var sb strings.Builder
	tree.Dump(&sb)
	if sb.Len() != 0 {
		t.Errorf("Dump on empty tree: expected no output, got %q", sb.String())
	}

	for _, v := range []int{5, 2, 9, 0, 3} {
		tree.Insert(mkKey(v))
	}
	sb.Reset()
	tree.Dump(&sb)
	out := sb.String()
	for _, v := range []int{0, 2, 3, 5, 9} {
		if !strings.Contains(out, mkKey(v).S) {
			t.Errorf("Dump: expected output to contain %q, got %q", mkKey(v).S, out)
		}
	}

	// Dump must not panic when the writer fails.
	tree.Dump(failingWriter{})
}

// TestTreeSequentialFills fills and drains trees of several orders with
// ascending and descending keys, checking the B-tree invariants after
// every structural change.  Ascending/descending input is the worst case
// for node splits and merges.
func TestTreeSequentialFills(t *testing.T) {
	const n = 300
	for _, order := range []int{3, 4, 5, 6, 7} {
		// Ascending fill, then ascending drain.
		tree := newTestTree(order)
		for i := range n {
			tree.Insert(mkKey(i))
			checkBTreeInvariants(t, tree)
		}
		if got := tree.Length(); got != n {
			t.Fatalf("order %d: expected length %d, got %d", order, n, got)
		}
		for i := range n {
			if !tree.Delete(mkKey(i)) {
				t.Fatalf("order %d: expected to delete %d", order, i)
			}
			checkBTreeInvariants(t, tree)
		}
		if !tree.IsEmpty() {
			t.Fatalf("order %d: expected empty tree after ascending drain", order)
		}

		// Descending fill, then drain alternately from both ends.
		for i := n - 1; i >= 0; i-- {
			tree.Insert(mkKey(i))
			checkBTreeInvariants(t, tree)
		}
		for i := range n {
			if i%2 == 0 {
				if !tree.DeleteAtHead() {
					t.Fatalf("order %d: DeleteAtHead failed at step %d", order, i)
				}
			} else {
				if !tree.DeleteAtTail() {
					t.Fatalf("order %d: DeleteAtTail failed at step %d", order, i)
				}
			}
			checkBTreeInvariants(t, tree)
		}
		if !tree.IsEmpty() || tree.Length() != 0 {
			t.Fatalf("order %d: expected empty tree after draining from both ends", order)
		}
	}
}

// TestTreeRandomModel is a fixed-seed randomized property test.  It
// performs hundreds of mixed operations (Insert, Delete, Search, FindMin,
// FindMax, DeleteAtHead, DeleteAtTail, iteration) on trees of several
// orders and cross-checks every result against a plain sorted-slice
// reference model.
func TestTreeRandomModel(t *testing.T) {
	for _, order := range []int{3, 4, 6} {
		rng := rand.New(rand.NewPCG(42, uint64(order)))
		const keySpace = 100 // small key space to force duplicate inserts

		tree := NewBTree[int](order)
		var model []int // sorted, unique

		contains := func(v int) bool {
			i := sort.SearchInts(model, v)
			return i < len(model) && model[i] == v
		}
		modelInsert := func(v int) {
			if !contains(v) {
				i := sort.SearchInts(model, v)
				model = append(model, 0)
				copy(model[i+1:], model[i:])
				model[i] = v
			}
		}
		modelDelete := func(v int) bool {
			i := sort.SearchInts(model, v)
			if i < len(model) && model[i] == v {
				model = append(model[:i], model[i+1:]...)
				return true
			}
			return false
		}
		checkAll := func(step int) {
			t.Helper()
			if got := tree.Length(); got != len(model) {
				t.Fatalf("order %d step %d: length mismatch: tree=%d model=%d", order, step, got, len(model))
			}
			// Full forward iteration must match the sorted model exactly.
			i := 0
			for pos, v := range tree.All() {
				if pos != i {
					t.Fatalf("order %d step %d: All() index %d, expected %d", order, step, pos, i)
				}
				if i >= len(model) || v != model[i] {
					t.Fatalf("order %d step %d: All()[%d]=%d, model has %v", order, step, i, v, model)
				}
				i++
			}
			if i != len(model) {
				t.Fatalf("order %d step %d: All() visited %d items, model has %d", order, step, i, len(model))
			}
			// Backward iteration must be the reverse.
			i = len(model) - 1
			for _, v := range tree.Backward() {
				if i < 0 || v != model[i] {
					t.Fatalf("order %d step %d: Backward()[%d]=%d, model has %v", order, step, i, v, model)
				}
				i--
			}
			checkBTreeInvariants(t, tree)
		}

		for step := range 800 {
			v := rng.IntN(keySpace)
			switch rng.IntN(8) {
			case 0, 1, 2: // Insert (may be a duplicate -> replace)
				added := tree.Insert(v)
				if added == contains(v) {
					t.Fatalf("order %d step %d: Insert(%d)=%v, model said present=%v", order, step, v, added, contains(v))
				}
				modelInsert(v)
			case 3, 4: // Delete
				if got := tree.Delete(v); got != modelDelete(v) {
					t.Fatalf("order %d step %d: Delete(%d) returned %v, model said %v", order, step, v, got, contains(v))
				}
			case 5: // Search
				_, found := tree.Search(v)
				if want := contains(v); found != want {
					t.Fatalf("order %d step %d: Search(%d) found=%v, model says %v", order, step, v, found, want)
				}
			case 6: // FindMin / FindMax
				mn, mnOK := tree.FindMin()
				mx, mxOK := tree.FindMax()
				if len(model) == 0 {
					if mnOK || mxOK {
						t.Fatalf("order %d step %d: FindMin/FindMax on empty tree returned %v/%v", order, step, mn, mx)
					}
				} else if mn != model[0] || mx != model[len(model)-1] {
					t.Fatalf("order %d step %d: FindMin=%d FindMax=%d, model %v", order, step, mn, mx, model)
				}
			case 7: // DeleteAtHead / DeleteAtTail
				if rng.IntN(2) == 0 {
					got := tree.DeleteAtHead()
					if len(model) == 0 {
						if got {
							t.Fatalf("order %d step %d: DeleteAtHead on empty tree returned true", order, step)
						}
					} else if got {
						modelDelete(model[0])
					} else {
						t.Fatalf("order %d step %d: DeleteAtHead failed on non-empty tree", order, step)
					}
				} else {
					got := tree.DeleteAtTail()
					if len(model) == 0 {
						if got {
							t.Fatalf("order %d step %d: DeleteAtTail on empty tree returned true", order, step)
						}
					} else if got {
						modelDelete(model[len(model)-1])
					} else {
						t.Fatalf("order %d step %d: DeleteAtTail failed on non-empty tree", order, step)
					}
				}
			}
			checkBTreeInvariants(t, tree)
			if step%50 == 0 {
				checkAll(step)
			}
		}
		checkAll(800)
	}
}

// -------------------------------------------------------------------------------------------------------
// JSON — encoding/json integration (MarshalJSON/UnmarshalJSON in json.go).
// -------------------------------------------------------------------------------------------------------

// upperString is a string with its own JSON representation, to verify
// that element-level marshalers are honored through the tree.
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

// newUpperTree builds a BTree over upperString, ordered by the string
// value.
func newUpperTree(order int) *BTree[upperString] {
	return NewBTreeFunc(order, func(a, b upperString) int {
		return strings.Compare(string(a), string(b))
	})
}

func TestMarshalJSON(t *testing.T) {
	// Exact array output in ascending order, regardless of insert order.
	tree := NewBTree[int](4)
	for _, v := range []int{3, 1, 2} {
		tree.Insert(v)
	}
	b, err := json.Marshal(tree)
	if err != nil {
		t.Fatalf("json.Marshal(tree): %v", err)
	}
	if string(b) != "[1,2,3]" {
		t.Errorf("Expected [1,2,3], got %s", b)
	}

	// Struct elements use their normal JSON encoding, sorted by the
	// comparison function.
	items := newTestTree(4)
	items.Insert(TestBTreeNode{S: "b"})
	items.Insert(TestBTreeNode{S: "a", N: 7})
	if b, err := json.Marshal(items); err != nil || string(b) != `[{"S":"a","N":7},{"S":"b","N":0}]` {
		t.Errorf(`Expected [{"S":"a","N":7},{"S":"b","N":0}], got (%s, %v)`, b, err)
	}

	// An empty tree encodes as [].
	if b, err := json.Marshal(NewBTree[int](4)); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] for an empty tree, got (%s, %v)", b, err)
	}

	// A zero-value tree is a tolerated read: [].
	var zero BTree[int]
	if b, err := zero.MarshalJSON(); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] for a zero-value tree, got (%s, %v)", b, err)
	}

	// A direct call on a nil tree encodes as []; json.Marshal on a nil
	// *BTree never reaches the method — the json package writes null for
	// nil pointers itself.
	var nilTree *BTree[int]
	if b, err := nilTree.MarshalJSON(); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] from a direct nil-tree call, got (%s, %v)", b, err)
	}
	if b, err := json.Marshal(nilTree); err != nil || string(b) != "null" {
		t.Errorf("Expected null from json.Marshal on a nil tree, got (%s, %v)", b, err)
	}

	// Element-level marshalers are honored.
	custom := newUpperTree(4)
	custom.Insert("x")
	custom.Insert("y")
	if b, err := json.Marshal(custom); err != nil || string(b) != `["X","Y"]` {
		t.Errorf(`Expected ["X","Y"], got (%s, %v)`, b, err)
	}

	// Encoding errors pass through unchanged.
	bad := NewBTreeFunc(4, func(a, b chan int) int { return 0 })
	bad.Insert(make(chan int))
	if _, err := json.Marshal(bad); err == nil {
		t.Errorf("Expected an error marshaling a tree of channels.")
	}
}

func TestUnmarshalJSON(t *testing.T) {
	// Decoded elements are inserted in array order; the tree is an ordered
	// set, so iteration afterwards is ascending regardless of that order.
	tree := NewBTree[int](4)
	if err := json.Unmarshal([]byte("[3,1,2]"), tree); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	var got []int
	for _, v := range tree.All() {
		got = append(got, v)
	}
	if fmt.Sprint(got) != "[1 2 3]" {
		t.Errorf("Expected [1 2 3], got %v", got)
	}
	if min, found := tree.FindMin(); !found || min != 1 {
		t.Errorf("Expected min 1, got (%v, %v)", min, found)
	}

	// A round trip rebuilds a structurally sound tree and keeps the
	// comparison function (Search works on the rebuilt tree).
	items := newTestTree(4)
	for _, s := range []string{"a", "b", "c"} {
		items.Insert(TestBTreeNode{S: s})
	}
	b, err := json.Marshal(items)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	again := newTestTree(4)
	if err := json.Unmarshal(b, again); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	checkBTreeInvariants(t, again)
	var keys []string
	for _, v := range again.All() {
		keys = append(keys, v.S)
	}
	if got, want := fmt.Sprint(keys), "[a b c]"; got != want {
		t.Errorf("Expected %s after round trip, got %s", want, got)
	}
	if _, found := again.Search(TestBTreeNode{S: "b"}); !found {
		t.Errorf("Expected Search to find b after unmarshal")
	}

	// Unmarshaling replaces the contents; it does not merge.
	if err := json.Unmarshal([]byte("[7]"), tree); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if got, want := tree.Len(), 1; got != want {
		t.Errorf("Expected replacement, got length %d, want %d", got, want)
	}

	// An empty array and null clear the tree.
	full := newTestTree(4)
	full.Insert(TestBTreeNode{S: "z"})
	if err := json.Unmarshal([]byte("[]"), full); err != nil {
		t.Fatalf("json.Unmarshal([]): %v", err)
	}
	if !full.IsEmpty() {
		t.Errorf("Expected [] to clear the tree.")
	}
	full.Insert(TestBTreeNode{S: "z"})
	if err := json.Unmarshal([]byte("null"), full); err != nil {
		t.Fatalf("json.Unmarshal(null): %v", err)
	}
	if !full.IsEmpty() {
		t.Errorf("Expected null to clear the tree.")
	}
	checkBTreeInvariants(t, full)

	// Element-level unmarshalers are honored.
	custom := newUpperTree(4)
	if err := json.Unmarshal([]byte(`["Y","X"]`), custom); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	var cs []string
	for _, v := range custom.All() {
		cs = append(cs, string(v))
	}
	if fmt.Sprint(cs) != "[X Y]" {
		t.Errorf("Expected [X Y], got %v", cs)
	}

	// Decode errors are returned and leave the tree untouched.
	keep := newTestTree(4)
	keep.Insert(TestBTreeNode{S: "keep"})
	for _, badData := range []string{"[1,", `["x"]`, `{"S":"a"}`, "7", `[{"S":3}]`} {
		if err := json.Unmarshal([]byte(badData), keep); err == nil {
			t.Errorf("Expected an error unmarshaling %s.", badData)
		}
		got, found := keep.Search(TestBTreeNode{S: "keep"})
		if !found || keep.Length() != 1 || got.S != "keep" {
			t.Errorf("Tree changed after the error on %s: %v", badData, got)
		}
	}
	checkBTreeInvariants(t, keep)
}

// TestUnmarshalJSONPanics verifies that UnmarshalJSON joins the insert
// family: storing elements into a nil or zero-value tree panics with a
// message naming the method and the fix, while [] and null — which
// store nothing — are tolerated everywhere.
func TestUnmarshalJSONPanics(t *testing.T) {
	var zero BTree[TestBTreeNode]
	for _, data := range []string{"[]", "null"} {
		if err := zero.UnmarshalJSON([]byte(data)); err != nil {
			t.Errorf("Expected %s on a zero-value tree to be tolerated, got %v", data, err)
		}
	}
	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Errorf("Expected UnmarshalJSON with elements to panic on a zero-value tree.")
				return
			}
			if msg, ok := r.(string); !ok || !strings.Contains(msg, "UnmarshalJSON") || !strings.Contains(msg, "NewBTree") {
				t.Errorf("Unexpected panic message: %v", r)
			}
		}()
		_ = zero.UnmarshalJSON([]byte(`[{"S":"a"}]`))
	}()

	var nilTree *BTree[TestBTreeNode]
	if err := nilTree.UnmarshalJSON([]byte("[]")); err != nil {
		t.Errorf("Expected [] on a nil tree to be tolerated, got %v", err)
	}
	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Errorf("Expected UnmarshalJSON with elements to panic on a nil tree.")
				return
			}
			if msg, ok := r.(string); !ok || !strings.Contains(msg, "UnmarshalJSON") || !strings.Contains(msg, "nil tree") {
				t.Errorf("Unexpected panic message: %v", r)
			}
		}()
		_ = nilTree.UnmarshalJSON([]byte(`[{"S":"a"}]`))
	}()
}

// TestJSONStructField marshals and unmarshals a BTree nested in a struct
// through the encoding/json package.  The tree must be created with
// NewBTree/NewBTreeFunc before unmarshaling: for a nil *BTree field the
// json package allocates a zero-value tree itself (no comparison
// function), so non-empty data panics with the insert-family message.
func TestJSONStructField(t *testing.T) {
	type Doc struct {
		Title string         `json:"title"`
		Tags  *BTree[string] `json:"tags"`
	}

	d := Doc{Title: "pluto", Tags: NewBTree[string](4)}
	d.Tags.Insert("go")
	d.Tags.Insert("ds")

	b, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if string(b) != `{"title":"pluto","tags":["ds","go"]}` {
		t.Errorf("Unexpected document: %s", b)
	}

	// Unmarshal into a pre-created tree field.
	var out Doc
	out.Tags = NewBTree[string](4)
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	var tags []string
	for _, v := range out.Tags.All() {
		tags = append(tags, v)
	}
	if fmt.Sprint(tags) != "[ds go]" {
		t.Errorf("Expected [ds go], got %v", tags)
	}

	// A nil tree field marshals as null (the json package's own nil
	// pointer rule); null clears a pre-created tree and never allocates.
	if b, err := json.Marshal(Doc{Title: "x"}); err != nil || string(b) != `{"title":"x","tags":null}` {
		t.Errorf("Unexpected null document: (%s, %v)", b, err)
	}
	clearDoc := Doc{Title: "x", Tags: NewBTree[string](4)}
	clearDoc.Tags.Insert("gone")
	if err := json.Unmarshal([]byte(`{"title":"x","tags":null}`), &clearDoc); err != nil {
		t.Fatalf("json.Unmarshal with null tags: %v", err)
	}
	if !clearDoc.Tags.IsEmpty() {
		t.Errorf("Expected null tags to clear the tree.")
	}

	// Non-empty data into a nil *BTree field: the json package allocates
	// a zero-value tree, and the insert contract panics through
	// json.Unmarshal (it does not recover panics).
	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Errorf("Expected unmarshaling into an uncreated tree field to panic.")
				return
			}
			if msg, ok := r.(string); !ok || !strings.Contains(msg, "NewBTree") {
				t.Errorf("Unexpected panic message: %v", r)
			}
		}()
		var bad Doc
		_ = json.Unmarshal([]byte(`{"title":"x","tags":["a"]}`), &bad)
	}()
}

// TestJSONRandomizedModel cross-checks marshaling and unmarshaling
// against a sorted-slice reference model at fixed seed.
func TestJSONRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewPCG(20260903, 42))
	const ops = 500

	tree := NewBTree[int](4)
	model := []int{} // sorted, unique; non-nil so an emptied model marshals as [] like the tree

	contains := func(v int) bool {
		i := sort.SearchInts(model, v)
		return i < len(model) && model[i] == v
	}
	modelInsert := func(v int) {
		if !contains(v) {
			i := sort.SearchInts(model, v)
			model = append(model, 0)
			copy(model[i+1:], model[i:])
			model[i] = v
		}
	}
	modelDelete := func(v int) {
		i := sort.SearchInts(model, v)
		if i < len(model) && model[i] == v {
			model = append(model[:i], model[i+1:]...)
		}
	}

	for step := range ops {
		v := rng.IntN(100)
		switch rng.IntN(2) {
		case 0:
			tree.Insert(v)
			modelInsert(v)
		case 1:
			tree.Delete(v)
			modelDelete(v)
		}

		// Marshal must equal the model marshaled as a plain slice.
		got, err := json.Marshal(tree)
		if err != nil {
			t.Fatalf("step %d: %v", step, err)
		}
		want, _ := json.Marshal(model)
		if string(got) != string(want) {
			t.Fatalf("step %d: marshaled %s, model %s", step, got, want)
		}

		// Unmarshaling into a fresh tree must reproduce the model.
		fresh := NewBTree[int](4)
		if err := json.Unmarshal(got, fresh); err != nil {
			t.Fatalf("step %d: %v", step, err)
		}
		checkBTreeInvariants(t, fresh)
		var vals []int
		for _, v := range fresh.All() {
			vals = append(vals, v)
		}
		if fmt.Sprint(vals) != fmt.Sprint(model) {
			t.Fatalf("step %d: round trip got %v, model %v", step, vals, model)
		}
	}
}
