# Compression codecs and the archive formats

Which codec each archive/container format actually uses, which pluto
package implements it, and what remains on the roadmap.  Written as
the research summary behind the 2026-11 codec wave (deflate, gzip,
zlib, compress_z, bzip2, lz4); the per-package notes files carry the
implementation detail.

## The mapping

| Format | Codec(s) | pluto package
|---
| tar | none — a stream container; compression is an *external filter* naming the tar file (`.tar.gz`, `.tar.bz2`, ...) | —
| `.tar.gz` | DEFLATE in a gzip member (RFC 1952 over RFC 1951) | `gzip` over `deflate`
| `.tar.bz2` | BWT+MTF+RLE+Huffman (bzip2) | `bzip2`
| `.tar.lz4` | LZ4 frame | `lz4`
| `.tar.Z` | UNIX compress LZW (zopen.c) | `compress_z`
| `.tar.xz` | LZMA2 in the xz container | roadmap (LZMA2)
| `.tar.zst` | zstd (FSE + t0 Huffman) | roadmap (zstd)
| `.tar.lzo` | LZO (mini-LZO) | roadmap (lzo)
| cpio | same scheme as tar: an uncompressed stream container; the same external filters apply (`cpio.gz`, `cpio.bz2`, ...) | as above
| zip method 0 | stored (no compression) | —
| zip method 8 | DEFLATE — the workhorse | `deflate`
| zip method 9 | Deflate64 (64 KB window) | roadmap (deflate64)
| zip method 12 | bzip2 | `bzip2`
| zip method 14 | LZMA (with a zip-specific header quirk) | roadmap (LZMA)
| zip method 93/94 | zstd / xz | roadmap
| 7z `040202` | bzip2 | `bzip2`
| 7z `030101`/`21` | LZMA / LZMA2 (the default) | roadmap
| 7z `030401` | PPMd (variant H/K/M) | roadmap (PPMd)
| 7z `03` | BCJ/Delta/ARM... executable + data filters (pre-processors, not codecs) | roadmap, low value standalone
| RAR3/RAR5 | proprietary LZ+PPMd hybrids (v3) / RAR Lab's own LZ (v5) | blocked (see below)

## The lzw question, answered

`pluto/lzw` is the Sedgwick §5.5 teaching variant — MSB-first,
widening 9→16-bit codes, an EOF sentinel — and is **not** any
archive format's wire format.  The format people usually mean by
"LZW in archives" is UNIX compress (`.Z`, `tar.Z`), whose LSB-first
block-mode dialect is a different codec; that one is `compress_z`,
interop-tested against compress(1).  Neither tar nor gzip ever used
LZW: **`tar.gz` is DEFLATE** (`gzip` over `deflate`, both landed),
and the ancient `tar.Z` route is `compress_z`.  So: to build a tar
tool today, pluto covers `.tar`, `.tar.gz`, `.tar.bz2`, `.tar.lz4`
and `.tar.Z` end to end (each codec package has a tar round-trip
test); `pluto/lzw` stays what it is — the algorithm-classics
implementation.

## Why RAR is blocked

The unrar license permits decompression but **forbids creating a
RAR-compatible compressor** (reverse-engineering to re-create RAR
compression is a license violation, and the format is patent-guarded
in spirit if no longer in force).  A clean-room RAR *extractor* is
legal and exists in free software (The Unarchiver's XADMaster,
unrar-free covers v2/v3 partially), but pluto's contract is
Compress+Expand pairs — shipping an Expand-only codec for a
compressed format would be a half.  If RAR extraction ever matters,
it should be a separate read-only package with that asymmetry called
out in its docs; it is not on the default roadmap.

## What landed (2026-11 wave)

- `deflate` — RFC 1951 (LZ77 hash-chain + lazy matching, canonical
  Huffman, stored/fixed/dynamic blocks).  The codec of gzip, zlib,
  zip-method-8, and (via their own containers) several others.
- `gzip` — RFC 1952 over deflate: tolerant multi-member Expand,
  FEXTRA/FNAME/FCOMMENT/FHCRC, ISIZE+CRC verification.
- `zlib` — RFC 1950 over deflate: Adler-32 trailer, stdlib interop.
- `compress_z` — `.Z` (zopen.c LZW): byte-identical re-encoding of
  compress(1) goldens, the `-b9..16` width range, block-mode CLEAR.
- `bzip2` — the `.bz2` codec; stdlib verifies our streams, we beat
  the stdlib decoder on speed.
- `lz4` — the v1.6.5 frame format; lz4(1) verifies both directions.

All stateless `Compress`/`Expand` over byte slices, stdlib-only, no
`_ts` twins (the `lzw` precedent), zero panics except
`bzip2.CompressLevel`'s level check and `compress_z`'s maxBits check.

## Roadmap order (suggested)

1. **LZMA2/xz** — the biggest coverage gap: `.tar.xz` is the
   de-facto Linux distribution format and zip method 14 / 7z default.
   Range coder + match modeling; the algorithm is a step up in
   complexity from everything above.
2. **zstd** — `.tar.zst`, zip 93, and increasingly the default
   everywhere.  Needs FSE (rANS) and Huff0 first; the frame format
   has many optional features (dictionaries, checksums, multiple
   frames) — the lz4 frame discipline applies.
3. **deflate64** — nearly free once deflate exists (64 KB window,
   wider distance codes); zip method 9 only.
4. **PPMd** — zip method 98 / 7z `030401`; a context-model
   arithmetic coder, no relation to anything above.
5. **lzo** — `.tar.lzo`; simple (LZ77, no entropy stage), mostly a
   licensing-cleanliness exercise (mini-LZO is GPL; implement from
   the format spec, not the code).
