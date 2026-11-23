# map_util

`UniqueIndex` maps each input value to a unique computed key and returns an
`immutable.Map`. Duplicate keys return `immutable.ErrDuplicateKey`; no partial
result is published. `UniqueIndexFunc` supports custom key equality/hash.
Construction is O(n) expected plus key-function cost.

`Difference` and `DifferenceFunc` partition two Go maps into fresh `Common`,
`Differing`, `LeftOnly`, and `RightOnly` maps. Differing entries retain both values
in `ValueDifference`. The Func form accepts non-comparable values. Runtime is
O(left+right) expected. These stateless functions have no `_ts` twin.
