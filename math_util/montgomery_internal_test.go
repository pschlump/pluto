/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package math_util

import (
	"math"
	"math/bits"
	"math/rand/v2"
	"testing"
)

// TestMontgomeryMulReduced pins the two montgomeryMul invariants IsPrime's
// equality tests rely on: the result is fully reduced (< modulus, including
// for a zero product) and result*2^64 ≡ a*b (mod modulus).
func TestMontgomeryMulReduced(t *testing.T) {
	random := rand.New(rand.NewPCG(20261127, 15))
	moduli := []uint64{3, 41, 1<<63 - 25, 1<<63 + 1, math.MaxUint64, math.MaxUint64 - 2}
	for range 2000 {
		moduli = append(moduli, (random.Uint64()>>random.IntN(62))|1|8)
	}
	for _, modulus := range moduli {
		inverse := montgomeryInverse(modulus)
		if modulus*inverse != 1 {
			t.Fatalf("montgomeryInverse(%d) = %d", modulus, inverse)
		}
		operands := []uint64{0, 1, 2, modulus - 1, modulus - 2, modulus / 2}
		for range 20 {
			operands = append(operands, random.Uint64()%modulus)
		}
		for _, a := range operands {
			for _, b := range operands {
				a, b := a%modulus, b%modulus
				got := montgomeryMul(a, b, modulus, inverse)
				if got >= modulus {
					t.Fatalf("montgomeryMul(%d,%d) mod %d = %d not reduced", a, b, modulus, got)
				}
				hi, lo := bits.Mul64(a, b)
				if bits.Rem64(got, 0, modulus) != bits.Rem64(hi, lo, modulus) {
					t.Fatalf("montgomeryMul(%d,%d) mod %d = %d", a, b, modulus, got)
				}
			}
		}
	}
}
