# skip_list_dll — Ordered Collection (Doubly-Linked Skip List)

Package `skip_list_dll` implements an ordered collection of generic items as
a doubly-linked [skip list](https://en.wikipedia.org/wiki/Skip_list).  It is
a variant of `skip_list` in which every node also carries a back pointer on
level 0, so the bottom level is a doubly-linked list.  Items must implement
`github.com/pschlump/pluto/comparable.Comparable` (a single `Compare`
method).

The back pointers make descending iteration cheap: `Backward()` walks from
the tail in O(1) extra space, where `skip_list.Backward()` needs an O(n)
snapshot.  For a thread-safe version of this package with an identical API,
see `skip_list_dll_ts`.

## Complexity

Skip lists are probabilistic: the expected cost of the ordered operations is
`O(log n)`, with an `O(n)` worst case.

| Operation                     | Expected | Worst case | Notes                              |
|-------------------------------|----------|------------|------------------------------------|
| `Insert`                      | O(log n) | O(n)       | duplicate keys replace stored item |
| `Search`                      | O(log n) | O(n)       | returns `nil` when not found       |
| `Delete`                      | O(log n) | O(n)       | returns `false` when not found     |
| `FindMin`                     | O(1)     | O(1)       | first node on level 0              |
| `FindMax`                     | O(log n) | O(n)       |                                    |
| `DeleteAtHead`                | O(1)     | O(log n)   | remove smallest                    |
| `DeleteAtTail`                | O(log n) | O(n)       | remove largest                     |
| `IsEmpty`/`Length`            | O(1)     | O(1)       |                                    |
| `Truncate`                    | O(1)     | O(1)       | drops the whole list               |
| `All`/`Backward`              | O(n)     | O(n)       | full traversal, O(1) extra space   |

## Usage

Identical to `skip_list`; import
`github.com/pschlump/pluto/skip_list_dll` instead:

```go
var list skip_list_dll.SkipList[Item]

list.Insert(Item(5))
list.Insert(Item(2))
list.Insert(Item(9))

for v := range list.All() {
	fmt.Println(v) // 2 5 9
}
for v := range list.Backward() {
	fmt.Println(v) // 9 5 2 — O(1) extra space
}
```

## Thread Safety

`SkipList` is **not** safe for concurrent use.  Use `skip_list_dll_ts`
instead, or guard all access with an external mutex.
