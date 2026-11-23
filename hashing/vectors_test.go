package hashing

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"math"
	"os"
	"reflect"
	"testing"
)

type goldenVector struct {
	Algorithm string `json:"algorithm"`
	Seed      uint32 `json:"seed"`
	Input     string `json:"input"`
	Hash      string `json:"hash"`
}

type goldenFile struct {
	Version  int            `json:"version"`
	Vectors  []goldenVector `json:"vectors"`
	Combined []goldenVector `json:"combined"`
	Buckets  []int          `json:"buckets"`
}

func readGolden(t *testing.T) goldenFile {
	t.Helper()
	data, err := os.ReadFile("testdata/guava_vectors.json")
	if err != nil {
		t.Fatal(err)
	}
	var golden goldenFile
	if err := json.Unmarshal(data, &golden); err != nil {
		t.Fatal(err)
	}
	return golden
}

func vectorFunction(algorithm string, seed uint32) HashFunction {
	switch algorithm {
	case "murmur3_32":
		return Murmur3_32(seed)
	case "murmur3_128":
		return Murmur3_128(seed)
	case "sipHash24":
		return SipHash24()
	case "farmHashFingerprint64":
		return FarmHashFingerprint64()
	case "fingerprint2011":
		return Fingerprint2011()
	default:
		panic(algorithm)
	}
}

func TestGuavaVectorsAndChunking(t *testing.T) {
	for _, vector := range readGolden(t).Vectors {
		input, err := base64.StdEncoding.DecodeString(vector.Input)
		if err != nil {
			t.Fatal(err)
		}
		function := vectorFunction(vector.Algorithm, vector.Seed)
		if got := function.HashBytes(input).String(); got != vector.Hash {
			t.Errorf("%s seed=%d input=%x: got %s want %s", vector.Algorithm, vector.Seed, input, got, vector.Hash)
			continue
		}
		for _, chunk := range []int{1, 2, 3, 7, 16, 64} {
			hasher := function.NewHasher()
			for offset := 0; offset < len(input); {
				end := min(offset+chunk, len(input))
				hasher.PutBytes(input[offset:end])
				offset = end
			}
			if got := hasher.Hash().String(); got != vector.Hash {
				t.Errorf("%s seed=%d len=%d chunk=%d: got %s want %s", vector.Algorithm, vector.Seed, len(input), chunk, got, vector.Hash)
			}
		}
	}
}

func TestPrimitiveWritesAndFunnels(t *testing.T) {
	var data []byte
	data = append(data, 0xff)
	data = binary.LittleEndian.AppendUint16(data, 64302)
	data = binary.LittleEndian.AppendUint32(data, 0xdeadbeef)
	data = binary.LittleEndian.AppendUint64(data, 0x0123456789abcdef)
	data = binary.LittleEndian.AppendUint32(data, math.Float32bits(1.25))
	data = binary.LittleEndian.AppendUint64(data, math.Float64bits(-3.5))
	data = append(data, "世界"...)
	want := Murmur3_128().HashBytes(data)
	h := Murmur3_128().NewHasher()
	h.PutInt8(-1).PutInt16(-1234).PutUint32(0xdeadbeef).PutUint64(0x0123456789abcdef).
		PutFloat32(1.25).PutFloat64(-3.5).PutString("世界")
	if got := h.Hash(); !got.Equal(want) {
		t.Fatalf("primitive stream %s != bytes %s", got, want)
	}
	type pair struct{ A, B string }
	ambiguous := func(value pair, sink PrimitiveSink) { sink.PutString(value.A).PutString(value.B) }
	framed := func(value pair, sink PrimitiveSink) {
		sink.PutUint32(uint32(len(value.A))).PutString(value.A).PutUint32(uint32(len(value.B))).PutString(value.B)
	}
	if !HashObject(pair{"ab", "c"}, ambiguous, Murmur3_32()).Equal(HashObject(pair{"a", "bc"}, ambiguous, Murmur3_32())) {
		t.Fatal("unframed test premise failed")
	}
	if HashObject(pair{"ab", "c"}, framed, Murmur3_32()).Equal(HashObject(pair{"a", "bc"}, framed, Murmur3_32())) {
		t.Fatal("length framing did not distinguish concatenation")
	}
}

func TestHashCodeAccessAndImmutability(t *testing.T) {
	source := []byte{1, 2, 3, 4, 5, 6, 7, 8}
	little := NewHashCode(source, LittleEndian)
	big := NewHashCode(source, BigEndian)
	source[0] = 99
	if little.AsUint32() != 0x04030201 || big.AsUint32() != 0x01020304 {
		t.Fatal("32-bit endian access mismatch")
	}
	if little.AsUint64() != 0x0807060504030201 || big.AsUint64() != 0x0102030405060708 {
		t.Fatal("64-bit endian access mismatch")
	}
	bytes := little.Bytes()
	bytes[0] = 88
	if little.Bytes()[0] != 1 || little.Equal(big) {
		t.Fatal("HashCode leaked mutable storage or ignored byte order")
	}
	out := make([]byte, 8)
	n, err := little.WriteBytesTo(out, 2, 4)
	if err != nil || n != 4 || !reflect.DeepEqual(out, []byte{0, 0, 1, 2, 3, 4, 0, 0}) {
		t.Fatalf("bounded copy n=%d err=%v out=%x", n, err, out)
	}
	if _, err := little.WriteBytesTo(out, 7, 2); err == nil {
		t.Fatal("invalid bounded copy succeeded")
	}
	short := NewHashCode([]byte{0xff}, LittleEndian)
	if short.PadToUint64() != 255 {
		t.Fatal("short hash padding mismatch")
	}
}

func TestCombiningAndConsistentHash(t *testing.T) {
	golden := readGolden(t)
	a, b := Murmur3_128().HashString("a"), Murmur3_128().HashString("b")
	ordered := CombineOrdered(a, b)
	unordered := CombineUnordered(a, b)
	if ordered.String() != golden.Combined[0].Hash || unordered.String() != golden.Combined[1].Hash {
		t.Fatalf("combined hashes differ: %s %s", ordered, unordered)
	}
	if ordered.Equal(CombineOrdered(b, a)) || !unordered.Equal(CombineUnordered(b, a)) {
		t.Fatal("ordered/unordered combination law failed")
	}
	for i, buckets := range []int{1, 2, 3, 7, 8, 100, 1024} {
		if got := ConsistentHash(a, buckets); got != golden.Buckets[i] {
			t.Errorf("ConsistentHash(%d)=%d want %d", buckets, got, golden.Buckets[i])
		}
	}
	counts := make([]int, 100)
	for i := range 10_000 {
		code := Murmur3_128().HashString(string(rune(i)))
		old := ConsistentHash(code, 100)
		counts[old]++
		updated := ConsistentHash(code, 101)
		if updated != old && updated != 100 {
			t.Fatalf("minimal remapping violated at %d: %d -> %d", i, old, updated)
		}
	}
	for bucket, count := range counts {
		if count < 50 || count > 150 {
			t.Fatalf("bucket %d has %d/10000 assignments; distribution is skewed", bucket, count)
		}
	}
}
