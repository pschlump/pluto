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
reorder them. Multiple indexes share one selection pass using deterministic
median-of-medians pivot selection and three-way partitioning, avoiding easy
quadratic pivot behavior. Results preserve requested index order and duplicate
indexes. Empty datasets, invalid scales/indexes, and empty index lists return
errors. Any input NaN makes all results NaN; infinite interpolation follows
Guava's endpoint rules.

Accumulators are not internally synchronized. Immutable `Stats`, `PairedStats`,
and `LinearTransformation` values are safe for concurrent reads. No `_ts` twin
is required.
