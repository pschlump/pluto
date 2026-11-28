package bloom_filter

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math"
	"math/rand"
	"testing"

	"github.com/pschlump/pluto/hashing"
)

// fastPathRecord exercises every PrimitiveSink method so the probeSink
// encoding is compared against hashing's buffered hasher method by method.
type fastPathRecord struct {
	B   byte
	Raw []byte
	S   string
	U16 uint16
	U32 uint32
	U64 uint64
	I8  int8
	I16 int16
	I32 int32
	I64 int64
	F32 float32
	F64 float64
}

func fastPathRecordFunnel(value fastPathRecord, sink hashing.PrimitiveSink) {
	sink.PutByte(value.B).PutBytes(value.Raw).PutString(value.S)
	sink.PutUint16(value.U16).PutUint32(value.U32).PutUint64(value.U64)
	sink.PutInt8(value.I8).PutInt16(value.I16).PutInt32(value.I32).PutInt64(value.I64)
	sink.PutFloat32(value.F32).PutFloat64(value.F64)
}

// referenceHashes is the original (pre-2026-11-27) probe derivation: the
// public HashObject path and HashCode.Bytes.
func referenceHashes[T any](value T, funnel hashing.Funnel[T]) (uint64, uint64) {
	code := hashing.HashObject(value, funnel, hashing.Murmur3_128())
	raw := code.Bytes()
	return binary.LittleEndian.Uint64(raw), binary.LittleEndian.Uint64(raw[8:])
}

// referencePut sets bits exactly as the original positions-slice Put did.
func referencePut[T any](words []uint64, bitSize uint64, hashCount int, value T, funnel hashing.Funnel[T]) {
	hash1, hash2 := referenceHashes(value, funnel)
	combined := hash1
	for range hashCount {
		position := (combined & math.MaxInt64) % bitSize
		words[position>>6] |= uint64(1) << (position & 63)
		combined += hash2
	}
}

func randomBytes(r *rand.Rand, maxLength int) []byte {
	out := make([]byte, r.Intn(maxLength+1))
	r.Read(out)
	return out
}

func TestFastHashMatchesHashObject(t *testing.T) {
	r := rand.New(rand.NewSource(42))
	strings := New(100, 0.01, hashing.StringFunnel)
	raw := New(100, 0.01, hashing.BytesFunnel)
	ints := New(100, 0.01, hashing.Int64Funnel)
	records := New(100, 0.01, fastPathRecordFunnel)
	for i := range 5000 {
		// Lengths 0..80 cross every Murmur3 tail length and several blocks.
		s := string(randomBytes(r, 80))
		a1, a2 := strings.hash128(s)
		if b1, b2 := referenceHashes(s, hashing.StringFunnel); a1 != b1 || a2 != b2 {
			t.Fatalf("string %d %q: fast %016x/%016x reference %016x/%016x", i, s, a1, a2, b1, b2)
		}
		data := randomBytes(r, 80)
		a1, a2 = raw.hash128(data)
		if b1, b2 := referenceHashes(data, hashing.BytesFunnel); a1 != b1 || a2 != b2 {
			t.Fatalf("bytes %d %x: fast/reference hashes differ", i, data)
		}
		n := int64(r.Uint64())
		a1, a2 = ints.hash128(n)
		if b1, b2 := referenceHashes(n, hashing.Int64Funnel); a1 != b1 || a2 != b2 {
			t.Fatalf("int64 %d %d: fast/reference hashes differ", i, n)
		}
		record := fastPathRecord{
			B: byte(r.Uint32()), Raw: randomBytes(r, 20), S: string(randomBytes(r, 20)),
			U16: uint16(r.Uint32()), U32: r.Uint32(), U64: r.Uint64(),
			I8: int8(r.Uint32()), I16: int16(r.Uint32()), I32: int32(r.Uint32()), I64: int64(r.Uint64()),
			F32: r.Float32(), F64: r.NormFloat64(),
		}
		a1, a2 = records.hash128(record)
		if b1, b2 := referenceHashes(record, fastPathRecordFunnel); a1 != b1 || a2 != b2 {
			t.Fatalf("record %d %+v: fast/reference hashes differ", i, record)
		}
	}
}

func TestFastPutMatchesReferenceBits(t *testing.T) {
	r := rand.New(rand.NewSource(42))
	shapes := []struct {
		bits   uint64
		hashes int
	}{{1, 1}, {64, 3}, {1000, 7}, {7360, 5}, {100_003, 13}, {1 << 20, 255}}
	for _, shape := range shapes {
		filter := NewWithBits(shape.bits, shape.hashes, hashing.StringFunnel)
		want := make([]uint64, len(filter.words))
		values := make([]string, 400)
		for i := range values {
			values[i] = string(randomBytes(r, 40))
			filter.Put(values[i])
			referencePut(want, filter.bitSize, shape.hashes, values[i], hashing.StringFunnel)
		}
		for i := range want {
			if filter.words[i] != want[i] {
				t.Fatalf("shape %d/%d word %d = %016x want %016x", shape.bits, shape.hashes, i, filter.words[i], want[i])
			}
		}
		var set uint64
		for _, word := range want {
			for ; word != 0; word &= word - 1 {
				set++
			}
		}
		if filter.SetBitCount() != set {
			t.Fatalf("shape %d/%d set bits = %d want %d", shape.bits, shape.hashes, filter.SetBitCount(), set)
		}
		for _, value := range values {
			if !filter.MightContain(value) {
				t.Fatalf("shape %d/%d false negative for %q", shape.bits, shape.hashes, value)
			}
		}
	}
}

// TestReentrantFunnel shows the pooled sink is not shared with a nested call:
// a funnel that queries the same filter still hashes its own bytes.
func TestReentrantFunnel(t *testing.T) {
	var filter *BloomFilter[string]
	nested := 0
	funnel := func(value string, sink hashing.PrimitiveSink) {
		sink.PutString(value[:len(value)/2])
		if nested == 0 {
			nested++
			filter.MightContain("nested-probe-value")
			nested--
		}
		sink.PutString(value[len(value)/2:])
	}
	filter = New(100, 0.01, funnel)
	filter.Put("outer-value")
	want := New(100, 0.01, hashing.StringFunnel)
	want.Put("outer-value")
	if !bytes.Equal(filter.Bytes(), want.Bytes()) {
		t.Fatal("re-entrant funnel corrupted the outer value's encoding")
	}
}

func TestProbeAllocations(t *testing.T) {
	if raceEnabled {
		t.Skip("sync.Pool deliberately drops items under the race detector")
	}
	filter := New(1000, 0.01, hashing.StringFunnel)
	values := []string{"value-0000", "value-0001", "a-somewhat-longer-value-crossing-blocks"}
	for _, value := range values {
		filter.Put(value)
	}
	i := 0
	if allocs := testing.AllocsPerRun(1000, func() {
		filter.MightContain(values[i%len(values)])
		filter.MightContain("absent-value")
		i++
	}); allocs != 0 {
		t.Errorf("MightContain allocates %v times per call, want 0", allocs)
	}
	if allocs := testing.AllocsPerRun(1000, func() {
		filter.Put(values[i%len(values)])
		i++
	}); allocs != 0 {
		t.Errorf("Put allocates %v times per call, want 0", allocs)
	}
	records := New(1000, 0.01, fastPathRecordFunnel)
	record := fastPathRecord{Raw: []byte("raw"), S: "string"}
	if allocs := testing.AllocsPerRun(1000, func() {
		records.Put(record)
		records.MightContain(record)
	}); allocs != 0 {
		t.Errorf("all-primitives funnel allocates %v times per call, want 0", allocs)
	}
}

// referenceGuava is the original word-at-a-time WriteGuavaTo encoding.
func referenceGuava(hashCount int, words []uint64) []byte {
	out := []byte{1, byte(hashCount), 0, 0, 0, 0}
	binary.BigEndian.PutUint32(out[2:], uint32(len(words)))
	for _, word := range words {
		out = binary.BigEndian.AppendUint64(out, word)
	}
	return out
}

// oneByteWriter accepts at most one byte per call and optionally fails after
// limit bytes, to exercise writeAll's partial-write loop across chunks.
type oneByteWriter struct {
	out   bytes.Buffer
	limit int
}

var errWriterFull = errors.New("writer full")

func (w *oneByteWriter) Write(data []byte) (int, error) {
	if w.limit >= 0 && w.out.Len() >= w.limit {
		return 0, errWriterFull
	}
	w.out.WriteByte(data[0])
	return 1, nil
}

func TestWriteGuavaChunking(t *testing.T) {
	r := rand.New(rand.NewSource(42))
	chunkWords := (guavaWriteChunk - 6) / 8
	for _, words := range []int{1, 2, chunkWords - 1, chunkWords, chunkWords + 1, 2*chunkWords + 3} {
		filter := NewWithBits(uint64(words)*64, 9, hashing.StringFunnel)
		for i := range filter.words {
			filter.words[i] = r.Uint64()
		}
		want := referenceGuava(9, filter.words)
		var out bytes.Buffer
		if err := filter.WriteGuavaTo(&out); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(out.Bytes(), want) {
			t.Fatalf("%d words: chunked Guava encoding differs from reference", words)
		}
		if words <= chunkWords+1 {
			slow := &oneByteWriter{limit: -1}
			if err := filter.WriteGuavaTo(slow); err != nil || !bytes.Equal(slow.out.Bytes(), want) {
				t.Fatalf("%d words: one-byte writer err=%v or bytes differ", words, err)
			}
		}
		full := &oneByteWriter{limit: len(want) - 1}
		if err := filter.WriteGuavaTo(full); !errors.Is(err, errWriterFull) {
			t.Fatalf("%d words: failing writer err = %v, want errWriterFull", words, err)
		}
	}
}
