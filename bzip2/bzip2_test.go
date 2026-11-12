/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package bzip2_test

import (
	"archive/tar"
	"bytes"
	stdbz "compress/bzip2"
	"encoding/hex"
	"io"
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/pschlump/pluto/bzip2"
)

// CLI golden vectors, generated with bzip2(1) on 2026-11-12:
//
//	printf 'TOBEORNOTTOBEORTOBEORNOT' | bzip2 -9 -c
//	printf '' | bzip2 -9 -c
//	head -c 2000 /usr/share/dict/words | bzip2 -9 -c
//
// The empty-input stream is byte-identical to ours (header + final
// magic + zero CRC — there are no blocks to differ over).  The other
// two differ from our output (bzip2 sorts with a different fallback
// and runs its multi-table optimizer) but decode byte-exact here,
// and our own streams are in turn verified by the stdlib reader and
// by bzip2(1) itself (checked out-of-band; tests stay hermetic).
const (
	goldenTobeHex  = "425a683931415926535917530afa00000a060012019400200021b50f48300e306512ca2cd9e2ee48a70a1202ea615f40"
	goldenEmptyHex = "425a683917724538509000000000"
	goldenDictHex  = "425a68393141592653598e40785600006d4580001020003fefdff05003ab37adbd75cd4c196c34d04d098427aa00c7b4a7e93247a8034d034c42049a4d00049aaa300000024d28224f4694d068d335d4eaebcb67e47295f79f0f7fbcb0b569a388cc15b952deb09b0a74f947d123597ab196a6ee32e8354045705f1421495a4ec49c10c4c6a2778cd3c58e88568f1487b7a4fea5eb4879cd6493659a792b1d49b9136a6a84ac0c3f8ca6f3e3359b3a5e271b538d866434f10616cd2d42d05b5c0020cee4b5799dc34afb210008c511103bf5cdc9b679487d32a0db35ae9d98f7ce70d4d9ba5be96c6a13c61d92e89c3777fa71f2df4cb39c0d41de1032165002137e42f2fb4c091d3aaf73619265b4c1202233f1c6c28011225a02808a2152564ac4b614615228b160a41411914149050158a2aa2a242b050508b34e7042b89558b0f7687c29b101b48c21fca22149864342a2411077295908f55426463008932a9730b206c986929c63bc779deacedd6b5d7de2abd9ce875624d0fbdf46496a8112235be8fd99a8cd4d136f8315c51030cfaee866cfe6f980099a69687362663c35d28270c405ae7ad3dc3d0d56481708ca11f8be0f4f7b5f34d4284928464883e4e44e7bc3e76bbeffb6ef252f610949c2ea73534e06184fc291a378e84e1429e861422304181eed6ae7bab518336c63555f5dc3bc06b079dd8004a123817c1e947c8d4062508c4b060aaa2e3a9d671e766de17d6fc3bc9f4c81c9c50630f2cde2a5b300b0abe321445933943a3162e59924a4c2aa2273e7a92fdfc78efdf78206cc3d4c35857760e42ece5dc36338013e75c2df9e8cb979c4e7b38f57b6e6ca61a15ac5bca3260a29eeaf5987c1306b16552b07ce2070208900a0b7989d09f509b72e5516ec0376510482606cc331485269b02a6122bd0a227337f4e2da1d9c98bc6af0cc8b1570cddd889af3bdbab8392a82c3dbae39e7c716d218101ccf9497252fb9af8108eaab79a061778efa5b75019d126460e204b5ae15e876eaca34e38487cd7f8bb9229c284847203c2b00"
)

func hexBytes(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatalf("bad golden hex: %v", err)
	}
	return b
}

func corpus() [][]byte {
	rng := rand.New(rand.NewPCG(42, 42))
	text := []byte(strings.Repeat("Call me Ishmael. Some years ago—never mind how long precisely—", 300))
	random := make([]byte, 65536)
	for i := range random {
		random[i] = byte(rng.IntN(256))
	}
	return [][]byte{
		nil,
		[]byte("a"),
		[]byte("TOBEORNOTTOBEORTOBEORNOT"),
		bytes.Repeat([]byte("a"), 300000),  // RLE1 chunking at 259
		bytes.Repeat([]byte("ab"), 100000), // duplicate rotations, ties in the BWT sort
		bytes.Repeat([]byte{0xAA, 0x00, 0xCD}, 9000), // alternating short runs
		text,
		random,
	}
}

func TestRoundTrip(t *testing.T) {
	for i, data := range corpus() {
		for _, level := range []int{1, 9} {
			z := bzip2.CompressLevel(data, level)
			if z == nil {
				t.Fatalf("corpus %d level %d: CompressLevel returned nil", i, level)
			}
			if back := bzip2.Expand(z); !bytes.Equal(back, data) {
				t.Fatalf("corpus %d level %d: round trip mismatch (%d back)", i, level, len(back))
			}
		}
	}
}

// TestStdlibReadsOurs is the interop oracle for our encoder: the
// standard library's reader must decode every stream we write.
func TestStdlibReadsOurs(t *testing.T) {
	for i, data := range corpus() {
		for _, level := range []int{1, 9} {
			r := stdbz.NewReader(bytes.NewReader(bzip2.CompressLevel(data, level)))
			got, err := io.ReadAll(r)
			if err != nil {
				t.Fatalf("corpus %d level %d: stdlib rejected our stream: %v", i, level, err)
			}
			if !bytes.Equal(got, data) {
				t.Fatalf("corpus %d level %d: stdlib decoded wrong bytes", i, level)
			}
		}
	}
}

func TestCLIGoldensDecode(t *testing.T) {
	if got := bzip2.Expand(hexBytes(t, goldenTobeHex)); string(got) != "TOBEORNOTTOBEORTOBEORNOT" {
		t.Fatalf("golden tobe: got %q", got)
	}
	if got := bzip2.Expand(hexBytes(t, goldenEmptyHex)); len(got) != 0 {
		t.Fatalf("golden empty: got %d bytes", len(got))
	}
	dict := bzip2.Expand(hexBytes(t, goldenDictHex))
	if len(dict) != 2000 || dict[0] != 'A' {
		t.Fatalf("golden dict: got %d bytes starting %q", len(dict), dict[:4])
	}
	// The empty stream is byte-identical to our own.
	if z := bzip2.Compress(nil); !bytes.Equal(z, hexBytes(t, goldenEmptyHex)) {
		t.Fatalf("empty stream: got % x", z)
	}
}

// TestLevelsAreBlockSizeOnly: below 100k the level changes only the
// header digit and the stream bytes after it.
func TestLevelsAreBlockSizeOnly(t *testing.T) {
	data := []byte(strings.Repeat("the quick brown fox", 500))
	z1 := bzip2.CompressLevel(data, 1)
	z9 := bzip2.CompressLevel(data, 9)
	if z1[3] != '1' || z9[3] != '9' {
		t.Fatalf("level digits: %q %q", z1[3], z9[3])
	}
	if !bytes.Equal(z1[4:], z9[4:]) {
		t.Fatal("sub-100k stream differs beyond the header digit")
	}
}

// TestMultiMember: concatenated streams (what `bzip2` of two files
// glued together looks like) decode to their concatenation.
func TestMultiMember(t *testing.T) {
	a := []byte(strings.Repeat("first member ", 100))
	b := []byte(strings.Repeat("second member ", 60))
	glued := append(bzip2.Compress(a), bzip2.Compress(b)...)
	if got := bzip2.Expand(glued); !bytes.Equal(got, append(append([]byte(nil), a...), b...)) {
		t.Fatalf("multi-member: got %d bytes", len(got))
	}
	// Different levels per member are fine too.
	glued2 := append(bzip2.CompressLevel(a, 1), bzip2.CompressLevel(b, 9)...)
	if got := bzip2.Expand(glued2); !bytes.HasPrefix(got, a) || !bytes.HasSuffix(got, b) {
		t.Fatal("multi-member mixed levels failed")
	}
}

func TestMalformed(t *testing.T) {
	valid := bzip2.Compress([]byte("some data to compress and then corrupt sensibly"))
	cases := []struct {
		name string
		z    []byte
	}{
		{"nil", nil},
		{"empty", []byte{}},
		{"short", valid[:3]},
		{"bad magic 0", []byte("XZh9")},
		{"bad magic 2", []byte("BZx9")},
		{"bad level 0", []byte("BZh0")},
		{"bad level A", []byte("BZhA")},
		{"truncated mid-block", valid[:20]},
		{"trailing garbage", append(append([]byte(nil), valid...), 'X', 'X')},
		{"trailing partial magic", append(append([]byte(nil), valid...), 'B')},
	}
	for _, tc := range cases {
		if got := bzip2.Expand(tc.z); got != nil {
			t.Errorf("%s: Expand returned %d bytes, want nil", tc.name, len(got))
		}
	}

	// A flipped block-CRC bit: header (4) + block magic (6) put the
	// CRC at bytes 10..13; corrupting it must fail the CRC check.
	corrupt := append([]byte(nil), valid...)
	corrupt[11] ^= 0x01
	if got := bzip2.Expand(corrupt); got != nil {
		t.Error("corrupt block CRC: Expand did not return nil")
	}

	// A flipped byte in the final CRC.
	corrupt = append([]byte(nil), valid...)
	corrupt[len(corrupt)-2] ^= 0x80
	if got := bzip2.Expand(corrupt); got != nil {
		t.Error("corrupt final CRC: Expand did not return nil")
	}
}

// TestTruncated: every proper prefix is nil or a clean prefix of the
// plaintext — never garbage.
func TestTruncated(t *testing.T) {
	data := []byte(strings.Repeat("the quick brown fox jumps over the lazy dog", 50))
	z := bzip2.Compress(data)
	for cut := 1; cut < len(z); cut++ {
		got := bzip2.Expand(z[:cut])
		if got != nil && !bytes.HasPrefix(data, got) {
			t.Fatalf("cut %d: non-prefix output (%d bytes)", cut, len(got))
		}
	}
}

func expectPanic(t *testing.T, level int) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("CompressLevel(%d) did not panic", level)
		}
		msg, ok := r.(string)
		if !ok || !strings.Contains(msg, "bzip2.CompressLevel") || !strings.Contains(msg, "want 1..9") {
			t.Fatalf("panic %v lacks method name or fix", r)
		}
	}()
	bzip2.CompressLevel([]byte("x"), level)
}

func TestPanics(t *testing.T) {
	expectPanic(t, 0)
	expectPanic(t, 10)
}

// TestRandomizedModel is the seeded property test: mixed-shape
// inputs through both levels with a stdlib cross-check.
func TestRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 42))
	for i := 0; i < 60; i++ {
		n := rng.IntN(5000)
		data := make([]byte, n)
		for j := range data {
			switch rng.IntN(4) {
			case 0:
				data[j] = byte(rng.IntN(256))
			case 1:
				data[j] = 'a' + byte(rng.IntN(4))
			case 2:
				data[j] = 0
			default:
				data[j] = data[max(0, j-1)] // runs
			}
		}
		level := 1 + 8*int(rng.IntN(2))
		z := bzip2.CompressLevel(data, level)
		if back := bzip2.Expand(z); !bytes.Equal(back, data) {
			t.Fatalf("iter %d level %d: mismatch (%d vs %d bytes)", i, level, len(back), len(data))
		}
		r := stdbz.NewReader(bytes.NewReader(z))
		if got, err := io.ReadAll(r); err != nil || !bytes.Equal(got, data) {
			t.Fatalf("iter %d level %d: stdlib mismatch: %v", i, level, err)
		}
	}
}

// TestTarBz2RoundTrip is the .tar.bz2 pipeline — the archive form
// this package exists for, the same shape as the tar.gz test in
// pluto/gzip.
func TestTarBz2RoundTrip(t *testing.T) {
	files := []struct {
		name, body string
	}{
		{"hello.txt", "Hello, compressed world!\n"},
		{"dir/data.bin", string(bytes.Repeat([]byte{0xAB, 0x00, 0xCD}, 500))},
		{"big.txt", strings.Repeat("the quick brown fox\n", 2000)},
	}

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for _, f := range files {
		hdr := &tar.Header{Name: f.name, Mode: 0o644, Size: int64(len(f.body))}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(f.body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	tarBytes := buf.Bytes()

	z := bzip2.Compress(tarBytes)
	if back := bzip2.Expand(z); !bytes.Equal(back, tarBytes) {
		t.Fatalf("tar.bz2 round trip: %d back, want %d", len(back), len(tarBytes))
	}

	tr := tar.NewReader(bytes.NewReader(bzip2.Expand(z)))
	i := 0
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("tar read: %v", err)
		}
		body, err := io.ReadAll(tr)
		if err != nil {
			t.Fatalf("tar body: %v", err)
		}
		if hdr.Name != files[i].name || string(body) != files[i].body {
			t.Fatalf("file %d mismatch: %q", i, hdr.Name)
		}
		i++
	}
	if i != len(files) {
		t.Fatalf("read %d files, want %d", i, len(files))
	}
}
