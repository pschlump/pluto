# skip_list_dll_ts — Ordered Collection (Thread-Safe Doubly-Linked Skip List)

Package `skip_list_dll_ts` implements an ordered collection of generic items
as a doubly-linked [skip list](https://en.wikipedia.org/wiki/Skip_list) that
is safe for concurrent use.  All operations are guarded by an internal
`sync.RWMutex`.  Items must implement
`github.com/pschlump/pluto/comparable.Comparable` (a single `Compare`
method).

This is the thread-safe version of `skip_list_dll`; both packages expose the
identical API.  Compared to `skip_list_ts`, the level-0 back pointers make
`Backward()` an O(1)-space walk from the tail instead of an O(n) snapshot.

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

Identical to `skip_list_dll`; import
`github.com/pschlump/pluto/skip_list_dll_ts` instead:

```go
var list skip_list_dll_ts.SkipList[Item]

list.Insert(Item(5))
if p := list.Search(Item(5)); p != nil {
	fmt.Println("found", *p)
}
for v := range list.Backward() {
	fmt.Println(v)
}
```

## Thread Safety

`SkipList` **is** safe for concurrent use.  Reads (`Search`, `FindMin`,
`FindMax`, `IsEmpty`, `Length`, and the iterators) take the read lock;
mutations take the write lock.

`Search`, `FindMin` and `FindMax` return a **copy** of the stored item, so
mutating the returned value cannot corrupt the list.

Unlike `skip_list_ts`, the `All` and `Backward` iterators do **not** take a
snapshot: they walk the live list while holding the read lock for the whole
iteration.  This keeps iteration at O(1) extra space, but the loop body
**must not call back into the same list** (a mutation would deadlock; a read
is fine).  This is the same rule that applies to the `Walk*` callbacks in
`avl_tree_ts`.
