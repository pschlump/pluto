package bloom_filter

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"testing"

	"github.com/pschlump/pluto/hashing"
)

type bloomGolden struct {
	Bloom struct {
		ExpectedInsertions int     `json:"expectedInsertions"`
		FPP                float64 `json:"fpp"`
		GuavaEncoding      string  `json:"guavaEncoding"`
		ExpectedFPP        float64 `json:"expectedFpp"`
		ApproximateCount   float64 `json:"approximateElementCount"`
	} `json:"bloom"`
}

func guavaGolden(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile("../hashing/testdata/guava_vectors.json")
	if err != nil {
		t.Fatal(err)
	}
	var golden bloomGolden
	if err := json.Unmarshal(data, &golden); err != nil {
		t.Fatal(err)
	}
	encoded, err := base64.StdEncoding.DecodeString(golden.Bloom.GuavaEncoding)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

func TestGuavaBitArrayDifferential(t *testing.T) {
	encoded := guavaGolden(t)
	guava, err := FromGuavaBytes(encoded, hashing.StringFunnel)
	if err != nil {
		t.Fatal(err)
	}
	filter := New(1000, 0.03, hashing.StringFunnel)
	for i := range 1000 {
		filter.Put(fmt.Sprintf("value-%04d", i))
	}
	if filter.BitCount() != 7360 || filter.HashCount() != 5 {
		t.Fatalf("shape = %d/%d, want 7360/5", filter.BitCount(), filter.HashCount())
	}
	wantWords := binary.BigEndian.Uint32(encoded[2:])
	if int(wantWords) != len(filter.words) {
		t.Fatalf("word count = %d want %d", len(filter.words), wantWords)
	}
	for i, word := range filter.words {
		want := binary.BigEndian.Uint64(encoded[6+i*8:])
		if word != want {
			t.Fatalf("word %d = %016x want %016x", i, word, want)
		}
	}
	for i := range 1000 {
		value := fmt.Sprintf("value-%04d", i)
		if !filter.MightContain(value) || !guava.MightContain(value) {
			t.Fatalf("false negative for %q", value)
		}
	}
	if math.Abs(guava.ExpectedFPP()-0.030000000000000027) > 1e-9 {
		// Check the formula separately; Guava's rounded shape differs from New.
		want := math.Pow(guava.Saturation(), float64(guava.HashCount()))
		if math.Abs(guava.ExpectedFPP()-want) > 1e-15 {
			t.Fatalf("Guava decoded FPP = %g want %g", guava.ExpectedFPP(), want)
		}
	}
	var out bytes.Buffer
	if err := filter.WriteGuavaTo(&out); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out.Bytes(), encoded) {
		t.Fatal("Guava compact encoding is not byte-identical")
	}
}

func TestMergeCopyAndPlutoStream(t *testing.T) {
	left := New(1000, 0.01, hashing.StringFunnel)
	right := New(1000, 0.01, hashing.StringFunnel)
	for i := range 500 {
		left.Put(fmt.Sprintf("left-%d", i))
		right.Put(fmt.Sprintf("right-%d", i))
	}
	copyFilter := left.Copy()
	if !left.Compatible(copyFilter) || !left.Compatible(right) || left.Compatible(New(1000, 0.02, hashing.StringFunnel)) {
		t.Fatal("copy/compatibility mismatch")
	}
	before := left.Bytes()
	if err := left.Merge(right, copyFilter, left); err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(before, left.Bytes()) {
		t.Fatal("merge did not change filter")
	}
	for i := range 500 {
		if !left.MightContain(fmt.Sprintf("left-%d", i)) || !left.MightContain(fmt.Sprintf("right-%d", i)) {
			t.Fatalf("merge lost value %d", i)
		}
	}
	bad := New(1000, 0.02, hashing.StringFunnel)
	bad.Put("x")
	before = left.Bytes()
	if err := left.Merge(bad); !errors.Is(err, ErrIncompatible) || !bytes.Equal(before, left.Bytes()) {
		t.Fatal("incompatible merge was not atomic")
	}
	encoded := left.Bytes()
	decoded, err := FromBytes(encoded, hashing.StringFunnel)
	if err != nil || !decoded.Compatible(left) || !bytes.Equal(decoded.Bytes(), encoded) {
		t.Fatalf("stream round trip failed: %v", err)
	}
	var stream bytes.Buffer
	if _, err := left.WriteTo(&stream); err != nil {
		t.Fatal(err)
	}
	decoded, err = ReadFrom(&stream, hashing.StringFunnel)
	if err != nil || !bytes.Equal(decoded.Bytes(), encoded) {
		t.Fatalf("ReadFrom failed: %v", err)
	}
	for _, mutate := range []func([]byte) []byte{
		func(data []byte) []byte { return data[:3] },
		func(data []byte) []byte { data[3]++; return data },
		func(data []byte) []byte { return data[:len(data)-1] },
		func(data []byte) []byte {
			for i := 4; i < 12; i++ {
				data[i] = 0
			}
			return data
		},
	} {
		if _, err := FromBytes(mutate(append([]byte(nil), encoded...)), hashing.StringFunnel); err == nil {
			t.Fatal("corrupt encoding accepted")
		}
	}
}

func TestAccuracyAndApproximateCount(t *testing.T) {
	const n = 10_000
	filter := New[string](n, 0.01, hashing.StringFunnel)
	for i := range n {
		filter.Put(fmt.Sprintf("member-%d", i))
	}
	falsePositives := 0
	for i := range n {
		if filter.MightContain(fmt.Sprintf("absent-%d", i)) {
			falsePositives++
		}
	}
	measured := float64(falsePositives) / n
	if measured > 0.025 {
		t.Fatalf("measured FPP %g exceeds bound", measured)
	}
	if count := filter.ApproximateElementCount(); count < 9000 || count > 11000 {
		t.Fatalf("approximate count %d outside 10%%", count)
	}
	if filter.ExpectedFPP() <= 0 || filter.ExpectedFPP() >= 0.1 {
		t.Fatalf("unexpected expected FPP %g", filter.ExpectedFPP())
	}
}

func TestZeroAndInvalidShapes(t *testing.T) {
	var filter *BloomFilter[string]
	if filter.MightContain("x") || filter.BitCount() != 0 || filter.Count() != 0 || filter.ExpectedFPP() != 0 {
		t.Fatal("nil read mismatch")
	}
	if !filter.Compatible(nil) || !filter.Compatible(&BloomFilter[string]{}) || filter.Compatible(New(1, 0.1, hashing.StringFunnel)) {
		t.Fatal("nil compatibility mismatch")
	}
	if err := filter.Merge(); !errors.Is(err, ErrInvalidShape) {
		t.Fatal("zero destination merge mismatch")
	}
}
