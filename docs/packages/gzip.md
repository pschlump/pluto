# gzip notes

`gzip` is RFC 1952 over `pluto/deflate` + `pluto/crc` (IEEE CRC-32): `Compress(data []byte) []byte` writes one minimal member (FLG 0, MTIME 0, OS 255, trailer CRC32+ISIZE little-endian); `Expand(gz []byte) []byte` decodes every member of a multi-member file (contents concatenate, like `gzip -d`), nil on malformed. Structure: one file, `expandMember` loops over members; `deflate.Expand`'s consumed-count return is what locates the trailer — that is why deflate's API returns `(out, n)`.

Parsing order after the 10-byte base header: FEXTRA (XLEN LE + skip), FNAME (zero-terminated), FCOMMENT (zero-terminated), FHCRC (2 bytes = low 16 of CRC-32 of everything before it, verified when present) — RFC 1952 §2.3's flag order. Reserved FLG bits set → malformed. Trailer: CRC-32 of the decompressed data (LE) and ISIZE = len mod 2^32 (LE), both verified; trailing bytes after a member must start a new member or the whole file is malformed.

Traps: FHCRC is 2 bytes — read them directly, do not run a 4-byte LE reader off the end. The multi-member loop must treat "0 bytes left" as clean EOF and "1-9 bytes left" as malformed. FEXTRA overrun (XLEN past the slice) must be caught before the header is used.

Tests: round trips over the shared corpus; **stdlib interop both directions** including members written by the stdlib writer with Name/Comment/Extra/ModTime set (all flag branches); hand-crafted FHCRC member (set the flag bit on our own member, insert the computed CRC bytes) plus its corrupt twin; multi-member joins of ours and stdlib's; **CLI goldens embedded** (`gzip -9 -n` of "TOBEORNOT…", empty, and 2000 bytes of /usr/share/dict/words — regenerate with `printf … | gzip -n | xxd -p`, splice programmatically, never retype hex by hand); malformed sweep (magic, CM, reserved flags, unterminated FNAME, XLEN overrun, truncated deflate/trailer, corrupt CRC/ISIZE, trailing junk). `TestTarRoundTrip` is the demo of record: archive/tar → `gzip.Compress` → stdlib gzip+tar read-back, and the mirror with stdlib writing. Seed-42 randomized round trips live in the gzip corpus test.

No `_ts` twin (stateless; lzw precedent). No panics.
