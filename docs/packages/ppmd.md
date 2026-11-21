# ppmd notes

`ppmd` is the PPMd variant-H context-modeling codec in the form
7-Zip uses for native PPMd members: `Decode(data, order, memSize,
outSize)` and `Encode(data, order, memSize)`. Structured as `ppmd.go` (arena, unit sub-allocator,
constants, restart), `model.go` (context creation along suffix
chains, per-symbol updates, rescale), `decode.go` (the carryless
range coder, the per-symbol decode driver) and `encode.go` (the
carry-propagating range coder, the per-symbol encode driver). zip
method 98 is PPMd variant I, a different model.

## The model in one paragraph

One byte arena; a context is a 12-byte unit (NumStats u16,
SummFreq/State2 overlay u16, Stats ref u32, Suffix ref u32) and a
statistic is a 6-byte state (Symbol, Freq, Successor stored as two
LE u16 halves). Successors are NULL only at the root, "raw" (a
position in the text) below UnitsStart, real context records at or
above it. Decoding starts at MinContext: multi-symbol contexts walk
the stats against the range coder's threshold; binary contexts price
one bit through BinSumm[128][64]; a miss masks the tried symbols and
drops to the suffix, with the escape frequency priced by SEE
(See[25][16]). Memory exhaustion restarts the model — the format's
own mechanism, mid-stream, not an error.

## The arithmetic details that bit

- **The reference's newFreq folds a leading 1 into both arms**: `1 +
  (2cf<=s0 ? (5cf>s0) : (2cf+s0-1)/(2s0)+1)`. The classic 2001
  Shkarin source ends the second arm at `+1`; the 2023 reference
  7-Zip builds from ends at `+2`. One low on every newly created
  context whose successor dominates its parent. Symptom: symbols
  stay correct for ~200k of a 201k-symbol golden while Range
  bookkeeping drifts, then one pick flips. Found by compiling the
  reference decoder as an oracle and diffing per-symbol
  (symbol, Code, Range, pos) — symbol 94 diverged in Range alone,
  and a second diff of every binary-decode index decomposition
  (cell, freq, prevSuccess, runLength, suffix NumStats, symbols)
  pinned it to one state's Freq: 3 vs 2.
- **SEE update shifts by the OLD shift**: `Count = 3 << Shift` then
  `Shift++`. Shifting after the increment starves every escape
  estimate; found on a random-data golden (text passes — few
  escapes).
- **The free-list link lives at offset 0 of a freed block**, not at
  the +4 where live node links sit. Walking a free list through +4
  walks live state records and eventually slices out of the arena.
- **The escape masking loop breaks at `s2 >= s` AFTER the +12
  stride** — the reference masks pairs with the condition evaluated
  post-increment; an off-by-one masks one state too few and picks
  the wrong symbol on the first escape.
- **rescale's collapse-to-binary is a do-while**: escFreq and freq
  halve at least once even when escFreq is already 1.
- **Renormalization sites are format-defining**: double-shift after
  every DecodeFinal, single after a binary bit-0 (min BinSumm 95 >
  1<<(14-8) makes one byte enough), nothing after the escape decode
  within one symbol walk, double at the head of each masked
  iteration. In the 2023 reference RC_NORM_LOCAL is empty — every
  norm in the file is the remote double form.
- **7zz writes props reduced by file size** when memSize is left
  default — parse the five prop bytes from the header (order, LE32
  memSize); never assume the requested memory.
- Reads past the end of the stream yield zeros and still increment
  the position — the model terminates on the padding it implies, so
  a truncated stream is a decode error, not a panic.

## Debugging method that found the last bug

Per-symbol traces from both implementations — symbol, Code, Range,
stream position — written by a throwaway driver on each side, then
diffed. The first divergent field localizes the bug to one decode
path; a second, finer diff of that path's inputs (for binary
decodes: the BinSumm cell index decomposition) names the exact
state. The C oracle is /tmp scratch, never shipped.

## The encoder

`encode.go` mirrors the decoder path for path: the same walk (find the
symbol in MinContext, escape through SEE-masked suffix contexts), the
same model mutations at the same points, and a range coder with
carry — 64-bit `Low`, the cache/0xFF-run carry flush, the first
emitted byte the zero the decoder's init consumes, five `shiftLow`s at
the end.  Norm sites mirror the decoder's: double after every final
encode, single after a binary hit (`rng = bound`), none after a plain
encode — the escape intervals *within* one symbol walk never
normalize, on either side.

The masked-path totals are the subtle part: the escape walk adds
`freq & mask` for free (already-tried symbols contribute zero), and
when the symbol is found the remaining total is accumulated from the
found state on — the found frequency enters through the odd-first term
when the remaining count is odd, or through the first pair when it is
even, never both.  Encoded `low` excludes the escape frequency, which
sits at the top of the interval.

Because everything is deterministic, `Encode` of a golden's data with
the golden's props reproduces 7zz's stream **byte for byte** — all
goldens, including a restart-heavy 500 KB encode inside a 1 MB arena
and random data, matched on the first working build.  That parity is
the encoder's primary test.
