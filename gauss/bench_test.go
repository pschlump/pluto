package gauss

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"testing"
)

// benchSystem returns a diagonally dominant n×n system with the exact
// solution x = 1 (fixed seed: deterministic setup).
func benchSystem(n int) ([][]float64, []float64) {
	rng := newRand()
	A := dominant(rng, n)
	b := make([]float64, n)
	for i := range b {
		b[i] = 1
		for j := range A[i] {
			b[i] += A[i][j]
		}
	}
	return A, b
}

// Gaussian vs Gauss-Jordan: both O(n³), but Jordan eliminates above
// and below and carries an identity block (2n+1 columns instead of
// n+1), and its pivot loop skips neither the j == q column nor the
// i == p row explicitly — measured (M4 Max): Gaussian 213µs at n=100
// and 6.0ms at n=300; Gauss-Jordan 1.36ms at n=100 and 34.7ms at
// n=300 (~6×) — both tracking n³ (28× for 3× n).  Gaussian is the
// solver of choice unless the Dual certificate is needed.
func BenchmarkGaussianElimination100(b *testing.B) { benchGaussAt(b, 100) }
func BenchmarkGaussianElimination300(b *testing.B) { benchGaussAt(b, 300) }

func benchGaussAt(b *testing.B, n int) {
	A, rhs := benchSystem(n)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		NewGaussianElimination(A, rhs).Primal()
	}
}

func BenchmarkGaussJordan100(b *testing.B) { benchJordanAt(b, 100) }
func BenchmarkGaussJordan300(b *testing.B) { benchJordanAt(b, 300) }

func benchJordanAt(b *testing.B, n int) {
	A, rhs := benchSystem(n)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		NewGaussJordanElimination(A, rhs).Primal()
	}
}
