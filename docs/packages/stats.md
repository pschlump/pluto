# stats

## Online and paired statistics

The zero-value `Accumulator` uses Welford's online mean and sum-of-squared-
deltas updates. `Snapshot` returns an immutable `Stats`; `Merge` combines either
snapshots or other accumulators. Count, sum, mean, min/max, population/sample
variance, and standard deviation are available. Empty sum is zero; empty means,
extrema, and variance are NaN. Sample variance requires two observations.
Non-finite observations follow Guava-style mean/min/max propagation and make
variance NaN.

`PairedAccumulator` maintains both univariate states and their online product
of deltas. `PearsonCorrelation` clamps roundoff to [-1,1] and returns NaN for
insufficient, non-finite, or zero-variance data. `LeastSquaresFit` returns a
regular, horizontal, vertical, or NaN `LinearTransformation`. Regular and NaN
transformations provide `Slope` and `Transform`; vertical transformations return
false for those operations. `Inverse` swaps horizontal/vertical forms and
inverts non-horizontal regular slopes.

## Quantiles

`Quantile`, `Quantiles`, `Median`, and `Percentile` implement exact Type-7
interpolation. Copying variants leave inputs unchanged; `InPlace` variants may
reorder them. Multiple indexes share one selection pass (see *Selection
invariants* below). Results preserve requested index order and duplicate
indexes. Empty datasets, invalid scales/indexes, and empty index lists return
errors. Any input NaN makes all results NaN; infinite interpolation follows
Guava's endpoint rules.

### Selection invariants (2026-11-27 introselect)

`selectAll` is an introselect: deterministic median-of-three (ranges under
128) or Tukey-ninther pivot *values* (no element is moved to pick them), a
Hoare partition, recursion only into partitions that still hold target ranks
(smaller target set recursed, larger one looped), insertion sort at 16 or fewer
elements, and after `selectDepth(n) = 2·bits.Len(n)` cheap levels the original
repeated median-of-five pivot. Keep these:

- **No NaN reaches selection.** `QuantilesInPlace` scans for NaN first (and
  that scan runs BEFORE index validation — a NaN input with a bad index
  returns NaN results, not `ErrIndex`; pinned by
  `TestQuantilesErrorParityWithReference`). Every comparison in selection is
  therefore a plain `<`/`>`, which equals `cmp.Compare` on NaN-free data.
  Adding NaN-tolerant selection would require restoring `cmp.Compare`.
- **Hoare scans run without bounds checks only because the pivot value is in
  the range** and each swap leaves sentinels. A pivot computed from outside
  values[low..high] (e.g. an average) breaks this.
- **Hoare's no-progress case** is exactly `j == high` (pivot is the maximum
  and the scans meet at the last slot); `partition` then falls back to
  `partition3`, whose equal block is non-empty. Remove the fallback and
  `selectAll` loops forever. `TestPartitionContracts` drives both partitions
  directly, including the fallback.
- **Signed zeros:** with both -0 and +0 in the input, which zero a selected
  slot holds is unspecified (it already was with the median-of-medians code);
  results are numerically equal. Every other result is bit-identical to the
  pre-change implementation, which lives on verbatim as
  `referenceQuantilesInPlace` in `select_internal_test.go` — keep it as the
  differential oracle.
- The median-of-five fallback is not BFPRT (it recurses on the medians, it
  does not select their true median): it keeps ≈n^0.68 elements per side, so
  the worst case is sub-quadratic, not linear. Do not claim linear worst case.

Accumulator `Add`/`Merge` use `minFloat`/`maxFloat`, bit-for-bit
reimplementations of `math.Min`/`math.Max` that inline (the out-of-line
`math.archMin`/`archMax` calls were ~45% of `Add`'s profile). The builtin
`min`/`max` alone are NOT equivalent: `math.Min(-Inf, NaN)` is -Inf and
`math.Max(+Inf, NaN)` is +Inf, where the builtins return NaN. The patch-up on
the NaN branch is load-bearing (`TestAccumulatorExtremaMatchMathMinMax`).
`finite` is `value-value == 0`, exact for every float64. The per-element
`delta / count` division is Guava's arithmetic and the serial dependency chain
it creates (fsub → fdiv → fadd on `mean`) is the remaining floor; replacing it
with a reciprocal multiply would change results in the last ulp.

Accumulators are not internally synchronized. Immutable `Stats`, `PairedStats`,
and `LinearTransformation` values are safe for concurrent reads. No `_ts` twin
is required.
