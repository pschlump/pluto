/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package lz4_test

import (
	"archive/tar"
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"io"
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/pschlump/pluto/lz4"
)

// CLI golden vectors, generated with lz4 v1.10.0 on 2026-11-12:
//
//	printf 'TOBEORNOTTOBEORTOBEORNOT' | lz4 -z -c
//	printf '' | lz4 -z -c
//	head -c 2000 /usr/share/dict/words | lz4 -z -c            (64KB blocks)
//	head -c 2000 /usr/share/dict/words | lz4 -z -c -BX        (block checksums)
//	head -c 2000 /usr/share/dict/words | lz4 -z -c --content-size
//
// The CLI writes 64KB blocks (BD 0x40) while we write 4MB blocks (BD
// 0x70), so streams differ in the BD/HC pair and block boundaries —
// every golden still decodes byte-exact here, and our own streams
// are in turn verified by lz4(1) itself (`lz4 -t`, `lz4 -dc | cmp`,
// checked out-of-band; tests stay hermetic).
const (
	goldenEmptyHex  = "04224d186440a700000000055dcc02"
	goldenTobeHex   = "04224d186440a71600000092544f42454f524e4f54090090544f42454f524e4f54000000009d6e2342"
	goldenDictHex   = "04224d186440a71d05000050410a610a610300f0006c0a61616c69690a61616d0a41616e09006172647661726b0900a2776f6c660a4161726f6e0600246963080023616c0a002474650900021d00f101750a41620a6162610a416261626465680800207561130011630500020b00326174650e001179070024696e1100000a0032696f6e160043736375730a0011740800116b06005674696e616c0a00126c440005370031746f721d0023756c3e00207573860022646974002066661e0011665100516973616e638a003169736536005069737365642100346c69659500020b00035f00406c6f6e654f00116dc900416d7065724000226e64b5000008003561626c1400024d00001600226564a900010c00062000017600001400406d656e745900106e49015162616e7465fc00507074697374810171626172616d626f090011691b0064727468726f730c00516963756c614e00030c00041101012c001173720003dc0014738c00637365646e6573210001810001150010726500307367690801227368060002c90000080002c700000a00053c0024686c4600010a0004220005520011692d01217369030221736b5f00217369e601717374617264697a9f0014743a0102aa01247465410012749d011274ec00237469c80000180101070001f7004174746f69ba00127470022274759b0111765c0022786934022278699d0101720050620a4162629c00516261636f6d71014162616379160030646964390002400100bd000207002173691e002473731f00127459000009001163630012620c012062659802706265797374656433022162695000106f650141626f74631f00806f746e756c6c6975840170626f7473686970680042726576691303040b00015401030d00028b02030d0001ee02050c0001550002330000ff0030416262120060636f756c6f6d99031164a3002064616b026364657269616e0900017301306465739e00336469638a01000a00146e13000223002469638100001d002174695f01010b00106f7e01306469659c032564691a0003960040646f6d65af0131646f6dc00314410a00017701030c00009000050d0001f300011900106f0f03217269eb00021100106336021061500203100010653103012002031100317973747002020f005267656e6974fc00008000206f68970160726563746f6d2101022500021500196f14003b706f737b004073636f70ca01023400000e000642006774686f726163780001d901022a0031766167f900051000216573490541616264758504000700106e0902010900017601247563070002f40130647563ee0220416281003165616d060000fe0233656172cb04516563656461d901050c001175280001180000ab0001cb03306569679d0501ad0120656c2a0230656c69b70500070000150253656c696365110001fa01020800001900516d6f736368da00010c0000d00340656c6f6e3800001300317472652800716e636572726167e001009f0111721801726570697468796d6c00317264651502616572646576695a05326572644b00000b0001390400460002a705020a0001c4002272729902001300029f00247272990201140022696f2606010d00013a02626572726f6d654c04000c0000bb010148014472756e63240000700123657479042265743402206574760171616265766163755b000079010188001479770100090004120000450040666172614a064068656e72180050686973656b6c04106881020360031168990052686f727265e200030b00013100011600004b0002150011741003010c00008c000009008069626c650a6162680000000027e46805"
	goldenDictBCHex = "04224d187440bd1d05000050410a610a610300f0006c0a61616c69690a61616d0a41616e09006172647661726b0900a2776f6c660a4161726f6e0600246963080023616c0a002474650900021d00f101750a41620a6162610a416261626465680800207561130011630500020b00326174650e001179070024696e1100000a0032696f6e160043736375730a0011740800116b06005674696e616c0a00126c440005370031746f721d0023756c3e00207573860022646974002066661e0011665100516973616e638a003169736536005069737365642100346c69659500020b00035f00406c6f6e654f00116dc900416d7065724000226e64b5000008003561626c1400024d00001600226564a900010c00062000017600001400406d656e745900106e49015162616e7465fc00507074697374810171626172616d626f090011691b0064727468726f730c00516963756c614e00030c00041101012c001173720003dc0014738c00637365646e6573210001810001150010726500307367690801227368060002c90000080002c700000a00053c0024686c4600010a0004220005520011692d01217369030221736b5f00217369e601717374617264697a9f0014743a0102aa01247465410012749d011274ec00237469c80000180101070001f7004174746f69ba00127470022274759b0111765c0022786934022278699d0101720050620a4162629c00516261636f6d71014162616379160030646964390002400100bd000207002173691e002473731f00127459000009001163630012620c012062659802706265797374656433022162695000106f650141626f74631f00806f746e756c6c6975840170626f7473686970680042726576691303040b00015401030d00028b02030d0001ee02050c0001550002330000ff0030416262120060636f756c6f6d99031164a3002064616b026364657269616e0900017301306465739e00336469638a01000a00146e13000223002469638100001d002174695f01010b00106f7e01306469659c032564691a0003960040646f6d65af0131646f6dc00314410a00017701030c00009000050d0001f300011900106f0f03217269eb00021100106336021061500203100010653103012002031100317973747002020f005267656e6974fc00008000206f68970160726563746f6d2101022500021500196f14003b706f737b004073636f70ca01023400000e000642006774686f726163780001d901022a0031766167f900051000216573490541616264758504000700106e0902010900017601247563070002f40130647563ee0220416281003165616d060000fe0233656172cb04516563656461d901050c001175280001180000ab0001cb03306569679d0501ad0120656c2a0230656c69b70500070000150253656c696365110001fa01020800001900516d6f736368da00010c0000d00340656c6f6e3800001300317472652800716e636572726167e001009f0111721801726570697468796d6c00317264651502616572646576695a05326572644b00000b0001390400460002a705020a0001c4002272729902001300029f00247272990201140022696f2606010d00013a02626572726f6d654c04000c0000bb010148014472756e63240000700123657479042265743402206574760171616265766163755b000079010188001479770100090004120000450040666172614a064068656e72180050686973656b6c04106881020360031168990052686f727265e200030b00013100011600004b0002150011741003010c00008c000009008069626c650a6162683bcb8ed10000000027e46805"
	goldenDictCSHex = "04224d186c40d007000000000000f01d05000050410a610a610300f0006c0a61616c69690a61616d0a41616e09006172647661726b0900a2776f6c660a4161726f6e0600246963080023616c0a002474650900021d00f101750a41620a6162610a416261626465680800207561130011630500020b00326174650e001179070024696e1100000a0032696f6e160043736375730a0011740800116b06005674696e616c0a00126c440005370031746f721d0023756c3e00207573860022646974002066661e0011665100516973616e638a003169736536005069737365642100346c69659500020b00035f00406c6f6e654f00116dc900416d7065724000226e64b5000008003561626c1400024d00001600226564a900010c00062000017600001400406d656e745900106e49015162616e7465fc00507074697374810171626172616d626f090011691b0064727468726f730c00516963756c614e00030c00041101012c001173720003dc0014738c00637365646e6573210001810001150010726500307367690801227368060002c90000080002c700000a00053c0024686c4600010a0004220005520011692d01217369030221736b5f00217369e601717374617264697a9f0014743a0102aa01247465410012749d011274ec00237469c80000180101070001f7004174746f69ba00127470022274759b0111765c0022786934022278699d0101720050620a4162629c00516261636f6d71014162616379160030646964390002400100bd000207002173691e002473731f00127459000009001163630012620c012062659802706265797374656433022162695000106f650141626f74631f00806f746e756c6c6975840170626f7473686970680042726576691303040b00015401030d00028b02030d0001ee02050c0001550002330000ff0030416262120060636f756c6f6d99031164a3002064616b026364657269616e0900017301306465739e00336469638a01000a00146e13000223002469638100001d002174695f01010b00106f7e01306469659c032564691a0003960040646f6d65af0131646f6dc00314410a00017701030c00009000050d0001f300011900106f0f03217269eb00021100106336021061500203100010653103012002031100317973747002020f005267656e6974fc00008000206f68970160726563746f6d2101022500021500196f14003b706f737b004073636f70ca01023400000e000642006774686f726163780001d901022a0031766167f900051000216573490541616264758504000700106e0902010900017601247563070002f40130647563ee0220416281003165616d060000fe0233656172cb04516563656461d901050c001175280001180000ab0001cb03306569679d0501ad0120656c2a0230656c69b70500070000150253656c696365110001fa01020800001900516d6f736368da00010c0000d00340656c6f6e3800001300317472652800716e636572726167e001009f0111721801726570697468796d6c00317264651502616572646576695a05326572644b00000b0001390400460002a705020a0001c4002272729902001300029f00247272990201140022696f2606010d00013a02626572726f6d654c04000c0000bb010148014472756e63240000700123657479042265743402206574760171616265766163755b000079010188001479770100090004120000450040666172614a064068656e72180050686973656b6c04106881020360031168990052686f727265e200030b00013100011600004b0002150011741003010c00008c000009008069626c650a6162680000000027e46805"

	// Our own empty frame, byte for byte: pins the header checksum
	// (0xb9 = XXH32(FLG 0x64, BD 0x70) >> 8 — the magic is NOT part
	// of the checksummed header) and the content checksum tail, which
	// is the XXH32 empty-input vector 0x02CC5D05.
	ourEmptyHex = "04224d186470b900000000055dcc02"
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
	random := make([]byte, 262144)
	for i := range random {
		random[i] = byte(rng.IntN(256))
	}
	return [][]byte{
		nil,
		[]byte("a"),
		[]byte("TOBEORNOTTOBEORTOBEORNOT"),
		bytes.Repeat([]byte("a"), 300000),  // one huge offset-1 match, 255-run length bytes
		bytes.Repeat([]byte("ab"), 100000), // period-2, offsets of 2
		bytes.Repeat([]byte{0xAA, 0x00, 0xCD}, 9000), // alternating short runs
		text,
		random,                  // incompressible: raw-block store path
		bytes.Repeat(text, 250), // > 4MB: multiple frame blocks
	}
}

func TestRoundTrip(t *testing.T) {
	for i, data := range corpus() {
		z := lz4.Compress(data)
		if z == nil {
			t.Fatalf("corpus %d: Compress returned nil", i)
		}
		if back := lz4.Expand(z); !bytes.Equal(back, data) {
			t.Fatalf("corpus %d: round trip mismatch (%d back)", i, len(back))
		}
	}
}

func TestCLIGoldensDecode(t *testing.T) {
	if got := lz4.Expand(hexBytes(t, goldenTobeHex)); string(got) != "TOBEORNOTTOBEORTOBEORNOT" {
		t.Fatalf("golden tobe: got %q", got)
	}
	if got := lz4.Expand(hexBytes(t, goldenEmptyHex)); len(got) != 0 {
		t.Fatalf("golden empty: got %d bytes", len(got))
	}
	dict := lz4.Expand(hexBytes(t, goldenDictHex))
	if len(dict) != 2000 || dict[0] != 'A' {
		t.Fatalf("golden dict: got %d bytes starting %q", len(dict), dict[:4])
	}
	// The -BX golden carries per-block checksums over the STORED
	// (compressed) bytes — the format's rule, not the decoded output.
	if got := lz4.Expand(hexBytes(t, goldenDictBCHex)); !bytes.Equal(got, dict) {
		t.Fatalf("golden dict -BX: %d bytes", len(got))
	}
	// The --content-size golden: header checksum covers FLG+BD+size.
	if got := lz4.Expand(hexBytes(t, goldenDictCSHex)); !bytes.Equal(got, dict) {
		t.Fatalf("golden dict --content-size: %d bytes", len(got))
	}
	// Our empty frame is pinned byte for byte.
	if z := lz4.Compress(nil); !bytes.Equal(z, hexBytes(t, ourEmptyHex)) {
		t.Fatalf("empty frame: got % x", z)
	}
}

// TestFrameShape checks the frame fields we write: magic, FLG
// (version 01, independent blocks, content checksum), BD (4MB
// blocks), a valid HC over FLG+BD, one size word per 4MB block, the
// raw-store high bit for incompressible blocks, the EndMark and the
// content checksum.
func TestFrameShape(t *testing.T) {
	z := lz4.Compress([]byte("TOBEORNOTTOBEORTOBEORNOT"))
	if len(z) < 15 {
		t.Fatalf("frame too short: %d", len(z))
	}
	if z[0] != 0x04 || z[1] != 0x22 || z[2] != 0x4D || z[3] != 0x18 {
		t.Fatalf("magic: % x", z[:4])
	}
	if z[4] != 0x64 {
		t.Fatalf("FLG: %#x, want 0x64", z[4])
	}
	if z[5] != 0x70 {
		t.Fatalf("BD: %#x, want 0x70", z[5])
	}
	if z[6] != 0xb9 {
		t.Fatalf("HC: %#x, want 0xb9", z[6])
	}
	end := z[len(z)-8 : len(z)-4]
	if !bytes.Equal(end, []byte{0, 0, 0, 0}) {
		t.Fatalf("missing EndMark: % x", end)
	}

	// Count blocks in a >4MB frame: walk size words from the header.
	big := lz4.Compress(bytes.Repeat([]byte("hello world "), 400000)) // 4.8MB
	pos := 7
	blocks := 0
	for {
		w := binary.LittleEndian.Uint32(big[pos:])
		pos += 4
		if w == 0 {
			break
		}
		sz := w & 0x7FFFFFFF
		pos += int(sz)
		blocks++
	}
	if blocks < 2 {
		t.Fatalf("4.8MB input: %d blocks, want >= 2", blocks)
	}

	// Incompressible input must be stored raw (bit 31 of the size word).
	rng := rand.New(rand.NewPCG(7, 7))
	incompressible := make([]byte, 100000)
	for i := range incompressible {
		incompressible[i] = byte(rng.IntN(256))
	}
	rz := lz4.Compress(incompressible)
	if w := binary.LittleEndian.Uint32(rz[7:]); w&0x80000000 == 0 {
		t.Fatalf("incompressible block not stored raw: size word %#x", w)
	}
}

// TestSkippableAndConcat: skippable frames are jumped anywhere a
// frame may appear, and concatenated frames decode to the
// concatenation of their contents.
func TestSkippableAndConcat(t *testing.T) {
	frame := hexBytes(t, goldenTobeHex)
	skip := append([]byte{0x50, 0x2A, 0x4D, 0x18, 5, 0, 0, 0}, []byte("hello")...)
	skipEmpty := []byte{0x5F, 0x2A, 0x4D, 0x18, 0, 0, 0, 0} // magic 0x184D2A5F, size 0

	for _, z := range [][]byte{
		append(append([]byte{}, skip...), frame...),
		append(append([]byte{}, skipEmpty...), frame...),
		append(append(append([]byte{}, frame...), skip...), hexBytes(t, goldenEmptyHex)...),
	} {
		if got := lz4.Expand(z); string(got) != "TOBEORNOTTOBEORTOBEORNOT" {
			t.Fatalf("skippable handling: got %q", got)
		}
	}

	two := append(append([]byte{}, frame...), frame...)
	if got := lz4.Expand(two); string(got) != "TOBEORNOTTOBEORTOBEORNOTTOBEORNOTTOBEORTOBEORNOT" {
		t.Fatalf("concat: got %q", got)
	}
}

// TestMalformed: every rejected shape returns nil (never panics).
// The badHeaders vectors carry header checksums precomputed offline
// with the reference XXH32, so each rejection is attributed to the
// rule it violates, not to a checksum mismatch.
func TestMalformed(t *testing.T) {
	dict := hexBytes(t, goldenDictHex)
	dictBC := hexBytes(t, goldenDictBCHex)
	dictCS := hexBytes(t, goldenDictCSHex)

	// Frames whose header passes the checksum but violates a rule.
	// (FLG, BD, optional bytes, HC — HC precomputed offline.)
	badHeaders := []struct {
		flg, bd, hc byte
		opt         []byte
		why         string
	}{
		{0x24, 0x70, 0x18, nil, "version 00"},
		{0xA4, 0x70, 0x3a, nil, "version 10"},
		{0x66, 0x70, 0x73, nil, "reserved FLG bit"},
		{0x44, 0x70, 0x1d, nil, "linked blocks"},
		{0x65, 0x70, 0x4c, []byte{1, 0, 0, 0}, "dictID frame"},
		{0x64, 0xC0, 0x42, nil, "BD reserved high bit"},
		{0x64, 0x41, 0xee, nil, "BD reserved low bits"},
		{0x64, 0x30, 0x13, nil, "block size id 3"},
	}
	for _, bh := range badHeaders {
		z := append([]byte{0x04, 0x22, 0x4D, 0x18, bh.flg, bh.bd}, bh.opt...)
		z = append(z, bh.hc)
		z = append(z, 0, 0, 0, 0, 0, 0, 0, 0) // EndMark + checksum
		if got := lz4.Expand(z); got != nil {
			t.Fatalf("%s: Expand returned %d bytes", bh.why, len(got))
		}
	}

	// Wrong header checksum on an otherwise valid golden.
	bad := append([]byte{}, dict...)
	bad[6] ^= 0xFF
	if got := lz4.Expand(bad); got != nil {
		t.Fatalf("bad HC: decoded %d bytes", len(got))
	}

	// Legacy magic.
	if got := lz4.Expand([]byte{0x02, 0x21, 0x4C, 0x18, 0, 0, 0, 0}); got != nil {
		t.Fatalf("legacy magic: decoded %d bytes", len(got))
	}

	// C.Size mismatch: patch the 8-byte size to 1999 and fix the HC
	// (0x62, precomputed) so only the length check can reject.
	bad = append([]byte{}, dictCS...)
	binary.LittleEndian.PutUint64(bad[6:14], 1999)
	bad[14] = 0x62
	if got := lz4.Expand(bad); got != nil {
		t.Fatalf("C.Size mismatch: decoded %d bytes", len(got))
	}

	// Block checksum corruption (over the stored bytes).
	bad = append([]byte{}, dictBC...)
	bad[len(bad)-9] ^= 0x01 // one byte of the block checksum
	if got := lz4.Expand(bad); got != nil {
		t.Fatalf("bad block checksum: decoded %d bytes", len(got))
	}

	// Content checksum corruption.
	bad = append([]byte{}, dict...)
	bad[len(bad)-1] ^= 0x01
	if got := lz4.Expand(bad); got != nil {
		t.Fatalf("bad content checksum: decoded %d bytes", len(got))
	}

	// Trailing garbage.
	if got := lz4.Expand(append(append([]byte{}, dict...), 0)); got != nil {
		t.Fatalf("trailing byte: decoded %d bytes", len(got))
	}

	// Malformed blocks inside a valid frame shell.
	block := func(body []byte) []byte {
		z := append([]byte{0x04, 0x22, 0x4D, 0x18, 0x64, 0x70, 0xb9}, 0, 0, 0, 0)
		binary.LittleEndian.PutUint32(z[7:11], uint32(len(body)))
		z = append(z[:11], body...)
		return append(z, 0, 0, 0, 0, 0, 0, 0, 0)
	}
	for _, c := range []struct {
		body []byte
		why  string
	}{
		{[]byte{0x00, 0x00, 0x00}, "offset 0"},
		{[]byte{0x00, 0xFF, 0x00}, "offset past history"},
		{[]byte{0xF0, 'a'}, "literals past end"},
		{[]byte{0x0F, 'a', 0x01, 0x00}, "extra matchLen bytes missing"},
		{[]byte{0x10, 'a', 0x01, 0x00, 0x00}, "truncated after offset"},
	} {
		if got := lz4.Expand(block(c.body)); got != nil {
			t.Fatalf("block %s: decoded %d bytes", c.why, len(got))
		}
	}

	// Expansion cap: a sequence demanding more than 1<<28 output bytes.
	extra := make([]byte, 1<<20)
	for i := range extra {
		extra[i] = 255
	}
	bomb := append([]byte{0x0F, 'a', 0x01, 0x00}, extra...)
	if got := lz4.Expand(block(bomb)); got != nil {
		t.Fatalf("expansion bomb: decoded %d bytes", len(got))
	}
}

// TestTruncated: every proper prefix of a golden is malformed.
func TestTruncated(t *testing.T) {
	for name, hex_ := range map[string]string{
		"tobe": goldenTobeHex,
		"dict": goldenDictHex,
	} {
		full := hexBytes(t, hex_)
		for cut := 0; cut < len(full); cut++ {
			if got := lz4.Expand(full[:cut]); got != nil {
				t.Fatalf("%s truncated to %d bytes: decoded %d bytes", name, cut, len(got))
			}
		}
	}
}

// TestTinyFrameHugeOutput: LZ4's overlap copy is a legal RLE — our
// encoder turns 10000 a's into a frame a few dozen bytes long.
func TestTinyFrameHugeOutput(t *testing.T) {
	z := lz4.Compress(bytes.Repeat([]byte("a"), 10000))
	if len(z) > 100 {
		t.Fatalf("10000 a's compressed to %d bytes", len(z))
	}
	if back := lz4.Expand(z); !bytes.Equal(back, bytes.Repeat([]byte("a"), 10000)) {
		t.Fatalf("overlap RLE round trip failed")
	}
}

func TestRandomizedModel(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 42))
	for i := 0; i < 200; i++ {
		n := rng.IntN(20000)
		data := make([]byte, n)
		switch rng.IntN(4) {
		case 0: // random bytes
			for j := range data {
				data[j] = byte(rng.IntN(256))
			}
		case 1: // runs
			for j := range data {
				if j == 0 || rng.IntN(8) == 0 {
					data[j] = byte(rng.IntN(256))
				} else {
					data[j] = data[j-1]
				}
			}
		case 2: // text-ish
			for j := range data {
				data[j] = byte('a' + rng.IntN(26))
			}
		case 3: // repeated chunks
			chunk := make([]byte, 1+rng.IntN(64))
			for j := range chunk {
				chunk[j] = byte(rng.IntN(256))
			}
			for j := range data {
				data[j] = chunk[j%len(chunk)]
			}
		}
		z := lz4.Compress(data)
		back := lz4.Expand(z)
		if !bytes.Equal(back, data) {
			t.Fatalf("iter %d (%d bytes): round trip mismatch", i, n)
		}
	}
}

// TestTarLz4RoundTrip is the .tar.lz4 pipeline.
func TestTarLz4RoundTrip(t *testing.T) {
	files := map[string]string{
		"greeting.txt": "hello, compressed world\n",
		"big.bin":      strings.Repeat("pluto", 20000),
	}
	var tarBuf bytes.Buffer
	tw := tar.NewWriter(&tarBuf)
	for name, content := range files {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(content))}); err != nil {
			t.Fatalf("tar header: %v", err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatalf("tar body: %v", err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("tar close: %v", err)
	}

	z := lz4.Compress(tarBuf.Bytes())
	if back := lz4.Expand(z); !bytes.Equal(back, tarBuf.Bytes()) {
		t.Fatalf("tar.lz4 round trip failed")
	}

	tr := tar.NewReader(bytes.NewReader(lz4.Expand(z)))
	seen := 0
	for {
		hdr, err := tr.Next()
		if err != nil {
			break
		}
		want := files[hdr.Name]
		got, err := io.ReadAll(tr)
		if err != nil {
			t.Fatalf("read %s: %v", hdr.Name, err)
		}
		if string(got) != want {
			t.Fatalf("file %s: content mismatch", hdr.Name)
		}
		seen++
	}
	if seen != len(files) {
		t.Fatalf("saw %d files, want %d", seen, len(files))
	}
}
