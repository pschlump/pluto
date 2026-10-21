package gauss

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"math"
	"math/rand"
	"testing"
)

// tol is the residual/certificate tolerance.  Test systems are integer
// valued (n <= 9, entries <= 220), so elimination noise stays orders
// of magnitude below this.
const tol = 1e-6

// residualScale returns max(1, max|b_i|) — the residual comparison
// scale.
func residualScale(b []float64) float64 {
	s := 1.0
	for _, v := range b {
		if math.Abs(v) > s {
			s = math.Abs(v)
		}
	}
	return s
}

// checkResidual fails unless |A·x − b|_inf <= tol·max(1, |b|_inf) — the
// algs4 certifySolution "Ax = b" half.
func checkResidual(t *testing.T, label string, A [][]float64, b, x []float64) {
	t.Helper()
	s := tol * residualScale(b)
	for i, row := range A {
		sum := 0.0
		for j, v := range row {
			sum += v * x[j]
		}
		if math.Abs(sum-b[i]) > s {
			t.Fatalf("%s: row %d residual %.3g, want <= %.3g", label, i, math.Abs(sum-b[i]), s)
		}
	}
}

// checkDual fails unless y is a valid certificate of infeasibility:
// yA = 0 and yb != 0 — the algs4 certifySolution "yA = 0, yb != 0"
// half.
func checkDual(t *testing.T, label string, A [][]float64, b, y []float64) {
	t.Helper()
	s := tol * residualScale(b)
	for j := range A[0] {
		sum := 0.0
		for i, row := range A {
			sum += y[i] * row[j]
		}
		if math.Abs(sum) > s {
			t.Fatalf("%s: (yA)[%d] = %.3g, want <= %.3g", label, j, sum, s)
		}
	}
	yb := 0.0
	for i, v := range b {
		yb += y[i] * v
	}
	if math.Abs(yb) <= epsilon {
		t.Fatalf("%s: y·b = %.3g, want > %g", label, yb, epsilon)
	}
}

// checkVec fails unless every component matches within tol.
func checkVec(t *testing.T, label string, got, want []float64) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: length %d, want %d", label, len(got), len(want))
	}
	for i := range want {
		if math.Abs(got[i]-want[i]) > tol {
			t.Fatalf("%s: component %d is %.6g, want %.6g", label, i, got[i], want[i])
		}
	}
}

// dominant returns a strictly row-diagonally-dominant integer matrix —
// guaranteed nonsingular, and partial pivoting picks its diagonal.
func dominant(rng *rand.Rand, n int) [][]float64 {
	A := make([][]float64, n)
	for i := range A {
		A[i] = make([]float64, n)
		sum := 0
		for j := range A[i] {
			if j != i {
				v := rng.Intn(19) - 9
				A[i][j] = float64(v)
				if v < 0 {
					sum -= v
				} else {
					sum += v
				}
			}
		}
		A[i][i] = float64(sum + 1 + rng.Intn(3))
	}
	return A
}

// mulVec computes A·x (exact for the integer test inputs).
func mulVec(A [][]float64, x []float64) []float64 {
	b := make([]float64, len(A))
	for i, row := range A {
		sum := 0.0
		for j, v := range row {
			sum += v * x[j]
		}
		b[i] = sum
	}
	return b
}

func TestKnownAnswers(t *testing.T) {
	cases := []struct {
		name     string
		A        [][]float64
		b        []float64
		solvable bool
		want     []float64 // the unique solution, when it is unique
	}{
		{"1x1", [][]float64{{5}}, []float64{10}, true, []float64{2}},
		{"2x2", [][]float64{{2, 1}, {1, 3}}, []float64{5, 10}, true, []float64{1, 3}},
		{"3x3 identity-ish", [][]float64{{2, 0, 0}, {0, 3, 0}, {0, 0, 4}}, []float64{2, 6, 12}, true, []float64{1, 2, 3}},
		{"overdetermined consistent", [][]float64{{1, 0}, {0, 1}, {1, 1}}, []float64{2, 3, 5}, true, []float64{2, 3}},
		{"overdetermined inconsistent", [][]float64{{1, 0}, {0, 1}, {1, 1}}, []float64{2, 3, 6}, false, nil},
		{"singular infinitely many", [][]float64{{1, 2}, {2, 4}}, []float64{1, 2}, true, nil}, // particular solution
		{"singular no solution", [][]float64{{1, 2}, {2, 4}}, []float64{1, 3}, false, nil},
	}
	for _, c := range cases {
		g := NewGaussianElimination(c.A, c.b)
		x, ok := g.Primal()
		if ok != c.solvable {
			t.Errorf("%s: Gauss Primal ok = %v, want %v", c.name, ok, c.solvable)
			continue
		}
		if !ok {
			if g.IsFeasible() {
				t.Errorf("%s: IsFeasible true after Primal false", c.name)
			}
			continue
		}
		checkResidual(t, c.name+" (Gauss)", c.A, c.b, x)
		if c.want != nil {
			checkVec(t, c.name+" (Gauss)", x, c.want)
		}

		// Gauss-Jordan agrees on square systems.
		if len(c.A) == len(c.b) && len(c.A) > 0 && len(c.A[0]) == len(c.A) {
			gj := NewGaussJordanElimination(c.A, c.b)
			xj, okj := gj.Primal()
			if okj != c.solvable {
				t.Errorf("%s: GaussJordan Primal ok = %v, want %v", c.name, okj, c.solvable)
				continue
			}
			if okj {
				checkResidual(t, c.name+" (Jordan)", c.A, c.b, xj)
				if c.want != nil {
					checkVec(t, c.name+" (Jordan)", xj, c.want)
				}
			}
		}
	}

	// The empty system is vacuously solvable, with the empty solution.
	g := NewGaussianElimination(nil, nil)
	if x, ok := g.Primal(); !ok || len(x) != 0 {
		t.Errorf("empty system: Primal = (%v, %v), want ([], true)", x, ok)
	}
	gj := NewGaussJordanElimination(nil, nil)
	if x, ok := gj.Primal(); !ok || len(x) != 0 {
		t.Errorf("empty system (Jordan): Primal = (%v, %v), want ([], true)", x, ok)
	}
}

// TestInfinitelyManyParticular pins the free-variable-zero contract:
// for [[1,2],[2,4]]x = [1,2] the returned particular solution sets the
// free variable x1 to zero.
func TestInfinitelyManyParticular(t *testing.T) {
	A := [][]float64{{1, 2}, {2, 4}}
	b := []float64{1, 2}

	g := NewGaussianElimination(A, b)
	x, ok := g.Primal()
	if !ok {
		t.Fatal("infinitely-many system: Gauss Primal ok = false, want true")
	}
	if x[1] != 0 {
		t.Errorf("free variable x[1] = %v, want 0", x[1])
	}
	checkResidual(t, "infinitely-many (Gauss)", A, b, x)

	gj := NewGaussJordanElimination(A, b)
	xj, okj := gj.Primal()
	if !okj {
		t.Fatal("infinitely-many system: GaussJordan Primal ok = false, want true")
	}
	if xj[1] != 0 {
		t.Errorf("free variable x[1] = %v, want 0", xj[1])
	}
	checkResidual(t, "infinitely-many (Jordan)", A, b, xj)
	if gj.IsNonsingular() {
		t.Error("duplicate-row system: IsNonsingular = true, want false")
	}
	if _, ok := gj.Dual(); ok {
		t.Error("feasible system: Dual ok = true, want false")
	}
}

// TestDualCertificate verifies the certificate of infeasibility for
// [[1,2],[2,4]]x = [1,3] generically (yA = 0, yb != 0) — which y the
// pivoting picks depends on the row swaps, so the vector itself is not
// pinned here (the example pins the actual one).
func TestDualCertificate(t *testing.T) {
	A := [][]float64{{1, 2}, {2, 4}}
	b := []float64{1, 3}

	gj := NewGaussJordanElimination(A, b)
	if gj.IsFeasible() {
		t.Fatal("inconsistent system: IsFeasible = true, want false")
	}
	y, ok := gj.Dual()
	if !ok {
		t.Fatal("inconsistent system: Dual ok = false, want true")
	}
	checkDual(t, "certificate", A, b, y)
}

// TestUnderdeterminedCorner pins the documented reference corner:
// variables at columns m..n-1 never pivot and stay zero, so a solvable
// underdetermined system that needs one of them nonzero is reported
// infeasible.  The square variant (same leading zero column) is NOT
// affected — back substitution balances row 0 against the solved x[1].
func TestUnderdeterminedCorner(t *testing.T) {
	// m=1, n=2: x = (0, 2) solves it, but x[1] is beyond the row count.
	g := NewGaussianElimination([][]float64{{0, 5}}, []float64{10})
	if _, ok := g.Primal(); ok {
		t.Error("underdetermined corner system: Gauss Primal ok = true — the documented corner flipped")
	}

	// Square, same leading zero column: solvable, x = (0, 2).
	A := [][]float64{{0, 5}, {0, 3}}
	b := []float64{10, 6}
	g = NewGaussianElimination(A, b)
	x, ok := g.Primal()
	if !ok {
		t.Fatal("square degenerate system: Gauss Primal ok = false, want true")
	}
	checkResidual(t, "degenerate square (Gauss)", A, b, x)
	checkVec(t, "degenerate square (Gauss)", x, []float64{0, 2})

	gj := NewGaussJordanElimination(A, b)
	xj, okj := gj.Primal()
	if !okj {
		t.Fatal("square degenerate system: GaussJordan Primal ok = false, want true")
	}
	checkResidual(t, "degenerate square (Jordan)", A, b, xj)
	checkVec(t, "degenerate square (Jordan)", xj, []float64{0, 2})
}

// TestRandomizedExact solves generated integer systems whose exact
// integer solution is known by construction: diagonally dominant
// (nonsingular, unique solution), and consistent overdetermined
// systems built by appending exact row combinations.
func TestRandomizedExact(t *testing.T) {
	rng := newRand()
	for step := 0; step < 300; step++ {
		n := 1 + rng.Intn(9)
		A := dominant(rng, n)
		xStar := make([]float64, n)
		for i := range xStar {
			xStar[i] = float64(rng.Intn(11) - 5)
		}
		b := mulVec(A, xStar)

		// Square: both solvers, unique solution, cross-family
		// agreement.
		xg, okg := NewGaussianElimination(A, b).Primal()
		if !okg {
			t.Fatalf("step %d: diagonally dominant system reported infeasible", step)
		}
		checkVec(t, "exact (Gauss)", xg, xStar)
		gj := NewGaussJordanElimination(A, b)
		if !gj.IsNonsingular() {
			t.Fatalf("step %d: diagonally dominant system reported singular", step)
		}
		xj, _ := gj.Primal()
		checkVec(t, "exact (Jordan)", xj, xStar)

		// Consistent extra rows: an exact combination of two existing
		// rows (integer coefficients, so b stays exact).
		if n >= 2 {
			extra := make([]float64, n)
			for j := range extra {
				extra[j] = 3*A[0][j] + 2*A[1][j]
			}
			A2 := append(append([][]float64{}, A...), extra)
			b2 := append(append([]float64{}, b...), 3*b[0]+2*b[1])
			xr, okr := NewGaussianElimination(A2, b2).Primal()
			if !okr {
				t.Fatalf("step %d: consistent overdetermined system reported infeasible", step)
			}
			checkVec(t, "exact (overdetermined)", xr, xStar)
		}
	}
}

// TestRandomizedCertificates builds singular square systems two ways —
// consistent (duplicate row, matching rhs) and inconsistent (duplicate
// row, perturbed rhs) — and verifies by certificate, so no truth is
// assumed: feasible answers must satisfy A·x = b; infeasible answers
// must carry a dual with yA = 0, yb != 0.
func TestRandomizedCertificates(t *testing.T) {
	rng := newRand()
	for step := 0; step < 200; step++ {
		n := 2 + rng.Intn(7)
		A := dominant(rng, n)
		xStar := make([]float64, n)
		for i := range xStar {
			xStar[i] = float64(rng.Intn(11) - 5)
		}
		copy(A[n-1], A[0]) // duplicate row 0: rank n-1

		b := mulVec(A, xStar) // consistent
		gj := NewGaussJordanElimination(A, b)
		if gj.IsNonsingular() {
			t.Fatalf("step %d: duplicate-row system reported nonsingular", step)
		}
		if _, ok := gj.Dual(); ok {
			t.Fatalf("step %d: consistent system has a dual certificate", step)
		}
		x, ok := gj.Primal()
		if !ok {
			t.Fatalf("step %d: consistent duplicate-row system reported infeasible", step)
		}
		checkResidual(t, "singular-consistent (Jordan)", A, b, x)

		b[n-1]++ // now 0 = 1 on the zeroed row: inconsistent
		gj = NewGaussJordanElimination(A, b)
		if x, ok := gj.Primal(); ok {
			checkResidual(t, "singular-inconsistent (Jordan)", A, b, x)
			t.Fatalf("step %d: inconsistent system reported feasible", step)
		}
		y, ok := gj.Dual()
		if !ok {
			t.Fatalf("step %d: inconsistent system has no dual certificate", step)
		}
		checkDual(t, "inconsistent (Jordan)", A, b, y)

		// Gaussian agrees on inconsistency (the zeroed row is caught
		// by back substitution; the leading-column corner cannot flip
		// this direction).
		if _, ok := NewGaussianElimination(A, b).Primal(); ok {
			t.Fatalf("step %d: Gaussian reported an inconsistent system feasible", step)
		}
	}
}

func TestPanics(t *testing.T) {
	sq := [][]float64{{1, 0}, {0, 1}}
	ragged := [][]float64{{1, 2, 3}, {4, 5}}
	nan := [][]float64{{1, math.NaN()}}

	expectPanic(t, "NewGaussianElimination called with A of 2 rows and b of length 3", func() {
		NewGaussianElimination(sq, []float64{1, 2, 3})
	})
	expectPanic(t, "row 1 of length 2 but row 0 of length 3", func() {
		NewGaussianElimination(ragged, []float64{1, 2})
	})
	expectPanic(t, "NewGaussianElimination found a non-finite entry (NaN or ±Inf) at A[0][1]", func() {
		NewGaussianElimination(nan, []float64{1})
	})
	expectPanic(t, "NewGaussianElimination found a non-finite entry (NaN or ±Inf) at b[1]", func() {
		NewGaussianElimination(sq, []float64{1, math.Inf(1)})
	})
	expectPanic(t, "NewGaussJordanElimination called with A of 2 rows and b of length 1", func() {
		NewGaussJordanElimination(sq, []float64{1})
	})
	expectPanic(t, "row 0 of length 3 but 2 columns elsewhere", func() {
		NewGaussJordanElimination(ragged, []float64{1, 2})
	})
	expectPanic(t, "NewGaussJordanElimination found a non-finite entry (NaN or ±Inf) at b[0]", func() {
		NewGaussJordanElimination(sq, []float64{math.Inf(-1), 1})
	})
}

// TestNilTolerated: nil receivers and zero values behave as empty
// systems — solvable, with the empty solution.
func TestNilTolerated(t *testing.T) {
	var g *GaussianElimination
	if x, ok := g.Primal(); !ok || len(x) != 0 {
		t.Errorf("nil GaussianElimination: Primal = (%v, %v), want ([], true)", x, ok)
	}
	if !g.IsFeasible() {
		t.Error("nil GaussianElimination: IsFeasible = false, want true")
	}

	var gj *GaussJordanElimination
	if x, ok := gj.Primal(); !ok || len(x) != 0 {
		t.Errorf("nil GaussJordanElimination: Primal = (%v, %v), want ([], true)", x, ok)
	}
	if !gj.IsFeasible() || !gj.IsNonsingular() {
		t.Error("nil GaussJordanElimination: not feasible/nonsingular, want both true")
	}
	if _, ok := gj.Dual(); ok {
		t.Error("nil GaussJordanElimination: Dual ok = true, want false")
	}

	var zero GaussJordanElimination
	if !zero.IsFeasible() {
		t.Error("zero-value GaussJordanElimination: IsFeasible = false, want true")
	}
}

// TestInputNotMutated: both constructors copy — the caller's A and b
// are never modified (the elimination is destructive on the copy).
func TestInputNotMutated(t *testing.T) {
	A := [][]float64{{2, 1}, {1, 3}, {3, 4}}
	b := []float64{5, 10, 17}
	Abak := [][]float64{{2, 1}, {1, 3}, {3, 4}}
	bbak := []float64{5, 10, 17}

	NewGaussianElimination(A, b).Primal()
	NewGaussJordanElimination(A[:2], b[:2]).Primal()
	for i := range A {
		for j := range A[i] {
			if A[i][j] != Abak[i][j] {
				t.Fatalf("A[%d][%d] mutated: %v, want %v", i, j, A[i][j], Abak[i][j])
			}
		}
	}
	for i := range b {
		if b[i] != bbak[i] {
			t.Fatalf("b[%d] mutated: %v, want %v", i, b[i], bbak[i])
		}
	}
}
