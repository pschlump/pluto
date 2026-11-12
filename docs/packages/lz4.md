# lz4 notes

`lz4` is the LZ4 frame format (v1.6.5): `Compress(data []byte) []byte`, `Expand(z []byte) []byte` (nil = malformed). The codec of `.lz4`/`.tar.lz4` (zip's fastest method family, some cpio/7z usage, and the "fast" arm of any modern archive tool). Structured as `xxh32.go` (the frame's checksum — reference xxHash32, seed 0, little-endian; short path for headers, 16-byte-round path for content), `lz4.go` (frame writer + hash-chain block encoder), `expand.go` (frame reader + block decoder).

## The wire facts that bit

- **The header checksum excludes the magic.** `LZ4F_headerChecksum(headerStart, ...)` in lz4frame.c starts *after* the 4-byte magic: HC = XXH32(FLG+BD [+C.Size] [+dictID]) >> 8, one byte. Checksumming the magic too produces frames every decoder rejects with `ERROR_headerChecksum_invalid` (found by hexdumping CLI goldens — byte-identical headers failing while byte-different ones passed). The HC byte is **mandatory** (`minFHSize` = 7 includes it); there is no "HC present" flag — FLG bit 1 is reserved-must-be-0, bit 0 is dictID-present.
- **In a sequence, the offset comes BEFORE the extra match-length bytes.** Token, [extra litLen], literals, offset (2-byte LE), [extra matchLen]. Writing the extra matchLen bytes first is self-consistent (our own decoder matched our encoder) and fails only on inputs with matchLen ≥ 19 — found by 'A'×27 round-tripping while 'A'×26 failed... and vice versa once HC was fixed: the minimal repro is a run just long enough to need one 255-run byte.
- **Block checksums cover the STORED bytes** — the compressed bytes as they sit in the file, or the raw bytes for uncompressed blocks (lz4frame.c line 904 and the decoder's `XXH32(selectedIn, tmpInTarget)`), never the decoded output. A `-BX` golden is the test that pins this.
- **FLG bit map**: 7-6 version (`01` only), 5 B.Indep, 4 B.Checksum, 3 C.Size, 2 C.Checksum, 1 reserved (reject), 0 dictID (reject — needs the external dictionary). BD: bit 7 reserved, bits 6-4 block-size id 4..7 (64KB/256KB/1MB/4MB, `1<<(8+2*id)`), low nibble reserved. All reserved bits are rejected on decode.
- **Empty input to Expand is malformed** — a valid empty frame is 15 bytes (7 header + 4 end mark + 4 content checksum). Every proper prefix of a frame is malformed (the prefix-sweep test).
- **The overlap copy is the RLE**: a match with offset 1 and length L appends L copies of the last byte — byte-by-byte copying, never `copy()`, since source and destination overlap. 10000 'a's is a 71-byte frame.
- **Skippable frames** `184D2A50`..`184D2A5F`: 4-byte magic + 4-byte LE size + that many bytes to jump. They may appear anywhere a frame may appear, including between real frames. Concatenated real frames decode to the concatenation of contents (the bzip2 multi-member precedent).
- The legacy magic `184C2102` (lz4's pre-frame format) is just a bad magic here — rejected.

## Encoder shape

4 MB independent blocks (BD 0x70 — the CLI's default is 64KB/BD 0x40; both legal, ours compresses better on big inputs). FLG 0x64: version+B.Indep+C.Checksum, no block checksums, no content size. Single-slot 64K hash table (`(readLE32 * 2654435761) >> 16`), a bounded 16-candidate chain walk at `(h+1) & mask`, `bestLen >= 64` early exit, matches never start in the last 12 bytes (mfLimit) and never extend into the last ~7, positions inside an emitted match are inserted into the table. Incompressible blocks are stored raw (size word bit 31) exactly when the compressed form is not smaller — the reference writer's rule.

## Decoder shape

Mirrors lz4frame.c's state machine flattened into one pass: magic/skippable dispatch, FLG/BD validation (version, reserved bits, block-size id 4..7), optional 8-byte content size, mandatory HC verify, then size words until the end mark — stored-bytes block checksum when B.Checksum, raw or sequence decode per bit 31, content-size and content-checksum verification at the end. Block decode: token → 255-run literal length → literals → (if not at end) 2-byte offset (must be 1..min(65535, output so far)) → 255-run match length → overlapping byte-by-byte copy. A 1<<28 output cap bounds expansion bombs. Last sequence is literals-only (the encoder's contract; a match there is malformed).

## Tests

`lz4_test.go`: corpus (nil, 1 byte, TOBE, 300 KB 'a' runs, 200 KB `abab`, alternating runs, Moby-Dick-ish text, 256 KB random → the raw-store path, >4 MB text → the multi-block path) round-tripped; CLI goldens (plain, `-BX` block checksum, `--content-size`) decoded byte-exact; our empty frame pinned byte-for-byte (pins HC 0xb9 and the XXH32("") = 0x02CC5D05 tail); frame-shape checks (magic/FLG/BD/HC, EndMark position, block count on >4 MB input, raw-store bit on random data); skippable frames before/between/zero-size and concatenated frames; malformed sweep (bad-header vectors with *precomputed valid* HC values so each rejection attributes to its rule — version 00/10, reserved FLG bit, linked blocks, dictID, BD reserved bits, block-size id 3; wrong HC; legacy magic; C.Size mismatch with fixed HC; flipped block checksum; flipped content checksum; trailing byte; five malformed block shapes; a 1 MB expansion bomb); truncation prefix sweep; the tiny-frame/huge-output overlap property; seed-42 200-step randomized model (four content shapes); the `.tar.lz4` pipeline. `bench_test.go`: 25 KiB mixed — both directions ~450 MB/s, 3-5 allocs.

Interop reality (Nov 2026, lz4 v1.10.0): all 9 CLI goldens decode byte-exact; `lz4 -t` and `lz4 -dc | cmp` verify our output for the whole matrix; our ratios are within a few bytes-to-percent of the CLI's (better on nothing, slightly behind on text — the greedy matcher).

No `_ts` twin (stateless, `lzw`/`deflate` precedent). Zero panics. Concurrency-safe by construction.
