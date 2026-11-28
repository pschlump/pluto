/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package avl_tree

// Tests for the merge-and-build set operations.  The oracle is the
// previous insert-based implementation of Copy/Union/Minus/Intersect/
// SymmetricDifference, kept here verbatim (modulo naming): every result is
// compared against it element by element, including the satellite N field
// the comparison ignores, so the tests pin WHICH of two equal elements a
// set operation keeps, not just the key set.

import (
	"fmt"
	"math/bits"
	"math/rand/v2"
	"runtime"
	"slices"
	"strings"
	"testing"
	"weak"
)

// -------------------------------------------------------------------------------------------------------
// Oracle: the old insert-per-element implementation.

func oracleCopy[T any](tt, yy *AvlTree[T]) {
	if tt == nil {
		return
	}
	data := yy.toSlice()
	tt.root, tt.length = nil, 0
	if !tt.adoptCmp(yy, nil) {
		return
	}
	for _, d := range data {
		tt.insert(d)
	}
}

func oracleUnion[T any](tt, yy, zz *AvlTree[T]) {
	if tt == nil {
		return
	}
	a, b := yy.toSlice(), zz.toSlice()
	tt.root, tt.length = nil, 0
	if !tt.adoptCmp(yy, zz) {
		return
	}
	for _, d := range a {
		tt.insert(d)
	}
	for _, d := range b {
		tt.insert(d)
	}
}

func oracleMinus[T any](tt, yy, zz *AvlTree[T]) {
	if tt == nil {
		return
	}
	a, b := yy.toSlice(), zz.toSlice()
	tt.root, tt.length = nil, 0
	if !tt.adoptCmp(yy, zz) {
		return
	}
	var zzTree AvlTree[T]
	zzTree.cmp = tt.cmp
	for _, d := range b {
		zzTree.insert(d)
	}
	for _, d := range a {
		if _, found := zzTree.Search(d); !found {
			tt.insert(d)
		}
	}
}

func oracleIntersect[T any](tt, yy, zz *AvlTree[T]) {
	if tt == nil {
		return
	}
	a, b := yy.toSlice(), zz.toSlice()
	tt.root, tt.length = nil, 0
	if !tt.adoptCmp(yy, zz) {
		return
	}
	var zzTree AvlTree[T]
	zzTree.cmp = tt.cmp
	for _, d := range b {
		zzTree.insert(d)
	}
	for _, d := range a {
		if _, found := zzTree.Search(d); found {
			tt.insert(d)
		}
	}
}

func oracleSymDiff[T any](tt, yy, zz *AvlTree[T]) {
	if tt == nil {
		return
	}
	a, b := yy.toSlice(), zz.toSlice()
	tt.root, tt.length = nil, 0
	if !tt.adoptCmp(yy, zz) {
		return
	}
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		switch c := tt.cmp(a[i], b[j]); {
		case c < 0:
			tt.insert(a[i])
			i++
		case c > 0:
			tt.insert(b[j])
			j++
		default:
			i++
			j++
		}
	}
	for ; i < len(a); i++ {
		tt.insert(a[i])
	}
	for ; j < len(b); j++ {
		tt.insert(b[j])
	}
}

// -------------------------------------------------------------------------------------------------------
// Helpers.

// setOpCase names a binary set operation with its new and oracle forms.
type setOpCase struct {
	name   string
	got    func(tt, yy, zz *AvlTree[TestTreeNode])
	oracle func(tt, yy, zz *AvlTree[TestTreeNode])
}

var setOpCases = []setOpCase{
	{"Union", func(tt, yy, zz *AvlTree[TestTreeNode]) { tt.Union(yy, zz) }, oracleUnion[TestTreeNode]},
	{"Minus", func(tt, yy, zz *AvlTree[TestTreeNode]) { tt.Minus(yy, zz) }, oracleMinus[TestTreeNode]},
	{"Intersect", func(tt, yy, zz *AvlTree[TestTreeNode]) { tt.Intersect(yy, zz) }, oracleIntersect[TestTreeNode]},
	{"SymmetricDifference", func(tt, yy, zz *AvlTree[TestTreeNode]) { tt.SymmetricDifference(yy, zz) }, oracleSymDiff[TestTreeNode]},
	// Copy as a binary case: tt = copy of yy (zz ignored).
	{"Copy", func(tt, yy, _ *AvlTree[TestTreeNode]) { tt.Copy(yy) }, func(tt, yy, _ *AvlTree[TestTreeNode]) { oracleCopy(tt, yy) }},
}

// elems returns the full elements (key and satellite) in in-order.
func elems(tt *AvlTree[TestTreeNode]) []TestTreeNode {
	if tt == nil {
		return nil
	}
	var out []TestTreeNode
	for v := range tt.All() {
		out = append(out, v)
	}
	return out
}

// checkSetOpResult validates the structure of a set-operation result
// (BST order, AVL balance, every cached height, Len vs node count) and
// compares it with the oracle's result, satellite data included.
func checkSetOpResult(t *testing.T, label string, got, want *AvlTree[TestTreeNode], balanced bool) {
	t.Helper()
	if got == nil || want == nil {
		if got != want {
			t.Fatalf("%s: nil mismatch", label)
		}
		return
	}
	validateAVLNode(t, got.root)
	g, w := elems(got), elems(want)
	if !slices.Equal(g, w) {
		t.Fatalf("%s: elements\n got %v\nwant %v", label, g, w)
	}
	if got.Len() != len(g) {
		t.Fatalf("%s: Len %d, %d nodes", label, got.Len(), len(g))
	}
	if (got.cmp == nil) != (want.cmp == nil) {
		t.Fatalf("%s: comparator presence got %v want %v", label, got.cmp != nil, want.cmp != nil)
	}
	// Straight out of the merge-and-build path, n nodes have the minimum
	// height bits.Len(n) (the oracle's insert-built tree may be taller).
	if balanced {
		if wantD := bits.Len(uint(got.Len())); got.Depth() != wantD {
			t.Fatalf("%s: depth %d, perfectly balanced %d", label, got.Depth(), wantD)
		}
	}
}

// randSetTree builds a tree of n random keys drawn from [0, keyRange),
// tagging each element's satellite N with tag*1_000_000 + insertion step
// so the source of every element in a result is identifiable.
func randSetTree(r *rand.Rand, n, keyRange, tag int) *AvlTree[TestTreeNode] {
	tt := newTestTree()
	for i := range n {
		tt.Insert(TestTreeNode{S: fmt.Sprintf("%08d", r.IntN(keyRange)), N: tag*1_000_000 + i})
	}
	return tt
}

// cloneTestTree deep-copies through the oracle so the two sides of a
// differential test start from identical, independent trees.
func cloneTestTree(tt *AvlTree[TestTreeNode]) *AvlTree[TestTreeNode] {
	if tt == nil {
		return nil
	}
	return tt.Clone()
}

// -------------------------------------------------------------------------------------------------------
// Tests.

// TestSetOpEqualElementKept pins which of two equal elements each set
// operation keeps (the satellite N tells them apart): Union keeps zz's,
// Intersect keeps yy's, Minus and Copy keep yy's.  These are the
// semantics of the original insert-based implementation.
func TestSetOpEqualElementKept(t *testing.T) {
	yy, zz := newTestTree(), newTestTree()
	for _, s := range []string{"a", "b", "c", "d"} {
		yy.Insert(TestTreeNode{S: s, N: 1})
	}
	for _, s := range []string{"c", "d", "e", "f"} {
		zz.Insert(TestTreeNode{S: s, N: 2})
	}
	render := func(tt *AvlTree[TestTreeNode]) string {
		var sb strings.Builder
		for v := range tt.All() {
			fmt.Fprintf(&sb, "%s%d ", v.S, v.N)
		}
		return strings.TrimSpace(sb.String())
	}
	cases := []struct {
		name string
		fn   func(tt *AvlTree[TestTreeNode])
		want string
	}{
		{"Union", func(tt *AvlTree[TestTreeNode]) { tt.Union(yy, zz) }, "a1 b1 c2 d2 e2 f2"},
		{"UnionSwapped", func(tt *AvlTree[TestTreeNode]) { tt.Union(zz, yy) }, "a1 b1 c1 d1 e2 f2"},
		{"Intersect", func(tt *AvlTree[TestTreeNode]) { tt.Intersect(yy, zz) }, "c1 d1"},
		{"IntersectSwapped", func(tt *AvlTree[TestTreeNode]) { tt.Intersect(zz, yy) }, "c2 d2"},
		{"Minus", func(tt *AvlTree[TestTreeNode]) { tt.Minus(yy, zz) }, "a1 b1"},
		{"MinusSwapped", func(tt *AvlTree[TestTreeNode]) { tt.Minus(zz, yy) }, "e2 f2"},
		{"SymmetricDifference", func(tt *AvlTree[TestTreeNode]) { tt.SymmetricDifference(yy, zz) }, "a1 b1 e2 f2"},
		{"Copy", func(tt *AvlTree[TestTreeNode]) { tt.Copy(yy) }, "a1 b1 c1 d1"},
	}
	for _, c := range cases {
		for _, zeroDst := range []bool{false, true} {
			dst := newTestTree()
			if zeroDst {
				dst = &AvlTree[TestTreeNode]{} // adopts an operand's comparator
			} else {
				dst.Insert(TestTreeNode{S: "zzz"}) // pre-existing content must vanish
			}
			c.fn(dst)
			if got := render(dst); got != c.want {
				t.Errorf("%s (zero dst %v): got %q want %q", c.name, zeroDst, got, c.want)
			}
			validateAVLNode(t, dst.root)
		}
	}
}

// TestSetOpDifferential compares every set operation with the oracle over
// fixed-seed random operands: sizes 0..200 and a few large ones, key
// ranges giving disjoint-ish, partial and heavy overlap, and every
// result is validated structurally and then mutated (inserts and
// deletes) to show it remains a fully working AVL tree.
func TestSetOpDifferential(t *testing.T) {
	r := rand.New(rand.NewPCG(42, 7))
	sizes := make([]int, 0, 210)
	for n := 0; n <= 200; n++ {
		sizes = append(sizes, n)
	}
	sizes = append(sizes, 1000, 4097, 20000)
	for _, n := range sizes {
		m := r.IntN(n + 3)
		for _, spread := range []int{1, 2, 8} { // key range relative to size: 1 = heavy overlap
			keyRange := max(1, (n+m)*spread/2+1)
			yy := randSetTree(r, n, keyRange, 1)
			zz := randSetTree(r, m, keyRange, 2)
			for _, c := range setOpCases {
				got, want := newTestTree(), newTestTree()
				c.got(got, yy, zz)
				c.oracle(want, yy, zz)
				label := fmt.Sprintf("%s n=%d m=%d spread=%d", c.name, n, m, spread)
				checkSetOpResult(t, label, got, want, true)
				if n > 300 && spread != 2 {
					continue // the large sizes: one mutation round is plenty
				}
				// The result must stay fully mutable: delete and insert
				// into it, mirroring on the oracle, validating each time.
				for k := 0; k < 8; k++ {
					key := TestTreeNode{S: fmt.Sprintf("%08d", r.IntN(keyRange)), N: 9_000_000 + k}
					if k%2 == 0 {
						if got.Delete(key) != want.Delete(key) {
							t.Fatalf("%s: Delete(%v) disagrees", label, key)
						}
					} else {
						if got.Insert(key) != want.Insert(key) {
							t.Fatalf("%s: Insert(%v) disagrees", label, key)
						}
					}
					checkSetOpResult(t, label+" after mutation", got, want, false)
				}
				if got.Len() <= 400 { // drain, validating each step (quadratic: small only)
					for got.DeleteAtHead() {
						validateAVLNode(t, got.root)
					}
				}
			}
		}
	}
}

// TestSetOpAliasing covers destinations that alias one or both operands.
func TestSetOpAliasing(t *testing.T) {
	r := rand.New(rand.NewPCG(5, 6))
	for _, n := range []int{0, 1, 2, 3, 7, 64, 129} {
		base := randSetTree(r, n, 2*n+1, 1)
		other := randSetTree(r, n, 2*n+1, 2)
		for _, c := range setOpCases {
			shapes := []struct {
				name string
				run  func(op func(tt, yy, zz *AvlTree[TestTreeNode]), tt, o *AvlTree[TestTreeNode])
			}{
				{"tt=op(tt,o)", func(op func(tt, yy, zz *AvlTree[TestTreeNode]), tt, o *AvlTree[TestTreeNode]) { op(tt, tt, o) }},
				{"tt=op(o,tt)", func(op func(tt, yy, zz *AvlTree[TestTreeNode]), tt, o *AvlTree[TestTreeNode]) { op(tt, o, tt) }},
				{"tt=op(tt,tt)", func(op func(tt, yy, zz *AvlTree[TestTreeNode]), tt, _ *AvlTree[TestTreeNode]) { op(tt, tt, tt) }},
			}
			for _, s := range shapes {
				got, want := cloneTestTree(base), cloneTestTree(base)
				s.run(c.got, got, other)
				s.run(c.oracle, want, cloneTestTree(other))
				checkSetOpResult(t, fmt.Sprintf("%s %s n=%d", c.name, s.name, n), got, want, true)
			}
		}
	}
}

// TestSetOpNilZeroAndAdoption covers nil and zero-value operands and
// destinations and the comparator-adoption rules.
func TestSetOpNilZeroAndAdoption(t *testing.T) {
	r := rand.New(rand.NewPCG(8, 9))
	full := randSetTree(r, 20, 30, 1)
	full2 := randSetTree(r, 20, 30, 2)
	operands := map[string]func() *AvlTree[TestTreeNode]{
		"nil":   func() *AvlTree[TestTreeNode] { return nil },
		"zero":  func() *AvlTree[TestTreeNode] { return &AvlTree[TestTreeNode]{} },
		"empty": newTestTree,
		"full":  func() *AvlTree[TestTreeNode] { return cloneTestTree(full) },
		"full2": func() *AvlTree[TestTreeNode] { return cloneTestTree(full2) },
	}
	dsts := map[string]func() *AvlTree[TestTreeNode]{
		"nil":  func() *AvlTree[TestTreeNode] { return nil },
		"zero": func() *AvlTree[TestTreeNode] { return &AvlTree[TestTreeNode]{} },
		"full": func() *AvlTree[TestTreeNode] { return cloneTestTree(full2) },
	}
	for _, c := range setOpCases {
		for dn, dmk := range dsts {
			for yn, ymk := range operands {
				for zn, zmk := range operands {
					got, want := dmk(), dmk()
					c.got(got, ymk(), zmk())
					c.oracle(want, ymk(), zmk())
					checkSetOpResult(t, fmt.Sprintf("%s dst=%s yy=%s zz=%s", c.name, dn, yn, zn), got, want, true)
					if got != nil && got.cmp != nil {
						// An adopted comparator must order the tree.
						got.Insert(TestTreeNode{S: "00000015", N: -1})
						want.Insert(TestTreeNode{S: "00000015", N: -1})
						checkSetOpResult(t, "adopted insert", got, want, false)
					}
				}
			}
		}
	}
}

// TestSetOpForeignOrder covers operands that are NOT ascending under the
// destination's comparison function — a destination with its own
// (reversed) comparator, and a Reverse()d operand.  The merge cannot be
// used there; the result must still equal the insert-built one exactly.
func TestSetOpForeignOrder(t *testing.T) {
	r := rand.New(rand.NewPCG(10, 11))
	rev := func(a, b TestTreeNode) int { return -cmpTestTreeNode(a, b) }
	for _, n := range []int{0, 1, 2, 5, 40} {
		yy := randSetTree(r, n, n+3, 1)
		zz := randSetTree(r, n, n+3, 2)
		for _, c := range setOpCases {
			got, want := NewAvlTreeFunc(rev), NewAvlTreeFunc(rev)
			c.got(got, yy, zz)
			c.oracle(want, yy, zz)
			if !slices.Equal(elems(got), elems(want)) || got.Len() != want.Len() {
				t.Fatalf("%s reversed dst n=%d: got %v want %v", c.name, n, elems(got), elems(want))
			}
			// Reverse()d operand (in-order is descending) into a normal dst.
			ry, rz := cloneTestTree(yy), cloneTestTree(zz)
			ry.Reverse()
			got, want = newTestTree(), newTestTree()
			c.got(got, ry, rz)
			c.oracle(want, ry, rz)
			checkSetOpResult(t, fmt.Sprintf("%s reversed operand n=%d", c.name, n), got, want, false)
		}
	}

	// Operands ordered by a FINER comparator (S, then N) than the
	// destination's (S only): ascending, but with adjacent duplicates
	// under the destination's ordering — not strictly ascending, so the
	// insert-built fallback (later duplicate wins) must decide.
	fine := func(a, b TestTreeNode) int {
		if c := cmpTestTreeNode(a, b); c != 0 {
			return c
		}
		return a.N - b.N
	}
	for _, n := range []int{2, 5, 40} {
		yy, zz := NewAvlTreeFunc(fine), NewAvlTreeFunc(fine)
		for i := range n {
			yy.Insert(TestTreeNode{S: fmt.Sprintf("%08d", r.IntN(n/2+1)), N: r.IntN(3)})
			zz.Insert(TestTreeNode{S: fmt.Sprintf("%08d", r.IntN(n/2+1)), N: 10 + i%3})
		}
		for _, c := range setOpCases {
			got, want := newTestTree(), newTestTree()
			c.got(got, yy, zz)
			c.oracle(want, yy, zz)
			checkSetOpResult(t, fmt.Sprintf("%s finer operands n=%d", c.name, n), got, want, false)
		}
	}
}

// TestSetOpPanicContract: the set operations never panic, on any
// combination of nil and zero-value receivers and operands.
func TestSetOpPanicContract(t *testing.T) {
	var nilTree *AvlTree[int]
	zero := &AvlTree[int]{}
	nilTree.Copy(zero)
	nilTree.Union(zero, nil)
	nilTree.Minus(nil, nil)
	nilTree.Intersect(nil, zero)
	nilTree.SymmetricDifference(zero, zero)
	zero.Copy(nil)
	zero.Union(nil, nil)
	zero.Minus(zero, nil)
	zero.Intersect(nil, zero)
	zero.SymmetricDifference(zero, zero)
	if zero.Len() != 0 || zero.cmp != nil {
		t.Fatalf("zero-value tree changed: len %d cmp %v", zero.Len(), zero.cmp != nil)
	}
}

// TestSetOpBalancedShape checks the build itself: every result of n
// elements has the minimum possible height, bits.Len(n).
func TestSetOpBalancedShape(t *testing.T) {
	for n := 0; n <= 300; n++ {
		src := NewAvlTree[int]()
		for i := range n {
			src.Insert(i)
		}
		dst := NewAvlTree[int]()
		dst.Copy(src)
		validateIntAVL(t, dst)
		want := 0
		for x := n; x > 0; x >>= 1 {
			want++
		}
		if dst.Depth() != want {
			t.Fatalf("n=%d: depth %d want %d", n, dst.Depth(), want)
		}
		if got := dst.Slice(); !slices.Equal(got, src.Slice()) {
			t.Fatalf("n=%d: Copy elements differ", n)
		}
	}
}

// TestSetOpSlabDeleteReleasesElement: a node deleted from a slab-built
// result is zeroed, so the slab (kept alive by the remaining nodes) does
// not keep the deleted element's referents alive.
func TestSetOpSlabDeleteReleasesElement(t *testing.T) {
	type elem struct {
		k int
		p *[256]byte
	}
	byK := func(a, b elem) int { return a.k - b.k }
	src := NewAvlTreeFunc(byK)
	for i := range 64 {
		src.Insert(elem{k: i, p: new([256]byte)})
	}
	victim, _ := src.Search(elem{k: 31})
	wp := weak.Make(victim.p)
	victim = elem{}
	dst := NewAvlTreeFunc(byK)
	dst.Copy(src)
	src = nil
	_ = src
	if !dst.Delete(elem{k: 31}) {
		t.Fatal("Delete(31) not found")
	}
	runtime.GC()
	runtime.GC()
	if wp.Value() != nil {
		t.Fatal("deleted element still reachable through the slab")
	}
	if dst.Len() != 63 {
		t.Fatalf("Len %d want 63", dst.Len())
	}
	runtime.KeepAlive(dst)
}
