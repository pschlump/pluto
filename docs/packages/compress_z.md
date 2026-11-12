# compress_z notes

`compress_z` is the historical UNIX compress `.Z` format (compress(1)/zopen.c — `file.Z` and `.tar.Z`): `Compress(data []byte) []byte` (block mode, maxBits 16), `CompressBits(data []byte, maxBits int) []byte` (the `compress -b` range), `Expand(z []byte) []byte` (nil = malformed). A different wire format from pluto `lzw` (algs4 MSB-first fixed-12-bit) — same algorithm family, not interchangeable bytes; `.tar.Z` is THIS format, `.tar.gz` is DEFLATE (pluto `deflate`/`gzip`).

Structure: `bits.go` (LSB-first bitWriter/bitReader with `realign` — the group machinery; both have two modes, see streaming below), `compress_z.go` (encoder), `expand.go` (decoder), `stream.go` (streaming Reader/Writer, below).

## The wire format (reverse-engineered to byte-exactness)

Header `1F 9D`, byte 3 = maxbits|0x80 (block mode). Codes LSB-first from 9 bits; CLEAR=256; first free slot 257. Verified against macOS `compress` (the FreeBSD 1992-93 zopen.c lineage) at every `-b` width, and cross-read against `ncompress` 4.2.4's compress42.c:

- **Width machine**: `maxcode = 2^n − 1`, checked AFTER each code's write but BEFORE its table add (encoder) — the decoder checks after its add. This exact ordering absorbs the one-add lag between the two sides; any other order produces streams the reference tools reject at the 9→10 growth. There is NO initial clamp of maxcode into maxbits: a growth landing exactly on maxbits clamps maxcode to `maxmax = 2^maxbits` (width stops growing), but at **-b9/-b10/-b11 the table fills while maxcode is still `2^maxbits−1`** — free runs past maxcode again, the width grows one bit PAST maxbits and stays (a -b9 stream ends in 10-bit codes with values <512). Overshoot exists ONLY at 9/10/11; at 12+ the clamp catches the boundary growth. This quirk is why BSD uncompress/gzip reject their own `-b9..-b11` output with EFTYPE; our decoder reads all widths 9..16. `TestOvershootWidths` and the `dict9` golden hold this.
- **8-code groups**: codes are packed in groups of eight — a group of n-bit codes is exactly 8×n bits, and a width change or CLEAR takes effect only at the next group boundary; the encoder zero-pads a partial group ("Write the whole buffer, because the input side won't discover the size increase until after it has read it" — zopen.c). Growth points are ALWAYS naturally aligned (they occur after 2^n−256 codes, a multiple of 8), so in practice only CLEAR observably pads (measured: 60-bit pad at a real CLI CLEAR, exactly the 96-bit boundary from segment start). `bitWriter.base`/`bitReader.base` anchor the group arithmetic to the segment start (post-header for the initial segment, post-pad after a CLEAR) — getting that anchor wrong (0 vs 24 bits) pads 48 spurious bits at the first growth.
- **CLEAR** resets to a fresh dictionary: realign, nBits=9, maxcode=511, free=257, oldcode=−1 — the next code is a literal with NO table add. BSD fires CLEAR only when the table is full AND `in_count ≥ checkpoint` (CHECK_GAP=10000): inputs <10000 bytes never contain CLI CLEARs (why the goldens are CLEAR-free and the crafted vector exists).
- **Empty input**: compress(1) writes a 0-byte file; we write the 3-byte header (a valid empty stream) — documented deviation, `TestEmptyStream`.

## Decoder

Strict `code > free || oldcode < 0` → nil; KwKwK (`code == free`, oldcode ≥ 0) pushes finchar then walks oldcode; chain walk bounded at maxmax (cycle guard — only malformed input reaches it); the very first code adds no entry (oldcode=−1 gate); non-block mode starts free at 256 and treats 256 as an ordinary undefined entry. No EOF code: readCode failing mid-bit is the clean end; truncated streams yield nil or a clean prefix (`TestTruncated`).

## Encoder

Greedy LZW, hash map keyed `uint64(ent)<<8|c`. Never emits CLEAR (the ratio heuristic is skipped): streams stay valid and byte-identical to the CLI on sub-CHECK_GAP inputs; on bigger inputs the CLI clears and we don't — both self-round, and `gzip -d` verifies ours at b12+. Panics only on maxBits outside [9,16], message naming the method and range.

## Tests

`compress_z_test.go`: corpus round trips at ALL widths 9..16; CLI goldens embedded at b9/b12/b16 for TOBE (21 B) and a 2000-byte dict slice — decoded byte-exact AND re-encoded demanding byte-for-byte identity with compress(1) (the corpus is recovered by expanding the golden, gzip-test style); `TestClearStream` hand-encodes a mid-stream CLEAR with exact group padding (the path sub-CHECK_GAP goldens can't reach); crafted vectors for KwKwK ([97,257]→"aaa", also asserted equal to `Compress("aaa")`) and non-block mode; malformed sweep (short input, both magics, maxbits 8/17/31, first-code-non-literal, code-above-table, KwKwK-without-oldcode, non-block first-256); truncation prefix rule; panic-message checks; seed-42 300-step randomized round trips with rotating width; `TestTarZRoundTrip` (tar → Compress → Expand → untar, the `.tar.Z` pipeline).

## Interop reality (macOS, Nov 2026)

`compress -b N` goldens: all 24 (3 corpora × 8 widths) decode byte-exact. Our re-encode is byte-identical for TOBE and dict at all 8 widths, and for a 401 KB mixed corpus at b14–b16 (the b9–b13 cells differ only by CLI CLEARs). `gzip -dc` verifies our b12+ streams. No stdlib `.Z` reader exists, so the CLI goldens carry the interop burden (tests are hermetic — no shelling out).

## Streaming (`stream.go`)

`NewWriter(dst)` / `NewWriterBits(dst, maxBits)` / `NewReader(src)` — the same stream over io.Writer/io.Reader with memory bounded by the LZW dictionary. The decode side is `expand.go`'s loop lifted into a resumable `decoder` (the loop locals became fields; `step()` decodes one code and reports stData/stClear/stEnd/stBad) driven over the refilling bitReader — decode rules are byte-identical to Expand's. Error model differs from the sibling codecs in one way: **.Z has no end-of-stream framing, so there is no truncation error** — the stream simply stops, any source end is a clean end, and a cut input decodes to a prefix of the output (the streaming analogue of `TestTruncated`).

- **Group alignment survives draining/compaction on an absolute bit scale.** The 8-code groups are anchored to absolute positions: the streaming bitReader counts `dropped` bytes removed by compaction and the streaming bitWriter counts `drained` bytes moved to dst; `consumedBits()`/`totalBits()` add them back so `realign` still lands on multiples of 8n from the segment start. Reset-to-zero traps: a CLEAR mid-stream with the old relative arithmetic padded 48 spurious bits.
- **The streaming Writer is chunk-independent**: greedy LZW is a pure function of the byte sequence with one byte of carry (the pending chain start) across Write calls, so the output is byte-identical to `CompressBits` regardless of chunking — pinned by test. (`free` must init to `firstEntry` in NewWriterBits; the zero value assigns dictionary slots from 0 and corrupts everything — burned in.)
- **`Flush` is drain-only and best-effort**: codes pack continuously with no alignment point (unlike lz4 blocks or bzip2 members), so a flush cannot make the tail decodable; it only pushes out whole bytes that already exist. The final partial byte is zero-padded by `Close` alone.
- LZW output is never re-referenced: decoded bytes are delivered and forgotten, no history window.

Tests (`stream_test.go`): round trips × chunks; chunk independence vs `CompressBits` (chunks 1/3/4096); all widths 9..16 + panic; empty == `Compress(nil)` (3-byte header); cross whole-buffer both ways; CLI goldens; OneByteReader; prefix-decodes-prefix stride-61 (short header → `io.ErrUnexpectedEOF`); ErrTypes (bad magic, first code 258); Write-after-Close sticky error.

No `_ts` twin (stateless, `lzw`/`deflate` precedent). Exactly one panic. Concurrency-safe by construction.
