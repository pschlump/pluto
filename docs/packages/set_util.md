# set_util

`Cartesian(inputs...)` lazily yields fresh tuples in lexicographic odometer order.
No inputs yield one empty tuple; any empty input yields none. Output size is the
product of input lengths. `PowerSet(values)` yields fresh subsets in binary
inclusion order and has 2^n outputs. PowerSet inputs are set elements and should
be distinct. Both iterators stop without producing more values when the consumer
stops; tests cover ordering, empty dimensions, independent output slices, and
early termination on a large input.
