/*
Copyright (C) Philip Schlump, 2012-2026.

BSD 3 Clause Licensed.
*/

// Package hashing provides immutable hash values, streaming hash functions,
// typed funnels, Guava-compatible non-cryptographic hashes, checksums, and
// jump-consistent hashing. None of these functions is intended for password
// hashing or a new cryptographic protocol. MD5 and SHA-1 are provided only for
// interoperability with existing formats.
package hashing

import "fmt"

// PrimitiveSink receives an unambiguous sequence of bytes and primitive values.
// Integer methods use little-endian encoding. String writes UTF-8 bytes without
// an implicit terminator; funnels should add a delimiter or length when needed.
type PrimitiveSink interface {
	PutByte(byte) PrimitiveSink
	PutBytes([]byte) PrimitiveSink
	PutString(string) PrimitiveSink
	PutUint16(uint16) PrimitiveSink
	PutUint32(uint32) PrimitiveSink
	PutUint64(uint64) PrimitiveSink
	PutInt8(int8) PrimitiveSink
	PutInt16(int16) PrimitiveSink
	PutInt32(int32) PrimitiveSink
	PutInt64(int64) PrimitiveSink
	PutFloat32(float32) PrimitiveSink
	PutFloat64(float64) PrimitiveSink
}

// Hasher accumulates one hash. A Hasher is not safe for concurrent use. Hash
// does not discard the input, but callers should normally obtain a new Hasher
// for each value.
type Hasher interface {
	PrimitiveSink
	Hash() HashCode
}

// HashFunction constructs independent streaming hashers and hashes common
// inputs directly.
type HashFunction interface {
	Bits() int
	NewHasher() Hasher
	HashBytes([]byte) HashCode
	HashString(string) HashCode
}

// Funnel encodes a typed value into a PrimitiveSink. It is a function rather
// than an interface so values never need to be boxed into any.
type Funnel[T any] func(T, PrimitiveSink)

// HashObject hashes value using funnel and function. A nil function or funnel
// is a programmer error and panics.
func HashObject[T any](value T, funnel Funnel[T], function HashFunction) HashCode {
	if function == nil || funnel == nil {
		panic("hashing: HashObject requires a non-nil function and funnel")
	}
	h := function.NewHasher()
	funnel(value, h)
	return h.Hash()
}

// StringFunnel writes the UTF-8 representation without a terminator.
func StringFunnel(value string, sink PrimitiveSink) { sink.PutString(value) }

// BytesFunnel writes bytes without a length prefix.
func BytesFunnel(value []byte, sink PrimitiveSink) { sink.PutBytes(value) }

// Int64Funnel writes an integer in little-endian form.
func Int64Funnel(value int64, sink PrimitiveSink) { sink.PutInt64(value) }

type hashFunction struct {
	bits int
	hash func([]byte) HashCode
}

func (f hashFunction) Bits() int { return f.bits }
func (f hashFunction) NewHasher() Hasher {
	return &bufferedHasher{function: f}
}
func (f hashFunction) HashBytes(data []byte) HashCode   { return f.hash(data) }
func (f hashFunction) HashString(value string) HashCode { return f.hash([]byte(value)) }

func newHashFunction(bits int, hash func([]byte) HashCode) HashFunction {
	if hash == nil {
		panic(fmt.Sprintf("hashing: nil %d-bit hash implementation", bits))
	}
	return hashFunction{bits: bits, hash: hash}
}
