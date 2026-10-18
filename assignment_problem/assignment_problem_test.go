/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package assignment_problem

import (
	"math"
	"testing"
)

func expectPanic(t *testing.T, what string, fn func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatalf("%s did not panic", what)
		}
	}()
	fn()
}

// classic4x4 is the textbook assignment instance (its unique optimum,
// total 13, pairs column-permutation [1, 0, 2, 3] — verified against
// the enumerate-all-permutations oracle).
var classic4x4 = [][]float64{
	{9, 2, 7, 8},
	{6, 4, 3, 7},
	{5, 8, 1, 8},
	{7, 6, 9, 4},
}

func TestClassicExample(t *testing.T) {
	ap := NewAssignmentProblem(classic4x4)
	if got, want := ap.Weight(), 13.0; got != want {
		t.Fatalf("Weight = %v, want %v", got, want)
	}
	wantSol := []int{1, 0, 2, 3} // the unique optimum
	for i, w := range wantSol {
		if got := ap.Sol(i); got != w {
			t.Fatalf("Sol(%d) = %d, want %d", i, got, w)
		}
	}
	for j, w := range wantSol {
		if got := ap.RowOf(j); got != w {
			t.Fatalf("RowOf(%d) = %d, want %d (Sol's inverse)", j, got, w)
		}
	}

	// Integer weights keep the whole solve in exact float64 arithmetic,
	// so the certificate identities hold with no tolerance at all.
	var dualSum float64
	for i := range 4 {
		dualSum += ap.DualRow(i)
	}
	for j := range 4 {
		dualSum += ap.DualCol(j)
	}
	if dualSum != ap.Weight() {
		t.Fatalf("dual sum = %v, want Weight %v", dualSum, ap.Weight())
	}
	for i := range 4 {
		for j := range 4 {
			u, v := ap.DualRow(i), ap.DualCol(j)
			if u+v > classic4x4[i][j] {
				t.Fatalf("dual infeasible at %d-%d: %v+%v > %v", i, j, u, v, classic4x4[i][j])
			}
			if j == ap.Sol(i) && u+v != classic4x4[i][j] {
				t.Fatalf("matched pair %d-%d not tight: %v+%v != %v", i, j, u, v, classic4x4[i][j])
			}
		}
	}
}

// TestNegativeWeights: negative entries are legal input (the solver
// shifts by the minimum weight internally).  This instance's unique
// optimum, total -6, is the diagonal [0, 1, 2].
func TestNegativeWeights(t *testing.T) {
	w := [][]float64{
		{-1, 2, 4},
		{3, -2, 0},
		{5, 1, -3},
	}
	ap := NewAssignmentProblem(w)
	if got, want := ap.Weight(), -6.0; got != want {
		t.Fatalf("Weight = %v, want %v", got, want)
	}
	for i := range 3 {
		if got := ap.Sol(i); got != i {
			t.Fatalf("Sol(%d) = %d, want %d", i, got, i)
		}
	}
	var dualSum float64
	for i := range 3 {
		dualSum += ap.DualRow(i) + ap.DualCol(i)
	}
	if dualSum != -6.0 {
		t.Fatalf("dual sum = %v, want -6", dualSum)
	}
}

// TestAllNegativeWeights: every entry negative.
func TestAllNegativeWeights(t *testing.T) {
	w := [][]float64{
		{-9, -2},
		{-6, -4},
	}
	ap := NewAssignmentProblem(w)
	if got, want := ap.Weight(), -13.0; got != want { // -9 + -4, the diagonal
		t.Fatalf("Weight = %v, want %v", got, want)
	}
}

// TestTies: matrices whose every assignment is optimal — the reported
// matching is one of them, and the weight is the certain value.
func TestTies(t *testing.T) {
	n := 5
	constant := make([][]float64, n)
	for i := range constant {
		constant[i] = make([]float64, n)
		for j := range n {
			constant[i][j] = 7
		}
	}
	ap := NewAssignmentProblem(constant)
	if got, want := ap.Weight(), 35.0; got != want {
		t.Fatalf("constant matrix: Weight = %v, want %v", got, want)
	}

	// Identity pricing: diagonal 0, off-diagonal 1 — the diagonal is the
	// unique optimum.
	ident := make([][]float64, n)
	for i := range ident {
		ident[i] = make([]float64, n)
		for j := range n {
			if i == j {
				ident[i][j] = 0
			} else {
				ident[i][j] = 1
			}
		}
	}
	ap = NewAssignmentProblem(ident)
	if got, want := ap.Weight(), 0.0; got != want {
		t.Fatalf("identity pricing: Weight = %v, want %v", got, want)
	}
	for i := range n {
		if ap.Sol(i) != i || ap.RowOf(i) != i {
			t.Fatalf("identity pricing: Sol(%d) = %d, RowOf(%d) = %d, want the diagonal", i, ap.Sol(i), i, ap.RowOf(i))
		}
	}

	// Identical rows: every column-permutation costs the same.
	rows := make([][]float64, 4)
	for i := range rows {
		rows[i] = []float64{10, 20, 30, 40}
	}
	if got, want := NewAssignmentProblem(rows).Weight(), 100.0; got != want {
		t.Fatalf("identical rows: Weight = %v, want %v", got, want)
	}
}

func TestSingleton(t *testing.T) {
	ap := NewAssignmentProblem([][]float64{{-42.5}})
	if got, want := ap.Weight(), -42.5; got != want {
		t.Fatalf("Weight = %v, want %v", got, want)
	}
	if ap.Sol(0) != 0 || ap.RowOf(0) != 0 {
		t.Fatalf("singleton: Sol(0) = %d, RowOf(0) = %d, want 0", ap.Sol(0), ap.RowOf(0))
	}
	if ap.DualRow(0)+ap.DualCol(0) != -42.5 {
		t.Fatalf("singleton duals %v+%v not tight", ap.DualRow(0), ap.DualCol(0))
	}
}

func TestNilAndEmptyTolerated(t *testing.T) {
	for name, ap := range map[string]*AssignmentProblem{
		"nil matrix":   NewAssignmentProblem(nil),
		"empty matrix": NewAssignmentProblem([][]float64{}),
		"zero value":   {},
		"nil receiver": nil,
	} {
		if got := ap.Sol(0); got != -1 {
			t.Errorf("%s: Sol(0) = %d, want -1", name, got)
		}
		if got := ap.RowOf(0); got != -1 {
			t.Errorf("%s: RowOf(0) = %d, want -1", name, got)
		}
		if got := ap.Weight(); got != 0 {
			t.Errorf("%s: Weight = %v, want 0", name, got)
		}
		if got := ap.DualRow(0); got != 0 {
			t.Errorf("%s: DualRow(0) = %v, want 0", name, got)
		}
		if got := ap.DualCol(0); got != 0 {
			t.Errorf("%s: DualCol(0) = %v, want 0", name, got)
		}
	}

	// Out-of-range indices on a real result report the same as nil.
	ap := NewAssignmentProblem(classic4x4)
	for _, i := range []int{-1, 4, 100} {
		if got := ap.Sol(i); got != -1 {
			t.Errorf("Sol(%d) = %d, want -1", i, got)
		}
		if got := ap.DualRow(i); got != 0 {
			t.Errorf("DualRow(%d) = %v, want 0", i, got)
		}
		if got := ap.RowOf(i); got != -1 {
			t.Errorf("RowOf(%d) = %d, want -1", i, got)
		}
		if got := ap.DualCol(i); got != 0 {
			t.Errorf("DualCol(%d) = %v, want 0", i, got)
		}
	}
}

func TestConstructorPanics(t *testing.T) {
	expectPanic(t, "ragged matrix", func() {
		NewAssignmentProblem([][]float64{{1, 2}, {3}})
	})
	expectPanic(t, "NaN entry", func() {
		NewAssignmentProblem([][]float64{{1, math.NaN()}, {2, 3}})
	})
	expectPanic(t, "+Inf entry", func() {
		NewAssignmentProblem([][]float64{{1, math.Inf(1)}, {2, 3}})
	})
	expectPanic(t, "-Inf entry", func() {
		NewAssignmentProblem([][]float64{{1, 2}, {math.Inf(-1), 3}})
	})
}

// TestMatrixSnapshot: the solve works on a copy — mutating the caller's
// matrix afterwards changes nothing.
func TestMatrixSnapshot(t *testing.T) {
	w := [][]float64{
		{1, 2},
		{2, 1},
	}
	ap := NewAssignmentProblem(w)
	w[0][0] = 100 // would flip the optimum to the anti-diagonal
	if got, want := ap.Weight(), 2.0; got != want {
		t.Fatalf("Weight = %v, want %v (the matrix is snapshotted)", got, want)
	}
	if ap.Sol(0) != 0 || ap.Sol(1) != 1 {
		t.Fatalf("Sol = [%d %d], want the diagonal", ap.Sol(0), ap.Sol(1))
	}
}
