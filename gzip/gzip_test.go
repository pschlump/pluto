/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package gzip_test

import (
	"archive/tar"
	"bytes"
	stdgzip "compress/gzip"
	"encoding/hex"
	"io"
	"math/rand/v2"
	"strings"
	"testing"
	"time"

	"github.com/pschlump/pluto/gzip"
)

// CLI golden vectors, generated with gzip(1) on 2026-11-12 (macOS
// BSD gzip; -n suppresses name/timestamp so the bytes are stable):
//
//	printf 'TOBEORNOTTOBEORTOBEORNOT' | gzip -9 -n
//	printf '' | gzip -n
//	head -c 2000 /usr/share/dict/words | gzip -6 -n
const (
	goldenTobeHex  = "1f8b08000000000002030bf17772f50ff2f30f09013342605c00f14e3d2d18000000"
	goldenEmptyHex = "1f8b080000000000000303000000000000000000"
	goldenDictHex  = "1f8b08000000000000035d556d72e33008fd9f5be528b2456326b294917076bda75f7820a7ed4c0b4f08f16d72bfa55bb2bf62ffcc4af7db3d55033dbf537f02fc69e54bc5bd55a7bc4eae0f1d094de097c7edbedcd29294a525d3067e2413ad200e93be033fc1b84e8121567fc0633d4620017f828a2a950f2a67e07826ad831f255e2bd32832bb8baf2f5018e491ea4a8ea83b1f940d14a61994c3305f5a2533b72391fd451d3a35fbb5f1b4944be6c61c79a0c09ffb3ed14e55cc30aaac4c08d1bf4493b7062ca9a77d69008cab2e5b6f6362d68453ff8667c4b81fe43407f35814541aa1800080ba39190fc669731a2fb7ebe9f6eded567ea0a97159e4e40c23309e303fd8a3131d34fe87f064564e824e03e26512cf559947239e60345ca43122179f3739bc356fd0bfec33f397dd053c2eaabe98b24e49db51704527a4993354924b073b537e771ed7e2860dac8ebc160b9d4e8750b69159180f9a385d4fe7f5288531a97a181bbf0c757ab3576142d4340eded83820f90b7fd3b2f4d5ab49d67694b6db879911624e366a993aa71a00ce32e143cb9a8977c2509d22999208c0e1fb9236ab7e668207be6e22aaac05aecef1fdde2742d9035b3cd7a19c1337fb1c3a235517ac363558297126fb60787c045a77fedc3f485794e78ef3666de9a4cb623f7f09db0fd9ab8d5faec7da5ef4e3f4d196ad755d571fbfc727a2777ac4e28ab3c6eb13930f2c2163754c200e268b921fab97d9f44997b6d1ee345619ad9453f742063ef60b5ba8f878881fb69cd12c2a04c8295875be5208d079e776de35eb0d99013f21d4df00f759a413f4349eded303fd2534d0ea422f96eddcdd57cf84a130f0666c56856eca2016875989dc80ce892480ccbba8d284282e0e689f229d41df25866723a91f759d3ab029b178487c6694fbed5b7f4d2e2f88e39ca19d33b2d303fbd28e58a137aaa8faa65beb69e96c986effbe369855dac9cd043c2f281f542ee97cc461e53f91c1f724d0070000"
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
		bytes.Repeat([]byte("a"), 100000),
		text,
		random,
	}
}

func TestRoundTrip(t *testing.T) {
	for i, data := range corpus() {
		gz := gzip.Compress(data)
		if gz == nil {
			t.Fatalf("corpus %d: Compress returned nil", i)
		}
		if back := gzip.Expand(gz); !bytes.Equal(back, data) {
			t.Fatalf("corpus %d: round trip mismatch (%d back)", i, len(back))
		}
	}
}

func TestCLIGoldens(t *testing.T) {
	if got := gzip.Expand(hexBytes(t, goldenTobeHex)); string(got) != "TOBEORNOTTOBEORTOBEORNOT" {
		t.Fatalf("golden TOBEORNOT: got %q", got)
	}
	if got := gzip.Expand(hexBytes(t, goldenEmptyHex)); len(got) != 0 {
		t.Fatalf("golden empty: got %d bytes", len(got))
	}
	dict := gzip.Expand(hexBytes(t, goldenDictHex))
	if len(dict) != 2000 || dict[0] != 'A' {
		t.Fatalf("golden dict: got %d bytes starting %q", len(dict), dict[:4])
	}
	// The stdlib must decode the same golden to the same bytes.
	r, err := stdgzip.NewReader(bytes.NewReader(hexBytes(t, goldenDictHex)))
	if err != nil {
		t.Fatalf("stdlib golden dict: %v", err)
	}
	std, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("stdlib golden dict read: %v", err)
	}
	if !bytes.Equal(std, dict) {
		t.Fatal("our golden-dict decode differs from stdlib's")
	}
}

// TestStdlibInterop is the oracle in both directions, including
// headers that use every optional field.
func TestStdlibInterop(t *testing.T) {
	for i, data := range corpus() {
		if got := stdGzip(t, data); !bytes.Equal(got, data) {
			t.Fatalf("corpus %d: stdlib rejected our member", i)
		}
	}
	for i, data := range corpus() {
		var buf bytes.Buffer
		w := stdgzip.NewWriter(&buf)
		w.Name = "corpus.bin"
		w.Comment = "pluto test"
		w.Extra = []byte{1, 2, 3, 4}
		w.ModTime = time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC)
		if _, err := w.Write(data); err != nil {
			t.Fatalf("corpus %d: %v", i, err)
		}
		if err := w.Close(); err != nil {
			t.Fatalf("corpus %d: %v", i, err)
		}
		if back := gzip.Expand(buf.Bytes()); !bytes.Equal(back, data) {
			t.Fatalf("corpus %d: our Expand mismatched stdlib member with Name/Comment/Extra", i)
		}
	}
}

func stdGzip(t *testing.T, data []byte) []byte {
	t.Helper()
	gz := gzip.Compress(data)
	r, err := stdgzip.NewReader(bytes.NewReader(gz))
	if err != nil {
		t.Fatalf("stdlib NewReader: %v", err)
	}
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("stdlib read: %v", err)
	}
	return out
}

func TestMultiMember(t *testing.T) {
	a := gzip.Compress([]byte("first member "))
	b := gzip.Compress([]byte("and the second"))
	joined := append(append([]byte{}, a...), b...)
	if got := gzip.Expand(joined); string(got) != "first member and the second" {
		t.Fatalf("multi-member: got %q", got)
	}
	// Also via stdlib output concatenated with ours.
	var buf bytes.Buffer
	w := stdgzip.NewWriter(&buf)
	if _, err := w.Write([]byte("X")); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	mixed := append(append([]byte{}, buf.Bytes()...), a...)
	if got := gzip.Expand(mixed); string(got) != "Xfirst member " {
		t.Fatalf("mixed multi-member: got %q", got)
	}
}

func TestFHCRC(t *testing.T) {
	// Craft a member with FHCRC from our own minimal member: set the
	// flag bit and insert the 2 low bytes of the header's CRC-32.
	base := gzip.Compress([]byte("header checksum please"))
	hdr := append([]byte{}, base[:10]...)
	hdr[3] |= 0x02
	crc := crc32(hdr)
	withHcrc := append(append(append([]byte{}, hdr...), byte(crc), byte(crc>>8)), base[10:]...)
	if got := gzip.Expand(withHcrc); string(got) != "header checksum please" {
		t.Fatalf("FHCRC member: got %q", got)
	}
	bad := append([]byte{}, withHcrc...)
	bad[10] ^= 0xFF
	if got := gzip.Expand(bad); got != nil {
		t.Fatalf("corrupt FHCRC accepted: %q", got)
	}
}

// crc32 is the IEEE CRC-32 of b (recomputed here only for crafting
// the FHCRC bytes; the package itself uses pluto/crc).
func crc32(b []byte) uint32 {
	c := ^uint32(0)
	for _, x := range b {
		c ^= uint32(x)
		for k := 0; k < 8; k++ {
			if c&1 != 0 {
				c = c>>1 ^ 0xEDB88320
			} else {
				c >>= 1
			}
		}
	}
	return ^c
}

func TestMalformed(t *testing.T) {
	base := gzip.Compress([]byte("some content to corrupt"))
	cases := []struct {
		name string
		in   []byte
	}{
		{"nil", nil},
		{"empty", []byte{}},
		{"short", base[:9]},
		{"bad-magic-1", func() []byte { b := append([]byte{}, base...); b[0] = 0x1e; return b }()},
		{"bad-magic-2", func() []byte { b := append([]byte{}, base...); b[1] = 0x8c; return b }()},
		{"bad-cm", func() []byte { b := append([]byte{}, base...); b[2] = 0x09; return b }()},
		{"reserved-flags", func() []byte { b := append([]byte{}, base...); b[3] = 0xE0; return b }()},
		{"fname-unterminated", func() []byte {
			b := append([]byte{}, base...)
			b[3] = 0x08 // FNAME, but no NUL anywhere after the header
			for i := 10; i < len(b); i++ {
				b[i] = 'x'
			}
			return b
		}()},
		{"fextra-overrun", func() []byte {
			b := append([]byte{}, base...)
			b[3] = 0x04
			b[10], b[11] = 0xFF, 0xFF // XLEN 65535
			return b
		}()},
		{"truncated-deflate", base[:14]},
		{"truncated-trailer", base[:len(base)-4]},
		{"crc-corrupt", func() []byte {
			b := append([]byte{}, base...)
			b[len(b)-6] ^= 0x01
			return b
		}()},
		{"isize-corrupt", func() []byte {
			b := append([]byte{}, base...)
			b[len(b)-1] ^= 0x01
			return b
		}()},
		{"trailing-junk", append(append([]byte{}, base...), 0x99)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := gzip.Expand(tc.in); got != nil {
				t.Fatalf("Expand accepted malformed input: %d bytes", len(got))
			}
		})
	}
}

// TestTarRoundTrip is the package's reason to exist: a tar archive
// gzipped here and read back by the stdlib, and the mirror image.
func TestTarRoundTrip(t *testing.T) {
	files := []struct {
		name, body string
	}{
		{"hello.txt", "Hello, compressed world!\n"},
		{"dir/data.bin", string(bytes.Repeat([]byte{0xAB, 0x00, 0xCD}, 500))},
		{"big.txt", strings.Repeat("the quick brown fox\n", 2000)},
	}

	buildTar := func() []byte {
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
		return buf.Bytes()
	}
	readTar := func(blob []byte) {
		tr := tar.NewReader(bytes.NewReader(blob))
		i := 0
		for {
			hdr, err := tr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("tar read: %v", err)
			}
			if hdr.Name != files[i].name {
				t.Fatalf("file %d: name %q, want %q", i, hdr.Name, files[i].name)
			}
			body, err := io.ReadAll(tr)
			if err != nil {
				t.Fatalf("tar body: %v", err)
			}
			if string(body) != files[i].body {
				t.Fatalf("file %d: body mismatch (%d bytes)", i, len(body))
			}
			i++
		}
		if i != len(files) {
			t.Fatalf("read %d files, want %d", i, len(files))
		}
	}

	tarBytes := buildTar()

	// Ours out, stdlib in.
	gz := gzip.Compress(tarBytes)
	zr, err := stdgzip.NewReader(bytes.NewReader(gz))
	if err != nil {
		t.Fatalf("stdlib gzip: %v", err)
	}
	back, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("stdlib gunzip: %v", err)
	}
	if !bytes.Equal(back, tarBytes) {
		t.Fatal("tar.gz round trip via stdlib gunzip mismatched")
	}

	// stdlib out, ours in.
	var sbuf bytes.Buffer
	sw := stdgzip.NewWriter(&sbuf)
	if _, err := sw.Write(tarBytes); err != nil {
		t.Fatal(err)
	}
	if err := sw.Close(); err != nil {
		t.Fatal(err)
	}
	if back := gzip.Expand(sbuf.Bytes()); !bytes.Equal(back, tarBytes) {
		t.Fatal("tar.gz round trip via our Expand mismatched")
	}
	readTar(back)
}
