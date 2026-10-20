package sparse_vector

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"math/rand"
	"testing"
)

// benchVector builds a vector of dimension d with nnz nonzero integer
// entries at pseudo-random indices (fixed seed — no global RNG).
func benchVector(d, nnz int) *SparseVector[float64] {
	rng := rand.New(rand.NewSource(42))
	v := NewSparseVector[float64](d)
	for i := 0; i < nnz; i++ {
		v.Put(rng.Intn(d), float64(i%97+1))
	}
	return v
}

// BenchmarkPut measures a stream of in-range puts (hits overwrite,
// misses add) on a vector that already holds 1000 entries.
func BenchmarkPut(b *testing.B) {
	const d, nnz = 1 << 20, 1000
	v := benchVector(d, nnz)
	rng := rand.New(rand.NewSource(7))
	idx := make([]int, b.N)
	for i := range idx {
		idx[i] = rng.Intn(d)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v.Put(idx[i], 1.5)
	}
}

// BenchmarkGet measures a hit/miss mix of point reads.
func BenchmarkGet(b *testing.B) {
	const d, nnz = 1 << 20, 1000
	v := benchVector(d, nnz)
	keys := make([]int, 0, nnz)
	v.Each(func(i int, _ float64) bool { keys = append(keys, i); return false })
	rng := rand.New(rand.NewSource(7))
	idx := make([]int, b.N)
	for i := range idx {
		if i%2 == 0 && len(keys) > 0 {
			idx[i] = keys[rng.Intn(len(keys))]
		} else {
			idx[i] = rng.Intn(d)
		}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v.Get(idx[i])
	}
}

// BenchmarkDotDense is the sparsity payoff: a 1000-entry vector dotted
// with a dense million-element operand costs 1000 multiply-adds.
func BenchmarkDotDense(b *testing.B) {
	const d, nnz = 1 << 20, 1000
	v := benchVector(d, nnz)
	y := make([]float64, d)
	for i := range y {
		y[i] = float64(i%13 + 1)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v.Dot(y)
	}
}

// BenchmarkDenseControl is the same product computed by the plain
// dense loop — the O(d) work the sparse vector avoids.
func BenchmarkDenseControl(b *testing.B) {
	const d = 1 << 20
	y := make([]float64, d)
	for i := range y {
		y[i] = float64(i%13 + 1)
	}
	x := make([]float64, d)
	x[3] = 1
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sum := 0.0
		for j := range x {
			sum += x[j] * y[j]
		}
		_ = sum
	}
}

// BenchmarkDotSparse measures the sparse·sparse product of two
// 1000-entry vectors of dimension 1<<20.
func BenchmarkDotSparse(b *testing.B) {
	const d, nnz = 1 << 20, 1000
	v := benchVector(d, nnz)
	w := benchVector(d, nnz)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v.DotSparse(w)
	}
}

// BenchmarkAxpy measures the in-place row operation on two 1000-entry
// vectors.
func BenchmarkAxpy(b *testing.B) {
	const d, nnz = 1 << 20, 1000
	y := benchVector(d, nnz)
	x := benchVector(d, nnz)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		y.Axpy(1.25, x)
	}
}
