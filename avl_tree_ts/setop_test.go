/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package avl_tree_ts

// Twin tests for the merge-and-build set operations: which equal element
// each operation keeps, op-by-op parity with the plain avl_tree package
// (whose own tests compare against the original insert-based algorithm),
// aliasing, nil/zero-value operands and comparator adoption, foreign
// orderings, results that stay mutable, and a race test with concurrent
// writers to the operands during set operations (run with -race).

import (
	"fmt"
	"math/bits"
	"math/rand/v2"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"weak"

	"github.com/pschlump/pluto/avl_tree"
)

type tsOp struct {
	name  string
	ts    func(tt, yy, zz *AvlTree[TestTreeNode])
	plain func(tt, yy, zz *avl_tree.AvlTree[TestTreeNode])
}

var tsSetOps = []tsOp{
	{"Union", func(tt, yy, zz *AvlTree[TestTreeNode]) { tt.Union(yy, zz) },
		func(tt, yy, zz *avl_tree.AvlTree[TestTreeNode]) { tt.Union(yy, zz) }},
	{"Minus", func(tt, yy, zz *AvlTree[TestTreeNode]) { tt.Minus(yy, zz) },
		func(tt, yy, zz *avl_tree.AvlTree[TestTreeNode]) { tt.Minus(yy, zz) }},
	{"Intersect", func(tt, yy, zz *AvlTree[TestTreeNode]) { tt.Intersect(yy, zz) },
		func(tt, yy, zz *avl_tree.AvlTree[TestTreeNode]) { tt.Intersect(yy, zz) }},
	{"SymmetricDifference", func(tt, yy, zz *AvlTree[TestTreeNode]) { tt.SymmetricDifference(yy, zz) },
		func(tt, yy, zz *avl_tree.AvlTree[TestTreeNode]) { tt.SymmetricDifference(yy, zz) }},
	{"Copy", func(tt, yy, _ *AvlTree[TestTreeNode]) { tt.Copy(yy) },
		func(tt, yy, _ *avl_tree.AvlTree[TestTreeNode]) { tt.Copy(yy) }},
}

// tsPair is the same logical tree in both packages.
type tsPair struct {
	ts    *AvlTree[TestTreeNode]
	plain *avl_tree.AvlTree[TestTreeNode]
}

func newPair(r *rand.Rand, n, keyRange, tag int) tsPair {
	p := tsPair{newTestTree(), avl_tree.NewAvlTreeFunc(cmpTestTreeNode)}
	for i := range n {
		e := TestTreeNode{S: fmt.Sprintf("%08d", r.IntN(keyRange)), N: tag*1_000_000 + i}
		p.ts.Insert(e)
		p.plain.Insert(e)
	}
	return p
}

func (p tsPair) clone() tsPair { return tsPair{p.ts.Clone(), p.plain.Clone()} }

// checkParity compares the twin's tree with the plain one: elements with
// satellite data, Len, Depth (both are built the same way), structure.
func checkParity(t *testing.T, label string, p tsPair, balanced bool) {
	t.Helper()
	if (p.ts == nil) != (p.plain == nil) {
		t.Fatalf("%s: nil mismatch", label)
	}
	if p.ts == nil {
		return
	}
	validateAVLNode(t, p.ts.root)
	g, w := p.ts.Slice(), p.plain.Slice()
	if !slices.Equal(g, w) {
		t.Fatalf("%s: elements\n got %v\nwant %v", label, g, w)
	}
	if p.ts.Len() != p.plain.Len() || p.ts.Depth() != p.plain.Depth() {
		t.Fatalf("%s: Len/Depth %d/%d, plain %d/%d", label, p.ts.Len(), p.ts.Depth(), p.plain.Len(), p.plain.Depth())
	}
	if balanced && p.ts.Depth() != bits.Len(uint(p.ts.Len())) {
		t.Fatalf("%s: depth %d, perfectly balanced %d", label, p.ts.Depth(), bits.Len(uint(p.ts.Len())))
	}
}

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
				dst = &AvlTree[TestTreeNode]{}
			} else {
				dst.Insert(TestTreeNode{S: "zzz"})
			}
			c.fn(dst)
			if got := render(dst); got != c.want {
				t.Errorf("%s (zero dst %v): got %q want %q", c.name, zeroDst, got, c.want)
			}
			validateAVLNode(t, dst.root)
		}
	}
}

// TestSetOpParity runs every set operation on both packages over
// fixed-seed random operands (sizes 0..200 plus large ones, three overlap
// levels), then mutates the results identically.
func TestSetOpParity(t *testing.T) {
	r := rand.New(rand.NewPCG(42, 7))
	sizes := make([]int, 0, 210)
	for n := 0; n <= 200; n++ {
		sizes = append(sizes, n)
	}
	sizes = append(sizes, 1000, 4097)
	for _, n := range sizes {
		m := r.IntN(n + 3)
		for _, spread := range []int{1, 2, 8} {
			keyRange := max(1, (n+m)*spread/2+1)
			yy := newPair(r, n, keyRange, 1)
			zz := newPair(r, m, keyRange, 2)
			for _, c := range tsSetOps {
				res := tsPair{newTestTree(), avl_tree.NewAvlTreeFunc(cmpTestTreeNode)}
				c.ts(res.ts, yy.ts, zz.ts)
				c.plain(res.plain, yy.plain, zz.plain)
				label := fmt.Sprintf("%s n=%d m=%d spread=%d", c.name, n, m, spread)
				checkParity(t, label, res, true)
				for k := range 6 {
					key := TestTreeNode{S: fmt.Sprintf("%08d", r.IntN(keyRange)), N: 9_000_000 + k}
					if k%2 == 0 {
						if res.ts.Delete(key) != res.plain.Delete(key) {
							t.Fatalf("%s: Delete disagrees", label)
						}
					} else if res.ts.Insert(key) != res.plain.Insert(key) {
						t.Fatalf("%s: Insert disagrees", label)
					}
					checkParity(t, label+" after mutation", res, false)
				}
			}
		}
	}
}

// TestSetOpAliasingAndOperands covers aliasing destinations, nil and
// zero-value operands, comparator adoption and foreign orderings.
func TestSetOpAliasingAndOperands(t *testing.T) {
	r := rand.New(rand.NewPCG(5, 6))
	for _, n := range []int{0, 1, 2, 3, 7, 64, 129} {
		base := newPair(r, n, 2*n+1, 1)
		other := newPair(r, n, 2*n+1, 2)
		for _, c := range tsSetOps {
			p := base.clone()
			c.ts(p.ts, p.ts, other.ts)
			c.plain(p.plain, p.plain, other.plain)
			checkParity(t, c.name+" tt=op(tt,o)", p, true)
			p = base.clone()
			c.ts(p.ts, other.ts, p.ts)
			c.plain(p.plain, other.plain, p.plain)
			checkParity(t, c.name+" tt=op(o,tt)", p, true)
			p = base.clone()
			c.ts(p.ts, p.ts, p.ts)
			c.plain(p.plain, p.plain, p.plain)
			checkParity(t, c.name+" tt=op(tt,tt)", p, true)

			// Reverse()d operand: not ascending, insert-built fallback.
			ry := base.clone()
			ry.ts.Reverse()
			ry.plain.Reverse()
			p = tsPair{newTestTree(), avl_tree.NewAvlTreeFunc(cmpTestTreeNode)}
			c.ts(p.ts, ry.ts, other.ts)
			c.plain(p.plain, ry.plain, other.plain)
			checkParity(t, c.name+" reversed operand", p, false)
		}
	}

	// Operands ordered by a FINER comparator (S, then N) than the
	// destination's: adjacent duplicates under the destination's ordering
	// must go to the insert-built fallback (later duplicate wins).
	fine := func(a, b TestTreeNode) int {
		if c := cmpTestTreeNode(a, b); c != 0 {
			return c
		}
		return a.N - b.N
	}
	for _, n := range []int{2, 5, 40} {
		yy := tsPair{NewAvlTreeFunc(fine), avl_tree.NewAvlTreeFunc(fine)}
		zz := tsPair{NewAvlTreeFunc(fine), avl_tree.NewAvlTreeFunc(fine)}
		for i := range n {
			e := TestTreeNode{S: fmt.Sprintf("%08d", r.IntN(n/2+1)), N: r.IntN(3)}
			yy.ts.Insert(e)
			yy.plain.Insert(e)
			e = TestTreeNode{S: fmt.Sprintf("%08d", r.IntN(n/2+1)), N: 10 + i%3}
			zz.ts.Insert(e)
			zz.plain.Insert(e)
		}
		for _, c := range tsSetOps {
			p := tsPair{newTestTree(), avl_tree.NewAvlTreeFunc(cmpTestTreeNode)}
			c.ts(p.ts, yy.ts, zz.ts)
			c.plain(p.plain, yy.plain, zz.plain)
			checkParity(t, fmt.Sprintf("%s finer operands n=%d", c.name, n), p, false)
		}
	}

	full := newPair(r, 20, 30, 1)
	mk := map[string]func() tsPair{
		"nil":   func() tsPair { return tsPair{} },
		"zero":  func() tsPair { return tsPair{&AvlTree[TestTreeNode]{}, &avl_tree.AvlTree[TestTreeNode]{}} },
		"empty": func() tsPair { return tsPair{newTestTree(), avl_tree.NewAvlTreeFunc(cmpTestTreeNode)} },
		"full":  full.clone,
	}
	for _, c := range tsSetOps {
		for dn, dmk := range mk {
			for yn, ymk := range mk {
				for zn, zmk := range mk {
					d, y, z := dmk(), ymk(), zmk()
					c.ts(d.ts, y.ts, z.ts)
					c.plain(d.plain, y.plain, z.plain)
					label := fmt.Sprintf("%s dst=%s yy=%s zz=%s", c.name, dn, yn, zn)
					checkParity(t, label, d, true)
					if d.ts != nil && d.ts.cmp != nil {
						e := TestTreeNode{S: "00000015", N: -1}
						d.ts.Insert(e)
						d.plain.Insert(e)
						checkParity(t, label+" adopted insert", d, false)
					}
				}
			}
		}
	}
}

// TestSetOpRace runs set operations while other goroutines write to the
// operands (and use the destinations as operands of their own set
// operations).  Run with -race; afterwards every tree must be valid.
func TestSetOpRace(t *testing.T) {
	yy, zz := newTestTree(), newTestTree()
	dst1, dst2 := newTestTree(), &AvlTree[TestTreeNode]{}
	for i := range 200 {
		yy.Insert(mkKey(i))
		zz.Insert(mkKey(i + 100))
	}
	var wg sync.WaitGroup
	for w := range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := rand.New(rand.NewPCG(uint64(w), 99))
			for i := range 400 {
				k := mkKey(r.IntN(400))
				switch i % 4 {
				case 0:
					yy.Insert(k)
				case 1:
					zz.Delete(k)
				case 2:
					zz.Upsert(k)
				default:
					yy.Remove(k)
				}
			}
		}()
	}
	for g := range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range 60 {
				switch (i + g) % 6 {
				case 0:
					dst1.Union(yy, zz)
				case 1:
					dst1.Intersect(zz, yy)
				case 2:
					dst2.Minus(yy, dst1)
				case 3:
					dst2.SymmetricDifference(dst1, zz)
				case 4:
					dst1.Union(dst1, dst2) // destination aliases an operand
				default:
					dst2.Copy(dst1)
				}
			}
		}()
	}
	wg.Wait()
	for i, tr := range []*AvlTree[TestTreeNode]{yy, zz, dst1, dst2} {
		validateAVLNode(t, tr.root)
		if got := len(tr.Slice()); got != tr.Len() {
			t.Fatalf("tree %d: Len %d, %d elements", i, tr.Len(), got)
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
