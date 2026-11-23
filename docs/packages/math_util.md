# math_util

`math_util` implements generic checked and saturating signed/unsigned addition,
subtraction, multiplication, exponentiation, and conversion. Checked arithmetic
returns `(zero, false)` on overflow; saturating arithmetic clamps to the type's
limits. Negative exponents are programmer-error panics.

`Divide`, `Log2`, `Log10`, and `Sqrt` take an explicit `RoundingMode`: floor,
ceiling, toward zero, away from zero, half-even, half-up, half-down, or exact.
Exact operations return `ErrInexact` when rounding would be required. Division
by zero, negative square roots, non-positive logarithms, and overflow have
distinct errors. Half-mode logarithms use exact big-integer comparisons rather
than floating-point midpoint approximations.

The package also includes positive-modulus `Modulo`, checked nonnegative `GCD`,
uint64 `Factorial`/`Binomial`, `IsPowerOfTwo`, floor-rounded overflow-safe
`Mean`, and deterministic Miller-Rabin `IsPrime` over all uint64 inputs.
Factorial/binomial report false if the result exceeds uint64; GCD reports
overflow when a signed type cannot represent the nonnegative magnitude.

All operations are stateless and safe for concurrent use; no `_ts` twin exists.
