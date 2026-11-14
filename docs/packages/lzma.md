# lzma notes

`lzma` is the .lzma "alone" format (the lzma(1) legacy container): `Compress`/`Expand` plus the streaming `NewReader`/`NewWriter`. The LZMA model itself lives in `internal/lzmacore` (shared with `xz`); this package is only the 13-byte header and the end-marker convention. Structured as `lzma.go` (both whole-buffer directions) and `stream.go`.

## The wire facts that bit

- **The end marker's isMatch bit is position-coded.** The marker is an ordinary simple match with distance `0xFFFFFFFF`, and its `isMatch` probability slot is keyed by `posState` at the marker's position — encoding it with posState 0 desyncs every stream whose length mod (1<<pb) isn't 0. The encoder must pass `len(data) & mask` (fixed by round-tripping a 30-byte input: every size congruent to 0 worked, others failed).
- **A marker synthesized from zero-fill is not a marker.** Truncated input, zero-filled past its end, can decode garbage bits that happen to form the marker distance; the truncated-prefix sweep caught a prefix that "decoded successfully." A marker is only accepted when no zero-fill byte was ever consumed (`starved == 0`).
- **Known-size and unknown-size streams both exist in the wild.** xz --format=lzma writes unknown-size + marker; Python's FORMAT_ALONE writes the exact size, no marker. The decoder handles both; the streaming reader finishes at the size without demanding a marker.
- **Position contexts must track the absolute stream position, not the buffer length.** The streaming reader delivers from the front and compacts its buffer; the first cut keyed `posState`/litState on `len(buf)`, which desynced the model at the first compaction (found as a 262144-byte boundary — exactly one decode window). `LzmaState.Pos` now carries the absolute position across calls.
- **Delivery must retain a history window.** Matches in a later decode slice may reach back into earlier bytes; draining the delivery buffer to empty made every cross-window match invalid. The reader now holds an 8 MiB reserve behind the delivery point and decodes past it before first delivery.
- **Short-rep at the first byte is legal** (rep0=0 means "repeat the last byte", so a short-rep before any literal is invalid — the decoder rejects it; the note in xz's page covers the same rule from the other side).

## Streaming shape

The reader pulls input through `lzmacore.NewRangeDecoderReader` (the range decoder refills its buffer from an io.Reader; past the end it zero-fills with a starvation counter). Decoding happens in 256 KiB slices through `LzmaDecodeSome`, which stops symbol-aligned at a target or at the marker; the probability model, reps, and absolute position ride the shared `LzmaState` across slices. The writer buffers and emits header + one `LzmaEncodeNoFlush` run + `EncodeEndMarker` + flush at Close — a single continuous coder stream, the format's only shape.

## Tests

`lzma_test.go`: corpus round trips; goldens from `xz --format=lzma` and `lzma -dc`; xz(1) and lzma(1) decode ours; a Python FORMAT_ALONE known-size golden; malformed vectors (short input, bad properties) and the every-prefix-is-malformed sweep. `stream_test.go`: streaming writer output satisfies Expand + the streaming reader + xz(1); the reader decodes goldens including a 300 KB known-size stream (the compaction boundary case); incremental delivery — the first 100 bytes arrive before the stream ends.

Interop reality (Nov 2026, xz 5.8.4): byte-exact both directions.
