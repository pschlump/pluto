// Copyright (C) Philip Schlump, 2012-2026.
// BSD 3 Clause Licensed.
//
// The structural invariant checker and the randomized model test.  Call
// checkInvariants after any structural change to the package.
package fib_heap

import (
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/pschlump/pluto/index_pq"
)

// fib returns the Fibonacci number F(i) with F(1) = F(2) = 1.
func fib(i int) int {
	a, b := 1, 1
	for range i - 1 {
		a, b = b, a+b
	}
	return a
}

// checkInvariants verifies every structural property the amortized
// bounds rest on:
//
//   - nodes[k] != nil exactly for present indices, node.index == k, and
//     the count agrees with length;
//   - the root list is a circular doubly-linked list of roots whose
//     left/right pointers are bidirectional, parents nil, marks clear;
//   - every child ring is circular with child.parent pointing back;
//   - heap order: every child orders at or after its parent;
//   - degree == number of children;
//   - the Fibonacci size lemma: a node of degree d roots a subtree of at
//     least F(d+2) nodes — this is what bounds degrees by log_phi n and
//     makes Pop amortized O(log n);
//   - min is the minimum root (and therefore the global minimum).
//
// It reads internals without any locking — single-goroutine tests only.
func checkInvariants[T any](t *testing.T, q *IndexFibonacciPQ[T]) {
	t.Helper()

	present := 0
	for k, x := range q.nodes {
		if x == nil {
			continue
		}
		present++
		if x.index != k {
			t.Errorf("nodes[%d].index = %d", k, x.index)
		}
	}
	if present != q.length {
		t.Errorf("Len = %d but %d nodes are present", q.length, present)
	}
	if q.length == 0 {
		if q.min != nil {
			t.Errorf("empty queue has a non-nil min")
		}
		return
	}
	if q.min == nil {
		t.Fatal("non-empty queue has a nil min")
	}

	visited := make(map[*fibNode[T]]bool)
	var visit func(n *fibNode[T]) int // returns the subtree size
	visit = func(n *fibNode[T]) int {
		if visited[n] {
			t.Fatalf("node %v reachable twice — a ring is crossed", n.value)
		}
		visited[n] = true
		size := 1
		children := 0
		if c := n.child; c != nil {
			cur := c
			for {
				if cur.parent != n {
					t.Errorf("child %v points at parent %p, want this node", cur.value, cur.parent)
				}
				if q.cmp(cur.value, n.value) < 0 {
					t.Errorf("heap order violated: child %v < parent %v", cur.value, n.value)
				}
				if cur.left.right != cur || cur.right.left != cur {
					t.Errorf("child-ring links broken around %v", cur.value)
				}
				size += visit(cur)
				children++
				cur = cur.right
				if cur == c {
					break
				}
			}
		}
		if children != n.degree {
			t.Errorf("node %v: degree %d but %d children", n.value, n.degree, children)
		}
		if min := fib(n.degree + 2); size < min {
			t.Errorf("node %v: degree %d roots %d nodes, Fibonacci lemma wants >= F(%d) = %d",
				n.value, n.degree, size, n.degree+2, min)
		}
		return size
	}

	total := 0
	cur := q.min
	for {
		if cur.parent != nil {
			t.Errorf("root %v has a parent", cur.value)
		}
		if cur.mark {
			t.Errorf("root %v is marked", cur.value)
		}
		if cur.left.right != cur || cur.right.left != cur {
			t.Errorf("root-ring links broken around %v", cur.value)
		}
		if q.cmp(cur.value, q.min.value) < 0 {
			t.Errorf("min %v is not the minimum root (%v is smaller)", q.min.value, cur.value)
		}
		total += visit(cur)
		cur = cur.right
		if cur == q.min {
			break
		}
	}
	if total != q.length {
		t.Errorf("structure holds %d nodes, Len = %d", total, q.length)
	}
}

// TestIndexFibonacciPQRandomizedModel cross-checks every return value
// of every operation against pluto's index_pq (the binary-heap
// IndexMinPQ — a structurally different implementation of the same
// contract) and against a map oracle that knows the ground truth.
// Values are drawn from a wide range so they never tie: with ties the
// two structures may legally pop different indices of the equal
// minimum, after which every presence-sensitive operation would
// diverge (tie behavior is covered by TestTies against the map oracle
// only).
func TestIndexFibonacciPQRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 7))
	const n = 64
	q := NewIndexFibonacciPQ[int](n)
	oracle := index_pq.NewIndexPQ[int](n)
	model := make(map[int]int)

	minVal := func() (int, bool) {
		best, ok := 0, false
		for _, v := range model {
			if !ok || v < best {
				best, ok = v, true
			}
		}
		return best, ok
	}

	for step := range 4000 {
		k := rng.IntN(n)
		v := int(rng.Int64N(1 << 30)) // wide range → no ties
		switch rng.IntN(12) {
		case 0, 1, 2, 3: // Insert (replace when present)
			if got, want := q.Insert(k, v), oracle.Insert(k, v); got != want {
				t.Fatalf("step %d: Insert(%d, %d) = %v, oracle %v", step, k, v, got, want)
			}
			model[k] = v
		case 4, 5: // Change, either direction
			if got, want := q.Change(k, v), oracle.Change(k, v); got != want {
				t.Fatalf("step %d: Change(%d, %d) = %v, oracle %v", step, k, v, got, want)
			}
			if oracle.Contains(k) {
				model[k] = v
			}
		case 6: // DecreaseKey
			got := q.DecreaseKey(k, v)
			want := false
			if cur, ok := model[k]; ok && v < cur {
				want = true
				oracle.Change(k, v)
				model[k] = v
			}
			if got != want {
				t.Fatalf("step %d: DecreaseKey(%d, %d) = %v, want %v", step, k, v, got, want)
			}
		case 7: // Delete
			if got, want := q.Delete(k), oracle.Delete(k); got != want {
				t.Fatalf("step %d: Delete(%d) = %v, oracle %v", step, k, got, want)
			}
			delete(model, k)
			if q.Contains(k) {
				t.Fatalf("step %d: Contains(%d) after Delete", step, k)
			}
		case 8, 9: // Pop
			gk, gv, gok := q.Pop()
			wk, wv, wok := oracle.Pop()
			if gok != wok {
				t.Fatalf("step %d: Pop ok = %v, oracle %v", step, gok, wok)
			}
			if gok {
				if gk != wk || gv != wv {
					t.Fatalf("step %d: Pop = (%d, %d), oracle (%d, %d)", step, gk, gv, wk, wv)
				}
				if mv, ok := minVal(); !ok || gv != mv {
					t.Fatalf("step %d: Pop value = %d, ground-truth min = %d (%v)", step, gv, mv, ok)
				}
				if model[gk] != gv {
					t.Fatalf("step %d: Pop returned index %d whose model value is %d, popped %d", step, gk, model[gk], gv)
				}
				delete(model, gk)
			}
		case 10: // Peek
			gk, gv, gok := q.Peek()
			wk, wv, wok := oracle.Peek()
			if gok != wok {
				t.Fatalf("step %d: Peek ok = %v, oracle %v", step, gok, wok)
			}
			if gok {
				if gk != wk || gv != wv {
					t.Fatalf("step %d: Peek = (%d, %d), oracle (%d, %d)", step, gk, gv, wk, wv)
				}
				if model[gk] != gv {
					t.Fatalf("step %d: Peek returned index %d whose model value is %d, peeked %d", step, gk, model[gk], gv)
				}
			}
		case 11: // Contains / Value / Len agreement
			if q.Contains(k) != oracle.Contains(k) {
				t.Fatalf("step %d: Contains(%d) disagrees with oracle", step, k)
			}
			gv, gok := q.Value(k)
			wv, wok := oracle.Value(k)
			if gok != wok || (gok && gv != wv) {
				t.Fatalf("step %d: Value(%d) = (%d, %v), oracle (%d, %v)", step, k, gv, gok, wv, wok)
			}
			if q.Len() != oracle.Len() || q.Len() != len(model) {
				t.Fatalf("step %d: Len = %d, oracle %d, model %d", step, q.Len(), oracle.Len(), len(model))
			}
		}
		if step%37 == 0 {
			checkInvariants(t, q)
			for i := range n {
				if q.Contains(i) != oracle.Contains(i) {
					t.Fatalf("step %d: full-state Contains(%d) disagrees with oracle", step, i)
				}
			}
		}
	}

	// Final drain: both structures must agree on the full order.
	var gotK, gotV, wantK, wantV []int
	for k, v := range q.All() {
		gotK, gotV = append(gotK, k), append(gotV, v)
	}
	for k, v := range oracle.All() {
		wantK, wantV = append(wantK, k), append(wantV, v)
	}
	if !slices.Equal(gotK, wantK) || !slices.Equal(gotV, wantV) {
		t.Fatalf("All after the run = (%v, %v), oracle (%v, %v)", gotK, gotV, wantK, wantV)
	}
}

// TestTies covers equal values, where Pop may return any index holding
// the minimum value: every index must come out exactly once, values
// nondecreasing, and the popped pair must be self-consistent.
func TestTies(t *testing.T) {
	const n = 100
	q := NewIndexFibonacciPQ[int](n)
	for i := range n {
		q.Insert(i, i/10) // ten indices per value 0..9
	}
	seen := make([]bool, n)
	prev := -1
	for range n {
		k, v, ok := q.Pop()
		if !ok {
			t.Fatal("Pop failed before the queue emptied")
		}
		if seen[k] {
			t.Fatalf("index %d popped twice", k)
		}
		seen[k] = true
		if v != k/10 {
			t.Fatalf("Pop returned (%d, %d); value %d does not belong to index %d", k, v, v, k)
		}
		if v < prev {
			t.Fatalf("values out of order: %d after %d", v, prev)
		}
		prev = v
	}
	if !q.IsEmpty() {
		t.Fatal("queue not empty after popping every index")
	}
}

// TestCascadingCutsAndDegreeBound builds a consolidated forest (repeated
// Pops link same-degree trees), then decreases every remaining key —
// each a cut, many cascading — and requires the invariants (including
// the Fibonacci size lemma, which bounds the degrees) to hold after
// each phase, then a fully sorted drain.
func TestCascadingCutsAndDegreeBound(t *testing.T) {
	const n = 4096
	q := NewIndexFibonacciPQ[int](n)
	for i := range n {
		q.Insert(i, i) // all lazy singleton roots
	}
	checkInvariants(t, q)

	// Pop a third: every Pop consolidates, so the roots link up into
	// high-degree heap-ordered trees.
	for range n / 3 {
		if _, _, ok := q.Pop(); !ok {
			t.Fatal("Pop on a non-empty queue failed")
		}
	}
	checkInvariants(t, q)

	// Decrease every remaining key below everything popped so far —
	// each call cuts a node out of its (now deep) tree.
	for i := n / 3; i < n; i++ {
		if !q.DecreaseKey(i, i-n) {
			t.Fatalf("DecreaseKey(%d, %d) = false", i, i-n)
		}
	}
	checkInvariants(t, q)

	// Drain in sorted order.
	prev := -(1 << 30)
	count := 0
	for {
		_, v, ok := q.Pop()
		if !ok {
			break
		}
		if v < prev {
			t.Fatalf("drain out of order: %d after %d", v, prev)
		}
		prev = v
		count++
	}
	if count != n-n/3 {
		t.Fatalf("drained %d items, want %d", count, n-n/3)
	}
}

// TestChangeIncreaseOnChild exercises the increase-direction Change on
// a node buried in a tree (not a root): the node is removed via the
// extract machinery — children promoted, parents cascaded — and
// re-inserted with the new value.
func TestChangeIncreaseOnChild(t *testing.T) {
	const n = 256
	q := NewIndexFibonacciPQ[int](n)
	for i := range n {
		q.Insert(n-1-i, i) // descending insertion order
	}
	// Consolidate into trees: pop most of the queue.
	for range n - 8 {
		q.Pop()
	}
	checkInvariants(t, q)
	// Whatever remains, increase every remaining value by a large
	// constant and re-check: the survivors must still drain sorted and
	// keep their (shifted) values.
	for k := range n {
		if v, ok := q.Value(k); ok {
			if !q.Change(k, v+10000) {
				t.Fatalf("Change(%d) = false on a present index", k)
			}
		}
	}
	checkInvariants(t, q)
	var got []int
	for {
		_, v, ok := q.Pop()
		if !ok {
			break
		}
		got = append(got, v)
	}
	if !slices.IsSorted(got) {
		t.Fatalf("drain after increases not sorted: %v", got)
	}
	for _, v := range got {
		if v < 10000 || v >= 10000+n {
			t.Fatalf("drain value %d outside the shifted range", v)
		}
	}
}

// TestLargeSequentialDrain is a scale check: the degree bound and the
// size lemma hold at 100k elements and the full drain comes out sorted.
func TestLargeSequentialDrain(t *testing.T) {
	const n = 100_000
	q := NewIndexFibonacciPQ[int](n)
	for i := range n {
		q.Insert(n-1-i, i)
	}
	if q.Len() != n {
		t.Fatalf("Len = %d, want %d", q.Len(), n)
	}
	prev := -1
	for range n {
		_, v, ok := q.Pop()
		if !ok {
			t.Fatal("Pop failed mid-drain")
		}
		if v < prev {
			t.Fatalf("drain out of order: %d after %d", v, prev)
		}
		prev = v
	}
	if !q.IsEmpty() {
		t.Fatal("queue not empty after draining n items")
	}
}

// TestCustomKey runs the queue on a struct ordered by one field with
// satellite data, verifying duplicate inserts replace the satellite
// value (the TestTreeNode shape used across the tree tests).
func TestCustomKey(t *testing.T) {
	type task struct {
		priority int
		name     string
	}
	q := NewIndexFibonacciPQFunc(4, func(a, b task) int {
		return Compare(a.priority, b.priority)
	})
	q.Insert(0, task{3, "write"})
	q.Insert(1, task{1, "review"})
	q.Insert(2, task{2, "test"})
	q.Insert(0, task{0, "plan"}) // replace: same priority slot, new satellite
	v, ok := q.Value(0)
	if !ok || v != (task{0, "plan"}) {
		t.Fatalf("Value(0) = (%v, %v), want ({0 plan}, true)", v, ok)
	}
	var names []string
	for _, v := range q.All() {
		names = append(names, v.name)
	}
	if want := []string{"plan", "review", "test"}; !slices.Equal(names, want) {
		t.Fatalf("All names = %v, want %v", names, want)
	}
}
