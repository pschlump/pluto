package hashing

import (
	"hash/adler32"
	"hash/crc32"
	"testing"
)

func TestStandardLibraryCompatibility(t *testing.T) {
	data := []byte("123456789")
	for _, test := range []struct {
		function HashFunction
		want     uint32
	}{
		{CRC32(), crc32.ChecksumIEEE(data)},
		{CRC32C(), crc32.Checksum(data, crc32.MakeTable(crc32.Castagnoli))},
		{Adler32(), adler32.Checksum(data)},
	} {
		if got := test.function.HashBytes(data).AsUint32(); got != test.want {
			t.Errorf("checksum got %08x want %08x", got, test.want)
		}
	}
	for _, test := range []struct {
		function HashFunction
		want     string
	}{
		{MD5(), "900150983cd24fb0d6963f7d28e17f72"},
		{SHA1(), "a9993e364706816aba3e25717850c26c9cd0d89d"},
		{SHA256(), "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"},
		{SHA512(), "ddaf35a193617abacc417349ae20413112e6fa4e89a97ea20a9eeee64b55d39a2192992a274fc1a836ba3c23a3feebbd454d4423643ce80e2a9ac94fa54ca49f"},
	} {
		if got := test.function.HashString("abc").String(); got != test.want {
			t.Errorf("digest got %s want %s", got, test.want)
		}
	}
}

func TestInvalidArguments(t *testing.T) {
	for _, function := range []func(){
		func() { Murmur3_32(1, 2) },
		func() { SipHash24(1) },
		func() { CombineOrdered() },
		func() { CombineUnordered() },
		func() { ConsistentHash(HashCode{}, 0) },
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Error("invalid argument did not panic")
				}
			}()
			function()
		}()
	}
}
