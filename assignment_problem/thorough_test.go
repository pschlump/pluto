/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package assignment_problem

import (
	"math"
	"math/rand/v2"
	"testing"
)

// checkCertificate verifies the complete optimality certificate over the
// caller's matrix, which PROVES the assignment optimal (the LP-duality
// counterpart of bipartite's König certificate):
//
//  1. Sol is a permutation and RowOf its inverse — a perfect matching.
//  2. Weight is the total of the matched entries.
//  3. DualRow(i) + DualCol(j) <= w[i][j] on every pair (dual
//     feasibility), equality on every matched pair (complementary
//     slackness), and the dual sum equals Weight (strong duality).
//
// eps == 0 demands exact arithmetic (integer-weight instances); float
// instances pass a relative tolerance.
func checkCertificate(t *testing.T, name string, w [][]float64, ap *AssignmentProblem, eps float64) {
	t.Helper()
	n := len(w)

	seen := make([]bool, n)
	for i := range n {
		j := ap.Sol(i)
		if j < 0 || j >= n {
			t.Fatalf("%s: Sol(%d) = %d, out of range", name, i, j)
		}
		if seen[j] {
			t.Fatalf("%s: column %d matched twice", name, j)
		}
		seen[j] = true
		if ap.RowOf(j) != i {
			t.Fatalf("%s: RowOf(%d) = %d, want %d (Sol's inverse)", name, j, ap.RowOf(j), i)
		}
	}

	var total float64
	for i := range n {
		total += w[i][ap.Sol(i)]
	}
	if d := math.Abs(total - ap.Weight()); d > eps {
		t.Fatalf("%s: Weight = %v, matched entries sum to %v", name, ap.Weight(), total)
	}

	for i := range n {
		for j := range n {
			u, v := ap.DualRow(i), ap.DualCol(j)
			if u+v > w[i][j]+eps {
				t.Fatalf("%s: dual infeasible at %d-%d: %v + %v > %v", name, i, j, u, v, w[i][j])
			}
			if j == ap.Sol(i) && math.Abs(u+v-w[i][j]) > eps {
				t.Fatalf("%s: matched pair %d-%d not tight: %v + %v != %v", name, i, j, u, v, w[i][j])
			}
		}
	}
	var dualSum float64
	for i := range n {
		dualSum += ap.DualRow(i) + ap.DualCol(i)
	}
	if d := math.Abs(dualSum - ap.Weight()); d > eps {
		t.Fatalf("%s: dual sum %v != Weight %v (strong duality)", name, dualSum, ap.Weight())
	}
}

// bruteForce enumerates every column permutation (small n only) and
// returns the minimum total — the oracle that cannot be wrong.
func bruteForce(w [][]float64) float64 {
	perm := make([]int, len(w))
	for i := range perm {
		perm[i] = i
	}
	best := math.Inf(1)
	for {
		var s float64
		for i, j := range perm {
			s += w[i][j]
		}
		if s < best {
			best = s
		}
		if !nextPerm(perm) {
			return best
		}
	}
}

// nextPerm advances perm to the next lexicographic permutation,
// reporting false after the last.
func nextPerm(perm []int) bool {
	i := len(perm) - 1
	for i > 0 && perm[i-1] >= perm[i] {
		i--
	}
	if i == 0 {
		return false
	}
	j := len(perm) - 1
	for perm[j] <= perm[i-1] {
		j--
	}
	perm[i-1], perm[j] = perm[j], perm[i-1]
	for l, r := i, len(perm)-1; l < r; l, r = l+1, r-1 {
		perm[l], perm[r] = perm[r], perm[l]
	}
	return true
}

// spfaReference is an independent solver used as a cross-check: one
// shortest augmenting path per free row, each found by queue-based
// Bellman-Ford (SPFA) over the residual graph of REAL weights — forward
// arcs at w[i][j], the matched arc reversed at -w.  Structurally unlike
// the shipped algorithm: no duals, no reduced costs, no Dijkstra, and
// it digests negative weights without any shifting.
func spfaReference(w [][]float64) float64 {
	n := len(w)
	matchR := make([]int, n) // row -> column
	matchC := make([]int, n) // column -> row
	for i := range n {
		matchR[i], matchC[i] = unmatched, unmatched
	}
	for s := range n {
		if matchR[s] != unmatched {
			continue
		}
		distRow := make([]float64, n)
		distCol := make([]float64, n)
		prevRow := make([]int, n)
		for i := range n {
			distRow[i] = math.Inf(1)
			distCol[i] = math.Inf(1)
			prevRow[i] = unmatched
		}
		distRow[s] = 0
		inQueue := make([]bool, 2*n) // rows 0..n-1, columns n..2n-1
		queue := []int{s}
		for len(queue) > 0 {
			v := queue[0]
			queue = queue[1:]
			inQueue[v] = false
			if v < n { // row v: forward arcs to every column but its partner
				for j := range n {
					if j == matchR[v] {
						continue
					}
					if nd := distRow[v] + w[v][j]; nd < distCol[j] {
						distCol[j] = nd
						prevRow[j] = v
						if !inQueue[n+j] {
							queue = append(queue, n+j)
							inQueue[n+j] = true
						}
					}
				}
			} else { // column v-n: the reversed arc back to its row
				j := v - n
				if i := matchC[j]; i != unmatched {
					if nd := distCol[j] - w[i][j]; nd < distRow[i] {
						distRow[i] = nd
						if !inQueue[i] {
							queue = append(queue, i)
							inQueue[i] = true
						}
					}
				}
			}
		}
		jT, best := unmatched, math.Inf(1)
		for j := range n {
			if matchC[j] == unmatched && distCol[j] < best {
				jT, best = j, distCol[j]
			}
		}
		if jT == unmatched {
			panic("spfaReference: no augmenting path — impossible on a complete matrix")
		}
		for j := jT; j != unmatched; {
			i := prevRow[j]
			old := matchR[i]
			matchR[i] = j
			matchC[j] = i
			j = old
		}
	}
	var total float64
	for i := range n {
		total += w[i][matchR[i]]
	}
	return total
}

// randomIntMatrix returns an n-by-n matrix of integers drawn uniformly
// from [lo, hi] — integer weights keep the whole solve in exact float64
// arithmetic, so the certificate can be checked with zero tolerance.
func randomIntMatrix(rng *rand.Rand, n int, lo, hi int) [][]float64 {
	w := make([][]float64, n)
	for i := range n {
		w[i] = make([]float64, n)
		for j := range n {
			w[i][j] = float64(lo + rng.IntN(hi-lo+1))
		}
	}
	return w
}

// TestRandomizedModel cross-checks random integer instances (several
// weight ranges, ties included) against the SPFA reference — and the
// permutation brute force wherever it is feasible — always on top of
// the full optimality certificate.  Ties are safe here: the compared
// quantity is the optimal WEIGHT, never the matching itself.
func TestRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 7))
	ranges := [][2]int{{0, 100}, {-50, 50}, {0, 1 << 20}, {0, 1}, {-1000, 1000}}
	for iter := range 300 {
		r := ranges[iter%len(ranges)]
		n := 1 + rng.IntN(20)
		w := randomIntMatrix(rng, n, r[0], r[1])
		ap := NewAssignmentProblem(w)
		checkCertificate(t, "random", w, ap, 0)
		if want := spfaReference(w); ap.Weight() != want {
			t.Fatalf("iter %d (n=%d, range %v): Weight = %v, SPFA reference = %v", iter, n, r, ap.Weight(), want)
		}
		if n <= 7 {
			if want := bruteForce(w); ap.Weight() != want {
				t.Fatalf("iter %d (n=%d): Weight = %v, brute force = %v", iter, n, ap.Weight(), want)
			}
		}
	}
}

// TestFloatWeights: non-integral weights (negatives included) checked
// by certificate and reference with a relative tolerance.
func TestFloatWeights(t *testing.T) {
	rng := rand.New(rand.NewPCG(7, 42))
	for iter := range 200 {
		n := 1 + rng.IntN(12)
		w := make([][]float64, n)
		for i := range n {
			w[i] = make([]float64, n)
			for j := range n {
				w[i][j] = (rng.Float64() - 0.25) * 1e3
			}
		}
		ap := NewAssignmentProblem(w)
		const eps = 1e-8
		checkCertificate(t, "float", w, ap, eps)
		got, want := ap.Weight(), spfaReference(w)
		if math.Abs(got-want) > eps*math.Max(1, math.Abs(want)) {
			t.Fatalf("iter %d (n=%d): Weight = %v, SPFA reference = %v", iter, n, got, want)
		}
	}
}

// checkInternalDuals verifies the solver's own invariants on the
// shifted instance — algs4 asserts exactly these after every augment:
// every reduced cost non-negative (dual feasibility) and exactly zero on
// every matched pair (complementary slackness).  The RAW (unsnapped)
// reduced cost is computed so the epsilon snap cannot hide a violation.
func checkInternalDuals(t *testing.T, ap *AssignmentProblem, round int) {
	t.Helper()
	for i := range ap.n {
		for j := range ap.n {
			rc := (ap.weight[i][j] - ap.minW) + ap.px[i] - ap.py[j]
			magnitude := math.Abs(ap.weight[i][j]) + math.Abs(ap.px[i]) + math.Abs(ap.py[j])
			if rc < -floatingPointEpsilon*magnitude {
				t.Fatalf("round %d: reduced cost %d-%d = %v < 0 (dual infeasible)", round, i, j, rc)
			}
			if ap.xy[i] == j && rc != 0 {
				t.Fatalf("round %d: matched pair %d-%d has reduced cost %v != 0", round, i, j, rc)
			}
		}
	}
}

// TestRoundByRoundDualInvariants runs the solver one round at a time
// (the prepare/augment split) and checks the dual invariants before
// every augment — the white-box counterpart of algs4's constructor
// asserts.  After the final round the public certificate applies.
func TestRoundByRoundDualInvariants(t *testing.T) {
	rng := rand.New(rand.NewPCG(11, 13))
	for iter := range 60 {
		n := 1 + rng.IntN(12)
		w := randomIntMatrix(rng, n, -100, 100)
		ap := prepare(w)
		for round := range n {
			checkInternalDuals(t, ap, round)
			ap.augment()
		}
		checkInternalDuals(t, ap, n)

		for i := range n {
			if ap.xy[i] == unmatched || ap.yx[i] == unmatched {
				t.Fatalf("iter %d: row %d still unmatched after n rounds", iter, i)
			}
		}
		checkCertificate(t, "round-by-round", w, ap, 0)
	}
}

// TestAdversarialShapes: degenerate and extreme instances — massive
// ties, unique optima with unit gaps, magnitude spreads, and integer
// weights near algs4's documented 2^52 exactness ceiling (intermediate
// sums stay integral and far below it).
func TestAdversarialShapes(t *testing.T) {
	shapes := map[string]func(n int) [][]float64{
		"identity-pricing": func(n int) [][]float64 { // unique optimum: the diagonal
			w := make([][]float64, n)
			for i := range w {
				w[i] = make([]float64, n)
				for j := range n {
					if i == j {
						w[i][j] = 0
					} else {
						w[i][j] = 1
					}
				}
			}
			return w
		},
		"anti-diagonal": func(n int) [][]float64 { // unique optimum: the anti-diagonal
			w := make([][]float64, n)
			for i := range w {
				w[i] = make([]float64, n)
				for j := range n {
					if i+j == n-1 {
						w[i][j] = 0
					} else {
						w[i][j] = 1
					}
				}
			}
			return w
		},
		"constant": func(n int) [][]float64 { // every assignment optimal
			w := make([][]float64, n)
			for i := range w {
				w[i] = make([]float64, n)
				for j := range n {
					w[i][j] = 7
				}
			}
			return w
		},
		"identical-rows": func(n int) [][]float64 { // n! optima
			w := make([][]float64, n)
			for i := range w {
				w[i] = make([]float64, n)
				for j := range n {
					w[i][j] = float64(j + 1)
				}
			}
			return w
		},
		"magnitudes": func(n int) [][]float64 { // entries spread over 12 decades
			w := make([][]float64, n)
			for i := range w {
				w[i] = make([]float64, n)
				for j := range n {
					w[i][j] = math.Pow(10, float64((i+3*j)%12))
				}
			}
			return w
		},
		"big-integers": func(n int) [][]float64 { // near the 2^52 exactness ceiling
			w := make([][]float64, n)
			for i := range w {
				w[i] = make([]float64, n)
				for j := range n {
					w[i][j] = 1e12 + float64((i*31+j*17)%1000)
				}
			}
			return w
		},
	}
	for name, gen := range shapes {
		for _, n := range []int{1, 2, 3, 7, 15, 40} {
			w := gen(n)
			ap := NewAssignmentProblem(w)
			checkCertificate(t, name, w, ap, 0)
			if want := spfaReference(w); ap.Weight() != want {
				t.Fatalf("%s n=%d: Weight = %v, SPFA reference = %v", name, n, ap.Weight(), want)
			}
		}
	}
}

// TestShapeOptima pins the known optima of the shapes with a unique
// answer (the certificate proves optimality; this pins WHICH optimum).
func TestShapeOptima(t *testing.T) {
	n := 8
	w := make([][]float64, n)
	for i := range w {
		w[i] = make([]float64, n)
		for j := range n {
			switch {
			case i == j:
				w[i][j] = 0
			case i+j == n-1:
				w[i][j] = 0.5
			default:
				w[i][j] = 2
			}
		}
	}
	ap := NewAssignmentProblem(w) // every diagonal entry beats the anti-diagonal
	if ap.Weight() != 0 {
		t.Fatalf("Weight = %v, want 0 (the diagonal)", ap.Weight())
	}
	for i := range n {
		if ap.Sol(i) != i {
			t.Fatalf("Sol(%d) = %d, want %d", i, ap.Sol(i), i)
		}
	}
}

// TestLargerInstances: sizes past any teaching example, certificate
// plus the SPFA reference (exact integer arithmetic throughout).
func TestLargerInstances(t *testing.T) {
	rng := rand.New(rand.NewPCG(2026, 10))
	for _, n := range []int{60, 150, 300} {
		w := randomIntMatrix(rng, n, -10000, 10000)
		ap := NewAssignmentProblem(w)
		checkCertificate(t, "large", w, ap, 0)
		if want := spfaReference(w); ap.Weight() != want {
			t.Fatalf("n=%d: Weight = %v, SPFA reference = %v", n, ap.Weight(), want)
		}
	}
}
