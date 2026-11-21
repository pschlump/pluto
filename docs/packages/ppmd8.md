# ppmd8 notes

`ppmd8` is the PPMd variant-I codec (zip method 98): `Decode(data,
props, outSize)` / `Encode(data, props)` over the two-byte property
header. Structured as `ppmd8.go` (arena, sub-allocator with the
EMPTY-stamp node discipline, restart), `model.go` (create/reduce,
rescale, SEE, the restore machinery: Refresh, CutOff, RestoreModel,
ExpandTextArea), `decode.go` (the carryless coder shared by both
sides, the decode driver, Props), `encode.go` (the encode driver).
pluto/ppmd is variant H — a different model, not shared code.

## The wire shape

Two props bytes (LE16): `order-1 | (memMB-1)<<4 | restor<<12`. Order
2..16 (four bits!), memory 1..256 MB in whole MB, restor 0 (restart)
or 1 (cut-off); restor 2 is FREEZE, disabled in the reference for
compatibility — readers reject it.  Then the stream: the range coder
initializes from FOUR bytes (no leading zero byte, unlike the 7z
form), and the payload ends with an ENCODED end marker — the escape
chain walked all the way to the root, which encodes no interval of
its own.  The encoder flushes four Low bytes at the end.

## The range coder

Subbotin's carryless form: both sides track `Low`.  The encoder adds
`start*(Range/total)`; the decoder subtracts the same product from
`Code`.  Renormalization shifts while the top bytes of `Low` and
`Low+Range` agree (a carry cannot escape the top byte), or when
Range has fallen below 2^15 — then Range is re-anchored to `(-Low) &
(2^15-1)` before shifting.  The binary bit-1 update is unlike the 7z
coder's: `Range = (Range & ~0x3FFF) - bound` (clear the low 14 bits
first).  Sums are clamped to Range before thresholding
(`PPMD8_CORRECT_SUM_RANGE`, the 20.01 fix for rare divide
misbehavior) — on both sides, and on the encode side also for the
escape totals.

## The model differences that bit

- **NumStats is count-1.**  The root stores 255; a binary context
  stores 0.  Every loop index and comparison shifts accordingly —
  the expansion loop's realloc condition becomes `ns1 & 1` where
  variant H had `(count & 1) == 0`.
- **The decoder's multi-symbol walk commits every subtraction.**
  The first cut checked `count - freq < 0` without committing the
  subtraction (the C's `count -=` lives inside the condition), so the
  walk picked one state late and every later start offset was wrong —
  symbol 0 of every stream decoded wrong.  Found by the same
  per-symbol oracle diff as variant H's newFreq bug.
- **Absorbed free nodes stay in the lists.**  glueFreeBlocks merges
  address-adjacent free blocks by zeroing the absorbed node's NU —
  but a node listed live can be absorbed by a later node before the
  refill re-reads its NU.  The refill must skip NU==0 entries; the
  first cut panicked (u2i(0) → index -1) on random data.
- **7zz's zip writer silently falls back to method 0** when PPMd
  does not pay (truly random input): the member is STORED, method
  byte 0.  A reader must not assume method-98 input compresses; a
  test harness must check the method byte before parsing props.
- Memory is allocated per the props (memMB); the cut-off restore's
  `GetUsedMemory` bookkeeping (Stamps[] counts) must be exact or the
  restart-vs-cutoff decision diverges from the encoder's.

## Proof of parity

The encoder is deterministic, so parity with 7-Zip is the test: all
goldens re-encode byte-identical, including a 2.7 MB text golden
through a 1 MB arena with restor=1 (many cut-off restores) and
mixed text→random streams.  The C oracle is /tmp scratch, never
shipped.
