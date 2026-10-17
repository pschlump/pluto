# Per-package implementation notes

One file per package (a file often covers the package and its `_ts`
twin together): structure, invariants, panic contracts,
algorithm-specific traps, test-oracle expectations, and benchmark
reality. `twins.md` holds the full `_ts` twin mapping and the
lock-discipline rules that apply across the twins.

These are working notes for anyone changing the library (humans and
agents alike): **read a package's file before changing the package, and
update it in the same change.** They were extracted verbatim from the
repository's `CLAUDE.md` on 2026-10-17, which now carries only the
global design contract and an index of these files.
