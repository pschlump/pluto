/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package avl_tree

// Tests for the allocation-free hot paths: Upsert and Remove (one-descent
// read-and-write), the iterative insert/delete (validated against a
// sorted model after every structural change), the explicit-stack
// iterators and walks (order and early stop at every position, against a
// recursive reference), and allocation counts.

import (
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
)

func TestUpsertBasic(t *testing.T) {
	tree := newTestTree()
	old, replaced := tree.Upsert(TestTreeNode{S: "b", N: 1})
	if replaced || old != (TestTreeNode{}) {
		t.Fatalf("Upsert into empty tree = (%v, %v), want (zero, false)", old, replaced)
	}
	old, replaced = tree.Upsert(TestTreeNode{S: "b", N: 2})
	if !replaced || old != (TestTreeNode{S: "b", N: 1}) {
		t.Fatalf("Upsert replace = (%v, %v), want ({b 1}, true)", old, replaced)
	}
	if got, _ := tree.Search(mkProbe("b")); got.N != 2 {
		t.Fatalf("after Upsert replace stored N = %d, want 2", got.N)
	}
	if tree.Len() != 1 {
		t.Fatalf("Len = %d, want 1", tree.Len())
	}
	for i, s := range []string{"a", "c", "d", "e"} {
		if _, replaced := tree.Upsert(TestTreeNode{S: s, N: 10 + i}); replaced {
			t.Fatalf("Upsert(%s) reported replaced on a new key", s)
		}
		validateAVLNode(t, tree.root)
	}
	if tree.Len() != 5 {
		t.Fatalf("Len = %d, want 5", tree.Len())
	}
}

func TestUpsertPanics(t *testing.T) {
	var nilTree *AvlTree[TestTreeNode]
	expectPanicMessage(t, "Upsert on nil", "Upsert called on a nil tree", func() { nilTree.Upsert(mkKey(1)) })
	expectPanicMessage(t, "Upsert on nil names fix", "NewAvlTree", func() { nilTree.Upsert(mkKey(1)) })
	var zero AvlTree[TestTreeNode]
	expectPanicMessage(t, "Upsert on zero value", "Upsert called on a tree with no comparison function (create the tree with NewAvlTree or NewAvlTreeFunc)", func() { zero.Upsert(mkKey(1)) })
}

func TestRemoveBasic(t *testing.T) {
	var nilTree *AvlTree[TestTreeNode]
	if v, ok := nilTree.Remove(mkKey(1)); ok || v != (TestTreeNode{}) {
		t.Fatalf("nil Remove = (%v, %v), want (zero, false)", v, ok)
	}
	var zero AvlTree[TestTreeNode]
	if _, ok := zero.Remove(mkKey(1)); ok {
		t.Fatalf("zero-value Remove found something")
	}
	tree := newTestTree()
	for i := range 10 {
		tree.Insert(TestTreeNode{S: mkKey(i).S, N: 100 + i})
	}
	v, ok := tree.Remove(mkKey(4)) // probe carries only the key
	if !ok || v.N != 104 || v.S != mkKey(4).S {
		t.Fatalf("Remove = (%v, %v), want ({%s 104}, true)", v, ok, mkKey(4).S)
	}
	validateAVLNode(t, tree.root)
	if _, ok := tree.Remove(mkKey(4)); ok {
		t.Fatalf("second Remove of the same key reported found")
	}
	if tree.Len() != 9 {
		t.Fatalf("Len = %d, want 9", tree.Len())
	}
}

// mkProbe makes a key-only probe.
func mkProbe(s string) TestTreeNode { return TestTreeNode{S: s} }

// modelCheck compares the tree with the reference model (key -> N) and
// validates every AVL invariant.
func modelCheck(t *testing.T, step int, tree *AvlTree[TestTreeNode], model map[string]int) {
	t.Helper()
	validateAVLNode(t, tree.root)
	if tree.Len() != len(model) {
		t.Fatalf("step %d: Len = %d, model %d", step, tree.Len(), len(model))
	}
	keys := make([]string, 0, len(model))
	for k := range model {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	i := 0
	for v := range tree.All() {
		if i >= len(keys) || v.S != keys[i] || v.N != model[keys[i]] {
			t.Fatalf("step %d: All()[%d] = %v, want {%s %d}", step, i, v, keys[i], model[keys[i]])
		}
		i++
	}
	if i != len(keys) {
		t.Fatalf("step %d: All yielded %d elements, want %d", step, i, len(keys))
	}
}

// TestAvlRandomizedModelUpsert interleaves Insert, Upsert, Delete, Remove,
// DeleteAtHead and DeleteAtTail against a map model (fixed seed),
// validating balance, cached heights and order after every operation.
func TestAvlRandomizedModelUpsert(t *testing.T) {
	r := rand.New(rand.NewPCG(42, 1))
	tree := newTestTree()
	model := map[string]int{}
	const keySpace = 300
	for step := range 6000 {
		k := mkKey(r.IntN(keySpace)).S
		n := r.IntN(1_000_000)
		switch op := r.IntN(10); {
		case op < 3:
			_, had := model[k]
			if added := tree.Insert(TestTreeNode{S: k, N: n}); added == had {
				t.Fatalf("step %d: Insert(%s) = %v, model had=%v", step, k, added, had)
			}
			model[k] = n
		case op < 6:
			prev, had := model[k]
			old, replaced := tree.Upsert(TestTreeNode{S: k, N: n})
			if replaced != had || (had && (old.S != k || old.N != prev)) || (!had && old != (TestTreeNode{})) {
				t.Fatalf("step %d: Upsert(%s) = (%v, %v), model (%d, %v)", step, k, old, replaced, prev, had)
			}
			model[k] = n
		case op < 7:
			_, had := model[k]
			if got := tree.Delete(mkProbe(k)); got != had {
				t.Fatalf("step %d: Delete(%s) = %v, model %v", step, k, got, had)
			}
			delete(model, k)
		case op < 9:
			prev, had := model[k]
			v, ok := tree.Remove(mkProbe(k))
			if ok != had || (had && (v.S != k || v.N != prev)) {
				t.Fatalf("step %d: Remove(%s) = (%v, %v), model (%d, %v)", step, k, v, ok, prev, had)
			}
			delete(model, k)
		default:
			if r.IntN(2) == 0 {
				if mn, ok := tree.FindMin(); ok {
					delete(model, mn.S)
				}
				tree.DeleteAtHead()
			} else {
				if mx, ok := tree.FindMax(); ok {
					delete(model, mx.S)
				}
				tree.DeleteAtTail()
			}
		}
		modelCheck(t, step, tree, model)
	}
}

// TestAvlSequentialShapes drives the retrace through long ascending and
// descending runs (every insert rotates) and deletes from both ends and
// the middle (two-child successor splices at every depth).
func TestAvlSequentialShapes(t *testing.T) {
	tree := newTestTree()
	model := map[string]int{}
	for i := range 512 {
		tree.Insert(TestTreeNode{S: mkKey(i).S, N: i})
		model[mkKey(i).S] = i
		validateAVLNode(t, tree.root)
	}
	for i := 1023; i >= 512; i-- {
		tree.Upsert(TestTreeNode{S: mkKey(i).S, N: i})
		model[mkKey(i).S] = i
		validateAVLNode(t, tree.root)
	}
	modelCheck(t, 0, tree, model)
	// Delete the current root repeatedly: always the two-children case
	// until the tree is tiny.
	for step := 0; tree.root != nil; step++ {
		k := tree.root.data.S
		if v, ok := tree.Remove(mkProbe(k)); !ok || v.S != k {
			t.Fatalf("Remove(root %s) = (%v, %v)", k, v, ok)
		}
		delete(model, k)
		modelCheck(t, step, tree, model)
	}
}

// refInOrder etc. are recursive reference walks used as oracles.
type walkRec struct {
	pos, depth int
	s          string
}

func refWalk(n *AvlTreeElement[TestTreeNode], depth int, order string, out *[]walkRec) {
	if n == nil {
		return
	}
	if order == "pre" {
		*out = append(*out, walkRec{len(*out), depth, n.data.S})
	}
	refWalk(n.left, depth+1, order, out)
	if order == "in" {
		*out = append(*out, walkRec{len(*out), depth, n.data.S})
	}
	refWalk(n.right, depth+1, order, out)
	if order == "post" {
		*out = append(*out, walkRec{len(*out), depth, n.data.S})
	}
}

// TestIterationOrderAndEarlyStop checks All, Backward and the three Walk*
// functions against recursive references for every size 0..64 (random
// insertion order, so shapes vary), and that stopping after k elements
// yields exactly the first k for every k.
func TestIterationOrderAndEarlyStop(t *testing.T) {
	r := rand.New(rand.NewPCG(7, 7))
	for size := 0; size <= 64; size++ {
		tree := newTestTree()
		for _, i := range r.Perm(size) {
			tree.Insert(mkKey(i))
		}
		var want []string
		for i := range size {
			want = append(want, mkKey(i).S)
		}
		rev := slices.Clone(want)
		slices.Reverse(rev)

		seqs := []struct {
			name string
			want []string
			seq  func(func(TestTreeNode) bool)
		}{{"All", want, tree.All()}, {"Backward", rev, tree.Backward()}}
		for _, sq := range seqs {
			var got []string
			for v := range sq.seq {
				got = append(got, v.S)
			}
			if !slices.Equal(got, sq.want) {
				t.Fatalf("size %d %s = %v, want %v", size, sq.name, got, sq.want)
			}
			for stop := 1; stop <= size; stop++ {
				got = got[:0]
				calls := 0
				sq.seq(func(v TestTreeNode) bool {
					calls++
					got = append(got, v.S)
					return len(got) < stop
				})
				if calls != stop || !slices.Equal(got, sq.want[:stop]) {
					t.Fatalf("size %d %s stop %d: %d calls, got %v", size, sq.name, stop, calls, got)
				}
			}
		}

		walks := []struct {
			order string
			walk  func(ApplyFunction[TestTreeNode])
		}{{"in", tree.WalkInOrder}, {"pre", tree.WalkPreOrder}, {"post", tree.WalkPostOrder}}
		for _, w := range walks {
			var ref []walkRec
			refWalk(tree.root, 0, w.order, &ref)
			var got []walkRec
			w.walk(func(pos, depth int, d TestTreeNode) bool {
				got = append(got, walkRec{pos, depth, d.S})
				return true
			})
			if !slices.Equal(got, ref) {
				t.Fatalf("size %d Walk %s = %v, want %v", size, w.order, got, ref)
			}
			for stop := 1; stop <= size; stop++ {
				got = got[:0]
				w.walk(func(pos, depth int, d TestTreeNode) bool {
					got = append(got, walkRec{pos, depth, d.S})
					return len(got) < stop
				})
				if !slices.Equal(got, ref[:stop]) {
					t.Fatalf("size %d Walk %s stop %d = %v, want %v", size, w.order, stop, got, ref[:stop])
				}
			}
		}

		for pos := -1; pos <= size; pos++ {
			v, ok := tree.Index(pos)
			if (pos >= 0 && pos < size) != ok || (ok && v.S != want[pos]) {
				t.Fatalf("size %d Index(%d) = (%v, %v)", size, pos, v, ok)
			}
		}
	}
}

// TestReverseIterative checks Reverse mirrors every node (twice restores).
func TestReverseIterative(t *testing.T) {
	tree := newTestTree()
	for _, i := range rand.New(rand.NewPCG(3, 3)).Perm(200) {
		tree.Insert(mkKey(i))
	}
	var want []string
	tree.WalkPreOrder(func(_, _ int, d TestTreeNode) bool { want = append(want, d.S); return true })
	tree.Reverse()
	got := slices.Collect(func(yield func(string) bool) {
		for v := range tree.All() {
			if !yield(v.S) {
				return
			}
		}
	})
	if !slices.IsSortedFunc(got, func(a, b string) int { return -strings.Compare(a, b) }) || len(got) != 200 {
		t.Fatalf("reversed tree in-order is not descending")
	}
	tree.Reverse()
	validateAVLNode(t, tree.root)
	var back []string
	tree.WalkPreOrder(func(_, _ int, d TestTreeNode) bool { back = append(back, d.S); return true })
	if !slices.Equal(back, want) {
		t.Fatalf("Reverse twice changed the shape")
	}
}

// TestAllocations pins the allocation-free paths.
func TestAllocations(t *testing.T) {
	tree, keys := perfTree()
	i := 0
	check := func(name string, want float64, fx func()) {
		t.Helper()
		if got := testing.AllocsPerRun(200, fx); got != want {
			t.Errorf("%s: %v allocs/op, want %v", name, got, want)
		}
	}
	check("Insert replace", 0, func() { tree.Insert(keys[i%perfSize]); i++ })
	check("Upsert replace", 0, func() { tree.Upsert(keys[i%perfSize]); i++ })
	check("Insert new", 1, func() { tree.Insert(-1 - i); tree.Delete(-1 - i); i++ })
	check("Remove miss", 0, func() { tree.Remove(1) })
	check("Search", 0, func() { tree.Search(keys[i%perfSize]); i++ })
	sum := 0
	check("All full", 0, func() {
		for v := range tree.All() {
			sum += v
		}
	})
	check("Backward full", 0, func() {
		for v := range tree.Backward() {
			sum += v
		}
	})
	check("All early stop", 0, func() {
		for v := range tree.All() {
			sum += v
			break
		}
	})
	check("WalkInOrder", 0, func() { tree.WalkInOrder(func(_, _ int, v int) bool { sum += v; return true }) })
	check("WalkPreOrder", 0, func() { tree.WalkPreOrder(func(_, _ int, v int) bool { sum += v; return true }) })
	check("WalkPostOrder", 0, func() { tree.WalkPostOrder(func(_, _ int, v int) bool { sum += v; return true }) })
	check("Index", 0, func() { tree.Index(perfSize / 2) })
	check("Reverse", 0, func() { tree.Reverse() })
	_ = sum

	// A struct element with a string field (the TestTreeNode shape).
	st := newTestTree()
	for k := range 100 {
		st.Insert(mkKey(k))
	}
	probe := TestTreeNode{S: mkKey(50).S, N: 9}
	check("TestTreeNode Insert replace", 0, func() { st.Insert(probe) })
	check("TestTreeNode Upsert replace", 0, func() { st.Upsert(probe) })
}

// BenchmarkPerfUpsert is the one-descent replacement for
// BenchmarkPerfSearchInsert.
func BenchmarkPerfUpsert(b *testing.B) {
	tree, keys := perfTree()
	b.ReportAllocs()
	i := 0
	sink := 0
	for b.Loop() {
		if old, ok := tree.Upsert(keys[i%perfSize]); ok {
			sink += old
		}
		i++
	}
	_ = sink
}

// BenchmarkPerfSearchDelete is the two-descent pattern Remove replaces;
// each op re-inserts so the tree stays full.
func BenchmarkPerfSearchDelete(b *testing.B) {
	tree, keys := perfTree()
	b.ReportAllocs()
	i := 0
	sink := 0
	for b.Loop() {
		k := keys[i%perfSize]
		if v, ok := tree.Search(k); ok {
			sink += v
			tree.Delete(k)
		}
		tree.Insert(k)
		i++
	}
	_ = sink
}

// BenchmarkPerfRemove is the one-descent replacement for
// BenchmarkPerfSearchDelete (same re-insert per op).
func BenchmarkPerfRemove(b *testing.B) {
	tree, keys := perfTree()
	b.ReportAllocs()
	i := 0
	sink := 0
	for b.Loop() {
		k := keys[i%perfSize]
		if v, ok := tree.Remove(k); ok {
			sink += v
		}
		tree.Insert(k)
		i++
	}
	_ = sink
}
