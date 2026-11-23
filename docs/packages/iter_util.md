# iter_util

`iter_util` provides lazy algorithms over standard Go `iter.Seq` and
`iter.Seq2` values. `Concat`, `Filter`, `Map`, `Take`, `Skip`, and `Partition`
propagate early termination and do not retain consumed elements. `Partition`
returns independent slices and never emits an empty final part.

`Frequency`, `ElementsEqual`, `First`, `Last`, `Only`, `At`, `IsEmpty`, and
`Len` provide explicit consuming queries. `First`, `At`, and `Only` stop as soon
as their answer is known; `Last`, `Frequency`, and `Len` necessarily consume the
whole input. Negative `At` indexes return false without starting the input.
Negative Take/Skip counts and non-positive Partition sizes panic as programmer
errors.

`NewPeeking` uses `iter.Pull` for one-value lookahead. `Peek` is idempotent,
`Next` consumes the lookahead, and `Close` releases the producer when iteration
ends early. A cursor is not internally synchronized.

`Pairs`/`Seq2` and `Keys`/`Values` adapt two-value sequences. `FromSlice` takes
a snapshot; `ToSlice` explicitly materializes a sequence. No thread-safe twin
is needed for stateless functions. Each cursor requires external synchronization
if shared.

## Complexity

Transformations are O(1) per consumed item, excluding caller callbacks.
Partition uses O(k) space for each part of size k. Frequency uses O(distinct)
space. Other queries use O(1) space; Peeking stores one value.
