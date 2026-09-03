package splay_tree

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"
)

// -------------------------------------------------------------------------------------------------------
// Helpers
// -------------------------------------------------------------------------------------------------------

// expectPanic runs fx and fails the test unless it panics.
func expectPanic(t *testing.T, name string, fx func()) {
	t.Helper()
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Expected %s to panic on a nil tree, it did not.", name)
		}
	}()
	fx()
}

// inOrder returns the keys of the tree in in-order (ascending) order.
// It always returns a non-nil slice so empty-tree comparisons work with
// reflect.DeepEqual.
func inOrder(tt *SplayTree[int]) []int {
	got := []int{}
	for _, v := range tt.All() {
		got = append(got, v)
	}
	return got
}

// countNodes counts the nodes of the tree by walking the child pointers,
// independently of the tree's cached length.
func countNodes(cur *SplayTreeElement[int]) int {
	if cur == nil {
		return 0
	}
	return 1 + countNodes(cur.left) + countNodes(cur.right)
}

// checkInvariants verifies the BST ordering invariant, that Length matches
// the number of nodes, and that every in-order key is found by Search.
// Call it after every structural change; note that the Search calls it
// makes splay the tree, which must leave the invariants intact.
func checkInvariants(t *testing.T, tt *SplayTree[int]) {
	t.Helper()
	got := inOrder(tt)
	if !sort.IntsAreSorted(got) {
		t.Errorf("BST invariant violated: in-order walk is not sorted: %v", got)
	}
	if len(got) != tt.Length() {
		t.Errorf("Length mismatch: Length()=%d but in-order walk has %d nodes", tt.Length(), len(got))
	}
	if n := countNodes(tt.root); n != tt.Length() {
		t.Errorf("Length mismatch: Length()=%d but the tree has %d nodes", tt.Length(), n)
	}
	seen := make(map[int]bool, len(got))
	for _, k := range got {
		if seen[k] {
			t.Errorf("BST invariant violated: duplicate key %d in tree", k)
		}
		seen[k] = true
		if _, found := tt.Search(k); !found {
			t.Errorf("Search failed to find in-order key %d", k)
		}
		// Search splays the found key to the root.
		if tt.root == nil || tt.root.data != k {
			t.Errorf("Search(%d) did not splay the key to the root (root=%v)", k, tt.root)
		}
	}
	if len(got) == 0 && !tt.IsEmpty() {
		t.Errorf("IsEmpty() is false but the tree has no nodes")
	}
	if len(got) > 0 && tt.IsEmpty() {
		t.Errorf("IsEmpty() is true but the tree has %d nodes", len(got))
	}
}

// -------------------------------------------------------------------------------------------------------
// Constructor and nil-tree panics
// -------------------------------------------------------------------------------------------------------

func TestTreeNewSplayTree(t *testing.T) {
	tree := newTestTree()
	if tree == nil {
		t.Fatalf("NewSplayTreeFunc returned nil.")
	}
	if !tree.IsEmpty() {
		t.Errorf("Expected new tree to be empty.")
	}
	if tree.Len() != 0 || tree.Length() != 0 {
		t.Errorf("Expected new tree to have length 0, got %d/%d", tree.Len(), tree.Length())
	}
	if tree.Depth() != 0 {
		t.Errorf("Expected new tree to have depth 0.")
	}
	// The constructed tree must be usable.
	if !tree.Insert(TestTreeNode{S: "05"}) {
		t.Errorf("Expected Insert on constructed tree to return true.")
	}
	if tree.Length() != 1 {
		t.Errorf("Expected length 1 after insert, got %d", tree.Length())
	}

	// Same checks for the natural-ordering constructor.
	ordered := NewSplayTree[string]()
	if !ordered.IsEmpty() || ordered.Len() != 0 || ordered.Depth() != 0 {
		t.Errorf("Expected NewSplayTree to start empty.")
	}
	if !ordered.Insert("05") {
		t.Errorf("Expected Insert on NewSplayTree tree to return true.")
	}
}

// TestTreeNilPanics verifies the documented panic when Insert is called on
// a nil tree — the one operation with no sane answer, since a nil tree
// cannot store an element.
func TestTreeNilPanics(t *testing.T) {
	var nilTree *SplayTree[TestTreeNode]
	key := TestTreeNode{S: "05"}

	expectPanic(t, "Insert", func() { nilTree.Insert(key) })

	// Verify the panic message names the method.
	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Errorf("Expected Insert to panic on nil tree.")
				return
			}
			if msg, ok := r.(string); !ok || !strings.Contains(msg, "Insert") {
				t.Errorf("Unexpected panic message: %v", r)
			}
		}()
		nilTree.Insert(key)
	}()
}

// TestTreeNilTolerated verifies that every operation other than Insert
// treats a nil tree as an empty tree instead of panicking.
func TestTreeNilTolerated(t *testing.T) {
	var nilTree *SplayTree[TestTreeNode]
	key := TestTreeNode{S: "05"}

	if !nilTree.IsEmpty() {
		t.Errorf("Expected nil tree to be empty.")
	}
	if nilTree.Len() != 0 || nilTree.Length() != 0 {
		t.Errorf("Expected nil tree to have length 0.")
	}
	if _, found := nilTree.Search(key); found {
		t.Errorf("Expected not-found from Search on nil tree.")
	}
	if nilTree.Delete(key) {
		t.Errorf("Expected false from Delete on nil tree.")
	}
	if _, found := nilTree.FindMin(); found {
		t.Errorf("Expected not-found from FindMin on nil tree.")
	}
	if _, found := nilTree.FindMax(); found {
		t.Errorf("Expected not-found from FindMax on nil tree.")
	}
	if nilTree.DeleteAtHead() || nilTree.DeleteAtTail() {
		t.Errorf("Expected false from DeleteAtHead/DeleteAtTail on nil tree.")
	}
	if nilTree.Depth() != 0 {
		t.Errorf("Expected depth 0 on nil tree.")
	}
	nilTree.Truncate() // no-op, must not panic

	for range nilTree.All() {
		t.Errorf("Expected no values from All on nil tree.")
	}
	for range nilTree.Backward() {
		t.Errorf("Expected no values from Backward on nil tree.")
	}

	var buf bytes.Buffer
	nilTree.Dump(&buf)
	if buf.Len() != 0 {
		t.Errorf("Expected no output from Dump on nil tree, got %q", buf.String())
	}
}

// -------------------------------------------------------------------------------------------------------
// Dump
// -------------------------------------------------------------------------------------------------------

// failingWriter always returns an error, to exercise the error path in Dump.
type failingWriter struct{}

func (failingWriter) Write(p []byte) (int, error) {
	return 0, errors.New("write failed")
}

func TestTreeDump(t *testing.T) {
	Tree1 := newTestTree()

	// Dump of an empty tree produces no output.
	var buf bytes.Buffer
	Tree1.Dump(&buf)
	if buf.Len() != 0 {
		t.Errorf("Expected no output from Dump on empty tree, got %q", buf.String())
	}

	Tree1.Insert(TestTreeNode{S: "05"})
	Tree1.Insert(TestTreeNode{S: "02"})
	Tree1.Insert(TestTreeNode{S: "09"})
	Tree1.Insert(TestTreeNode{S: "00"})
	Tree1.Insert(TestTreeNode{S: "03"})

	buf.Reset()
	Tree1.Dump(&buf)
	out := buf.String()
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 5 {
		t.Errorf("Expected 5 lines from Dump, got %d:\n%s", len(lines), out)
	}
	// Dump is an in-order traversal, so keys appear in ascending order.
	// Values are printed with %v, so a TestTreeNode shows up as "{00 0}".
	var keys []string
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			t.Fatalf("Unexpected empty line in Dump output: %q", out)
		}
		keys = append(keys, strings.Trim(fields[0], "{}"))
	}
	if expect := []string{"00", "02", "03", "05", "09"}; !reflect.DeepEqual(keys, expect) {
		t.Errorf("Dump order error, expected %s got %s", expect, keys)
	}
	// The last inserted node (03) was splayed to the root, so it has the
	// least indentation; every other line is indented more.
	rootIndent := strings.Index(lines[2], "03")
	for i, line := range lines {
		if i == 2 {
			continue
		}
		if strings.Index(line, "{") <= rootIndent {
			t.Errorf("Expected non-root line %d to be indented more than the root:\n%s", i, out)
		}
	}

	// A failing writer must not panic; the traversal just stops early.
	Tree1.Dump(failingWriter{})
}

// -------------------------------------------------------------------------------------------------------
// Single-element and degenerate shapes
// -------------------------------------------------------------------------------------------------------

func TestTreeSingleElement(t *testing.T) {
	Tree1 := newTestTree()
	Tree1.Insert(TestTreeNode{S: "05"})

	if Tree1.IsEmpty() {
		t.Errorf("Expected non-empty tree.")
	}
	if Tree1.Length() != 1 {
		t.Errorf("Expected length 1, got %d", Tree1.Length())
	}
	if Tree1.Depth() != 1 {
		t.Errorf("Expected depth 1, got %d", Tree1.Depth())
	}
	if x, found := Tree1.FindMin(); !found || x.S != "05" {
		t.Errorf("Expected min 05, got %+v", x)
	}
	if x, found := Tree1.FindMax(); !found || x.S != "05" {
		t.Errorf("Expected max 05, got %+v", x)
	}
	var got []string
	for _, v := range Tree1.All() {
		got = append(got, v.S)
	}
	if !reflect.DeepEqual(got, []string{"05"}) {
		t.Errorf("Expected in-order [05], got %v", got)
	}
	var bk []string
	for _, v := range Tree1.Backward() {
		bk = append(bk, v.S)
	}
	if !reflect.DeepEqual(bk, []string{"05"}) {
		t.Errorf("Expected backward [05], got %v", bk)
	}

	// DeleteAtHead then re-insert; DeleteAtTail on the fresh node.
	if !Tree1.DeleteAtHead() {
		t.Errorf("Expected DeleteAtHead to return true.")
	}
	if !Tree1.IsEmpty() || Tree1.Length() != 0 {
		t.Errorf("Expected empty tree after DeleteAtHead.")
	}
	Tree1.Insert(TestTreeNode{S: "07"})
	if !Tree1.DeleteAtTail() {
		t.Errorf("Expected DeleteAtTail to return true.")
	}
	if !Tree1.IsEmpty() {
		t.Errorf("Expected empty tree after DeleteAtTail.")
	}
}

// TestTreeSortedInput exercises the self-adjusting property: inserting
// sorted input builds (as with a plain BST) a degenerate chain — this is
// the O(n) worst case of a single operation — but a single Search for the
// deepest node splays it to the root and roughly halves the depth of the
// whole tree.
func TestTreeSortedInput(t *testing.T) {
	tree := NewSplayTree[int]()
	const n = 128
	for i := range n {
		tree.Insert(i)
		checkInvariants(t, tree)
	}
	if d := tree.Depth(); d != n {
		t.Errorf("Expected depth %d (a chain) right after sorted inserts, got %d", n, d)
	}

	// Splaying the deepest node (the minimum, at the bottom of the chain)
	// to the root must dramatically reduce the depth — the mechanism behind
	// the amortized O(log₂ n) bound.
	if _, found := tree.Search(0); !found {
		t.Fatalf("Expected to find 0.")
	}
	if d := tree.Depth(); d > n/2+4 {
		t.Errorf("Expected depth near %d after splaying the deepest node, got %d", n/2, d)
	}

	// Every key is searchable, and the tree remains fully intact.
	for i := range n {
		if _, found := tree.Search(i); !found {
			t.Fatalf("Expected to find %d.", i)
		}
	}
	checkInvariants(t, tree)
}

// TestTreeDeleteChildShapes exercises Delete for leaf, single-child and
// two-children victims, verifying the surviving set after each removal.
func TestTreeDeleteChildShapes(t *testing.T) {
	sorted := []int{5, 10, 15, 20, 25, 28, 30, 40}
	build := func() *SplayTree[int] {
		tree := NewSplayTree[int]()
		for _, k := range []int{20, 10, 30, 5, 15, 25, 40, 28} {
			tree.Insert(k)
		}
		return tree
	}
	remove := func(k int) []int {
		var out []int
		for _, v := range sorted {
			if v != k {
				out = append(out, v)
			}
		}
		return out
	}

	// Delete each key from a fresh tree, checking the survivors each time.
	for _, k := range sorted {
		tree := build()
		if !tree.Delete(k) {
			t.Fatalf("expected Delete(%d) to return true", k)
		}
		if got := inOrder(tree); !reflect.DeepEqual(got, remove(k)) {
			t.Errorf("after deleting %d: expected %v got %v", k, remove(k), got)
		}
		checkInvariants(t, tree)
	}

	// Deleting a key that is not in the tree returns false and keeps the set.
	tree := build()
	if tree.Delete(12) {
		t.Errorf("expected Delete of missing key to return false")
	}
	if got := inOrder(tree); !reflect.DeepEqual(got, sorted) {
		t.Errorf("missing-key delete changed the tree: expected %v got %v", sorted, got)
	}
	checkInvariants(t, tree)

	// Delete every node, one at a time, in-order.
	tree = build()
	for i, k := range sorted {
		if !tree.Delete(k) {
			t.Fatalf("expected Delete(%d) to return true", k)
		}
		if got := inOrder(tree); !reflect.DeepEqual(got, sorted[i+1:]) {
			t.Errorf("after deleting %d: expected %v got %v", k, sorted[i+1:], got)
		}
		checkInvariants(t, tree)
	}
	if !tree.IsEmpty() {
		t.Errorf("expected empty tree after deleting all nodes")
	}
}

// TestTreeDeleteAtHeadTailDrain drains a tree from both ends, checking
// that the extremes move inward in sorted order.
func TestTreeDeleteAtHeadTailDrain(t *testing.T) {
	keys := []int{5, 2, 9, 0, 3, 7, 8, 1}
	sortedKeys := append([]int(nil), keys...)
	sort.Ints(sortedKeys)

	// Drain from the head: min must increase each time.
	tree := NewSplayTree[int]()
	for _, k := range keys {
		tree.Insert(k)
	}
	for i, k := range sortedKeys {
		if x, found := tree.FindMin(); !found || x != k {
			t.Fatalf("Drain head: expected min %d, got %d", k, x)
		}
		if !tree.DeleteAtHead() {
			t.Fatalf("Drain head: DeleteAtHead returned false at step %d", i)
		}
		checkInvariants(t, tree)
	}
	if tree.DeleteAtHead() {
		t.Errorf("Expected DeleteAtHead on drained tree to return false.")
	}

	// Refill and drain from the tail.
	for _, k := range keys {
		tree.Insert(k)
	}
	for i, k := range slices.Backward(sortedKeys) {
		if x, found := tree.FindMax(); !found || x != k {
			t.Fatalf("Drain tail: expected max %d, got %d", k, x)
		}
		if !tree.DeleteAtTail() {
			t.Fatalf("Drain tail: DeleteAtTail returned false at step %d", i)
		}
		checkInvariants(t, tree)
	}
	if tree.DeleteAtTail() {
		t.Errorf("Expected DeleteAtTail on drained tree to return false.")
	}
	if !tree.IsEmpty() {
		t.Errorf("Expected empty tree after draining.")
	}
}

// -------------------------------------------------------------------------------------------------------
// Randomized property test against a reference model (fixed seed)
// -------------------------------------------------------------------------------------------------------

func TestTreeRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewSource(42)) // fixed seed: deterministic run

	tree := NewSplayTree[int]()
	model := make(map[int]bool) // set of keys the tree should contain

	sortedModel := func() []int {
		keys := []int{}
		for k := range model {
			keys = append(keys, k)
		}
		sort.Ints(keys)
		return keys
	}

	verify := func(step int) {
		keys := sortedModel()
		if tree.Length() != len(model) {
			t.Fatalf("step %d: Length()=%d, model has %d keys", step, tree.Length(), len(model))
		}
		if got := inOrder(tree); !reflect.DeepEqual(got, keys) {
			t.Fatalf("step %d: in-order mismatch, expected %v got %v", step, keys, got)
		}
		// Backward must be the exact reverse of All.
		var bk []int
		for _, v := range tree.Backward() {
			bk = append(bk, v)
		}
		for i := range bk {
			if bk[i] != keys[len(keys)-1-i] {
				t.Fatalf("step %d: Backward mismatch at %d: %v vs %v", step, i, bk, keys)
			}
		}
		// FindMin/FindMax must match the model extremes.
		if len(keys) == 0 {
			if _, found := tree.FindMin(); found {
				t.Fatalf("step %d: expected no min on empty tree", step)
			}
			if _, found := tree.FindMax(); found {
				t.Fatalf("step %d: expected no max on empty tree", step)
			}
		} else {
			if x, found := tree.FindMin(); !found || x != keys[0] {
				t.Fatalf("step %d: FindMin=%d, expected %d", step, x, keys[0])
			}
			if x, found := tree.FindMax(); !found || x != keys[len(keys)-1] {
				t.Fatalf("step %d: FindMax=%d, expected %d", step, x, keys[len(keys)-1])
			}
		}
		checkInvariants(t, tree)
	}

	const maxKey = 60 // small key space so duplicates and deletes are common
	for step := range 800 {
		k := rng.Intn(maxKey)
		switch rng.Intn(10) {
		case 0, 1, 2, 3: // Insert (duplicates replace in both tree and model)
			added := tree.Insert(k)
			if added == model[k] {
				t.Fatalf("step %d: Insert(%d)=%v, model said present=%v", step, k, added, model[k])
			}
			model[k] = true
		case 4, 5, 6: // Delete
			got := tree.Delete(k)
			if got != model[k] {
				t.Fatalf("step %d: Delete(%d)=%v, model said present=%v", step, k, got, model[k])
			}
			delete(model, k)
		case 7: // Search
			_, found := tree.Search(k)
			if found != model[k] {
				t.Fatalf("step %d: Search(%d) found=%v, model said present=%v", step, k, found, model[k])
			}
		case 8: // DeleteAtHead
			got := tree.DeleteAtHead()
			if len(model) == 0 {
				if got {
					t.Fatalf("step %d: DeleteAtHead on empty tree returned true", step)
				}
			} else {
				if !got {
					t.Fatalf("step %d: DeleteAtHead returned false on non-empty tree", step)
				}
				delete(model, sortedModel()[0])
			}
		case 9: // DeleteAtTail
			got := tree.DeleteAtTail()
			if len(model) == 0 {
				if got {
					t.Fatalf("step %d: DeleteAtTail on empty tree returned true", step)
				}
			} else {
				if !got {
					t.Fatalf("step %d: DeleteAtTail returned false on non-empty tree", step)
				}
				keys := sortedModel()
				delete(model, keys[len(keys)-1])
			}
		}
		checkInvariants(t, tree) // after every structural change
		if step%37 == 0 {
			verify(step)
		}
	}
	verify(800)

	// Depth sanity: the reported depth is at least 1 and at most the size.
	if n := tree.Length(); n > 0 {
		d := tree.Depth()
		if d < 1 || d > n {
			t.Errorf("Depth %d out of range for %d nodes", d, n)
		}
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

func TestTreeMarshalJSON(t *testing.T) {
	// Exact array output, in-order (ascending) regardless of insert order.
	tree := NewSplayTree[int]()
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

	// Struct elements use their normal JSON encoding.
	items := newTestTree()
	for _, s := range []string{"b", "a"} {
		items.Insert(TestTreeNode{S: s})
	}
	if b, err := json.Marshal(items); err != nil || string(b) != `[{"S":"a","N":0},{"S":"b","N":0}]` {
		t.Errorf(`Expected [{"S":"a","N":0},{"S":"b","N":0}], got (%s, %v)`, b, err)
	}

	// An empty tree encodes as [].
	if b, err := json.Marshal(NewSplayTree[int]()); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] for an empty tree, got (%s, %v)", b, err)
	}

	// A zero-value tree is a tolerated read: [].
	var zero SplayTree[int]
	if b, err := zero.MarshalJSON(); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] for a zero-value tree, got (%s, %v)", b, err)
	}

	// A direct call on a nil tree encodes as []; json.Marshal on a nil
	// *SplayTree never reaches the method — the json package writes null
	// for nil pointers itself.
	var nilTree *SplayTree[int]
	if b, err := nilTree.MarshalJSON(); err != nil || string(b) != "[]" {
		t.Errorf("Expected [] from a direct nil-tree call, got (%s, %v)", b, err)
	}
	if b, err := json.Marshal(nilTree); err != nil || string(b) != "null" {
		t.Errorf("Expected null from json.Marshal on a nil tree, got (%s, %v)", b, err)
	}

	// Element-level marshalers are honored.
	custom := NewSplayTree[upperString]()
	custom.Insert("x")
	custom.Insert("y")
	if b, err := json.Marshal(custom); err != nil || string(b) != `["X","Y"]` {
		t.Errorf(`Expected ["X","Y"], got (%s, %v)`, b, err)
	}

	// Encoding errors pass through unchanged.
	bad := NewSplayTreeFunc[chan int](func(a, b chan int) int { return 0 })
	bad.Insert(make(chan int))
	if _, err := json.Marshal(bad); err == nil {
		t.Errorf("Expected an error marshaling a tree of channels.")
	}
}

func TestTreeUnmarshalJSON(t *testing.T) {
	// The decoded elements land in the tree; in-order is ascending.
	tree := NewSplayTree[int]()
	if err := json.Unmarshal([]byte("[3,1,2]"), tree); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if got := inOrder(tree); !reflect.DeepEqual(got, []int{1, 2, 3}) {
		t.Errorf("Expected [1 2 3], got %v", got)
	}
	if min, found := tree.FindMin(); !found || min != 1 {
		t.Errorf("Expected min 1, got (%v, %v)", min, found)
	}

	// A round trip rebuilds a structurally sound tree and keeps the
	// comparison function (Search works on the rebuilt tree).
	items := newTestTree()
	for _, s := range []string{"c", "a", "b"} {
		items.Insert(TestTreeNode{S: s})
	}
	b, err := json.Marshal(items)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	again := newTestTree()
	if err := json.Unmarshal(b, again); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	var keys []string
	for _, v := range again.All() {
		keys = append(keys, v.S)
	}
	if got, want := fmt.Sprint(keys), "[a b c]"; got != want {
		t.Errorf("Expected %s after round trip, got %s", want, got)
	}
	if _, found := again.Search(TestTreeNode{S: "b"}); !found {
		t.Errorf("Expected Search to work after unmarshal.")
	}

	// Unmarshaling replaces the contents; it does not add.
	if err := json.Unmarshal([]byte("[7]"), tree); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if got, want := tree.Len(), 1; got != want {
		t.Errorf("Expected replacement, got length %d, want %d", got, want)
	}
	checkInvariants(t, tree)

	// An empty array and null clear the tree.
	full := newTestTree()
	full.Insert(TestTreeNode{S: "z"})
	if err := json.Unmarshal([]byte("[]"), full); err != nil {
		t.Fatalf("json.Unmarshal([]): %v", err)
	}
	if !full.IsEmpty() {
		t.Errorf("Expected [] to clear the tree.")
	}
	full.Insert(TestTreeNode{S: "z"})
	if err := json.Unmarshal([]byte("null"), full); err != nil {
		t.Fatalf("json.Unmarshal(null): %v", err)
	}
	if !full.IsEmpty() {
		t.Errorf("Expected null to clear the tree.")
	}
	if full.Length() != 0 || full.root != nil {
		t.Errorf("Expected null to leave an empty tree, got length %d", full.Length())
	}

	// Element-level unmarshalers are honored.
	custom := NewSplayTree[upperString]()
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
	keep := newTestTree()
	keep.Insert(TestTreeNode{S: "keep"})
	for _, badData := range []string{"[1,", `["x"]`, `{"S":"a"}`, "7", `["a",3]`} {
		if err := json.Unmarshal([]byte(badData), keep); err == nil {
			t.Errorf("Expected an error unmarshaling %s.", badData)
		}
		var vals []string
		for _, v := range keep.All() {
			vals = append(vals, v.S)
		}
		if got, want := fmt.Sprint(vals), "[keep]"; got != want {
			t.Errorf("Tree changed after the error on %s: %s", badData, got)
		}
	}
	if keep.Length() != 1 {
		t.Errorf("Expected length 1 after decode errors, got %d", keep.Length())
	}
}

// TestTreeUnmarshalJSONPanics verifies that UnmarshalJSON joins the insert
// family: storing elements into a nil or zero-value tree panics with a
// message naming the method and the fix, while [] and null — which store
// nothing — are tolerated everywhere.
func TestTreeUnmarshalJSONPanics(t *testing.T) {
	var zero SplayTree[TestTreeNode]
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
			if msg, ok := r.(string); !ok || !strings.Contains(msg, "UnmarshalJSON") || !strings.Contains(msg, "NewSplayTree") {
				t.Errorf("Unexpected panic message: %v", r)
			}
		}()
		_ = zero.UnmarshalJSON([]byte(`[{"S":"a"}]`))
	}()

	var nilTree *SplayTree[TestTreeNode]
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

// TestTreeJSONStructField marshals and unmarshals a SplayTree nested in a
// struct through the encoding/json package.  The tree must be created with
// NewSplayTree/NewSplayTreeFunc before unmarshaling: for a nil *SplayTree
// field the json package allocates a zero-value tree itself (no comparison
// function), so non-empty data panics with the insert-family message.
func TestTreeJSONStructField(t *testing.T) {
	type Doc struct {
		Title string             `json:"title"`
		Tags  *SplayTree[string] `json:"tags"`
	}

	d := Doc{Title: "pluto", Tags: NewSplayTree[string]()}
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
	out.Tags = NewSplayTree[string]()
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
	clearDoc := Doc{Title: "x", Tags: NewSplayTree[string]()}
	clearDoc.Tags.Insert("gone")
	if err := json.Unmarshal([]byte(`{"title":"x","tags":null}`), &clearDoc); err != nil {
		t.Fatalf("json.Unmarshal with null tags: %v", err)
	}
	if !clearDoc.Tags.IsEmpty() {
		t.Errorf("Expected null tags to clear the tree.")
	}

	// Non-empty data into a nil *SplayTree field: the json package
	// allocates a zero-value tree, and the insert contract panics through
	// json.Unmarshal (it does not recover panics).
	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Errorf("Expected unmarshaling into an uncreated tree field to panic.")
				return
			}
			if msg, ok := r.(string); !ok || !strings.Contains(msg, "NewSplayTree") {
				t.Errorf("Unexpected panic message: %v", r)
			}
		}()
		var bad Doc
		_ = json.Unmarshal([]byte(`{"title":"x","tags":["a"]}`), &bad)
	}()
}

// TestTreeJSONRandomizedModel cross-checks marshaling and unmarshaling
// against a sorted-slice reference model at fixed seed.
func TestTreeJSONRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewSource(42)) // fixed seed: deterministic run
	const ops = 500

	tree := NewSplayTree[int]()
	model := make(map[int]bool) // set of keys the tree should contain

	sortedModel := func() []int {
		keys := []int{}
		for k := range model {
			keys = append(keys, k)
		}
		sort.Ints(keys)
		return keys
	}

	for step := range ops {
		k := rng.Intn(100)
		switch rng.Intn(3) {
		case 0, 1: // Insert (duplicates replace in both tree and model)
			tree.Insert(k)
			model[k] = true
		case 2: // Delete
			tree.Delete(k)
			delete(model, k)
		}

		// Marshal must equal the sorted model marshaled as a plain slice.
		got, err := json.Marshal(tree)
		if err != nil {
			t.Fatalf("step %d: %v", step, err)
		}
		want, _ := json.Marshal(sortedModel())
		if string(got) != string(want) {
			t.Fatalf("step %d: marshaled %s, model %s", step, got, want)
		}

		// Unmarshaling into a fresh tree must reproduce the model.
		fresh := NewSplayTree[int]()
		if err := json.Unmarshal(got, fresh); err != nil {
			t.Fatalf("step %d: %v", step, err)
		}
		if vals := inOrder(fresh); !reflect.DeepEqual(vals, sortedModel()) {
			t.Fatalf("step %d: round trip got %v, model %v", step, vals, sortedModel())
		}
	}
	checkInvariants(t, tree)
}
