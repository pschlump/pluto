/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package compress_z_test

import (
	"archive/tar"
	"bytes"
	"encoding/hex"
	"io"
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/pschlump/pluto/compress_z"
)

// CLI golden vectors, generated with compress(1) on 2026-11-12
// (macOS BSD compress, the FreeBSD zopen.c lineage):
//
//	printf 'TOBEORNOTTOBEORTOBEORNOT' | compress -b N -c
//	head -c 2000 /usr/share/dict/words | compress -b N -c
//
// for N in 9, 12 and 16.  These inputs sit under the 10000-byte
// CHECK_GAP, so the CLI emits no CLEARs here and its bytes are the
// pure greedy-LZW stream our encoder must reproduce exactly —
// including the width machine's -b9 overshoot (dict.b9) and the
// clamp-on-growth-into-maxbits path (dict.b12).
const (
	goldenTobe9Hex  = "1f9d89549e0829f2448a932754020e2ca890a04184"
	goldenTobe12Hex = "1f9d8c549e0829f2448a932754020e2ca890a04184"
	goldenTobe16Hex = "1f9d90549e0829f2448a932754020e2ca890a04184"

	goldenDict9Hex  = "1f9d894114841138304c183604d9a44943b08d822061dc303428878c9d3072d610ac78e70d1b330f31be711352cec834634a9e1c7350a5443a655ca6a183b2649d8762048a191864a7183265d0e00c23a64ec19d29898661a9736918984d59e6899ac6cdd3984ac7547d9a6624d53963eaccf94a27aac6ac34ad2244bb950d9ba96cbda27d23276a1d3662ed8eed1986cc4cac3bcd80546aa6acd23473228e011c067199ba87e7cc2943a6a9c23256a12abd9cb92bc9cd2363f26d73344c1b388f19bb212377e7ea9144d9a866ed8672d3d7b5c9bcbd4dbbccecd0905dd36e83b92c5f89298fc31cab140ecd3974bcf295635acc9ba17210379543078dc939da9572477917e3769a61d9706d1d86f9cec94d2757562a7f377dcab525c72f43dc8de1f78f0d35c71913bd27147d68d8461f1a94d9f71e1af895a11f7d68c8a61f7d68c826997defa1d1df7feda5511a7849bda7115f9255151f1d18f9a5076374c4c698663bc1d4df7f5705c7957b5cc9d754749fed145d704f4597465d7cd161545374d4210763763086471a2d2985471ab235a5076039f5b4934e48bd419c7b484dd5d34e7e01a5d37b3abd379118ef8187d37be001a5d34e34b544d44ee8b544941865e847941865c025861865e4011d65a28921461a8059f79f75748c019718d6d1e1c65d0ae545947574cc81461a70e824861c65d891c6553a8921471976a471957d628821471976a4b1de67628821471976a4f1145d3a8921471976a4f1145d708921861c65d891c65375c8219a1862c025c6186fdcf5461b391145464b4491f1144e406517114e4065a7d94f6540a7935f2c89219b4e7eb1e49f4e7eb1a4d94f28adf7d94f28716507607eb1145d5d3df955c65a3fcd94861d80f9159d1c70fdf4467f3ab1d6465b38b1d6465b65b8c75a1b6da511914eacb5d1967d3fbdd146556f4404537674e9c45a1b55bdc15245692ca5136b6d54f5c662fe95019e7bacb551d51b63e4015d4d44b1d646556f9c81d94c2d11c55a1b55bd81461ed0a53646746dc0f5d31b6d54f5061a7940f7d81bd1b501d74f6fb451d51b70bc01dd635d05c75a1b55bd01d61ba8e9c45a1b55bd01d61b70c0f5d31b6d54f54677742d551351acb551d51b7911c55a1b55bd71d1196de9c45a1b55bd614719e0b1b4d64f752ca61319752ce6867b64d4b1987f3a9151c718ff9151c71834b546461d634457574f8095619a4e659847541918290658198b9181511a11e954c662646094461d0e1155c6626460049718b61155461a6708d553196b8951866c3895a1104f6294a150443895a1d0623c895186429a8951864250f554061b6dbc01161a791155061b6dbc3187463d95c1c64911e954061b74c8e11b4e982d2607756794e15e718fd544541970cc84461e6da4c193188f0185994e8f01654755a289f1186d694484d363691cf518756e2ca6d363d4b931065c623c469d7f3a3d469d66623c461d4dad3d461d4db0ad25c663d44517dc6326110753708f7d37c61ba8e9f4581d6eb0145d7065fc57061dfdfd57061d74b44454197444175c191785b5de67629401971865e4a1186065e4a1185c6294914744ff9981515f3aa181991c70898186636b1c85c61b6d6c25866c3aa141974e68d02507668be984065d726036065c62a041971cc5e984065d7214679f1868d025c7633aa141577662c8a6131a"
	goldenDict12Hex = "1f9d8c4114841138304c183604d9a44943b08d822061dc303428878c9d3072d610ac78e70d1b330f31be711352cec834634a9e1c7350a5443a655ca6a183b2649d8762048a191864a7183265d0e00c23a64ec19d29898661a9736918984d59e6899ac6cdd3984ac7547d9a6624d53963eaccf94a27aac6ac34ad2244bb950d9ba96cbda27d23276a1d3662ed8eed1986cc4cac3bcd80546aa6acd23473228e011c067199ba87e7cc2943a6a9c23256a12abd9cb92bc9cd2363f26d73344c1b388f19bb212377e7ea9144d9a866ed8672d3d7b5c9bcbd4dbbccecd0905dd36e83b92c5f89298fc31cab140ecd3974bcf295635acc9ba17210379543078dc939da9572477917e3769a61d9706d1d86f9cec94d2757562a7f377dcab525c72f43dc8de1f78f0d35c71913bd27147d68d8862065f619885f19fa21285b84064e384783eda1d1df7feda5511a7849bda7115f9255151f1d18f9a50763288a215b539aed04d3863006a8d473303a369f8cec4517dc53d1a551175f741805631d72306607637878b8d64e4dbea8d48a3ae5d4d34e552ef50671ee2135d5957da50154965d7648267838bd07de9844c9e86496e8b5d426845d9601971862d8091d65a2e1990660d6fd17e8187706eac65d0ae5d5e61b74cc81461a705499a41d1ec628c6a49596d1e0a56550bade677862fa145d92765a295d858a5a649269de39c61b776d99135164c8f9d3533801955d44b93ef6974e4041072c4ab101e6174bfe0dcb528cc77eaa2c574b3eeb63ae7f3ee9174dd1d23a13aac06e8959b76db495eb966d41086e5b1e82ca5ab86a35b86e556f44045376a4d24aae1b6fb054918721be8bef62fe4118de4ff7e69b077435d9cbee1b67603693adfebe81c6c1f3963146746ddc19f1c47b9a84b1c605c3f146c75d051731586fa0762ebe28c301f2c2ddd1b554c2042fac68cdf05e44a05a2bbfb1248810d7b118b04263e61e1945274babd0ff217db167445f4c6a4f8095619a4e560797b562552f56ebaea05a4c194669d4e11051627f7da78279a6718650543fa9699f9a7a8853ddbcc68d92d5772b1463dd50c5ddc6c863a071b3a683cf31629e6c9cc42bda6cd09124dd6e2c2607750dbb579caf219601c74c1387cb539e15f91676e994d67677458e9344f5ae585fce75ecd455beb6ec4a934edddfb2d3d4da63bb7b662bf0a36a7db9b7f3d24e78ca55cb5147e5c563fd5f1974d088361d28ca8d7dbd795e1496b368af9dc7ece12b26bebc3a9981515f3a25e8861c773e3ad91a47a1716fb1ed736fffe5980d4dd4fe49b25dfef8973b0016a741060c0e00d3e022c0a001"
	goldenDict16Hex = "1f9d904114841138304c183604d9a44943b08d822061dc303428878c9d3072d610ac78e70d1b330f31be711352cec834634a9e1c7350a5443a655ca6a183b2649d8762048a191864a7183265d0e00c23a64ec19d29898661a9736918984d59e6899ac6cdd3984ac7547d9a6624d53963eaccf94a27aac6ac34ad2244bb950d9ba96cbda27d23276a1d3662ed8eed1986cc4cac3bcd80546aa6acd23473228e011c067199ba87e7cc2943a6a9c23256a12abd9cb92bc9cd2363f26d73344c1b388f19bb212377e7ea9144d9a866ed8672d3d7b5c9bcbd4dbbccecd0905dd36e83b92c5f89298fc31cab140ecd3974bcf295635acc9ba17210379543078dc939da9572477917e3769a61d9706d1d86f9cec94d2757562a7f377dcab525c72f43dc8de1f78f0d35c71913bd27147d68d8862065f619885f19fa21285b84064e384783eda1d1df7feda5511a7849bda7115f9255151f1d18f9a50763288a215b539aed04d3863006a8d473303a369f8cec4517dc53d1a551175f741805631d72306607637878b8d64e4dbea8d48a3ae5d4d34e552ef50671ee2135d5957da50154965d7648267838bd07de9844c9e86496e8b5d426845d9601971862d8091d65a2e1990660d6fd17e8187706eac65d0ae5d5e61b74cc81461a705499a41d1ec628c6a49596d1e0a56550bade677862fa145d92765a295d858a5a649269de39c61b776d99135164c8f9d3533801955d44b93ef6974e4041072c4ab101e6174bfe0dcb528cc77eaa2c574b3eeb63ae7f3ee9174dd1d23a13aac06e8959b76db495eb966d41086e5b1e82ca5ab86a35b86e556f44045376a4d24aae1b6fb054918721be8bef62fe4118de4ff7e69b077435d9cbee1b67603693adfebe81c6c1f3963146746ddc19f1c47b9a84b1c605c3f146c75d051731586fa0762ebe28c301f2c2ddd1b554c2042fac68cdf05e44a05a2bbfb1248810d7b118b04263e61e1945274babd0ff217db167445f4c6a4f8095619a4e560797b562552f56ebaea05a4c194669d4e11051627f7da78279a6718650543fa9699f9a7a8853ddbcc68d92d5772b1463dd50c5ddc6c863a071b3a683cf31629e6c9cc42bda6cd09124dd6e2c2607750dbb579caf219601c74c1387cb539e15f91676e994d67677458e9344f5ae585fce75ecd455beb6ec4a934edddfb2d3d4da63bb7b662bf0a36a7db9b7f3d24e78ca55cb5147e5c563fd5f1974d088361d28ca8d7dbd795e1496b368af9dc7ece12b26bebc3a9981515f3a25e8861c773e3ad91a47a1716fb1ed736fffe5980d4dd4fe49b25dfef8973b0016a741060c0e00d3e022c0a001"
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

// TestRoundTripAllWidths drives every corpus entry through every
// width the format has, including the three overshoot widths.
func TestRoundTripAllWidths(t *testing.T) {
	for i, data := range corpus() {
		for bits := 9; bits <= 16; bits++ {
			z := compress_z.CompressBits(data, bits)
			if z == nil {
				t.Fatalf("corpus %d bits %d: CompressBits returned nil", i, bits)
			}
			if back := compress_z.Expand(z); !bytes.Equal(back, data) {
				t.Fatalf("corpus %d bits %d: round trip mismatch (%d back)", i, bits, len(back))
			}
		}
	}
}

// TestCLIGoldensDecode proves our decoder reads real compress(1)
// output byte-exact at narrow, clamping and overshoot widths.
func TestCLIGoldensDecode(t *testing.T) {
	for name, g := range map[string]string{
		"tobe9":  goldenTobe9Hex,
		"tobe12": goldenTobe12Hex,
		"tobe16": goldenTobe16Hex,
	} {
		if got := compress_z.Expand(hexBytes(t, g)); string(got) != "TOBEORNOTTOBEORTOBEORNOT" {
			t.Fatalf("golden %s: got %q", name, got)
		}
	}
	dict := compress_z.Expand(hexBytes(t, goldenDict9Hex))
	if len(dict) != 2000 || dict[0] != 'A' {
		t.Fatalf("golden dict9: got %d bytes starting %q", len(dict), dict[:4])
	}
	for name, g := range map[string]string{
		"dict12": goldenDict12Hex,
		"dict16": goldenDict16Hex,
	} {
		if got := compress_z.Expand(hexBytes(t, g)); !bytes.Equal(got, dict) {
			t.Fatalf("golden %s: decode differs from dict9's plaintext", name)
		}
	}
}

// TestCLIGoldensByteIdentical re-encodes the golden plaintexts and
// demands byte-for-byte equality with compress(1) — the encoder is
// the reference encoder, not merely decoder-compatible.
func TestCLIGoldensByteIdentical(t *testing.T) {
	tobe := []byte("TOBEORNOTTOBEORTOBEORNOT")
	dict := compress_z.Expand(hexBytes(t, goldenDict9Hex))
	for name, tc := range map[string]struct {
		data []byte
		bits int
		want string
	}{
		"tobe9":  {tobe, 9, goldenTobe9Hex},
		"tobe12": {tobe, 12, goldenTobe12Hex},
		"tobe16": {tobe, 16, goldenTobe16Hex},
		"dict9":  {dict, 9, goldenDict9Hex},
		"dict12": {dict, 12, goldenDict12Hex},
		"dict16": {dict, 16, goldenDict16Hex},
	} {
		got := compress_z.CompressBits(tc.data, tc.bits)
		if want := hexBytes(t, tc.want); !bytes.Equal(got, want) {
			t.Fatalf("golden %s: re-encode differs from compress(1) (%d vs %d bytes)",
				name, len(got), len(want))
		}
	}
}

// TestOvershootWidths pins the format's sharpest quirk: at -b9..-b11
// the table fills while maxcode is still 2^maxbits-1, so the width
// grows one bit PAST maxbits and stays there.  The streams round
// trip through our own decoder (the BSD tools cannot read their own
// output at these widths).
func TestOvershootWidths(t *testing.T) {
	data := bytes.Repeat([]byte("abcdefgh"), 512) // enough to fill a b9 table
	for _, bits := range []int{9, 10, 11} {
		z := compress_z.CompressBits(data, bits)
		if z[2] != byte(bits)|0x80 {
			t.Fatalf("bits %d: header byte %#x", bits, z[2])
		}
		if back := compress_z.Expand(z); !bytes.Equal(back, data) {
			t.Fatalf("bits %d: overshoot round trip mismatch", bits)
		}
	}
}

// lsbPacker is a minimal LSB-first bit writer for hand-crafted
// streams (the same convention as the format: first bit of a code in
// the 0x01 position of the next byte).
type lsbPacker struct {
	buf []byte
	acc uint32
	n   uint
}

func (p *lsbPacker) write(code uint32, n uint) {
	p.acc |= code << p.n
	p.n += n
	for p.n >= 8 {
		p.buf = append(p.buf, byte(p.acc))
		p.acc >>= 8
		p.n -= 8
	}
}

func (p *lsbPacker) bytes() []byte {
	if p.n > 0 {
		p.buf = append(p.buf, byte(p.acc))
	}
	return p.buf
}

func pack(codes []uint32, n uint) []byte {
	var p lsbPacker
	for _, c := range codes {
		p.write(c, n)
	}
	return p.bytes()
}

// TestClearStream crafts a block-mode stream with a mid-stream CLEAR
// — the one table-reset path the sub-CHECK_GAP CLI goldens cannot
// cover — and proves Expand handles the group padding, the width
// reset and the no-add first code after the reset.  The CLEAR fires
// early, far from any width growth, so only the CLEAR path is
// exercised (growth alignment is covered by the goldens).
func TestClearStream(t *testing.T) {
	input := []byte("ababababababababXYZXYZXYZXYZ" + strings.Repeat("Q", 40))

	// Hand encoder: greedy LZW that emits CLEAR after the 4th code,
	// padding to the 8-code group boundary exactly like zopen.c.
	var p lsbPacker
	out := []byte{0x1f, 0x9d, 0x90}
	p.buf = out
	emitted := 0
	data := input
	for len(data) > 0 {
		tab := map[uint64]uint32{}
		free := uint32(257)
		ent := uint32(data[0])
		i := 1
		for i < len(data) {
			c := data[i]
			key := uint64(ent)<<8 | uint64(c)
			if code, ok := tab[key]; ok {
				ent = code
				i++
				continue
			}
			p.write(ent, 9)
			emitted++
			if emitted == 4 {
				p.write(256, 9) // CLEAR at the current width...
				// ...then zero-pad to the 9-bit 8-code group boundary
				// (measured from the end of the 3-byte header).
				for (len(p.buf)*8+int(p.n)-24)%72 != 0 {
					p.write(0, 1)
				}
				data = data[i:] // restart greedy from the next byte
				break
			}
			if free < 512 {
				tab[key] = free
				free++
			}
			ent = uint32(c)
			i++
		}
		if i >= len(data) && emitted != 4 {
			p.write(ent, 9)
			break
		}
	}
	if got := compress_z.Expand(p.bytes()); !bytes.Equal(got, input) {
		t.Fatalf("clear stream: got %d bytes, want %d", len(got), len(input))
	}
}

// TestKwKwKSpecialCase: the encoding of "aaa" is [97, 257] — the
// second code is the entry being defined right now, finished with
// the prior byte.
func TestKwKwKSpecialCase(t *testing.T) {
	stream := append([]byte{0x1f, 0x9d, 0x90}, pack([]uint32{97, 257}, 9)...)
	if got := compress_z.Expand(stream); string(got) != "aaa" {
		t.Fatalf("KwKwK: got %q", got)
	}
	if got := compress_z.Compress([]byte("aaa")); !bytes.Equal(got, stream) {
		t.Fatalf("Compress(aaa) = %x, want %x", got, stream)
	}
}

// TestNonBlockMode: header flag 0x00 — free starts at 256, code 256
// is an ordinary (not-yet-defined) entry, and a literal-only stream
// is valid.
func TestNonBlockMode(t *testing.T) {
	stream := append([]byte{0x1f, 0x9d, 0x10}, pack([]uint32{65, 66}, 9)...)
	if got := compress_z.Expand(stream); string(got) != "AB" {
		t.Fatalf("non-block: got %q", got)
	}
}

func TestMalformed(t *testing.T) {
	cases := []struct {
		name string
		z    []byte
	}{
		{"nil", nil},
		{"empty", []byte{}},
		{"one byte", []byte{0x1f}},
		{"two bytes", []byte{0x1f, 0x9d}},
		{"bad magic 0", []byte{0x2f, 0x9d, 0x90, 0x00, 0x00}},
		{"bad magic 1", []byte{0x1f, 0x8c, 0x90, 0x00, 0x00}},
		{"maxbits 8", append([]byte{0x1f, 0x9d, 0x88}, pack([]uint32{65}, 9)...)},
		{"maxbits 17", append([]byte{0x1f, 0x9d, 0x91}, pack([]uint32{65}, 9)...)},
		{"maxbits 31", append([]byte{0x1f, 0x9d, 0x9f}, pack([]uint32{65}, 9)...)},
		{"first code non-literal", append([]byte{0x1f, 0x9d, 0x90}, pack([]uint32{257}, 9)...)},
		{"code above table", append([]byte{0x1f, 0x9d, 0x90}, pack([]uint32{97, 98, 300}, 9)...)},
		{"kwkwk without oldcode", append([]byte{0x1f, 0x9d, 0x90}, pack([]uint32{97, 258}, 9)...)},
		{"non-block code 256 first", append([]byte{0x1f, 0x9d, 0x10}, pack([]uint32{256}, 9)...)},
	}
	for _, tc := range cases {
		if got := compress_z.Expand(tc.z); got != nil {
			t.Errorf("%s: Expand returned %d bytes, want nil", tc.name, len(got))
		}
	}
}

// TestTruncated: a chopped stream must yield either nil or a clean
// prefix of the plaintext — never garbage beyond it.
func TestTruncated(t *testing.T) {
	data := []byte(strings.Repeat("the quick brown fox jumps over the lazy dog", 50))
	z := compress_z.Compress(data)
	for chop := 1; chop <= 8 && chop < len(z); chop++ {
		got := compress_z.Expand(z[:len(z)-chop])
		if got != nil && !bytes.HasPrefix(data, got) {
			t.Fatalf("chop %d: non-prefix output (%d bytes)", chop, len(got))
		}
	}
}

func TestEmptyStream(t *testing.T) {
	// Documented deviation: compress(1) writes a 0-byte file for
	// empty input; we write a valid header-only stream instead.
	z := compress_z.Compress(nil)
	if !bytes.Equal(z, []byte{0x1f, 0x9d, 0x90}) {
		t.Fatalf("empty: got %x", z)
	}
	if got := compress_z.Expand(z); len(got) != 0 {
		t.Fatalf("empty: decoded %d bytes", len(got))
	}
	if got := compress_z.Expand(nil); got != nil {
		t.Fatal("Expand(nil) not nil")
	}
}

func expectPanic(t *testing.T, bits int) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("CompressBits(%d) did not panic", bits)
		}
		msg, ok := r.(string)
		if !ok || !strings.Contains(msg, "compress_z.CompressBits") || !strings.Contains(msg, "want 9..16") {
			t.Fatalf("panic %v lacks method name or fix", r)
		}
	}()
	compress_z.CompressBits([]byte("x"), bits)
}

func TestPanics(t *testing.T) {
	expectPanic(t, 8)
	expectPanic(t, 17)
	expectPanic(t, 0)
}

// TestRandomizedModel is the seeded property test: biased-random
// inputs (small alphabet to force long matches and table growth)
// through a rotating width, fixed seed 42.
func TestRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 42))
	for i := 0; i < 300; i++ {
		n := rng.IntN(4000)
		data := make([]byte, n)
		for j := range data {
			if rng.IntN(3) == 0 {
				data[j] = byte(rng.IntN(256))
			} else {
				data[j] = byte('a' + rng.IntN(6))
			}
		}
		bits := 9 + i%8
		z := compress_z.CompressBits(data, bits)
		if back := compress_z.Expand(z); !bytes.Equal(back, data) {
			t.Fatalf("iter %d bits %d: mismatch (%d vs %d bytes)", i, bits, len(back), len(data))
		}
	}
}

// TestTarZRoundTrip is the .tar.Z pipeline — the archive form this
// package exists for, the same shape as the tar.gz test in pluto/gzip.
func TestTarZRoundTrip(t *testing.T) {
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

	z := compress_z.Compress(tarBytes)
	if back := compress_z.Expand(z); !bytes.Equal(back, tarBytes) {
		t.Fatalf("tar.Z round trip: %d back, want %d", len(back), len(tarBytes))
	}

	tr := tar.NewReader(bytes.NewReader(compress_z.Expand(z)))
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
