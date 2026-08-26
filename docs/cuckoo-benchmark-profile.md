 ● The profiling benchmark is in place and runs. Here's a summary:

   What I added

   cuckoo_ts/profile_test.go — BenchmarkProfileMix (external test package, so it only uses the public API):

   • Workload: 100,000 inserts of int keys in a fixed pseudo-random permutation (xorshift64 PRNG, same workload every run), 10,000 deletes interleaved (one every 10th
     insert), and 1,000,000 searches (10 per insert).
   • Missing keys: deletes probe a range 20% wider than the inserted keys, and half the searches target keys never inserted — so a large share of both miss.
   • Metrics: elapsed time (benchmark ns/op), allocation count and total bytes (runtime.MemStats TotalAlloc/Mallocs delta), live memory at the end (post-GC Alloc), final
     table capacity, and grow/shrink counts (capacity is polled after every insert; the run waits for the background resizer to settle before measuring).

   Baseline results on this machine (M4 Max)

   ```
     67929166 ns/op   (~68 ms for the whole mix)
     24 allocs        (13,385,296 alloc-bytes total, incl. the 800KB permutation slice)
     6,634,400 live-bytes at the end
     10 table-grows, 0 table-shrinks
     final-capacity 262,144, final-len 96,018
     243,404 search-hits / 756,596 search-misses
     3,982 delete-hits / 6,018 delete-misses
   ```

   A few things worth noting for your comparison:

   • 10 grows is exactly log₂(262144/256) — the table went from the 256 minimum to 262,144, doubling each time saturation crossed 0.85. No collision-loop (kick-limit)
     resizes occurred; these are all threshold-triggered, run on the background goroutine.
   • The 6.6 MB live memory matches the theoretical floor: 262,144 slots × 24 bytes/slot (int + hash + used flag) ≈ 6.29 MB plus the key permutation.
   • Delete misses are high by design (only ~98k live keys in a 120k probe range, and early probes hit a nearly empty table).

   Run it anytime with:

   ```
     go test -run xxx -bench ProfileMix -benchmem -benchtime 1x -v ./cuckoo_ts/
   ```

   Since the PRNG seed is fixed, your alternative hashing scheme will see the identical operation sequence, making the comparison apples-to-apples.
