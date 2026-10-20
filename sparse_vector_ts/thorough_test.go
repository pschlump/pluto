package sparse_vector_ts

/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

import (
	"math/rand"
	"testing"
)

// TestSparseVectorRandomizedModel runs a fixed-seed property test
// against a plain []float64 reference vector: random puts (zeros
// included, so removals are exercised), then every read and every
// arithmetic operation is checked against the reference.  All values
// are small integers in float64, so every arithmetic result is exact
// and comparisons use ==.
func TestSparseVectorRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewSource(42)) // fixed seed: deterministic run

	for trial := range 300 {
		n := 1 + rng.Intn(20)
		v := NewSparseVector[float64](n)
		ref := make([]float64, n)

		nnz := func() int {
			c := 0
			for _, x := range ref {
				if x != 0 {
					c++
				}
			}
			return c
		}

		// Random puts, including zeros (the canonical removal) and
		// out-of-range indices (which must be reported, not stored).
		for range 4 * n {
			i := rng.Intn(n+2) - 1 // -1..n, so both edges are hit
			w := float64(rng.Intn(19) - 9)
			ok := v.Put(i, w)
			wantOK := i >= 0 && i < n
			if ok != wantOK {
				t.Fatalf("trial %d: Put(%d, %v) = %v, expected %v", trial, i, w, ok, wantOK)
			}
			if wantOK {
				ref[i] = w
			}
		}

		if v.Dimension() != n {
			t.Fatalf("trial %d: Dimension = %d, expected %d", trial, v.Dimension(), n)
		}
		if v.NNz() != nnz() {
			t.Fatalf("trial %d: NNz = %d, reference counts %d", trial, v.NNz(), nnz())
		}
		for i := range n {
			val, ok := v.Get(i)
			if val != ref[i] || ok != (ref[i] != 0) {
				t.Fatalf("trial %d: Get(%d) = %v, %v; reference %v", trial, i, val, ok, ref[i])
			}
		}

		// Sparse · dense against the plain loop.
		wantDot := 0.0
		for i := range n {
			wantDot += ref[i] * float64(i+1)
		}
		y := make([]float64, n)
		for i := range n {
			y[i] = float64(i + 1)
		}
		if got := v.Dot(y); got != wantDot {
			t.Fatalf("trial %d: Dot = %v, reference %v", trial, got, wantDot)
		}

		// Sparse · sparse (a second random vector) against the plain
		// loop, in both operand orders.
		u := NewSparseVector[float64](n)
		uref := make([]float64, n)
		for i := range n {
			uref[i] = float64(rng.Intn(7) - 3)
			u.Put(i, uref[i])
		}
		wantSS := 0.0
		for i := range n {
			wantSS += ref[i] * uref[i]
		}
		if got := v.DotSparse(u); got != wantSS {
			t.Fatalf("trial %d: DotSparse = %v, reference %v", trial, got, wantSS)
		}
		if got := u.DotSparse(v); got != wantSS {
			t.Fatalf("trial %d: DotSparse reversed = %v, reference %v", trial, got, wantSS)
		}

		// Scale and Plus against the reference; the operands must not
		// change.
		alpha := float64(rng.Intn(5) - 2)
		s := v.Scale(alpha)
		for i := range n {
			if val, _ := s.Get(i); val != alpha*ref[i] {
				t.Fatalf("trial %d: Scale(%v) Get(%d) = %v, reference %v", trial, alpha, i, val, alpha*ref[i])
			}
		}
		p := v.Plus(u)
		for i := range n {
			if val, _ := p.Get(i); val != ref[i]+uref[i] {
				t.Fatalf("trial %d: Plus Get(%d) = %v, reference %v", trial, i, val, ref[i]+uref[i])
			}
		}
		for i := range n {
			if val, _ := v.Get(i); val != ref[i] {
				t.Fatalf("trial %d: operand v changed at %d", trial, i)
			}
		}

		// Axpy in place against the reference.
		beta := float64(rng.Intn(5) - 2)
		v.Axpy(beta, u)
		for i := range n {
			ref[i] += beta * uref[i]
		}
		for i := range n {
			if val, _ := v.Get(i); val != ref[i] {
				t.Fatalf("trial %d: Axpy(%v) Get(%d) = %v, reference %v", trial, beta, i, val, ref[i])
			}
		}
		if v.NNz() != nnz() {
			t.Fatalf("trial %d: NNz after Axpy = %d, reference counts %d", trial, v.NNz(), nnz())
		}

		// Each visits exactly the nonzero reference entries, ascending.
		var seen []int
		v.Each(func(i int, value float64) bool {
			seen = append(seen, i)
			if value != ref[i] {
				t.Fatalf("trial %d: Each(%d) value %v, reference %v", trial, i, value, ref[i])
			}
			return false
		})
		if len(seen) != nnz() {
			t.Fatalf("trial %d: Each visited %d entries, reference counts %d", trial, len(seen), nnz())
		}
		for k := 1; k < len(seen); k++ {
			if seen[k-1] >= seen[k] {
				t.Fatalf("trial %d: Each order %v is not ascending", trial, seen)
			}
		}

		// Dense materializes the reference exactly.
		d := v.Dense()
		if len(d) != n {
			t.Fatalf("trial %d: Dense length %d, expected %d", trial, len(d), n)
		}
		for i := range n {
			if d[i] != ref[i] {
				t.Fatalf("trial %d: Dense[%d] = %v, reference %v", trial, i, d[i], ref[i])
			}
		}
	}
}
