# binary_tree notes

`BinaryTree[T any]` holds `root`, `length`, and the stored `cmp` func. `deleteBy` implements delete (two-children case promotes the in-order successor). `DeleteAtHead`/`DeleteAtTail` splice the extreme node out in a single descent — they do not call `FindMin`/`FindMax` + `Delete`. Split across files: core in `binary_tree.go`, `Front`/`All`/`Backward` iterators in `iter.go` (slice stack, no external stack package), `WalkFunc` in `apply.go`.

`binary_tree_ts` adds a `lock sync.RWMutex` to the same struct, lock-free `nl*` helpers (`nlIsEmpty`, `nlFindMin`, `nlFindMax`, `nlDepth`) for use while the lock is held, and `snapshotInOrder() []T`, which backs `Front`/`All`/`Backward`.

