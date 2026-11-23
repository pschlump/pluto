/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

package hashing

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"hash"
	"hash/adler32"
	"hash/crc32"
)

func standardHash(bits int, newHash func() hash.Hash) HashFunction {
	return newHashFunction(bits, func(data []byte) HashCode {
		h := newHash()
		_, _ = h.Write(data)
		return NewHashCode(h.Sum(nil), BigEndian)
	})
}

// MD5 returns the MD5 interoperability hash. MD5 is not collision resistant.
func MD5() HashFunction { return standardHash(md5.Size*8, md5.New) }

// SHA1 returns the SHA-1 interoperability hash. SHA-1 is not collision resistant.
func SHA1() HashFunction { return standardHash(sha1.Size*8, sha1.New) }

// SHA256 returns SHA-256.
func SHA256() HashFunction { return standardHash(sha256.Size*8, sha256.New) }

// SHA512 returns SHA-512.
func SHA512() HashFunction { return standardHash(sha512.Size*8, sha512.New) }

// CRC32 returns IEEE CRC-32 with a little-endian 32-bit HashCode.
func CRC32() HashFunction {
	return newHashFunction(32, func(data []byte) HashCode {
		return hashFromUint32(crc32.ChecksumIEEE(data))
	})
}

// CRC32C returns Castagnoli CRC-32C with a little-endian 32-bit HashCode.
func CRC32C() HashFunction {
	return newHashFunction(32, func(data []byte) HashCode {
		return hashFromUint32(crc32.Checksum(data, crc32.MakeTable(crc32.Castagnoli)))
	})
}

// Adler32 returns Adler-32 with a little-endian 32-bit HashCode.
func Adler32() HashFunction {
	return newHashFunction(32, func(data []byte) HashCode {
		return hashFromUint32(adler32.Checksum(data))
	})
}
