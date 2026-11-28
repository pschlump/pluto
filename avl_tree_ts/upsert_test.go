/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package avl_tree_ts

// Twin tests for Upsert/Remove and the iterative insert/delete/walks:
// semantics, panic contract, a fixed-seed model test with full AVL
// validation after every operation, op-by-op parity with the plain
// avl_tree package, walks against recursive references, allocation counts
// and a concurrent Upsert/Remove race test (run with -race).

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"sync"
	"testing"

	"github.com/pschlump/pluto/avl_tree"
)

func mkProbe(s string) TestTreeNode { return TestTreeNode{S: s} }

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
}

func TestUpsertPanics(t *testing.T) {
	var nilTree *AvlTree[TestTreeNode]
	expectPanicMessage(t, "Upsert on nil", "Upsert called on a nil tree", func() { nilTree.Upsert(mkKey(1)) })
	expectPanicMessage(t, "Upsert on nil names fix", "NewAvlTree", func() { nilTree.Upsert(mkKey(1)) })
	var zero AvlTree[TestTreeNode]
	expectPanicMessage(t, "Upsert on zero value", "Upsert called on a tree with no comparison function (create the tree with NewAvlTree or NewAvlTreeFunc)", func() { zero.Upsert(mkKey(1)) })
	// The write lock must have been released by the panic (deferred
	// unlock): the zero-value tree is still usable for reads and writes.
	if zero.Len() != 0 {
		t.Fatalf("zero-value tree Len = %d after panic", zero.Len())
	}
	zero.Truncate()
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
	v, ok := tree.Remove(mkKey(4))
	if !ok || v.N != 104 || v.S != mkKey(4).S {
		t.Fatalf("Remove = (%v, %v), want ({%s 104}, true)", v, ok, mkKey(4).S)
	}
	validateAVLNode(t, tree.root)
	if _, ok := tree.Remove(mkKey(4)); ok {
		t.Fatalf("second Remove of the same key reported found")
	}
}

// TestAvlRandomizedModelUpsert interleaves Insert, Upsert, Delete and
// Remove on the twin and on the plain package with the same seed (42),
// checking every result against a map model and against the plain
// package (parity), and validating balance, cached heights and order
// after every operation.
func TestAvlRandomizedModelUpsert(t *testing.T) {
	r := rand.New(rand.NewPCG(42, 1))
	tree := newTestTree()
	plain := avl_tree.NewAvlTreeFunc(func(a, b TestTreeNode) int { return cmpTestTreeNode(a, b) })
	model := map[string]int{}
	for step := range 5000 {
		k := mkKey(r.IntN(300)).S
		n := r.IntN(1_000_000)
		item := TestTreeNode{S: k, N: n}
		prev, had := model[k]
		switch op := r.IntN(10); {
		case op < 3:
			got, want := tree.Insert(item), plain.Insert(item)
			if got != want || got == had {
				t.Fatalf("step %d: Insert(%s) = %v, plain %v, model had=%v", step, k, got, want, had)
			}
			model[k] = n
		case op < 6:
			old, replaced := tree.Upsert(item)
			pOld, pReplaced := plain.Upsert(item)
			if old != pOld || replaced != pReplaced || replaced != had || (had && old.N != prev) {
				t.Fatalf("step %d: Upsert(%s) = (%v, %v), plain (%v, %v), model (%d, %v)", step, k, old, replaced, pOld, pReplaced, prev, had)
			}
			model[k] = n
		case op < 7:
			got, want := tree.Delete(mkProbe(k)), plain.Delete(mkProbe(k))
			if got != want || got != had {
				t.Fatalf("step %d: Delete(%s) = %v, plain %v, model %v", step, k, got, want, had)
			}
			delete(model, k)
		default:
			v, ok := tree.Remove(mkProbe(k))
			pv, pok := plain.Remove(mkProbe(k))
			if v != pv || ok != pok || ok != had || (had && v.N != prev) {
				t.Fatalf("step %d: Remove(%s) = (%v, %v), plain (%v, %v), model (%d, %v)", step, k, v, ok, pv, pok, prev, had)
			}
			delete(model, k)
		}
		validateAVLNode(t, tree.root)
		if tree.Len() != len(model) || tree.Depth() != plain.Depth() {
			t.Fatalf("step %d: Len %d / Depth %d, model %d / plain depth %d", step, tree.Len(), tree.Depth(), len(model), plain.Depth())
		}
		if step%50 == 0 || step == 4999 {
			if !slices.Equal(tree.Slice(), plain.Slice()) {
				t.Fatalf("step %d: twin and plain contents differ", step)
			}
		}
	}
}

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

// TestWalksIterative checks the three Walk* functions and Index against
// recursive references for sizes 0..48, with early stop at every position.
func TestWalksIterative(t *testing.T) {
	r := rand.New(rand.NewPCG(7, 7))
	for size := 0; size <= 48; size++ {
		tree := newTestTree()
		for _, i := range r.Perm(size) {
			tree.Insert(mkKey(i))
		}
		walks := []struct {
			order string
			walk  func(ApplyFunction[TestTreeNode])
		}{{"in", tree.WalkInOrder}, {"pre", tree.WalkPreOrder}, {"post", tree.WalkPostOrder}}
		for _, w := range walks {
			var ref []walkRec
			refWalk(tree.root, 0, w.order, &ref)
			for stop := 0; stop <= size; stop++ {
				var got []walkRec
				w.walk(func(pos, depth int, d TestTreeNode) bool {
					got = append(got, walkRec{pos, depth, d.S})
					return stop == 0 || len(got) < stop
				})
				want := ref
				if stop > 0 {
					want = ref[:stop]
				}
				if !slices.Equal(got, want) {
					t.Fatalf("size %d Walk %s stop %d = %v, want %v", size, w.order, stop, got, want)
				}
			}
		}
		for pos := -1; pos <= size; pos++ {
			v, ok := tree.Index(pos)
			if (pos >= 0 && pos < size) != ok || (ok && v.S != mkKey(pos).S) {
				t.Fatalf("size %d Index(%d) = (%v, %v)", size, pos, v, ok)
			}
		}
	}
}

// TestAllocations pins the allocation-free write paths of the twin (its
// iterators snapshot by contract, so they allocate the snapshot).
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
	sum := 0
	check("WalkInOrder", 0, func() { tree.WalkInOrder(func(_, _ int, v int) bool { sum += v; return true }) })
	check("Index", 0, func() { tree.Index(perfSize / 2) })
	_ = sum
}

// TestUpsertRemoveConcurrent hammers Upsert/Remove/Insert/Delete from
// several goroutines on overlapping keys while readers iterate snapshots
// and walk; the final tree must validate and every Upsert/Remove result
// must be self-consistent.  Run with -race.
func TestUpsertRemoveConcurrent(t *testing.T) {
	tree := newTestTree()
	const workers = 8
	const ops = 2000
	stop := make(chan struct{})
	var readers sync.WaitGroup
	readers.Add(1)
	go func() {
		defer readers.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			prev := ""
			for v := range tree.All() {
				if v.S <= prev {
					t.Errorf("snapshot out of order: %s after %s", v.S, prev)
					return
				}
				prev = v.S
			}
			tree.WalkInOrder(func(_, _ int, _ TestTreeNode) bool { return true })
			_ = tree.Depth()
		}
	}()
	var writers sync.WaitGroup
	for w := range workers {
		writers.Add(1)
		go func(w int) {
			defer writers.Done()
			r := rand.New(rand.NewPCG(uint64(w), 99))
			for range ops {
				k := fmt.Sprintf("%04d", r.IntN(64))
				switch r.IntN(4) {
				case 0:
					if old, replaced := tree.Upsert(TestTreeNode{S: k, N: w}); replaced && old.S != k {
						t.Errorf("Upsert(%s) returned old %v", k, old)
					}
				case 1:
					if v, ok := tree.Remove(mkProbe(k)); ok && v.S != k {
						t.Errorf("Remove(%s) returned %v", k, v)
					}
				case 2:
					tree.Insert(TestTreeNode{S: k, N: w})
				default:
					tree.Delete(mkProbe(k))
				}
			}
		}(w)
	}
	writers.Wait()
	close(stop)
	readers.Wait()
	validateAVLNode(t, tree.root)
	if n := len(tree.Slice()); n != tree.Len() {
		t.Fatalf("Len %d but %d elements", tree.Len(), n)
	}
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

// BenchmarkPerfRemove deletes-and-returns then re-inserts a present key.
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
