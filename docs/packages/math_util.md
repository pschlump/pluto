# math_util

`math_util` implements generic checked and saturating signed/unsigned addition,
subtraction, multiplication, exponentiation, and conversion. Checked arithmetic
returns `(zero, false)` on overflow; saturating arithmetic clamps to the type's
limits. Negative exponents are programmer-error panics.

`Divide`, `Log2`, `Log10`, and `Sqrt` take an explicit `RoundingMode`: floor,
ceiling, toward zero, away from zero, half-even, half-up, half-down, or exact.
Exact operations return `ErrInexact` when rounding would be required. Division
by zero, negative square roots, non-positive logarithms, and overflow have
distinct errors. Half-mode logarithms are exact (no floating-point midpoint
approximation) and allocation-free: see *Fast-path invariants* below.

The package also includes positive-modulus `Modulo`, checked nonnegative `GCD`,
uint64 `Factorial`/`Binomial`, `IsPowerOfTwo`, floor-rounded overflow-safe
`Mean`, and deterministic Miller-Rabin `IsPrime` over all uint64 inputs.
Factorial/binomial report false if the result exceeds uint64; GCD reports
overflow when a signed type cannot represent the nonnegative magnitude.

All operations are stateless and safe for concurrent use; no `_ts` twin exists.

## Fast-path invariants (2026-11-27)

The package is stdlib-only and allocation-free on every path; `math/big`
appears only in tests, as the oracle. Keep these when editing:

- **Half-mode logarithms have no ties.** The midpoints 2^(f+1/2) and
  10^(f+1/2) are irrational, so value² never equals 2^(2f+1) or 10^(2f+1)
  and HalfUp/HalfDown/HalfEven all reduce to "round up iff value > T(f)",
  T(f) = floor(sqrt(midpoint²)). `Log2` derives T(f) as
  `0xB504F333F9DE6484 >> (63-f)` (floor(sqrt(2^127)) shifted — floor of a
  floor quotient is exact). `Log10` reads `halfPowersOf10[f]`; entry 18 is
  ...331 (Guava's table says ...332 because it compares differently — do not
  "fix" it to match), entry 19 is MaxUint64 because sqrt(10^39) > 2^64.
- **`log10Floor`** estimates `bits.Len64(x)*1233>>12` (never low, at most one
  high for 1..64 bits) and corrects with one `powersOf10` comparison. 1234 is
  an equivalent multiplier over 1..64 bits (a mutation test cannot tell).
- **`Binomial`** looks up n ≤ 67 in a Pascal-triangle table (k ≤ n/2 only,
  1190 entries, built in `init` by additions; C(67,33) fits, C(68,34) does
  not). Above 67 it runs the multiplicative recurrence with a 128-bit
  intermediate: C(n,i)·(n−i) = C(n,i+1)·(i+1) is exact, and because C(n,i)
  increases for i ≤ n/2, `hi >= i+1` proves the FINAL result overflows, so
  the early `false` is correct (the old big.Int loop ran all k steps, which
  for huge k took impractically long).
- **`IsPrime`** uses Montgomery multiplication (R = 2^64) after the
  small-prime filter guarantees an odd modulus > 37. `montgomeryMul` must
  return a FULLY reduced residue (< modulus — the `hi < mhi` correction),
  because witnesses are compared by equality against Montgomery one and
  minus-one; `TestMontgomeryMulReduced` pins this (the zero-product case is
  the one Miller-Rabin rarely exercises). Witness set unchanged.
- **`Divide`** makes the rounding decision inline. Invalid modes and
  `RoundExact`-with-remainder still return the truncated quotient next to the
  error, and the divide-by-zero and MinInt/−1 checks still precede the mode
  check — the differential tests compare values AND errors.
- **`Sqrt`** corrects the float estimate with multiplications, capped at
  2^32−1 so squares never wrap. The upward correction loop is unreachable for
  uint64 (the float estimate never undershoots) but is kept as a guard; its
  removal survives mutation testing for that reason.
- `fastpath_test.go` keeps the previous implementations (`oracle*`) and
  compares every fast path against them over all thresholds ±2, exhaustive
  8/16-bit ranges, and seeded random samples. Extend it rather than deleting
  the oracles.
