/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package crc

import (
	"strings"
	"testing"
)

// The published check vectors for the ASCII string "123456789" — the
// standard catalogue value every CRC implementation is verified
// against.
func TestPublishedCheckVectors(t *testing.T) {
	data := []byte("123456789")
	cases := []struct {
		name string
		got  uint64
		want uint64
	}{
		{"CRC-32/IEEE", uint64(Checksum32(data, IEEETable)), 0xCBF43926},
		{"CRC-32C/Castagnoli", uint64(Checksum32(data, CastagnoliTable)), 0xE3069283},
		{"CRC-32K/Koopman", uint64(Checksum32(data, KoopmanTable)), 0x2D3DD0AE},
		{"CRC-64/ECMA", Checksum64(data, ECMATable), 0x995DC9BBDF1939FA},
		{"CRC-64/ISO", Checksum64(data, ISOTable), 0xB90956C775A41001},
	}
	for _, tc := range cases {
		if tc.got != tc.want {
			t.Errorf("%s of \"123456789\" = 0x%X, want 0x%X", tc.name, tc.got, tc.want)
		}
	}
}

// The prebuilt package tables must be identical to freshly made ones.
func TestPrebuiltTablesMatchMakeTable(t *testing.T) {
	if *IEEETable != *MakeTable32(IEEE) {
		t.Error("IEEETable differs from MakeTable32(IEEE)")
	}
	if *CastagnoliTable != *MakeTable32(Castagnoli) {
		t.Error("CastagnoliTable differs from MakeTable32(Castagnoli)")
	}
	if *KoopmanTable != *MakeTable32(Koopman) {
		t.Error("KoopmanTable differs from MakeTable32(Koopman)")
	}
	if *ECMATable != *MakeTable64(ECMA) {
		t.Error("ECMATable differs from MakeTable64(ECMA)")
	}
	if *ISOTable != *MakeTable64(ISO) {
		t.Error("ISOTable differs from MakeTable64(ISO)")
	}
}

// An incremental checksum built in odd-sized chunks must equal the
// one-shot checksum of the whole buffer.
func TestUpdate32Incremental(t *testing.T) {
	data := []byte("the quick brown fox jumps over the lazy dog, again and again")
	oneShot := Checksum32(data, CastagnoliTable)
	var crc uint32
	for _, size := range []int{1, 7, 3, 100, 5} { // 100 runs past the end: clamp
		n := min(size, len(data))
		crc = Update32(crc, CastagnoliTable, data[:n])
		data = data[n:]
		if len(data) == 0 {
			break
		}
	}
	if len(data) != 0 {
		t.Fatalf("test setup error: %d bytes left over", len(data))
	}
	if crc != oneShot {
		t.Errorf("chunked Update32 = 0x%X, one-shot Checksum32 = 0x%X", crc, oneShot)
	}
}

// Same incremental check for the 64-bit flavor.
func TestUpdate64Incremental(t *testing.T) {
	data := []byte("the quick brown fox jumps over the lazy dog, again and again")
	oneShot := Checksum64(data, ECMATable)
	var crc uint64
	for _, size := range []int{13, 2, 9, 1, 64} {
		n := min(size, len(data))
		crc = Update64(crc, ECMATable, data[:n])
		data = data[n:]
		if len(data) == 0 {
			break
		}
	}
	if crc != oneShot {
		t.Errorf("chunked Update64 = 0x%X, one-shot Checksum64 = 0x%X", crc, oneShot)
	}
}

// An empty input leaves the running CRC unchanged (and starts at 0).
func TestEmptyInput(t *testing.T) {
	if got := Checksum32(nil, IEEETable); got != 0 {
		t.Errorf("Checksum32(nil) = 0x%X, want 0", got)
	}
	if got := Checksum64([]byte{}, ISOTable); got != 0 {
		t.Errorf("Checksum64(empty) = 0x%X, want 0", got)
	}
	const sentinel = 0xDEADBEEF
	if got := Update32(sentinel, IEEETable, nil); got != sentinel {
		t.Errorf("Update32 with no data = 0x%X, want the running value 0x%X", got, sentinel)
	}
}

// expectPanicMsg runs fx, requires it to panic, and requires the panic
// message to name the method and the fix.
func expectPanicMsg(t *testing.T, method string, fx func()) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Errorf("Expected %s to panic on a nil table, it did not.", method)
			return
		}
		msg, ok := r.(string)
		if !ok {
			t.Errorf("%s panicked with %v, not a message string", method, r)
			return
		}
		if !strings.Contains(msg, method) || !strings.Contains(msg, "MakeTable") {
			t.Errorf("%s panic message %q should name the method and the fix (MakeTable…)", method, msg)
		}
	}()
	fx()
}

// A nil table is a programmer error with no sane answer: every
// checksum entry point panics, naming the method and the fix.
func TestNilTablePanics(t *testing.T) {
	data := []byte("some data")
	expectPanicMsg(t, "Checksum32", func() { Checksum32(data, nil) })
	expectPanicMsg(t, "Update32", func() { Update32(0, nil, data) })
	expectPanicMsg(t, "Checksum64", func() { Checksum64(data, nil) })
	expectPanicMsg(t, "Update64", func() { Update64(0, nil, data) })
}

var benchData4K = func() []byte {
	b := make([]byte, 4096)
	for i := range b {
		b[i] = byte(i * 31)
	}
	return b
}()

// A 4 KiB block is the journal-batch granularity of b_tree_disk_ts.
func BenchmarkChecksum32_4K(b *testing.B) {
	b.SetBytes(int64(len(benchData4K)))
	for i := 0; i < b.N; i++ {
		Checksum32(benchData4K, CastagnoliTable)
	}
}

func BenchmarkChecksum64_4K(b *testing.B) {
	b.SetBytes(int64(len(benchData4K)))
	for i := 0; i < b.N; i++ {
		Checksum64(benchData4K, ECMATable)
	}
}
