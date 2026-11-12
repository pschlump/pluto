# zlib notes

`zlib` is RFC 1950 over `pluto/deflate`: `Compress(data []byte) []byte` / `Expand(z []byte) []byte` (nil on malformed). Adler-32 is unexported here (`adler32(s1, s2, data)` — RFC 1950 §8.2, s1 starts 1, s2 starts 0, mod 65521).

Traps burned in during development, keep the tests:

- **The ADLER32 trailer is ONE 32-bit big-endian word** (`s2<<16 | s1`), not two words. Writing s2 and s1 as separate 4-byte fields produced streams the stdlib reader accepts structurally but rejects with "invalid checksum"; `TestAdlerMatchesStdlib` holds this by comparing our trailer to `hash/adler32` byte-for-byte.
- **FDICT is FLG bit 5 (0x20); FLEVEL lives in bits 6-7.** Writing `flg = 0x20` for "FLEVEL 2" actually sets FDICT — stdlib then demands a dictionary ("zlib: invalid dictionary"). FLEVEL 2 is `0x80`; with FCHECK correction the header is the canonical `78 9c` (there is a test asserting exactly those two bytes).

Header rules on read: CM must be 8, CINFO ≤ 7, `(CMF*256+FLG) % 31 == 0`, FDICT rejected (no dictionary), and the stream must end exactly at the Adler-32 (trailing bytes are malformed — zlib streams do not concatenate). The crafted `78 bb` test vector is FDICT-set with valid FCHECK, pinning the check order (FCHECK passes, FDICT rejects).

Tests: round trips; stdlib interop both directions; Adler trailer vs `hash/adler32` (and the "Wikipedia" = `0x11E60398` sanity vector); malformed sweep (short, bad CM, CINFO 8, bad FCHECK, FDICT, truncated deflate/trailer, corrupt Adler, trailing junk); seed-42 200-step randomized round trips with stdlib cross-checks every 16th step.

No `_ts` twin (stateless; lzw precedent). No panics.
