package zstd

import (
	"bytes"
	"encoding/base64"
	"io"
	"testing"
)

// TestLibarchiveGolden pins a frame from libarchive's zstd encoder
// (macOS bsdtar --zstd).  It exercises the FSE-coded Huffman-weights
// path, whose header byte bounds the table description and bitstream
// together — a fact the zstd(1) goldens missed because the CLI writes
// direct 4-bit weights for these alphabets.
func TestLibarchiveGolden(t *testing.T) {
	const b64 = "KLUv/QBY5Q8AhhZNNlBplA4QMzMzM4NVGbNJ2VqsybKRbystYvdbeoJ6EREbI7AfoQCE2L3l3nvTpy5fHNQy3vyWAjgAQQBGABphQC8RAZthhQEN/e9MJpqUJ42koZh+e6JJVb56WWzs+g4bazC2Zepee+0L43/re/C/52euTGUFG737qN177+zhkZhITj7RgUUDwoMpocAEMxaWF07EkQw6nmlJaZqitelcBCwC2I5guwJCQiJSYS+w3WwDsM2waoKYSTnQjiRJChdfc+wOBQ6USUOEQs8heWCQNJhgpPiNUB4iC8szZ/RZEgSUSQNEhIHioHEdD0KsHYKgBYEQyOODIluQG80MclKSw2Jz8We7DDLo2rLrTjwsFXW2jl13orIelopa9d613w9LRUHbIuTuX9BG9uJ7uxv/WyqqsgJEqHGdYQwRkYyStFwYAzADEVFQ2DoSQIE4hqCjCCiKYwgcgimGgKIIKorQGIERRipjH/13J6T/LxmODRg3q0ADvpU/+QXIhQxop47qneT3gQk8dqkDlnXA+9QEqwbjILjFYwTvu26AJ2MDbNMAllugxrZVL/wA9zMXCHrW7AzHbmwAvNAdQH5/+W1GFvPPx2bEE1IiWwV2NV8G+M4t8IUpL970IXURguBQR5/2yNDUwDby8tYQYLA7vMl8ChgGcMANSoSLAg=="
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		t.Fatal(err)
	}
	got := Expand(raw)
	if got == nil {
		t.Fatal("libarchive frame failed to decode")
	}
	if len(got) != 9728 {
		t.Fatalf("unexpected payload size %d", len(got))
	}
	r := NewReader(bytes.NewReader(raw))
	all, err := io.ReadAll(r)
	if err != nil || !bytes.Equal(all, got) {
		t.Fatalf("streaming mismatch: %v", err)
	}
}
