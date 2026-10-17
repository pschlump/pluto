# union_find notes

`UnionFind` is the one non-generic structure: the elements ARE the indices `0..n-1` (Sedgwick §1.5 UF). Union-by-rank + path halving in the lock-free `nlFind` body shared by `Find`/`Union`/`Connected`. Out-of-range indices report `false`/`ok=false` (the heap indexed-op convention), never panic; the panic contract is exactly 2 — `NewUnionFind(n < 1)` and `Union` on a nil set. `union_find_ts` is the twin whose `Find`/`Connected` take the **write** lock (path halving mutates `parent`) — only `Count`/`Len` take the read lock; documented prominently in the package doc and README. `checkInvariants` in thorough_test.go verifies cycle-free parent chains, Find/Connected agreement, and Count == distinct roots — call it after structural changes.

